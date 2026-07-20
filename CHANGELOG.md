# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial read-only CLI for LinkedIn job search via the unofficial Voyager API.
- `jobs search` — keywords, `--location` (geoId-resolved), `--remote`, `--since`, `--job-type`,
  `--experience`, pagination (`--count`/`--limit`/`--all`). Builds the Rest.li `query=(...)` blob by
  hand (structural chars literal).
- `jobs get <id>` — full, trustworthy job detail (workRemoteAllowed, listedAt, applyMethod,
  description, company URN); charges the ban-safety daily cap.
- `company get <slug>` — organization lookup by universalName.
- `geo <name>` — resolve a place name to a LinkedIn geoId (cached), plus inline `--location`
  resolution.
- **Borrow-the-browser-session** cookie auth (`internal/browserauth`, kooky-backed): `li_at` +
  `JSESSIONID`, csrf derived by trimming JSESSIONID's quotes; keyring storage with AES-256-GCM
  encrypted-file fallback; `LI_AT`/`JSESSIONID` env overrides.
- **Ban-safety defaults (on by default):** jittered 3–15s inter-request delay, one request in flight,
  a persisted ~30/day job-detail cap, and no retry on HTTP 999 / 429 / challenge.
- **Schema-drift firewall** (`internal/voyager/schema.go`) with substring `$type` matching and a
  loud `ErrSchemaMoved` signal.
- Standard fleet meta: `auth`, `config`, `doctor`, `completion`, `alias`, `api` (GET-only escape
  hatch), `version`, `update`, `mcp`, `agent guard`; output `-o table|json|yaml|csv|id`, `--jq`,
  `--dry-run`.

[Unreleased]: https://github.com/jjuanrivvera/linkedin-cli/commits/develop
