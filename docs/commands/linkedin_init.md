## linkedin init

First-run setup wizard

### Synopsis

Walk through linkedin setup: read the unofficial-API caveat, then borrow your browser
session so the CLI can call Voyager. Pass --cookie-from-browser to capture cookies
non-interactively.

```
linkedin init [flags]
```

### Examples

```
  linkedin init
  linkedin init --cookie-from-browser chrome
```

### Options

```
      --cookie-from-browser string   capture the session from this browser: chrome|chromium|brave|edge|firefox
  -h, --help                         help for init
```

### Options inherited from parent commands

```
      --all                   page through all results (search commands)
      --base-url string       override the Voyager API host (default https://www.linkedin.com/voyager/api)
      --columns strings       comma-separated columns to show
      --count int             results per page (search commands) (default 25)
      --daily-cap int         override the ban-safety daily job-detail fetch cap (0 keeps the default of 30)
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

* [linkedin](linkedin.md)	 - A read-only, agent-friendly CLI for LinkedIn job search (unofficial Voyager API)

