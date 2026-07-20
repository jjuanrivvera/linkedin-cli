package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/auth"
	"github.com/jjuanrivvera/linkedin-cli/internal/browserauth"
)

// Credentials is the borrowed-session cookie pair. It is stored in the OS keyring as a single
// JSON blob under the profile key (the fleet-standard auth.Store persists one string per
// profile), never in the config file. JSESSIONID keeps its surrounding quotes.
type Credentials struct {
	LiAt       string `json:"li_at"`
	JSessionID string `json:"JSESSIONID"`
}

// linkedInDomain and the cookie names are the borrow-the-session parameters for LinkedIn.
const linkedInDomain = ".linkedin.com"

var requiredCookies = []string{"li_at", "JSESSIONID"}

// newExtractor is a seam so tests can inject a fake cookie source instead of reading a real
// browser via kooky.
var newExtractor = func(browsers []string) *browserauth.Extractor {
	return &browserauth.Extractor{
		Domain:              linkedInDomain,
		Browsers:            browsers,
		RequiredCookieNames: requiredCookies,
	}
}

func marshalCredentials(c Credentials) (string, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func parseCredentials(raw string) (Credentials, error) {
	var c Credentials
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, fmt.Errorf("stored session is corrupt (%w) — re-run `linkedin auth --cookie-from-browser <browser>`", err)
	}
	if c.LiAt == "" || c.JSessionID == "" {
		return c, fmt.Errorf("stored session is incomplete — re-run `linkedin auth --cookie-from-browser <browser>`")
	}
	return c, nil
}

func init() {
	metaRegistrars = append(metaRegistrars, func(d *deps) *cobra.Command {
		var fromBrowser, liAt, jsession string
		authCmd := &cobra.Command{
			Use:   "auth",
			Short: "Borrow a LinkedIn browser session (cookie auth)",
			Long: `Store the LinkedIn session cookies (li_at + JSESSIONID) for the active profile in your OS
keyring (encrypted-file fallback on headless hosts, keyed by $LINKEDIN_KEYRING_PASSWORD).

The primary path borrows a live browser session — log in to linkedin.com in Chrome/Brave/
Firefox, then:
  linkedin auth --cookie-from-browser chrome

For headless use, pass the cookies directly (or via env LI_AT / JSESSIONID):
  linkedin auth --li-at "AQED..." --jsessionid '"ajax:1234567890"'

Sub-commands: status (whoami), logout.`,
			Example: `  linkedin auth --cookie-from-browser chrome
  linkedin auth --cookie-from-browser firefox --profile work
  linkedin auth --li-at "AQED..." --jsessionid '"ajax:123..."'`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				profileName, cfg, err := d.resolveProfile()
				if err != nil {
					return err
				}
				var creds Credentials
				var source string
				switch {
				case fromBrowser != "":
					ex := newExtractor([]string{fromBrowser})
					got, src, eerr := ex.Extract(cmd.Context())
					if eerr != nil {
						return eerr
					}
					creds = Credentials{LiAt: got["li_at"], JSessionID: got["JSESSIONID"]}
					source = src
				case liAt != "" && jsession != "":
					creds = Credentials{LiAt: liAt, JSessionID: jsession}
					source = "manual"
				default:
					// Interactive: read the cookies with a HIDDEN prompt (li_at is a secret, so it
					// must never echo to the terminal). A read error (empty/piped stdin) is treated
					// as "not provided" so the actionable guidance below fires.
					creds.LiAt, _ = promptSecret(cmd, "li_at cookie: ")
					creds.JSessionID, _ = promptSecret(cmd, `JSESSIONID cookie (looks like "ajax:..."): `)
					if creds.LiAt == "" || creds.JSessionID == "" {
						return fmt.Errorf("no cookies provided — pass --cookie-from-browser <chrome|brave|firefox>, or --li-at and --jsessionid")
					}
					source = "manual"
				}

				blob, err := marshalCredentials(creds)
				if err != nil {
					return err
				}
				if err := d.store().Set(profileName, blob); err != nil {
					return err
				}
				prof, _ := cfg.Profile(profileName)
				prof.HasCookies = true
				prof.CookieSource = source
				if err := cfg.SetProfile(profileName, prof); err != nil {
					return err
				}
				if cfg.CurrentProfile == "" {
					cfg.CurrentProfile = profileName
				}
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Stored a LinkedIn session for profile %q (source: %s).\n", profileName, source)
				fmt.Fprintln(cmd.ErrOrStderr(), "Reminder: this is an UNOFFICIAL API. Keep volume low; ban-safety pacing is on by default.")
				return nil
			},
		}
		authCmd.Flags().StringVar(&fromBrowser, "cookie-from-browser", "", "extract the session from this browser: chrome|chromium|brave|edge|firefox")
		authCmd.Flags().StringVar(&liAt, "li-at", "", "set the li_at session cookie directly (headless; prefer --cookie-from-browser)")
		authCmd.Flags().StringVar(&jsession, "jsessionid", "", `set the JSESSIONID cookie directly (value looks like "ajax:123..." WITH quotes)`)

		authCmd.AddCommand(newAuthStatusCmd(d), newAuthLogoutCmd(d))
		return authCmd
	})
}

func newAuthStatusCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"whoami"},
		Short:   "Show the active profile and whether a LinkedIn session is stored",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profileName, cfg, err := d.resolveProfile()
			if err != nil {
				return err
			}
			store := d.store()
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "profile: %s\n", profileName)
			if raw, gerr := store.Get(profileName); gerr == nil {
				if _, perr := parseCredentials(raw); perr == nil {
					prof, _ := cfg.Profile(profileName)
					src := prof.CookieSource
					if src == "" {
						src = "unknown"
					}
					fmt.Fprintf(out, "session: stored (backend: %s, source: %s)\n", store.Backend(), src)
				} else {
					fmt.Fprintf(out, "session: stored but unreadable — %v\n", perr)
				}
			} else {
				fmt.Fprintln(out, "session: none — run `linkedin auth --cookie-from-browser <browser>`")
			}
			return nil
		},
	}
}

func newAuthLogoutCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Remove the stored LinkedIn session for the active profile",
		Example: "  linkedin auth logout\n  linkedin auth logout --profile work",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profileName, cfg, err := d.resolveProfile()
			if err != nil {
				return err
			}
			if err := d.store().Delete(profileName); err != nil && !errors.Is(err, auth.ErrNotFound) {
				return err
			}
			if prof, ok := cfg.Profile(profileName); ok && prof.HasCookies {
				prof.HasCookies = false
				prof.CookieSource = ""
				_ = cfg.SetProfile(profileName, prof)
				_ = cfg.Save()
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed any stored session for profile %q.\n", profileName)
			return nil
		},
	}
}
