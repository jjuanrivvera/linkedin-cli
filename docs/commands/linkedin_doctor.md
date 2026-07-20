## linkedin doctor

Diagnose config, keyring, stored session, and ban-safety budget

### Synopsis

Run local health checks: config file, keyring backend, whether a LinkedIn session is
stored and readable, and today's remaining job-detail budget. By default doctor makes NO
LinkedIn request (protecting your account). With --live it makes ONE low-risk, UNAUTHENTICATED
geo-typeahead request to confirm connectivity — it never touches an authenticated endpoint.
Exits non-zero when a check fails, so it is scriptable.

```
linkedin doctor [flags]
```

### Examples

```
  linkedin doctor
  linkedin doctor --json
  linkedin doctor --live
```

### Options

```
  -h, --help   help for doctor
      --json   output as JSON
      --live   also make ONE unauthenticated geo-typeahead request to check connectivity
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

