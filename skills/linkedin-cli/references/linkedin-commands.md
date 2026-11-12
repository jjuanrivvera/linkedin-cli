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
Wraps the GraphQL messenger surface `GET /voyager/api/voyagerMessagingGraphQL/graphql?queryId=messengerConversations.<hash>&variables=(mailboxUrn:…)`
(the caller's mailbox URN is resolved once from `GET /voyager/api/me`). Lists conversations
most-recent-first: id, participant name(s), last-activity time, snippet. Conversation ids are full
`urn:li:msg_conversation:…` URNs — exactly what `read`/`send` accept. `--count` (default 20). Full
elements under `-o json`. Read-only. NOTE: the queryId hash rotates on LinkedIn's frontend build —
if list 500s/empties, refresh it in `internal/voyager/schema.go`.

## messages read <conversationId>
Wraps `GET /voyager/api/voyagerMessagingGraphQL/graphql?queryId=messengerMessages.<hash>&variables=(conversationUrn:…,countBefore:N,countAfter:0,deliveredAt:<nowMs>)`.
Prints the thread (sender, time, text) oldest→newest. Pass the `urn:li:msg_conversation:…` id from
`messages list` (a bare id is prefixed automatically). Read-only.

## messages send <conversationId> --text "..."
Wraps `POST /voyager/api/voyagerMessagingDashMessengerMessages?action=createMessage` (Content-Type
`text/plain`) with the Dash `createMessage` body. THE ONE WRITE — and the riskiest command:
automated messaging is the classic LinkedIn account-restriction trigger. It prints a stderr warning
and requires interactive confirmation (skip with `--yes`), charges a persisted daily send cap
(default 20/day, `--daily-send-cap`), and is never retried. Honors `--dry-run` (prints the
equivalent curl with cookies redacted, sends nothing). Classified **destructive** — an agent must
never auto-approve it.
