---
name: linkedin-cli
description: Use this when you need to search LinkedIn jobs or inspect a LinkedIn job/company from the terminal — search postings by keywords, location (geoId-resolved), remoteness, recency, job type and experience; fetch one posting's trustworthy detail (remote-allowed, posted date, apply URL, description); or look up a company by slug. Read-only, unofficial Voyager API, borrows your browser session; ban-safety is on by default.
version: 0.1.0
homepage: https://github.com/jjuanrivvera/linkedin-cli
license: MIT
allowed-tools: Bash(linkedin:*)
metadata: {"openclaw":{"category":"jobs","emoji":"💼","requires":{"bins":["linkedin"],"env":["LI_AT","JSESSIONID"]},"install":[{"kind":"brew","formula":"jjuanrivvera/linkedin-cli/linkedin-cli","bins":["linkedin"]},{"kind":"go","package":"github.com/jjuanrivvera/linkedin-cli/cmd/linkedin@latest","bins":["linkedin"]}]}}
---

# linkedin — LinkedIn job-search CLI

`linkedin` is a **read-first** client for LinkedIn's internal **Voyager** API. Prefer it over raw
`curl`: it builds the tricky Rest.li job-search query correctly, borrows your browser session,
paces requests for ban-safety, and emits clean JSON / `-o id` / `--jq` output. It also reads your
message inbox; the one write, `messages send`, is confirmation-gated, daily-capped, and classified
destructive (never auto-approved by an agent).

## ⚠️ Unofficial API — ban-risk disclaimer (read first)

This uses LinkedIn's **private, unofficial** endpoints with the user's **own session cookies**. It
is **not** sanctioned by LinkedIn and may violate the [User Agreement](https://www.linkedin.com/legal/user-agreement);
automated access can get an account **rate-limited, restricted, or banned**. Same caveat class as
slackctl's `xoxc` session auth.

- Only ever use the **user's own account**, on **their own machine**, over a **residential IP**.
- **Keep volume low.** Ban-safety is on by default (3–15s paced delays, one request in flight, a
  ~30/day job-detail cap, **no retry** on HTTP 999/429/challenge). Do not try to defeat it.
- On a soft-block (HTTP `999`) or challenge, **STOP** — do not loop. The CLI says so and refuses to
  retry.

## Prerequisites
- Install: `go install github.com/jjuanrivvera/linkedin-cli/cmd/linkedin@latest` (or brew).
- Auth (once): `linkedin auth --cookie-from-browser chrome` (log in to linkedin.com in that browser
  first). Headless: `export LI_AT=… JSESSIONID='"ajax:…"'` (the quotes are part of JSESSIONID).
- `linkedin doctor` checks config/session/budget locally; `linkedin doctor --live` adds one
  low-risk, unauthenticated geo probe.

## Golden rules
1. **`--location` is a place name, resolved to a geoId** (`locationUnion:(geoId:…)`). "remote" is
   NOT a location — use `--remote` (`workplaceType:List(2)`).
2. **`--since` is a window**, not a date: `24h`/`7d`/`2w` → `timePostedRange` (r86400/r604800/…).
3. **`jobs search` returns thin cards** (id/title/company/location). For the trustworthy fields
   (remote-allowed, posted epoch, apply URL, description), fetch `jobs get <id>` — but it counts
   against the daily cap, so batch deliberately.
4. **Emit machine output**: `-o json`, `-o id` (pipe to `xargs`), or slice with `--jq`.
5. **If a search errors with "schema moved — check schema.go"**, LinkedIn rotated its Voyager schema;
   that's a maintenance signal, not "no results".

## Workflow (discover → inspect → match)

```sh
# 1. Discover — recent remote Go roles, ids only
linkedin jobs search --keywords golang --remote --since 7d -o id

# 2. Narrow by location (resolved to geoId) or job type / experience
linkedin geo "Bogota, Colombia"
linkedin jobs search --keywords backend --location "Bogota, Colombia" --job-type F,C --limit 50 -o json

# 3. Inspect one posting's trustworthy detail
linkedin jobs get 4012345678 -o json
linkedin jobs get 4012345678 --jq '{remote: .workRemoteAllowed, posted: .listedAt, apply: .companyApplyUrl}'

# 4. Company context
linkedin company get stripe --jq '{name, staffCount}'

# See the exact request without sending it
linkedin jobs search --keywords go --remote --dry-run
```

## Command cheatsheet
- `linkedin jobs search --keywords … [--location …] [--remote] [--since 7d] [--job-type F,C] [--experience 3,4] [--limit N|--all]`
- `linkedin jobs get <id>` · `linkedin company get <slug>` · `linkedin geo <name>`
- `linkedin auth --cookie-from-browser <chrome|brave|firefox>` · `auth status` · `auth logout`
- `linkedin api <PATH> -q k=v` (GET-only raw escape hatch) · `linkedin doctor [--live]`
- Global: `-o table|json|yaml|csv|id`, `--jq`, `--columns`, `--dry-run`, `--daily-cap N`

## Troubleshooting
- **401/403 or "session expired"** → `linkedin auth --cookie-from-browser <browser>` (re-login first).
- **HTTP 999 / challenge** → you're being throttled/flagged. STOP, wait, lower volume.
- **Empty/`ErrSchemaMoved`** → check `internal/voyager/schema.go` (decorationId / `$type` drift).

See `references/` for auth-and-config, the LinkedIn command deep-dive, and output/filtering.
