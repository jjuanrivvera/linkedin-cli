package voyager

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// ErrMessagingSchemaMoved is the messaging twin of ErrSchemaMoved: a conversations/events
// page decoded structurally but contained no recognizable messaging entities while its paging
// says content exists — the signature of a Voyager messaging schema rotation, NOT an empty
// inbox (which has total 0 and is fine).
var ErrMessagingSchemaMoved = errors.New(
	"no recognizable messaging entities in the response — LinkedIn likely rotated the Voyager " +
		"messaging schema ($type / eventContent key / keyVersion). Check and bump the constants " +
		"in internal/voyager/schema.go")

// Conversation is the thin, table-friendly slice of one inbox conversation. The complete
// entity is preserved in Raw for -o json.
type Conversation struct {
	ID             string          `json:"id,omitempty"`
	Participants   []string        `json:"participants,omitempty"`
	LastActivityAt int64           `json:"lastActivityAt,omitempty"` // epoch ms
	Snippet        string          `json:"snippet,omitempty"`
	URN            string          `json:"urn,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

// Message is one resolved thread event: who said what, when. Raw preserves the full event
// entity for -o json.
type Message struct {
	Sender    string          `json:"sender,omitempty"`
	CreatedAt int64           `json:"createdAt,omitempty"` // epoch ms
	Text      string          `json:"text,omitempty"`
	URN       string          `json:"urn,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// conversationEnt is the field subset read off a legacy Conversation entity. Participants
// and events arrive as normalized *reference lists resolved against included[].
type conversationEnt struct {
	EntityURN       string   `json:"entityUrn"`
	LastActivityAt  int64    `json:"lastActivityAt"`
	ParticipantRefs []string `json:"*participants"`
	EventRefs       []string `json:"*events"`
}

// eventEnt is the field subset read off a legacy Event entity. `from` is a *reference in
// the normalized envelope but arrives inline on some variants — both are tolerated.
type eventEnt struct {
	EntityURN    string                     `json:"entityUrn"`
	CreatedAt    int64                      `json:"createdAt"`
	FromRef      string                     `json:"*from"`
	FromInline   json.RawMessage            `json:"from"`
	EventContent map[string]json.RawMessage `json:"eventContent"`
}

// ParseConversations decodes a normalized legacy-inbox page into thin conversations sorted
// most-recent-first. A page whose paging says content exists but yields zero recognizable
// conversation entities returns ErrMessagingSchemaMoved, never a silent empty inbox.
func ParseConversations(raw json.RawMessage) ([]Conversation, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	index, _ := indexIncluded(env.Included)

	var out []Conversation
	for _, ent := range env.Included {
		var h entityHeader
		if json.Unmarshal(ent, &h) != nil || !typeContains(h.Type, TypeConversation) {
			continue
		}
		var c conversationEnt
		_ = json.Unmarshal(ent, &c)
		conv := Conversation{
			ID:             lastURNSegment(c.EntityURN),
			LastActivityAt: c.LastActivityAt,
			URN:            c.EntityURN,
			Raw:            ent,
		}
		for _, ref := range c.ParticipantRefs {
			if name := memberName(index, ref); name != "" {
				conv.Participants = append(conv.Participants, name)
			}
		}
		conv.Snippet = latestEventText(index, c.EventRefs)
		out = append(out, conv)
	}

	if len(out) == 0 {
		if total, _, _ := readPaging(unwrapData(env.Data)); total > 0 {
			return nil, ErrMessagingSchemaMoved
		}
		return out, nil
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActivityAt > out[j].LastActivityAt })
	return out, nil
}

// ParseEvents decodes a normalized conversation-events page into resolved messages sorted
// oldest→newest. Same drift rule as ParseConversations: positive paging with zero
// recognizable events is a moved schema, not an empty thread.
func ParseEvents(raw json.RawMessage) ([]Message, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	index, _ := indexIncluded(env.Included)

	var out []Message
	for _, ent := range env.Included {
		if msg, ok := messageFromEvent(ent, index); ok {
			out = append(out, msg)
		}
	}
	if len(out) == 0 {
		if total, _, _ := readPaging(unwrapData(env.Data)); total > 0 {
			return nil, ErrMessagingSchemaMoved
		}
		return out, nil
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

// messageFromEvent resolves one Event entity into a Message, or (…,false) for non-events.
func messageFromEvent(raw json.RawMessage, index map[string]json.RawMessage) (Message, bool) {
	var h entityHeader
	if json.Unmarshal(raw, &h) != nil || !typeContains(h.Type, TypeMessagingEvent) {
		return Message{}, false
	}
	var ev eventEnt
	_ = json.Unmarshal(raw, &ev)
	msg := Message{
		CreatedAt: ev.CreatedAt,
		Text:      eventMessageText(ev.EventContent),
		URN:       ev.EntityURN,
		Raw:       raw,
	}
	if ev.FromRef != "" {
		msg.Sender = memberName(index, ev.FromRef)
	}
	if msg.Sender == "" && len(ev.FromInline) > 0 {
		msg.Sender = nameFromInlineFrom(ev.FromInline, index)
	}
	return msg, true
}

// eventMessageText pulls the message text out of eventContent. The payload sits under a
// fully-qualified MessageEvent key (matched by CONTAINS, per the drift rule), with a
// flattened attributedBody variant tolerated as a fallback.
func eventMessageText(content map[string]json.RawMessage) string {
	for k, v := range content {
		if typeContains(k, TypeMessageEventContent) {
			return attributedText(v)
		}
	}
	if v, ok := content["attributedBody"]; ok {
		var t textOrStr
		_ = json.Unmarshal(v, &t)
		return t.Text
	}
	return ""
}

// attributedText reads a MessageEvent payload's text: attributedBody first (either the
// {"text":…} object or a bare string, via textOrStr), then the plain body fallback.
func attributedText(raw json.RawMessage) string {
	var m struct {
		AttributedBody textOrStr `json:"attributedBody"`
		Body           textOrStr `json:"body"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if m.AttributedBody.Text != "" {
		return m.AttributedBody.Text
	}
	return m.Body.Text
}

// latestEventText resolves a conversation's event references and returns the newest
// resolvable message text — the inbox snippet.
func latestEventText(index map[string]json.RawMessage, refs []string) string {
	best := Message{CreatedAt: -1}
	for _, ref := range refs {
		ent, ok := index[ref]
		if !ok {
			continue
		}
		if msg, ok := messageFromEvent(ent, index); ok && msg.Text != "" && msg.CreatedAt > best.CreatedAt {
			best = msg
		}
	}
	return best.Text
}

// memberName resolves a *participants / *from reference to a display name via the entity
// pool: member entity → miniProfile (inline or referenced) → firstName lastName.
func memberName(index map[string]json.RawMessage, urn string) string {
	ent, ok := index[urn]
	if !ok {
		return ""
	}
	return nameFromMemberJSON(ent, index)
}

// nameFromMemberJSON reads a display name from a MessagingMember-shaped blob. It tolerates
// the miniProfile arriving inline, as a *miniProfile reference, or the blob itself being a
// MiniProfile with firstName/lastName directly.
func nameFromMemberJSON(raw json.RawMessage, index map[string]json.RawMessage) string {
	var m struct {
		MiniProfileRef string          `json:"*miniProfile"`
		MiniProfile    json.RawMessage `json:"miniProfile"`
		FirstName      textOrStr       `json:"firstName"`
		LastName       textOrStr       `json:"lastName"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if name := joinName(m.FirstName.Text, m.LastName.Text); name != "" {
		return name
	}
	if len(m.MiniProfile) > 0 {
		if name := profileName(m.MiniProfile); name != "" {
			return name
		}
	}
	if m.MiniProfileRef != "" {
		if ent, ok := index[m.MiniProfileRef]; ok {
			return profileName(ent)
		}
	}
	return ""
}

// nameFromInlineFrom handles a non-normalized inline `from`: either a type-keyed wrapper
// ({"com.linkedin.…MessagingMember":{…}}) or the member object directly.
func nameFromInlineFrom(raw json.RawMessage, index map[string]json.RawMessage) string {
	var wrapper map[string]json.RawMessage
	if json.Unmarshal(raw, &wrapper) == nil {
		for k, v := range wrapper {
			if typeContains(k, TypeMessagingMember) {
				if name := nameFromMemberJSON(v, index); name != "" {
					return name
				}
			}
		}
	}
	return nameFromMemberJSON(raw, index)
}

// profileName reads firstName/lastName off a MiniProfile blob.
func profileName(raw json.RawMessage) string {
	var p struct {
		FirstName textOrStr `json:"firstName"`
		LastName  textOrStr `json:"lastName"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return joinName(p.FirstName.Text, p.LastName.Text)
}

func joinName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}
