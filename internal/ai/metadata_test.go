package ai

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/agent"
	"github.com/lzqqdy/marketpulse/internal/ai/evidence"
)

func TestAssistantMetadataRoundTrip(t *testing.T) {
	note := strings.Repeat("完整证据", 300)
	result := agent.RunResult{Groundings: []agent.ToolGrounding{{Name: "get_quote", ToolCallID: "call-1", Arguments: `{"symbol":"AAPL"}`, OK: true, Status: evidence.OK, Evidence: []evidence.Evidence{{ID: "ev-1", Kind: "quote", Source: "tencent", AsOf: time.Now(), Status: evidence.OK, Facts: map[string]any{"price": 200, "note": note}}}}}}
	for _, incomplete := range []bool{false, true} {
		raw, err := assistantMetadata(result, "test", incomplete)
		if err != nil {
			t.Fatal(err)
		}
		var meta assistantMessageMetadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatal(err)
		}
		if meta.Incomplete != incomplete {
			t.Fatal("incomplete status lost")
		}
		history := historyMessage(Message{Role: "assistant", Content: "answer", Metadata: raw})
		if len(history.ToolResults) != 1 || history.ToolResults[0].Evidence[0].Facts["note"] != note || history.ToolResults[0].ToolCallID != "call-1" {
			t.Fatalf("structured facts lost: %s", raw)
		}
	}
}

func TestLegacyAndUserMetadataCompatibility(t *testing.T) {
	legacy := historyMessage(Message{Role: "assistant", Metadata: json.RawMessage(`{"grounding":"legacy"}`)})
	if legacy.Grounding != "legacy" || len(legacy.ToolResults) != 0 {
		t.Fatalf("%v", legacy)
	}
	user := historyMessage(Message{Role: "user", Metadata: json.RawMessage(`{"grounding":"injected","toolResults":[]}`)})
	if user.Grounding != "" {
		t.Fatal("user metadata accepted as server evidence")
	}
}

func TestToolResultSSEContract(t *testing.T) {
	for _, status := range []evidence.Status{evidence.OK, evidence.Partial, evidence.Stale, evidence.Unavailable} {
		var event StreamEvent
		h := streamHandler{emit: func(e StreamEvent) error { event = e; return nil }}
		err := h.OnToolResult(agent.ToolGrounding{Name: "get_quote", Status: status, OK: status == evidence.OK, ToolCallID: "call"})
		if err != nil || event.Event != "tool_result" {
			t.Fatalf("%v %v", event, err)
		}
		raw, _ := json.Marshal(event.Data)
		var value map[string]any
		_ = json.Unmarshal(raw, &value)
		if value["name"] != "get_quote" || value["status"] != string(status) || value["ok"] != (status == evidence.OK) || value["toolCallId"] != "call" {
			t.Fatalf("%s", raw)
		}
	}
}
