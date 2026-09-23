package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyTree_ReadFirst(t *testing.T) {
	// Read-first CLI: every API command (jobs/company/geo/messages list|read/api) classifies
	// as read; the ONE write, `messages send`, classifies as DESTRUCTIVE (irreversible + the
	// classic account-restriction trigger) so every guard hard-blocks it.
	cls := classifyAPICommands(false)
	assert.NotEmpty(t, cls.Read)
	assert.Empty(t, cls.Write)

	paths := map[string]bool{}
	for _, c := range cls.Read {
		paths[c.Path] = true
	}
	for _, want := range []string{"jobs search", "jobs get", "company get", "geo", "api",
		"messages list", "messages read"} {
		assert.True(t, paths[want], want)
	}

	require.Len(t, cls.Destructive, 1)
	assert.Equal(t, "messages send", cls.Destructive[0].Path)
	assert.Contains(t, cls.Destructive[0].AllPaths(), "message send", "cobra alias path is guarded too")
}

// TestMessagesSend_GuardFailClosed pins that `messages send` can never fall through as
// harmless: its own annotation classifies it destructive, and even with the annotation
// stripped the fail-closed default classifies an unannotated leaf destructive.
func TestMessagesSend_GuardFailClosed(t *testing.T) {
	root := NewRootCmd()
	send, _, err := root.Find([]string{"messages", "send"})
	require.NoError(t, err)
	assert.Equal(t, kindDestructive, kindOf(send), "annotated destructive")
	send.Annotations = nil
	assert.Equal(t, kindDestructive, kindOf(send), "unannotated ⇒ destructive (fail-closed)")
}

func TestEveryAPICommandIsAnnotated(t *testing.T) {
	// Any API-backed leaf missing an annotation would classify DESTRUCTIVE (fail-closed). Walk
	// the tree and assert every non-local leaf carries a read/write/destructive annotation.
	var walk func(cmd *cobra.Command, top string)
	walk = func(cmd *cobra.Command, top string) {
		for _, child := range cmd.Commands() {
			name := top
			if name == "" {
				name = child.Name()
			}
			isLocal := false
			for _, g := range localGroups {
				if name == g {
					isLocal = true
				}
			}
			if child.Runnable() && !isLocal {
				a := child.Annotations
				has := a[annReadOnly] == "true" || a[annOpenWorld] == "true" || a[annDestructive] == "true"
				assert.True(t, has, "unannotated API command: %s", child.Name())
			}
			walk(child, name)
		}
	}
	walk(NewRootCmd(), "")
}

func TestGuard_ClaudeCode(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("agent", "guard", "--host", "claude-code")
	require.NoError(t, err)
	assert.Contains(t, out, "linkedin-guard.sh")
	// messages send (incl. its MCP tool + alias path) and alias set are denied; reads allowed.
	assert.Contains(t, out, "Bash(linkedin messages send:*)")
	assert.Contains(t, out, "Bash(linkedin message send:*)")
	assert.Contains(t, out, "mcp__linkedin__linkedin_messages_send")
	assert.Contains(t, out, "Bash(linkedin alias set:*)")
	assert.Contains(t, out, "Bash(linkedin jobs search:*)")
	assert.Contains(t, out, "Bash(linkedin messages list:*)")
}

func TestGuard_Codex(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("agent", "guard", "--host", "codex")
	require.NoError(t, err)
	assert.Contains(t, out, "sandbox_mode = \"read-only\"")
	assert.Contains(t, out, "Never auto-approve these irreversible linkedin operations")
	assert.Contains(t, out, "linkedin messages send")
}

func TestGuard_OpenCode(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("agent", "guard", "--host", "opencode")
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &cfg))
	perm := cfg["permission"].(map[string]any)["bash"].(map[string]any)
	assert.Equal(t, "deny", perm["linkedin alias set*"])
	assert.Equal(t, "deny", perm["linkedin messages send*"])
	assert.Equal(t, "deny", perm["linkedin message send*"], "alias path denied too")
	assert.Equal(t, "allow", perm["linkedin jobs search*"])
	assert.Equal(t, "allow", perm["linkedin messages list*"])
}

func TestGuard_UnknownHost(t *testing.T) {
	e := newEnv(t, nil)
	_, _, err := e.run("agent", "guard", "--host", "nope")
	require.Error(t, err)
}

func TestMCPExcludesSetupCommands(t *testing.T) {
	root := NewRootCmd()
	for _, name := range excludedMCPPaths {
		var found *cobra.Command
		for _, c := range root.Commands() {
			if c.Name() == name {
				found = c
			}
		}
		if found != nil {
			assert.True(t, mcpExcluded(found), "%s should be MCP-excluded", name)
		}
	}
	// A resource command must NOT be excluded.
	for _, c := range root.Commands() {
		if c.Name() == "jobs" {
			assert.False(t, mcpExcluded(c))
		}
	}
}

func TestMCPSecretFlagsAreRedacted(t *testing.T) {
	for _, f := range []string{"show-token", "profile", "base-url", "web-base-url"} {
		assert.Contains(t, secretFlags, f)
	}
}

func TestBashPatternAndTool(t *testing.T) {
	assert.Equal(t, "Bash(linkedin jobs search:*)", bashPattern("jobs search"))
	assert.Equal(t, "mcp__linkedin__linkedin_jobs_search", mcpToolPattern("jobs search"))
}

func TestGuardText_NoMethodGating(t *testing.T) {
	// The read-only api escape hatch must NOT be method-gated in the emitted config.
	cls := classifyAPICommands(false)
	hook := hookScript(cls)
	assert.NotContains(t, hook, "api_is_blocked")
	assert.True(t, strings.Contains(hook, "alias set"))
}
