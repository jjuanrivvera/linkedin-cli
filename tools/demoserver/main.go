// Command demoserver supplies invented API responses for an account-free VHS recording.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	addr := "127.0.0.1:8654"
	if len(args) > 0 {
		addr = args[0]
	}
	server := &http.Server{
		Addr: addr, Handler: demoHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	fmt.Fprintln(stderr, "demo API listening on", addr)
	return server.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func demoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /voyager/api/voyagerJobsDashJobCards":
			writeJSON(w, map[string]any{
				"data": map[string]any{"data": map[string]any{
					"paging":    map[string]int{"total": 2, "start": 0, "count": 2},
					"*elements": []string{"urn:li:fsd_jobPostingCard:(900101,JOBS_SEARCH)", "urn:li:fsd_jobPostingCard:(900102,JOBS_SEARCH)"},
				}},
				"included": []any{
					map[string]any{
						"$type":         "com.linkedin.voyager.dash.jobs.JobPostingCard",
						"entityUrn":     "urn:li:fsd_jobPostingCard:(900101,JOBS_SEARCH)",
						"jobPostingUrn": "urn:li:fsd_jobPosting:900101",
						"title":         "Go Platform Engineer", "primaryDescription": "Fictional Demo Workshop",
						"secondaryDescription": "Remote",
					},
					map[string]any{
						"$type":         "com.linkedin.voyager.dash.jobs.JobPostingCard",
						"entityUrn":     "urn:li:fsd_jobPostingCard:(900102,JOBS_SEARCH)",
						"jobPostingUrn": "urn:li:fsd_jobPosting:900102",
						"title":         "Go Backend Developer", "primaryDescription": "Invented Sample Studio",
						"secondaryDescription": "Remote",
					},
				},
			})
		case "GET /voyager/api/jobs/jobPostings/900101":
			writeJSON(w, map[string]any{"data": map[string]any{
				"$type": "com.linkedin.voyager.jobs.JobPosting", "entityUrn": "urn:li:fsd_jobPosting:900101",
				"title": "Go Platform Engineer", "workRemoteAllowed": true,
				"description":    map[string]string{"text": "Build Go services and thoughtful CLI tools at a fictional workshop."},
				"companyDetails": map[string]string{"company": "urn:li:fsd_company:900201"},
			}})
		case "GET /voyager/api/organization/companies":
			if r.URL.Query().Get("universalName") != "fictional-demo-workshop" {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, map[string]any{"included": []any{map[string]any{
				"$type": "com.linkedin.voyager.organization.Company", "entityUrn": "urn:li:fsd_company:900201",
				"name": "Fictional Demo Workshop", "universalName": "fictional-demo-workshop",
				"description": "An invented employer for this local recording.", "staffCount": 24,
				"companyPageUrl": "https://workshop.example.invalid",
			}}})
		default:
			http.NotFound(w, r)
		}
	})
}
