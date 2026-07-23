package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/llm"
	"github.com/lzqqdy/marketpulse/internal/ai/tools"
)

const defaultSystemPrompt = `你是 MarketPulse 的行情分析助手。规则：
1. 只能依据工具返回的数据给出价格、涨跌幅、盘面数字；没有工具数据时明确说「暂无数据」，禁止编造。
2. 用简洁中文回答；先结论后依据；必要时说明数据时间。
3. 不提供投资建议保证收益，不做下单/改仓等操作指引以外的执行承诺。
4. 若用户说「这个/它」且上下文有 focusSymbol，按该标的理解；用户显式写出的标的优先。
5. 回复末尾加一句：以上内容仅供参考，不构成投资建议。`

// HistoryMessage is a prior chat turn loaded from storage.
type HistoryMessage struct {
	Role    string
	Content string
}

// PageContext is optional dashboard focus for the current turn.
type PageContext struct {
	FocusSymbol    string   `json:"focusSymbol,omitempty"`
	AssetClass     string   `json:"assetClass,omitempty"`
	Page           string   `json:"page,omitempty"`
	VisibleSymbols []string `json:"visibleSymbols,omitempty"`
}

// EventHandler receives progress while the agent runs.
type EventHandler interface {
	OnToken(text string) error
	OnToolStart(name, arguments string) error
	OnToolResult(name string, ok bool, summary string) error
}

// Runner executes the tool-calling loop then streams the final answer.
type Runner struct {
	LLM            *llm.Client
	Tools          *tools.Registry
	SystemPrompt   string
	MaxToolRounds  int
	MaxHistoryMsgs int
	ToolTimeout    time.Duration
}

func (r *Runner) systemPrompt() string {
	if strings.TrimSpace(r.SystemPrompt) != "" {
		return r.SystemPrompt
	}
	return defaultSystemPrompt
}

// RunChat builds messages from history + user turn and streams the assistant reply.
func (r *Runner) RunChat(ctx context.Context, history []HistoryMessage, userText string, page *PageContext, h EventHandler) (string, error) {
	if r.LLM == nil || r.Tools == nil {
		return "", fmt.Errorf("agent not configured")
	}
	maxRounds := r.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = 6
	}
	msgs := []llm.Message{{Role: "system", Content: r.systemPrompt()}}
	for _, m := range truncateHistory(history, r.MaxHistoryMsgs) {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
		}
	}
	userContent := userText
	if page != nil && page.FocusSymbol != "" {
		b, _ := json.Marshal(page)
		userContent = userText + "\n\n[page_context]" + string(b) + "[/page_context]"
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: userContent})

	toolDefs := make([]llm.ToolDefinition, 0, len(r.Tools.Definitions()))
	for _, d := range r.Tools.Definitions() {
		toolDefs = append(toolDefs, llm.ToolDefinition{Type: d.Type, Function: d.Function})
	}

	for round := 0; round < maxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		msg, finish, err := r.LLM.Chat(ctx, msgs, toolDefs)
		if err != nil {
			return "", err
		}
		if len(msg.ToolCalls) > 0 {
			msgs = append(msgs, llm.Message{
				Role:      "assistant",
				Content:   msg.Content,
				ToolCalls: msg.ToolCalls,
			})
			for _, tc := range msg.ToolCalls {
				name := tc.Function.Name
				args := tc.Function.Arguments
				if h != nil {
					_ = h.OnToolStart(name, args)
				}
				toolCtx := ctx
				var cancel context.CancelFunc
				if r.ToolTimeout > 0 {
					toolCtx, cancel = context.WithTimeout(ctx, r.ToolTimeout)
				}
				result, execErr := r.Tools.Execute(toolCtx, name, args)
				if cancel != nil {
					cancel()
				}
				ok := execErr == nil
				if execErr != nil {
					result = fmt.Sprintf(`{"ok":false,"error":%q}`, execErr.Error())
				}
				if h != nil {
					summary := result
					if len(summary) > 240 {
						summary = summary[:240] + "…"
					}
					_ = h.OnToolResult(name, ok, summary)
				}
				msgs = append(msgs, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
				})
			}
			continue
		}
		if strings.TrimSpace(msg.Content) != "" && finish != "tool_calls" {
			text := msg.Content
			if h != nil {
				if err := h.OnToken(text); err != nil {
					return text, err
				}
			}
			return text, nil
		}
		text, err := r.LLM.ChatStream(ctx, msgs, func(tok string) error {
			if h == nil {
				return nil
			}
			return h.OnToken(tok)
		})
		return text, err
	}
	return "", fmt.Errorf("max tool rounds exceeded")
}

func truncateHistory(history []HistoryMessage, max int) []HistoryMessage {
	if max <= 0 {
		max = 40
	}
	if len(history) <= max {
		return history
	}
	return history[len(history)-max:]
}
