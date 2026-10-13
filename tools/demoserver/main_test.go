package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func request(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestDemoHandlerRecordedPaths(t *testing.T) {
	h := demoHandler()
	for _, path := range []string{
		"/voyager/api/voyagerJobsDashJobCards?start=0&count=25",
		"/voyager/api/jobs/jobPostings/900101",
		"/voyager/api/organization/companies?q=universalName&universalName=fictional-demo-workshop",
	} {
		rec := request(h, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body)
		}
		if rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("GET %s did not return JSON", path)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		switch {
		case path == "/voyager/api/jobs/jobPostings/900101":
			data := body["data"].(map[string]any)
			if data["title"] != "Go Platform Engineer" || data["workRemoteAllowed"] != true {
				t.Fatalf("unexpected job detail: %s", rec.Body)
			}
		case body["included"] != nil && body["data"] == nil:
			companies := body["included"].([]any)
			if len(companies) != 1 || companies[0].(map[string]any)["name"] != "Fictional Demo Workshop" {
				t.Fatalf("unexpected company: %s", rec.Body)
			}
		default:
			cards := body["included"].([]any)
			data := body["data"].(map[string]any)["data"].(map[string]any)
			paging := data["paging"].(map[string]any)
			refs := data["*elements"].([]any)
			if len(cards) != 2 || len(refs) != 2 || paging["total"] != float64(2) {
				t.Fatalf("unexpected search: %s", rec.Body)
			}
			for i, card := range cards {
				c := card.(map[string]any)
				if c["entityUrn"] != refs[i] || c["title"] == "" || c["primaryDescription"] == "" {
					t.Fatalf("unresolvable or empty job card: %s", rec.Body)
				}
			}
		}
	}
}

// The checked-out CLI is read-only: a fake write endpoint would claim a capability it lacks.
func TestDemoHandlerUnknownPathsAndWrites(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/unknown"},
		{http.MethodGet, "/voyager/api/jobs/jobPostings/900999"},
		{http.MethodGet, "/voyager/api/organization/companies?universalName=unknown"},
		{http.MethodPost, "/voyager/api/voyagerJobsDashJobCards"},
		{http.MethodPost, "/voyager/api/jobs/jobPostings/900101"},
		{http.MethodPost, "/voyager/api/organization/companies?universalName=fictional-demo-workshop"},
	} {
		rec := request(demoHandler(), route.method, route.path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", route.method, route.path, rec.Code)
		}
	}
}

func TestDemoServerRejectsInvalidAddress(t *testing.T) {
	var log bytes.Buffer
	// Invalid syntax fails before opening a socket, even in the recording sandbox.
	err := run([]string{"invalid::address"}, &log)
	if err == nil || !strings.Contains(err.Error(), "too many colons") {
		t.Fatalf("invalid listen address returned %v", err)
	}
	if !strings.Contains(log.String(), "demo API listening on invalid::address") {
		t.Fatalf("missing startup diagnostic: %s", &log)
	}
}
