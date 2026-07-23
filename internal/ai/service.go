package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lzqqdy/marketpulse/internal/ai/agent"
	"github.com/lzqqdy/marketpulse/internal/ai/llm"
	aimigrate "github.com/lzqqdy/marketpulse/internal/ai/migrate"
	"github.com/lzqqdy/marketpulse/internal/ai/tools"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
	"github.com/lzqqdy/marketpulse/internal/users"
)

const disclaimerSuffix = "以上内容仅供参考，不构成投资建议。"

// Service is the AI module facade.
type Service interface {
	Enabled() bool
	Chat(ctx context.Context, userID int64, req ChatRequest, emit func(StreamEvent) error) error
	ListConversations(ctx context.Context, userID int64, page, pageSize int) (ConversationListResponse, error)
	ListMessages(ctx context.Context, userID int64, conversationID string, limit int, includeTools bool) (MessagesResponse, error)
	DeleteConversation(ctx context.Context, userID int64, conversationID string) error
	UpdateConversationTitle(ctx context.Context, userID int64, conversationID, title string) (*Conversation, error)
}

// ConversationListResponse is GET /conversations payload.
type ConversationListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Items    []Conversation `json:"items"`
}

// MessagesResponse is GET /conversations/:id/messages payload.
type MessagesResponse struct {
	ConversationID string    `json:"conversationId"`
	Messages       []Message `json:"messages"`
}

type service struct {
	cfg    config.AiConfig
	repo   *repository
	runner *agent.Runner
	quota  *quotaStore
	busy   sync.Map // publicID -> struct{}
}

// BootstrapArgs bundles deps for the AI module.
type BootstrapArgs struct {
	AI         config.AiConfig
	DB         *sql.DB
	Redis      *platformredis.Client // optional; quota falls back to MySQL
	MarketData marketdata.MarketDataService
	Users      users.Service
}

// Bootstrap opens migrations and builds the AI service when enabled.
func Bootstrap(ctx context.Context, args BootstrapArgs) (Service, error) {
	cfg := args.AI
	if !cfg.Enabled {
		return &service{cfg: cfg}, nil
	}
	if args.DB == nil {
		return nil, fmt.Errorf("ai: mysql required when ai.enabled")
	}
	if args.Users == nil || !args.Users.Enabled() {
		return nil, fmt.Errorf("ai: users module required when ai.enabled")
	}
	if args.MarketData == nil {
		return nil, fmt.Errorf("ai: market data required when ai.enabled")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("ai: api_key required when ai.enabled")
	}
	if cfg.IsAutoMigrate() {
		if err := aimigrate.Run(ctx, args.DB); err != nil {
			return nil, err
		}
		slog.Info("ai migrations applied")
	}
	repo := newRepository(args.DB)
	llmClient := llm.New(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.Timeout)
	toolReg := tools.NewRegistry(args.MarketData)
	return &service{
		cfg:   cfg,
		repo:  repo,
		quota: newQuotaStore(args.Redis, repo, cfg.DailyQuotaPerUser),
		runner: &agent.Runner{
			LLM:            llmClient,
			Tools:          toolReg,
			SystemPrompt:   cfg.SystemPrompt,
			MaxToolRounds:  cfg.MaxToolRounds,
			MaxHistoryMsgs: cfg.MaxHistoryMessages,
			ToolTimeout:    15 * time.Second,
		},
	}, nil
}

func (s *service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.repo != nil && s.runner != nil
}

type streamHandler struct {
	emit func(StreamEvent) error
}

func (h streamHandler) OnToken(text string) error {
	return h.emit(StreamEvent{Event: "token", Data: map[string]string{"text": text}})
}

func (h streamHandler) OnToolStart(name, arguments string) error {
	return h.emit(StreamEvent{Event: "tool_start", Data: map[string]string{"name": name, "arguments": arguments}})
}

func (h streamHandler) OnToolResult(name string, ok bool, summary string) error {
	return h.emit(StreamEvent{Event: "tool_result", Data: map[string]any{"name": name, "ok": ok, "summary": summary}})
}

func (s *service) Chat(ctx context.Context, userID int64, req ChatRequest, emit func(StreamEvent) error) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return fmt.Errorf("%w: message required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(msg) > 4000 {
		return fmt.Errorf("%w: message too long", ErrInvalidInput)
	}

	if s.quota != nil {
		if err := s.quota.Take(ctx, userID); err != nil {
			return err
		}
	}

	var conv *Conversation
	var err error
	if strings.TrimSpace(req.ConversationID) == "" {
		title := msg
		if utf8.RuneCountInString(title) > 40 {
			title = string([]rune(title)[:40]) + "…"
		}
		conv, err = s.repo.CreateConversation(ctx, userID, title)
		if err != nil {
			return err
		}
	} else {
		conv, err = s.repo.GetConversationByPublicID(ctx, userID, req.ConversationID)
		if err != nil {
			return err
		}
	}

	if _, loaded := s.busy.LoadOrStore(conv.PublicID, struct{}{}); loaded {
		return ErrConversationBusy
	}
	defer s.busy.Delete(conv.PublicID)

	history, err := s.repo.ListMessages(ctx, conv.ID, s.cfg.MaxHistoryMessages)
	if err != nil {
		return err
	}
	hist := make([]agent.HistoryMessage, 0, len(history))
	for _, m := range history {
		hist = append(hist, agent.HistoryMessage{Role: m.Role, Content: m.Content})
	}

	var meta json.RawMessage
	if req.Context != nil {
		meta, _ = json.Marshal(map[string]any{"context": req.Context})
	}
	userMsgID, err := s.repo.AppendMessage(ctx, conv.ID, "user", msg, meta)
	if err != nil {
		return err
	}
	_ = s.repo.TouchConversation(ctx, conv.ID)

	if err := emit(StreamEvent{
		Event: "meta",
		Data: map[string]any{
			"conversationId": conv.PublicID,
			"messageId":      userMsgID,
		},
	}); err != nil {
		return err
	}

	var page *agent.PageContext
	if req.Context != nil {
		page = &agent.PageContext{
			FocusSymbol:    req.Context.FocusSymbol,
			AssetClass:     req.Context.AssetClass,
			Page:           req.Context.Page,
			VisibleSymbols: req.Context.VisibleSymbols,
		}
	}

	runCtx := ctx
	if s.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, s.cfg.Timeout)
		defer cancel()
	}

	text, runErr := s.runner.RunChat(runCtx, hist, msg, page, streamHandler{emit: emit})
	if runErr != nil {
		_ = emit(StreamEvent{Event: "error", Data: map[string]string{
			"code":    "ai_upstream",
			"message": runErr.Error(),
		}})
		text = ensureDisclaimer(text)
		if strings.TrimSpace(text) != "" {
			assistMeta, _ := json.Marshal(map[string]any{"incomplete": true, "model": s.cfg.Model})
			_, _ = s.repo.AppendMessage(ctx, conv.ID, "assistant", text, assistMeta)
		}
		return fmt.Errorf("%w: %v", ErrUpstream, runErr)
	}

	if extra := disclaimerDelta(text); extra != "" {
		_ = emit(StreamEvent{Event: "token", Data: map[string]string{"text": extra}})
		text += extra
	}

	assistMeta, _ := json.Marshal(map[string]any{"finishReason": "stop", "model": s.cfg.Model})
	_, err = s.repo.AppendMessage(ctx, conv.ID, "assistant", text, assistMeta)
	if err != nil {
		return err
	}
	_ = s.repo.TouchConversation(ctx, conv.ID)
	return emit(StreamEvent{Event: "done", Data: map[string]any{
		"finishReason":   "stop",
		"conversationId": conv.PublicID,
	}})
}

func ensureDisclaimer(text string) string {
	if extra := disclaimerDelta(text); extra != "" {
		return strings.TrimSpace(text) + extra
	}
	return strings.TrimSpace(text)
}

func disclaimerDelta(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return ""
	}
	if strings.Contains(t, "不构成投资建议") {
		return ""
	}
	return "\n\n" + disclaimerSuffix
}

func (s *service) ListConversations(ctx context.Context, userID int64, page, pageSize int) (ConversationListResponse, error) {
	if !s.Enabled() {
		return ConversationListResponse{}, ErrDisabled
	}
	res, err := s.repo.ListConversations(ctx, userID, page, pageSize)
	if err != nil {
		return ConversationListResponse{}, err
	}
	return ConversationListResponse{
		Total: res.Total, Page: res.Page, PageSize: res.PageSize, Items: res.Items,
	}, nil
}

func (s *service) ListMessages(ctx context.Context, userID int64, conversationID string, limit int, includeTools bool) (MessagesResponse, error) {
	if !s.Enabled() {
		return MessagesResponse{}, ErrDisabled
	}
	conv, err := s.repo.GetConversationByPublicID(ctx, userID, conversationID)
	if err != nil {
		return MessagesResponse{}, err
	}
	msgs, err := s.repo.ListVisibleMessages(ctx, conv.ID, limit, includeTools)
	if err != nil {
		return MessagesResponse{}, err
	}
	return MessagesResponse{ConversationID: conv.PublicID, Messages: msgs}, nil
}

func (s *service) DeleteConversation(ctx context.Context, userID int64, conversationID string) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	return s.repo.SoftDeleteConversation(ctx, userID, conversationID)
}

func (s *service) UpdateConversationTitle(ctx context.Context, userID int64, conversationID, title string) (*Conversation, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	return s.repo.UpdateTitle(ctx, userID, conversationID, title)
}
