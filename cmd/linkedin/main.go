// Command linkedin is a read-only, agent-friendly CLI for LinkedIn job search via the unofficial
// Voyager API. It borrows a browser session (li_at + JSESSIONID cookies) and paces requests for
// ban-safety. See the README for the ToS / account-risk disclaimer.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jjuanrivvera/linkedin-cli/commands"
	"github.com/jjuanrivvera/linkedin-cli/internal/output"
	"github.com/jjuanrivvera/linkedin-cli/internal/version"
)

func main() {
	// signal.NotifyContext makes Ctrl-C (SIGINT/SIGTERM) cancel in-flight work: pagination, the
	// ban-safety inter-request delay, and retry backoff all observe this context.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	root := commands.NewRootCmd()
	root.Version = version.Get().Version
	root.SetVersionTemplate(version.String() + "\n")

	// Expand user-defined aliases BEFORE cobra parses, so an alias can map to any command
	// without shadowing a built-in.
	root.SetArgs(commands.ExpandAliases(os.Args[1:]))

	if err := root.ExecuteContext(ctx); err != nil {
		// Error text can carry API-returned free text (a job title, a company name); strip
		// terminal escapes before printing so a crafted value can't hijack the terminal.
		fmt.Fprintln(os.Stderr, "Error:", output.SanitizeTerminal(err.Error()))
		os.Exit(1)
	}
}
