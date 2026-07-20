package api

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// GetCompany fetches an organization by its universalName (the slug in a company URL,
// linkedin.com/company/<slug>) and returns the trustworthy Company entity's raw JSON.
func (c *Client) GetCompany(ctx context.Context, slug string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("q", "universalName")
	q.Set("universalName", slug)
	q.Set("decorationId", voyager.DecorationCompany)
	raw, err := c.getVoyager(ctx, voyager.PathCompanies, q.Encode())
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	return voyager.ParseEntity(raw, voyager.TypeCompany)
}
