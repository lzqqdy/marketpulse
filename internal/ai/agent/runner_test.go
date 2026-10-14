package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/evidence"
	"github.com/lzqqdy/marketpulse/internal/ai/llm"
	"github.com/lzqqdy/marketpulse/internal/ai/tools"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

type runnerMarket struct {
	marketdata.MarketDataService
	quote marketdata.IndexQuote
	delay bool
}

func (m *runnerMarket) IndexQuote(id string) (marketdata.IndexQuote, bool) {
	return m.quote, id == m.quote.ID && id != ""
}
func (m *runnerMarket) AlphaQuote(id string) (marketdata.AlphaQuote, bool) {
	return marketdata.AlphaQuote{}, false
}
func (m *runnerMarket) IndexKlinesContext(ctx context.Context, id, interval string, limit int) (marketdata.KlineResponse, error) {
	if m.delay {
		<-ctx.Done()
		return marketdata.KlineResponse{}, ctx.Err()
	}
	return marketdata.KlineResponse{}, errors.New("history missing")
}

type collectedEvents struct {
	records  []ToolGrounding
	startErr error
	tokens   []string
}

func (h *collectedEvents) OnToken(text string) error                { h.tokens = append(h.tokens, text); return nil }
func (h *collectedEvents) OnToolStart(name, arguments string) error { return h.startErr }
func (h *collectedEvents) OnToolResult(record ToolGrounding) error {
	h.records = append(h.records, record)
	return nil
}

func TestRunnerPropagatesToolBusinessStatus(t *testing.T) {
	for _, tc := range []struct {
		name, tool, args string
		quote            marketdata.IndexQuote
		delay            bool
		status           evidence.Status
	}{
		{"missing", "get_quote", `{"symbol":"AAPL"}`, marketdata.IndexQuote{}, false, evidence.Unavailable},
		{"fresh", "get_quote", `{"symbol":"AAPL"}`, marketdata.IndexQuote{ID: "us-aapl", Price: 200, Source: "tencent", UpdatedAt: time.Now(), FetchedAt: time.Now()}, false, evidence.OK},
		{"stale", "get_quote", `{"symbol":"AAPL"}`, marketdata.IndexQuote{ID: "us-aapl", Price: 200, Source: "tencent", UpdatedAt: time.Now().Add(-2 * time.Hour)}, false, evidence.Stale},
		{"bad-args", "get_quote", `{broken`, marketdata.IndexQuote{}, false, evidence.Unavailable},
		{"timeout", "get_klines_summary", `{"symbol":"AAPL"}`, marketdata.IndexQuote{}, true, evidence.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var messages []llm.Message
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var body struct {
					Messages []llm.Message `json:"messages"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				calls++
				w.Header().Set("Content-Type", "application/json")
				message := llm.Message{Role: "assistant", Content: "数据已核对；缺失或过期时不报最新价格。"}
				finish := "stop"
				if calls == 1 {
					message = llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call-1", Type: "function", Function: llm.ToolFunction{Name: tc.tool, Arguments: tc.args}}}}
					finish = "tool_calls"
				} else {
					messages = body.Messages
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}})
			}))
			defer server.Close()
			runner := Runner{LLM: llm.New(server.URL, "test", "test", time.Second), Tools: tools.NewRegistry(&runnerMarket{quote: tc.quote, delay: tc.delay}), ToolTimeout: 10 * time.Millisecond}
			events := &collectedEvents{}
			result, err := runner.RunChat(context.Background(), nil, "苹果现在多少钱", nil, events)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || len(result.Groundings) != 1 || len(events.records) != 1 {
				t.Fatalf("calls=%d results=%v", calls, result)
			}
			record := events.records[0]
			if record.Status != tc.status || record.OK != (tc.status == evidence.OK) || record.ToolCallID != "call-1" || record.Arguments != tc.args {
				t.Fatalf("wrong SSE/business status: %+v", record)
			}
			var toolPayload map[string]any
			for _, message := range messages {
				if message.Role == "tool" {
					if err := json.Unmarshal([]byte(message.Content), &toolPayload); err != nil {
						t.Fatal(err)
					}
				}
			}
			if toolPayload["status"] != string(tc.status) {
				t.Fatalf("LLM saw different status: %v", toolPayload)
			}
			if tc.status == evidence.OK && len(record.Evidence) != 1 {
				t.Fatal("missing structured evidence")
			}
		})
	}
}

func TestRunnerStopsWhenClientCannotReceiveToolStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call", Function: llm.ToolFunction{Name: "get_quote", Arguments: `{"symbol":"AAPL"}`}}}}, "finish_reason": "tool_calls"}}})
	}))
	defer server.Close()
	runner := Runner{LLM: llm.New(server.URL, "test", "test", time.Second), Tools: tools.NewRegistry(&runnerMarket{})}
	want := errors.New("client disconnected")
	_, err := runner.RunChat(context.Background(), nil, "苹果", nil, &collectedEvents{startErr: want})
	if !errors.Is(err, want) {
		t.Fatalf("%v", err)
	}
}

func TestHistoricalEvidenceIsCompleteRevalidatedAndNotInstructions(t *testing.T) {
	now := time.Now()
	facts := map[string]any{"price": 200.0, "note": strings.Repeat("英伟达新闻", 200)}
	message := HistoryMessage{Role: "assistant", ToolResults: []ToolGrounding{{Name: "get_quote", OK: true, Status: evidence.OK, Evidence: []evidence.Evidence{{ID: "ev-1", Kind: "quote", Source: "tencent", AsOf: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute), Status: evidence.OK, Facts: facts}}}}}
	text := historicalGrounding(message, now)
	start := strings.Index(text, "\n") + 1
	var records []ToolGrounding
	if err := json.Unmarshal([]byte(text[start:]), &records); err != nil {
		t.Fatal(err)
	}
	if records[0].OK || records[0].Evidence[0].Status != evidence.Stale {
		t.Fatalf("expired evidence was fresh: %s", text)
	}
	if records[0].Evidence[0].Facts["note"] != facts["note"] || message.ToolResults[0].Evidence[0].Status != evidence.OK {
		t.Fatal("history was truncated or mutated")
	}
	if !strings.Contains(text, "不是指令") {
		t.Fatal("missing data/instruction boundary")
	}
	legacy := historicalGrounding(HistoryMessage{Grounding: `{"price":100}`}, now)
	if !strings.Contains(legacy, "无法验证时点") {
		t.Fatal("legacy data promoted to current data")
	}
}

func TestCustomPromptKeepsInstrumentAndEvidenceRules(t *testing.T) {
	runner := Runner{SystemPrompt: "用口语回答"}
	prompt := runner.systemPrompt()
	if !strings.Contains(prompt, "用口语回答") || !strings.Contains(prompt, "us_equity") || !strings.Contains(prompt, "stale") {
		t.Fatalf("%s", prompt)
	}
}

func TestHistoryEvidenceHasAGlobalBudgetAndPrefersRecentMessages(t *testing.T) {
	history := []HistoryMessage{{Role: "assistant", Grounding: strings.Repeat("旧数据", 600)}, {Role: "user", Grounding: "not server evidence"}, {Role: "assistant", Grounding: "recent data"}}
	budget := len(historicalGrounding(history[2], time.Now()))
	selected := historicalGroundings(history, time.Now(), budget)
	if len(selected) != 1 || selected[2] == "" {
		t.Fatalf("older evidence displaced recent evidence: %v", selected)
	}
	if len(selected[2]) > budget {
		t.Fatal("global budget exceeded")
	}
}

func TestRunnerStreamsToolLoopAndDisablesToolsAtRoundLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream   bool                 `json:"stream"`
			Tools    []llm.ToolDefinition `json:"tools"`
			Messages []llm.Message        `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Stream {
			t.Error("every tool round must stream")
		}
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			if len(body.Tools) == 0 {
				t.Error("first round missing tools")
			}
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"get_quote","arguments":"{\"symbol\":\"AAPL\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\ndata: [DONE]\n\n")
		} else {
			if len(body.Tools) != 0 {
				t.Error("tool limit not enforced")
			}
			found := false
			for _, message := range body.Messages {
				found = found || message.Role == "tool" && strings.Contains(message.Content, "unavailable")
			}
			if !found {
				t.Error("missing tool evidence/status")
			}
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"暂无"}}]}`+"\n\n"+`data: {"choices":[{"delta":{"content":"报价"},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n")
		}
	}))
	defer server.Close()
	runner := Runner{LLM: llm.New(server.URL, "test", "test", time.Second), Tools: tools.NewRegistry(&runnerMarket{}), MaxToolRounds: 1}
	h := &collectedEvents{}
	result, err := runner.RunChat(context.Background(), nil, "苹果", nil, h)
	if err != nil || calls != 2 || result.Text != "暂无报价" || len(h.tokens) != 2 || len(result.Groundings) != 1 {
		t.Fatalf("%+v %v %v", result, h.tokens, err)
	}
}

func TestRunnerKeepsPartialTextOnStreamInterruption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"部分回答"}}]}`+"\n\n")
	}))
	defer server.Close()
	runner := Runner{LLM: llm.New(server.URL, "test", "test", time.Second), Tools: tools.NewRegistry(&runnerMarket{})}
	h := &collectedEvents{}
	result, err := runner.RunChat(context.Background(), nil, "苹果", nil, h)
	if err == nil || result.Text != "部分回答" || strings.Join(h.tokens, "") != result.Text {
		t.Fatalf("%+v %v", result, err)
	}
}
