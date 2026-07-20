package browserauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeFinder(cookies []RawCookie) Finder {
	return func(context.Context, string) ([]RawCookie, error) { return cookies, nil }
}

func newLinkedInExtractor(finder Finder, browsers ...string) *Extractor {
	e := &Extractor{
		Domain:              ".linkedin.com",
		Browsers:            browsers,
		RequiredCookieNames: []string{"li_at", "JSESSIONID"},
	}
	return e.WithFinder(finder)
}

func TestExtract_Complete(t *testing.T) {
	e := newLinkedInExtractor(fakeFinder([]RawCookie{
		{Name: "li_at", Value: "AQED", Browser: "chrome", Profile: "Default"},
		{Name: "JSESSIONID", Value: `"ajax:99"`, Browser: "chrome", Profile: "Default"},
		{Name: "bcookie", Value: "junk", Browser: "chrome", Profile: "Default"},
	}))
	got, src, err := e.Extract(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AQED", got["li_at"])
	assert.Equal(t, `"ajax:99"`, got["JSESSIONID"])
	assert.Equal(t, "chrome (Default)", src)
}

func TestExtract_IncompleteSource(t *testing.T) {
	// li_at from chrome, JSESSIONID from firefox — no single source is complete.
	e := newLinkedInExtractor(fakeFinder([]RawCookie{
		{Name: "li_at", Value: "AQED", Browser: "chrome", Profile: "Default"},
		{Name: "JSESSIONID", Value: `"ajax:99"`, Browser: "firefox", Profile: "default"},
	}))
	_, _, err := e.Extract(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "li_at, JSESSIONID")
}

func TestExtract_BrowserFilter(t *testing.T) {
	cookies := []RawCookie{
		{Name: "li_at", Value: "chrome-at", Browser: "chrome", Profile: "Default"},
		{Name: "JSESSIONID", Value: `"ajax:c"`, Browser: "chrome", Profile: "Default"},
		{Name: "li_at", Value: "ff-at", Browser: "firefox", Profile: "default"},
		{Name: "JSESSIONID", Value: `"ajax:f"`, Browser: "firefox", Profile: "default"},
	}
	e := newLinkedInExtractor(fakeFinder(cookies), "firefox")
	got, src, err := e.Extract(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ff-at", got["li_at"])
	assert.Contains(t, src, "firefox")
}

func TestExtract_DeterministicSourceOrder(t *testing.T) {
	// Two complete sources → the alphabetically-first browser wins, deterministically.
	cookies := []RawCookie{
		{Name: "li_at", Value: "ff", Browser: "firefox", Profile: "default"},
		{Name: "JSESSIONID", Value: `"ajax:f"`, Browser: "firefox", Profile: "default"},
		{Name: "li_at", Value: "br", Browser: "brave", Profile: "Default"},
		{Name: "JSESSIONID", Value: `"ajax:b"`, Browser: "brave", Profile: "Default"},
	}
	got, src, err := newLinkedInExtractor(fakeFinder(cookies)).Extract(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "br", got["li_at"])
	assert.Contains(t, src, "brave")
}

func TestExtract_FinderError(t *testing.T) {
	e := newLinkedInExtractor(func(context.Context, string) ([]RawCookie, error) {
		return nil, assert.AnError
	})
	_, _, err := e.Extract(context.Background())
	assert.Error(t, err)
}

func TestExtract_NoneFound(t *testing.T) {
	_, _, err := newLinkedInExtractor(fakeFinder(nil)).Extract(context.Background())
	assert.Error(t, err)
}

func TestBrowserAllowed(t *testing.T) {
	e := &Extractor{Browsers: []string{"chrome"}}
	assert.True(t, e.browserAllowed("chrome"))
	assert.True(t, e.browserAllowed("Chrome"))
	assert.False(t, e.browserAllowed("firefox"))
	e2 := &Extractor{}
	assert.True(t, e2.browserAllowed("anything"))
}
