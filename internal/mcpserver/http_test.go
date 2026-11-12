package mcpserver_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/mcpserver"
)

func serve(t *testing.T, src mcpserver.Source) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mcpserver.Handler(src, nil))
	t.Cleanup(srv.Close)
	return srv
}

// TestBothProtocolEras: one endpoint answers the stateless revision and the older initialize handshake.
func TestBothProtocolEras(t *testing.T) {
	srv := serve(t, stand(t))
	for _, version := range []string{"" /* the SDK's latest: 2026-07-28 */, "2025-11-25", "2025-06-18", "2025-03-26"} {
		t.Run("protocol "+version, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx,
				&mcp.StreamableClientTransport{Endpoint: srv.URL, DisableStandaloneSSE: true, MaxRetries: -1},
				&mcp.ClientSessionOptions{ProtocolVersion: version})
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			tools, err := cs.ListTools(ctx, nil)
			if err != nil || len(tools.Tools) != 4 {
				t.Fatalf("tools/list: %v, %d tools", err, len(tools.Tools))
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_employee", Arguments: map[string]any{"id": "harnsy/sample-plain"}})
			if err != nil || res.IsError || !strings.Contains(text(res), "sample-plain/AGENT.md") {
				t.Fatalf("tools/call: %v, %s", err, text(res))
			}
		})
	}
}

const listTools = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

func post(t *testing.T, url, body string, header map[string]string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range header {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestHTTPLimits(t *testing.T) {
	srv := serve(t, stand(t, big))

	res, body := post(t, srv.URL, listTools, nil)
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") || !strings.Contains(body, "search_employees") {
		t.Fatalf("a plain POST without a session: %d %s %s", res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	if res.Header.Get("Mcp-Session-Id") != "" {
		t.Fatal("the server keeps no sessions")
	}

	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodPut} {
		req, _ := http.NewRequest(method, srv.URL, nil)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want 405", method, r.StatusCode)
		}
	}

	if r, _ := post(t, srv.URL, listTools, map[string]string{"Origin": "https://evil.example"}); r.StatusCode != http.StatusForbidden {
		t.Errorf("a browser page of another origin: %d, want 403", r.StatusCode)
	}
	if r, _ := post(t, srv.URL, listTools, map[string]string{"Sec-Fetch-Site": "cross-site"}); r.StatusCode != http.StatusForbidden {
		t.Errorf("a cross-site browser request: %d, want 403", r.StatusCode)
	}
	// The deploy note rests on this: the SDK refuses a request that reaches a loopback listener under another Host name
	// (DNS rebinding), so the proxy in front must not pass the public Host on.
	if r, _ := post(t, srv.URL, listTools, map[string]string{"Host": "hr.harnsy.dev"}); r.StatusCode != http.StatusForbidden {
		t.Errorf("a loopback request under a public Host: %d, want 403", r.StatusCode)
	}

	huge := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_employees","arguments":{"query":"` +
		strings.Repeat("a", mcpserver.MaxRequest) + `"}}}`
	if r, _ := post(t, srv.URL, huge, nil); r.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("a body over %d bytes: %d, want 413", mcpserver.MaxRequest, r.StatusCode)
	}

	call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_employee","arguments":{"id":"harnsy/big"}}}`
	if r, body := post(t, srv.URL, call, nil); r.StatusCode != 200 || len(body) > mcpserver.MaxResponse || !strings.Contains(body, "big/AGENT.md") {
		t.Errorf("the largest answer: %d, %d bytes, at most %d", r.StatusCode, len(body), mcpserver.MaxResponse)
	}
}

// TestBatchIsRefused: a JSON array of calls is refused before the SDK reads it, whatever protocol version the request
// names. The older versions would answer every call of the array in one response: a small request, a huge answer.
func TestBatchIsRefused(t *testing.T) {
	srv := serve(t, stand(t))
	batch := "[" + listTools + "," + strings.Replace(listTools, `"id":1`, `"id":2`, 1) + "]"
	for _, version := range []string{"", "2025-03-26", "2025-06-18", "2025-11-25"} {
		for _, body := range []string{batch, " \r\n\t" + batch} {
			header := map[string]string{}
			if version != "" {
				header["MCP-Protocol-Version"] = version
			}
			res, answer := post(t, srv.URL, body, header)
			if res.StatusCode != http.StatusBadRequest || strings.Contains(answer, "search_employees") {
				t.Errorf("a batch of two calls, protocol %q: %d %.80q, want 400 and no answer to a call", version, res.StatusCode, answer)
			}
		}
	}
	if res, answer := post(t, srv.URL, "\n "+listTools, nil); res.StatusCode != http.StatusOK || !strings.Contains(answer, "search_employees") {
		t.Errorf("one call after white space: %d %.80q, want 200", res.StatusCode, answer)
	}
}

type nothing struct{}

func (nothing) Release() *catalog.Release { return nil }

func TestNoReleaseIs503(t *testing.T) {
	srv := serve(t, nothing{})
	res, _ := post(t, srv.URL, listTools, nil)
	if res.StatusCode != http.StatusServiceUnavailable || res.Header.Get("Retry-After") == "" {
		t.Fatalf("no verified release: %d (Retry-After %q), want 503", res.StatusCode, res.Header.Get("Retry-After"))
	}
}
