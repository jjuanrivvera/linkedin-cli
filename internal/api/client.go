// Package api is the LinkedIn Voyager client core. Voyager is LinkedIn's INTERNAL, unofficial
// web API (the same endpoints linkedin.com's SPA calls). Auth is a borrowed browser session: the
// li_at session cookie + the JSESSIONID cookie, with a csrf-token header derived from JSESSIONID.
// Plain net/http is sufficient for read Voyager — no TLS-fingerprint spoofing. The client sends
// the exact header set the web client sends, paces requests for ban-safety, and never retries a
// throttle signal.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// Default hosts. Both are overridable (config/flags, and tests point them at one httptest
// server that routes by path).
const (
	// DefaultVoyagerBaseURL is the authenticated internal API base.
	DefaultVoyagerBaseURL = "https://www.linkedin.com/voyager/api"
	// DefaultWebBaseURL is the host for the unauthenticated jobs-guest geo typeahead.
	DefaultWebBaseURL = "https://www.linkedin.com"

	// DefaultUserAgent is a CURRENT desktop Chrome UA. Voyager rejects a bare Go UA, and a
	// STALE UA is itself a fingerprint that looks automated — keep this single constant fresh.
	// Overridable via LINKEDIN_USER_AGENT.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"
)

// CookieFunc supplies the borrowed session cookies per request: li_at (session) and JSESSIONID
// (whose value looks like `"ajax:1234567890"` WITH the surrounding quotes). It may be nil for the
// unauthenticated typeahead path.
type CookieFunc func(ctx context.Context) (liAt, jsessionID string, err error)

// Client is a LinkedIn Voyager HTTP client.
type Client struct {
	voyagerBase string
	webBase     string
	cookies     CookieFunc
	httpc       *http.Client
	userAgent   string
	pacer       *Pacer

	// DryRun prints the equivalent curl to DryRunOut instead of sending the request.
	DryRun    bool
	DryRunOut io.Writer
	// ShowToken reveals the session cookies in dry-run output (redacted by default).
	ShowToken bool

	Verbose    bool
	VerboseOut io.Writer

	maxRetries int
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the HTTP transport (tests point it at httptest servers).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpc = h } }

// WithDryRun enables curl-printing mode.
func WithDryRun(dry bool, out io.Writer) Option {
	return func(c *Client) { c.DryRun = dry; c.DryRunOut = out }
}

// WithCookies sets the borrowed-session cookie source.
func WithCookies(f CookieFunc) Option { return func(c *Client) { c.cookies = f } }

// WithUserAgent overrides the default User-Agent (empty keeps the default).
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithPacer installs the ban-safety pacer. Without one, requests are not paced (tests default to
// no pacer for speed).
func WithPacer(p *Pacer) Option { return func(c *Client) { c.pacer = p } }

// WithMaxRetries overrides the transient-error retry budget (tests set 0 for speed).
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// New builds a Voyager client. Empty bases fall back to the defaults.
func New(voyagerBase, webBase string, opts ...Option) *Client {
	if voyagerBase == "" {
		voyagerBase = DefaultVoyagerBaseURL
	}
	if webBase == "" {
		webBase = DefaultWebBaseURL
	}
	c := &Client{
		voyagerBase: strings.TrimRight(voyagerBase, "/"),
		webBase:     strings.TrimRight(webBase, "/"),
		httpc:       &http.Client{Timeout: 30 * time.Second},
		userAgent:   DefaultUserAgent,
		DryRunOut:   os.Stdout,
		VerboseOut:  os.Stderr,
		maxRetries:  2,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewClientWithBaseURL points both hosts at the same base URL. Tests drive every endpoint against
// one httptest server (routing by path); it also backs a single-host --base-url override.
func NewClientWithBaseURL(base string, opts ...Option) *Client {
	return New(base, base, opts...)
}

// VoyagerBaseURL returns the resolved Voyager host.
func (c *Client) VoyagerBaseURL() string { return c.voyagerBase }

// WebBaseURL returns the resolved web (typeahead) host.
func (c *Client) WebBaseURL() string { return c.webBase }

// Pacer returns the installed pacer (may be nil).
func (c *Client) Pacer() *Pacer { return c.pacer }

// getVoyager GETs a Voyager path (relative to the voyager base) with an already-assembled RAW
// query string (see jobs.go — the Rest.li query blob must keep its structural chars literal, so
// it is NOT built from url.Values). Returns the raw JSON body.
func (c *Client) getVoyager(ctx context.Context, path, rawQuery string) (json.RawMessage, error) {
	u := c.voyagerBase + "/" + strings.TrimLeft(path, "/")
	if rawQuery != "" {
		u += "?" + rawQuery
	}
	return c.get(ctx, u, true)
}

// getWeb GETs a web-host path (the unauthenticated typeahead). q is encoded normally.
func (c *Client) getWeb(ctx context.Context, path string, q url.Values) (json.RawMessage, error) {
	u := c.webBase + "/" + strings.TrimLeft(path, "/")
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return c.get(ctx, u, false)
}

// get is the shared request path: it paces (ban-safety), applies the Voyager header set +
// borrowed-session cookies (when authed), retries only genuine transient failures, and supports
// dry-run. authed selects whether cookies/csrf are attached.
func (c *Client) get(ctx context.Context, fullURL string, authed bool) (json.RawMessage, error) {
	headers := map[string]string{
		"User-Agent": c.userAgent,
		"Accept":     voyager.AcceptNormalized,
	}
	liAt, jsession := "", ""
	if authed {
		headers["x-restli-protocol-version"] = voyager.RestliProtoVersion
		headers["x-li-lang"] = voyager.LiLang
		if c.cookies != nil {
			a, j, err := c.cookies(ctx)
			if err != nil {
				return nil, err
			}
			liAt, jsession = a, j
		}
	}

	if c.DryRun {
		c.printCurl(fullURL, headers, liAt, jsession)
		return nil, nil
	}

	// Ban-safety pacing: block for the jittered inter-request delay BEFORE sending. Never in
	// dry-run (above). A cancelled context (Ctrl-C) aborts the wait.
	if c.pacer != nil {
		if err := c.pacer.Wait(ctx); err != nil {
			return nil, err
		}
	}

	send := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if authed && liAt != "" {
			// The Cookie header keeps JSESSIONID's surrounding quotes; the csrf-token header
			// uses the value with the quotes stripped.
			req.Header.Set("Cookie", fmt.Sprintf("li_at=%s; JSESSIONID=%s", liAt, jsession))
			req.Header.Set("csrf-token", strings.Trim(jsession, `"`))
		}
		if c.Verbose {
			fmt.Fprintf(c.VerboseOut, "> GET %s\n", fullURL)
		}
		return c.httpc.Do(req)
	}

	resp, err := c.sendWithRetry(ctx, http.MethodGet, send)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if c.Verbose {
		fmt.Fprintf(c.VerboseOut, "< HTTP %d (%d bytes)\n", resp.StatusCode, len(body))
	}
	if resp.StatusCode >= 400 || resp.StatusCode == StatusSoftBlock {
		return nil, parseAPIError(resp.StatusCode, body, resp.Header)
	}
	return body, nil
}

// Do sends one raw authenticated Voyager request and returns status, headers, and body — the
// escape hatch behind `linkedin api`. Only GET is ever sent for read Voyager; method is validated
// by the caller. A dry-run returns status 0.
func (c *Client) Do(ctx context.Context, path string, q url.Values) (int, json.RawMessage, error) {
	u := c.voyagerBase + "/" + strings.TrimLeft(path, "/")
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	body, err := c.get(ctx, u, true)
	if err != nil {
		var apiErr *APIError
		if ok := asAPIError(err, &apiErr); ok {
			return apiErr.StatusCode, apiErr.Body, err
		}
		return 0, nil, err
	}
	if body == nil { // dry-run
		return 0, nil, nil
	}
	return http.StatusOK, body, nil
}

// printCurl emits a copy-pasteable curl equivalent, redacting the session cookies unless
// --show-token.
func (c *Client) printCurl(fullURL string, headers map[string]string, liAt, jsession string) {
	var b strings.Builder
	b.WriteString("curl " + shellQuote(fullURL))
	for _, k := range sortedKeys(headers) {
		b.WriteString(" \\\n  -H " + shellQuote(k+": "+headers[k]))
	}
	if liAt != "" || jsession != "" {
		la, js := "REDACTED", "REDACTED"
		if c.ShowToken {
			la, js = liAt, jsession
		}
		b.WriteString(" \\\n  -H " + shellQuote(fmt.Sprintf("Cookie: li_at=%s; JSESSIONID=%s", la, js)))
		csrf := "REDACTED"
		if c.ShowToken {
			csrf = strings.Trim(jsession, `"`)
		}
		b.WriteString(" \\\n  -H " + shellQuote("csrf-token: "+csrf))
	}
	fmt.Fprintln(c.DryRunOut, b.String())
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// asAPIError is a tiny errors.As shim kept here so client.go doesn't import errors just for Do.
func asAPIError(err error, target **APIError) bool {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
