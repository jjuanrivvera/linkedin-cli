// Package voyager isolates every LinkedIn-Voyager schema detail that DRIFTS without notice:
// the decorationId version suffixes, the Rest.li filter/query keys, and the normalized-JSON
// $type names. LinkedIn rotates these silently (a decorationId's trailing "-88"/"-174"/"-192"
// bumps, a `JobSearchCardsCollection`→`…Lite` variant swap, a `JobPosting`→`JobPostingCard`
// rename). Keeping them ALL in this one file — behind named constants — means a schema break
// is a one-line edit here, not a hunt across the codebase.
//
// ┌─────────────────────────────────────────────────────────────────────────────────────┐
// │  THESE STRINGS DRIFT. When a search suddenly returns zero job entities, or a detail    │
// │  fetch decodes to nothing, LinkedIn almost certainly bumped one of these. Bump it HERE. │
// │  Match $type by CONTAINS/suffix (see typeContains), never by exact equality.            │
// └─────────────────────────────────────────────────────────────────────────────────────┘
package voyager

// decorationId values — the server-side projection selectors. The trailing integer is a
// version that LinkedIn increments on its own schedule; when a request starts returning a
// thin/empty envelope, try bumping the suffix first.
const (
	// DecorationJobSearch selects the job-search card projection for voyagerJobsDashJobCards.
	DecorationJobSearch = "com.linkedin.voyager.dash.deco.jobs.search.JobSearchCardsCollection-207"
	// DecorationJobDetail selects the full job-posting projection for jobs/jobPostings/{id}.
	DecorationJobDetail = "com.linkedin.voyager.deco.jobs.web.shared.WebFullJobPosting-65"
	// DecorationCompany selects the organization projection for organization/companies.
	// WebCompanyMainRelated-16 started 400ing (live smoke 2026-07-21); WebFullCompanyMain-12 is
	// the projection the community clients ship (transitive-bullshit/linkedin-api, nsandman).
	DecorationCompany = "com.linkedin.voyager.deco.organization.web.WebFullCompanyMain-12"
)

// Endpoint paths (relative to the Voyager base https://www.linkedin.com/voyager/api).
const (
	PathJobSearch = "/voyagerJobsDashJobCards"
	PathJobDetail = "/jobs/jobPostings/" // + {id}
	PathCompanies = "/organization/companies"
)

// TypeaheadPath is the unauthenticated jobs-guest geo typeahead (relative to
// https://www.linkedin.com).
const TypeaheadPath = "/jobs-guest/api/typeaheadHits"

// ┌─────────────────────────────────────────────────────────────────────────────────────┐
// │  MESSENGER GRAPHQL — queryId hashes rotate on LinkedIn's frontend build.               │
// │  When `messages list`/`read` starts 500ing or returning empties, REFRESH these two     │
// │  queryId hashes (and, if the URLs moved, the paths) from a maintained client            │
// │  (mautrix/linkedin pkg/linkedingo/constants.go) or the browser Network tab. They drift  │
// │  HARDER and more often than the decorationIds above — this is the #1 messaging break.   │
// └─────────────────────────────────────────────────────────────────────────────────────┘
//
// The web app abandoned the legacy /messaging/conversations + /events inbox (it now 500s,
// live smoke 2026-07-22) for a GraphQL "messenger" surface. list/read are GraphQL GETs that
// require the caller's mailbox URN (resolved from /me — see PathMe); send is a Dash POST.
// DECISIONS.md #27–#29.
const (
	// PathMe resolves the caller's own profile (for the mailbox URN the GraphQL calls need).
	// GET /me → {"miniProfile":{"entityUrn":"urn:li:fs_miniProfile:<ID>","dashEntityUrn":"urn:li:fsd_profile:<ID>"}}.
	PathMe = "/me"

	// PathMessengerGraphQL is the GraphQL GET surface for conversations + thread reads.
	// Called with ?queryId=<hash>&variables=(...) and the accept: application/graphql header.
	PathMessengerGraphQL = "/voyagerMessagingGraphQL/graphql"

	// PathMessengerDashSend is the Dash send endpoint (POST ?action=createMessage,
	// Content-Type text/plain; charset=UTF-8).
	PathMessengerDashSend = "/voyagerMessagingDashMessengerMessages"

	// ListQueryID selects the conversations-list GraphQL query. ROTATES on LinkedIn's build.
	// Captured live from Juan's browser 2026-07-22 (the prior …f0873b9… hash had gone dead).
	ListQueryID = "messengerConversations.0d5e6781bbee71c3e51c8843c6519f48"
	// MessagesQueryID selects the thread-read GraphQL query. ROTATES on LinkedIn's build.
	MessagesQueryID = "messengerMessages.4088d03bc70c91c3fa68965cb42336de"

	// AcceptGraphQL is the accept header the GraphQL messenger GETs require (NOT the
	// normalized+json accept the rest of the client sends).
	AcceptGraphQL = "application/graphql"

	// ActionCreateMessage is the ?action= token on the Dash send POST.
	ActionCreateMessage = "createMessage"
	// SendContentType is the Content-Type the web client sends on the createMessage POST —
	// plain text, not JSON (that is genuinely what LinkedIn expects).
	SendContentType = "text/plain; charset=UTF-8"

	// GraphQL variables=(...) key names for the messenger queries.
	VarMailboxURN      = "mailboxUrn"
	VarConversationURN = "conversationUrn"
	VarCountBefore     = "countBefore"
	VarCountAfter      = "countAfter"
	VarDeliveredAt     = "deliveredAt"

	// Response-envelope container keys — the GraphQL result field wrapping each elements[]
	// list. These drift with the queryId, so they live here too.
	KeyConversationsResult = "messengerConversationsBySyncToken"
	KeyMessagesResult      = "messengerMessagesByAnchorTimestamp"

	// ConversationURNPrefix is the msg_conversation URN namespace the read/send calls accept
	// (list emits full URNs; a bare id is prefixed defensively).
	ConversationURNPrefix = "urn:li:msg_conversation:"
	// MiniProfileURNPrefix / ProfileURNPrefix bridge the /me miniProfile.entityUrn to the
	// fsd_profile mailbox URN when dashEntityUrn is absent.
	MiniProfileURNPrefix = "urn:li:fs_miniProfile:"
	ProfileURNPrefix     = "urn:li:fsd_profile:"
)

// Rest.li query keys for the job-search `query=(...)` blob. These are structural names LinkedIn
// uses inside the parenthesised query; they drift less often than decorationIds but still live
// here so a rename is one place.
const (
	QueryOrigin        = "origin"
	QueryKeywords      = "keywords"
	QueryLocationUnion = "locationUnion"
	QueryGeoID         = "geoId"
	QuerySelectedTypes = "selectedFilters"
	QuerySpellCorrect  = "spellCorrectionEnabled"

	FilterWorkplaceType   = "workplaceType"
	FilterTimePostedRange = "timePostedRange"
	FilterJobType         = "jobType"
	FilterExperience      = "experience"

	// OriginJobSearch is the origin token sent with an interactive job search.
	OriginJobSearch = "JOB_SEARCH_PAGE_QUERY_EXPANSION"
	// WorkplaceRemote is the workplaceType code for "Remote" (1=on-site, 2=remote, 3=hybrid).
	WorkplaceRemote = "2"
)

// Accept / protocol header VALUES. The normalized accept header carries a version suffix
// (`+2.1`) that has changed historically, so it lives here too.
const (
	AcceptNormalized     = "application/vnd.linkedin.normalized+json+2.1"
	RestliProtoVersion   = "2.0.0"
	LiLang               = "en_US"
	TypeaheadGeoType     = "GEO"
	TypeaheadPlacesParam = "POPULATED_PLACE"
)

// $type tokens matched by CONTAINS (never exact) against the normalized `included[]` entities.
// LinkedIn renamed the job entity `JobPosting`→`JobPostingCard` once already, so we match on a
// stable substring and tolerate the version drift around it.
const (
	// TypeJobCard is the substring identifying a job-search result card entity. Matched with
	// typeContains, so both `…JobPostingCard` and a future `…JobPostingCardLite` still resolve.
	TypeJobCard = "JobPostingCard"
	// TypeJobPosting identifies a full job-detail entity from jobs/jobPostings/{id}.
	TypeJobPosting = "JobPosting"
	// TypeCompany identifies an organization/company entity.
	TypeCompany = "Company"
)
