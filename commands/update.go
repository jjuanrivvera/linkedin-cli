package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/update"
	"github.com/jjuanrivvera/linkedin-cli/internal/version"
)

// newUpdater is a seam so tests can point the updater at an httptest GitHub API.
var newUpdater = func() *update.Updater { return update.NewUpdater(version.Version) }

func init() {
	metaRegistrars = append(metaRegistrars, func(_ *deps) *cobra.Command {
		cmd := &cobra.Command{
			Use:   "update",
			Short: "Update linkedin to the latest GitHub release",
			Long: `Download the latest linkedin release, verify it against checksums.txt, and replace
the running binary in place. Use 'linkedin update check' to see what's available without
installing.`,
			Example: `  linkedin update
  linkedin update check`,
			RunE: func(cmd *cobra.Command, _ []string) error {
				ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
				defer cancel()

				res := newUpdater().CheckAndUpdate(ctx)
				if res.Error != nil {
					return res.Error
				}
				if res.Updated {
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %s → %s. Restart to use the new version.\n", res.FromVersion, res.ToVersion)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Already on the latest version.")
				}
				return nil
			},
		}
		cmd.AddCommand(newUpdateCheckCmd())
		return cmd
	})
}

func newUpdateCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for a newer release without installing it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			rel, err := newUpdater().GetLatestRelease(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Current: %s\n", version.Version)
			fmt.Fprintf(out, "Latest:  %s\n", rel.TagName)
			switch {
			case version.Version == "dev" || version.Version == "":
				fmt.Fprintln(out, "This is a development build; self-update is disabled.")
			case strings.TrimPrefix(rel.TagName, "v") == strings.TrimPrefix(version.Version, "v"):
				fmt.Fprintln(out, "You are on the latest version.")
			default:
				fmt.Fprintln(out, "A newer version is available. Run `linkedin update` to install it.")
			}
			return nil
		},
	}
}
