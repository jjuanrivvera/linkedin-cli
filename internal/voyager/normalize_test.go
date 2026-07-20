package voyager

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const searchFixture = `{
  "data": {
    "data": {
      "paging": {"total": 42, "start": 0, "count": 2},
      "*elements": [
        "urn:li:fsd_jobPostingCard:(4011,JOBS_SEARCH)",
        "urn:li:fsd_jobPostingCard:(4022,JOBS_SEARCH)"
      ]
    }
  },
  "included": [
    {"$type":"com.linkedin.voyager.dash.jobs.JobPostingCard","entityUrn":"urn:li:fsd_jobPostingCard:(4011,JOBS_SEARCH)","jobPostingUrn":"urn:li:fsd_jobPosting:4011","jobPostingTitle":"Senior Go Engineer","primaryDescription":{"text":"Acme Corp"},"secondaryDescription":{"text":"Bogota, Colombia (Remote)"}},
    {"$type":"com.linkedin.voyager.dash.jobs.JobPostingCard","entityUrn":"urn:li:fsd_jobPostingCard:(4022,JOBS_SEARCH)","jobPostingUrn":"urn:li:fsd_jobPosting:4022","jobPostingTitle":"Backend Engineer","primaryDescription":{"text":"Globex"},"secondaryDescription":{"text":"Remote"}}
  ]
}`

func TestParseSearch(t *testing.T) {
	res, err := ParseSearch(json.RawMessage(searchFixture))
	require.NoError(t, err)
	assert.Equal(t, 42, res.Total)
	require.Len(t, res.Cards, 2)
	assert.Equal(t, "4011", res.Cards[0].ID)
	assert.Equal(t, "Senior Go Engineer", res.Cards[0].Title)
	assert.Equal(t, "Acme Corp", res.Cards[0].Company)
	assert.Equal(t, "Bogota, Colombia (Remote)", res.Cards[0].Location)
	assert.Equal(t, "4022", res.Cards[1].ID)
}

func TestParseSearch_TitleAsBareString(t *testing.T) {
	// A variant where jobPostingTitle is a bare string and elements is the non-* key.
	fixture := `{"data":{"paging":{"total":1},"elements":["urn:li:x:(9,S)"]},
      "included":[{"$type":"x.JobPostingCard","entityUrn":"urn:li:x:(9,S)","jobPostingUrn":"urn:li:fsd_jobPosting:9","jobPostingTitle":"Dev","primaryDescription":"Acme","secondaryDescription":"Remote"}]}`
	res, err := ParseSearch(json.RawMessage(fixture))
	require.NoError(t, err)
	require.Len(t, res.Cards, 1)
	assert.Equal(t, "Dev", res.Cards[0].Title)
	assert.Equal(t, "Acme", res.Cards[0].Company)
}

func TestParseSearch_EmptyIsNotSchemaMoved(t *testing.T) {
	fixture := `{"data":{"data":{"paging":{"total":0},"*elements":[]}},"included":[]}`
	res, err := ParseSearch(json.RawMessage(fixture))
	require.NoError(t, err)
	assert.Empty(t, res.Cards)
	assert.Equal(t, 0, res.Total)
}

func TestParseSearch_SchemaMoved(t *testing.T) {
	// Positive total but no recognizable job entities → the schema rotated.
	fixture := `{"data":{"data":{"paging":{"total":5},"*elements":["urn:li:unknown:1"]}},
      "included":[{"$type":"com.linkedin.voyager.SomethingElse","entityUrn":"urn:li:unknown:1"}]}`
	_, err := ParseSearch(json.RawMessage(fixture))
	assert.ErrorIs(t, err, ErrSchemaMoved)
}

func TestParseSearch_FallbackHarvest(t *testing.T) {
	// *elements references don't resolve, but job entities exist in the pool → harvested.
	fixture := `{"data":{"data":{"paging":{"total":1},"*elements":["urn:li:mismatch:1"]}},
      "included":[{"$type":"x.JobPostingCardLite","entityUrn":"urn:li:actual:1","jobPostingUrn":"urn:li:fsd_jobPosting:77","jobPostingTitle":"T"}]}`
	res, err := ParseSearch(json.RawMessage(fixture))
	require.NoError(t, err)
	require.Len(t, res.Cards, 1)
	assert.Equal(t, "77", res.Cards[0].ID)
}

func TestParseSearch_BadJSON(t *testing.T) {
	_, err := ParseSearch(json.RawMessage(`not json`))
	assert.Error(t, err)
}

func TestParseEntity(t *testing.T) {
	detail := `{"data":{},"included":[
      {"$type":"com.linkedin.voyager.jobs.JobPosting","entityUrn":"urn:li:job:4011","title":"Senior Go Engineer","workRemoteAllowed":true,"listedAt":1719000000000}]}`
	ent, err := ParseEntity(json.RawMessage(detail), TypeJobPosting)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(ent, &got))
	assert.Equal(t, "Senior Go Engineer", got["title"])
}

func TestParseEntity_InData(t *testing.T) {
	// Entity lives directly in `data`, not `included`.
	detail := `{"data":{"$type":"org.Company","name":"Stripe"},"included":[]}`
	ent, err := ParseEntity(json.RawMessage(detail), TypeCompany)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(ent, &got))
	assert.Equal(t, "Stripe", got["name"])
}

func TestParseEntity_SchemaMoved(t *testing.T) {
	_, err := ParseEntity(json.RawMessage(`{"data":{},"included":[{"$type":"x.Other"}]}`), TypeCompany)
	assert.ErrorIs(t, err, ErrSchemaMoved)
}

func TestParseEntity_BadJSON(t *testing.T) {
	_, err := ParseEntity(json.RawMessage(`nope`), TypeCompany)
	assert.Error(t, err)
}

func TestLastURNSegment(t *testing.T) {
	cases := map[string]string{
		"urn:li:fsd_jobPosting:4011":            "4011",
		"urn:li:fsd_jobPostingCard:(4011,JOBS)": "4011",
		"4011":                                  "4011",
		"":                                      "",
		"urn:li:company:(9)":                    "9",
	}
	for in, want := range cases {
		assert.Equal(t, want, lastURNSegment(in), in)
	}
}

func TestTypeContains(t *testing.T) {
	assert.True(t, typeContains("com.linkedin.x.JobPostingCard", TypeJobCard))
	assert.True(t, typeContains("com.linkedin.x.JobPostingCardLite", TypeJobCard))
	assert.False(t, typeContains("com.linkedin.x.Company", TypeJobCard))
}

func TestTextOrStr(t *testing.T) {
	var a textOrStr
	require.NoError(t, a.UnmarshalJSON([]byte(`"hello"`)))
	assert.Equal(t, "hello", a.Text)

	var b textOrStr
	require.NoError(t, b.UnmarshalJSON([]byte(`{"text":"world"}`)))
	assert.Equal(t, "world", b.Text)

	var c textOrStr
	require.NoError(t, c.UnmarshalJSON([]byte(`null`)))
	assert.Empty(t, c.Text)

	var d textOrStr
	require.NoError(t, d.UnmarshalJSON([]byte(`12345`))) // unknown shape → empty, no error
	assert.Empty(t, d.Text)
}
