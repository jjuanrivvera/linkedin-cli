## linkedin messages send

Send a text message to an existing conversation (⚠ ban-risk; confirmation-gated)

### Synopsis

Send one text message into an EXISTING conversation (id from `linkedin messages list`).

This is the single write operation in the CLI, and the riskiest thing it can do to your
account. Before sending it prints a warning and asks for interactive confirmation (skip with
--yes), charges the persisted daily send cap (default 20/day, --daily-send-cap), and never
retries a failed send. With --dry-run it prints the equivalent curl (cookies redacted) and
sends nothing.

⚠ UNOFFICIAL API — ELEVATED ACCOUNT-RESTRICTION RISK. Messaging drives LinkedIn's private
GraphQL messenger endpoints with YOUR session. Automated messaging is the classic trigger for
a LinkedIn account restriction: keep volume very low, write like a human, and prefer reading
over sending. Sends are confirmation-gated and capped (default 20/day, --daily-send-cap).

```
linkedin messages send <conversationId> --text <message> [flags]
```

### Examples

```
  linkedin messages send urn:li:msg_conversation:2-YWJjZGVm== --text "Thanks, talk soon!"
  linkedin messages send urn:li:msg_conversation:2-YWJjZGVm== --text "On my way" --yes
  linkedin messages send urn:li:msg_conversation:2-YWJjZGVm== --text "hello" --dry-run
```

### Options

```
  -h, --help          help for send
      --text string   message text to send (required)
      --yes           skip the interactive send confirmation
```

### Options inherited from parent commands

```
      --all                   page through all results (search commands)
      --base-url string       override the Voyager API host (default https://www.linkedin.com/voyager/api)
      --columns strings       comma-separated columns to show
      --count int             results per page (search commands) (default 25)
      --daily-cap int         override the ban-safety daily job-detail fetch cap (0 keeps the default of 30)
      --daily-send-cap int    override the ban-safety daily message-send cap (0 keeps the default of 20)
      --dry-run               print the equivalent curl and make no request
      --jq string             gojq expression applied to the response before rendering
      --limit int             max items to return across pages (search commands)
      --no-color              disable colored output
  -o, --output string         output format: table|json|yaml|csv|id
      --profile string        named profile to use
      --quiet                 suppress non-essential chatter
      --show-token            reveal the session cookies in dry-run output
  -v, --verbose               verbose request logging (stderr)
      --web-base-url string   override the web host used for geo typeahead (default https://www.linkedin.com)
```

### SEE ALSO

* [linkedin messages](linkedin_messages.md)	 - Read and send LinkedIn messages (⚠ elevated ban risk)

