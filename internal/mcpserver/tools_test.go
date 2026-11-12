package mcpserver_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
	"github.com/harnesy/hr-mcp/internal/mcpserver"
)

// stand is a catalog root with the made-up employees of the renderer's tests, and a store that has loaded it.
func stand(t *testing.T, extra ...func(*testing.T, *catalogtest.Kit) catalogtest.Packed) *catalog.Store {
	t.Helper()
	k, root := catalogtest.New(t), t.TempDir()
	packs := k.Catalog(t, filepath.Join("..", "render", "testdata", "employees"))
	for _, x := range extra {
		packs = append(packs, x(t, k))
	}
	k.Release(t, filepath.Join(root, "current"), packs, nil)
	s := catalog.NewStore(root, k.Key, catalogtest.Host, func() time.Time { return k.Now })
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	return s
}

// connect runs the server in memory and returns a client session on it.
func connect(t *testing.T, s *catalog.Store) *mcp.ClientSession {
	t.Helper()
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpserver.New(s, nil).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// call calls a tool and decodes its structured answer into out; it returns the result for the error cases.
func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if out != nil {
		if res.IsError {
			t.Fatalf("%s: tool error: %s", tool, text(res))
		}
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: %v in %s", tool, err, b)
		}
		var same any
		if json.Unmarshal([]byte(text(res)), &same) != nil {
			t.Fatalf("%s: the text content must be the same JSON, got %q", tool, text(res))
		}
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestToolsAreListedReadOnly(t *testing.T) {
	s := stand(t)
	res, err := connect(t, s).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		a := tool.Annotations
		if a == nil || !a.ReadOnlyHint || !a.IdempotentHint || a.DestructiveHint == nil || *a.DestructiveHint {
			t.Errorf("%s: want readOnly, idempotent, not destructive; got %+v", tool.Name, a)
		}
		if tool.Title == "" || tool.Description == "" || tool.OutputSchema == nil {
			t.Errorf("%s: want a title, a description and an output schema", tool.Name)
		}
	}
	if got := strings.Join(names, " "); got != "get_employee get_employee_file install_hint search_employees" {
		t.Fatalf("tools: %s", got)
	}
}

func TestSearchEmployees(t *testing.T) {
	s := stand(t)
	cs := connect(t, s)
	var out mcpserver.SearchOut
	call(t, cs, "search_employees", map[string]any{"query": "audit shelves"}, &out)
	if len(out.Employees) == 0 || out.Employees[0].ID != "harnsy/sample-skills" {
		t.Fatalf("got %+v", out.Employees)
	}
	c := out.Employees[0]
	if c.Version != "1.2.0" || c.Title != "Sample archivist" || c.Persona.Name != "Otis" || c.Department != "operations" ||
		c.License != "CC-BY-4.0" || c.CVURL != "https://hr.harnsy.dev/e/harnsy/sample-skills" || len(c.Skills) != 1 || c.Summary == "" {
		t.Fatalf("card %+v", c)
	}

	call(t, cs, "search_employees", map[string]any{"limit": 3}, &out)
	if len(out.Employees) != 3 || out.Total != 4 || out.NextCursor == "" {
		t.Fatalf("first page: %d of %d, next %q", len(out.Employees), out.Total, out.NextCursor)
	}
	var last mcpserver.SearchOut
	call(t, cs, "search_employees", map[string]any{"limit": 3, "cursor": out.NextCursor}, &last)
	if len(last.Employees) != 1 || last.NextCursor != "" {
		t.Fatalf("second page: %d, next %q", len(last.Employees), last.NextCursor)
	}
	if res := call(t, cs, "search_employees", map[string]any{"cursor": "nonsense"}, nil); !res.IsError {
		t.Fatal("a bad cursor must be a tool error")
	}
}

// TestSearchRefusesALongQuery: a query no person types (64 KB of words) is a plain tool error at once; it is not cut and
// searched, and the error does not repeat it.
func TestSearchRefusesALongQuery(t *testing.T) {
	cs := connect(t, stand(t))
	var long strings.Builder
	for i := 0; long.Len() < 64<<10; i++ {
		long.WriteString("w" + strconv.Itoa(i) + "x ")
	}
	start := time.Now()
	res := call(t, cs, "search_employees", map[string]any{"query": long.String()}, nil)
	if took := time.Since(start); !res.IsError || took > 250*time.Millisecond {
		t.Fatalf("a 64 KB query: tool error %v in %s, want a refusal within 250 ms", res.IsError, took)
	}
	if msg := text(res); !strings.Contains(msg, "query") || strings.Contains(msg, "w1x") || len(msg) > 300 {
		t.Fatalf("the refusal: %.400q", msg)
	}
	for _, args := range []map[string]any{{"department": strings.Repeat("d", 65)}, {"cursor": "7:" + strings.Repeat("0", 64)}} {
		if res := call(t, cs, "search_employees", args, nil); !res.IsError {
			t.Fatalf("%.40v must be a tool error", args)
		}
	}
}
