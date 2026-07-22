package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jjuanrivvera/linkedin-cli/internal/api"
	"github.com/jjuanrivvera/linkedin-cli/internal/auth"
	"github.com/jjuanrivvera/linkedin-cli/internal/browserauth"
)

// fakeStore is an in-memory auth.Store so tests never touch a real OS keyring.
type fakeStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string]string{}} }

func (f *fakeStore) Set(profile, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[profile] = token
	return nil
}

func (f *fakeStore) Get(profile string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.data[profile]; ok && t != "" {
		return t, nil
	}
	return "", auth.ErrNotFound
}

func (f *fakeStore) Delete(profile string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, profile)
	return nil
}

func (f *fakeStore) Backend() string { return "fake" }

// env wires one test invocation: an httptest Voyager server, an isolated config dir, and a
// fake keyring seeded with a session.
type env struct {
	t     *testing.T
	srv   *httptest.Server
	store *fakeStore
	tmp   string
}

// newEnv starts a mock server (both hosts route to it) and isolates state under t.TempDir().
func newEnv(t *testing.T, handler http.HandlerFunc) *env {
	t.Helper()
	e := &env{t: t, store: newFakeStore(), tmp: t.TempDir()}
	if handler != nil {
		e.srv = httptest.NewServer(handler)
		t.Cleanup(e.srv.Close)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LINKEDIN_PROFILE", "")
	// Provide a borrowed session via env so authed requests resolve cookies.
	t.Setenv("LI_AT", "TEST_LI_AT")
	t.Setenv("JSESSIONID", `"ajax:test"`)
	t.Setenv("LINKEDIN_BASE_URL", "")
	t.Setenv("LINKEDIN_WEB_BASE_URL", "")
	t.Setenv("LINKEDIN_USER_AGENT", "")
	t.Setenv("NO_COLOR", "1")
	return e
}

func (e *env) deps() *deps {
	d := newDeps()
	d.store = func() auth.Store { return e.store }
	if e.srv != nil {
		url := e.srv.URL
		httpc := e.srv.Client()
		tmp := e.tmp
		gf := d.gf
		d.newClient = func(_, _ string, opts ...api.Option) *api.Client {
			// Force both hosts at the mock server, disable retry, and override the real
			// 3–15s pacer with a zero-delay one (fast tests, isolated daily-cap state). The
			// daily caps honor the global-flag overrides so a test can drive cap exhaustion.
			sendCap := 20
			if gf.dailySendCap > 0 {
				sendCap = gf.dailySendCap
			}
			dailyCap := 30
			if gf.dailyCap > 0 {
				dailyCap = gf.dailyCap
			}
			opts = append(opts,
				api.WithHTTPClient(httpc),
				api.WithMaxRetries(0),
				api.WithPacer(&api.Pacer{DailyCap: dailyCap, DailySendCap: sendCap, StatePath: filepath.Join(tmp, "state.json")}),
			)
			return api.NewClientWithBaseURL(url, opts...)
		}
	}
	return d
}

// run executes the real command tree with captured output.
func (e *env) run(args ...string) (string, string, error) {
	e.t.Helper()
	return runWithDeps(e.t, e.deps(), args...)
}

// runWithDeps builds a fresh tree from d and runs it with captured output.
func runWithDeps(t *testing.T, d *deps, args ...string) (string, string, error) {
	t.Helper()
	var out, errB bytes.Buffer
	d.out = &out // dry-run curls go here too, so they are captured
	root := newRootCmd(d)
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errB)
	root.SetIn(bytes.NewReader(nil)) // deterministic empty stdin for interactive prompts
	err := root.ExecuteContext(t.Context())
	return out.String(), errB.String(), err
}

// fakeExtractor installs a browserauth finder that returns a complete LinkedIn session, so
// `auth --cookie-from-browser` tests never read a real browser. Restores on cleanup.
func fakeExtractor(t *testing.T, cookies []browserauth.RawCookie) {
	t.Helper()
	orig := newExtractor
	t.Cleanup(func() { newExtractor = orig })
	newExtractor = func(browsers []string) *browserauth.Extractor {
		e := &browserauth.Extractor{Domain: linkedInDomain, Browsers: browsers, RequiredCookieNames: requiredCookies}
		return e.WithFinder(func(context.Context, string) ([]browserauth.RawCookie, error) { return cookies, nil })
	}
}
