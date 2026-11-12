// Package licences_test keeps THIRD_PARTY_LICENSES in step with go.mod: a built binary carries these libraries, and
// their licences ask that the notice and the licence text travel with every copy.
package licences_test

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite THIRD_PARTY_LICENSES from the module cache (needs the modules downloaded)")

const (
	root    = "../.."
	file    = "THIRD_PARTY_LICENSES"
	stdlib  = "The Go standard library"
	hermes  = "Hermes Agent v0.23.1 (the patterns in internal/hermesscan are copied from it)"
	heading = "\n================================================================================\n"
	intro   = "Libraries contained in a built binary of hr-mcp, each with its version and the full text of its licence.\n" +
		"Generated from go.mod: go test ./internal/licences -update\n"
)

var required = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-z0-9.\-]+\.[a-z]+/\S+)\s+(v\S+)`)

// modules reads the libraries go.mod requires, as «path version», sorted.
func modules(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range required.FindAllStringSubmatch(string(b), -1) {
		out = append(out, m[1]+" "+m[2])
	}
	if len(out) == 0 {
		t.Fatal("go.mod requires nothing: the pattern is wrong")
	}
	sort.Strings(out)
	return out
}

// TestThirdPartyLicensesMatchGoMod: the file has one section per required module at its exact version, one for the
// standard library, one for the work a part of the code is copied from, and nothing else; no section is empty.
func TestThirdPartyLicensesMatchGoMod(t *testing.T) {
	want := append(modules(t), stdlib, hermes)
	if *update {
		write(t, want)
	}
	b, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	parts := strings.Split(string(b), heading)
	if parts[0] != intro {
		t.Errorf("the file must start with its two intro lines, starts with %q", parts[0])
	}
	if len(parts)%2 != 1 {
		t.Fatalf("%d parts between the rules: every section is a name between two rules, then its text", len(parts))
	}
	var got []string
	for i := 1; i < len(parts); i += 2 { // a name between two rules, then the licence text
		name, text := parts[i], parts[i+1]
		got = append(got, name)
		if len(strings.TrimSpace(text)) < 500 || !strings.Contains(strings.ToLower(text), "copyright") {
			t.Errorf("%s: the section has no licence text with a copyright line", name)
		}
	}
	if g, w := strings.Join(got, "\n"), strings.Join(want, "\n"); g != w {
		t.Errorf("sections differ from go.mod (run with -update and read the diff):\n got:\n%s\n want:\n%s", g, w)
	}
}

// write regenerates the file: each module's LICENSE from the module cache, the standard library's from GOROOT,
// Hermes Agent's from the folder of the code copied from it.
func write(t *testing.T, sections []string) {
	t.Helper()
	var b bytes.Buffer
	b.WriteString(intro)
	for _, s := range sections {
		dir := runtime.GOROOT()
		if s == hermes {
			dir = filepath.Join(root, "internal", "hermesscan") // its licence is kept beside the copied patterns
		} else if s != stdlib {
			path, _, _ := strings.Cut(s, " ")
			out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", path).Output()
			if err != nil || strings.TrimSpace(string(out)) == "" {
				t.Fatalf("%s is not in the module cache (go mod download): %v", s, err)
			}
			dir = strings.TrimSpace(string(out))
		}
		text, err := os.ReadFile(filepath.Join(dir, "LICENSE"))
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString(heading + s + heading + "\n" + strings.TrimSpace(string(text)) + "\n")
	}
	if err := os.WriteFile(filepath.Join(root, file), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
