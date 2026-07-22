## linkedin messages list

List your conversations (most recent first)

### Synopsis

List the most recent conversations in your inbox: conversation id, participant name(s),
last-activity time, and a snippet of the latest message. The full conversation entities are
available under -o json.

⚠ UNOFFICIAL API — ELEVATED ACCOUNT-RESTRICTION RISK. Messaging drives LinkedIn's private
legacy inbox endpoints with YOUR session. Automated messaging is the classic trigger for a
LinkedIn account restriction: keep volume very low, write like a human, and prefer reading
over sending. Sends are confirmation-gated and capped (default 20/day, --daily-send-cap).

```
linkedin messages list [flags]
```

### Examples

```
  linkedin messages list
  linkedin messages list --count 5 -o json
  linkedin messages list -o id
```

### Options

```
      --count int   conversations to fetch (default 20)
  -h, --help        help for list
```

### Options inherited from parent commands

```
      --all                   page through all results (search commands)
      --base-url string       override the Voyager API host (default https://www.linkedin.com/voyager/api)
      --columns strings       comma-separated columns to show
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

