package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const searchBody = `{
  "data":{"data":{"paging":{"total":2,"start":0,"count":2},"*elements":[
    "urn:li:fsd_jobPostingCard:(1,S)","urn:li:fsd_jobPostingCard:(2,S)"]}},
  "included":[
    {"$type":"x.JobPostingCard","entityUrn":"urn:li:fsd_jobPostingCard:(1,S)","jobPostingUrn":"urn:li:fsd_jobPosting:1","jobPostingTitle":"Go Dev","primaryDescription":{"text":"Acme"},"secondaryDescription":{"text":"Remote"}},
    {"$type":"x.JobPostingCard","entityUrn":"urn:li:fsd_jobPostingCard:(2,S)","jobPostingUrn":"urn:li:fsd_jobPosting:2","jobPostingTitle":"SRE","primaryDescription":{"text":"Globex"},"secondaryDescription":{"text":"NYC"}}
  ]}`

func TestBuildSearchQuery_StructuralCharsLiteral(t *testing.T) {
	q := buildSearchQuery(SearchFilters{
		Keywords: "product design", GeoID: "100876405", Remote: true, SinceSecs: 604800,
		JobType: []string{"F", "C"}, Experience: []string{"3", "4"},
	}, 0, 25)
	// Structural chars stay literal; only the keyword value is percent-encoded.
	assert.Contains(t, q, "keywords:product%20design")
	assert.Contains(t, q, "locationUnion:(geoId:100876405)")
	assert.Contains(t, q, "workplaceType:List(2)")
	assert.Contains(t, q, "timePostedRange:List(r604800)")
	assert.Contains(t, q, "jobType:List(F,C)")
	assert.Contains(t, q, "experience:List(3,4)")
	assert.Contains(t, q, "&q=jobSearch")
	// The blob's parentheses/colons/commas must not be percent-encoded.
	assert.NotContains(t, q, "%28")
	assert.NotContains(t, q, "%3A")
}

func TestParseSinceSeconds(t *testing.T) {
	cases := map[string]int{
		"":      0,
		"24h":   86400,
		"7d":    604800,
		"2w":    1209600,
		"day":   86400,
		"week":  604800,
		"month": 2592000,
		"3d":    259200,
	}
	for in, want := range cases {
		got, err := ParseSinceSeconds(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err := ParseSinceSeconds("bogus")
	assert.Error(t, err)
	_, err = ParseSinceSeconds("5y")
	assert.Error(t, err)
}

func TestSearchJobs(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/voyagerJobsDashJobCards", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "q=jobSearch")
		_, _ = w.Write([]byte(searchBody))
	})
	res, err := c.SearchJobs(t.Context(), SearchFilters{Keywords: "go"}, 0, 25)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 2, res.Total)
	require.Len(t, res.Cards, 2)
	assert.Equal(t, "1", res.Cards[0].ID)
	assert.NotEmpty(t, res.Raw)
}

func TestSearchJobsAll_Paginates(t *testing.T) {
	page2 := strings.ReplaceAll(searchBody, `"total":2`, `"total":4`)
	page2 = strings.ReplaceAll(page2, "(1,S)", "(3,S)")
	page2 = strings.ReplaceAll(page2, "(2,S)", "(4,S)")
	page2 = strings.ReplaceAll(page2, "jobPosting:1", "jobPosting:3")
	page2 = strings.ReplaceAll(page2, "jobPosting:2", "jobPosting:4")
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.RawQuery, "start=0") {
			_, _ = w.Write([]byte(strings.ReplaceAll(searchBody, `"total":2`, `"total":4`)))
			return
		}
		_, _ = w.Write([]byte(page2))
	})
	cards, err := c.SearchJobsAll(t.Context(), SearchFilters{Keywords: "go"}, 2, 4, true)
	require.NoError(t, err)
	assert.Len(t, cards, 4)
	assert.Equal(t, 2, calls)
}

func TestSearchJobsAll_SinglePage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(searchBody)) })
	cards, err := c.SearchJobsAll(t.Context(), SearchFilters{}, 25, 0, false)
	require.NoError(t, err)
	assert.Len(t, cards, 2)
}

func TestSearchJobsAll_Dedup(t *testing.T) {
	// The server ignores start and returns the same page — dedup must prevent duplicates.
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.ReplaceAll(searchBody, `"total":2`, `"total":99`)))
	})
	cards, err := c.SearchJobsAll(t.Context(), SearchFilters{}, 2, 50, true)
	require.NoError(t, err)
	assert.Len(t, cards, 2, "repeated pages must be de-duplicated by id")
}

func TestGetJob_ChargesDailyCap(t *testing.T) {
	detail := `{"data":{},"included":[{"$type":"x.JobPosting","entityUrn":"urn:li:job:1","title":"Go Dev","workRemoteAllowed":true,"listedAt":1719000000000}]}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/jobs/jobPostings/1")
		_, _ = w.Write([]byte(detail))
	})
	pacer := &Pacer{DailyCap: 2, StatePath: filepath.Join(t.TempDir(), "state.json")}
	c.pacer = pacer
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)

	body, err := c.GetJob(t.Context(), "1", now)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Go Dev")

	_, err = c.GetJob(t.Context(), "1", now)
	require.NoError(t, err)
	// Third fetch exceeds the daily cap of 2 → refused before any request.
	_, err = c.GetJob(t.Context(), "1", now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily job-detail cap")
}
