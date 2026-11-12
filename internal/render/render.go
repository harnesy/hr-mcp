// Package render turns a verified employee bundle into the files an agent system reads: who the employee is, its job,
// its skills byte for byte, and the notice with its licence and credits. One renderer serves every target; a target
// differs only in where the files go and how the persona is split.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// Target is the agent system a tree is rendered for.
type Target string

// The targets.
const (
	Generic    Target = "generic"     // one folder, one AGENT.md: any agent that reads files
	ClaudeCode Target = "claude-code" // a subagent file and project skills under .claude/
)

// Targets lists every target, in the order they are offered.
func Targets() []Target { return []Target{Generic, ClaudeCode} }

// File is one file of a tree: its path from the place the tree is written to, with forward slashes.
type File struct {
	Path    string
	Content []byte
}

// Tree is an employee rendered for a target.
type Tree struct {
	Target     Target
	Name       string // the employee's short name: its id without the publisher
	Persona    string // the path of the file that says who the employee is; it ends with the credit line
	NoticeFile string // the path of the notice
	Notice     string // the notice's text: licence, source, changes, credits
	Files      []File // in path order
}

// layout is where a target keeps things.
type layout struct {
	target   Target
	change   string                    // the «Changes:» of the notice
	persona  string                    // path of the persona file
	notice   string                    // path of the notice
	casebook string                    // path of the casebook
	skillDir func(skill string) string // folder of a skill
	skillRef func(skill string) string // how the persona file points at a skill
}

// Render makes the employee's tree for a target.
func Render(e *catalog.Employee, t Target) (Tree, error) {
	name := shortName(e)
	switch t {
	case Generic:
		return generic(e, name), nil
	case ClaudeCode:
		return claudeCode(e, name), nil
	}
	return Tree{}, fmt.Errorf("target %q: want one of %s", t, targetList())
}

func targetList() string {
	var s []string
	for _, t := range Targets() {
		s = append(s, string(t))
	}
	return strings.Join(s, ", ")
}

// shortName is the id without its publisher: harnsy/accountant → accountant.
func shortName(e *catalog.Employee) string {
	_, name, _ := strings.Cut(e.Entry.ID, "/")
	return name
}

// title is the employee's display title.
func title(e *catalog.Employee) string {
	if e.Entry.Title != "" {
		return e.Entry.Title
	}
	return e.Doc.Role.Name
}

// assemble puts a tree together: the persona file and any other files a target wrote itself, the casebook, every
// skill file unchanged, the notice.
func assemble(e *catalog.Employee, name string, l layout, own ...File) Tree {
	t := Tree{Target: l.target, Name: name, Persona: l.persona, NoticeFile: l.notice, Notice: notice(e, l.change), Files: own}
	t.Files = append(t.Files, File{l.notice, []byte("# Notice\n\n" + t.Notice)})
	for path, b := range e.Files {
		switch rest, isSkill := strings.CutPrefix(path, "skills/"); {
		case isSkill:
			skill, file, _ := strings.Cut(rest, "/")
			t.Files = append(t.Files, File{l.skillDir(skill) + "/" + file, b})
		case path == casebookFile:
			t.Files = append(t.Files, File{l.casebook, b})
		}
	}
	sort.Slice(t.Files, func(i, j int) bool { return t.Files[i].Path < t.Files[j].Path })
	return t
}

const casebookFile = "docs/persona-casebook.md"

func hasCasebook(e *catalog.Employee) bool { _, ok := e.Files[casebookFile]; return ok }
