package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/linkedin-cli/internal/update"
)

func TestUpdate_DevBuildNoOp(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("update")
	require.NoError(t, err)
	// version.Version is "dev" in tests → self-update is disabled.
	assert.Contains(t, out, "latest version")
}

func TestUpdateCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer srv.Close()
	orig := newUpdater
	t.Cleanup(func() { newUpdater = orig })
	newUpdater = func() *update.Updater { return update.NewUpdaterWithBaseURL("dev", srv.URL) }

	e := newEnv(t, nil)
	out, _, err := e.run("update", "check")
	require.NoError(t, err)
	assert.Contains(t, out, "Latest:")
	assert.Contains(t, out, "9.9.9")
}

func TestVersionCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0"}`))
	}))
	defer srv.Close()
	orig := latestReleaseURL
	t.Cleanup(func() { latestReleaseURL = orig })
	latestReleaseURL = srv.URL

	e := newEnv(t, nil)
	out, _, err := e.run("version", "--check")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestAgentGuard_WriteFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newEnv(t, nil)
	_, _, err := e.run("agent", "guard", "--host", "claude-code", "--write")
	require.NoError(t, err)
	hook := filepath.Join(".claude", "hooks", "linkedin-guard.sh")
	_, statErr := os.Stat(hook)
	require.NoError(t, statErr)

	// A second run must refuse to overwrite.
	_, _, err = e.run("agent", "guard", "--host", "claude-code", "--write")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAgentGuard_OutFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "codex.toml")
	e := newEnv(t, nil)
	_, _, err := e.run("agent", "guard", "--host", "codex", "--out", outPath)
	require.NoError(t, err)
	b, err := os.ReadFile(outPath) //nolint:gosec // test-controlled path
	require.NoError(t, err)
	assert.Contains(t, string(b), "read-only")
}

func TestDirOf(t *testing.T) {
	assert.Equal(t, ".claude/hooks", dirOf(".claude/hooks/x.sh"))
	assert.Equal(t, ".", dirOf("x.sh"))
}

func TestMCPCommandRegistered(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "mcp" {
			found = true
		}
	}
	assert.True(t, found)
}
