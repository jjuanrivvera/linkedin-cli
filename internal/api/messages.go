package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// defaultConversationsCount is the thread page size `messages read` requests (countBefore).
const defaultConversationsCount = 20

// mailboxURNPlaceholder stands in for the caller's real fsd_profile mailbox URN during a
// dry-run, where /me can't be resolved offline. It is kept human-readable in the previewed curl
// (a real mailbox URN is never equal to this sentinel, so non-dry-run behavior is unchanged).
const mailboxURNPlaceholder = "urn:li:fsd_profile:<ME>"

// ConversationsResult is the parsed inbox page plus the raw envelope (for -o json).
type ConversationsResult struct {
	Conversations []voyager.Conversation
	Raw           json.RawMessage
}

// ThreadResult is one parsed conversation thread plus the raw envelope.
type ThreadResult struct {
	Messages []voyager.Message
	Raw      json.RawMessage
}

// Test seams for the send POST's client-generated tokens: both MUST be injectable so a test
// asserts a deterministic body rather than raw randomness (mirrors the pacer/clock seams).
var (
	newOriginToken = randomUUIDv4
	newTrackingID  = randomTrackingID
)

// GetMailboxURN resolves (and caches for the process) the caller's fsd_profile mailbox URN via
// GET /me — the NEW dependency the GraphQL messenger calls need. It is a normal normalized
// Voyager GET (paced, GET-retryable). In dry-run getVoyager sends nothing, so callers use the
// placeholder instead of calling this.
func (c *Client) GetMailboxURN(ctx context.Context) (string, error) {
	c.mailboxMu.Lock()
	defer c.mailboxMu.Unlock()
	if c.mailboxURN != "" {
		return c.mailboxURN, nil
	}
	raw, err := c.getVoyager(ctx, voyager.PathMe, "")
	if err != nil {
		return "", err
	}
	if raw == nil { // dry-run
		return mailboxURNPlaceholder, nil
	}
	urn, err := voyager.MailboxURNFromMe(raw)
	if err != nil {
		return "", err
	}
	c.mailboxURN = urn
	return urn, nil
}

// mailboxURNForRequest returns the mailbox URN to build a request with: the offline placeholder
// in dry-run (no /me call), else the resolved-and-cached real URN.
func (c *Client) mailboxURNForRequest(ctx context.Context) (string, error) {
	if c.DryRun {
		return mailboxURNPlaceholder, nil
	}
	return c.GetMailboxURN(ctx)
}

// ListConversations fetches the caller's most-recent conversations via the GraphQL messenger
// surface (queryId=ListQueryID, variables=(mailboxUrn:…)). The legacy keyVersion=LEGACY_INBOX
// endpoint is dead (500s live 2026-07-22); see DECISIONS.md #27.
func (c *Client) ListConversations(ctx context.Context, _ int) (*ConversationsResult, error) {
	mailbox, err := c.mailboxURNForRequest(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := c.getVoyagerGraphQL(ctx, voyager.PathMessengerGraphQL, buildConversationsQuery(mailbox))
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	convs, err := voyager.ParseConversations(raw)
	if err != nil {
		return nil, err
	}
	return &ConversationsResult{Conversations: convs, Raw: raw}, nil
}

// GetConversationEvents fetches one conversation's message thread (resolved oldest→newest) via
// the GraphQL messenger surface (queryId=MessagesQueryID). conversationID is the full
// msg_conversation URN that `messages list` prints (a bare id is prefixed defensively). now is
// injected for deterministic tests — it forms the deliveredAt anchor timestamp.
func (c *Client) GetConversationEvents(ctx context.Context, conversationID string, now time.Time) (*ThreadResult, error) {
	convURN := voyager.EnsureConversationURN(conversationID)
	q := buildMessagesQuery(convURN, defaultConversationsCount, now.UnixMilli())
	raw, err := c.getVoyagerGraphQL(ctx, voyager.PathMessengerGraphQL, q)
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	msgs, err := voyager.ParseMessages(raw)
	if err != nil {
		return nil, err
	}
	return &ThreadResult{Messages: msgs, Raw: raw}, nil
}

// SendMessage posts one text message into an existing conversation via the Dash messenger
// endpoint (POST ?action=createMessage, Content-Type text/plain). It CHARGES the daily
// message-send cap first (ban-safety): when today's budget is spent it refuses rather than
// sending. The POST is never retried (retry.go is idempotent-only). Expects 200/201. now is
// injected for deterministic tests. See DECISIONS.md #29.
func (c *Client) SendMessage(ctx context.Context, conversationID, text string, now time.Time) (json.RawMessage, error) {
	if !c.DryRun && c.pacer != nil {
		if err := c.pacer.ChargeDailySend(now); err != nil {
			return nil, err
		}
	}
	mailbox, err := c.mailboxURNForRequest(ctx)
	if err != nil {
		return nil, err
	}
	convURN := voyager.EnsureConversationURN(conversationID)
	// SetEscapeHTML(false): a message body (or the dry-run mailbox placeholder) may contain
	// <, >, & — LinkedIn decodes JSON either way, and the un-escaped form keeps the previewed
	// --data curl readable.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(messageCreateBody(text, convURN, mailbox)); err != nil {
		return nil, err
	}
	body := bytes.TrimRight(buf.Bytes(), "\n")
	rawQuery := "action=" + voyager.ActionCreateMessage
	return c.postVoyager(ctx, voyager.PathMessengerDashSend, rawQuery, voyager.SendContentType, body)
}

// messageCreateBody builds the Dash createMessage envelope LinkedIn's messenger send endpoint
// expects (DECISIONS.md #29). originToken (uuid v4) and trackingId (16-char) come from
// injectable seams so tests assert a deterministic body.
func messageCreateBody(text, conversationURN, mailboxURN string) map[string]any {
	return map[string]any{
		"message": map[string]any{
			"body":            map[string]any{"text": text},
			"conversationUrn": conversationURN,
			"originToken":     newOriginToken(),
		},
		"mailboxUrn":                   mailboxURN,
		"trackingId":                   newTrackingID(),
		"dedupeByClientGeneratedToken": true,
	}
}

// buildConversationsQuery assembles the conversations-list GraphQL query string BY HAND: the
// queryId and the variables=(...) blob keep their structural chars (`(),:`) literal, only the
// mailbox URN VALUE is URL-escaped (its own colons become %3A). Same discipline as the
// job-search query blob — url.Values would corrupt the structural chars.
func buildConversationsQuery(mailboxURN string) string {
	return "queryId=" + voyager.ListQueryID + "&variables=" +
		buildVariablesBlob([][2]string{{voyager.VarMailboxURN, mailboxURN}})
}

// buildMessagesQuery assembles the thread-read GraphQL query string BY HAND (see
// buildConversationsQuery). nowMillis is the deliveredAt anchor; countBefore bounds how many
// messages back to fetch.
func buildMessagesQuery(conversationURN string, count int, nowMillis int64) string {
	return "queryId=" + voyager.MessagesQueryID + "&variables=" +
		buildVariablesBlob([][2]string{
			{voyager.VarConversationURN, conversationURN},
			{voyager.VarCountBefore, strconv.Itoa(count)},
			{voyager.VarCountAfter, "0"},
			{voyager.VarDeliveredAt, strconv.FormatInt(nowMillis, 10)},
		})
}

// randomUUIDv4 generates an RFC-4122 v4 UUID for the send originToken (crypto/rand; no external
// uuid dependency). Overridden by a seam in tests.
func randomUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// randomTrackingID generates a 16-character client tracking token for the send body. Overridden
// by a seam in tests.
func randomTrackingID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	out := make([]byte, 16)
	for i, v := range raw {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out)
}
