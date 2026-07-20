package commands

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/api"
	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// jobsColumns are the default table columns for a search result list.
var jobsColumns = []string{"id", "title", "company", "location"}

func init() {
	registrars = append(registrars, func(d *deps) *cobra.Command {
		jobsCmd := &cobra.Command{
			Use:     "jobs",
			Aliases: []string{"job"},
			Short:   "Search and inspect LinkedIn jobs",
			Long:    "Search LinkedIn job postings and fetch a single posting's full detail.",
		}
		jobsCmd.AddCommand(newJobsSearchCmd(d), newJobsGetCmd(d))
		return jobsCmd
	})
}

func newJobsSearchCmd(d *deps) *cobra.Command {
	var f api.SearchFilters
	var location, since string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search job postings",
		Long: `Search LinkedIn jobs by keywords, location, remoteness, recency, job type and experience.
Results paginate with --count/--limit/--all. Machine output (-o json / -o id / --jq) is the
primary interface for an assistant; -o table is the human view.

--location resolves a place NAME to a LinkedIn geoId via the typeahead (cached), then filters
with locationUnion:(geoId:<id>). "remote" is NOT a location — use --remote (workplaceType
List(2)). --since maps to LinkedIn's timePostedRange: 24h→r86400, 7d→r604800, 30d→r2592000.`,
		Example: `  linkedin jobs search --keywords "golang" --remote --since 7d -o json
  linkedin jobs search --keywords "product design" --location "Bogota, Colombia" --limit 50
  linkedin jobs search --keywords backend --job-type F,C --experience 3,4 -o id
  linkedin jobs search --keywords sre --remote --since 24h --all -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, _, err := d.getAPIClient()
			if err != nil {
				return err
			}
			if since != "" {
				secs, serr := api.ParseSinceSeconds(since)
				if serr != nil {
					return serr
				}
				f.SinceSecs = secs
			}
			if location != "" {
				geoID, gerr := d.resolveLocation(cmd, c, location)
				if gerr != nil {
					return gerr
				}
				f.GeoID = geoID
			}
			cards, err := c.SearchJobsAll(cmd.Context(), f, d.gf.count, d.gf.limit, d.gf.all)
			if err != nil {
				return err
			}
			if cards == nil { // dry-run
				return nil
			}
			return d.render(cmd, cardsToJSON(cards), jobsColumns)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.Keywords, "keywords", "", "role/skill keywords to match")
	fl.StringVar(&f.Keywords, "query", "", "alias for --keywords")
	_ = fl.MarkHidden("query")
	fl.StringVar(&location, "location", "", "place name resolved to a geoId (e.g. \"Bogota, Colombia\"); NOT for remote — use --remote")
	fl.BoolVar(&f.Remote, "remote", false, "only remote roles (workplaceType List(2))")
	fl.StringVar(&since, "since", "", "only postings within this window: Nh/Nd/Nw (e.g. 24h, 7d, 2w) or day|week|month")
	fl.StringSliceVar(&f.JobType, "job-type", nil, "LinkedIn job-type codes (F=full-time,P=part-time,C=contract,T=temporary,I=internship,V=volunteer,O=other)")
	fl.StringSliceVar(&f.Experience, "experience", nil, "LinkedIn experience-level codes (1=intern,2=entry,3=associate,4=mid-senior,5=director,6=executive)")
	return annotate(cmd, kindRead)
}

func newJobsGetCmd(d *deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch one job posting's full detail",
		Long: `Fetch the complete, trustworthy detail for one job posting by its id (from
` + "`linkedin jobs search`" + `): workRemoteAllowed/workplaceTypes, listedAt (posted epoch-ms),
applyMethod + companyApplyUrl, description.text, and the company URN.

Each fetch counts against the ban-safety daily job-detail cap (default 30/day).`,
		Example: `  linkedin jobs get 4012345678
  linkedin jobs get 4012345678 -o yaml
  linkedin jobs get 4012345678 --jq '.description.text'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := d.getAPIClient()
			if err != nil {
				return err
			}
			body, err := c.GetJob(cmd.Context(), args[0], time.Now())
			if err != nil {
				return err
			}
			if body == nil { // dry-run
				return nil
			}
			return d.render(cmd, body, nil)
		},
	}
	return annotate(cmd, kindRead)
}

// cardsToJSON marshals the parsed job cards into a JSON array for the renderer. Each element is
// the thin card (id/title/company/location/urn); the full record is available per-id via
// `linkedin jobs get`.
func cardsToJSON(cards []voyager.JobCard) json.RawMessage {
	if cards == nil {
		cards = []voyager.JobCard{}
	}
	b, _ := json.Marshal(cards)
	return b
}
