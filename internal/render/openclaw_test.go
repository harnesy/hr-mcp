package render_test

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/render"
)

// TestOpenClawWorkspace: an employee for OpenClaw is one agent workspace in a folder of its own. AGENTS.md carries
// the job (a sub-agent is given only that file), SOUL.md who it is, IDENTITY.md the name and the vibe as the lines
// OpenClaw parses. The person's own files (USER.md, MEMORY.md) and the first-run ritual (BOOTSTRAP.md) are never written.
func TestOpenClawWorkspace(t *testing.T) {
	r := staff(t)
	for _, e := range r.Employees {
		tree := mustRender(t, e, render.OpenClaw)
		_, name, _ := strings.Cut(e.Entry.ID, "/")
		home := "workspace-" + name + "/"
		files := filesUnder(t, e, tree, home)
		for _, never := range []string{"USER.md", "MEMORY.md", "BOOTSTRAP.md", "TOOLS.md", "HEARTBEAT.md"} {
			if _, ok := files[never]; ok {
				t.Errorf("%s: %s must not be written", e.Entry.ID, never)
			}
		}
		if tree.Persona != home+"SOUL.md" || tree.Job != home+"AGENTS.md" || tree.NoticeFile != home+"NOTICE.md" {
			t.Errorf("%s: persona %q, job %q, notice %q", e.Entry.ID, tree.Persona, tree.Job, tree.NoticeFile)
		}
		soul, agents, identity := files["SOUL.md"], files["AGENTS.md"], files["IDENTITY.md"]
		for _, want := range []string{e.Doc.Persona.Character, "AI agent"} {
			if !strings.Contains(soul, want) {
				t.Errorf("%s: SOUL.md lacks %q", e.Entry.ID, want)
			}
		}
		if strings.Contains(soul, e.Doc.Role.Prompt) {
			t.Errorf("%s: the job belongs in AGENTS.md, not in SOUL.md", e.Entry.ID)
		}
		for _, want := range []string{e.Doc.Role.Mandate, e.Doc.Role.Deliverable, e.Doc.Role.Prompt, "AI agent"} {
			if !strings.Contains(agents, want) {
				t.Errorf("%s: AGENTS.md lacks %q", e.Entry.ID, want)
			}
		}
		for _, f := range []string{"SOUL.md", "AGENTS.md"} {
			if last := lastLine(files[f]); !strings.Contains(last, e.Entry.CVURL) || !strings.Contains(last, "assembled into an OpenClaw agent workspace") {
				t.Errorf("%s: %s must end with the credit line, ends with %q", e.Entry.ID, f, last)
			}
		}
		identityHasItsLines(t, e, identity)
		for skill := range skillsOf(e.Files) {
			if _, ok := files["skills/"+skill+"/SKILL.md"]; !ok {
				t.Errorf("%s: skill %s is not at skills/%s/SKILL.md of the workspace", e.Entry.ID, skill, skill)
			}
		}
		fitsOpenClaw(t, e, files)
	}
}

// filesUnder is a tree's files by their paths inside home; every file must be there.
func filesUnder(t *testing.T, e *catalog.Employee, tree render.Tree, home string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, f := range tree.Files {
		if !strings.HasPrefix(f.Path, home) {
			t.Errorf("%s: %q is outside %s: a file beside the employee's folder could replace the person's own", e.Entry.ID, f.Path, home)
		}
		files[strings.TrimPrefix(f.Path, home)] = string(f.Content)
	}
	return files
}

// identityHasItsLines: IDENTITY.md has one `- Label: value` line per field, the form OpenClaw parses.
func identityHasItsLines(t *testing.T, e *catalog.Employee, identity string) {
	t.Helper()
	who := e.Doc.Persona.Name
	if who == "" {
		who = e.Entry.Title
	}
	for _, line := range []string{"- Name: " + who, "- Creature: AI agent, " + e.Doc.Role.Name, "- Vibe: " + e.Doc.Role.Tagline} {
		if !hasLine(identity, line) {
			t.Errorf("%s: IDENTITY.md lacks the line %q:\n%s", e.Entry.ID, line, identity)
		}
	}
	if !regexp.MustCompile(`(?m)^- Emoji: \S+$`).MatchString(identity) {
		t.Errorf("%s: IDENTITY.md lacks an emoji line:\n%s", e.Entry.ID, identity)
	}
}

// fitsOpenClaw: the files OpenClaw loads every session stay within its limits, per file and together.
func fitsOpenClaw(t *testing.T, e *catalog.Employee, files map[string]string) {
	t.Helper()
	total := 0
	for _, f := range []string{"AGENTS.md", "SOUL.md", "IDENTITY.md"} {
		n := len(utf16.Encode([]rune(files[f])))
		if total += n; n == 0 || n > render.OpenClawFileMax {
			t.Errorf("%s: %s is %d characters; OpenClaw loads at most %d of a file", e.Entry.ID, f, n, render.OpenClawFileMax)
		}
	}
	if total > render.OpenClawTotalMax {
		t.Errorf("%s: the files OpenClaw loads every session are %d characters, at most %d", e.Entry.ID, total, render.OpenClawTotalMax)
	}
}

// TestOpenClawIdentityIsOneLinePerField: a tagline or a name with a line break must not break the parser's lines.
func TestOpenClawIdentityIsOneLinePerField(t *testing.T) {
	e := *staff(t).Employees[0]
	e.Doc.Role.Tagline = "Counts twice.\nWrites it  down once.\n"
	e.Doc.Role.Department = "a department nobody listed"
	tree := mustRender(t, &e, render.OpenClaw)
	for _, f := range tree.Files {
		if strings.HasSuffix(f.Path, "/IDENTITY.md") {
			if !hasLine(string(f.Content), "- Vibe: Counts twice. Writes it down once.") {
				t.Errorf("the vibe must be one line:\n%s", f.Content)
			}
			if !regexp.MustCompile(`(?m)^- Emoji: \S+$`).Match(f.Content) {
				t.Errorf("an unknown department still gets an emoji:\n%s", f.Content)
			}
			return
		}
	}
	t.Fatal("no IDENTITY.md")
}

func hasLine(text, line string) bool {
	for _, l := range strings.Split(text, "\n") {
		if l == line {
			return true
		}
	}
	return false
}

// skillsOf names the skills among a bundle's files.
func skillsOf(files map[string][]byte) map[string]bool {
	out := map[string]bool{}
	for path := range files {
		if rest, ok := strings.CutPrefix(path, "skills/"); ok {
			skill, _, _ := strings.Cut(rest, "/")
			out[skill] = true
		}
	}
	return out
}
