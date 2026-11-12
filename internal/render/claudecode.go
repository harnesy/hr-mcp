package render

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/harnesy/hr-mcp/internal/catalog"
)

// claudeCode is a project subagent (.claude/agents/<name>.md) with its skills as project skills (.claude/skills/). The
// casebook and the notice sit outside .claude/, in harnsy-hr/<name>/: Claude Code guards its own folder (a person is
// asked before every write there, an unattended run is refused), and a Markdown file under .claude/agents may be read
// as an agent. So the credit is an ordinary project file, the easiest one to write.
func claudeCode(e *catalog.Employee, name string) Tree {
	home := "harnsy-hr/" + name
	l := layout{target: ClaudeCode, change: "rendered for claude-code",
		persona: ".claude/agents/" + name + ".md", notice: home + "/NOTICE.md", casebook: home + "/" + casebookFile,
		skillDir: func(skill string) string { return ".claude/skills/" + skill },
		skillRef: func(skill string) string { return "`" + skill + "` (a project skill)" }}
	description := title(e)
	if s := e.Entry.Summary; s != "" {
		description += ": " + s
	}
	body := "---\nname: " + yamlString(name) + "\ndescription: " + yamlString(description) + "\n---\n\n" +
		"# " + title(e) + "\n\n" + intro(e) + who(e) + job(e, l) + creditLine(e, l)
	return assemble(e, name, l, File{l.persona, []byte(body)})
}

// yamlString writes s as a double-quoted YAML scalar on one line (a JSON string is one).
func yamlString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSpace(b.String())
}
