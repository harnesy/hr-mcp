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

	call(t, cs, "install_hint", map[string]any{"client": "OpenClaw", "id": "harnsy/sample-skills"}, &out)
	all = strings.Join(out.Steps, "\n")
	for _, want := range []string{`"target": "openclaw"`, "workspace-sample-skills/", "state directory",
		"openclaw agents add sample-skills --workspace", "--non-interactive", "openclaw agent --agent sample-skills", "SOUL.md", "AGENTS.md"} {
		if !strings.Contains(all, want) {
			t.Errorf("the OpenClaw steps lack %q:\n%s", want, all)
		}
	}
	if out.Client != "openclaw" || out.Target != "openclaw" || !strings.Contains(out.AddServer, `"transport":"streamable-http"`) {
		t.Fatalf("client %q target %q add %q", out.Client, out.Target, out.AddServer)
	}

	call(t, cs, "install_hint", map[string]any{"client": "hermes", "id": "harnsy/sample-skills"}, &out)
	all = strings.Join(out.Steps, "\n")
	for _, want := range []string{`"target": "hermes"`, "sample-skills/", "distribution.yaml", "hermes profile install",
		"--name sample-skills", "hermes -p sample-skills chat", "SOUL.md", "model"} {
		if !strings.Contains(all, want) {
			t.Errorf("the Hermes steps lack %q:\n%s", want, all)
		}
	}
	if out.Client != "hermes" || out.Target != "hermes" || !strings.Contains(out.AddServer, "mcp_servers") {
		t.Fatalf("client %q target %q add %q", out.Client, out.Target, out.AddServer)
	}

	call(t, cs, "install_hint", map[string]any{"client": "Some New IDE"}, &out)
	if out.Client != "generic" || out.Target != "generic" || len(out.KnownClients) < 4 || !strings.Contains(strings.Join(out.Steps, "\n"), "<name>/AGENT.md") {
		t.Fatalf("an unknown client gets the generic steps and the list of known ones: %+v", out)
	}
	if res := call(t, cs, "install_hint", map[string]any{"client": "codex", "id": "harnsy/nobody"}, nil); !res.IsError {
		t.Fatal("an unknown employee must be a tool error")
	}
}

// TestInstallHintStopsBeforeReplacingAFile: where an employee lands among a person's own files (a project, OpenClaw's
// state directory), the steps say to stop and ask when something is already there. A skill or a subagent of the same
// name may be the person's own.
func TestInstallHintStopsBeforeReplacingAFile(t *testing.T) {
	cs := connect(t, stand(t))
	for _, client := range []string{"claude-code", "codex", "generic", "openclaw", "Some New IDE"} {
		var out mcpserver.HintOut
		call(t, cs, "install_hint", map[string]any{"client": client, "id": "harnsy/sample-skills"}, &out)
		if all := strings.Join(out.Steps, "\n"); !strings.Contains(all, "already there") || !strings.Contains(all, "stop and ask the person") {
			t.Errorf("%s: the steps do not say to stop when a file or folder is already there:\n%s", client, all)
		}
	}
}
