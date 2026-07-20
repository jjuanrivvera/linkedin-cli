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
	DecorationCompany = "com.linkedin.voyager.deco.organization.web.WebCompanyMainRelated-16"
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
