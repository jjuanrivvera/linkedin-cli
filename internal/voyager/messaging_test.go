package voyager

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conversationsFixture is a GraphQL messenger conversations page. It exercises the participant
// shapes the parser tolerates: attributed {text} names (Alice), a bare-string name (Bob), and a
// non-member participant variant that yields no name (an org/bot participant is skipped). Each
// conversation carries an embedded latest message as the inbox snippet.
const conversationsFixture = `{"data":{"messengerConversationsBySyncToken":{"elements":[
  {"entityUrn":"urn:li:msg_conversation:2-AAA==","lastActivityAt":1721500000000,
   "conversationParticipants":[
     {"participantType":{"member":{"firstName":{"text":"Alice"},"lastName":{"text":"Smith"}}}},
     {"participantType":{"organization":{"name":{"text":"Acme"}}}}],
   "messages":{"elements":[{"deliveredAt":1721500000000,"body":{"text":"hey there"}}]}},
  {"entityUrn":"urn:li:msg_conversation:2-BBB==","lastActivityAt":1721600000000,
   "conversationParticipants":[
     {"participantType":{"member":{"firstName":"Bob","lastName":"Jones"}}}],
   "messages":{"elements":[{"deliveredAt":1721600000000,"body":{"text":"bare snippet"}}]}}
]}}}`

func TestParseConversations(t *testing.T) {
	convs, err := ParseConversations(json.RawMessage(conversationsFixture))
	require.NoError(t, err)
	require.Len(t, convs, 2)

	// Most recent first: BBB (1721600000000) before AAA.
	assert.Equal(t, "urn:li:msg_conversation:2-BBB==", convs[0].ID, "id is the full URN read/send accept")
	assert.Equal(t, "urn:li:msg_conversation:2-BBB==", convs[0].URN)
	assert.Equal(t, []string{"Bob Jones"}, convs[0].Participants, "bare-string name tolerated")
	assert.Equal(t, "bare snippet", convs[0].Snippet)
	assert.Equal(t, int64(1721600000000), convs[0].LastActivityAt)

	assert.Equal(t, "urn:li:msg_conversation:2-AAA==", convs[1].ID)
	assert.Equal(t, []string{"Alice Smith"}, convs[1].Participants, "non-member participant skipped")
	assert.Equal(t, "hey there", convs[1].Snippet)
	assert.NotEmpty(t, convs[1].Raw, "full element preserved for -o json")
}

func TestParseConversations_EmptyInboxIsFine(t *testing.T) {
	raw := json.RawMessage(`{"data":{"messengerConversationsBySyncToken":{"elements":[]}}}`)
	convs, err := ParseConversations(raw)
	require.NoError(t, err)
	assert.Empty(t, convs)
}

func TestParseConversations_SchemaMoved(t *testing.T) {
	// The result container is absent (a rotated queryId / renamed field) ⇒ moved, not empty.
	raw := json.RawMessage(`{"data":{"someRenamedField":{"elements":[]}}}`)
	_, err := ParseConversations(raw)
	require.ErrorIs(t, err, ErrMessagingSchemaMoved)
	assert.Contains(t, err.Error(), "internal/voyager/schema.go")
}

// eventsFixture arrives deliberately out of order to prove oldest→newest sorting.
const eventsFixture = `{"data":{"messengerMessagesByAnchorTimestamp":{"elements":[
  {"entityUrn":"urn:li:msg_message:S2","deliveredAt":1721502000000,
   "sender":{"participantType":{"member":{"firstName":{"text":"Bob"},"lastName":{"text":"Jones"}}}},
   "body":{"text":"second message"}},
  {"entityUrn":"urn:li:msg_message:S1","deliveredAt":1721501000000,
   "sender":{"participantType":{"member":{"firstName":{"text":"Alice"},"lastName":{"text":"Smith"}}}},
   "body":{"text":"first message"}}
]}}}`

func TestParseMessages(t *testing.T) {
	msgs, err := ParseMessages(json.RawMessage(eventsFixture))
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	// Oldest → newest regardless of wire order.
	assert.Equal(t, "first message", msgs[0].Text)
	assert.Equal(t, "Alice Smith", msgs[0].Sender)
	assert.Equal(t, int64(1721501000000), msgs[0].CreatedAt)

	assert.Equal(t, "second message", msgs[1].Text)
	assert.Equal(t, "Bob Jones", msgs[1].Sender)
	assert.NotEmpty(t, msgs[1].Raw)
}

func TestParseMessages_MissingSenderNameTolerated(t *testing.T) {
	raw := json.RawMessage(`{"data":{"messengerMessagesByAnchorTimestamp":{"elements":[
	  {"deliveredAt":1,"sender":{"participantType":{}},"body":{"text":"anon"}}]}}}`)
	msgs, err := ParseMessages(raw)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "anon", msgs[0].Text)
	assert.Empty(t, msgs[0].Sender, "a missing member yields an empty sender, not a crash")
}

func TestParseMessages_SchemaMoved(t *testing.T) {
	raw := json.RawMessage(`{"data":{"otherField":{}}}`)
	_, err := ParseMessages(raw)
	require.ErrorIs(t, err, ErrMessagingSchemaMoved)
}

func TestParseMessages_EmptyThreadIsFine(t *testing.T) {
	raw := json.RawMessage(`{"data":{"messengerMessagesByAnchorTimestamp":{"elements":[]}}}`)
	msgs, err := ParseMessages(raw)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestParseMessaging_BadJSON(t *testing.T) {
	_, err := ParseConversations(json.RawMessage(`{`))
	require.Error(t, err)
	_, err = ParseMessages(json.RawMessage(`{`))
	require.Error(t, err)
}

func TestMailboxURNFromMe(t *testing.T) {
	// dashEntityUrn is preferred verbatim.
	urn, err := MailboxURNFromMe(json.RawMessage(
		`{"miniProfile":{"entityUrn":"urn:li:fs_miniProfile:ACoAA1","dashEntityUrn":"urn:li:fsd_profile:ACoAA1"}}`))
	require.NoError(t, err)
	assert.Equal(t, "urn:li:fsd_profile:ACoAA1", urn)

	// Absent dashEntityUrn ⇒ convert fs_miniProfile → fsd_profile.
	urn, err = MailboxURNFromMe(json.RawMessage(`{"miniProfile":{"entityUrn":"urn:li:fs_miniProfile:ACoAA9"}}`))
	require.NoError(t, err)
	assert.Equal(t, "urn:li:fsd_profile:ACoAA9", urn)

	// Neither ⇒ /me shape moved.
	_, err = MailboxURNFromMe(json.RawMessage(`{"miniProfile":{}}`))
	require.ErrorIs(t, err, ErrMessagingSchemaMoved)

	_, err = MailboxURNFromMe(json.RawMessage(`{`))
	require.Error(t, err)
}

func TestEnsureConversationURN(t *testing.T) {
	assert.Equal(t, "urn:li:msg_conversation:2-AAA==", EnsureConversationURN("2-AAA=="))
	assert.Equal(t, "urn:li:msg_conversation:2-AAA==", EnsureConversationURN("urn:li:msg_conversation:2-AAA=="))
}
