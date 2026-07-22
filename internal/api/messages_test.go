package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const conversationsBody = `{
  "data":{"data":{"paging":{"total":1,"start":0,"count":1},"*elements":["urn:li:fs_conversation:2-AAA=="]}},
  "included":[
    {"$type":"com.linkedin.voyager.messaging.Conversation","entityUrn":"urn:li:fs_conversation:2-AAA==",
     "lastActivityAt":1721500000000,"*participants":["urn:li:fs_messagingMember:(2-AAA==,M1)"],
     "*events":["urn:li:fs_event:(2-AAA==,S1)"]},
    {"$type":"com.linkedin.voyager.messaging.MessagingMember","entityUrn":"urn:li:fs_messagingMember:(2-AAA==,M1)",
     "miniProfile":{"firstName":"Alice","lastName":"Smith"}},
    {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-AAA==,S1)",
     "createdAt":1721500000000,"*from":"urn:li:fs_messagingMember:(2-AAA==,M1)",
     "eventContent":{"com.linkedin.voyager.messaging.event.MessageEvent":{"attributedBody":{"text":"hola"}}}}
  ]}`

const eventsBody = `{
  "data":{"data":{"paging":{"total":1,"start":0,"count":1},"*elements":["urn:li:fs_event:(2-AAA==,S1)"]}},
  "included":[
    {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-AAA==,S1)",
     "createdAt":1721500000000,
     "from":{"com.linkedin.voyager.messaging.MessagingMember":{"miniProfile":{"firstName":"Alice","lastName":"Smith"}}},
     "eventContent":{"com.linkedin.voyager.messaging.event.MessageEvent":{"attributedBody":{"text":"hola"}}}}
  ]}`

func TestListConversations(t *testing.T) {
	var gotPath, gotKeyVersion, gotCount string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKeyVersion = r.URL.Query().Get("keyVersion")
		gotCount = r.URL.Query().Get("count")
		_, _ = w.Write([]byte(conversationsBody))
	})
	res, err := c.ListConversations(t.Context(), 0)
	require.NoError(t, err)
	assert.Contains(t, gotPath, "/messaging/conversations")
	assert.Equal(t, "LEGACY_INBOX", gotKeyVersion)
	assert.Equal(t, "20", gotCount, "default count")
	require.Len(t, res.Conversations, 1)
	assert.Equal(t, "2-AAA==", res.Conversations[0].ID)
	assert.Equal(t, "hola", res.Conversations[0].Snippet)
	assert.NotEmpty(t, res.Raw)
}

func TestGetConversationEvents(t *testing.T) {
	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(eventsBody))
	})
	res, err := c.GetConversationEvents(t.Context(), "2-AAA==")
	require.NoError(t, err)
	assert.Contains(t, gotPath, "/messaging/conversations/2-AAA==/events")
	require.Len(t, res.Messages, 1)
	assert.Equal(t, "Alice Smith", res.Messages[0].Sender)
	assert.Equal(t, "hola", res.Messages[0].Text)
}

func TestSendMessage_PostShape(t *testing.T) {
	var gotMethod, gotPath, gotAction, gotCT, gotCSRF, gotCookie string
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAction = r.URL.Query().Get("action")
		gotCT = r.Header.Get("Content-Type")
		gotCSRF = r.Header.Get("csrf-token")
		gotCookie = r.Header.Get("Cookie")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"value":{"createdAt":1}}}`))
	})
	raw, err := c.SendMessage(t.Context(), "2-AAA==", "hi there", time.Now())
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Contains(t, gotPath, "/messaging/conversations/2-AAA==/events")
	assert.Equal(t, "create", gotAction)
	assert.Equal(t, "application/json", gotCT)
	assert.Equal(t, "ajax:99", gotCSRF, "csrf drops the JSESSIONID quotes on POST too")
	assert.Equal(t, `li_at=LI_AT_X; JSESSIONID="ajax:99"`, gotCookie)
	assert.NotEmpty(t, raw)

	// The exact community-proven MessageCreate envelope (DECISIONS.md #24).
	var body map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &body))
	value := body["eventCreate"].(map[string]any)["value"].(map[string]any)
	mc, ok := value["com.linkedin.voyager.messaging.create.MessageCreate"].(map[string]any)
	require.True(t, ok, "MessageCreate type key")
	ab := mc["attributedBody"].(map[string]any)
	assert.Equal(t, "hi there", ab["text"])
	assert.Empty(t, ab["attributes"])
	assert.Empty(t, mc["attachments"])
}

// TestSendMessage_NeverRetriesPOST pins the ban-safety rule that a send is fired at most
// ONCE even when the retry budget would allow retries for a GET: a 500 must not re-send a
// message (a duplicate DM is worse than a failed one, and retry-hammering flags accounts).
func TestSendMessage_NeverRetriesPOST(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
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
	assert.Equal(t, 1, hits, "a POST must never be retried")
}

func TestSendMessage_DailyCapRefuses(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
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
	assert.Equal(t, 1, hits, "the capped send must refuse BEFORE any request")
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
	assert.Contains(t, out, "action=create")
	assert.Contains(t, out, "--data")
	assert.Contains(t, out, "preview only")
	assert.Contains(t, out, "REDACTED")
	assert.NotContains(t, out, "LI_AT_X", "cookies stay redacted in the previewed curl")
	assert.Equal(t, 0, hits, "dry-run must send nothing")
	// Dry-run must not charge the daily send budget either.
	p := &Pacer{DailySendCap: 1, StatePath: statePath}
	assert.Equal(t, 1, p.DailySendRemaining(time.Now()))
}
