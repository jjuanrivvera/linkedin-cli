## linkedin config set

Set a per-profile option (voyager_base_url, web_base_url)

### Synopsis

Set a non-secret host override on the ACTIVE profile (--profile selects it).
Keys: voyager_base_url (default https://www.linkedin.com/voyager/api), web_base_url
(default https://www.linkedin.com).

```
linkedin config set <key> <value> [flags]
```

### Examples

```
  linkedin config set voyager_base_url https://www.linkedin.com/voyager/api --profile default
```

### Options

```
  -h, --help   help for set
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

* [linkedin config](linkedin_config.md)	 - Inspect and edit linkedin configuration

