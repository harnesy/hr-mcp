package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/render"
)

// Sizes of an answer. Agent clients cut a tool's answer (Claude Code at 25,000 tokens), so an employee that does not
// fit comes as a list of files with the large ones left for get_employee_file.
const (
	MaxAnswer    = 96 << 10 // the most a tool's answer is, as JSON text
	inlineBudget = 60 << 10 // file content, as JSON, that get_employee puts in its answer
	partBudget   = 60 << 10 // file content, as JSON, in one answer of get_employee_file
)

// EmployeeIn is the input of get_employee.
type EmployeeIn struct {
	ID      string `json:"id" jsonschema:"the employee's id from search_employees, e.g. harnsy/accountant"`
	Version string `json:"version,omitempty" jsonschema:"the version to read; left out: the one the catalog serves"`
	Target  string `json:"target,omitempty" jsonschema:"what the files are laid out for: generic (default), claude-code or openclaw; ask install_hint which one fits a client"`
}

// EmployeeOut is the answer of get_employee.
type EmployeeOut struct {
	Employee    Card                  `json:"employee"`
	Target      string                `json:"target"`
	PersonaFile string                `json:"persona_file" jsonschema:"the path of the file that says who the employee is"`
	JobFile     string                `json:"job_file" jsonschema:"the path of the file that holds the employee's job; the persona file where one file holds both"`
	Files       []FileOut             `json:"files" jsonschema:"every file to write, by its path from the place the employee is installed to"`
	License     string                `json:"license" jsonschema:"the licence of the employee's text, SPDX"`
	Attribution []catalog.Attribution `json:"attribution" jsonschema:"the works the text is built on"`
	Notice      string                `json:"notice" jsonschema:"the credit that must stay with the files; it is also in the tree as a file"`
	Bundle      BundleRef             `json:"bundle" jsonschema:"the signed file these files were rendered from"`
	Feed        FeedRef               `json:"feed" jsonschema:"the signed feed that lists the bundle"`
	CVURL       string                `json:"cv_url,omitempty"`
	Note        string                `json:"note"`
}

// FileOut is one file of an employee's tree.
type FileOut struct {
	Path       string  `json:"path"`
	Bytes      int     `json:"bytes" jsonschema:"the whole file's size"`
	SHA256     string  `json:"sha256" jsonschema:"the whole file's SHA-256, hex"`
	Content    *string `json:"content" jsonschema:"the file's text, or this part of it; null when left out of get_employee: fetch it with get_employee_file"`
	Offset     int     `json:"offset,omitempty" jsonschema:"where this part starts, in bytes"`
	NextOffset int     `json:"next_offset,omitempty" jsonschema:"where the next part starts; absent when this is the last"`
}

// BundleRef names the signed bundle file.
type BundleRef struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// FeedRef names the feed the server verified.
type FeedRef struct {
	Seq int64  `json:"seq"`
	Kid string `json:"kid" jsonschema:"the catalog key that signed it"`
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// jsonLen is how many bytes a text takes inside a JSON answer.
func jsonLen(b []byte) int {
	enc, _ := json.Marshal(string(b))
	return len(enc)
}

// tree renders the employee for the target a caller named; none means generic.
func (s *server) tree(id, version, target string) (*catalog.Release, *catalog.Employee, render.Tree, error) {
	rel, e, err := s.find(id, version)
	if err != nil {
		return nil, nil, render.Tree{}, err
	}
	if target == "" {
		target = string(render.Generic)
	}
	t, err := render.Render(e, render.Target(target))
	return rel, e, t, err
}

func (s *server) employee(_ context.Context, _ *mcp.CallToolRequest, in EmployeeIn) (*mcp.CallToolResult, EmployeeOut, error) {
	rel, e, t, err := s.tree(in.ID, in.Version, in.Target)
	if err != nil {
		return nil, EmployeeOut{Files: []FileOut{}, Attribution: []catalog.Attribution{}}, s.logged("get_employee", in.ID, err)
	}
	out := EmployeeOut{Employee: card(e), Target: string(t.Target), PersonaFile: t.Persona, JobFile: t.Job, License: e.Doc.License,
		Attribution: e.Doc.Attribution, Notice: t.Notice, CVURL: e.Entry.CVURL,
		Bundle: BundleRef{URL: e.Entry.URL, SHA256: e.Entry.SHA256, Size: e.Entry.Size},
		Feed:   FeedRef{Seq: rel.Feed.Seq, Kid: rel.Kid},
		Note:   dataNote + " A file with content null was left out for size: fetch it with get_employee_file and the same id, version, target and path. Check each file you write against its sha256.",
		Files:  make([]FileOut, len(t.Files))}
	if out.Attribution == nil {
		out.Attribution = []catalog.Attribution{}
	}
	// The persona file, the job and the notice always come; then what fits, in path order.
	left := inlineBudget
	inline := func(i int, must bool) {
		f := t.Files[i]
		if n := jsonLen(f.Content); out.Files[i].Content == nil && utf8.Valid(f.Content) && (must || n <= left) {
			text := string(f.Content)
			out.Files[i].Content, left = &text, left-n
		}
	}
	for i, f := range t.Files {
		out.Files[i] = FileOut{Path: f.Path, Bytes: len(f.Content), SHA256: sum(f.Content)}
		if f.Path == t.Persona || f.Path == t.Job || f.Path == t.NoticeFile {
			inline(i, true)
		}
	}
	for i := range t.Files {
		inline(i, false)
	}
	return nil, out, s.logged("get_employee", in.ID, nil)
}

// FileIn is the input of get_employee_file.
type FileIn struct {
	ID      string `json:"id"`
	Version string `json:"version" jsonschema:"the version get_employee answered with"`
	Target  string `json:"target" jsonschema:"the target get_employee answered with"`
	Path    string `json:"path" jsonschema:"the file's path, as get_employee listed it"`
	Offset  int    `json:"offset,omitempty" jsonschema:"next_offset of the part before; left out for the start"`
}

func (s *server) file(_ context.Context, _ *mcp.CallToolRequest, in FileIn) (*mcp.CallToolResult, FileOut, error) {
	out, err := s.part(in)
	return nil, out, s.logged("get_employee_file", in.ID, err)
}

func (s *server) part(in FileIn) (FileOut, error) {
	_, _, t, err := s.tree(in.ID, in.Version, in.Target)
	if err != nil {
		return FileOut{}, err
	}
	for _, f := range t.Files {
		if f.Path != in.Path {
			continue
		}
		b := f.Content
		if !utf8.Valid(b) {
			return FileOut{}, fmt.Errorf("%s is not text", in.Path)
		}
		if in.Offset < 0 || in.Offset > len(b) || (in.Offset == len(b) && len(b) > 0) || (in.Offset < len(b) && !utf8.RuneStart(b[in.Offset])) {
			return FileOut{}, fmt.Errorf("offset %d: not the start of a part of %s (%d bytes)", in.Offset, in.Path, len(b))
		}
		end := len(b)
		for jsonLen(b[in.Offset:end]) > partBudget {
			end = in.Offset + (end-in.Offset)*3/4
			for !utf8.RuneStart(b[end]) {
				end--
			}
		}
		text := string(b[in.Offset:end])
		out := FileOut{Path: f.Path, Bytes: len(b), SHA256: sum(b), Content: &text, Offset: in.Offset}
		if end < len(b) {
			out.NextOffset = end
		}
		return out, nil
	}
	return FileOut{}, errors.New(in.Path + " is not a file of this employee for this target; take the paths from get_employee")
}
