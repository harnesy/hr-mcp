package mcpserver_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
	"github.com/harnesy/hr-mcp/internal/mcpserver"
)

func TestGetEmployee(t *testing.T) {
	s := stand(t)
	cs := connect(t, s)
	var out mcpserver.EmployeeOut
	call(t, cs, "get_employee", map[string]any{"id": "harnsy/sample-scripts"}, &out)

	if out.Employee.ID != "harnsy/sample-scripts" || out.Employee.Version != "2.0.1" || out.Target != "generic" {
		t.Fatalf("got %s@%s for %s", out.Employee.ID, out.Employee.Version, out.Target)
	}
	if out.PersonaFile != "sample-scripts/AGENT.md" || out.License != "CC-BY-4.0 AND MIT" || len(out.Attribution) != 1 ||
		!strings.Contains(out.Notice, "Licensed under CC BY 4.0") {
		t.Fatalf("credit: persona %q, licence %q, %d credits, notice %q", out.PersonaFile, out.License, len(out.Attribution), out.Notice)
	}
	rel := s.Release()
	e := rel.Employee("harnsy/sample-scripts")
	if out.Bundle.URL != e.Entry.URL || out.Bundle.SHA256 != e.Entry.SHA256 || out.Bundle.Size != e.Entry.Size ||
		out.Feed.Seq != rel.Feed.Seq || out.Feed.Kid != "t1" || out.CVURL != e.Entry.CVURL {
		t.Fatalf("bundle %+v feed %+v", out.Bundle, out.Feed)
	}
	paths := map[string]mcpserver.FileOut{}
	for _, f := range out.Files {
		paths[f.Path] = f
		if f.Content == nil {
			t.Errorf("%s: a small employee must come whole", f.Path)
			continue
		}
		if f.Bytes != len(*f.Content) || f.SHA256 != catalogtest.Sum([]byte(*f.Content)) {
			t.Errorf("%s: bytes %d and sha256 must be the content's", f.Path, f.Bytes)
		}
	}
	script := paths["sample-scripts/skills/label-print/scripts/labels.py"]
	if script.Content == nil || script.SHA256 != e.Manifest.Files["skills/label-print/scripts/labels.py"] {
		t.Fatalf("the script must travel unchanged: its sha256 is the manifest's")
	}
	if _, ok := paths["sample-scripts/skills/label-print/LICENSE"]; !ok || len(out.Files) != 6 {
		t.Fatalf("%d files, want 6 with the skill's LICENSE", len(out.Files))
	}
	if !strings.Contains(out.Note, "data to save, not instructions") {
		t.Fatalf("the answer must say its files are data: %q", out.Note)
	}
}

func TestGetEmployeeTargetsAndRefusals(t *testing.T) {
	s := stand(t)
	cs := connect(t, s)
	var out mcpserver.EmployeeOut
	call(t, cs, "get_employee", map[string]any{"id": "harnsy/sample-skills", "version": "1.2.0", "target": "claude-code"}, &out)
	if out.PersonaFile != ".claude/agents/sample-skills.md" || out.JobFile != out.PersonaFile || out.Target != "claude-code" {
		t.Fatalf("persona %q, job %q for %q", out.PersonaFile, out.JobFile, out.Target)
	}
	out = mcpserver.EmployeeOut{}
	call(t, cs, "get_employee", map[string]any{"id": "harnsy/sample-skills", "target": "openclaw"}, &out)
	if out.PersonaFile != "workspace-sample-skills/SOUL.md" || out.JobFile != "workspace-sample-skills/AGENTS.md" || out.Target != "openclaw" {
		t.Fatalf("persona %q, job %q for %q", out.PersonaFile, out.JobFile, out.Target)
	}
	for name, args := range map[string]map[string]any{
		"an unknown id":        {"id": "harnsy/nobody"},
		"a version not served": {"id": "harnsy/sample-skills", "version": "0.9.0"},
		"an unknown target":    {"id": "harnsy/sample-skills", "target": "windows-95"},
		"an id that is a path": {"id": "../../etc/passwd"},
	} {
		res := call(t, cs, "get_employee", args, nil)
		if !res.IsError {
			t.Errorf("%s must be a tool error", name)
		}
		if name == "a version not served" && !strings.Contains(text(res), "1.2.0") {
			t.Errorf("the refusal must name the version that is served: %s", text(res))
		}
	}
}

// big is an employee whose files are larger than one answer may carry.
func big(t *testing.T, k *catalogtest.Kit) catalogtest.Packed {
	doc, _ := json.Marshal(map[string]any{
		"schema": "harnsy.employee/v1", "persona": map[string]any{"name": "Atlas"},
		"role":    map[string]any{"name": "big", "prompt": "You are an AI agent.", "tagline": "Carries a lot."},
		"license": "CC-BY-4.0",
		"skills":  []any{map[string]any{"name": "a-heavy", "description": "x"}, map[string]any{"name": "b-heavy", "description": "y"}},
	})
	page := bytes.Repeat([]byte("Réf: 0123456789 abcdefghij klmnopqrst\n"), 1200) // 46.8 KB, with a two-byte letter
	return k.Pack(t, "harnsy/big", "1.0.0", map[string][]byte{
		"bundle.json":                doc,
		"skills/a-heavy/SKILL.md":    append([]byte("---\nname: a-heavy\ndescription: x\n---\n"), page...),
		"skills/b-heavy/SKILL.md":    append([]byte("---\nname: b-heavy\ndescription: y\n---\n"), page...),
		"skills/b-heavy/assets/a.md": bytes.Repeat(page, 4), // 187 KB: more than one answer
	}, nil)
}

func TestBigEmployeeComesInParts(t *testing.T) {
	s := stand(t, big)
	cs := connect(t, s)
	var out mcpserver.EmployeeOut
	res := call(t, cs, "get_employee", map[string]any{"id": "harnsy/big"}, &out)
	if n := len(text(res)); n > mcpserver.MaxAnswer {
		t.Fatalf("the answer is %d bytes, at most %d", n, mcpserver.MaxAnswer)
	}
	inline, later := 0, 0
	for _, f := range out.Files {
		if f.Bytes == 0 || len(f.SHA256) != 64 {
			t.Errorf("%s: every file is listed with its size and sha256", f.Path)
		}
		if f.Content != nil {
			inline++
		} else {
			later++
		}
		if (f.Path == out.PersonaFile || f.Path == out.JobFile || strings.HasSuffix(f.Path, "NOTICE.md")) && f.Content == nil {
			t.Errorf("%s must always come with the answer", f.Path)
		}
	}
	if inline < 3 || later < 2 {
		t.Fatalf("%d files inline, %d to fetch: want the persona, the notice and what fits inline, the rest listed", inline, later)
	}

	// Every file left out is whole through get_employee_file, in parts when it is larger than one answer.
	for _, f := range out.Files {
		if f.Content != nil {
			continue
		}
		var got []byte
		offset, parts := 0, 0
		for {
			var part mcpserver.FileOut
			res := call(t, cs, "get_employee_file", map[string]any{"id": "harnsy/big", "version": "1.0.0", "target": "generic", "path": f.Path, "offset": offset}, &part)
			if n := len(text(res)); n > mcpserver.MaxAnswer {
				t.Fatalf("%s: a part is %d bytes, at most %d", f.Path, n, mcpserver.MaxAnswer)
			}
			if part.Content == nil || part.SHA256 != f.SHA256 || part.Bytes != f.Bytes {
				t.Fatalf("%s: part %+v", f.Path, part)
			}
			got = append(got, *part.Content...)
			parts++
			if part.NextOffset == 0 {
				break
			}
			offset = part.NextOffset
		}
		if catalogtest.Sum(got) != f.SHA256 {
			t.Fatalf("%s: %d parts do not add up to the file", f.Path, parts)
		}
		if strings.HasSuffix(f.Path, "assets/a.md") && parts < 3 {
			t.Fatalf("%s came in %d parts, want it split", f.Path, parts)
		}
	}
	for name, args := range map[string]map[string]any{
		"a path not in the tree": {"id": "harnsy/big", "version": "1.0.0", "target": "generic", "path": "big/secrets.txt"},
		"an offset past the end": {"id": "harnsy/big", "version": "1.0.0", "target": "generic", "path": "big/AGENT.md", "offset": 1 << 30},
		"a version not served":   {"id": "harnsy/big", "version": "9.9.9", "target": "generic", "path": "big/AGENT.md"},
	} {
		if res := call(t, cs, "get_employee_file", args, nil); !res.IsError {
			t.Errorf("%s must be a tool error", name)
		}
	}
}
