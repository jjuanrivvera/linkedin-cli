package commands

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const meJSON = `{"miniProfile":{"entityUrn":"urn:li:fs_miniProfile:ACoAA0","dashEntityUrn":"urn:li:fsd_profile:ACoAA0"}}`

const conversationsJSON = `{"data":{"messengerConversationsBySyncToken":{"elements":[
  {"entityUrn":"urn:li:msg_conversation:2-AAA==","lastActivityAt":1721500000000,
   "conversationParticipants":[{"participantType":{"member":{"firstName":{"text":"Alice"},"lastName":{"text":"Smith"}}}}],
   "messages":{"elements":[{"deliveredAt":1721500000000,"body":{"text":"see you soon"}}]}},
  {"entityUrn":"urn:li:msg_conversation:2-BBB==","lastActivityAt":1721600000000,
   "conversationParticipants":[{"participantType":{"member":{"firstName":{"text":"Bob"},"lastName":{"text":"Jones"}}}}],
   "messages":{"elements":[{"deliveredAt":1721600000000,"body":{"text":"thanks for connecting"}}]}}
]}}}`

const eventsJSON = `{"data":{"messengerMessagesByAnchorTimestamp":{"elements":[
  {"entityUrn":"urn:li:msg_message:S2","deliveredAt":1721502000000,
   "sender":{"participantType":{"member":{"firstName":{"text":"Bob"},"lastName":{"text":"Jones"}}}},
   "body":{"text":"newer message"}},
  {"entityUrn":"urn:li:msg_message:S1","deliveredAt":1721501000000,
   "sender":{"participantType":{"member":{"firstName":{"text":"Alice"},"lastName":{"text":"Smith"}}}},
   "body":{"text":"older message"}}
]}}}`

// messagingRouter serves the GraphQL messenger surface (/me + list/read GETs) and records every
// send POST to the Dash createMessage endpoint.
func messagingRouter(t *testing.T, sends *int, sentBodies *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "voyagerMessagingDashMessengerMessages"):
			if sends != nil {
				*sends++
			}
			if sentBodies != nil {
				b, _ := io.ReadAll(r.Body)
				*sentBodies = append(*sentBodies, string(b))
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"value":{"createdAt":1721700000000}}`))
		case strings.HasSuffix(r.URL.Path, "/me"):
			_, _ = w.Write([]byte(meJSON))
		case strings.Contains(r.URL.Path, "voyagerMessagingGraphQL"):
			if strings.HasPrefix(r.URL.Query().Get("queryId"), "messengerConversations") {
				_, _ = w.Write([]byte(conversationsJSON))
			} else {
				_, _ = w.Write([]byte(eventsJSON))
			}
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}
}

func TestMessagesList_Table(t *testing.T) {
	e := newEnv(t, messagingRouter(t, nil, nil))
	out, _, err := e.run("messages", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "PARTICIPANTS")
	// Most recent conversation first.
	assert.Less(t, strings.Index(out, "2-BBB=="), strings.Index(out, "2-AAA=="))
	assert.Contains(t, out, "Bob Jones")
	assert.Contains(t, out, "thanks for connecting")
}

func TestMessagesList_JSONCarriesFullEntity(t *testing.T) {
	e := newEnv(t, messagingRouter(t, nil, nil))
	out, _, err := e.run("messages", "list", "--count", "5", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"id": "urn:li:msg_conversation:2-BBB=="`, "id is the full URN read/send accept")
	assert.Contains(t, out, `"entity"`, "full conversation element under -o json")
	assert.Contains(t, out, "lastActivityAt", "raw GraphQL fields preserved")
}

func TestMessagesRead_OldestFirst(t *testing.T) {
	e := newEnv(t, messagingRouter(t, nil, nil))
	out, _, err := e.run("messages", "read", "urn:li:msg_conversation:2-AAA==")
	require.NoError(t, err)
	assert.Contains(t, out, "SENDER")
	assert.Less(t, strings.Index(out, "older message"), strings.Index(out, "newer message"))
	assert.Contains(t, out, "Alice Smith")
}

func TestMessagesSend_WithYes(t *testing.T) {
	var sends int
	var bodies []string
	e := newEnv(t, messagingRouter(t, &sends, &bodies))
	out, errOut, err := e.run("messages", "send", "urn:li:msg_conversation:2-AAA==", "--text", "hello!", "--yes")
	require.NoError(t, err)
	assert.Equal(t, 1, sends)
	assert.Contains(t, errOut, "⚠", "warning printed to stderr before sending")
	assert.Contains(t, errOut, "message sent")
	assert.Empty(t, out, "nothing on stdout without an explicit -o format")
	require.Len(t, bodies, 1)
	assert.Contains(t, bodies[0], `"conversationUrn":"urn:li:msg_conversation:2-AAA=="`)
	assert.Contains(t, bodies[0], `"text":"hello!"`)
	assert.Contains(t, bodies[0], `"mailboxUrn":"urn:li:fsd_profile:ACoAA0"`)
	assert.Contains(t, bodies[0], `"dedupeByClientGeneratedToken":true`)
}

func TestMessagesSend_AbortsWithoutConfirmation(t *testing.T) {
	// runWithDeps wires an EMPTY stdin: the confirmation prompt cannot be answered, so the
	// send must abort — automation that forgets --yes fails safe.
	var sends int
	e := newEnv(t, messagingRouter(t, &sends, nil))
	_, errOut, err := e.run("messages", "send", "2-AAA==", "--text", "hello!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOT sent")
	assert.Contains(t, errOut, "⚠")
	assert.Equal(t, 0, sends, "no request without confirmation")
}

func TestMessagesSend_InteractiveYes(t *testing.T) {
	var sends int
	e := newEnv(t, messagingRouter(t, &sends, nil))
	d := e.deps()
	var out, errB bytes.Buffer
	d.out = &out
	root := newRootCmd(d)
	root.SetArgs([]string{"messages", "send", "2-AAA==", "--text", "hi"})
	root.SetOut(&out)
	root.SetErr(&errB)
	root.SetIn(strings.NewReader("y\n"))
	require.NoError(t, root.ExecuteContext(t.Context()))
	assert.Equal(t, 1, sends)
	assert.Contains(t, errB.String(), "Send it?")
}

func TestMessagesSend_EmptyTextRefused(t *testing.T) {
	var sends int
	e := newEnv(t, messagingRouter(t, &sends, nil))
	_, _, err := e.run("messages", "send", "2-AAA==", "--text", "  ", "--yes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--text")
	assert.Equal(t, 0, sends)
}

// TestMessagesSend_DryRun proves the send dry-run is a fully offline preview: no session
// needed, no /me call, no confirmation prompt, cookies redacted, nothing sent.
func TestMessagesSend_DryRun(t *testing.T) {
	var hits int
	e := newEnv(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	t.Setenv("LI_AT", "")
	t.Setenv("JSESSIONID", "")
	out, _, err := e.run("messages", "send", "urn:li:msg_conversation:2-AAA==", "--text", "preview", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "curl -X POST")
	assert.Contains(t, out, "action=createMessage")
	assert.Contains(t, out, "preview")
	assert.Contains(t, out, "urn:li:fsd_profile:<ME>", "offline mailbox placeholder")
	assert.Contains(t, out, "REDACTED")
	assert.Equal(t, 0, hits, "dry-run must send nothing")
}

// TestMessagesSend_DailyCapRefusal drives the persisted send cap end-to-end through the
// command: with a cap of 1, the second invocation refuses before any request.
func TestMessagesSend_DailyCapRefusal(t *testing.T) {
	var sends int
	e := newEnv(t, messagingRouter(t, &sends, nil))
	d := e.deps() // shared deps → shared state.json across both runs

	_, _, err := runWithDeps(t, d, "messages", "send", "2-AAA==", "--text", "one", "--yes", "--daily-send-cap", "1")
	require.NoError(t, err)
	_, _, err = runWithDeps(t, d, "messages", "send", "2-AAA==", "--text", "two", "--yes", "--daily-send-cap", "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily message-send cap reached")
	assert.Equal(t, 1, sends, "the capped send must refuse before any request")
}

func TestMessagesList_DryRun(t *testing.T) {
	var hits int
	e := newEnv(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	out, _, err := e.run("messages", "list", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "voyagerMessagingGraphQL/graphql")
	assert.Contains(t, out, "mailboxUrn:urn:li:fsd_profile:<ME>")
	assert.Contains(t, out, "queryId=messengerConversations")
	assert.Equal(t, 0, hits, "dry-run must not call /me or the GraphQL surface")
}

func TestMessagesRead_DryRun(t *testing.T) {
	var hits int
	e := newEnv(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(500)
	})
	out, _, err := e.run("messages", "read", "2-AAA==", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "queryId=messengerMessages")
	// A real conversation URN is URL-escaped in the blob (only the dry-run mailbox placeholder
	// stays literal); its colons become %3A.
	assert.Contains(t, out, "conversationUrn:urn%3Ali%3Amsg_conversation%3A2-AAA")
	assert.Equal(t, 0, hits, "dry-run makes no request")
}

func TestDailySendCapFlagWiresIntoPacer(t *testing.T) {
	e := newEnv(t, nil)
	_ = e // env isolation only (XDG_CONFIG_HOME, session env)
	d := newDeps()
	d.gf.dailySendCap = 3
	c, _, err := d.getAPIClient()
	require.NoError(t, err)
	require.NotNil(t, c.Pacer())
	assert.Equal(t, 3, c.Pacer().DailySendCap)

	d2 := newDeps()
	c2, _, err := d2.getAPIClient()
	require.NoError(t, err)
	assert.Equal(t, 20, c2.Pacer().DailySendCap, "shipped default")
}

// TestMessagesMCPAndAnnotations pins the agent surface: list/read are read-only MCP tools,
// send carries the destructive annotation (never a safe tool), and the messages group is NOT
// blanket-excluded from MCP (exclusions stay exact-path, never substring).
func TestMessagesMCPAndAnnotations(t *testing.T) {
	root := NewRootCmd()
	var messages *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "messages" {
			messages = c
		}
	}
	require.NotNil(t, messages)
	assert.False(t, mcpExcluded(messages), "messages group stays on the MCP surface")

	find := func(name string) *cobra.Command {
		for _, c := range messages.Commands() {
			if c.Name() == name {
				return c
			}
		}
		return nil
	}
	for _, name := range []string{"list", "read"} {
		c := find(name)
		require.NotNil(t, c, name)
		assert.Equal(t, "true", c.Annotations[annReadOnly], "%s is a read", name)
	}
	send := find("send")
	require.NotNil(t, send)
	assert.Equal(t, "true", send.Annotations[annDestructive], "send must carry destructiveHint")
	assert.NotEqual(t, "true", send.Annotations[annReadOnly], "send must never look read-only/safe")
	// Help text carries the explicit unofficial-API / restriction-risk warning.
	assert.Contains(t, send.Long, "⚠")
	assert.Contains(t, messages.Long, "ACCOUNT-RESTRICTION")
}
