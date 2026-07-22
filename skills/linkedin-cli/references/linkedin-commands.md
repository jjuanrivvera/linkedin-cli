# LinkedIn commands deep-dive

## jobs search
Wraps `GET /voyager/api/voyagerJobsDashJobCards?decorationId=…&q=jobSearch&start=&count=25&query=(…)`.
The `query=(...)` Rest.li blob keeps `(),:` and `List(...)` literal; only the keyword value is
percent-encoded. Filters: `--remote`→`workplaceType:List(2)`, `--since`→`timePostedRange:List(r<secs>)`,
`--location`→resolved geoId→`locationUnion:(geoId:…)`, `--job-type`→`jobType:List(…)`,
`--experience`→`experience:List(…)`. Returns thin cards (id/title/company/location).

## jobs get <id>
Wraps `GET /voyager/api/jobs/jobPostings/<id>?decorationId=…`. The trustworthy fields:
`workRemoteAllowed`/`workplaceTypes`, `listedAt` (posted epoch-ms), `applyMethod`+`companyApplyUrl`,
`description.text`, company URN. Charged against the daily ban-safety cap.

## company get <slug>
Wraps `GET /voyager/api/organization/companies?q=universalName&universalName=<slug>&decorationId=…`.

## geo <name>
Wraps the unauthenticated `GET /jobs-guest/api/typeaheadHits?typeaheadType=GEO&geoTypes=POPULATED_PLACE&query=<name>`
→ `[{id, displayName}]`. Cached name→geoId in the config dir. "remote" is not a geo.

## messages list
Wraps `GET /voyager/api/messaging/conversations?keyVersion=LEGACY_INBOX` (the community-proven
legacy inbox surface). Lists conversations most-recent-first: id, participant name(s),
last-activity time, snippet. `--count` (default 20). Full entities under `-o json`. Read-only.

## messages read <conversationId>
Wraps `GET /voyager/api/messaging/conversations/{conversationId}/events`. Prints the thread
(sender, time, text) oldest→newest. Read-only.

## messages send <conversationId> --text "..."
Wraps `POST /voyager/api/messaging/conversations/{conversationId}/events?action=create` with the
legacy `MessageCreate` body. THE ONE WRITE — and the riskiest command: automated messaging is the
classic LinkedIn account-restriction trigger. It prints a stderr warning and requires interactive
confirmation (skip with `--yes`), charges a persisted daily send cap (default 20/day,
`--daily-send-cap`), and is never retried. Honors `--dry-run` (prints the equivalent curl with
cookies redacted, sends nothing). Classified **destructive** — an agent must never auto-approve it.
