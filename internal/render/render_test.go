package render_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
	"github.com/harnesy/hr-mcp/internal/render"
)

var update = flag.Bool("update", false, "rewrite the golden trees under testdata/golden")

// staff is the release of the made-up employees under testdata/employees.
func staff(t *testing.T) *catalog.Release {
	t.Helper()
	k, dir := catalogtest.New(t), t.TempDir()
	k.Release(t, dir, k.Catalog(t, filepath.Join("testdata", "employees")), nil)
	r, err := catalog.Load(dir, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Employees) != 4 {
		t.Fatalf("%d test employees, want 4", len(r.Employees))
	}
	return r
}

func mustRender(t *testing.T, e *catalog.Employee, target render.Target) render.Tree {
	t.Helper()
	tree, err := render.Render(e, target)
	if err != nil {
		t.Fatalf("%s for %s: %v", e.Entry.ID, target, err)
	}
	return tree
}

// TestGolden: every test employee renders, for every target, to exactly the files under testdata/golden.
func TestGolden(t *testing.T) {
	r := staff(t)
	for _, target := range render.Targets() {
		for _, e := range r.Employees {
			name := string(target) + "/" + strings.ReplaceAll(e.Entry.ID, "/", "-")
			t.Run(name, func(t *testing.T) {
				tree := mustRender(t, e, target)
				dir := filepath.Join("testdata", "golden", filepath.FromSlash(name))
				if *update {
					if err := os.RemoveAll(dir); err != nil {
						t.Fatal(err)
					}
					for _, f := range tree.Files {
						catalogtest.Write(t, filepath.Join(dir, filepath.FromSlash(f.Path)), f.Content)
					}
				}
				want := catalogtest.Source(t, dirWithMarker(t, dir))
				delete(want, "bundle.json")
				got := map[string][]byte{}
				for _, f := range tree.Files {
					if _, twice := got[f.Path]; twice {
						t.Errorf("%s is in the tree twice", f.Path)
					}
					got[f.Path] = f.Content
				}
				if g, w := keys(got), keys(want); g != w {
					t.Fatalf("files:\n got  %s\n want %s", g, w)
				}
				for p, b := range got {
					if !bytes.Equal(b, want[p]) {
						t.Errorf("%s differs from the golden file (run with -update and read the diff):\n%s", p, b)
					}
				}
			})
		}
	}
}

// dirWithMarker lets catalogtest.Source read a golden folder: it asks for a bundle.json, which a rendered tree never has.
func dirWithMarker(t *testing.T, dir string) string {
	t.Helper()
	tmp := t.TempDir()
	if err := os.CopyFS(tmp, os.DirFS(dir)); err != nil {
		t.Fatalf("no golden tree (run with -update): %v", err)
	}
	catalogtest.Write(t, filepath.Join(tmp, "bundle.json"), []byte("{}"))
	return tmp
}

func keys(m map[string][]byte) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, " ")
}

// TestSkillFilesTravelByteForByte: every file of every skill is in the tree unchanged, with its licence beside it.
func TestSkillFilesTravelByteForByte(t *testing.T) {
	r := staff(t)
	for _, target := range render.Targets() {
		for _, e := range r.Employees {
			tree := mustRender(t, e, target)
			for name, b := range e.Files {
				rest, ok := strings.CutPrefix(name, "skills/")
				if !ok {
					continue
				}
				found := false
				for _, f := range tree.Files {
					if strings.HasSuffix(f.Path, "/"+rest) && bytes.Equal(f.Content, b) {
						found = true
					}
				}
				if !found {
					t.Errorf("%s for %s: %s is missing or changed", e.Entry.ID, target, name)
				}
			}
		}
	}
}

// TestTreeShape: what every tree has, whatever the target.
func TestTreeShape(t *testing.T) {
	r := staff(t)
	for _, target := range render.Targets() {
		for _, e := range r.Employees {
			tree := mustRender(t, e, target)
			files := map[string]string{}
			for i, f := range tree.Files {
				files[f.Path] = string(f.Content)
				if i > 0 && tree.Files[i-1].Path >= f.Path {
					t.Errorf("%s for %s: files are not in path order at %s", e.Entry.ID, target, f.Path)
				}
				if strings.Contains(f.Path, "..") || strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "avatar") {
					t.Errorf("%s for %s: path %q", e.Entry.ID, target, f.Path)
				}
			}
			persona, ok := files[tree.Persona]
			if !ok {
				t.Fatalf("%s for %s: persona file %q is not in the tree", e.Entry.ID, target, tree.Persona)
			}
			job, ok := files[tree.Job]
			if !ok || !strings.Contains(job, e.Doc.Role.Prompt) {
				t.Fatalf("%s for %s: the job file %q must be in the tree and hold the prompt unchanged", e.Entry.ID, target, tree.Job)
			}
			for path, text := range map[string]string{tree.Persona: persona, tree.Job: job} {
				if !strings.Contains(text, "AI agent") {
					t.Errorf("%s for %s: %s must say it is an AI agent", e.Entry.ID, target, path)
				}
				if last := lastLine(text); !strings.Contains(last, e.Entry.CVURL) || !strings.Contains(last, e.Entry.Title+" "+e.Entry.Version) {
					t.Errorf("%s for %s: %s must end with the credit line, ends with %q", e.Entry.ID, target, path, last)
				}
			}
			notice, ok := files[tree.NoticeFile]
			if !ok || !strings.Contains(notice, tree.Notice) {
				t.Fatalf("%s for %s: notice file %q must hold the notice", e.Entry.ID, target, tree.NoticeFile)
			}
			all := strings.Join(sortedValues(files), "\n")
			for _, sealed := range []string{"A made-up desk manual", "nodes", "[S1]"} {
				if strings.Contains(all, sealed) {
					t.Errorf("%s for %s: %q must not leave the bundle (sources and harnsy Max needs stay sealed)", e.Entry.ID, target, sealed)
				}
			}
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

func sortedValues(m map[string]string) []string {
	var vs []string
	for _, v := range m {
		vs = append(vs, v)
	}
	sort.Strings(vs)
	return vs
}

func TestRenderRefusesAnUnknownTarget(t *testing.T) {
	r := staff(t)
	if _, err := render.Render(r.Employees[0], "windows-95"); err == nil {
		t.Fatal("an unknown target must be refused")
	}
}

// TestClaudeCodeKeepsItsOwnFolderSmall: Claude Code asks a person before every write under .claude/ and refuses some
// of them outright in unattended runs. Only what Claude Code reads there goes there: the subagent and its skills. The
// notice and the casebook are ordinary project files, so the credit is never the file that could not be written.
func TestClaudeCodeKeepsItsOwnFolderSmall(t *testing.T) {
	r := staff(t)
	for _, e := range r.Employees {
		tree := mustRender(t, e, render.ClaudeCode)
		_, name, _ := strings.Cut(e.Entry.ID, "/")
		for _, f := range tree.Files {
			inAgents := f.Path == ".claude/agents/"+name+".md"
			inSkills := strings.HasPrefix(f.Path, ".claude/skills/")
			inHome := strings.HasPrefix(f.Path, "harnsy-hr/"+name+"/")
			if !inAgents && !inSkills && !inHome {
				t.Errorf("%s: %q is neither the subagent, a skill, nor under harnsy-hr/%s/", e.Entry.ID, f.Path, name)
			}
		}
		if want := "harnsy-hr/" + name + "/NOTICE.md"; tree.NoticeFile != want {
			t.Errorf("%s: the notice is at %q, want %q", e.Entry.ID, tree.NoticeFile, want)
		}
	}
}
