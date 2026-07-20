package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	metaRegistrars = append(metaRegistrars, func(d *deps) *cobra.Command {
		var fromBrowser string
		cmd := &cobra.Command{
			Use:     "init",
			Aliases: []string{"setup"},
			Short:   "First-run setup wizard",
			Long: `Walk through linkedin setup: read the unofficial-API caveat, then borrow your browser
session so the CLI can call Voyager. Pass --cookie-from-browser to capture cookies
non-interactively.`,
			Example: `  linkedin init
  linkedin init --cookie-from-browser chrome`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				out := cmd.OutOrStdout()
				profileName, cfg, err := d.resolveProfile()
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "linkedin setup — profile %q\n", profileName)
				fmt.Fprintln(out, "")
				fmt.Fprintln(out, "⚠ This tool uses LinkedIn's UNOFFICIAL Voyager API with YOUR browser session.")
				fmt.Fprintln(out, "  It may violate the LinkedIn User Agreement and risks account restriction.")
				fmt.Fprintln(out, "  Ban-safety pacing is on by default. Use your own account, low volume, own machine.")
				fmt.Fprintln(out, "")

				browser := fromBrowser
				if browser == "" {
					// A closed/empty stdin (non-interactive) means "skip" — never fail setup on it.
					ans, _ := promptLine(cmd, "Which browser are you logged into LinkedIn on? [chrome/brave/firefox, or blank to skip]: ")
					browser = strings.TrimSpace(ans)
				}
				if browser == "" {
					fmt.Fprintln(out, "Skipped session capture. Run `linkedin auth --cookie-from-browser <browser>` when ready.")
					return nil
				}

				ex := newExtractor([]string{browser})
				got, src, eerr := ex.Extract(cmd.Context())
				if eerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "could not capture a session: %v\n", eerr)
					return nil
				}
				blob, err := marshalCredentials(Credentials{LiAt: got["li_at"], JSessionID: got["JSESSIONID"]})
				if err != nil {
					return err
				}
				if err := d.store().Set(profileName, blob); err != nil {
					return err
				}
				prof, _ := cfg.Profile(profileName)
				prof.HasCookies = true
				prof.CookieSource = src
				_ = cfg.SetProfile(profileName, prof)
				if cfg.CurrentProfile == "" {
					cfg.CurrentProfile = profileName
				}
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Fprintf(out, "Stored a LinkedIn session (source: %s). You're ready:\n", src)
				fmt.Fprintln(out, "  linkedin jobs search --keywords golang --remote --since 7d -o json")
				return nil
			},
		}
		cmd.Flags().StringVar(&fromBrowser, "cookie-from-browser", "", "capture the session from this browser: chrome|chromium|brave|edge|firefox")
		return cmd
	})
}
