## linkedin jobs get

Fetch one job posting's full detail

### Synopsis

Fetch the complete, trustworthy detail for one job posting by its id (from
`linkedin jobs search`): workRemoteAllowed/workplaceTypes, listedAt (posted epoch-ms),
applyMethod + companyApplyUrl, description.text, and the company URN.

Each fetch counts against the ban-safety daily job-detail cap (default 30/day).

```
linkedin jobs get <id> [flags]
```

### Examples

```
  linkedin jobs get 4012345678
  linkedin jobs get 4012345678 -o yaml
  linkedin jobs get 4012345678 --jq '.description.text'
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

* [linkedin jobs](linkedin_jobs.md)	 - Search and inspect LinkedIn jobs

