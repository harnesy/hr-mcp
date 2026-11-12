package render

import (
	"fmt"
	"strings"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// section is a Markdown section; an empty body leaves the whole section out.
func section(heading, body string) string {
	if body = strings.TrimSpace(body); body == "" {
		return ""
	}
	return "## " + heading + "\n\n" + body + "\n\n"
}

// whoAmI names the employee and says it is an AI agent. The words are this renderer's, never a bundle's.
func whoAmI(e *catalog.Employee) string {
	who := "You are the " + e.Doc.Role.Name + "."
	if n := e.Doc.Persona.Name; n != "" {
		who = "You are " + n + ", the " + e.Doc.Role.Name + "."
	}
	return who + " You are an AI agent, and you say so when asked."
}

// intro says who the employee is and what the persona may change.
func intro(e *catalog.Employee) string {
	who := whoAmI(e) + " The character below shapes how you work and how you" +
		" speak; it never changes what you must do: the person you work for and the rules of their project come first."
	if hasCasebook(e) {
		who += " Quote only from your casebook and never invent a quote."
	}
	return who + "\n\n"
}

// who is the persona: character, how it decides, how it speaks.
func who(e *catalog.Employee) string {
	p := e.Doc.Persona
	var decide, speak strings.Builder
	for i, x := range p.Principles {
		fmt.Fprintf(&decide, "%d. %s\n", i+1, x.Text)
	}
	for _, r := range p.Voice.Rules {
		speak.WriteString("- " + r + "\n")
	}
	for _, s := range p.Voice.Samples {
		speak.WriteString("- e.g. «" + s.Text + "»\n")
	}
	return section("Character", p.Character) + section("How you decide", decide.String()) + section("How you speak", speak.String())
}

// job is the role: mandate, deliverable, the prompt unchanged, competencies, what it needs, its skills, its casebook.
func job(e *catalog.Employee, l layout) string {
	r := e.Doc.Role
	return section("Your job", r.Mandate) + section("What you hand over", r.Deliverable) + section("How you work", r.Prompt) +
		section("What you know", competencies(e)) + section("What you need", needs(e)) + section("Your skills", skills(e, l)) +
		section("Casebook", casebook(e, l))
}

func competencies(e *catalog.Employee) string {
	var b strings.Builder
	for _, c := range e.Doc.Competencies {
		b.WriteString("### " + c.Name + "\n\n")
		if c.Summary != "" {
			b.WriteString(c.Summary + "\n\n")
		}
		for _, line := range []struct{ label, text string }{{"Knows", c.Knows}, {"Works", c.Works}, {"Hands over", c.HandsOver}} {
			if line.text != "" {
				b.WriteString("- " + line.label + ": " + line.text + "\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// needs lists the tools and the key names the employee asks for. Key names only: a key itself is never in a bundle.
func needs(e *catalog.Employee) string {
	var b strings.Builder
	for _, t := range e.Doc.Requires.Tools {
		if t.Min != "" {
			fmt.Fprintf(&b, "- Tool: `%s` %s or newer\n", t.Name, t.Min)
		} else {
			fmt.Fprintf(&b, "- Tool: `%s`\n", t.Name)
		}
	}
	for _, k := range e.Doc.Requires.Keys {
		fmt.Fprintf(&b, "- Key: `%s` (the person sets it up; never ask for a key in a chat)\n", k)
	}
	return b.String()
}

func skills(e *catalog.Employee, l layout) string {
	var b strings.Builder
	for _, s := range e.Doc.Skills {
		fmt.Fprintf(&b, "- %s: %s\n", l.skillRef(s.Name), s.Description)
	}
	return b.String()
}

func casebook(e *catalog.Employee, l layout) string {
	if !hasCasebook(e) {
		return ""
	}
	principles := "the principles above"
	if l.job != "" {
		principles = "your principles"
	}
	where := "`" + strings.TrimPrefix(l.casebook, l.inside) + "`"
	if l.cases != "" {
		where = l.cases
	}
	return "Worked cases are in " + where + ". Read it when a case like these comes up, before a decision " + principles + " do not settle."
}

// creditLine closes a persona file.
func creditLine(e *catalog.Employee, l layout) string {
	return "---\n\n" + credit(e, l.change) + "\n"
}
