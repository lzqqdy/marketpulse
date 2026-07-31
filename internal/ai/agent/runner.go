package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/llm"
	"github.com/lzqqdy/marketpulse/internal/ai/tools"
)

const defaultSystemPrompt = `你是 MarketPulse 看板里的行情助手「观澜」。用中文简洁回答，像资深交易员口头点评，不要客服腔。

## 硬性规则
1. 价格、涨跌幅、家数、市值、费率、均线位置、量能比等数字必须来自工具或 page_context 盘面摘要；没有数据就说「暂无数据」，禁止编造。
2. 先给结论（1–2 句），再列关键数据与时间；需要时补一句对比或风险提示。
3. 用户说「这个/它/刚才那个」时：优先用 page_context.focusSymbol；用户显式写出的标的优先于焦点。
4. 追问时优先复用 prior_data_grounding 里的数字做对比，缺新数据再调工具。
5. page_context 里若已有「看板实时盘面摘要」，谈整体盘面时可直接引用，不必再调 get_snapshot_summary（除非用户要更细涨跌榜）。
6. 不做下单/改仓承诺；不保证收益。整段回复末尾只加一次：「以上内容仅供参考，不构成投资建议。」

## 工具选用（尽量少轮、一次问清）
- 单标「怎么样/怎么看/现在多少」→ 优先 get_symbol_brief。
- 只要精确报价 → get_quote。
- 只要走势/均线/量能 → get_klines_summary。
- 「BTC和ETH比/谁更强/相对强弱」→ compare_symbols（一次对比 2–5 个）。
- 「大盘/币圈整体/涨跌榜/情绪」→ 先看 page_context 摘要；不够再 get_snapshot_summary。
- 「A股/港股/美股涨跌家数、热门板块」→ get_market_breadth。
- 「有什么新闻/快讯」→ get_express_news（币圈用 tag=币圈，A股用 tag=A股）。

## 回答风格
- 避免只念一个价格；至少带上涨跌与数据时间。
- 提到走势时可用工具里的 closeVsSma7/30、volumeNote、nearRangeHigh/Low，但不要编造未给出的指标。
- 有快讯时挑 1 条最相关的一句话带过，不要堆标题列表。
- 对比多个标的时用表格感短句（谁涨更多、谁更贴近均线上方）。`

// HistoryMessage is a prior chat turn loaded from storage.
type HistoryMessage struct {
	Role      string
	Content   string
	Grounding string // compact prior tool summaries for follow-ups
}

// PageContext is optional dashboard focus for the current turn.
type PageContext struct {
	FocusSymbol     string   `json:"focusSymbol,omitempty"`
	AssetClass      string   `json:"assetClass,omitempty"`
	Page            string   `json:"page,omitempty"`
	VisibleSymbols  []string `json:"visibleSymbols,omitempty"`
	FearGreedValue  int      `json:"fearGreedValue,omitempty"`
	FearGreedLabel  string   `json:"fearGreedLabel,omitempty"`
	BtcDominancePct float64  `json:"btcDominancePct,omitempty"`
	UsdtCny         float64  `json:"usdtCny,omitempty"`
	BoardBriefing   string   `json:"boardBriefing,omitempty"` // server-filled live snapshot text
}

// ToolGrounding is one tool result kept for persistence / follow-up.
type ToolGrounding struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
}

// RunResult is the agent output for one user turn.
type RunResult struct {
	Text       string
	Groundings []ToolGrounding
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
func (r *Runner) RunChat(ctx context.Context, history []HistoryMessage, userText string, page *PageContext, h EventHandler) (RunResult, error) {
	out := RunResult{}
	if r.LLM == nil || r.Tools == nil {
		return out, fmt.Errorf("agent not configured")
	}
	maxRounds := r.MaxToolRounds
	if maxRounds <= 0 {
		maxRounds = 6
	}
	msgs := []llm.Message{{Role: "system", Content: r.systemPrompt()}}
	for _, m := range truncateHistory(history, r.MaxHistoryMsgs) {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
		if m.Role == "assistant" && strings.TrimSpace(m.Grounding) != "" {
			msgs = append(msgs, llm.Message{
				Role: "system",
				Content: "prior_data_grounding（上一轮工具摘要，供指代消解与对比；勿整段复述 JSON）：\n" +
					trimRunes(m.Grounding, 1800),
			})
		}
	}
	userContent := userText
	if briefing := formatPageBriefing(page); briefing != "" {
		userContent = userText + "\n\n[page_context]\n" + briefing + "\n[/page_context]"
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: userContent})

	toolDefs := make([]llm.ToolDefinition, 0, len(r.Tools.Definitions()))
	for _, d := range r.Tools.Definitions() {
		toolDefs = append(toolDefs, llm.ToolDefinition{Type: d.Type, Function: d.Function})
	}

	for round := 0; round < maxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		msg, finish, err := r.LLM.Chat(ctx, msgs, toolDefs)
		if err != nil {
			return out, err
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
				summary := result
				if len(summary) > 280 {
					summary = summary[:280] + "…"
				}
				out.Groundings = append(out.Groundings, ToolGrounding{Name: name, OK: ok, Summary: summary})
				if h != nil {
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
					out.Text = text
					return out, err
				}
			}
			out.Text = text
			return out, nil
		}
		text, err := r.LLM.ChatStream(ctx, msgs, func(tok string) error {
			if h == nil {
				return nil
			}
			return h.OnToken(tok)
		})
		out.Text = text
		return out, err
	}
	return out, fmt.Errorf("max tool rounds exceeded")
}

func formatPageBriefing(page *PageContext) string {
	if page == nil {
		return ""
	}
	var b strings.Builder
	if page.FocusSymbol != "" {
		fmt.Fprintf(&b, "- 当前焦点标的: %s", page.FocusSymbol)
		if page.AssetClass != "" {
			fmt.Fprintf(&b, " (%s)", page.AssetClass)
		}
		b.WriteByte('\n')
	}
	if page.Page != "" {
		fmt.Fprintf(&b, "- 页面: %s\n", page.Page)
	}
	if len(page.VisibleSymbols) > 0 {
		fmt.Fprintf(&b, "- 看板可见: %s\n", strings.Join(page.VisibleSymbols, ", "))
	}
	if page.FearGreedValue > 0 || page.FearGreedLabel != "" {
		fmt.Fprintf(&b, "- 情绪: %d %s\n", page.FearGreedValue, page.FearGreedLabel)
	}
	if page.BtcDominancePct > 0 {
		fmt.Fprintf(&b, "- BTC占比: %.1f%%\n", page.BtcDominancePct)
	}
	if page.UsdtCny > 0 {
		fmt.Fprintf(&b, "- USDT/CNY: %.2f\n", page.UsdtCny)
	}
	if strings.TrimSpace(page.BoardBriefing) != "" {
		b.WriteString(page.BoardBriefing)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
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

func trimRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
