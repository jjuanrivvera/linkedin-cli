package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// StatusSoftBlock is LinkedIn's non-standard "999" soft-block status. It is not a real HTTP
// status; LinkedIn returns it when it throttles or challenges an unofficial client. It must
// NEVER be retried — hammering a soft-block is exactly how an account gets flagged.
const StatusSoftBlock = 999

// APIError is a LinkedIn Voyager error with an actionable, ban-aware hint keyed by status.
type APIError struct {
	StatusCode int
	Message    string
	Body       []byte
	// Challenge is set when LinkedIn returned a checkpoint/challenge (a security-verification
	// interstitial), signalling the session needs human re-verification in a browser.
	Challenge bool
}

// voyagerErrorBody covers LinkedIn's error envelope: {status,code,message} and the plain
// {message} shape.
type voyagerErrorBody struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func parseAPIError(status int, body []byte, h http.Header) *APIError {
	e := &APIError{StatusCode: status, Body: body}
	var t voyagerErrorBody
	if json.Unmarshal(body, &t) == nil && t.Message != "" {
		e.Message = t.Message
	}
	if e.Message == "" {
		if status == StatusSoftBlock {
			e.Message = "soft-blocked by LinkedIn (HTTP 999)"
		} else {
			e.Message = http.StatusText(status)
		}
	}
	// A checkpoint/challenge is signalled by a redirect location or an x-li-* challenge header.
	if loc := h.Get("Location"); loc != "" && containsAny(loc, "checkpoint", "challenge") {
		e.Challenge = true
	}
	if h.Get("x-li-uuid") != "" && status == http.StatusForbidden {
		// forbidden with a li tracking header often accompanies a challenge
		e.Challenge = e.Challenge || containsAny(string(body), "CHALLENGE", "checkpoint")
	}
	return e
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("LinkedIn Voyager error %d: %s", e.StatusCode, e.Message)
	if hint := e.hint(); hint != "" {
		msg += "\nHint: " + hint
	}
	return msg
}

// hint maps a status to the remedy a user actually needs — with the ban-safety posture baked
// into the soft-block/rate-limit/challenge cases (slow down, never hammer).
func (e *APIError) hint() string {
	if e.Challenge {
		return "LinkedIn issued a security challenge — open linkedin.com in your browser, complete the " +
			"checkpoint, then re-run `linkedin auth --cookie-from-browser <browser>` to refresh the session. " +
			"Do NOT retry in a loop."
	}
	switch e.StatusCode {
	case StatusSoftBlock:
		return "LinkedIn soft-blocked this request (HTTP 999) — you are being rate-limited/flagged as " +
			"automated. STOP for a while (hours), reduce volume, and use your normal browser for a bit. " +
			"The CLI deliberately did NOT retry."
	case http.StatusUnauthorized:
		return "session expired or missing — refresh cookies with `linkedin auth --cookie-from-browser <browser>` " +
			"(log in to linkedin.com in that browser first)"
	case http.StatusForbidden:
		return "access denied — the li_at/JSESSIONID pair may be stale, or this resource is gated. " +
			"Refresh with `linkedin auth --cookie-from-browser <browser>`; do not retry-hammer."
	case http.StatusNotFound:
		return "not found — verify the job id (from `linkedin jobs search`) or the company slug (the " +
			"universalName in the company URL, e.g. linkedin.com/company/<slug>)"
	case http.StatusTooManyRequests:
		return "rate limited (HTTP 429) — back off HARD (wait, lower volume). The CLI did NOT auto-retry, " +
			"deliberately: retry-hammering LinkedIn risks an account restriction."
	case http.StatusBadRequest:
		return "LinkedIn rejected the request — a decorationId or query filter may have drifted; check " +
			"internal/voyager/schema.go"
	}
	if e.StatusCode >= 500 {
		return "LinkedIn server error — usually transient, but retry sparingly"
	}
	return ""
}

// containsAny reports whether s contains any of subs, case-insensitively.
func containsAny(s string, subs ...string) bool {
	ls := strings.ToLower(s)
	for _, sub := range subs {
		if sub != "" && strings.Contains(ls, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
