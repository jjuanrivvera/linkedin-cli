package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// SearchFilters are the user-facing job-search filters. They compile into LinkedIn's Rest.li
// `query=(...)` blob (see buildSearchQuery). Zero-valued fields are omitted.
type SearchFilters struct {
	Keywords   string   // free-text role/skill match
	GeoID      string   // resolved location geoId (see ResolveGeo); "" = anywhere
	Remote     bool     // → workplaceType:List(2)
	SinceSecs  int      // → timePostedRange:List(r<secs>); 0 = any time
	JobType    []string // → jobType:List(F,C,…)
	Experience []string // → experience:List(2,3,…)
}

// SearchResult is one parsed page plus the raw envelope (for -o json).
type SearchResult struct {
	*voyager.SearchResult
	Raw json.RawMessage
}

// defaultCount is the Voyager job-search page size the web client uses.
const defaultCount = 25

// GeoIDPlaceholder stands in for a resolved geoId during a --location dry-run, where the geo
// typeahead can't run offline. It is kept literal (not percent-encoded) in the previewed
// job-search curl so a human sees locationUnion:(geoId:<GEO_ID>).
const GeoIDPlaceholder = "<GEO_ID>"

// SearchJobs runs one page of a job search (start/count) and returns the parsed page + raw JSON.
func (c *Client) SearchJobs(ctx context.Context, f SearchFilters, start, count int) (*SearchResult, error) {
	if count <= 0 {
		count = defaultCount
	}
	raw, err := c.getVoyager(ctx, voyager.PathJobSearch, buildSearchQuery(f, start, count))
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	parsed, err := voyager.ParseSearch(raw)
	if err != nil {
		return nil, err
	}
	return &SearchResult{SearchResult: parsed, Raw: raw}, nil
}

// SearchJobsAll walks pages until it collects `limit` cards (limit<=0 && !all → one page),
// pacing every request via the ban-safety Pacer. It de-duplicates by job id as a safety net.
func (c *Client) SearchJobsAll(ctx context.Context, f SearchFilters, count, limit int, all bool) ([]voyager.JobCard, error) {
	const pageCap = 40 // ban-safety: never spin unbounded even with --all
	if count <= 0 {
		count = defaultCount
	}
	seen := map[string]struct{}{}
	var out []voyager.JobCard
	start := 0
	for page := 0; page < pageCap; page++ {
		res, err := c.SearchJobs(ctx, f, start, count)
		if err != nil {
			return nil, err
		}
		if res == nil { // dry-run prints only the first request
			return nil, nil
		}
		added := 0
		for _, card := range res.Cards {
			if card.ID != "" {
				if _, dup := seen[card.ID]; dup {
					continue
				}
				seen[card.ID] = struct{}{}
			}
			out = append(out, card)
			added++
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		if added == 0 || len(res.Cards) == 0 {
			return out, nil
		}
		if res.Total > 0 && start+len(res.Cards) >= res.Total {
			return out, nil
		}
		if !all && limit == 0 {
			return out, nil // single page
		}
		start += len(res.Cards)
	}
	return out, nil
}

// GetJob fetches one job posting's full detail and returns the trustworthy JobPosting entity's
// raw JSON. It CHARGES the daily job-detail cap first (ban-safety): if today's budget is spent,
// it refuses rather than fetching. now is injected for deterministic tests.
func (c *Client) GetJob(ctx context.Context, id string, now time.Time) (json.RawMessage, error) {
	if !c.DryRun && c.pacer != nil {
		if err := c.pacer.ChargeDaily(now); err != nil {
			return nil, err
		}
	}
	q := url.Values{}
	q.Set("decorationId", voyager.DecorationJobDetail)
	// The detail path keeps its query normal (no Rest.li blob).
	raw, err := c.getVoyager(ctx, voyager.PathJobDetail+url.PathEscape(id), q.Encode())
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	return voyager.ParseEntity(raw, voyager.TypeJobPosting)
}

// buildSearchQuery assembles the Voyager job-search query string BY HAND. The Rest.li
// `query=(...)` blob uses bare structural characters — `(`, `)`, `,`, `:` and `List(...)` — that
// must NOT be percent-encoded, or LinkedIn 400s the request. So only the dynamic VALUES (keywords)
// are escaped; the structure is assembled literally. This is why the query is not built from
// url.Values (which would encode the structural chars).
func buildSearchQuery(f SearchFilters, start, count int) string {
	var b strings.Builder
	b.WriteString("decorationId=")
	b.WriteString(voyager.DecorationJobSearch)
	b.WriteString("&q=jobSearch")
	b.WriteString("&start=")
	b.WriteString(strconv.Itoa(start))
	b.WriteString("&count=")
	b.WriteString(strconv.Itoa(count))
	b.WriteString("&query=")
	b.WriteString(buildQueryBlob(f))
	return b.String()
}

// buildQueryBlob builds the parenthesised Rest.li query value, keeping all structural characters
// literal and escaping only free-text values.
func buildQueryBlob(f SearchFilters) string {
	parts := []string{voyager.QueryOrigin + ":" + voyager.OriginJobSearch}
	if f.Keywords != "" {
		parts = append(parts, voyager.QueryKeywords+":"+escapeValue(f.Keywords))
	}
	if f.GeoID != "" {
		geo := escapeValue(f.GeoID)
		if f.GeoID == GeoIDPlaceholder {
			// Dry-run placeholder: keep it human-readable in the previewed curl (a real geoId is
			// numeric and never equals this sentinel, so non-dry-run behavior is unchanged).
			geo = f.GeoID
		}
		parts = append(parts, voyager.QueryLocationUnion+":("+voyager.QueryGeoID+":"+geo+")")
	}
	if sel := buildSelectedFilters(f); sel != "" {
		parts = append(parts, voyager.QuerySelectedTypes+":("+sel+")")
	}
	parts = append(parts, voyager.QuerySpellCorrect+":true")
	return "(" + strings.Join(parts, ",") + ")"
}

// buildSelectedFilters builds the selectedFilters inner blob: List(...) per active filter.
func buildSelectedFilters(f SearchFilters) string {
	var sel []string
	if f.Remote {
		sel = append(sel, voyager.FilterWorkplaceType+":"+list(voyager.WorkplaceRemote))
	}
	if f.SinceSecs > 0 {
		sel = append(sel, voyager.FilterTimePostedRange+":"+list("r"+strconv.Itoa(f.SinceSecs)))
	}
	if len(f.JobType) > 0 {
		sel = append(sel, voyager.FilterJobType+":"+list(f.JobType...))
	}
	if len(f.Experience) > 0 {
		sel = append(sel, voyager.FilterExperience+":"+list(f.Experience...))
	}
	return strings.Join(sel, ",")
}

// list wraps values in the Rest.li List(a,b,c) structure. Values are escaped (they are short
// codes, but escaping keeps a stray char from breaking the blob).
func list(values ...string) string {
	esc := make([]string, len(values))
	for i, v := range values {
		esc[i] = escapeValue(v)
	}
	return "List(" + strings.Join(esc, ",") + ")"
}

// escapeValue percent-encodes a dynamic value so it can sit inside the Rest.li blob without its
// characters being mistaken for structure. url.QueryEscape uses '+' for space; LinkedIn expects
// %20, so we convert. Structural chars are never routed through here.
func escapeValue(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

var sinceRe = regexp.MustCompile(`^([0-9]+)\s*([hdw])$`)

// ParseSinceSeconds converts a --since window into seconds for timePostedRange:List(r<secs>). It
// accepts a relative shorthand Nh/Nd/Nw (e.g. 24h, 7d, 2w) or the named windows day/week/month.
// An empty value returns 0 (any time). The three canonical LinkedIn windows are r86400 (24h),
// r604800 (7d) and r2592000 (30d).
func ParseSinceSeconds(value string) (int, error) {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" {
		return 0, nil
	}
	switch v {
	case "day", "24h", "past-24h":
		return 86400, nil
	case "week", "past-week":
		return 604800, nil
	case "month", "past-month":
		return 2592000, nil
	}
	if m := sinceRe.FindStringSubmatch(v); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, sinceError(value)
		}
		switch m[2] {
		case "h":
			return n * 3600, nil
		case "d":
			return n * 86400, nil
		case "w":
			return n * 604800, nil
		}
	}
	return 0, sinceError(value)
}

func sinceError(value string) error {
	return fmt.Errorf("invalid --since value %q: use Nh/Nd/Nw (e.g. 24h, 7d, 2w) or day|week|month", value)
}
