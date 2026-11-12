package mcpserver_test

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harnesy/hr-mcp/internal/mcpserver"
)

// connectLogged is connect with the server's log kept.
func connectLogged(t *testing.T) (*mcp.ClientSession, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpserver.New(stand(t), &mcpserver.Options{Logger: slog.New(slog.NewJSONHandler(&log, nil))}).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, &log
}

// TestABadValueIsNotRepeated: what a caller types in place of an id (or a version, a target, a path) does not come
// back in the error and does not go into the log whole: an id that is not one is refused before any lookup.
func TestABadValueIsNotRepeated(t *testing.T) {
	cs, log := connectLogged(t)
	junk := strings.Repeat("A", 60000)
	for _, id := range []string{"harnsy/" + junk, junk, "nobody", "harnsy/Sample-plain", "harnsy/sample-plain\n", "harnsy/../x", ""} {
		for tool, args := range map[string]map[string]any{
			"get_employee":      {"id": id},
			"get_employee_file": {"id": id, "version": "1.0.0", "target": "generic", "path": "x"},
			"install_hint":      {"id": id, "client": "codex"},
		} {
			if tool == "install_hint" && id == "" {
				continue // no id there means the steps for any employee
			}
			res := call(t, cs, tool, args, nil)
			if msg := text(res); !res.IsError || !strings.Contains(msg, "not an employee id") || strings.Contains(msg, "AAAA") || len(msg) > 300 {
				t.Errorf("%s with the id %.30q: tool error %v, %.400q", tool, id, res.IsError, msg)
			}
		}
	}
	if res := call(t, cs, "get_employee", map[string]any{"id": "harnsy/nobody"}, nil); !res.IsError || !strings.Contains(text(res), `no employee "harnsy/nobody"`) {
		t.Errorf("an id of the grammar that the catalog lacks is named: %q", text(res))
	}
	for name, args := range map[string]map[string]any{
		"version": {"id": "harnsy/sample-plain", "version": junk},
		"target":  {"id": "harnsy/sample-plain", "target": junk},
		"path":    {"id": "harnsy/sample-plain", "version": "1.0.0", "target": "generic", "path": junk},
	} {
		tool := "get_employee"
		if name == "path" {
			tool = "get_employee_file"
		}
		res := call(t, cs, tool, args, nil)
		if msg := text(res); !res.IsError || !strings.Contains(msg, name) || strings.Contains(msg, strings.Repeat("A", 200)) || len(msg) > 500 {
			t.Errorf("a long %s: tool error %v, %d bytes: %.300q", name, res.IsError, len(msg), msg)
		}
	}
	lines := 0
	for sc := bufio.NewScanner(log); sc.Scan(); lines++ {
		if line := sc.Text(); len(line) > 800 || strings.Contains(line, strings.Repeat("A", 131)) {
			t.Errorf("a log line of %d bytes keeps more than 130 characters of what the caller typed: %.200q…", len(line), line)
		}
	}
	if lines < 20 {
		t.Fatalf("%d log lines: every tool call is logged", lines)
	}
}
