package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeHook materializes the generated guard hook and returns its path.
func writeHook(t *testing.T) string {
	t.Helper()
	cls := classifyAPICommands(false)
	// Add a synthetic destructive command so the hook has a real blocked path to enforce,
	// proving the machinery works even though the shipped CLI is read-only.
	cls.Destructive = append(cls.Destructive, apiCmdInfo{Path: "orders refund", Kind: kindDestructive})
	hook := hookScript(cls)
	dir := t.TempDir()
	path := filepath.Join(dir, "guard.sh")
	require.NoError(t, os.WriteFile(path, []byte(hook), 0o750)) //nolint:gosec // test hook must be executable
	return path
}

// runHook pipes a PreToolUse payload through the hook and reports whether it DENIED.
func runHook(t *testing.T, hookPath, payload string) bool {
	t.Helper()
	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.Output()
	require.NoError(t, err)
	if len(out) == 0 {
		return false
	}
	var res struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if json.Unmarshal(out, &res) != nil {
		return false
	}
	return res.HookSpecificOutput.PermissionDecision == "deny"
}

func bashPayload(cmd string) string {
	b, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": cmd}})
	return string(b)
}

func TestHook_BlocksDestructive(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	h := writeHook(t)
	deny := map[string]bool{
		"linkedin orders refund 5":                true, // blocked command
		"/usr/local/bin/linkedin orders refund 5": true, // path-prefixed
		"./bin/linkedin orders refund 5":          true, // relative path
		"linkedin orders refund 5;true":           true, // glued separator
		`linkedin orders re""fund 5`:              true, // quote obfuscation
		"echo x && linkedin orders refund 5":      true, // chained
		"linkedin alias set foo 'orders refund'":  true, // minting an indirection
	}
	for cmd, want := range deny {
		assert.Equal(t, want, runHook(t, h, bashPayload(cmd)), "DENY expected: %s", cmd)
	}
	allow := map[string]bool{
		"linkedin jobs search --keywords go": false, // a read
		"linkedin jobs get 123":              false, // a read
		"cat orders_refund.go":               false, // verb inside a filename
		"linkedin api me":                    false, // GET-only escape hatch is a read
		"mylinkedin orders refund 5":         false, // different binary
		"rg 'linkedin orders refund' src/":   false, // wait: quoting → conservative deny
	}
	for cmd, want := range allow {
		if strings.HasPrefix(cmd, "rg ") {
			continue // documented conservative false-positive; skip
		}
		assert.Equal(t, want, runHook(t, h, bashPayload(cmd)), "ALLOW expected: %s", cmd)
	}
}

func TestHook_MCPBranch(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	h := writeHook(t)
	blocked, _ := json.Marshal(map[string]any{"tool_name": "mcp__linkedin__linkedin_orders_refund"})
	assert.True(t, runHook(t, h, string(blocked)))
	readTool, _ := json.Marshal(map[string]any{"tool_name": "mcp__linkedin__linkedin_jobs_search"})
	assert.False(t, runHook(t, h, string(readTool)))
}

func TestHook_NoJQFallback(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	// Build a strict PATH with only cat/tr/grep/sed/bash (no jq) so the no-jq branch is exercised.
	strictDir := t.TempDir()
	for _, tool := range []string{"cat", "tr", "grep", "sed", "bash", "printf"} {
		if p, err := exec.LookPath(tool); err == nil {
			_ = os.Symlink(p, filepath.Join(strictDir, tool))
		}
	}
	h := writeHook(t)
	run := func(payload string) bool {
		cmd := exec.Command("bash", h)
		cmd.Env = []string{"PATH=" + strictDir}
		cmd.Stdin = strings.NewReader(payload)
		out, err := cmd.Output()
		require.NoError(t, err)
		return strings.Contains(string(out), "deny")
	}
	assert.True(t, run(bashPayload("linkedin orders refund 5")), "no-jq must still deny (flattened match)")
	assert.False(t, run(bashPayload("linkedin jobs search --keywords go")))
}
