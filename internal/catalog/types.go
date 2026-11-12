// Package catalog loads a signed release of the hr.harnsy.dev employee catalog from disk and refuses anything that does
// not verify: the feed's signature, each bundle's size, hashes and catalog signature. The rules are the catalog's own
// (bundle v1); a release that fails is never served.
package catalog

import (
	"crypto/ed25519"
	"encoding/json"
)

// Limits of bundle v1.
const (
	MaxFeed     = 4 << 20 // feed.json
	MaxBundle   = 8 << 20 // a .harnsy file, and all its files unpacked
	MaxZipFiles = 128     // entries in a .harnsy file
)

// Key is a catalog public key and the id the signatures name it by.
type Key struct {
	Kid string
	Pub ed25519.PublicKey
}

// Feed is v1/feed.json.
type Feed struct {
	V       int      `json:"v"`
	Seq     int64    `json:"seq"`
	Expires string   `json:"expires"`
	Bundles []Entry  `json:"bundles"`
	Revoked []string `json:"revoked"`
}

// Entry is one bundle of the feed: its card and where its file is.
type Entry struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind"`
	Version       string        `json:"version"`
	Title         string        `json:"title"`
	Summary       string        `json:"summary"`
	Persona       PersonaCard   `json:"persona"`
	Department    string        `json:"department,omitempty"`
	Industries    []string      `json:"industries,omitempty"`
	Tags          []string      `json:"tags"`
	Skills        []string      `json:"skills"`
	Keywords      []string      `json:"keywords,omitempty"`
	Published     string        `json:"published,omitempty"`
	Updated       string        `json:"updated,omitempty"`
	PublisherName string        `json:"publisher_name,omitempty"`
	URL           string        `json:"url"`
	SHA256        string        `json:"sha256"`
	Size          int64         `json:"size"`
	Publisher     string        `json:"publisher"`
	AvatarURL     string        `json:"avatar_url,omitempty"`
	CVURL         string        `json:"cv_url,omitempty"`
	License       string        `json:"license,omitempty"`
	Attribution   []Attribution `json:"attribution,omitempty"`
}

// PersonaCard is the persona as the feed shows it.
type PersonaCard struct {
	Name string `json:"name"`
}

// Attribution is one public credit of a bundle.
type Attribution struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	License  string `json:"license"`
	Modified bool   `json:"modified"`
}

// Manifest is a bundle's manifest.json: what the file is and the SHA-256 of every other file but signatures/*.
type Manifest struct {
	Format        string            `json:"format"`
	FormatVersion int               `json:"format_version"`
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	Kind          string            `json:"kind"`
	Publisher     string            `json:"publisher"`
	CreatedAt     string            `json:"created_at"`
	CVURL         string            `json:"cv_url,omitempty"`
	Files         map[string]string `json:"files"`
}

// Claims is the payload of signatures/catalog.jws.
type Claims struct {
	Iss            string `json:"iss"`
	Sub            string `json:"sub"`
	Ver            string `json:"ver"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Iat            int64  `json:"iat"`
}

// Bundle is bundle.json: the employee.
type Bundle struct {
	Schema       string        `json:"schema"`
	Persona      Persona       `json:"persona"`
	Role         Role          `json:"role"`
	Requires     Requires      `json:"requires"`
	Competencies []Competency  `json:"competencies"`
	License      string        `json:"license"`
	Attribution  []Attribution `json:"attribution"`
	Skills       SkillRefs     `json:"skills"`
}

// Persona is who the employee is.
type Persona struct {
	Name       string      `json:"name"`
	Character  string      `json:"character"`
	Principles []Principle `json:"principles"`
	Voice      Voice       `json:"voice"`
}

// Principle is one line of «how it decides».
type Principle struct {
	Text string `json:"text"`
}

// Voice is «how it speaks».
type Voice struct {
	Rules   []string `json:"rules"`
	Samples []Sample `json:"samples"`
}

// Sample is one example line of the persona's voice.
type Sample struct {
	Text string `json:"text"`
}

// Role is the job.
type Role struct {
	Name        string `json:"name"`
	Department  string `json:"department"`
	Mandate     string `json:"mandate"`
	Deliverable string `json:"deliverable"`
	Prompt      string `json:"prompt"`
	Tagline     string `json:"tagline"`
}

// Requires is what the employee needs to work. The harnsy Max list of a bundle is not read: it means nothing outside harnsy.
type Requires struct {
	Tools []Tool   `json:"tools"`
	Keys  []string `json:"keys"`
}

// Tool is a command the employee needs, with its lowest version.
type Tool struct {
	Name string `json:"name"`
	Min  string `json:"min"`
}

// Competency is one thing the employee knows and how it works with it.
type Competency struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Knows     string `json:"knows"`
	Works     string `json:"works"`
	HandsOver string `json:"hands_over"`
}

// SkillRef names a skill the bundle carries in skills/<name>/.
type SkillRef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SkillRefs is a bundle's skills list. An older bundle may hold plain strings there: labels that carry nothing, left out.
type SkillRefs []SkillRef

// UnmarshalJSON keeps the entries that are objects.
func (s *SkillRefs) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*s = (*s)[:0]
	for _, r := range raw {
		var ref SkillRef
		if json.Unmarshal(r, &ref) == nil && ref.Name != "" {
			*s = append(*s, ref)
		}
	}
	return nil
}

// Employee is one verified bundle of a release.
type Employee struct {
	Entry          Entry
	Manifest       Manifest
	ManifestSHA256 string
	Doc            Bundle
	Files          map[string][]byte // every file the manifest lists, by its path in the bundle
}

// Release is a verified release: the feed and every employee it serves, in the feed's order.
type Release struct {
	Feed      Feed
	Kid       string
	Employees []*Employee
	byID      map[string]*Employee
}

// Employee returns the employee with that id, or nil.
func (r *Release) Employee(id string) *Employee { return r.byID[id] }
