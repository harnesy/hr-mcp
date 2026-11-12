package catalogtest

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Catalog packs the newest version of every employee under dir, laid out as the catalog repository's employees/ folder
// (<publisher>/<name>/<version>/), in the order of their ids: what the feed lists.
func (k *Kit) Catalog(t testing.TB, dir string) []Packed {
	t.Helper()
	versions, err := filepath.Glob(filepath.Join(dir, "*", "*", "*", "bundle.json"))
	if err != nil || len(versions) == 0 {
		t.Fatalf("%s: no employees (%v)", dir, err)
	}
	newest := map[string]string{} // id → version
	for _, bj := range versions {
		rel, _ := filepath.Rel(dir, filepath.Dir(bj))
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if id := parts[0] + "/" + parts[1]; newest[id] == "" || semverLess(newest[id], parts[2]) {
			newest[id] = parts[2]
		}
	}
	ids := make([]string, 0, len(newest))
	for id := range newest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var packs []Packed
	for _, id := range ids {
		packs = append(packs, k.Pack(t, id, newest[id], Source(t, filepath.Join(dir, filepath.FromSlash(id), newest[id])), nil))
	}
	return packs
}

// semverLess compares two MAJOR.MINOR.PATCH versions by their numbers.
func semverLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return len(pa) < len(pb)
}

// Source reads a version folder in the catalog repository's layout (bundle.json, listing.json, docs/, skills/): every
// file by its path with forward slashes. The picture is left out: no test here needs it.
func Source(t testing.TB, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "avatar.") {
			return nil
		}
		b, err := os.ReadFile(p)
		files[rel] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if files["bundle.json"] == nil {
		t.Fatalf("%s: no bundle.json", dir)
	}
	return files
}
