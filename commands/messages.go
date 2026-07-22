package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// messagesRiskWarning is the shared help-text warning for the messaging surface — same tone
// as the root command's disclaimer, sharpened for messaging because automated DMs are the
// classic LinkedIn account-restriction trigger.
const messagesRiskWarning = `⚠ UNOFFICIAL API — ELEVATED ACCOUNT-RESTRICTION RISK. Messaging drives LinkedIn's private
legacy inbox endpoints with YOUR session. Automated messaging is the classic trigger for a
LinkedIn account restriction: keep volume very low, write like a human, and prefer reading
over sending. Sends are confirmation-gated and capped (default 20/day, --daily-send-cap).`

var (
	conversationsColumns = []string{"id", "participants", "lastActivity", "snippet"}
	threadColumns        = []string{"sender", "time", "text"}
)

func init() {
	registrars = append(registrars, func(d *deps) *cobra.Command {
		messagesCmd := &cobra.Command{
			Use:     "messages",
			Aliases: []string{"message"},
			Short:   "Read and send LinkedIn messages (⚠ elevated ban risk)",
			Long: `List your conversations, read a thread, and send a text message via LinkedIn's legacy
Voyager messaging endpoints.

` + messagesRiskWarning,
		}
		messagesCmd.AddCommand(newMessagesListCmd(d), newMessagesReadCmd(d), newMessagesSendCmd(d))
		return messagesCmd
	})
}

func newMessagesListCmd(d *deps) *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your conversations (most recent first)",
		Long: `List the most recent conversations in your inbox: conversation id, participant name(s),
last-activity time, and a snippet of the latest message. The full conversation entities are
available under -o json.

` + messagesRiskWarning,
		Example: `  linkedin messages list
  linkedin messages list --count 5 -o json
  linkedin messages list -o id`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, _, err := d.getAPIClient()
			if err != nil {
				return err
			}
			res, err := c.ListConversations(cmd.Context(), count)
			if err != nil {
				return err
			}
			if res == nil { // dry-run
				return nil
			}
			return d.render(cmd, conversationsToJSON(res.Conversations), conversationsColumns)
		},
	}
	cmd.Flags().IntVar(&count, "count", 20, "conversations to fetch")
	return annotate(cmd, kindRead)
}

func newMessagesReadCmd(d *deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <conversationId>",
		Short: "Print one conversation's message thread (oldest first)",
		Long: `Print the message thread of one conversation — sender, time, and text — oldest→newest.
The conversation id comes from ` + "`linkedin messages list`" + `. Full event entities are available
under -o json.

` + messagesRiskWarning,
		Example: `  linkedin messages read 2-YWJjZGVm==
  linkedin messages read 2-YWJjZGVm== -o json --jq '.[].text'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := d.getAPIClient()
			if err != nil {
				return err
			}
			res, err := c.GetConversationEvents(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if res == nil { // dry-run
				return nil
			}
			return d.render(cmd, messagesToJSON(res.Messages), threadColumns)
		},
	}
	return annotate(cmd, kindRead)
}

func newMessagesSendCmd(d *deps) *cobra.Command {
	var text string
	var yes bool
	cmd := &cobra.Command{
		Use:   "send <conversationId> --text <message>",
		Short: "Send a text message to an existing conversation (⚠ ban-risk; confirmation-gated)",
		Long: `Send one text message into an EXISTING conversation (id from ` + "`linkedin messages list`" + `).

This is the single write operation in the CLI, and the riskiest thing it can do to your
account. Before sending it prints a warning and asks for interactive confirmation (skip with
--yes), charges the persisted daily send cap (default 20/day, --daily-send-cap), and never
retries a failed send. With --dry-run it prints the equivalent curl (cookies redacted) and
sends nothing.

` + messagesRiskWarning,
		Example: `  linkedin messages send 2-YWJjZGVm== --text "Thanks, talk soon!"
  linkedin messages send 2-YWJjZGVm== --text "On my way" --yes
  linkedin messages send 2-YWJjZGVm== --text "hello" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("--text must be a non-empty message")
			}
			c, _, err := d.getAPIClient()
			if err != nil {
				return err
			}
			if !d.gf.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "⚠ You are about to SEND a LinkedIn message as YOUR account.")
				fmt.Fprintln(cmd.ErrOrStderr(), "  Automated messaging is the classic LinkedIn account-restriction trigger — keep volume low.")
				if !yes {
					ans, perr := promptLine(cmd, "  Send it? [y/N]: ")
					if perr != nil || !isYes(ans) {
						return fmt.Errorf("aborted — message NOT sent (confirm with y, or pass --yes)")
					}
				}
			}
			raw, err := c.SendMessage(cmd.Context(), args[0], text, time.Now())
			if err != nil {
				return err
			}
			if d.gf.dryRun {
				return nil
			}
			if !d.gf.quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "✓ message sent")
			}
			if len(raw) > 0 && d.gf.outputFormat != "" {
				return d.render(cmd, raw, nil)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "message text to send (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive send confirmation")
	// The one write in the CLI, and an irreversible one (a sent DM cannot be unsent) — the
	// destructive classification keeps it OUT of every agent's safe surface: the MCP tool
	// carries destructiveHint and `agent guard` hard-blocks it (fail-closed either way).
	return annotate(cmd, kindDestructive)
}

// conversationRow flattens a conversation for the table/csv renderer; Entity preserves the
// complete Voyager entity for -o json.
type conversationRow struct {
	ID           string          `json:"id"`
	Participants string          `json:"participants"`
	LastActivity string          `json:"lastActivity"`
	Snippet      string          `json:"snippet"`
	URN          string          `json:"urn,omitempty"`
	Entity       json.RawMessage `json:"entity,omitempty"`
}

func conversationsToJSON(convs []voyager.Conversation) json.RawMessage {
	rows := make([]conversationRow, 0, len(convs))
	for _, c := range convs {
		rows = append(rows, conversationRow{
			ID:           c.ID,
			Participants: strings.Join(c.Participants, ", "),
			LastActivity: formatEpochMs(c.LastActivityAt),
			Snippet:      c.Snippet,
			URN:          c.URN,
			Entity:       c.Raw,
		})
	}
	b, _ := json.Marshal(rows)
	return b
}

// messageRow flattens one thread event; Entity preserves the full event for -o json.
type messageRow struct {
	Sender string          `json:"sender"`
	Time   string          `json:"time"`
	Text   string          `json:"text"`
	URN    string          `json:"urn,omitempty"`
	Entity json.RawMessage `json:"entity,omitempty"`
}

func messagesToJSON(msgs []voyager.Message) json.RawMessage {
	rows := make([]messageRow, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, messageRow{
			Sender: m.Sender,
			Time:   formatEpochMs(m.CreatedAt),
			Text:   m.Text,
			URN:    m.URN,
			Entity: m.Raw,
		})
	}
	b, _ := json.Marshal(rows)
	return b
}

// formatEpochMs renders a LinkedIn epoch-ms timestamp in local time; zero stays empty.
func formatEpochMs(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04")
}

func isYes(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}
