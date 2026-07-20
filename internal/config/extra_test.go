package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_UsesXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, c.Profiles)
	assert.NotEmpty(t, c.FilePath())
}

func TestProfileNames(t *testing.T) {
	c := &Config{Profiles: map[string]Profile{"a": {}, "b": {}}}
	assert.ElementsMatch(t, []string{"a", "b"}, c.ProfileNames())
}

func TestProfile_Getter(t *testing.T) {
	c := &Config{Profiles: map[string]Profile{"x": {HasCookies: true}}}
	p, ok := c.Profile("x")
	assert.True(t, ok)
	assert.True(t, p.HasCookies)
	_, ok = c.Profile("nope")
	assert.False(t, ok)
}

func TestSetProfile_InvalidName(t *testing.T) {
	c := &Config{}
	assert.Error(t, c.SetProfile("a/b", Profile{}))
}

func TestSave_DefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := &Config{Profiles: map[string]Profile{}}
	require.NoError(t, c.SetProfile("default", Profile{}))
	require.NoError(t, c.Save())
	assert.Equal(t, "config.yaml", filepath.Base(c.FilePath()))
}
