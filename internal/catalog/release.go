package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Load reads the release in dir (v1/feed.json, v1/feed.json.sig and the bundle files at their feed urls) and verifies
// all of it under key, for the catalog at host, as of now. Any fault refuses the whole release: nothing of a release
// that does not verify is served.
func Load(dir string, key Key, host string, now time.Time) (*Release, error) {
	fb, err := readLimited(filepath.Join(dir, "v1", "feed.json"), MaxFeed)
	if err != nil {
		return nil, err
	}
	sb, err := readLimited(filepath.Join(dir, "v1", "feed.json.sig"), MaxFeed)
	if err != nil {
		return nil, err
	}
	if err := verifyFeed(key, fb, sb); err != nil {
		return nil, err
	}
	r := &Release{Kid: key.Kid, byID: map[string]*Employee{}}
	if err := json.Unmarshal(fb, &r.Feed); err != nil {
		return nil, fmt.Errorf("feed.json: %w", err)
	}
	exp, err := time.Parse(time.RFC3339, r.Feed.Expires)
	if r.Feed.V != 1 || err != nil || !exp.After(now) {
		return nil, fmt.Errorf("feed.json: v %d, expires %q (now %s)", r.Feed.V, r.Feed.Expires, now.Format(time.RFC3339))
	}
	r.Expires = exp
	revoked := map[string]bool{}
	for _, v := range r.Feed.Revoked {
		revoked[v] = true
	}
	for _, e := range r.Feed.Bundles {
		if e.Kind != "employee" || revoked[e.ID+"@"+e.Version] {
			continue // bundle v1 reads employees only; a revoked version is never served
		}
		rel, ok := localPath(e.URL, host)
		if !ok {
			return nil, fmt.Errorf("%s: url not on %s", e.ID, host)
		}
		file, err := readLimited(filepath.Join(dir, filepath.FromSlash(rel)), MaxBundle)
		if err != nil {
			return nil, fmt.Errorf("%s@%s: %w", e.ID, e.Version, err)
		}
		emp, err := openBundle(key, host, e, file)
		if err != nil {
			return nil, fmt.Errorf("%s@%s: %w", e.ID, e.Version, err)
		}
		r.Employees = append(r.Employees, emp)
		r.byID[e.ID] = emp
	}
	return r, nil
}

// localPath turns a feed url into the file's path inside the release; false when the url is not a clean path on the
// catalog's own https host.
func localPath(url, host string) (string, bool) {
	rel, ok := strings.CutPrefix(url, "https://"+host+"/")
	if !ok || rel == "" || strings.ContainsAny(rel, "\\?#\x00") || path.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

// readLimited reads a file of at most max bytes.
func readLimited(file string, max int64) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("%s: more than %d bytes", filepath.Base(file), max)
	}
	return b, nil
}
