package render

import "github.com/harnesy/hr-mcp/internal/catalog"

// generic is one folder named after the employee: AGENT.md, docs/, skills/, NOTICE.md.
func generic(e *catalog.Employee, name string) Tree {
	l := layout{target: Generic, change: "rendered for generic",
		persona: name + "/AGENT.md", notice: name + "/NOTICE.md", casebook: name + "/" + casebookFile,
		skillDir: func(skill string) string { return name + "/skills/" + skill },
		skillRef: func(skill string) string { return "`" + skill + "` (`" + name + "/skills/" + skill + "/SKILL.md`)" }}
	body := "# " + title(e) + "\n\n" + intro(e) + who(e) + job(e, l) + creditLine(e, l)
	return assemble(e, name, l, File{l.persona, []byte(body)})
}
