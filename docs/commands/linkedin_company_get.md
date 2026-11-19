## linkedin company get

Fetch a company by its universalName (slug)

### Synopsis

Fetch a LinkedIn organization by its universalName — the slug in
linkedin.com/company/<slug> (e.g. "stripe"). Returns the full company record; the company URN
also appears on a job posting's detail, linking a role to its employer.

```
linkedin company get <slug> [flags]
```

### Examples

```
  linkedin company get stripe
  linkedin company get google -o json
  linkedin company get stripe --jq '.name'
```

### Options

```
  -h, --help   help for get
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

* [linkedin company](linkedin_company.md)	 - Look up LinkedIn companies

