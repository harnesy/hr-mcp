package render

import (
	"strings"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// casebookSkill is the skill a Hermes profile reads the casebook through.
const casebookSkill = "persona-casebook"

// HermesSoulMax is the smallest number of characters Hermes cuts a context file at (context_file_max_chars: the cap
// grows with the model's context window; this is its floor). A longer SOUL.md loses its middle on a small model.
const HermesSoulMax = 20000

// hermes is a profile distribution for Hermes Agent in a folder named after the employee: `hermes profile install`
// takes the folder and makes a profile of its own. SOUL.md is the one file Hermes loads as the identity, so it holds
// who the employee is and its job. Skills sit under skills/<publisher>/, a category that says where they came from.
// The casebook is a skill too, so it is read when a case comes up and not on every turn. There is no config.yaml:
// the model settings of the profile are the person's.
func hermes(e *catalog.Employee, name string) Tree {
	home := name + "/"
	publisher, _, _ := strings.Cut(e.Entry.ID, "/")
	skills := home + "skills/" + publisher + "/"
	l := layout{target: Hermes, change: "assembled into a Hermes profile", inside: home,
		persona: home + "SOUL.md", notice: home + "NOTICE.md",
		casebook: skills + casebookSkill + "/references/persona-casebook.md", cases: "your skill `" + casebookSkill + "`",
		skillDir: func(skill string) string { return skills + skill },
		skillRef: func(skill string) string { return "`" + skill + "` (a skill of this profile)" }}
	soul := "# " + title(e) + "\n\n" + intro(e) + who(e) + job(e, l) + creditLine(e, l)
	own := []File{{l.persona, []byte(soul)}, {home + "distribution.yaml", []byte(distribution(e, name))}}
	if hasCasebook(e) {
		own = append(own, File{skills + casebookSkill + "/SKILL.md", []byte(casebookWrapper(e))})
	}
	return assemble(e, name, l, own...)
}

// distribution is distribution.yaml. Every value is a quoted string: Hermes reads YAML 1.1, where a bare `no` or
// `1.0` is not a string.
func distribution(e *catalog.Employee, name string) string {
	description := title(e)
	if s := oneLine(e.Entry.Summary); s != "" {
		description += ": " + s
	}
	author := publisherName(e)
	if own(e) {
		author = ownAuthor
	}
	b := "name: " + yamlString(name) + "\nversion: " + yamlString(e.Entry.Version) + "\ndescription: " + yamlString(description) +
		"\nauthor: " + yamlString(author) + "\n"
	if e.Doc.License != "" {
		b += "license: " + yamlString(e.Doc.License) + "\n"
	}
	return b
}

// casebookWrapper is the SKILL.md that makes the casebook a skill: the cases themselves are its reference, unchanged.
func casebookWrapper(e *catalog.Employee) string {
	whose := "the " + e.Doc.Role.Name
	if n := e.Doc.Persona.Name; n != "" {
		whose = n + ", the " + e.Doc.Role.Name
	}
	description := "Worked cases of " + oneLine(whose) + ": situation, decision, why. Read it when a case like these comes up, before a decision your principles do not settle."
	return "---\nname: " + casebookSkill + "\ndescription: " + yamlString(description) + "\n---\n\n# Casebook\n\n" +
		"The worked cases are in `references/persona-casebook.md` in this skill's folder. Read that file when a case like" +
		" these comes up. Quote only from it and never invent a quote.\n"
}
