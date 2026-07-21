package voyager

import (
	"encoding/json"
	"errors"
	"strings"
)

// ErrSchemaMoved is returned when a response decodes structurally but contains none of the
// entity types we expect — the signature of a LinkedIn schema rotation. It is deliberately
// distinct from "no results": an empty search still carries paging + an (empty) element list,
// whereas a moved schema carries elements/entities we can no longer recognize. Callers surface
// it with a pointer at internal/voyager/schema.go, where the drift is fixed.
var ErrSchemaMoved = errors.New(
	"no recognizable job entities in the response — LinkedIn likely rotated the Voyager schema " +
		"(decorationId / $type / collection variant). Check and bump the constants in " +
		"internal/voyager/schema.go")

// JobCard is the thin, table-friendly slice of one job-search result. The complete card is
// always available via -o json (Raw), since unknown fields are preserved.
type JobCard struct {
	ID       string          `json:"id,omitempty"`
	Title    string          `json:"title,omitempty"`
	Company  string          `json:"company,omitempty"`
	Location string          `json:"location,omitempty"`
	URN      string          `json:"urn,omitempty"`
	Raw      json.RawMessage `json:"-"`
}

// SearchResult is the parsed job-search page: paging totals plus the resolved cards.
type SearchResult struct {
	Total int       `json:"total"`
	Start int       `json:"start"`
	Count int       `json:"count"`
	Cards []JobCard `json:"cards"`
}

// envelope is the normalized Voyager response: a `data` collection plus a flat `included`
// entity pool. References inside `data` are URN strings resolved against `included[].entityUrn`.
type envelope struct {
	Data     json.RawMessage   `json:"data"`
	Included []json.RawMessage `json:"included"`
}

// entityHeader pulls just the discriminators every included entity carries.
type entityHeader struct {
	Type      string `json:"$type"`
	EntityURN string `json:"entityUrn"`
}

// typeContains reports whether an entity's $type carries token as a substring. Matching by
// CONTAINS (never exact) is deliberate: LinkedIn versions and renames these ($type suffixes,
// JobPosting→JobPostingCard), so a substring match survives the drift (schema.go).
func typeContains(entityType, token string) bool {
	return strings.Contains(entityType, token)
}

// ParseSearch decodes a normalized voyagerJobsDashJobCards page. It builds a urn→entity index
// from included[], reads paging + the *elements reference list out of the data collection, and
// resolves each reference to a JobPostingCard. If the page carries paging but not a single
// recognizable job entity, it returns ErrSchemaMoved (a moved schema), NOT an empty result.
func ParseSearch(raw json.RawMessage) (*SearchResult, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	index, jobEntities := indexIncluded(env.Included)

	res := &SearchResult{}
	collection := unwrapData(env.Data)
	res.Total, res.Start, res.Count = readPaging(collection)

	// LinkedIn ships the same job as multiple card variants — a hydrated (id,JOBS_SEARCH) card
	// next to a thin (id,JOB_DETAILS) prefetch stub with no text. Harvest the best variant per
	// job id up front, then emit each referenced card once, swapping a stub for its hydrated twin.
	best := bestCardsByID(env.Included, index)
	seen := make(map[string]bool)
	appendCard := func(card JobCard) {
		if card.ID != "" {
			if hydrated, ok := best[card.ID]; ok && card.Title == "" {
				card = hydrated
			}
			if seen[card.ID] {
				return
			}
			seen[card.ID] = true
		}
		res.Cards = append(res.Cards, card)
	}

	for _, urn := range readElements(collection) {
		ent, ok := index[urn]
		if !ok {
			continue
		}
		if card, ok := jobCardFrom(ent, index); ok {
			appendCard(card)
		}
	}

	// Fallback: if the *elements references didn't resolve (a variant we don't index by that
	// key), harvest any JobPostingCard entities directly from the pool so a minor structural
	// change still yields results.
	if len(res.Cards) == 0 && jobEntities > 0 {
		for _, ent := range env.Included {
			if card, ok := jobCardFrom(ent, index); ok {
				appendCard(card)
			}
		}
	}

	// A page with a positive total but zero recognizable job entities means the schema moved
	// (we can't see the cards we know are there). Treat that as a loud drift signal, not "0
	// results" — an honest empty search has total 0 AND no job entities, which is fine.
	if len(res.Cards) == 0 && jobEntities == 0 && res.Total > 0 {
		return nil, ErrSchemaMoved
	}
	return res, nil
}

// ParseEntity returns the raw JSON of the first included entity whose $type contains token
// (e.g. TypeJobPosting for a job detail, TypeCompany for a company). It is the trustworthy-fields
// path for `jobs get` / `company get`. ErrSchemaMoved when no such entity exists.
func ParseEntity(raw json.RawMessage, token string) (json.RawMessage, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	for _, ent := range env.Included {
		var h entityHeader
		if json.Unmarshal(ent, &h) == nil && typeContains(h.Type, token) {
			return ent, nil
		}
	}
	// Some detail responses put the entity directly in `data` rather than `included`.
	if len(env.Data) > 0 {
		var h entityHeader
		if json.Unmarshal(env.Data, &h) == nil && typeContains(h.Type, token) {
			return env.Data, nil
		}
	}
	return nil, ErrSchemaMoved
}

// indexIncluded builds a urn→rawEntity map and counts how many entities look like job cards, so
// ParseSearch can distinguish "schema moved" from "genuinely empty".
func indexIncluded(included []json.RawMessage) (map[string]json.RawMessage, int) {
	index := make(map[string]json.RawMessage, len(included))
	jobEntities := 0
	for _, ent := range included {
		var h entityHeader
		if json.Unmarshal(ent, &h) != nil {
			continue
		}
		if h.EntityURN != "" {
			index[h.EntityURN] = ent
		}
		if typeContains(h.Type, TypeJobCard) {
			jobEntities++
		}
	}
	return index, jobEntities
}

// unwrapData descends the collection wrapper: the paging + *elements live at data.data for the
// dash job-cards endpoint, but some variants keep them at data — try the inner one, fall back.
func unwrapData(data json.RawMessage) map[string]json.RawMessage {
	var outer map[string]json.RawMessage
	if json.Unmarshal(data, &outer) != nil {
		return nil
	}
	if inner, ok := outer["data"]; ok {
		var m map[string]json.RawMessage
		if json.Unmarshal(inner, &m) == nil {
			if _, hasEl := m["*elements"]; hasEl {
				return m
			}
			if _, hasPg := m["paging"]; hasPg {
				return m
			}
		}
	}
	return outer
}

// readPaging extracts total/start/count from a collection's paging block.
func readPaging(collection map[string]json.RawMessage) (total, start, count int) {
	pg, ok := collection["paging"]
	if !ok {
		return 0, 0, 0
	}
	var p struct {
		Total int `json:"total"`
		Start int `json:"start"`
		Count int `json:"count"`
	}
	_ = json.Unmarshal(pg, &p)
	return p.Total, p.Start, p.Count
}

// readElements returns the *elements reference URN list from a collection.
func readElements(collection map[string]json.RawMessage) []string {
	el, ok := collection["*elements"]
	if !ok {
		el, ok = collection["elements"] // tolerate the non-normalized key
		if !ok {
			return nil
		}
	}
	var urns []string
	if json.Unmarshal(el, &urns) == nil {
		return urns
	}
	return nil
}

// jobCard is the field subset we read off a JobPostingCard entity. LinkedIn puts the title as a
// bare string on some variants and a {text} object on others, so titleText handles both; the
// dash variant names it `title` instead of `jobPostingTitle`, so both keys are read.
type jobCard struct {
	Type          string    `json:"$type"`
	JobPostingURN string    `json:"jobPostingUrn"`
	EntityURN     string    `json:"entityUrn"`
	Title         textOrStr `json:"jobPostingTitle"`
	TitleAlt      textOrStr `json:"title"`
	Primary       textOrStr `json:"primaryDescription"`
	Secondary     textOrStr `json:"secondaryDescription"`
	JobPostingRef string    `json:"*jobPosting"`
}

// textOrStr decodes either a bare "…" string or a {"text":"…"} object into its text.
type textOrStr struct{ Text string }

func (t *textOrStr) UnmarshalJSON(b []byte) error {
	b = trimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &t.Text)
	}
	var obj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		//nolint:nilerr // an unknown title/description shape must not fail the whole parse — leave empty
		return nil
	}
	t.Text = obj.Text
	return nil
}

// jobCardFrom extracts a JobCard from a raw entity if it is a JobPostingCard, else (…,false).
// A card that carries no text of its own is hydrated by following its *jobPosting /
// jobPostingUrn reference into the entity pool.
func jobCardFrom(raw json.RawMessage, index map[string]json.RawMessage) (JobCard, bool) {
	var h entityHeader
	if json.Unmarshal(raw, &h) != nil || !typeContains(h.Type, TypeJobCard) {
		return JobCard{}, false
	}
	var c jobCard
	_ = json.Unmarshal(raw, &c)
	urn := c.JobPostingURN
	if urn == "" {
		urn = c.EntityURN
	}
	title := c.Title.Text
	if title == "" {
		title = c.TitleAlt.Text
	}
	if title == "" {
		title = titleFromPosting(index, c.JobPostingRef, c.JobPostingURN)
	}
	return JobCard{
		ID:       lastURNSegment(urn),
		Title:    title,
		Company:  c.Primary.Text,
		Location: c.Secondary.Text,
		URN:      urn,
		Raw:      raw,
	}, true
}

// bestCardsByID harvests every JobPostingCard in the pool and keeps, per job id, the most
// hydrated variant, so ParseSearch can swap a JOB_DETAILS prefetch stub for its titled twin.
func bestCardsByID(included []json.RawMessage, index map[string]json.RawMessage) map[string]JobCard {
	best := make(map[string]JobCard)
	for _, ent := range included {
		card, ok := jobCardFrom(ent, index)
		if !ok || card.ID == "" {
			continue
		}
		if cur, exists := best[card.ID]; !exists || (cur.Title == "" && card.Title != "") {
			best[card.ID] = card
		}
	}
	return best
}

// titleFromPosting resolves a card's posting references against the entity pool and reads the
// referenced JobPosting's title — the hydration path for cards that only carry URNs.
func titleFromPosting(index map[string]json.RawMessage, refs ...string) string {
	for _, ref := range refs {
		ent, ok := index[ref]
		if !ok {
			continue
		}
		var p struct {
			Title textOrStr `json:"title"`
		}
		if json.Unmarshal(ent, &p) == nil && p.Title.Text != "" {
			return p.Title.Text
		}
	}
	return ""
}

// lastURNSegment returns the id at the tail of an URN: the segment after the final ':'. For
// "urn:li:fsd_jobPostingCard:(4012345678,JOBS_SEARCH)" it strips a trailing "(…)" tuple to the
// leading numeric id, which is what jobs/jobPostings/{id} expects.
func lastURNSegment(urn string) string {
	if urn == "" {
		return ""
	}
	seg := urn
	if i := strings.LastIndex(urn, ":"); i >= 0 {
		seg = urn[i+1:]
	}
	seg = strings.TrimPrefix(seg, "(")
	if i := strings.IndexAny(seg, ",)"); i >= 0 {
		seg = seg[:i]
	}
	return seg
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r') {
		b = b[1:]
	}
	for len(b) > 0 {
		c := b[len(b)-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			b = b[:len(b)-1]
			continue
		}
		break
	}
	return b
}
