# AGENTS.md — working in the linkedin-cli repo

`linkedin` is a read-only, agent-friendly CLI for **LinkedIn job search via the unofficial Voyager
API**. It borrows a browser session (li_at + JSESSIONID cookies) and paces requests for ban-safety.
Built to the cliwright standard (Go + Cobra + GoReleaser). This file orients an AI agent (or human)
contributing.

## The one rule that matters
**`make verify` is the gate.** A change is done only when `make verify` exits `0`: `make check`
(fmt, vet, golangci-lint, gosec, govulncheck, tests) + `spec-check` (built surface ⊆
`api-manifest.json`) + `spec-completeness` (manifest wraps ≥90% of the 4-endpoint enumerated
read-only surface — currently 100%) + `cover-check` (≥80%) + `dod-check.sh`. Run the full
`make verify` for any change that touches the surface or a documented behavior.

## NEVER hit LinkedIn
Everything is tested against `httptest` fakes. **No test, and no build step, may make a live
LinkedIn request.** A live run happens only when the operator explicitly supplies their own cookies
and opts in — it is never part of the gate.

## Architecture (where things live)
- `internal/voyager/` — **the schema-drift firewall.** `schema.go` holds EVERY string LinkedIn
  rotates (decorationId suffixes, `$type` match tokens, Rest.li filter keys, accept/x-restli header
  values) behind named constants with a "THESE DRIFT — bump here" banner. `normalize.go` parses the
  normalized envelope (`{data:{data:{paging,*elements}},included:[…]}`), matches `$type` by CONTAINS,
  resolves `*elements` against `included[]`, and returns `ErrSchemaMoved` when a positive-total page
  has zero recognizable job entities.
- `internal/api/` — the Voyager client core. Voyager headers + borrowed-session cookies (Cookie keeps
  JSESSIONID's quotes; `csrf-token` drops them), dry-run curl (cookies redacted), **ban-aware retry**
  (`retry.go`: 429/999/challenge are terminal, only 5xx + transient network retry, GET-only),
  `APIError` with ban-aware hints, flexible JSON types, the **`Pacer`** (jittered delay + per-run cap
  + persisted daily job-detail cap), and the typed service methods (`SearchJobs`, `SearchJobsAll`,
  `GetJob`, `GetCompany`, `ResolveGeo` + `GeoCache`). **Pattern B (service-layer)** — read-only,
  non-CRUD (DECISIONS.md #19).
- `internal/browserauth/` — the reusable **borrow-the-browser-session** primitive:
  `Extractor{Domain, Browsers, RequiredCookieNames}.Extract(ctx) (map, source, error)` behind a
  `Finder` seam (kooky in prod, a fake in tests). A future cookie-CLI imports it verbatim.
- `internal/auth/` — the cookie PAIR stored as a JSON blob in the OS keyring (service `linkedin-cli`,
  key `<profile>`), AES-256-GCM encrypted-file fallback (`LINKEDIN_KEYRING_PASSWORD`).
- `commands/` — the cobra tree. `init()` appends builders to `registrars`/`metaRegistrars`;
  `NewRootCmd()` drains the queue onto a fresh root. MCP annotations are stamped via `annotate(cmd,
  kind)` as commands are built (everything is `kindRead`). `getAPIClient()` wires cookies (env
  LI_AT/JSESSIONID → keyring) + the pacer.
- `internal/{config,output,version,update}` — profiles + manual precedence (no Viper), the
  table/json/yaml/csv/id renderer (CSV formula-injection guard, terminal-escape sanitizer, NO_COLOR),
  build metadata, the checksum-verified self-updater.
- `cmd/linkedin/main.go` — `signal.NotifyContext` (Ctrl-C cancels pagination + the pacing delay +
  retry backoff) + alias expansion before cobra parses.

## LinkedIn specifics you must NOT re-derive (see DECISIONS.md; enumerated from the build brief)
- Job search: `GET /voyager/api/voyagerJobsDashJobCards?decorationId=…&q=jobSearch&start=&count=25&query=(…)`.
  The `query=(...)` is a **Rest.li blob** — build it BY HAND keeping `(),:` and `List(...)` literal,
  encoding only the keyword value (`buildQueryBlob` in `internal/api/jobs.go`). `url.Values` would
  break it.
- Job detail: `GET /voyager/api/jobs/jobPostings/{id}?decorationId=…` — the trustworthy fields.
- Company: `GET /voyager/api/organization/companies?q=universalName&universalName=<slug>`.
- Geo: `GET https://www.linkedin.com/jobs-guest/api/typeaheadHits?typeaheadType=GEO&geoTypes=POPULATED_PLACE&query=<name>`
  (UNAUTHENTICATED). "remote" is not a geo — it's `workplaceType:List(2)`.
- Headers: `x-restli-protocol-version: 2.0.0`, `x-li-lang: en_US`,
  `accept: application/vnd.linkedin.normalized+json+2.1`, current-Chrome UA (single constant
  `api.DefaultUserAgent`, override `LINKEDIN_USER_AGENT`).

## Ban-safety is a feature, not a knob
Don't remove or weaken the pacer, the daily cap, or the no-retry-on-throttle rule to "make tests
faster" or "get more results". Tests inject a zero-delay pacer; production keeps 3–15s. On 999/429/
challenge the CLI surfaces and stops.

## Adding a command
1. Add the `internal/api` method + the `commands/*.go` builder (`annotate(cmd, kindRead)`), and tests
   against `httptest`, in the same commit (coverage is a ratchet).
2. Regenerate docs (`make docs-gen`) and update `api-manifest.json` + `DECISIONS.md`.
3. Any drift-prone string goes in `internal/voyager/schema.go`, never inline.
4. Exclude meta/destructive commands from MCP by EXACT path; keep the agent-guard classification
   fail-closed (unannotated ⇒ destructive).
