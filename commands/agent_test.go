package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyTree_AllReads(t *testing.T) {
	// This is a read-only CLI: every API command (jobs/company/geo/api) classifies as read;
	// no writes or destructive.
	cls := classifyAPICommands(false)
	assert.NotEmpty(t, cls.Read)
	assert.Empty(t, cls.Write)
	assert.Empty(t, cls.Destructive)

	paths := map[string]bool{}
	for _, c := range cls.Read {
		paths[c.Path] = true
	}
	for _, want := range []string{"jobs search", "jobs get", "company get", "geo", "api"} {
		assert.True(t, paths[want], want)
	}
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
	// alias set is denied; reads are allowed.
	assert.Contains(t, out, "Bash(linkedin alias set:*)")
	assert.Contains(t, out, "Bash(linkedin jobs search:*)")
}

func TestGuard_Codex(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("agent", "guard", "--host", "codex")
	require.NoError(t, err)
	assert.Contains(t, out, "sandbox_mode = \"read-only\"")
	assert.Contains(t, out, "no irreversible operations")
}

func TestGuard_OpenCode(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("agent", "guard", "--host", "opencode")
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &cfg))
	perm := cfg["permission"].(map[string]any)["bash"].(map[string]any)
	assert.Equal(t, "deny", perm["linkedin alias set*"])
	assert.Equal(t, "allow", perm["linkedin jobs search*"])
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
