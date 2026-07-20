package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	metaRegistrars = append(metaRegistrars, func(d *deps) *cobra.Command {
		var query []string
		cmd := &cobra.Command{
			Use:   "api <PATH> [-q key=value ...]",
			Short: "Send a raw Voyager GET request (read-only escape hatch)",
			Long: `Call any Voyager endpoint directly (relative to https://www.linkedin.com/voyager/api).
This is the documented escape hatch for anything linkedin does not wrap as a first-class
command. It is GET-ONLY by design — this is a read-only client and driving writes against the
unofficial API is not supported. Honors --dry-run, -o/--output, and --jq.`,
			Example: `  linkedin api me
  linkedin api jobs/jobPostings/4012345678 -q decorationId=com.linkedin...WebFullJobPosting-65
  linkedin api organization/companies -q q=universalName -q universalName=stripe`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				path := strings.TrimLeft(args[0], "/")
				q := url.Values{}
				for _, kv := range query {
					k, v, ok := strings.Cut(kv, "=")
					if !ok {
						return fmt.Errorf("invalid -q %q (want key=value)", kv)
					}
					q.Add(k, v)
				}
				c, _, err := d.getAPIClient()
				if err != nil {
					return err
				}
				status, body, err := c.Do(cmd.Context(), path, q)
				if err != nil {
					return err
				}
				if status == 0 { // dry-run
					return nil
				}
				if len(body) == 0 {
					if !d.gf.quiet {
						fmt.Fprintf(cmd.OutOrStdout(), "HTTP %d (empty body)\n", status)
					}
					return nil
				}
				if json.Valid(body) {
					return d.render(cmd, body, nil)
				}
				_, err = cmd.OutOrStdout().Write(body)
				return err
			},
		}
		cmd.Flags().StringArrayVarP(&query, "query", "q", nil, "query parameter key=value (repeatable)")
		// Read-only: the escape hatch only issues GETs, so it is safe to expose as a read verb.
		return annotate(cmd, kindRead)
	})
}
