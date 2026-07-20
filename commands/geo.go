package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/api"
	"github.com/jjuanrivvera/linkedin-cli/internal/config"
)

func init() {
	registrars = append(registrars, func(d *deps) *cobra.Command {
		cmd := &cobra.Command{
			Use:   "geo <name>",
			Short: "Resolve a location name to a LinkedIn geoId",
			Long: `Resolve a place name to LinkedIn geo hits via the unauthenticated jobs-guest typeahead,
so you can pass a geoId to a job search. The first hit is the best match; results are cached
in the config dir so a repeated lookup makes no request.

Note: "remote" is NOT a geo — it is a workplace type. Use ` + "`--remote`" + ` on jobs search.`,
			Example: `  linkedin geo "Bogota, Colombia"
  linkedin geo "San Francisco Bay Area" -o json
  linkedin geo Colombia --jq '.[0].id'`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, _, err := d.getAPIClient()
				if err != nil {
					return err
				}
				hits, err := c.ResolveGeo(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if hits == nil { // dry-run
					return nil
				}
				// Warm the cache with the best match so a later --location skips the round-trip.
				if len(hits) > 0 {
					if dir, derr := config.Dir(); derr == nil {
						api.NewGeoCache(dir).Put(args[0], hits[0].ID)
					}
				}
				b, _ := json.Marshal(hits)
				return d.render(cmd, json.RawMessage(b), []string{"id", "displayName"})
			},
		}
		return annotate(cmd, kindRead)
	})
}

// resolveLocation turns a --location NAME into a geoId, using the on-disk cache first (fewer
// requests = safer) and the typeahead otherwise. It is shared by `jobs search`.
func (d *deps) resolveLocation(cmd *cobra.Command, c *api.Client, name string) (string, error) {
	dir, derr := config.Dir()
	var cache *api.GeoCache
	if derr == nil {
		cache = api.NewGeoCache(dir)
		if id, ok := cache.Get(name); ok {
			return id, nil
		}
	}
	hits, err := c.ResolveGeo(cmd.Context(), name)
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "", fmt.Errorf("no geo match for %q — try a broader name (e.g. a city or country)", name)
	}
	if cache != nil {
		cache.Put(name, hits[0].ID)
	}
	if !d.gf.quiet {
		fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q → geoId %s (%s)\n", name, hits[0].ID, hits[0].DisplayName)
	}
	return hits[0].ID, nil
}
