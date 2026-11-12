package mcpserver_test

import (
	"strings"
	"testing"

	"github.com/harnesy/hr-mcp/internal/mcpserver"
)

func TestInstallHint(t *testing.T) {
	cs := connect(t, stand(t))
	var out mcpserver.HintOut
	call(t, cs, "install_hint", map[string]any{"client": "claude-code", "id": "harnsy/sample-skills"}, &out)
	all := strings.Join(out.Steps, "\n")
	for _, want := range []string{`"target": "claude-code"`, ".claude/agents/sample-skills.md", "claude --agent sample-skills", "sha256"} {
		if !strings.Contains(all, want) {
			t.Errorf("the steps lack %q:\n%s", want, all)
		}
	}
	if out.Client != "claude-code" || out.Target != "claude-code" ||
		out.AddServer != "claude mcp add --transport http hr https://hr.harnsy.dev/mcp" {
		t.Fatalf("client %q target %q add %q", out.Client, out.Target, out.AddServer)
	}
	if !strings.Contains(out.SignedInstall, "hr-openclaw") || !strings.Contains(out.SignedInstall, "hr-hermes") || !strings.Contains(out.SignedInstall, "harnsy") {
		t.Fatalf("the hint must name the installs that check the signature: %q", out.SignedInstall)
	}

	call(t, cs, "install_hint", map[string]any{"client": "Some New IDE"}, &out)
	if out.Client != "generic" || out.Target != "generic" || len(out.KnownClients) < 4 || !strings.Contains(strings.Join(out.Steps, "\n"), "<name>/AGENT.md") {
		t.Fatalf("an unknown client gets the generic steps and the list of known ones: %+v", out)
	}
	if res := call(t, cs, "install_hint", map[string]any{"client": "codex", "id": "harnsy/nobody"}, nil); !res.IsError {
		t.Fatal("an unknown employee must be a tool error")
	}
}
