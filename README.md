<div align="center">

# linkedin

[![CI](https://github.com/jjuanrivvera/linkedin-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/jjuanrivvera/linkedin-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jjuanrivvera/linkedin-cli)](https://github.com/jjuanrivvera/linkedin-cli/releases/latest)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](https://github.com/jjuanrivvera/linkedin-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jjuanrivvera/linkedin-cli.svg)](https://pkg.go.dev/github.com/jjuanrivvera/linkedin-cli)
[![Go version](https://img.shields.io/github/go-mod/go-version/jjuanrivvera/linkedin-cli)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/jjuanrivvera/linkedin-cli)
[![Built with cliwright](https://img.shields.io/badge/built_with-cliwright-1f6feb)](https://cliwright.jjuanrivvera.com)

**LinkedIn from your terminal — job search, companies, geo lookup and messaging over the unofficial Voyager API, agent-friendly output (JSON/YAML/CSV/MCP).**

[Documentation](https://jjuanrivvera.github.io/linkedin-cli/) · [Command reference](https://jjuanrivvera.github.io/linkedin-cli/commands/linkedin/)

</div>

`linkedin` is a fast, scriptable, **read-first** command-line client for LinkedIn's internal
**Voyager** API: search jobs, fetch a job's full detail, look up a company, resolve a location
to a geoId, and read your message inbox — with machine-first output (JSON/YAML/CSV, `-o id`,
`--jq`) built for shell pipelines and AI agents. The one write, `messages send`, is
confirmation-gated and daily-capped.

> ## ⚠️ Unofficial API — use at your own risk
>
> This tool drives the **same private endpoints the linkedin.com web app calls**, authenticated
> with **your own browser session cookies**. It is **not** provided, sanctioned, or supported by
> LinkedIn, and using it **may violate the [LinkedIn User Agreement](https://www.linkedin.com/legal/user-agreement)**.
> Automated access can lead to your account being **rate-limited, restricted, or banned**.
>
> This is the same caveat class as `slackctl`'s `xoxc`/session auth. To stay on the safe side:
>
> - **Use your own account**, on **your own machine**, over a **residential IP** — never a shared
>   or server IP.
> - **Keep volume low.** Ban-safety is **on by default** (human-paced 3–15s delays, one request in
>   flight, a ~30/day job-detail cap, a **20/day message-send cap**, and **no retry** on
>   throttle/soft-block/challenge or on any send). Don't defeat it.
> - **Messaging is the riskiest thing here.** Automated messaging is the classic account-restriction
>   trigger. `messages send` warns, asks for confirmation (skip with `--yes`), and refuses once the
>   daily send cap is spent. Prefer reading over sending.
> - Treat this as a personal job-hunt helper, not a scraper. If LinkedIn soft-blocks you (HTTP
>   `999`) or issues a challenge, **stop** — the CLI will tell you to.
>
> You are responsible for how you use it.

## Install

```sh
# From source (Go 1.25+)
go install github.com/jjuanrivvera/linkedin-cli/cmd/linkedin@latest

# Or via Homebrew (once released)
brew install jjuanrivvera/linkedin-cli/linkedin-cli
```

The zero-infra install script (checksum-verified) is the first-class path once a release is tagged:

```sh
curl -fsSL https://raw.githubusercontent.com/jjuanrivvera/linkedin-cli/main/install.sh | sh
```

## Authenticate — borrow your browser session

Log in to linkedin.com in Chrome/Brave/Firefox, then hand the CLI your session cookies:

```sh
linkedin auth --cookie-from-browser chrome
```

It extracts the `li_at` and `JSESSIONID` cookies for `.linkedin.com` and stores them in your OS
keyring (AES-256-GCM encrypted-file fallback on headless hosts, keyed by `LINKEDIN_KEYRING_PASSWORD`).

Headless? Pass them directly or via env:

```sh
export LI_AT='AQED...'
export JSESSIONID='"ajax:1234567890"'   # NOTE: the quotes are part of the value
linkedin jobs search --keywords go --remote
```

`linkedin auth status` shows what's stored; `linkedin auth logout` removes it.

## Usage

```sh
# Search — recent remote Go roles as JSON
linkedin jobs search --keywords "golang" --remote --since 7d -o json

# Resolve a location to a geoId, then filter by it
linkedin geo "Bogota, Colombia"                       # → 90009706
linkedin jobs search --keywords backend --location "Bogota, Colombia" --limit 50

# Filter by job type / experience (LinkedIn codes) and page through results
linkedin jobs search --keywords sre --job-type F,C --experience 3,4 --all -o id

# One job's trustworthy detail (workRemoteAllowed, listedAt, applyMethod, description)
linkedin jobs get 4012345678 -o json
linkedin jobs get 4012345678 --jq '.description.text'

# A company by its slug (universalName)
linkedin company get stripe --jq '.name'

# Messaging (read-first; sending is the riskiest command — warns, confirms, and is capped)
linkedin messages list --count 10 -o json
linkedin messages read 2-YWJjZGVm==
linkedin messages send 2-YWJjZGVm== --text "Thanks, talk soon!" --dry-run

# See the exact request without sending it
linkedin jobs search --keywords go --remote --since 24h --dry-run
```

### Flags that matter

| Flag | Effect |
|---|---|
| `--keywords` | free-text role/skill match |
| `--location <name>` | resolves a place name → geoId → `locationUnion:(geoId:…)`. **"remote" is not a location** — use `--remote`. |
| `--remote` | remote-only (`workplaceType:List(2)`) |
| `--since <Nh/Nd/Nw>` | `timePostedRange` window (24h→r86400, 7d→r604800, 30d→r2592000) |
| `--job-type F,C,…` | LinkedIn job-type codes (F=full-time, P=part-time, C=contract, T=temporary, I=internship, V=volunteer, O=other) |
| `--experience 3,4,…` | LinkedIn experience codes (1=intern, 2=entry, 3=associate, 4=mid-senior, 5=director, 6=executive) |
| `--limit` / `--count` / `--all` | pagination controls |
| `-o table\|json\|yaml\|csv\|id`, `--jq`, `--columns`, `--dry-run` | output & inspection |
| `--daily-cap N` | raise the ban-safety daily job-detail cap (default 30) |
| `--daily-send-cap N` | raise the ban-safety daily message-send cap (default 20) |

## Output & agents

One renderer serves every command: `table` (default, colored only on a TTY; honors `NO_COLOR`),
`json`, `yaml`, `csv` (formula-injection-safe), and `-o id` (one id per line, `xargs`-friendly). A
global `--jq` slices any response.

- **MCP server:** `linkedin mcp` exposes the read commands as annotated MCP tools for an AI host.
  `messages send` carries the `destructiveHint` so a well-behaved host treats it as unsafe, never
  auto-approving it.
- **Agent guard:** `linkedin agent guard --host claude-code|codex|opencode` emits safety config from
  the live command tree (reads are allowed; `messages send` and `alias set` are hard-blocked, and
  any future write/destructive command is hard-blocked automatically).

## Schema drift

LinkedIn rotates Voyager's `decorationId` version suffixes, its collection variants, and its `$type`
names without notice. Every such string is isolated in **`internal/voyager/schema.go`** ("these
drift; bump here"), `$type` is matched by substring, and a search that returns zero recognizable job
entities surfaces a clear "schema moved — check schema.go" error instead of a silent empty result.

## Doctor

```sh
linkedin doctor          # local checks only (no LinkedIn request)
linkedin doctor --live   # + ONE low-risk, UNAUTHENTICATED geo-typeahead probe
```

## Development

`make verify` is the gate (fmt, vet, lint, gosec, govulncheck, tests, ≥80% coverage, spec-check,
spec-completeness, dod-check). Everything is tested against `httptest` fakes — **no test ever hits
LinkedIn.** See `AGENTS.md` and `DECISIONS.md`.

## License

MIT. Not affiliated with, endorsed by, or connected to LinkedIn Corporation.
