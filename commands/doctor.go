package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/config"
	"github.com/jjuanrivvera/linkedin-cli/internal/version"
)

// doctorCheck is one diagnostic result.
type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func init() {
	metaRegistrars = append(metaRegistrars, func(d *deps) *cobra.Command {
		var jsonOut, live bool
		cmd := &cobra.Command{
			Use:   "doctor",
			Short: "Diagnose config, keyring, stored session, and ban-safety budget",
			Long: `Run local health checks: config file, keyring backend, whether a LinkedIn session is
stored and readable, and today's remaining job-detail budget. By default doctor makes NO
LinkedIn request (protecting your account). With --live it makes ONE low-risk, UNAUTHENTICATED
geo-typeahead request to confirm connectivity — it never touches an authenticated endpoint.
Exits non-zero when a check fails, so it is scriptable.`,
			Example: `  linkedin doctor
  linkedin doctor --json
  linkedin doctor --live`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				checks := d.runDoctor(cmd, live)
				failed := false
				for _, c := range checks {
					if !c.OK {
						failed = true
					}
				}
				if jsonOut {
					b, err := json.MarshalIndent(checks, "", "  ")
					if err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
				} else {
					for _, c := range checks {
						mark := "✓"
						if !c.OK {
							mark = "✗"
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s %-14s %s\n", mark, c.Name, c.Detail)
					}
				}
				if failed {
					return fmt.Errorf("doctor found problems")
				}
				return nil
			},
		}
		cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
		cmd.Flags().BoolVar(&live, "live", false, "also make ONE unauthenticated geo-typeahead request to check connectivity")
		return cmd
	})
}

func (d *deps) runDoctor(cmd *cobra.Command, live bool) []doctorCheck {
	var checks []doctorCheck
	add := func(name string, ok bool, detail string) {
		checks = append(checks, doctorCheck{Name: name, OK: ok, Detail: detail})
	}

	add("version", true, version.String())

	cfgPath, err := config.Path()
	if err != nil {
		add("config", false, err.Error())
		return checks
	}
	cfg, err := d.loadConfig()
	if err != nil {
		add("config", false, err.Error())
		return checks
	}
	add("config", true, cfgPath)

	profileName := cfg.ResolveProfileName(d.gf.profile)
	add("profile", true, profileName)

	store := d.store()
	if raw, gerr := store.Get(profileName); gerr == nil {
		if _, perr := parseCredentials(raw); perr == nil {
			add("session", true, "stored (backend: "+store.Backend()+")")
		} else {
			add("session", false, "stored but unreadable — re-run auth")
		}
	} else {
		add("session", false, "none — run `linkedin auth --cookie-from-browser <browser>`")
	}

	c, _, err := d.getAPIClient()
	if err != nil {
		add("client", false, err.Error())
		return checks
	}
	if p := c.Pacer(); p != nil {
		add("daily-budget", true, fmt.Sprintf("%d job-detail fetches left today", p.DailyRemaining(time.Now())))
		add("send-budget", true, fmt.Sprintf("%d message sends left today", p.DailySendRemaining(time.Now())))
	}

	if !live {
		add("connectivity", true, "skipped (pass --live for a low-risk geo probe)")
		return checks
	}
	hits, err := c.ResolveGeo(cmd.Context(), "Colombia")
	switch {
	case err != nil:
		add("connectivity", false, err.Error())
	case hits == nil:
		add("connectivity", true, "dry-run")
	default:
		add("connectivity", true, fmt.Sprintf("geo typeahead OK (%s)", c.WebBaseURL()))
	}
	return checks
}
