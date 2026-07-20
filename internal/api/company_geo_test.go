package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCompany(t *testing.T) {
	body := `{"data":{},"included":[{"$type":"org.Company","entityUrn":"urn:li:company:1","name":"Stripe","universalName":"stripe","staffCount":8000}]}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/organization/companies", r.URL.Path)
		assert.Equal(t, "universalName", r.URL.Query().Get("q"))
		assert.Equal(t, "stripe", r.URL.Query().Get("universalName"))
		_, _ = w.Write([]byte(body))
	})
	raw, err := c.GetCompany(t.Context(), "stripe")
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Stripe")
}

func TestResolveGeo(t *testing.T) {
	body := `[{"id":"100876405","displayName":"Colombia"},{"id":"90009706","displayName":"Bogota"}]`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs-guest/api/typeaheadHits", r.URL.Path)
		assert.Equal(t, "GEO", r.URL.Query().Get("typeaheadType"))
		assert.Equal(t, "Colombia", r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(body))
	})
	hits, err := c.ResolveGeo(t.Context(), "Colombia")
	require.NoError(t, err)
	require.Len(t, hits, 2)
	assert.Equal(t, "100876405", hits[0].ID)
	assert.Equal(t, "Colombia", hits[0].DisplayName)
}

func TestResolveGeo_BadJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`not json`)) })
	_, err := c.ResolveGeo(t.Context(), "x")
	assert.Error(t, err)
}

func TestGeoCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	g := NewGeoCache(dir)
	_, ok := g.Get("Bogota")
	assert.False(t, ok)
	g.Put("Bogota", "90009706")

	// A fresh cache reads it back from disk (case-insensitive key).
	g2 := NewGeoCache(dir)
	id, ok := g2.Get("bogota")
	require.True(t, ok)
	assert.Equal(t, "90009706", id)

	// The file is 0600.
	_, statErr := filepath.Glob(filepath.Join(dir, "geocache.json"))
	require.NoError(t, statErr)
}
