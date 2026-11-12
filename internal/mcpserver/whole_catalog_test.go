package mcpserver_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
	"github.com/harnesy/hr-mcp/internal/hermesscan"
	"github.com/harnesy/hr-mcp/internal/mcpserver"
	"github.com/harnesy/hr-mcp/internal/render"
)

// The real catalog is not in this repository (it holds code only). These tests read a checkout of the catalog
// repository named by HR_CATALOG (its employees/ folder), pack it with a test key and skip without it.

func realCatalog(t *testing.T) string {
	dir := os.Getenv("HR_CATALOG")
	if dir == "" {
		t.Skip("HR_CATALOG is not set: the path of a catalog checkout's employees/ folder")
	}
	return dir
}

// TestWholeCatalog: every employee, for every target, fits the answer limit, and every file comes back whole.
func TestWholeCatalog(t *testing.T) {
	dir := realCatalog(t)
	k, root := catalogtest.New(t), t.TempDir()
	k.Release(t, filepath.Join(root, "current"), k.Catalog(t, dir), nil)
	s := catalog.NewStore(root, k.Key, catalogtest.Host, func() time.Time { return k.Now })
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	cs := connect(t, s)
	rel := s.Release()
	largest, files, fetched, refused := 0, 0, 0, 0
	for _, e := range rel.Employees {
		for _, target := range render.Targets() {
			if target == render.Hermes && hermesBlocks(t, e) {
				refused++ // not handed out for Hermes; TestWholeCatalogFitsHermes names it
				continue
			}
			var out mcpserver.EmployeeOut
			res := call(t, cs, "get_employee", map[string]any{"id": e.Entry.ID, "target": string(target)}, &out)
			n := len(text(res))
			if largest = max(largest, n); n > mcpserver.MaxAnswer {
				t.Errorf("%s for %s: the answer is %d bytes, at most %d", e.Entry.ID, target, n, mcpserver.MaxAnswer)
			}
			skills := map[string]bool{}
			for _, f := range out.Files {
				files++
				content := f.Content
				if content == nil {
					fetched++
					text, _ := inParts(t, cs, e.Entry.ID, e.Entry.Version, string(target), f)
					content = &text
				}
				if catalogtest.Sum([]byte(*content)) != f.SHA256 {
					t.Errorf("%s for %s: %s does not match its sha256", e.Entry.ID, target, f.Path)
				}
				skills[f.SHA256] = true
			}
			for name, sum := range e.Manifest.Files {
				if strings.HasPrefix(name, "skills/") && !skills[sum] {
					t.Errorf("%s for %s: %s is not in the tree byte for byte", e.Entry.ID, target, name)
				}
			}
		}
	}
	t.Logf("%d employees × %d targets: %d files, %d fetched one by one, largest answer %d bytes; %d not handed out for Hermes", len(rel.Employees), len(render.Targets()), files, fetched, largest, refused)
}

// hermesBlocks says whether Hermes' scanner would block the employee's SOUL.md.
func hermesBlocks(t *testing.T, e *catalog.Employee) bool {
	tree, err := render.Render(e, render.Hermes)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range tree.Files {
		if f.Path == tree.Persona {
			return len(hermesscan.Scan(string(f.Content))) > 0
		}
	}
	return false
}

// TestWriteStand writes a catalog root for a local stand of the real binary: HR_STAND_ROOT/current with the catalog
// of HR_CATALOG signed by a test key, and HR_STAND_ROOT/pub.txt with that key's public half (kid t1).
func TestWriteStand(t *testing.T) {
	root := os.Getenv("HR_STAND_ROOT")
	if root == "" {
		t.Skip("HR_STAND_ROOT is not set")
	}
	k := catalogtest.New(t)
	k.Now = time.Now().UTC()
	k.Release(t, filepath.Join(root, "releases", "1"), k.Catalog(t, realCatalog(t)), nil)
	os.Remove(filepath.Join(root, "current"))
	if err := os.Symlink(filepath.Join("releases", "1"), filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	catalogtest.Write(t, filepath.Join(root, "pub.txt"), []byte(base64.StdEncoding.EncodeToString(k.Key.Pub)+"\n"))
}

// TestWholeCatalogFitsOpenClaw: for every employee the files OpenClaw loads at the start of a session (AGENTS.md,
// SOUL.md, IDENTITY.md) stay within its limits, per file and together, so none is cut. A character is a UTF-16 unit,
// as OpenClaw counts. Skill descriptions longer than OpenClaw's advice (one line, under 160 characters) are counted,
// not refused: a skill file travels byte for byte.
func TestWholeCatalogFitsOpenClaw(t *testing.T) {
	dir := realCatalog(t)
	k, root := catalogtest.New(t), t.TempDir()
	k.Release(t, root, k.Catalog(t, dir), nil)
	rel, err := catalog.Load(root, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	largestFile, largestTotal, longDescriptions, skills := 0, 0, 0, 0
	for _, e := range rel.Employees {
		tree, err := render.Render(e, render.OpenClaw)
		if err != nil {
			t.Fatal(err)
		}
		total := 0
		for _, f := range tree.Files {
			if name := f.Path[strings.LastIndex(f.Path, "/")+1:]; strings.Count(f.Path, "/") != 1 || (name != "AGENTS.md" && name != "SOUL.md" && name != "IDENTITY.md") {
				continue
			}
			n := len(utf16.Encode([]rune(string(f.Content))))
			total += n
			if largestFile = max(largestFile, n); n > render.OpenClawFileMax {
				t.Errorf("%s: %s is %d characters, OpenClaw loads at most %d of a file", e.Entry.ID, f.Path, n, render.OpenClawFileMax)
			}
		}
		if largestTotal = max(largestTotal, total); total > render.OpenClawTotalMax {
			t.Errorf("%s: the loaded files are %d characters together, at most %d", e.Entry.ID, total, render.OpenClawTotalMax)
		}
		for _, s := range e.Doc.Skills {
			if skills++; len([]rune(s.Description)) >= 160 || strings.Contains(strings.TrimSpace(s.Description), "\n") {
				longDescriptions++
			}
		}
	}
	t.Logf("%d employees for openclaw: largest loaded file %d characters (limit %d), largest total %d (limit %d); %d of %d skill descriptions are longer than OpenClaw's advice of 160 characters on one line",
		len(rel.Employees), largestFile, render.OpenClawFileMax, largestTotal, render.OpenClawTotalMax, longDescriptions, skills)
}

// TestWholeCatalogFitsHermes: every employee rendered for Hermes Agent would load there. Its SOUL.md trips none of
// the patterns Hermes blocks a distribution's SOUL.md on (a hit is a text for the profile's author to change, never
// one to force through) and stays under the smallest cap Hermes cuts a context file at; its name is a profile name
// Hermes takes; its skills have names and descriptions Hermes takes, and none is called like the casebook skill.
func TestWholeCatalogFitsHermes(t *testing.T) {
	dir := realCatalog(t)
	k, root := catalogtest.New(t), t.TempDir()
	k.Release(t, root, k.Catalog(t, dir), nil)
	rel, err := catalog.Load(root, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	profile, skill := regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`), regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	reserved := map[string]bool{"hermes": true, "default": true, "test": true, "tmp": true, "root": true, "sudo": true}
	largest, blocked, long := 0, 0, 0
	for _, e := range rel.Employees {
		tree, err := render.Render(e, render.Hermes)
		if err != nil {
			t.Fatal(err)
		}
		if !profile.MatchString(tree.Name) || reserved[tree.Name] {
			t.Errorf("%s: %q is no profile name Hermes takes", e.Entry.ID, tree.Name)
		}
		seen := map[string]bool{}
		for _, f := range tree.Files {
			if seen[f.Path] {
				t.Errorf("%s: %s is in the tree twice", e.Entry.ID, f.Path)
			}
			if seen[f.Path] = true; f.Path != tree.Persona {
				continue
			}
			n := len([]rune(string(f.Content)))
			if largest = max(largest, n); n > render.HermesSoulMax {
				long++
				t.Logf("%s: SOUL.md is %d characters; a model with a small context window gets it cut above %d", e.Entry.ID, n, render.HermesSoulMax)
			}
			if hits := hermesscan.Scan(string(f.Content)); len(hits) > 0 {
				blocked++
				t.Errorf("%s: Hermes would block this SOUL.md: %s", e.Entry.ID, strings.Join(hits, ", "))
			}
		}
		for _, s := range e.Doc.Skills {
			if !skill.MatchString(s.Name) || s.Name == "persona-casebook" || len([]rune(s.Description)) > 1024 || strings.TrimSpace(s.Description) == "" {
				t.Errorf("%s: skill %q: Hermes wants a name of lowercase letters, digits, dots and dashes and a description of at most 1024 characters", e.Entry.ID, s.Name)
			}
		}
	}
	t.Logf("%d employees for hermes: %d would be blocked by the scanner (%d patterns); largest SOUL.md %d characters, %d above %d",
		len(rel.Employees), blocked, hermesscan.Patterns(), largest, long, render.HermesSoulMax)
}

// TestWriteTrees writes every employee of HR_CATALOG rendered for every target under HR_TREES_OUT, as
// <target>/<publisher>/<name>/<the tree>: for a stand of another agent system, and to run that system's own checks
// (Hermes' scanner, OpenClaw's loader) over what it would be given.
func TestWriteTrees(t *testing.T) {
	out := os.Getenv("HR_TREES_OUT")
	if out == "" {
		t.Skip("HR_TREES_OUT is not set")
	}
	k, root := catalogtest.New(t), t.TempDir()
	k.Release(t, root, k.Catalog(t, realCatalog(t)), nil)
	rel, err := catalog.Load(root, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range rel.Employees {
		for _, target := range render.Targets() {
			tree, err := render.Render(e, target)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range tree.Files {
				catalogtest.Write(t, filepath.Join(out, string(target), filepath.FromSlash(e.Entry.ID), filepath.FromSlash(f.Path)), f.Content)
			}
		}
	}
}

// TestWholeCatalogSearchIsBounded: on the whole catalog a 64 KB query is refused at once, and a query of the most
// words that are taken (16, all real ones) is answered in a few milliseconds: the most one request can cost.
func TestWholeCatalogSearchIsBounded(t *testing.T) {
	k, root := catalogtest.New(t), t.TempDir()
	k.Release(t, root, k.Catalog(t, realCatalog(t)), nil)
	rel, err := catalog.Load(root, k.Key, catalogtest.Host, k.Now)
	if err != nil {
		t.Fatal(err)
	}
	fastest := func(q string) (best time.Duration, err error) {
		for i := range 5 {
			start := time.Now()
			_, err = rel.Search(catalog.Query{Text: q})
			if took := time.Since(start); i == 0 || took < best {
				best = took
			}
		}
		return best, err
	}
	long := strings.Repeat("review contracts and invoices ", (64<<10)/30)
	refused, err := fastest(long)
	if err == nil || refused > 5*time.Millisecond {
		t.Errorf("a %d-byte query: err %v in %s, want a refusal within 5 ms", len(long), err, refused)
	}
	most := "accountant reviewer contracts support designer sales legal finance marketing product engineering operations content data manager writer"
	taken, err := fastest(most)
	if err != nil || taken > 50*time.Millisecond {
		t.Errorf("a query of 16 words (%d bytes): err %v in %s, want an answer within 50 ms", len(most), err, taken)
	}
	t.Logf("%d employees: a %d-byte query refused in %s; a query of 16 words answered in %s", len(rel.Employees), len(long), refused, taken)
}
