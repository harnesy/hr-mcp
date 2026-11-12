package mcpserver_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
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
	largest, files, fetched := 0, 0, 0
	for _, e := range rel.Employees {
		for _, target := range render.Targets() {
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
					var whole strings.Builder
					for offset := 0; ; {
						var part mcpserver.FileOut
						call(t, cs, "get_employee_file", map[string]any{"id": e.Entry.ID, "version": e.Entry.Version, "target": string(target), "path": f.Path, "offset": offset}, &part)
						whole.WriteString(*part.Content)
						if offset = part.NextOffset; offset == 0 {
							break
						}
					}
					text := whole.String()
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
	t.Logf("%d employees × %d targets: %d files, %d fetched one by one, largest answer %d bytes", len(rel.Employees), len(render.Targets()), files, fetched, largest)
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
