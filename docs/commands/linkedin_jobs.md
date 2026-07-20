## linkedin jobs

Search and inspect LinkedIn jobs

### Synopsis

Search LinkedIn job postings and fetch a single posting's full detail.

### Options

```
  -h, --help   help for jobs
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
* [linkedin jobs get](linkedin_jobs_get.md)	 - Fetch one job posting's full detail
* [linkedin jobs search](linkedin_jobs_search.md)	 - Search job postings

