package commands

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const searchJSON = `{
  "data":{"data":{"paging":{"total":2,"start":0,"count":2},"*elements":[
    "urn:li:fsd_jobPostingCard:(1,S)","urn:li:fsd_jobPostingCard:(2,S)"]}},
  "included":[
    {"$type":"x.JobPostingCard","entityUrn":"urn:li:fsd_jobPostingCard:(1,S)","jobPostingUrn":"urn:li:fsd_jobPosting:1","jobPostingTitle":"Go Dev","primaryDescription":{"text":"Acme"},"secondaryDescription":{"text":"Remote"}},
    {"$type":"x.JobPostingCard","entityUrn":"urn:li:fsd_jobPostingCard:(2,S)","jobPostingUrn":"urn:li:fsd_jobPosting:2","jobPostingTitle":"SRE","primaryDescription":{"text":"Globex"},"secondaryDescription":{"text":"NYC"}}
  ]}`

const detailJSON = `{"data":{},"included":[{"$type":"x.JobPosting","entityUrn":"urn:li:job:1","title":"Go Dev","workRemoteAllowed":true,"listedAt":1719000000000,"description":{"text":"Build things"}}]}`

const companyJSON = `{"data":{},"included":[{"$type":"org.Company","entityUrn":"urn:li:company:1","name":"Stripe","universalName":"stripe","staffCount":8000}]}`

const geoJSON = `[{"id":"100876405","displayName":"Colombia"},{"id":"90009706","displayName":"Bogota"}]`

// router routes by path prefix so one server serves job search, detail, company, and geo.
func router() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "voyagerJobsDashJobCards"):
			_, _ = w.Write([]byte(searchJSON))
		case strings.Contains(r.URL.Path, "jobs/jobPostings/"):
			_, _ = w.Write([]byte(detailJSON))
		case strings.Contains(r.URL.Path, "organization/companies"):
			_, _ = w.Write([]byte(companyJSON))
		case strings.Contains(r.URL.Path, "typeaheadHits"):
			_, _ = w.Write([]byte(geoJSON))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}
}

func TestJobsSearch_JSON(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("jobs", "search", "--keywords", "go", "--remote", "--since", "7d", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Go Dev")
	assert.Contains(t, out, `"id": "1"`)
}

func TestJobsSearch_IDFormat(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("jobs", "search", "--keywords", "go", "-o", "id")
	require.NoError(t, err)
	assert.Equal(t, "1\n2\n", out)
}

func TestJobsSearch_Table(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("jobs", "search", "--keywords", "go")
	require.NoError(t, err)
	assert.Contains(t, out, "TITLE")
	assert.Contains(t, out, "Acme")
}

func TestJobsSearch_WithLocationResolvesGeo(t *testing.T) {
	e := newEnv(t, router())
	_, errOut, err := e.run("jobs", "search", "--keywords", "go", "--location", "Colombia", "-o", "id")
	require.NoError(t, err)
	assert.Contains(t, errOut, "geoId 100876405")
}

func TestJobsSearch_DryRun(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("jobs", "search", "--keywords", "go", "--remote", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "voyagerJobsDashJobCards")
	assert.Contains(t, out, "workplaceType:List(2)")
}

// TestJobsSearch_DryRunNoSession proves a --dry-run is a fully offline preview that needs NO stored
// session: an EMPTY keyring (and no LI_AT/JSESSIONID env) still prints a redacted curl, exits 0,
// and makes zero HTTP calls. Bug A regression guard.
func TestJobsSearch_DryRunNoSession(t *testing.T) {
	var hits int
	e := newEnv(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	t.Setenv("LI_AT", "")      // no headless session either
	t.Setenv("JSESSIONID", "") // keyring stays empty (fakeStore has no entry)
	out, _, err := e.run("jobs", "search", "--keywords", "go", "--remote", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "voyagerJobsDashJobCards")
	assert.Contains(t, out, "Cookie: li_at=REDACTED; JSESSIONID=REDACTED")
	assert.NotContains(t, out, "no LinkedIn session stored")
	assert.Equal(t, 0, hits, "dry-run must make no real HTTP request")
}

// TestJobsSearch_LocationDryRun proves a --location --dry-run previews offline: it prints the
// typeahead curl it WOULD send AND the job-search curl with a placeholder geoId, without resolving
// a real geo and without erroring "no geo match". Bug B regression guard.
func TestJobsSearch_LocationDryRun(t *testing.T) {
	var hits int
	e := newEnv(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	out, _, err := e.run("jobs", "search", "--keywords", "go", "--location", "Colombia", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "typeaheadHits")           // (1) the typeahead curl for X
	assert.Contains(t, out, "query=Colombia")          // typeahead queries the raw name
	assert.Contains(t, out, "voyagerJobsDashJobCards") // (2) the job-search curl
	assert.Contains(t, out, "locationUnion:(geoId:<GEO_ID>)")
	assert.NotContains(t, out, "no geo match")
	assert.Equal(t, 0, hits, "location dry-run must make no real HTTP request")
}

func TestJobsGet_JSON(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("jobs", "get", "1", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Build things")
	assert.Contains(t, out, "workRemoteAllowed")
}

func TestJobsGet_JQ(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("jobs", "get", "1", "--jq", ".description.text")
	require.NoError(t, err)
	assert.Contains(t, out, "Build things")
}

func TestCompanyGet(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("company", "get", "stripe", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Stripe")
}

func TestGeo(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("geo", "Colombia", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "100876405")
}

func TestGeo_JQ(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("geo", "Colombia", "--jq", ".[0].id")
	require.NoError(t, err)
	assert.Contains(t, out, "100876405")
}

func TestJobsSearch_SchemaMovedHint(t *testing.T) {
	moved := `{"data":{"data":{"paging":{"total":5},"*elements":["urn:li:unknown:1"]}},"included":[{"$type":"x.Other","entityUrn":"urn:li:unknown:1"}]}`
	e := newEnv(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(moved)) })
	_, _, err := e.run("jobs", "search", "--keywords", "go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema.go")
}

func TestApiEscapeHatch(t *testing.T) {
	e := newEnv(t, router())
	out, _, err := e.run("api", "organization/companies", "-q", "q=universalName", "-q", "universalName=stripe", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Stripe")
}

func TestNoSession_ActionableError(t *testing.T) {
	e := newEnv(t, router())
	// Clear the env session so no cookies resolve.
	t.Setenv("LI_AT", "")
	t.Setenv("JSESSIONID", "")
	_, _, err := e.run("jobs", "get", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cookie-from-browser")
}
