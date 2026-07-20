package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jjuanrivvera/linkedin-cli/internal/config"
)

func init() {
	metaRegistrars = append(metaRegistrars, func(d *deps) *cobra.Command {
		cfgCmd := &cobra.Command{
			Use:   "config",
			Short: "Inspect and edit linkedin configuration",
			Long: `The config file holds only non-secret settings (profiles, host overrides, aliases).
Any bearer token lives in the OS keyring — never here.`,
		}
		cfgCmd.AddCommand(
			newConfigPathCmd(),
			newConfigViewCmd(d),
			newConfigUseCmd(d),
			newConfigListProfilesCmd(d),
			newConfigSetCmd(d),
		)
		return cfgCmd
	})
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func newConfigViewCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show the resolved configuration",
		Long:  "Print the config as YAML. No secrets are stored in the config, so nothing needs redacting.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := d.loadConfig()
			if err != nil {
				return err
			}
			b, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(b)
			return err
		},
	}
}

func newConfigUseCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set the default profile for future invocations",
		Example: `  linkedin config use default
  linkedin config use work`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateProfileName(args[0]); err != nil {
				return err
			}
			cfg, err := d.loadConfig()
			if err != nil {
				return err
			}
			cfg.CurrentProfile = args[0]
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "default profile is now %q\n", args[0])
			return nil
		},
	}
}

func newConfigListProfilesCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:     "list-profiles",
		Aliases: []string{"profiles"},
		Short:   "List configured profiles",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := d.loadConfig()
			if err != nil {
				return err
			}
			names := cfg.ProfileNames()
			sort.Strings(names)
			for _, n := range names {
				marker := " "
				if n == cfg.CurrentProfile {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", marker, n)
			}
			return nil
		},
	}
}

// configSetKeys are the per-profile keys `config set` accepts.
var configSetKeys = map[string]func(*config.Profile, string){
	"voyager_base_url": func(p *config.Profile, v string) { p.VoyagerBaseURL = v },
	"web_base_url":     func(p *config.Profile, v string) { p.WebBaseURL = v },
}

func newConfigSetCmd(d *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a per-profile option (voyager_base_url, web_base_url)",
		Long: `Set a non-secret host override on the ACTIVE profile (--profile selects it).
Keys: voyager_base_url (default https://www.linkedin.com/voyager/api), web_base_url
(default https://www.linkedin.com).`,
		Example: `  linkedin config set voyager_base_url https://www.linkedin.com/voyager/api --profile default`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			setter, ok := configSetKeys[args[0]]
			if !ok {
				return fmt.Errorf("unknown key %q (want voyager_base_url|web_base_url)", args[0])
			}
			if args[1] != "" {
				if err := config.ValidateBaseURL(args[1]); err != nil {
					return err
				}
			}
			profileName, cfg, err := d.resolveProfile()
			if err != nil {
				return err
			}
			prof, _ := cfg.Profile(profileName)
			setter(&prof, args[1])
			if err := cfg.SetProfile(profileName, prof); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %q on profile %q\n", args[0], args[1], profileName)
			return nil
		},
	}
}
