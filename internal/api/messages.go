package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/jjuanrivvera/linkedin-cli/internal/voyager"
)

// defaultConversationsCount is the inbox page size `messages list` requests.
const defaultConversationsCount = 20

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

// ListConversations fetches the user's most-recent conversations via the legacy inbox
// projection (keyVersion=LEGACY_INBOX — the community-proven surface, DECISIONS.md #23).
func (c *Client) ListConversations(ctx context.Context, count int) (*ConversationsResult, error) {
	if count <= 0 {
		count = defaultConversationsCount
	}
	q := url.Values{}
	q.Set(voyager.KeyVersionParam, voyager.KeyVersionLegacyInbox)
	q.Set("count", strconv.Itoa(count))
	raw, err := c.getVoyager(ctx, voyager.PathConversations, q.Encode())
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

// GetConversationEvents fetches one conversation's message thread (resolved oldest→newest).
func (c *Client) GetConversationEvents(ctx context.Context, conversationID string) (*ThreadResult, error) {
	path := voyager.PathConversations + "/" + url.PathEscape(conversationID) + "/events"
	raw, err := c.getVoyager(ctx, path, "")
	if err != nil {
		return nil, err
	}
	if raw == nil { // dry-run
		return nil, nil
	}
	msgs, err := voyager.ParseEvents(raw)
	if err != nil {
		return nil, err
	}
	return &ThreadResult{Messages: msgs, Raw: raw}, nil
}

// SendMessage posts one text message into an existing conversation. It CHARGES the daily
// message-send cap first (ban-safety: automated messaging is the classic account-restriction
// trigger): when today's budget is spent it refuses rather than sending. The POST is never
// retried (retry.go is idempotent-only). Returns the raw response body (LinkedIn answers 201
// with the created event). now is injected for deterministic tests.
func (c *Client) SendMessage(ctx context.Context, conversationID, text string, now time.Time) (json.RawMessage, error) {
	if !c.DryRun && c.pacer != nil {
		if err := c.pacer.ChargeDailySend(now); err != nil {
			return nil, err
		}
	}
	body, err := json.Marshal(messageCreateBody(text))
	if err != nil {
		return nil, err
	}
	path := voyager.PathConversations + "/" + url.PathEscape(conversationID) + "/events"
	return c.postVoyager(ctx, path, voyager.ActionParam+"="+voyager.ActionCreate, body)
}

// messageCreateBody builds the legacy MessageCreate envelope LinkedIn's send endpoint
// expects (DECISIONS.md #24). The type key is the drift-prone part and lives in schema.go.
func messageCreateBody(text string) map[string]any {
	return map[string]any{
		"eventCreate": map[string]any{
			"value": map[string]any{
				voyager.KeyMessageCreate: map[string]any{
					"attributedBody": map[string]any{
						"text":       text,
						"attributes": []any{},
					},
					"attachments": []any{},
				},
			},
		},
	}
}
