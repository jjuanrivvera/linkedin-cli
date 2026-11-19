## linkedin geo

Resolve a location name to a LinkedIn geoId

### Synopsis

Resolve a place name to LinkedIn geo hits via the unauthenticated jobs-guest typeahead,
so you can pass a geoId to a job search. The first hit is the best match; results are cached
in the config dir so a repeated lookup makes no request.

Note: "remote" is NOT a geo — it is a workplace type. Use `--remote` on jobs search.

```
linkedin geo <name> [flags]
```

### Examples

```
  linkedin geo "Bogota, Colombia"
  linkedin geo "San Francisco Bay Area" -o json
  linkedin geo Colombia --jq '.[0].id'
```

### Options

```
  -h, --help   help for geo
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

* [linkedin](linkedin.md)	 - An agent-friendly CLI for LinkedIn job search and messaging (unofficial Voyager API)

