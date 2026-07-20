package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIError_Hints(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusUnauthorized, "cookie-from-browser"},
		{http.StatusForbidden, "stale"},
		{http.StatusNotFound, "company slug"},
		{http.StatusTooManyRequests, "did NOT auto-retry"},
		{http.StatusBadRequest, "schema.go"},
		{StatusSoftBlock, "soft-blocked"},
		{500, "server error"},
	}
	for _, c := range cases {
		e := parseAPIError(c.status, []byte(`{}`), http.Header{})
		assert.Contains(t, e.Error(), c.want, c.status)
	}
}

func TestAPIError_Challenge(t *testing.T) {
	h := http.Header{}
	h.Set("Location", "https://www.linkedin.com/checkpoint/challenge/")
	e := parseAPIError(http.StatusForbidden, []byte(`{}`), h)
	assert.True(t, e.Challenge)
	assert.Contains(t, e.Error(), "security challenge")
}

func TestAPIError_MessageFromBody(t *testing.T) {
	e := parseAPIError(400, []byte(`{"message":"bad decoration"}`), http.Header{})
	assert.Contains(t, e.Message, "bad decoration")
}

func TestContainsAny(t *testing.T) {
	assert.True(t, containsAny("a CHECKPOINT here", "checkpoint"))
	assert.False(t, containsAny("nothing", "challenge"))
	assert.True(t, containsAny("xCHALLENGEy", "foo", "challenge"))
}

func TestRetry_NoRetryOn429(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := NewClientWithBaseURL(srv.URL, WithHTTPClient(srv.Client()), WithMaxRetries(3), WithCookies(testCookies))
	_, _, err := c.Do(t.Context(), "me", nil)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "429 must not be retried (ban-safety)")
}

func TestRetry_NoRetryOnSoftBlock(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(StatusSoftBlock)
	}))
	defer srv.Close()
	c := NewClientWithBaseURL(srv.URL, WithHTTPClient(srv.Client()), WithMaxRetries(3), WithCookies(testCookies))
	_, _, err := c.Do(t.Context(), "me", nil)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "999 must not be retried")
}

func TestRetry_RetriesOn503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"data":{},"included":[]}`))
	}))
	defer srv.Close()
	c := NewClientWithBaseURL(srv.URL, WithHTTPClient(srv.Client()), WithMaxRetries(2), WithCookies(testCookies))
	_, _, err := c.Do(t.Context(), "me", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "genuine 5xx should retry")
}

func TestIsRetryableStatus(t *testing.T) {
	assert.False(t, isRetryableStatus(http.StatusTooManyRequests))
	assert.False(t, isRetryableStatus(StatusSoftBlock))
	assert.True(t, isRetryableStatus(500))
	assert.True(t, isRetryableStatus(503))
	assert.False(t, isRetryableStatus(404))
}

func TestRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "5")
	assert.Equal(t, int64(5), int64(retryAfter(h).Seconds()))
	assert.Equal(t, int64(0), int64(retryAfter(http.Header{})))
}
