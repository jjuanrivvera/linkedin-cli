package voyager

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// ErrMessagingSchemaMoved is the messaging twin of ErrSchemaMoved: a GraphQL messenger
// response decoded structurally but did not carry the expected result container
// (messengerConversationsBySyncToken / messengerMessagesByAnchorTimestamp). That is the
// signature of a queryId rotation or a renamed GraphQL field — NOT an empty inbox (which
// carries the container with an empty elements[]). The queryId hashes drift HARD; see the
// MESSENGER GRAPHQL banner in internal/voyager/schema.go.
var ErrMessagingSchemaMoved = errors.New(
	"no recognizable messenger result in the GraphQL response — LinkedIn likely rotated the " +
		"queryId hash or renamed the result field. Refresh the queryId/field constants in the " +
		"MESSENGER GRAPHQL section of internal/voyager/schema.go")

// Conversation is the thin, table-friendly slice of one messenger conversation. The complete
// element is preserved in Raw for -o json. ID is the FULL msg_conversation URN — exactly what
// `messages read` / `messages send` accept (list output feeds read/send verbatim).
type Conversation struct {
	ID             string          `json:"id,omitempty"`
	Participants   []string        `json:"participants,omitempty"`
	LastActivityAt int64           `json:"lastActivityAt,omitempty"` // epoch ms
	Snippet        string          `json:"snippet,omitempty"`
	URN            string          `json:"urn,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

// Message is one resolved thread message: who said what, when. Raw preserves the full element
// for -o json.
type Message struct {
	Sender    string          `json:"sender,omitempty"`
	CreatedAt int64           `json:"createdAt,omitempty"` // deliveredAt, epoch ms
	Text      string          `json:"text,omitempty"`
	URN       string          `json:"urn,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// gqlMember is the {firstName,lastName} pair on a participant's member union. Both names are
// attributed strings ({text}) but a bare "…" string is tolerated via textOrStr.
type gqlMember struct {
	FirstName textOrStr `json:"firstName"`
	LastName  textOrStr `json:"lastName"`
}

// gqlParticipant is a conversationParticipant / message sender. participantType is a union;
// only the member variant carries a person name (other variants — org, bot — yield "").
type gqlParticipant struct {
	ParticipantType struct {
		Member *gqlMember `json:"member"`
	} `json:"participantType"`
}

func (p gqlParticipant) name() string {
	if p.ParticipantType.Member == nil {
		return ""
	}
	return joinName(p.ParticipantType.Member.FirstName.Text, p.ParticipantType.Member.LastName.Text)
}

// gqlConversation is the field subset read off a messengerConversationsBySyncToken element.
type gqlConversation struct {
	EntityURN                string           `json:"entityUrn"`
	LastActivityAt           int64            `json:"lastActivityAt"`
	ConversationParticipants []gqlParticipant `json:"conversationParticipants"`
	Messages                 struct {
		Elements []json.RawMessage `json:"elements"`
	} `json:"messages"`
}

// gqlMessage is the field subset read off a messengerMessagesByAnchorTimestamp element (and,
// for the inbox snippet, off a conversation's embedded messages.elements[0]).
type gqlMessage struct {
	EntityURN   string         `json:"entityUrn"`
	DeliveredAt int64          `json:"deliveredAt"`
	Sender      gqlParticipant `json:"sender"`
	Body        struct {
		Text string `json:"text"`
	} `json:"body"`
}

// ParseConversations decodes a messenger conversations GraphQL page into thin conversations
// sorted most-recent-first. A response missing the result container returns
// ErrMessagingSchemaMoved (a rotated queryId / renamed field), never a silent empty inbox — an
// honest empty inbox carries the container with an empty elements[].
func ParseConversations(raw json.RawMessage) ([]Conversation, error) {
	elements, err := gqlElements(raw, KeyConversationsResult)
	if err != nil {
		return nil, err
	}
	out := make([]Conversation, 0, len(elements))
	for _, el := range elements {
		var c gqlConversation
		_ = json.Unmarshal(el, &c)
		conv := Conversation{
			ID:             c.EntityURN,
			URN:            c.EntityURN,
			LastActivityAt: c.LastActivityAt,
			Snippet:        latestMessageText(c.Messages.Elements),
			Raw:            el,
		}
		for _, p := range c.ConversationParticipants {
			if n := p.name(); n != "" {
				conv.Participants = append(conv.Participants, n)
			}
		}
		out = append(out, conv)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActivityAt > out[j].LastActivityAt })
	return out, nil
}

// ParseMessages decodes a messenger thread GraphQL page into resolved messages sorted
// oldest→newest. Same drift rule as ParseConversations.
func ParseMessages(raw json.RawMessage) ([]Message, error) {
	elements, err := gqlElements(raw, KeyMessagesResult)
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(elements))
	for _, el := range elements {
		var m gqlMessage
		_ = json.Unmarshal(el, &m)
		out = append(out, Message{
			Sender:    m.Sender.name(),
			CreatedAt: m.DeliveredAt,
			Text:      m.Body.Text,
			URN:       m.EntityURN,
			Raw:       el,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

// gqlElements pulls the elements[] list out of {data:{<key>:{elements:[…]}}}. A missing result
// container (key absent) is a moved schema (ErrMessagingSchemaMoved); the container present
// with an empty (or absent) elements[] is an honest empty inbox/thread. Malformed JSON returns
// the decode error.
func gqlElements(raw json.RawMessage, key string) ([]json.RawMessage, error) {
	var env struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	container, ok := env.Data[key]
	if !ok {
		return nil, ErrMessagingSchemaMoved
	}
	var wrap struct {
		Elements []json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(container, &wrap); err != nil {
		return nil, err
	}
	return wrap.Elements, nil
}

// latestMessageText returns the newest embedded message's body text — the inbox snippet.
// LinkedIn ships messages.elements[0] as the latest, but we scan defensively for the
// highest deliveredAt (falling back to the first non-empty body) so wire order can't fool us.
func latestMessageText(elements []json.RawMessage) string {
	best := gqlMessage{DeliveredAt: -1}
	found := false
	for _, el := range elements {
		var m gqlMessage
		if json.Unmarshal(el, &m) != nil || m.Body.Text == "" {
			continue
		}
		if !found || m.DeliveredAt > best.DeliveredAt {
			best = m
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.Body.Text
}

// MailboxURNFromMe resolves the caller's fsd_profile mailbox URN from a /me response. It
// prefers miniProfile.dashEntityUrn; absent that, it converts miniProfile.entityUrn
// (urn:li:fs_miniProfile:<ID> → urn:li:fsd_profile:<ID>). ErrMessagingSchemaMoved when neither
// is present — /me shape moved.
func MailboxURNFromMe(raw json.RawMessage) (string, error) {
	var me struct {
		MiniProfile struct {
			EntityURN     string `json:"entityUrn"`
			DashEntityURN string `json:"dashEntityUrn"`
		} `json:"miniProfile"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		return "", err
	}
	if me.MiniProfile.DashEntityURN != "" {
		return me.MiniProfile.DashEntityURN, nil
	}
	if id := strings.TrimPrefix(me.MiniProfile.EntityURN, MiniProfileURNPrefix); id != me.MiniProfile.EntityURN && id != "" {
		return ProfileURNPrefix + id, nil
	}
	return "", ErrMessagingSchemaMoved
}

// EnsureConversationURN prefixes a bare conversation id with the msg_conversation namespace so
// read/send accept either the full URN that `messages list` prints or a bare id.
func EnsureConversationURN(id string) string {
	if strings.HasPrefix(id, "urn:") {
		return id
	}
	return ConversationURNPrefix + id
}

func joinName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}
