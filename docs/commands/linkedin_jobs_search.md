## linkedin jobs search

Search job postings

### Synopsis

Search LinkedIn jobs by keywords, location, remoteness, recency, job type and experience.
Results paginate with --count/--limit/--all. Machine output (-o json / -o id / --jq) is the
primary interface for an assistant; -o table is the human view.

--location resolves a place NAME to a LinkedIn geoId via the typeahead (cached), then filters
with locationUnion:(geoId:<id>). "remote" is NOT a location — use --remote (workplaceType
List(2)). --since maps to LinkedIn's timePostedRange: 24h→r86400, 7d→r604800, 30d→r2592000.

```
linkedin jobs search [flags]
```

### Examples

```
  linkedin jobs search --keywords "golang" --remote --since 7d -o json
  linkedin jobs search --keywords "product design" --location "Bogota, Colombia" --limit 50
  linkedin jobs search --keywords backend --job-type F,C --experience 3,4 -o id
  linkedin jobs search --keywords sre --remote --since 24h --all -o json
```

### Options

```
      --experience strings   LinkedIn experience-level codes (1=intern,2=entry,3=associate,4=mid-senior,5=director,6=executive)
  -h, --help                 help for search
      --job-type strings     LinkedIn job-type codes (F=full-time,P=part-time,C=contract,T=temporary,I=internship,V=volunteer,O=other)
      --keywords string      role/skill keywords to match
      --location string      place name resolved to a geoId (e.g. "Bogota, Colombia"); NOT for remote — use --remote
      --remote               only remote roles (workplaceType List(2))
      --since string         only postings within this window: Nh/Nd/Nw (e.g. 24h, 7d, 2w) or day|week|month
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

