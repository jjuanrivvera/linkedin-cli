package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	c, err := LoadFrom(p)
	require.NoError(t, err)
	require.NoError(t, c.SetProfile("work", Profile{VoyagerBaseURL: "https://linkedin.ai/api", HasCookies: true}))
	c.CurrentProfile = "work"
	c.Aliases = map[string]string{"go": "jobs search --skill golang"}
	require.NoError(t, c.Save())

	// File perms must be 0600.
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	got, err := LoadFrom(p)
	require.NoError(t, err)
	assert.Equal(t, "work", got.CurrentProfile)
	prof, ok := got.Profile("work")
	require.True(t, ok)
	assert.True(t, prof.HasCookies)
	assert.Equal(t, "jobs search --skill golang", got.Aliases["go"])
}

func TestLoadFrom_Missing(t *testing.T) {
	c, err := LoadFrom(filepath.Join(t.TempDir(), "none.yaml"))
	require.NoError(t, err)
	assert.NotNil(t, c.Profiles)
}

func TestResolveProfileName_Precedence(t *testing.T) {
	c := &Config{CurrentProfile: "cfg"}
	assert.Equal(t, "flag", c.ResolveProfileName("flag"))
	t.Setenv("LINKEDIN_PROFILE", "env")
	assert.Equal(t, "env", c.ResolveProfileName(""))
	os.Unsetenv("LINKEDIN_PROFILE")
	assert.Equal(t, "cfg", c.ResolveProfileName(""))
	empty := &Config{}
	assert.Equal(t, DefaultProfile, empty.ResolveProfileName(""))
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "b", FirstNonEmpty("", "b", "c"))
	assert.Equal(t, "", FirstNonEmpty("", ""))
}

func TestValidateProfileName(t *testing.T) {
	require.NoError(t, ValidateProfileName("work"))
	for _, bad := range []string{"", ".", "..", "a/b", "a:b", "a#b"} {
		assert.Error(t, ValidateProfileName(bad), bad)
	}
}

func TestValidateBaseURL(t *testing.T) {
	require.NoError(t, ValidateBaseURL("https://linkedin.ai/api"))
	require.NoError(t, ValidateBaseURL("http://localhost:8080"))
	require.NoError(t, ValidateBaseURL("http://127.0.0.1"))
	assert.Error(t, ValidateBaseURL("ftp://x"))
	assert.Error(t, ValidateBaseURL("https://"))
	assert.Error(t, ValidateBaseURL("http://linkedin.ai"), "cleartext non-loopback must be rejected")
}

func TestSetProfile_ValidatesURLs(t *testing.T) {
	c := &Config{}
	assert.Error(t, c.SetProfile("x", Profile{VoyagerBaseURL: "http://linkedin.ai"}))
	assert.NoError(t, c.SetProfile("x", Profile{VoyagerBaseURL: "https://linkedin.ai/api"}))
}

func TestDirAndPath_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "linkedin", filepath.Base(dir))
	p, err := Path()
	require.NoError(t, err)
	assert.Equal(t, "config.yaml", filepath.Base(p))
}
