## linkedin

An agent-friendly CLI for LinkedIn job search and messaging (unofficial Voyager API)

### Synopsis

linkedin is a READ-FIRST client for LinkedIn's internal Voyager API: search jobs, fetch a
job's detail, look up a company, resolve a location to a geoId, and read your message inbox —
with machine-first output (JSON/YAML/CSV, -o id, --jq) for pipelines and AI agents. The ONE
write is `messages send`, which is confirmation-gated and daily-capped.

⚠ UNOFFICIAL API — USE-AT-YOUR-OWN-RISK. This drives the same private endpoints linkedin.com's
web app calls, using YOUR browser session cookies. It is not sanctioned by LinkedIn and may
violate the LinkedIn User Agreement. Ban-safety defaults are ON (human-paced delays, daily
fetch/send caps, no retry on throttles) — keep volume low, use your own account on your own
machine. Automated MESSAGING is the classic account-restriction trigger; treat `messages send`
as the riskiest command here.

Authenticate by borrowing your browser session:
  linkedin auth --cookie-from-browser chrome
  linkedin jobs search --keywords "golang" --remote --since 7d -o json
  linkedin jobs get 4012345678 -o json
  linkedin company get stripe
  linkedin geo "Bogota, Colombia"
  linkedin messages list

### Options

```
      --all                   page through all results (search commands)
      --base-url string       override the Voyager API host (default https://www.linkedin.com/voyager/api)
      --columns strings       comma-separated columns to show
      --count int             results per page (search commands) (default 25)
      --daily-cap int         override the ban-safety daily job-detail fetch cap (0 keeps the default of 30)
      --daily-send-cap int    override the ban-safety daily message-send cap (0 keeps the default of 20)
      --dry-run               print the equivalent curl and make no request
  -h, --help                  help for linkedin
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

* [linkedin agent](linkedin_agent.md)	 - AI-agent integration helpers
* [linkedin alias](linkedin_alias.md)	 - Manage user-defined command aliases
* [linkedin api](linkedin_api.md)	 - Send a raw Voyager GET request (read-only escape hatch)
* [linkedin auth](linkedin_auth.md)	 - Borrow a LinkedIn browser session (cookie auth)
* [linkedin company](linkedin_company.md)	 - Look up LinkedIn companies
* [linkedin completion](linkedin_completion.md)	 - Generate shell completion scripts
* [linkedin config](linkedin_config.md)	 - Inspect and edit linkedin configuration
* [linkedin doctor](linkedin_doctor.md)	 - Diagnose config, keyring, stored session, and ban-safety budget
* [linkedin geo](linkedin_geo.md)	 - Resolve a location name to a LinkedIn geoId
* [linkedin init](linkedin_init.md)	 - First-run setup wizard
* [linkedin jobs](linkedin_jobs.md)	 - Search and inspect LinkedIn jobs
* [linkedin mcp](linkedin_mcp.md)	 - MCP server management
* [linkedin messages](linkedin_messages.md)	 - Read and send LinkedIn messages (⚠ elevated ban risk)
* [linkedin update](linkedin_update.md)	 - Update linkedin to the latest GitHub release
* [linkedin version](linkedin_version.md)	 - Print version, commit, and build date

