package commands

import (
	"github.com/spf13/cobra"
)

func init() {
	registrars = append(registrars, func(d *deps) *cobra.Command {
		companyCmd := &cobra.Command{
			Use:     "company",
			Aliases: []string{"companies", "org"},
			Short:   "Look up LinkedIn companies",
			Long:    "Fetch a LinkedIn organization by its universalName (the slug in a company URL).",
		}
		companyCmd.AddCommand(newCompanyGetCmd(d))
		return companyCmd
	})
}

func newCompanyGetCmd(d *deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <slug>",
		Short: "Fetch a company by its universalName (slug)",
		Long: `Fetch a LinkedIn organization by its universalName — the slug in
linkedin.com/company/<slug> (e.g. "stripe"). Returns the full company record; the company URN
also appears on a job posting's detail, linking a role to its employer.`,
		Example: `  linkedin company get stripe
  linkedin company get google -o json
  linkedin company get stripe --jq '.name'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := d.getAPIClient()
			if err != nil {
				return err
			}
			body, err := c.GetCompany(cmd.Context(), args[0])
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
