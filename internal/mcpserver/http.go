package mcpserver

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Limits of the HTTP endpoint.
const (
	MaxRequest  = 64 << 10         // a request body
	MaxResponse = 500 << 10        // an answer: a larger one is not sent; the tools keep theirs far below it
	Timeout     = 10 * time.Second // one request
)

// Handler serves the MCP endpoint: streamable HTTP without sessions, plain JSON answers. POST only; a browser page of
// another origin is refused; a JSON-RPC batch is refused; an answer above MaxResponse is not sent; with no verified
// release it answers 503.
//
// It is meant to listen on a loopback address behind a proxy. The SDK refuses a loopback request whose Host is not a
// loopback name (DNS rebinding), so the proxy must not pass the public Host header on.
func Handler(src Source, o *Options) http.Handler {
	m := New(src, o)
	h := http.Handler(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: MaxRequest}))
	h = http.NewCrossOriginProtection().Handler(refuseBatch(h))
	h = http.TimeoutHandler(capped(h), Timeout, "the request took too long")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method != http.MethodPost:
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
		case src.Release() == nil:
			w.Header().Set("Retry-After", "30")
			http.Error(w, "the catalog is not available right now", http.StatusServiceUnavailable)
		default:
			h.ServeHTTP(w, r)
		}
	})
}
