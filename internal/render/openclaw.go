package render

import (
	"strings"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// What OpenClaw loads of a workspace at the start of a session (agents.defaults.bootstrapMaxChars and
// bootstrapTotalMaxChars); a longer file is cut. The casebook stays out of the loaded files for that reason.
const (
	OpenClawFileMax  = 20000 // characters of one file
	OpenClawTotalMax = 60000 // characters of all loaded files together
)

// openClaw is an agent workspace in a folder of its own, workspace-<name>/: SOUL.md says who the employee is,
// AGENTS.md holds the job (a sub-agent is given only that file, so it also says who it is), IDENTITY.md the name and
// the vibe, skills/ its skills, which belong to this agent only. The person's own files (USER.md, MEMORY.md) and the
// first-run ritual (BOOTSTRAP.md) are never written. The folder keeps the files away from the workspace of the agent
// that installs them, whose own SOUL.md and AGENTS.md have the same names.
func openClaw(e *catalog.Employee, name string) Tree {
	home := "workspace-" + name + "/"
	l := layout{target: OpenClaw, change: "rendered for openclaw", inside: home,
		persona: home + "SOUL.md", job: home + "AGENTS.md", notice: home + "NOTICE.md", casebook: home + casebookFile,
		skillDir: func(skill string) string { return home + "skills/" + skill },
		skillRef: func(skill string) string { return "`" + skill + "` (a skill of this workspace)" }}
	soul := "# " + title(e) + "\n\n" + intro(e) + who(e) + creditLine(e, l)
	agents := "# " + title(e) + ": the job\n\n" + whoAmI(e) + " This file is your job; your character is in `SOUL.md`.\n\n" +
		job(e, l) + creditLine(e, l)
	return assemble(e, name, l, File{l.persona, []byte(soul)}, File{l.job, []byte(agents)},
		File{home + "IDENTITY.md", []byte(identity(e))})
}

// identity is IDENTITY.md: one `- Label: value` line per field, the form OpenClaw parses and writes itself.
func identity(e *catalog.Employee) string {
	name := e.Doc.Persona.Name
	if name == "" {
		name = title(e)
	}
	b := "# IDENTITY.md\n\n- Name: " + oneLine(name) + "\n- Creature: AI agent, " + oneLine(e.Doc.Role.Name) + "\n"
	if vibe := oneLine(e.Doc.Role.Tagline); vibe != "" {
		b += "- Vibe: " + vibe + "\n"
	}
	return b + "- Emoji: " + emoji(e.Doc.Role.Department) + "\n"
}

// oneLine joins a text into one line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// departmentEmoji is one sign per department of the catalog. Each is a single character: no joined or tinted signs,
// which some readers take for hidden text.
var departmentEmoji = map[string]string{
	"content": "📝", "data": "📊", "design": "🎨", "engineering": "🔧", "finance": "🧮", "hr": "👥", "legal": "📜",
	"management": "🧭", "marketing": "📣", "operations": "📋", "product": "📦", "sales": "🤝", "support": "🎧",
}

func emoji(department string) string {
	if s, ok := departmentEmoji[strings.ToLower(strings.TrimSpace(department))]; ok {
		return s
	}
	return "🤖"
}
