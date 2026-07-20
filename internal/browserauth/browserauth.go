// Package browserauth borrows a live browser session by extracting specific cookies for a domain
// from the user's installed browsers. It is the reusable "borrow-the-browser-session" primitive
// (the same shape slackctl uses for xoxc/xoxd): configure an Extractor with a domain and the set
// of cookie names you need, and Extract returns them plus the source browser/profile they came
// from. The browser-reading itself is behind a seam, so callers (and tests) can inject cookies
// without ever touching a real browser store.
//
// This package is deliberately self-contained and org-agnostic — a future cookie-auth CLI can
// import it and only change Domain / RequiredCookieNames.
package browserauth

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RawCookie is one cookie discovered in a browser store.
type RawCookie struct {
	Name    string
	Value   string
	Browser string // e.g. "chrome", "brave", "firefox"
	Profile string // browser profile, e.g. "Default"
}

// Finder discovers cookies for a domain across the browsers on this host. It is the seam that
// isolates kooky: production uses the kooky-backed finder, tests inject a fake.
type Finder func(ctx context.Context, domain string) ([]RawCookie, error)

// Extractor pulls a required set of cookies for one domain out of the browser stores.
type Extractor struct {
	// Domain is the cookie domain suffix to match, e.g. ".linkedin.com".
	Domain string
	// Browsers optionally restricts extraction to these browser names (case-insensitive:
	// "chrome", "brave", "firefox", "chromium", "edge"). Empty means "any browser".
	Browsers []string
	// RequiredCookieNames are the cookies that MUST all be present from a single source for a
	// successful extraction (e.g. []string{"li_at", "JSESSIONID"}).
	RequiredCookieNames []string

	// finder is the cookie source; nil defaults to the kooky-backed finder.
	finder Finder
}

// WithFinder overrides the cookie source (tests inject a fake so they never read a real browser).
func (e *Extractor) WithFinder(f Finder) *Extractor {
	e.finder = f
	return e
}

// Extract returns the required cookies as a name→value map plus a human-readable source
// ("chrome (Default)"). It succeeds only when a SINGLE source (one browser+profile) supplies
// every RequiredCookieName — mixing cookies from two sessions would produce an invalid pair. An
// error names exactly what was missing so the user knows to log in / pick another browser.
func (e *Extractor) Extract(ctx context.Context) (map[string]string, string, error) {
	find := e.finder
	if find == nil {
		find = defaultFinder
	}
	cookies, err := find(ctx, e.Domain)
	if err != nil {
		return nil, "", err
	}

	// Group by source (browser+profile), keeping only requested browsers and required names.
	type source struct{ browser, profile string }
	bySource := map[source]map[string]string{}
	order := []source{}
	for _, ck := range cookies {
		if !e.browserAllowed(ck.Browser) {
			continue
		}
		if !e.required(ck.Name) {
			continue
		}
		s := source{browser: ck.Browser, profile: ck.Profile}
		if _, ok := bySource[s]; !ok {
			bySource[s] = map[string]string{}
			order = append(order, s)
		}
		// First non-empty value wins (a store can list a cookie twice; keep the first usable).
		if ck.Value != "" && bySource[s][ck.Name] == "" {
			bySource[s][ck.Name] = ck.Value
		}
	}

	// Deterministic source order so repeated runs pick the same session.
	sort.Slice(order, func(i, j int) bool {
		if order[i].browser != order[j].browser {
			return order[i].browser < order[j].browser
		}
		return order[i].profile < order[j].profile
	})

	for _, s := range order {
		got := bySource[s]
		if e.complete(got) {
			src := s.browser
			if s.profile != "" {
				src += " (" + s.profile + ")"
			}
			return got, src, nil
		}
	}

	return nil, "", e.notFoundErr()
}

func (e *Extractor) browserAllowed(name string) bool {
	if len(e.Browsers) == 0 {
		return true
	}
	name = strings.ToLower(name)
	for _, b := range e.Browsers {
		if strings.EqualFold(b, name) || strings.Contains(name, strings.ToLower(b)) {
			return true
		}
	}
	return false
}

func (e *Extractor) required(name string) bool {
	for _, r := range e.RequiredCookieNames {
		if r == name {
			return true
		}
	}
	return false
}

func (e *Extractor) complete(got map[string]string) bool {
	for _, r := range e.RequiredCookieNames {
		if got[r] == "" {
			return false
		}
	}
	return len(e.RequiredCookieNames) > 0
}

// notFoundErr builds an actionable message naming the required cookies and where they were sought.
func (e *Extractor) notFoundErr() error {
	names := strings.Join(e.RequiredCookieNames, ", ")
	target := "any browser"
	if len(e.Browsers) > 0 {
		target = strings.Join(e.Browsers, "/")
	}
	return fmt.Errorf(
		"could not find a complete cookie set (%s) for %s in %s — log in to the site in that browser "+
			"first, make sure it's fully closed if it locks its cookie DB, or set the cookies via env "+
			"overrides", names, e.Domain, target)
}
