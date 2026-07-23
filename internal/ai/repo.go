package ai

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type repository struct {
	db *sql.DB
}

func newRepository(db *sql.DB) *repository {
	return &repository{db: db}
}

func newPublicID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// UUID-ish without external dep
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32]), nil
}

func (r *repository) CreateConversation(ctx context.Context, userID int64, title string) (*Conversation, error) {
	publicID, err := newPublicID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	title = strings.TrimSpace(title)
	if title == "" {
		title = "新对话"
	}
	if len([]rune(title)) > 128 {
		title = string([]rune(title)[:128])
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO ai_conversations (public_id, user_id, title, status, created_at, updated_at)
VALUES (?, ?, ?, 'active', ?, ?)`, publicID, userID, title, now, now)
	if err != nil {
		return nil, fmt.Errorf("ai create conversation: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Conversation{
		ID:        id,
		PublicID:  publicID,
		UserID:    userID,
		Title:     title,
		Status:    "active",
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
	}, nil
}

func (r *repository) GetConversationByPublicID(ctx context.Context, userID int64, publicID string) (*Conversation, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, public_id, user_id, title, status, created_at, updated_at
FROM ai_conversations
WHERE public_id = ? AND user_id = ? AND deleted_at IS NULL`, publicID, userID)
	var c Conversation
	var created, updated time.Time
	if err := row.Scan(&c.ID, &c.PublicID, &c.UserID, &c.Title, &c.Status, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ai get conversation: %w", err)
	}
	c.CreatedAt = created.Format(time.RFC3339)
	c.UpdatedAt = updated.Format(time.RFC3339)
	return &c, nil
}

func (r *repository) TouchConversation(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE ai_conversations SET updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (r *repository) AppendMessage(ctx context.Context, conversationID int64, role, content string, metadata json.RawMessage) (int64, error) {
	var meta any
	if len(metadata) > 0 {
		meta = string(metadata)
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO ai_messages (conversation_id, role, content, metadata, created_at)
VALUES (?, ?, ?, ?, ?)`, conversationID, role, content, meta, time.Now())
	if err != nil {
		return 0, fmt.Errorf("ai append message: %w", err)
	}
	return res.LastInsertId()
}

func (r *repository) ListMessages(ctx context.Context, conversationID int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, conversation_id, role, content, metadata, created_at
FROM ai_messages
WHERE conversation_id = ?
ORDER BY id ASC
LIMIT ?`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("ai list messages: %w", err)
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var meta sql.NullString
		var created time.Time
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &meta, &created); err != nil {
			return nil, err
		}
		if meta.Valid && meta.String != "" {
			m.Metadata = json.RawMessage(meta.String)
		}
		m.CreatedAt = created.Format(time.RFC3339)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *repository) ListVisibleMessages(ctx context.Context, conversationID int64, limit int, includeTools bool) ([]Message, error) {
	msgs, err := r.ListMessages(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}
	if includeTools {
		return msgs, nil
	}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "assistant" {
			out = append(out, m)
		}
	}
	return out, nil
}

type conversationListResult struct {
	Total    int
	Page     int
	PageSize int
	Items    []Conversation
}

func (r *repository) ListConversations(ctx context.Context, userID int64, page, pageSize int) (conversationListResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM ai_conversations
WHERE user_id = ? AND deleted_at IS NULL`, userID).Scan(&total); err != nil {
		return conversationListResult{}, fmt.Errorf("ai count conversations: %w", err)
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx, `
SELECT id, public_id, user_id, title, status, created_at, updated_at
FROM ai_conversations
WHERE user_id = ? AND deleted_at IS NULL
ORDER BY updated_at DESC
LIMIT ? OFFSET ?`, userID, pageSize, offset)
	if err != nil {
		return conversationListResult{}, fmt.Errorf("ai list conversations: %w", err)
	}
	defer rows.Close()
	items := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		var created, updated time.Time
		if err := rows.Scan(&c.ID, &c.PublicID, &c.UserID, &c.Title, &c.Status, &created, &updated); err != nil {
			return conversationListResult{}, err
		}
		c.CreatedAt = created.Format(time.RFC3339)
		c.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, c)
	}
	return conversationListResult{Total: total, Page: page, PageSize: pageSize, Items: items}, rows.Err()
}

func (r *repository) SoftDeleteConversation(ctx context.Context, userID int64, publicID string) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE ai_conversations SET deleted_at = ?, updated_at = ?
WHERE public_id = ? AND user_id = ? AND deleted_at IS NULL`, time.Now(), time.Now(), publicID, userID)
	if err != nil {
		return fmt.Errorf("ai soft delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) UpdateTitle(ctx context.Context, userID int64, publicID, title string) (*Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("%w: title required", ErrInvalidInput)
	}
	if len([]rune(title)) > 128 {
		title = string([]rune(title)[:128])
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE ai_conversations SET title = ?, updated_at = ?
WHERE public_id = ? AND user_id = ? AND deleted_at IS NULL`, title, time.Now(), publicID, userID)
	if err != nil {
		return nil, fmt.Errorf("ai update title: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return r.GetConversationByPublicID(ctx, userID, publicID)
}

func (r *repository) IncrUsageDaily(ctx context.Context, userID int64, day string, limit int) (int, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO ai_usage_daily (user_id, day, chat_count) VALUES (?, ?, 1)
ON DUPLICATE KEY UPDATE chat_count = chat_count + 1`, userID, day)
	if err != nil {
		return 0, fmt.Errorf("ai incr usage: %w", err)
	}
	var count int
	if err := r.db.QueryRowContext(ctx, `
SELECT chat_count FROM ai_usage_daily WHERE user_id = ? AND day = ?`, userID, day).Scan(&count); err != nil {
		return 0, err
	}
	if limit > 0 && count > limit {
		_, _ = r.db.ExecContext(ctx, `
UPDATE ai_usage_daily SET chat_count = chat_count - 1 WHERE user_id = ? AND day = ? AND chat_count > 0`, userID, day)
		return count, ErrQuotaExceeded
	}
	return count, nil
}
