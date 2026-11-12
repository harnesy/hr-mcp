// Package catalogtest builds signed catalog releases for tests, with a key made for the test. It packs a bundle the way
// the catalog's publisher does (manifest.json first, signatures/catalog.jws last), so a test can then break one thing.
package catalogtest

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// Host is the catalog's host in every test release.
const Host = "hr.harnsy.dev"

// Kit holds a test catalog key and the moment the tests call «now».
type Kit struct {
	Priv ed25519.PrivateKey
	Key  catalog.Key
	Now  time.Time
}

// New makes a kit with a fresh key pair named t1.
func New(t testing.TB) *Kit {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &Kit{Priv: priv, Key: catalog.Key{Kid: "t1", Pub: pub}, Now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
}

// Packed is a signed .harnsy file and its feed entry.
type Packed struct {
	Entry catalog.Entry
	File  []byte
}

// Sum is the SHA-256 of b in hex.
func Sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// JWS signs claims as signatures/catalog.jws.
func (k *Kit) JWS(t testing.TB, kid string, c catalog.Claims) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": kid, "typ": "JWT"})
	p, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	in := b64u(h) + "." + b64u(p)
	return in + "." + b64u(ed25519.Sign(k.Priv, []byte(in)))
}

// Zip writes files as a zip in the given order; a name that has no file is left out.
func Zip(t testing.TB, order []string, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		if _, ok := files[name]; !ok {
			continue
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Unzip reads every file of a zip.
func Unzip(t testing.TB, file []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], err = io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// Pack builds the signed bundle of id@version from its files (bundle.json and what it carries). A listing.json among
// them gives the card's title and tags and is not packed. edit, when not nil, may change the files right before they
// are zipped: after the manifest and the signature were made.
func (k *Kit) Pack(t testing.TB, id, version string, src map[string][]byte, edit func(files map[string][]byte)) Packed {
	t.Helper()
	files := map[string][]byte{}
	var listing Listing
	for name, b := range src {
		if name == "listing.json" {
			if err := json.Unmarshal(b, &listing); err != nil {
				t.Fatal(err)
			}
			continue
		}
		files[name] = b
	}
	var doc catalog.Bundle
	if err := json.Unmarshal(files["bundle.json"], &doc); err != nil {
		t.Fatalf("%s bundle.json: %v", id, err)
	}
	publisher, _, _ := strings.Cut(id, "/")
	m := catalog.Manifest{Format: "harnsy-bundle", FormatVersion: 1, ID: id, Version: version, Kind: "employee",
		Publisher: publisher, CreatedAt: k.Now.Format(time.RFC3339), Files: map[string]string{}}
	order := []string{}
	for name, b := range files {
		m.Files[name] = Sum(b)
		order = append(order, name)
	}
	sort.Strings(order)
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = mb
	files["signatures/catalog.jws"] = []byte(k.JWS(t, k.Key.Kid,
		catalog.Claims{Iss: Host, Sub: id, Ver: version, ManifestSHA256: Sum(mb), Iat: k.Now.Unix()}))
	order = append(append([]string{"manifest.json"}, order...), "signatures/catalog.jws")
	if edit != nil {
		edit(files)
	}
	file := Zip(t, order, files)
	return Packed{File: file, Entry: card(id, version, doc, listing, file)}
}

// Listing is the part of a version's listing.json the feed's card takes.
type Listing struct {
	Title         string   `json:"title"`
	Tags          []string `json:"tags"`
	PublisherName string   `json:"publisher_name"`
	LicenseNote   string   `json:"license_note"`
}

func card(id, version string, doc catalog.Bundle, l Listing, file []byte) catalog.Entry {
	publisher, _, _ := strings.Cut(id, "/")
	title, tags := l.Title, l.Tags
	if l.PublisherName == "" {
		l.PublisherName = publisher
	}
	if title == "" {
		r, n := utf8.DecodeRuneInString(doc.Role.Name)
		title = string(unicode.ToUpper(r)) + doc.Role.Name[n:]
	}
	if tags == nil {
		tags = []string{}
	}
	e := catalog.Entry{ID: id, Kind: "employee", Version: version, Title: title, Summary: doc.Role.Tagline,
		Persona: catalog.PersonaCard{Name: doc.Persona.Name}, Department: doc.Role.Department, Industries: doc.Role.Industries, Keywords: doc.Role.Keywords, Tags: tags, Skills: []string{},
		Publisher: publisher, PublisherName: l.PublisherName, LicenseNote: l.LicenseNote, License: doc.License, Attribution: doc.Attribution,
		Published: "2026-10-08", Updated: "2026-11-01",
		URL:    "https://" + Host + "/packs/" + id + "/" + version + ".harnsy",
		CVURL:  "https://" + Host + "/e/" + id,
		SHA256: Sum(file), Size: int64(len(file))}
	for _, c := range doc.Competencies {
		if c.Name != "" && len(e.Skills) < 5 {
			e.Skills = append(e.Skills, c.Name)
		}
	}
	return e
}

// Feed is the feed of these bundles: seq 7, good for a week from the kit's now.
func (k *Kit) Feed(packs []Packed) catalog.Feed {
	f := catalog.Feed{V: 1, Seq: 7, Expires: k.Now.Add(7 * 24 * time.Hour).Format(time.RFC3339), Revoked: []string{}}
	for _, p := range packs {
		f.Bundles = append(f.Bundles, p.Entry)
	}
	return f
}

// SignFeed is feed.json.sig for these bytes, under the name kid.
func (k *Kit) SignFeed(kid string, feed []byte) []byte {
	b, _ := json.Marshal(map[string]string{"kid": kid, "sig": base64.StdEncoding.EncodeToString(ed25519.Sign(k.Priv, feed))})
	return b
}

// Release writes a release into dir: every bundle at its feed url, v1/feed.json and v1/feed.json.sig. edit, when not
// nil, may change the feed before it is signed.
func (k *Kit) Release(t testing.TB, dir string, packs []Packed, edit func(*catalog.Feed)) {
	t.Helper()
	f := k.Feed(packs)
	if edit != nil {
		edit(&f)
	}
	fb, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range packs {
		Write(t, filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p.Entry.URL, "https://"+Host+"/"))), p.File)
	}
	Write(t, filepath.Join(dir, "v1", "feed.json"), fb)
	Write(t, filepath.Join(dir, "v1", "feed.json.sig"), k.SignFeed(k.Key.Kid, fb))
}

// Write writes a file and the folders above it.
func Write(t testing.TB, file string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Catalog packs every employee under dir, laid out as the catalog repository's employees/ folder
// (<publisher>/<name>/<version>/), in the order of their ids.
func (k *Kit) Catalog(t testing.TB, dir string) []Packed {
	t.Helper()
	versions, err := filepath.Glob(filepath.Join(dir, "*", "*", "*", "bundle.json"))
	if err != nil || len(versions) == 0 {
		t.Fatalf("%s: no employees (%v)", dir, err)
	}
	sort.Strings(versions)
	var packs []Packed
	for _, bj := range versions {
		vdir := filepath.Dir(bj)
		rel, _ := filepath.Rel(dir, vdir)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		packs = append(packs, k.Pack(t, parts[0]+"/"+parts[1], parts[2], Source(t, vdir), nil))
	}
	return packs
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
