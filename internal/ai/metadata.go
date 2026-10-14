package ai

import (
	"encoding/json"

	"github.com/lzqqdy/marketpulse/internal/ai/agent"
)

type assistantMessageMetadata struct {
	FinishReason string                `json:"finishReason,omitempty"`
	Model        string                `json:"model,omitempty"`
	Incomplete   bool                  `json:"incomplete,omitempty"`
	Grounding    string                `json:"grounding,omitempty"`
	ToolResults  []agent.ToolGrounding `json:"toolResults,omitempty"`
}

func assistantMetadata(result agent.RunResult, model string, incomplete bool) (json.RawMessage, error) {
	meta := assistantMessageMetadata{Model: model, Incomplete: incomplete,
		Grounding: compactGrounding(result.Groundings), ToolResults: result.Groundings}
	if !incomplete {
		meta.FinishReason = "stop"
	}
	return json.Marshal(meta)
}

func historyMessage(message Message) agent.HistoryMessage {
	out := agent.HistoryMessage{Role: message.Role, Content: message.Content}
	if message.Role != "assistant" {
		return out
	}
	var meta assistantMessageMetadata
	if json.Unmarshal(message.Metadata, &meta) == nil {
		out.Grounding, out.ToolResults = meta.Grounding, meta.ToolResults
	}
	return out
}
