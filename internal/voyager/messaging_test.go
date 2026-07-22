package voyager

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// conversationsFixture exercises every participant/miniProfile shape the parser tolerates:
// a *miniProfile reference (Alice) and an inline miniProfile (Bob), plus a snippet event per
// conversation (one {text} object, one bare-string attributedBody).
const conversationsFixture = `{
  "data":{"data":{"paging":{"total":2,"start":0,"count":2},"*elements":[
    "urn:li:fs_conversation:2-AAA==","urn:li:fs_conversation:2-BBB=="]}},
  "included":[
    {"$type":"com.linkedin.voyager.messaging.Conversation","entityUrn":"urn:li:fs_conversation:2-AAA==",
     "lastActivityAt":1721500000000,
     "*participants":["urn:li:fs_messagingMember:(2-AAA==,ACoAA1)"],
     "*events":["urn:li:fs_event:(2-AAA==,S1)"]},
    {"$type":"com.linkedin.voyager.messaging.Conversation","entityUrn":"urn:li:fs_conversation:2-BBB==",
     "lastActivityAt":1721600000000,
     "*participants":["urn:li:fs_messagingMember:(2-BBB==,ACoAA2)"],
     "*events":["urn:li:fs_event:(2-BBB==,S2)"]},
    {"$type":"com.linkedin.voyager.messaging.MessagingMember","entityUrn":"urn:li:fs_messagingMember:(2-AAA==,ACoAA1)",
     "*miniProfile":"urn:li:fs_miniProfile:ACoAA1"},
    {"$type":"com.linkedin.voyager.messaging.MessagingMember","entityUrn":"urn:li:fs_messagingMember:(2-BBB==,ACoAA2)",
     "miniProfile":{"firstName":"Bob","lastName":"Jones"}},
    {"$type":"com.linkedin.voyager.identity.shared.MiniProfile","entityUrn":"urn:li:fs_miniProfile:ACoAA1",
     "firstName":"Alice","lastName":"Smith"},
    {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-AAA==,S1)",
     "createdAt":1721500000000,"*from":"urn:li:fs_messagingMember:(2-AAA==,ACoAA1)",
     "eventContent":{"com.linkedin.voyager.messaging.event.MessageEvent":{"attributedBody":{"text":"hey there","attributes":[]}}}},
    {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-BBB==,S2)",
     "createdAt":1721600000000,"*from":"urn:li:fs_messagingMember:(2-BBB==,ACoAA2)",
     "eventContent":{"com.linkedin.voyager.messaging.event.MessageEvent":{"attributedBody":"bare snippet"}}}
  ]}`

func TestParseConversations(t *testing.T) {
	convs, err := ParseConversations(json.RawMessage(conversationsFixture))
	require.NoError(t, err)
	require.Len(t, convs, 2)

	// Most recent first: BBB (1721600000000) before AAA.
	assert.Equal(t, "2-BBB==", convs[0].ID)
	assert.Equal(t, []string{"Bob Jones"}, convs[0].Participants, "inline miniProfile name")
	assert.Equal(t, "bare snippet", convs[0].Snippet, "bare-string attributedBody tolerated")
	assert.Equal(t, int64(1721600000000), convs[0].LastActivityAt)

	assert.Equal(t, "2-AAA==", convs[1].ID)
	assert.Equal(t, []string{"Alice Smith"}, convs[1].Participants, "*miniProfile reference resolved")
	assert.Equal(t, "hey there", convs[1].Snippet)
	assert.NotEmpty(t, convs[1].Raw, "full entity preserved for -o json")
}

func TestParseConversations_EmptyInboxIsFine(t *testing.T) {
	raw := json.RawMessage(`{"data":{"data":{"paging":{"total":0,"start":0,"count":0},"*elements":[]}},"included":[]}`)
	convs, err := ParseConversations(raw)
	require.NoError(t, err)
	assert.Empty(t, convs)
}

func TestParseConversations_SchemaMoved(t *testing.T) {
	// Positive total but zero recognizable conversation entities ⇒ the schema rotated.
	raw := json.RawMessage(`{"data":{"data":{"paging":{"total":3,"start":0,"count":3},"*elements":[]}},
	  "included":[{"$type":"com.linkedin.voyager.messaging.SomethingNew","entityUrn":"urn:x:1"}]}`)
	_, err := ParseConversations(raw)
	require.ErrorIs(t, err, ErrMessagingSchemaMoved)
	assert.Contains(t, err.Error(), "internal/voyager/schema.go")
}

// eventsFixture arrives deliberately out of order; message 2 uses an inline (type-keyed)
// `from` and a plain `body` fallback instead of attributedBody.
const eventsFixture = `{
  "data":{"data":{"paging":{"total":2,"start":0,"count":2},"*elements":[
    "urn:li:fs_event:(2-AAA==,S2)","urn:li:fs_event:(2-AAA==,S1)"]}},
  "included":[
    {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-AAA==,S2)",
     "createdAt":1721502000000,
     "from":{"com.linkedin.voyager.messaging.MessagingMember":{"miniProfile":{"firstName":"Bob","lastName":"Jones"}}},
     "eventContent":{"com.linkedin.voyager.messaging.event.MessageEvent":{"body":"second message"}}},
    {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-AAA==,S1)",
     "createdAt":1721501000000,"*from":"urn:li:fs_messagingMember:(2-AAA==,ACoAA1)",
     "eventContent":{"com.linkedin.voyager.messaging.event.MessageEvent":{"attributedBody":{"text":"first message","attributes":[]}}}},
    {"$type":"com.linkedin.voyager.messaging.MessagingMember","entityUrn":"urn:li:fs_messagingMember:(2-AAA==,ACoAA1)",
     "*miniProfile":"urn:li:fs_miniProfile:ACoAA1"},
    {"$type":"com.linkedin.voyager.identity.shared.MiniProfile","entityUrn":"urn:li:fs_miniProfile:ACoAA1",
     "firstName":"Alice","lastName":"Smith"}
  ]}`

func TestParseEvents(t *testing.T) {
	msgs, err := ParseEvents(json.RawMessage(eventsFixture))
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	// Oldest → newest regardless of wire order.
	assert.Equal(t, "first message", msgs[0].Text)
	assert.Equal(t, "Alice Smith", msgs[0].Sender, "*from reference resolved")
	assert.Equal(t, int64(1721501000000), msgs[0].CreatedAt)

	assert.Equal(t, "second message", msgs[1].Text, "plain body fallback")
	assert.Equal(t, "Bob Jones", msgs[1].Sender, "inline type-keyed from resolved")
	assert.NotEmpty(t, msgs[1].Raw)
}

func TestParseEvents_FlattenedAttributedBody(t *testing.T) {
	raw := json.RawMessage(`{"data":{},"included":[
	  {"$type":"com.linkedin.voyager.messaging.Event","entityUrn":"urn:li:fs_event:(2-C==,S9)",
	   "createdAt":1,"eventContent":{"attributedBody":{"text":"flat variant"}}}]}`)
	msgs, err := ParseEvents(raw)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "flat variant", msgs[0].Text)
}

func TestParseEvents_SchemaMoved(t *testing.T) {
	raw := json.RawMessage(`{"data":{"data":{"paging":{"total":5,"start":0,"count":5},"*elements":[]}},"included":[]}`)
	_, err := ParseEvents(raw)
	require.ErrorIs(t, err, ErrMessagingSchemaMoved)
}

func TestParseEvents_EmptyThreadIsFine(t *testing.T) {
	raw := json.RawMessage(`{"data":{"data":{"paging":{"total":0,"start":0,"count":0},"*elements":[]}},"included":[]}`)
	msgs, err := ParseEvents(raw)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestParseConversations_BadJSON(t *testing.T) {
	_, err := ParseConversations(json.RawMessage(`{`))
	require.Error(t, err)
	_, err = ParseEvents(json.RawMessage(`{`))
	require.Error(t, err)
}
