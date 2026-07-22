# DECISIONS — pinned LinkedIn Voyager API assumptions

One line each: question → decision → why. Read back every iteration; never silently re-decide.

LinkedIn publishes **no** official OpenAPI/llms.txt/Postman collection for Voyager — it is an
internal, unofficial API. Endpoints/headers below come from the cliwright build brief plus
community reverse-engineering of the linkedin.com web client. **No live LinkedIn request was made
during this build; everything is exercised against `httptest` fakes.**

## Endpoints

1. **Job search** → `GET /voyager/api/voyagerJobsDashJobCards?decorationId=…&q=jobSearch&start=&count=25&query=(…)`.
   The `query=(...)` is a Rest.li blob using bare `(),:` and `List(...)`. Why: it is the endpoint
   the web client's job search calls.
2. **Job detail** → `GET /voyager/api/jobs/jobPostings/{id}?decorationId=…`. This is where the
   TRUSTWORTHY fields live (workRemoteAllowed/workplaceTypes, listedAt, applyMethod/companyApplyUrl,
   description.text, company URN).
3. **Company** → `GET /voyager/api/organization/companies?q=universalName&universalName=<slug>&decorationId=…`.
4. **Geo typeahead** → `GET https://www.linkedin.com/jobs-guest/api/typeaheadHits?typeaheadType=GEO&geoTypes=POPULATED_PLACE&query=<name>` → `[{id, displayName}]`. UNAUTHENTICATED (jobs-guest),
   so it is the low-risk probe used by `doctor --live`. "remote" is NOT a geo — it is
   `workplaceType:List(2)`.

## Messaging (the one write surface — added in feat/messages)

23. **Use the community-proven LEGACY Voyager messaging endpoints, NOT the newer
    voyagerMessagingDash GraphQL surface** → list is
    `GET /voyager/api/messaging/conversations?keyVersion=LEGACY_INBOX`, a thread is
    `GET /voyager/api/messaging/conversations/{conversationId}/events`, and a send is
    `POST /voyager/api/messaging/conversations/{conversationId}/events?action=create`. Why:
    these are the exact endpoints the tomquirk/linkedin-api Python library has driven for years
    — well-understood, stable, and normalized-envelope shaped like the rest of this client. The
    Dash/GraphQL messaging surface is newer, less documented in the community, and would need a
    different query mechanism. Every drift-prone string (paths, `keyVersion` value, the
    `action=create` token, the `$type` match tokens, the MessageCreate body key) lives behind a
    named constant in `internal/voyager/schema.go`; parsing (`ParseConversations`/`ParseEvents`)
    matches `$type` by CONTAINS and tolerates both bare-string and `{text}` attributedBody
    shapes via the existing `textOrStr`, exactly like `normalize.go`.
24. **Send body is the legacy MessageCreate envelope** →
    `{"eventCreate":{"value":{"com.linkedin.voyager.messaging.create.MessageCreate":{"attributedBody":{"text":"<text>","attributes":[]},"attachments":[]}}}}`,
    POSTed as `application/json`, expecting **201 Created**. Why: this is the payload the web
    client and the community libraries send; the `MessageCreate` type key is the drift-prone part
    and is pinned in `schema.go` (`KeyMessageCreate`).
25. **`messages send` charges a NEW persisted daily send cap (default 20/day,
    `--daily-send-cap`)** → the same persistence mechanism the job-detail cap uses
    (`internal/api/pacer.go`), generalized to named counters in one `state.json` (the legacy
    top-level `count` field stays the job-detail counter for backward compatibility; the send
    counter lives under `counters.message_send`). It is charged BEFORE the request and refuses
    (does not send) when spent. Why: automated messaging is the classic LinkedIn
    account-restriction trigger, so the riskiest command gets its own tighter budget.
26. **No retry on the send POST** → the client core gained a `postVoyager` mirroring `getVoyager`'s
    header/cookie/csrf handling (Cookie keeps JSESSIONID's quotes; `csrf-token` strips them), but
    `retry.go` stays idempotent-only: a POST is passed with a zero retry budget and fired at most
    ONCE. Why: a duplicate DM is worse than a failed one, and retry-hammering a write is exactly
    what flags an account. Additionally `messages send` prints a stderr warning and requires
    interactive confirmation (skippable with `--yes`), honors global `--dry-run` (prints the
    equivalent curl with cookies REDACTED, sends nothing, needs no session), and is classified
    **destructive** so every agent guard (MCP + Bash) hard-blocks it, fail-closed.

## Auth — borrow the browser session (cookie auth)

5. **Cookie auth, not a token** → extract `li_at` (session) + `JSESSIONID` (value looks like
   `"ajax:123..."` WITH quotes) for `.linkedin.com` from the user's browser via
   `github.com/browserutils/kooky`. Store the pair as a JSON blob in the OS keyring
   (AES-256-GCM encrypted-file fallback, `LINKEDIN_KEYRING_PASSWORD`). Why: Voyager has no public
   token scheme; the same cookies the web client sends are the only auth.
6. **CSRF derivation** → `csrf-token = strings.Trim(JSESSIONID, "\"")` (drop the quotes; the Cookie
   header keeps them). Verified in the dry-run curl output.
7. **Request headers (every Voyager GET)** → `Cookie: li_at=…; JSESSIONID="ajax:…"`, `csrf-token:
   ajax:…`, `x-restli-protocol-version: 2.0.0`, `x-li-lang: en_US`,
   `accept: application/vnd.linkedin.normalized+json+2.1`, and a CURRENT desktop Chrome
   User-Agent (a single overridable constant `api.DefaultUserAgent`, overridable via
   `LINKEDIN_USER_AGENT`). Why: a bare Go UA — or a STALE UA — is itself an automated fingerprint.
8. **Plain net/http, no TLS-fingerprint spoofing** → read Voyager does not require it (the §1
   standard; adding uTLS is complexity we don't need for reads).
9. **Env overrides `LI_AT` / `JSESSIONID`** → for headless use; both required together (JSESSIONID
   is needed to derive csrf). They take precedence over the keyring.

## Rest.li query construction (the #1 correctness trap)

10. **Build the `query=(...)` blob BY HAND with a `safe` set** → structural chars `(`, `)`, `,`,
    `:` and `List(...)` stay LITERAL; only free-text VALUES (keywords) are percent-encoded
    (`url.QueryEscape` then `+`→`%20`). Why: `url.Values.Encode()` would percent-encode the
    structural chars and LinkedIn 400s the request. Verified in the dry-run:
    `query=(origin:…,keywords:product%20design,selectedFilters:(workplaceType:List(2),timePostedRange:List(r604800)))`.
11. **Filter mappings** → `--remote`→`workplaceType:List(2)`; `--since`→`timePostedRange:List(r<secs>)`
    (r86400=24h, r604800=7d, r2592000=30d); `--location <name>`→resolve to geoId→`locationUnion:(geoId:<id>)`;
    `--job-type`→`jobType:List(F,C,…)`; `--experience`→`experience:List(2,3,…)`.

## SCHEMA DRIFT — the #1 maintenance risk (designed for)

12. **Every drift-prone string is isolated in `internal/voyager/schema.go`** → decorationId version
    suffixes (`JobSearchCardsCollection-207`, `WebFullJobPosting-65`, `WebFullCompanyMain-12` — the
    brief's `WebCompanyMainRelated-16` 400ed in the 2026-07-21 live smoke; `WebFullCompanyMain-12`
    is what community clients ship and it works),
    the `$type` match tokens, the Rest.li filter keys, and the accept/x-restli header values. Comment:
    "THESE STRINGS DRIFT — bump here." Why: LinkedIn rotates decorationId suffixes, swaps
    `JobSearchCardsCollection`↔`…Lite`, and renames `$type`s (`JobPosting`→`JobPostingCard`) with no
    notice.
13. **Match `$type` by CONTAINS/suffix, never exact** (`voyager.typeContains`) → survives the
    `JobPosting`→`JobPostingCard`→`…Lite` renames.
14. **A search with a positive total but ZERO recognizable job entities returns `ErrSchemaMoved`**,
    surfaced with a pointer at `internal/voyager/schema.go` — NOT silently "no results". An honest
    empty search (total 0, no entities) is fine. Why: silent zero-results would hide a schema
    rotation for weeks.
14b. **Search cards dedupe by job id, preferring the hydrated variant** → LinkedIn ships the same
    job as a titled `(id,JOBS_SEARCH)` card AND a thin `(id,JOB_DETAILS)` prefetch stub (observed
    live 2026-07-21: 3 of 5 results came back as bare stubs). `ParseSearch` harvests the best card
    per id from the pool, swaps a stub for its titled twin, reads `title` as well as
    `jobPostingTitle`, and follows `*jobPosting`/`jobPostingUrn` into `included[]` as a last-resort
    title source. Why: emitting stubs as results loses title/company/location the response carries.

## Ban-safety defaults (ON by default, not options)

15. **Jittered human-paced delay between requests** → 3–15s randomized, ONE request in flight, NO
    parallelism (`internal/api/pacer.go`). The first request of a run is free; the delay applies
    between successive requests. Why: the biggest risk is not a bug, it is an account restriction
    from looking automated.
16. **Per-day job-detail cap ~30**, persisted in the config/state dir (`state.json`), charged by
    `jobs get` BEFORE the request; refuses (does not fetch) when spent. Overridable with
    `--daily-cap`. A per-run cap is also supported (default off). Why: bounds daily footprint on the
    riskiest endpoint (authenticated detail fetches).
17. **On HTTP 999 (soft-block), 429, or a challenge → back off HARD, surface an actionable message,
    NEVER retry** → `retry.go` treats 999/429 as terminal (even though 999≥500). A challenge
    (checkpoint) response tells the user to re-verify in a browser. Only genuine 5xx + transient
    network errors retry, on GET only. Why: retry-hammering a throttle signal is exactly what flags
    an account.
18. **README + SKILL.md carry a prominent ToS/ban-risk disclaimer** (unofficial API, use your own
    account, low volume, your own machine/residential IP), mirroring how slackctl documents its
    xoxc/unofficial caveat.

## Architecture

19. **Pattern B (service-layer), not generic-core** → the endpoints are read-only and non-CRUD
    (GET job-cards search with a Rest.li query DSL, GET-by-id detail, GET company by universalName,
    GET geo typeahead), the documented §11 trigger for Pattern B. Typed service methods
    (`SearchJobs`, `SearchJobsAll`, `GetJob`, `GetCompany`, `ResolveGeo`) render raw JSON through
    the shared formatter.
20. **`internal/browserauth` is a self-contained, reusable primitive** → `Extractor{Domain,
    Browsers, RequiredCookieNames}` with `Extract(ctx) (map, source, error)` behind a `Finder`
    seam (kooky in prod, a fake in tests). A future cookie-CLI imports it and changes only
    Domain/RequiredCookieNames.
21. **`api` escape hatch is GET-ONLY** → the raw `linkedin api <PATH>` only issues GETs (the one
    write in the CLI is the dedicated `messages send`, never this escape hatch), so the agent guard
    classifies `api` as a read (no METHOD gating) and allows it.
22. **Two hosts, one client** → the Voyager base (authenticated) and the web base (unauthenticated
    typeahead); `NewClientWithBaseURL` points both at one URL for tests.

## Conditional patterns (§3d) — decisions

- Event-store / offline-cache: **N/A** — LinkedIn is a stateless read API; no ephemeral stream, no
  need for a local system-of-record. (A tiny name→geoId cache exists purely to cut requests.)
- Spec-contract test / smoke.yml / spec-sync.yml: **N/A** — no machine spec exists at a stable URL,
  and a live smoke test against LinkedIn is exactly what ban-safety forbids. Drift is caught by the
  isolated schema.go + `ErrSchemaMoved`.
- Multi-group credentials: **N/A** — one cookie pair.
- Adopt a typed library: **N/A** — no mature Go Voyager client.
- Terminal-escape sanitization: **applied** (shared renderer) — job titles/company names are free text.
- CSV: **kept** — job search results are tabular; a job/company detail is not, so it renders best as
  json/yaml.
- Binary self-update + keyring encrypted-file fallback: **applied** (fleet standard).

## Completeness

- `api_method_total = 7` is the full enumerated surface this CLI targets (see
  `enumerated_endpoints`): the 4 read-only job-search/company/geo endpoints plus the 3
  community-proven legacy messaging endpoints (conversations list, thread events, send). The
  manifest covers all 7 (jobs search/get, company get, geo, messages list/read/send), so
  `make spec-completeness` reports 100%. No coverage-waiver needed. Other write endpoints
  (apply/save/connect) remain OUT OF SCOPE — this stays a ban-safety-first tool whose only write
  is the confirmation-gated, daily-capped `messages send`.
