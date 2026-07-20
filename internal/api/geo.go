package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// GeoHit is one geo typeahead result.
type GeoHit struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// typeaheadHit mirrors the jobs-guest typeahead JSON: [{id, displayName}]. The id is the geoId.
type typeaheadHit struct {
	ID          json.Number `json:"id"`
	DisplayName string      `json:"displayName"`
}

// ResolveGeo resolves a location name to LinkedIn geo hits via the unauthenticated jobs-guest
// typeahead. It returns every hit (the first is the best match). "remote" is NOT a geo — it is a
// workplaceType filter — so callers must not route "remote" through here.
func (c *Client) ResolveGeo(ctx context.Context, name string) ([]GeoHit, error) {
	q := url.Values{}
	q.Set("typeaheadType", voyager.TypeaheadGeoType)
	q.Set("geoTypes", voyager.TypeaheadPlacesParam)
	q.Set("query", name)
	raw, err := c.getWeb(ctx, voyager.TypeaheadPath, q)
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	var hits []typeaheadHit
	if err := json.Unmarshal(raw, &hits); err != nil {
		return nil, fmt.Errorf("decode geo typeahead: %w", err)
	}
	out := make([]GeoHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, GeoHit{ID: h.ID.String(), DisplayName: h.DisplayName})
	}
	return out, nil
}

// GeoCache is a name→geoId cache persisted in the config dir, so a repeated --location resolves
// without another typeahead round-trip (fewer requests = safer). Keys are lowercased names.
type GeoCache struct {
	path string
	data map[string]string
}

// NewGeoCache loads (or starts) the cache at dir/geocache.json.
func NewGeoCache(dir string) *GeoCache {
	g := &GeoCache{path: filepath.Join(dir, "geocache.json"), data: map[string]string{}}
	if b, err := os.ReadFile(g.path); err == nil { // #nosec G304 -- fixed path in config dir
		_ = json.Unmarshal(b, &g.data)
	}
	return g
}

// Get returns a cached geoId for name (and whether it was present).
func (g *GeoCache) Get(name string) (string, bool) {
	id, ok := g.data[strings.ToLower(strings.TrimSpace(name))]
	return id, ok
}

// Put stores name→geoId and persists the cache atomically.
func (g *GeoCache) Put(name, geoID string) {
	g.data[strings.ToLower(strings.TrimSpace(name))] = geoID
	if err := os.MkdirAll(filepath.Dir(g.path), 0o700); err != nil {
		return
	}
	b, err := json.Marshal(g.data)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(g.path), ".geo-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpName, g.path)
}
