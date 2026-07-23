package ai

import "encoding/json"

// PageContext is optional dashboard focus sent with each chat turn.
type PageContext struct {
	FocusSymbol     string   `json:"focusSymbol,omitempty"`
	AssetClass      string   `json:"assetClass,omitempty"`
	Page            string   `json:"page,omitempty"`
	VisibleSymbols  []string `json:"visibleSymbols,omitempty"`
}

// ChatRequest is the POST /api/v1/ai/chat body.
type ChatRequest struct {
	ConversationID string       `json:"conversationId"`
	Message        string       `json:"message"`
	Context        *PageContext `json:"context,omitempty"`
}

// Conversation is a chat thread owned by a user.
type Conversation struct {
	ID        int64  `json:"-"`
	PublicID  string `json:"conversationId"`
	UserID    int64  `json:"-"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Message is one persisted chat message.
type Message struct {
	ID             int64           `json:"id"`
	ConversationID int64           `json:"-"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

// StreamEvent is one SSE payload (JSON in data field).
type StreamEvent struct {
	Event string
	Data  any
}
