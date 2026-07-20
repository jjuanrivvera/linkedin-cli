package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/linkedin-cli/internal/browserauth"
)

func TestAuth_CookieFromBrowser(t *testing.T) {
	e := newEnv(t, nil)
	fakeExtractor(t, []browserauth.RawCookie{
		{Name: "li_at", Value: "AQED", Browser: "chrome", Profile: "Default"},
		{Name: "JSESSIONID", Value: `"ajax:99"`, Browser: "chrome", Profile: "Default"},
	})
	out, _, err := e.run("auth", "--cookie-from-browser", "chrome")
	require.NoError(t, err)
	assert.Contains(t, out, "Stored a LinkedIn session")
	// The credential must be stored as a JSON pair.
	raw, _ := e.store.Get("default")
	assert.Contains(t, raw, "li_at")
	assert.Contains(t, raw, "AQED")
}

func TestAuth_Manual(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("auth", "--li-at", "AQED", "--jsessionid", `"ajax:1"`)
	require.NoError(t, err)
	assert.Contains(t, out, "Stored a LinkedIn session")
}

func TestAuth_MissingArgs(t *testing.T) {
	e := newEnv(t, nil)
	_, _, err := e.run("auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cookie-from-browser")
}

func TestAuth_StatusAndLogout(t *testing.T) {
	e := newEnv(t, nil)
	require.NoError(t, e.store.Set("default", `{"li_at":"a","JSESSIONID":"\"ajax:1\""}`))
	out, _, err := e.run("auth", "status")
	require.NoError(t, err)
	assert.Contains(t, out, "session: stored")

	out, _, err = e.run("auth", "logout")
	require.NoError(t, err)
	assert.Contains(t, out, "Removed")
	_, gerr := e.store.Get("default")
	assert.Error(t, gerr)
}

func TestAuth_StatusNoSession(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("auth", "status")
	require.NoError(t, err)
	assert.Contains(t, out, "session: none")
}

func TestDoctor_Local(t *testing.T) {
	e := newEnv(t, nil)
	require.NoError(t, e.store.Set("default", `{"li_at":"a","JSESSIONID":"\"ajax:1\""}`))
	out, _, err := e.run("doctor")
	require.NoError(t, err)
	assert.Contains(t, out, "session")
	assert.Contains(t, out, "daily-budget")
	assert.Contains(t, out, "skipped")
}

func TestDoctor_LiveJSON(t *testing.T) {
	e := newEnv(t, router())
	require.NoError(t, e.store.Set("default", `{"li_at":"a","JSESSIONID":"\"ajax:1\""}`))
	out, _, err := e.run("doctor", "--live", "--json")
	require.NoError(t, err)
	assert.Contains(t, out, "connectivity")
	assert.Contains(t, out, "geo typeahead OK")
}

func TestInit_NonInteractiveSkips(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "UNOFFICIAL")
}

func TestInit_WithBrowser(t *testing.T) {
	e := newEnv(t, nil)
	fakeExtractor(t, []browserauth.RawCookie{
		{Name: "li_at", Value: "AQED", Browser: "chrome", Profile: "Default"},
		{Name: "JSESSIONID", Value: `"ajax:99"`, Browser: "chrome", Profile: "Default"},
	})
	out, _, err := e.run("init", "--cookie-from-browser", "chrome")
	require.NoError(t, err)
	assert.Contains(t, out, "Stored a LinkedIn session")
}

func TestConfig_PathViewSet(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("config", "path")
	require.NoError(t, err)
	assert.Contains(t, out, "config.yaml")

	_, _, err = e.run("config", "set", "voyager_base_url", "https://www.linkedin.com/voyager/api")
	require.NoError(t, err)

	out, _, err = e.run("config", "view")
	require.NoError(t, err)
	assert.Contains(t, out, "voyager_base_url")
}

func TestConfig_UseAndListProfiles(t *testing.T) {
	e := newEnv(t, nil)
	// Create the profile, then select it as default, then list.
	_, _, err := e.run("config", "set", "voyager_base_url", "https://www.linkedin.com/voyager/api", "--profile", "work")
	require.NoError(t, err)
	_, _, err = e.run("config", "use", "work")
	require.NoError(t, err)
	out, _, err := e.run("config", "list-profiles")
	require.NoError(t, err)
	assert.Contains(t, out, "work")
}

func TestVersion(t *testing.T) {
	e := newEnv(t, nil)
	out, _, err := e.run("version")
	require.NoError(t, err)
	assert.Contains(t, out, "linkedin")

	out, _, err = e.run("version", "--json")
	require.NoError(t, err)
	assert.Contains(t, out, `"version"`)
}

func TestCompletion(t *testing.T) {
	e := newEnv(t, nil)
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		out, _, err := e.run("completion", sh)
		require.NoError(t, err, sh)
		assert.NotEmpty(t, out)
	}
}

func TestAlias_SetListRemove(t *testing.T) {
	e := newEnv(t, nil)
	_, _, err := e.run("alias", "set", "remote-go", "jobs search --keywords go --remote")
	require.NoError(t, err)
	out, _, err := e.run("alias", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "remote-go")

	assert.Equal(t, []string{"jobs", "search", "--keywords", "go", "--remote", "-o", "json"},
		ExpandAliases([]string{"remote-go", "-o", "json"}))

	_, _, err = e.run("alias", "remove", "remote-go")
	require.NoError(t, err)
}

func TestAlias_CannotShadowBuiltin(t *testing.T) {
	e := newEnv(t, nil)
	_, _, err := e.run("alias", "set", "jobs", "geo x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
}

func TestUnknownOutputFormat(t *testing.T) {
	e := newEnv(t, nil)
	_, _, err := e.run("jobs", "search", "-o", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown output format")
}

func TestExpandAliases_Passthrough(t *testing.T) {
	assert.Equal(t, []string{"jobs"}, ExpandAliases([]string{"jobs"}))
	assert.Equal(t, []string(nil), ExpandAliases(nil))
}

func TestPromptHelpersExistInTree(t *testing.T) {
	// Ensure the root builds and every subcommand has a Short description (help hygiene).
	root := NewRootCmd()
	for _, c := range root.Commands() {
		assert.NotEmpty(t, c.Short, c.Name())
		assert.False(t, strings.Contains(c.Short, "TODO"), c.Name())
	}
}
