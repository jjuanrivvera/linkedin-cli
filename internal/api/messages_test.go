package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// meBody is the /me response that resolves the caller's mailbox URN.
const meBody = `{"miniProfile":{"entityUrn":"urn:li:fs_miniProfile:ACoAA1","dashEntityUrn":"urn:li:fsd_profile:ACoAA1"}}`

const conversationsBody = `{"data":{"messengerConversationsBySyncToken":{"elements":[
  {"entityUrn":"urn:li:msg_conversation:2-AAA==","lastActivityAt":1721500000000,
   "conversationParticipants":[{"participantType":{"member":{"firstName":{"text":"Alice"},"lastName":{"text":"Smith"}}}}],
   "messages":{"elements":[{"deliveredAt":1721500000000,"body":{"text":"hola"}}]}}]}}}`

const messagesBody = `{"data":{"messengerMessagesByAnchorTimestamp":{"elements":[
  {"entityUrn":"urn:li:msg_message:S1","deliveredAt":1721500000000,
   "sender":{"participantType":{"member":{"firstName":{"text":"Alice"},"lastName":{"text":"Smith"}}}},
   "body":{"text":"hola"}}]}}}`

// graphQLRouter serves /me plus the GraphQL messenger GETs (routing by queryId prefix).
func graphQLRouter(t *testing.T, capture func(r *http.Request)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			capture(r)
		}
		switch {
		case r.URL.Path == "/me" || strings.HasSuffix(r.URL.Path, "/me"):
			_, _ = w.Write([]byte(meBody))
		case strings.Contains(r.URL.Path, "voyagerMessagingGraphQL"):
			if strings.HasPrefix(r.URL.Query().Get("queryId"), "messengerConversations") {
				_, _ = w.Write([]byte(conversationsBody))
			} else {
				_, _ = w.Write([]byte(messagesBody))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestListConversations(t *testing.T) {
	var gotPath, gotQueryID, gotVars, gotAccept string
	c := newTestClient(t, graphQLRouter(t, func(r *http.Request) {
		if strings.Contains(r.URL.Path, "voyagerMessagingGraphQL") {
			gotPath = r.URL.Path
			gotQueryID = r.URL.Query().Get("queryId")
			gotVars = r.URL.Query().Get("variables")
			gotAccept = r.Header.Get("Accept")
		}
	}))
	res, err := c.ListConversations(t.Context(), 0)
	require.NoError(t, err)
	assert.Contains(t, gotPath, "/voyagerMessagingGraphQL/graphql")
	assert.Equal(t, "messengerConversations.0d5e6781bbee71c3e51c8843c6519f48", gotQueryID)
	assert.Contains(t, gotVars, "mailboxUrn:urn:li:fsd_profile:ACoAA1", "mailbox URN from /me, escaped in-blob")
	assert.Equal(t, "application/graphql", gotAccept, "GraphQL GET uses the graphql accept header")
	require.Len(t, res.Conversations, 1)
	assert.Equal(t, "urn:li:msg_conversation:2-AAA==", res.Conversations[0].ID)
	assert.Equal(t, "hola", res.Conversations[0].Snippet)
	assert.NotEmpty(t, res.Raw)
}

// TestGetMailboxURN_SurvivesRedirectHandshake reproduces the live blocker: /voyager/api/me answers
// the first hit with a 302 that Set-Cookies `lidc` and redirects back to /me. A client with a
// cookie jar persists lidc and the second hop returns 200; without one it re-sends the same cookies
// and loops until net/http's 10-redirect cap. This asserts the jar breaks the loop.
func TestGetMailboxURN_SurvivesRedirectHandshake(t *testing.T) {
	var meHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/me") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		meHits++
		if _, err := r.Cookie("lidc"); err != nil { // no routing cookie yet → bounce once, setting it
			http.SetCookie(w, &http.Cookie{Name: "lidc", Value: "b=routed", Path: "/"})
			http.Redirect(w, r, r.URL.Path, http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(meBody))
	}))
	t.Cleanup(srv.Close)

	hc := srv.Client()
	hc.Jar = newCookieJar() // production wires this in New(); srv.Client() does not
	c := NewClientWithBaseURL(srv.URL, WithHTTPClient(hc), WithMaxRetries(0), WithCookies(testCookies))

	urn, err := c.GetMailboxURN(t.Context())
	require.NoError(t, err, "the cookie jar must let the /me redirect handshake complete")
	assert.Equal(t, "urn:li:fsd_profile:ACoAA1", urn)
	assert.Equal(t, 2, meHits, "one 302 bounce, then a 200 — not a 10-redirect loop")
}

func TestGetConversationEvents(t *testing.T) {
	var gotQueryID, gotVars string
	c := newTestClient(t, graphQLRouter(t, func(r *http.Request) {
		if strings.Contains(r.URL.Path, "voyagerMessagingGraphQL") {
			gotQueryID = r.URL.Query().Get("queryId")
			gotVars = r.URL.Query().Get("variables")
		}
	}))
	now := time.UnixMilli(1721599999000)
	res, err := c.GetConversationEvents(t.Context(), "urn:li:msg_conversation:2-AAA==", now)
	require.NoError(t, err)
	assert.Equal(t, "messengerMessages.4088d03bc70c91c3fa68965cb42336de", gotQueryID)
	assert.Contains(t, gotVars, "conversationUrn:urn:li:msg_conversation:2-AAA==")
	assert.Contains(t, gotVars, "countAfter:0")
	assert.Contains(t, gotVars, "deliveredAt:1721599999000", "injected clock drives the anchor timestamp")
	require.Len(t, res.Messages, 1)
	assert.Equal(t, "Alice Smith", res.Messages[0].Sender)
	assert.Equal(t, "hola", res.Messages[0].Text)
}

// TestGetConversationEvents_BareIDPrefixed proves a bare conversation id is prefixed to the
// msg_conversation URN the GraphQL query needs.
func TestGetConversationEvents_BareIDPrefixed(t *testing.T) {
	var gotVars string
	c := newTestClient(t, graphQLRouter(t, func(r *http.Request) {
		if strings.Contains(r.URL.Path, "voyagerMessagingGraphQL") {
			gotVars = r.URL.Query().Get("variables")
		}
	}))
	_, err := c.GetConversationEvents(t.Context(), "2-AAA==", time.UnixMilli(1))
	require.NoError(t, err)
	assert.Contains(t, gotVars, "conversationUrn:urn:li:msg_conversation:2-AAA==")
}

func withFixedSendTokens(t *testing.T) {
	t.Helper()
	origO, origT := newOriginToken, newTrackingID
	newOriginToken = func() string { return "origin-fixed-uuid" }
	newTrackingID = func() string { return "track-fixed-1234" }
	t.Cleanup(func() { newOriginToken, newTrackingID = origO, origT })
}

func TestSendMessage_PostShape(t *testing.T) {
	withFixedSendTokens(t)
	var gotMethod, gotPath, gotAction, gotCT, gotCSRF, gotCookie string
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/me"):
			_, _ = w.Write([]byte(meBody))
		case r.Method == http.MethodPost:
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotAction = r.URL.Query().Get("action")
			gotCT = r.Header.Get("Content-Type")
			gotCSRF = r.Header.Get("csrf-token")
			gotCookie = r.Header.Get("Cookie")
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"value":{"createdAt":1}}`))
		}
	})
	raw, err := c.SendMessage(t.Context(), "urn:li:msg_conversation:2-AAA==", "hi there", time.Now())
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Contains(t, gotPath, "/voyagerMessagingDashMessengerMessages")
	assert.Equal(t, "createMessage", gotAction)
	assert.Equal(t, "text/plain; charset=UTF-8", gotCT, "the web client sends plain text, not JSON")
	assert.Equal(t, "ajax:99", gotCSRF, "csrf drops the JSESSIONID quotes on POST too")
	assert.Equal(t, `li_at=LI_AT_X; JSESSIONID="ajax:99"`, gotCookie)
	assert.NotEmpty(t, raw)

	// The exact Dash createMessage envelope (DECISIONS.md #29).
	var body map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &body))
	msg := body["message"].(map[string]any)
	assert.Equal(t, "hi there", msg["body"].(map[string]any)["text"])
	assert.Equal(t, "urn:li:msg_conversation:2-AAA==", msg["conversationUrn"])
	assert.Equal(t, "origin-fixed-uuid", msg["originToken"])
	assert.Equal(t, "urn:li:fsd_profile:ACoAA1", body["mailboxUrn"])
	assert.Equal(t, "track-fixed-1234", body["trackingId"])
	assert.Equal(t, true, body["dedupeByClientGeneratedToken"])
}

// TestSendMessage_NeverRetriesPOST pins the ban-safety rule that a send is fired at most ONCE
// even when the retry budget would allow retries for a GET: a 500 must not re-send.
func TestSendMessage_NeverRetriesPOST(t *testing.T) {
	var postHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/me") {
			_, _ = w.Write([]byte(meBody))
			return
		}
		postHits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := NewClientWithBaseURL(srv.URL,
		WithHTTPClient(srv.Client()),
		WithMaxRetries(2), // a GET would retry twice; the POST path must ignore this budget
		WithCookies(testCookies),
	)
	_, err := c.SendMessage(t.Context(), "2-AAA==", "hi", time.Now())
	require.Error(t, err)
	assert.Equal(t, 1, postHits, "a POST must never be retried")
}

func TestSendMessage_DailyCapRefuses(t *testing.T) {
	var postHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/me") {
			_, _ = w.Write([]byte(meBody))
			return
		}
		postHits++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClientWithBaseURL(srv.URL,
		WithHTTPClient(srv.Client()),
		WithMaxRetries(0),
		WithCookies(testCookies),
		WithPacer(&Pacer{DailySendCap: 1, StatePath: filepath.Join(t.TempDir(), "state.json")}),
	)
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	_, err := c.SendMessage(t.Context(), "2-AAA==", "first", now)
	require.NoError(t, err)
	_, err = c.SendMessage(t.Context(), "2-AAA==", "second", now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily message-send cap reached")
	assert.Contains(t, err.Error(), "--daily-send-cap")
	assert.Equal(t, 1, postHits, "the capped send must refuse BEFORE any request")
}

func TestSendMessage_DryRun(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	var buf bytes.Buffer
	statePath := filepath.Join(t.TempDir(), "state.json")
	c := NewClientWithBaseURL(srv.URL,
		WithHTTPClient(srv.Client()),
		WithCookies(testCookies),
		WithDryRun(true, &buf),
		WithPacer(&Pacer{DailySendCap: 1, StatePath: statePath}),
	)
	raw, err := c.SendMessage(t.Context(), "2-AAA==", "preview only", time.Now())
	require.NoError(t, err)
	assert.Nil(t, raw)
	out := buf.String()
	assert.Contains(t, out, "curl -X POST")
	assert.Contains(t, out, "action=createMessage")
	assert.Contains(t, out, "--data")
	assert.Contains(t, out, "preview only")
	assert.Contains(t, out, "urn:li:fsd_profile:<ME>", "dry-run uses the offline mailbox placeholder — no /me call")
	assert.Contains(t, out, "REDACTED")
	assert.NotContains(t, out, "LI_AT_X", "cookies stay redacted in the previewed curl")
	assert.Equal(t, 0, hits, "dry-run must send nothing (not even /me)")
	// Dry-run must not charge the daily send budget either.
	p := &Pacer{DailySendCap: 1, StatePath: statePath}
	assert.Equal(t, 1, p.DailySendRemaining(time.Now()))
}

func TestListConversations_DryRunUsesPlaceholder(t *testing.T) {
	var hits int
	var buf bytes.Buffer
	c := New("https://www.linkedin.com/voyager/api", "https://www.linkedin.com",
		WithCookies(testCookies), WithDryRun(true, &buf),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			hits++
			return nil, assert.AnError
		})}))
	res, err := c.ListConversations(t.Context(), 0)
	require.NoError(t, err)
	assert.Nil(t, res)
	out := buf.String()
	assert.Contains(t, out, "/voyagerMessagingGraphQL/graphql")
	assert.Contains(t, out, "mailboxUrn:urn:li:fsd_profile:<ME>")
	assert.Equal(t, 0, hits, "dry-run must not hit /me or the GraphQL surface")
}

// TestSendTokenSeams proves the default token generators produce distinct, well-formed tokens
// (the seams are only overridden in tests; production randomness must still be sane).
func TestSendTokenSeams(t *testing.T) {
	a, b := randomUUIDv4(), randomUUIDv4()
	assert.NotEqual(t, a, b)
	assert.Len(t, a, 36, "uuid v4 canonical form")
	assert.Equal(t, byte('4'), a[14], "version nibble")
	tid := randomTrackingID()
	assert.Len(t, tid, 16)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
