// Package registry_test keeps server.json, the entry for the MCP Registry, within the registry's own schema
// (https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json) and in step with the server. What is
// published there is dedicated to the public domain and cannot be taken back, so the file holds short facts only.
package registry_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	file     = "../../server.json"
	schema   = "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json"
	endpoint = "https://hr.harnsy.dev/mcp" // the same as mcpserver's default endpoint and the README's
)

var (
	name    = regexp.MustCompile(`^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$`) // the schema's pattern
	semver  = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	allowed = map[string]bool{"$schema": true, "name": true, "title": true, "description": true, "version": true, "repository": true, "remotes": true}
)

type entry struct {
	Schema      string `json:"$schema"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Repository  struct {
		URL    string `json:"url"`
		Source string `json:"source"`
	} `json:"repository"`
	Remotes []map[string]any `json:"remotes"`
}

func TestServerJSONFitsTheRegistrySchema(t *testing.T) {
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var e entry
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	if e.Schema != schema {
		t.Errorf("$schema = %q, want %q", e.Schema, schema)
	}
	if e.Name != "dev.harnsy/hr" || !name.MatchString(e.Name) {
		t.Errorf("name = %q, want dev.harnsy/hr (reverse DNS of harnsy.dev, one slash)", e.Name)
	}
	for field, v := range map[string]string{"title": e.Title, "description": e.Description} {
		if n := utf8.RuneCountInString(v); n < 1 || n > 100 {
			t.Errorf("%s has %d characters, the schema allows 1 to 100", field, n)
		}
	}
	if !semver.MatchString(e.Version) {
		t.Errorf("version = %q: one exact version, never a range", e.Version)
	}
	if e.Repository.URL != "https://github.com/harnesy/hr-mcp" || e.Repository.Source != "github" {
		t.Errorf("repository = %+v", e.Repository)
	}
}

// TestServerJSONNamesOnlyTheRemoteEndpoint: the server is remote only; nothing to install, no header, no key.
func TestServerJSONNamesOnlyTheRemoteEndpoint(t *testing.T) {
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for k := range raw {
		if !allowed[k] {
			t.Errorf("field %q: the entry holds short facts only (it cannot be taken back once published)", k)
		}
	}
	var e entry
	_ = json.Unmarshal(b, &e)
	if len(e.Remotes) != 1 {
		t.Fatalf("remotes: %d, want one", len(e.Remotes))
	}
	r := e.Remotes[0]
	if len(r) != 2 || r["type"] != "streamable-http" || r["url"] != endpoint {
		t.Errorf("remote = %v, want only type streamable-http and url %s", r, endpoint)
	}
	if low := strings.ToLower(string(b)); strings.Contains(low, "token") || strings.Contains(low, "secret") {
		t.Error("the entry must not speak of tokens or secrets: the server asks for none")
	}
}
