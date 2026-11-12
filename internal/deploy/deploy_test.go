// Package deploy_test keeps the files a deploy rests on in step with what the server needs: the Go toolchain the
// binary is built with and the limits of the systemd unit.
package deploy_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestBuildToolchainIsNamed: the standard library is part of the binary. The go directive stays at the language floor
// the MCP SDK needs; the toolchain line names the patch release to build with, and the runbook says the same.
func TestBuildToolchainIsNamed(t *testing.T) {
	mod := read(t, "go.mod")
	if !regexp.MustCompile(`(?m)^go 1\.25\.0$`).MatchString(mod) {
		t.Error("go.mod: the go directive is the SDK's floor, 1.25.0")
	}
	m := regexp.MustCompile(`(?m)^toolchain go(1\.\d+\.\d+)$`).FindStringSubmatch(mod)
	if m == nil {
		t.Fatal("go.mod names no toolchain: a build with the floor's own standard library carries its known advisories")
	}
	runbook := read(t, "deploy/README.md")
	if !strings.Contains(runbook, "today "+m[1]) || !strings.Contains(runbook, "newest patch release") || strings.Contains(runbook, "Go 1.25 or newer") {
		t.Errorf("deploy/README.md must say to build with the newest patch release of a supported Go line, today %s", m[1])
	}
}
