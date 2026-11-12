package catalog

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

const signatureFile = "signatures/catalog.jws"

// unzip reads a .harnsy file within the format's limits: at most MaxZipFiles entries, MaxBundle bytes unpacked whatever
// the headers say, no name twice, no name that leaves the bundle.
func unzip(file []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		return nil, err
	}
	if len(zr.File) > MaxZipFiles {
		return nil, fmt.Errorf("%d zip entries, at most %d", len(zr.File), MaxZipFiles)
	}
	files, left := map[string][]byte{}, int64(MaxBundle)
	for _, f := range zr.File {
		if _, twice := files[f.Name]; twice {
			return nil, fmt.Errorf("%s is in the zip twice", f.Name)
		}
		if !safeName(f.Name) || !f.Mode().IsRegular() {
			return nil, fmt.Errorf("zip entry %q: not a plain file inside the bundle", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, left+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if left -= int64(len(b)); left < 0 {
			return nil, fmt.Errorf("unpacks to more than %d bytes", MaxBundle)
		}
		files[f.Name] = b
	}
	return files, nil
}

// safeName reports whether a zip entry's name is a clean relative path with forward slashes.
func safeName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\\x00") && !path.IsAbs(name) && path.Clean(name) == name &&
		name != ".." && !strings.HasPrefix(name, "../")
}

// openBundle checks a .harnsy file against its feed entry (size, SHA-256, the manifest's names and file hashes, the
// catalog's signature over the manifest) and returns the employee. Only files the manifest lists are kept.
func openBundle(key Key, host string, e Entry, file []byte) (*Employee, error) {
	if int64(len(file)) != e.Size || hexSum(file) != e.SHA256 {
		return nil, fmt.Errorf("size or sha256 differs from the feed")
	}
	all, err := unzip(file)
	if err != nil {
		return nil, err
	}
	mb, ok := all["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("no manifest.json")
	}
	emp := &Employee{Entry: e, ManifestSHA256: hexSum(mb), Files: map[string][]byte{}, index: indexOf(e)}
	m := &emp.Manifest
	if err := json.Unmarshal(mb, m); err != nil {
		return nil, fmt.Errorf("manifest.json: %w", err)
	}
	if m.Format != "harnsy-bundle" || m.FormatVersion != 1 || m.ID != e.ID || m.Version != e.Version || m.Kind != "employee" {
		return nil, fmt.Errorf("manifest names %s@%s (%s v%d, %s)", m.ID, m.Version, m.Format, m.FormatVersion, m.Kind)
	}
	for name, sum := range m.Files {
		b, ok := all[name]
		if !ok || strings.HasPrefix(name, "signatures/") || hexSum(b) != sum {
			return nil, fmt.Errorf("%s missing or its sha256 differs", name)
		}
		emp.Files[name] = b
	}
	c, err := verifyJWS(key, string(all[signatureFile]))
	if err != nil {
		return nil, err
	}
	if c.Iss != host || c.Sub != e.ID || c.Ver != e.Version || c.ManifestSHA256 != emp.ManifestSHA256 {
		return nil, fmt.Errorf("catalog.jws claims %+v do not match", c)
	}
	bj, ok := emp.Files["bundle.json"]
	if !ok {
		return nil, fmt.Errorf("no bundle.json")
	}
	if err := json.Unmarshal(bj, &emp.Doc); err != nil {
		return nil, fmt.Errorf("bundle.json: %w", err)
	}
	return emp, nil
}
