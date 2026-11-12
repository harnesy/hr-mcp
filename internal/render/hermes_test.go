package render_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/hermesscan"
	"github.com/harnesy/hr-mcp/internal/render"
)

// TestHermesDistribution: an employee for Hermes Agent is a profile distribution in a folder named after it:
// distribution.yaml, one SOUL.md with who it is and its job, its skills under skills/<publisher>/, the casebook as a
// skill read on demand, the notice. No config.yaml (the model settings are the person's), nothing of the person's.
func TestHermesDistribution(t *testing.T) {
	r := staff(t)
	for _, e := range r.Employees {
		tree := mustRender(t, e, render.Hermes)
		publisher, name, _ := strings.Cut(e.Entry.ID, "/")
		home := name + "/"
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`).MatchString(name) {
			t.Errorf("%s: %q is no Hermes profile name", e.Entry.ID, name)
		}
		files := map[string][]byte{}
		for _, f := range tree.Files {
			if !strings.HasPrefix(f.Path, home) {
				t.Errorf("%s: %q is outside %s", e.Entry.ID, f.Path, home)
			}
			files[strings.TrimPrefix(f.Path, home)] = f.Content
		}
		for _, never := range []string{"config.yaml", ".env", "mcp.json", "cron/jobs.json", "auth.json"} {
			if _, ok := files[never]; ok {
				t.Errorf("%s: %s must not be in the distribution: it is the person's", e.Entry.ID, never)
			}
		}
		if tree.Persona != home+"SOUL.md" || tree.Job != tree.Persona || tree.NoticeFile != home+"NOTICE.md" {
			t.Errorf("%s: persona %q, job %q, notice %q", e.Entry.ID, tree.Persona, tree.Job, tree.NoticeFile)
		}
		soul := string(files["SOUL.md"])
		for _, want := range []string{e.Doc.Persona.Character, e.Doc.Role.Mandate, e.Doc.Role.Prompt, "AI agent"} {
			if !strings.Contains(soul, want) {
				t.Errorf("%s: SOUL.md lacks %q", e.Entry.ID, want)
			}
		}
		if hits := hermesscan.Scan(soul); len(hits) > 0 {
			t.Errorf("%s: Hermes would block this SOUL.md: %v", e.Entry.ID, hits)
		}
		manifest := string(files["distribution.yaml"])
		for _, line := range []string{`name: "` + name + `"`, `version: "` + e.Entry.Version + `"`} {
			if !hasLine(manifest, line) {
				t.Errorf("%s: distribution.yaml lacks the line %q:\n%s", e.Entry.ID, line, manifest)
			}
		}
		if e.Doc.License != "" && !strings.Contains(manifest, e.Doc.License) {
			t.Errorf("%s: distribution.yaml must name the licence %s:\n%s", e.Entry.ID, e.Doc.License, manifest)
		}
		for skill := range skillsOf(e.Files) {
			if _, ok := files["skills/"+publisher+"/"+skill+"/SKILL.md"]; !ok {
				t.Errorf("%s: skill %s is not at skills/%s/%s/SKILL.md", e.Entry.ID, skill, publisher, skill)
			}
		}
		wrapper, has := files["skills/"+publisher+"/persona-casebook/SKILL.md"]
		casebook, inBundle := e.Files["docs/persona-casebook.md"]
		if has != inBundle {
			t.Fatalf("%s: the casebook skill is there: %v, the bundle has a casebook: %v", e.Entry.ID, has, inBundle)
		}
		if !inBundle {
			continue
		}
		if !bytes.Equal(files["skills/"+publisher+"/persona-casebook/references/persona-casebook.md"], casebook) {
			t.Errorf("%s: the casebook must travel unchanged as the skill's reference", e.Entry.ID)
		}
		m := regexp.MustCompile(`(?s)^---\nname: persona-casebook\ndescription: "([^"\n]+)"\n---\n\n(.+)$`).FindSubmatch(wrapper)
		if m == nil || len(m[1]) < 20 || len(m[1]) > 1024 || !bytes.Contains(m[2], []byte("references/persona-casebook.md")) {
			t.Errorf("%s: the casebook skill needs a name, a one-line description and a body that points at the reference:\n%s", e.Entry.ID, wrapper)
		}
		if !strings.Contains(soul, "`persona-casebook`") {
			t.Errorf("%s: SOUL.md must name the skill persona-casebook", e.Entry.ID)
		}
	}
}
