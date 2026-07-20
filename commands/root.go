// Package commands wires the cobra command tree. root.go owns the global flags, the shared
// LinkedIn Voyager client factory (borrowed-session cookie auth + ban-safety pacing), and the
// single render() path used by every command. The tree is built fresh per NewRootCmd() call so
// tests never leak flag state across cases.
package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/api"
	"github.com/jjuanrivvera/linkedin-cli/internal/auth"
	"github.com/jjuanrivvera/linkedin-cli/internal/config"
	"github.com/jjuanrivvera/linkedin-cli/internal/output"
)

// globalFlags holds the persistent flag values for one command tree.
type globalFlags struct {
	outputFormat   string
	profile        string
	voyagerBaseURL string // overrides the Voyager API host
	webBaseURL     string // overrides the web (typeahead) host
	dryRun         bool
	showToken      bool
	verbose        bool
	noColor        bool
	columns        []string
	quiet          bool
	jq             string
	dailyCap       int // override the ban-safety daily job-detail cap (0 = keep default)

	// list flags (read by search commands)
	all   bool
	limit int
	count int
}

// deps carries the per-tree state into every command builder.
type deps struct {
	gf *globalFlags

	// overridable in tests
	loadConfig func() (*config.Config, error)
	store      func() auth.Store
	// newClient builds the API client; tests inject one pointed at an httptest server.
	newClient func(voyagerBase, webBase string, opts ...api.Option) *api.Client
	// out overrides where dry-run curls go (tests capture it; default os.Stdout).
	out io.Writer
}

func newDeps() *deps {
	return &deps{
		gf:         &globalFlags{},
		loadConfig: config.Load,
		store: func() auth.Store {
			dir, err := config.Dir()
			if err != nil {
				dir = "."
			}
			return auth.New(dir)
		},
		newClient: api.New,
	}
}

// NewRootCmd assembles the full command tree.
func NewRootCmd() *cobra.Command { return newRootCmd(newDeps()) }

// registrars build the resource commands; resource files append from init().
var registrars []func(d *deps) *cobra.Command

// metaRegistrars register the non-resource commands (auth, config, doctor, …).
var metaRegistrars []func(d *deps) *cobra.Command

func newRootCmd(d *deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "linkedin",
		Short: "A read-only, agent-friendly CLI for LinkedIn job search (unofficial Voyager API)",
		Long: `linkedin is a READ-ONLY client for LinkedIn's internal Voyager API: search jobs, fetch a
job's detail, look up a company, and resolve a location to a geoId — with machine-first output
(JSON/YAML/CSV, -o id, --jq) for pipelines and AI agents.

⚠ UNOFFICIAL API — USE-AT-YOUR-OWN-RISK. This drives the same private endpoints linkedin.com's
web app calls, using YOUR browser session cookies. It is not sanctioned by LinkedIn and may
violate the LinkedIn User Agreement. Ban-safety defaults are ON (human-paced delays, a daily
fetch cap, no retry on throttles) — keep volume low, use your own account on your own machine.

Authenticate by borrowing your browser session:
  linkedin auth --cookie-from-browser chrome
  linkedin jobs search --keywords "golang" --remote --since 7d -o json
  linkedin jobs get 4012345678 -o json
  linkedin company get stripe
  linkedin geo "Bogota, Colombia"`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if d.gf.outputFormat != "" && !output.Format(d.gf.outputFormat).Valid() {
				return fmt.Errorf("unknown output format %q (want table|json|yaml|csv|id)", d.gf.outputFormat)
			}
			if d.gf.profile != "" {
				if err := config.ValidateProfileName(d.gf.profile); err != nil {
					return err
				}
			}
			return nil
		},
	}
	registerGlobalFlags(root, d.gf)

	for _, build := range registrars {
		root.AddCommand(build(d))
	}
	for _, build := range metaRegistrars {
		root.AddCommand(build(d))
	}
	return root
}

func registerGlobalFlags(root *cobra.Command, gf *globalFlags) {
	pf := root.PersistentFlags()
	pf.StringVarP(&gf.outputFormat, "output", "o", "", "output format: table|json|yaml|csv|id")
	// LinkedIn is a single fixed service; a "profile" scopes a stored session + host overrides.
	pf.StringVar(&gf.profile, "profile", "", "named profile to use")
	pf.StringVar(&gf.voyagerBaseURL, "base-url", "", "override the Voyager API host (default https://www.linkedin.com/voyager/api)")
	pf.StringVar(&gf.webBaseURL, "web-base-url", "", "override the web host used for geo typeahead (default https://www.linkedin.com)")
	pf.BoolVar(&gf.dryRun, "dry-run", false, "print the equivalent curl and make no request")
	pf.BoolVar(&gf.showToken, "show-token", false, "reveal the session cookies in dry-run output")
	pf.BoolVarP(&gf.verbose, "verbose", "v", false, "verbose request logging (stderr)")
	pf.BoolVar(&gf.noColor, "no-color", false, "disable colored output")
	pf.StringSliceVar(&gf.columns, "columns", nil, "comma-separated columns to show")
	pf.BoolVar(&gf.quiet, "quiet", false, "suppress non-essential chatter")
	pf.StringVar(&gf.jq, "jq", "", "gojq expression applied to the response before rendering")
	pf.IntVar(&gf.dailyCap, "daily-cap", 0, "override the ban-safety daily job-detail fetch cap (0 keeps the default of 30)")

	pf.BoolVar(&gf.all, "all", false, "page through all results (search commands)")
	pf.IntVar(&gf.limit, "limit", 0, "max items to return across pages (search commands)")
	pf.IntVar(&gf.count, "count", 25, "results per page (search commands)")
}

// resolveProfile returns the active profile name and config.
func (d *deps) resolveProfile() (string, *config.Config, error) {
	cfg, err := d.loadConfig()
	if err != nil {
		return "", nil, err
	}
	return cfg.ResolveProfileName(d.gf.profile), cfg, nil
}

// getAPIClient builds a Voyager client for the ACTIVE profile: it resolves the hosts (flag > env
// > config > default), wires the borrowed-session cookie source (env LI_AT/JSESSIONID first, then
// the keyring), and installs the ban-safety pacer.
func (d *deps) getAPIClient() (*api.Client, *config.Config, error) {
	profileName, cfg, err := d.resolveProfile()
	if err != nil {
		return nil, nil, err
	}
	prof, _ := cfg.Profile(profileName)

	voyagerBase := config.FirstNonEmpty(d.gf.voyagerBaseURL, os.Getenv("LINKEDIN_BASE_URL"), prof.VoyagerBaseURL, api.DefaultVoyagerBaseURL)
	webBase := config.FirstNonEmpty(d.gf.webBaseURL, os.Getenv("LINKEDIN_WEB_BASE_URL"), prof.WebBaseURL, api.DefaultWebBaseURL)
	for _, u := range []string{voyagerBase, webBase} {
		if err := config.ValidateBaseURL(u); err != nil {
			return nil, nil, err
		}
	}

	dir, derr := config.Dir()
	if derr != nil {
		dir = "."
	}
	pacer := api.DefaultPacer(filepath.Join(dir, "state.json"))
	if d.gf.dailyCap > 0 {
		pacer.DailyCap = d.gf.dailyCap
	}

	store := d.store()
	opts := []api.Option{
		api.WithDryRun(d.gf.dryRun, d.stdout()),
		api.WithUserAgent(os.Getenv("LINKEDIN_USER_AGENT")),
		api.WithPacer(pacer),
		api.WithCookies(cookieSource(store, profileName, d.gf.dryRun)),
	}

	c := d.newClient(voyagerBase, webBase, opts...)
	c.ShowToken = d.gf.showToken
	c.Verbose = d.gf.verbose
	c.VerboseOut = os.Stderr
	return c, cfg, nil
}

// Dry-run cookie placeholders. A --dry-run is a fully offline preview that must work WITHOUT a
// stored session, so a missing session yields these instead of an error. printCurl redacts them to
// "REDACTED" anyway (unless --show-token), matching the placeholder intent.
const (
	dryRunLiAtPlaceholder     = "<REDACTED>"
	dryRunJSessionPlaceholder = `"<REDACTED>"`
)

// cookieSource returns a CookieFunc that resolves the borrowed session: env LI_AT/JSESSIONID
// (for headless use) take precedence, else the keyring-stored pair for the profile. When dryRun is
// set, a missing session is NOT an error — placeholder cookies keep the offline curl preview
// working before the user has authenticated.
func cookieSource(store auth.Store, profileName string, dryRun bool) api.CookieFunc {
	return func(_ context.Context) (string, string, error) {
		if liAt := os.Getenv("LI_AT"); liAt != "" {
			js := os.Getenv("JSESSIONID")
			if js == "" {
				return "", "", fmt.Errorf("LI_AT is set but JSESSIONID is not — both are required (JSESSIONID looks like \"ajax:123...\" WITH quotes)")
			}
			return liAt, js, nil
		}
		raw, err := store.Get(profileName)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				if dryRun {
					return dryRunLiAtPlaceholder, dryRunJSessionPlaceholder, nil
				}
				return "", "", fmt.Errorf("no LinkedIn session stored for profile %q — run "+
					"`linkedin auth --cookie-from-browser <chrome|brave|firefox>` or set LI_AT/JSESSIONID", profileName)
			}
			return "", "", err
		}
		creds, err := parseCredentials(raw)
		if err != nil {
			return "", "", err
		}
		return creds.LiAt, creds.JSessionID, nil
	}
}

func (d *deps) stdout() io.Writer {
	if d.out != nil {
		return d.out
	}
	return os.Stdout
}

// render is the single output path for every command.
func (d *deps) render(cmd *cobra.Command, v any, defaultColumns []string) error {
	raw, ok := v.(json.RawMessage)
	if !ok {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		raw = b
	}
	format := output.Format(config.FirstNonEmpty(d.gf.outputFormat, string(output.FormatTable)))
	cols := normalizeColumns(d.gf.columns)
	if len(cols) == 0 && format != output.FormatID {
		cols = defaultColumns
	}
	return output.Render(raw, output.Options{
		Format:  format,
		Columns: cols,
		NoColor: d.gf.noColor,
		Quiet:   d.gf.quiet,
		JQ:      d.gf.jq,
		Out:     cmd.OutOrStdout(),
		Err:     cmd.ErrOrStderr(),
	})
}

func normalizeColumns(cols []string) []string {
	var out []string
	for _, c := range cols {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}
