## linkedin auth

Borrow a LinkedIn browser session (cookie auth)

### Synopsis

Store the LinkedIn session cookies (li_at + JSESSIONID) for the active profile in your OS
keyring (encrypted-file fallback on headless hosts, keyed by $LINKEDIN_KEYRING_PASSWORD).

The primary path borrows a live browser session — log in to linkedin.com in Chrome/Brave/
Firefox, then:
  linkedin auth --cookie-from-browser chrome

For headless use, pass the cookies directly (or via env LI_AT / JSESSIONID):
  linkedin auth --li-at "AQED..." --jsessionid '"ajax:1234567890"'

Sub-commands: status (whoami), logout.

```
linkedin auth [flags]
```

### Examples

```
  linkedin auth --cookie-from-browser chrome
  linkedin auth --cookie-from-browser firefox --profile work
  linkedin auth --li-at "AQED..." --jsessionid '"ajax:123..."'
```

### Options

```
      --cookie-from-browser string   extract the session from this browser: chrome|chromium|brave|edge|firefox
  -h, --help                         help for auth
      --jsessionid string            set the JSESSIONID cookie directly (value looks like "ajax:123..." WITH quotes)
      --li-at string                 set the li_at session cookie directly (headless; prefer --cookie-from-browser)
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
* [linkedin auth logout](linkedin_auth_logout.md)	 - Remove the stored LinkedIn session for the active profile
* [linkedin auth status](linkedin_auth_status.md)	 - Show the active profile and whether a LinkedIn session is stored

