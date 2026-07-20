package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCookies is a stable borrowed-session pair for tests (JSESSIONID keeps its quotes).
func testCookies(context.Context) (string, string, error) { return "LI_AT_X", `"ajax:99"`, nil }

// newTestClient points both hosts at one httptest server and injects fixed cookies + no retry.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClientWithBaseURL(srv.URL,
		WithHTTPClient(srv.Client()),
		WithMaxRetries(0),
		WithCookies(testCookies),
	)
}

func TestClient_HeadersAndCookies(t *testing.T) {
	var gotCookie, gotCSRF, gotAccept, gotRestli, gotUA string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotCSRF = r.Header.Get("csrf-token")
		gotAccept = r.Header.Get("Accept")
		gotRestli = r.Header.Get("x-restli-protocol-version")
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"data":{},"included":[]}`))
	})
	_, _, err := c.Do(t.Context(), "me", nil)
	require.NoError(t, err)
	assert.Equal(t, `li_at=LI_AT_X; JSESSIONID="ajax:99"`, gotCookie)
	assert.Equal(t, "ajax:99", gotCSRF, "csrf must drop the JSESSIONID quotes")
	assert.Contains(t, gotAccept, "normalized")
	assert.Equal(t, "2.0.0", gotRestli)
	assert.Contains(t, gotUA, "Chrome/")
}

func TestClient_DryRunRedactsCookies(t *testing.T) {
	var buf bytes.Buffer
	c := New("https://www.linkedin.com/voyager/api", "https://www.linkedin.com",
		WithCookies(testCookies), WithDryRun(true, &buf))
	_, _, err := c.Do(t.Context(), "me", url.Values{"x": {"1"}})
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "curl ")
	assert.Contains(t, out, "REDACTED")
	assert.NotContains(t, out, "LI_AT_X", "the session must be redacted by default")

	buf.Reset()
	c.ShowToken = true
	_, _, err = c.Do(t.Context(), "me", nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "LI_AT_X")
}

func TestClient_SoftBlockSurfaced(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(StatusSoftBlock)
		_, _ = w.Write([]byte(`{}`))
	})
	_, _, err := c.Do(t.Context(), "me", nil)
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, StatusSoftBlock, apiErr.StatusCode)
	assert.Contains(t, err.Error(), "soft-blocked")
	assert.Contains(t, err.Error(), "did NOT retry")
}

func TestClient_BaseURLAccessors(t *testing.T) {
	c := New("", "")
	assert.Equal(t, DefaultVoyagerBaseURL, c.VoyagerBaseURL())
	assert.Equal(t, DefaultWebBaseURL, c.WebBaseURL())
	assert.Nil(t, c.Pacer())
}

func TestClient_DoReturnsStatusAndBodyOnError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"gone"}`))
	})
	status, body, err := c.Do(t.Context(), "missing", nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Contains(t, string(body), "gone")
}

func TestClient_DoDryRunReturnsZero(t *testing.T) {
	var buf bytes.Buffer
	c := New("https://x/voyager/api", "https://x", WithCookies(testCookies), WithDryRun(true, &buf))
	status, _, err := c.Do(t.Context(), "me", nil)
	require.NoError(t, err)
	assert.Equal(t, 0, status)
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, `'a b'`, shellQuote("a b"))
	assert.Equal(t, `'a'\''b'`, shellQuote("a'b"))
}

func TestSortedKeys(t *testing.T) {
	got := sortedKeys(map[string]string{"c": "", "a": "", "b": ""})
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestCookieError_Propagates(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	c.cookies = func(context.Context) (string, string, error) {
		return "", "", assert.AnError
	}
	_, _, err := c.Do(t.Context(), "me", nil)
	assert.Error(t, err)
}

func TestVerboseLogging(t *testing.T) {
	var buf bytes.Buffer
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	c.Verbose = true
	c.VerboseOut = &buf
	_, _, err := c.Do(t.Context(), "me", nil)
	require.NoError(t, err)
	assert.True(t, strings.Contains(buf.String(), "GET"))
}
