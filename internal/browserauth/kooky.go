package browserauth

import (
	"context"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // register all browser cookie-store finders
)

// defaultFinder is the production cookie source: it walks every browser cookie store kooky can
// find on this host and returns the cookies whose domain has the requested suffix. It never
// filters by name here (the Extractor does that) so one traversal serves any required set.
//
// This function talks to real browser stores, so it is intentionally thin — all the selection
// logic that MATTERS lives in Extractor.Extract, which is exercised by tests through an injected
// finder.
var defaultFinder Finder = func(ctx context.Context, domain string) ([]RawCookie, error) {
	var out []RawCookie
	stores := kooky.FindAllCookieStores(ctx)
	for _, store := range stores {
		func() {
			defer func() { _ = store.Close() }()
			browser := store.Browser()
			profile := store.Profile()
			seq := store.TraverseCookies(kooky.DomainHasSuffix(domain))
			for cookie, err := range seq {
				if err != nil || cookie == nil {
					continue
				}
				out = append(out, RawCookie{
					Name:    cookie.Name,
					Value:   cookie.Value,
					Browser: browser,
					Profile: profile,
				})
			}
		}()
	}
	return out, nil
}
