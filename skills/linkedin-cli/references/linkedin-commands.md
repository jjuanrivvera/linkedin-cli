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
