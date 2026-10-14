package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type streamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index int `json:"index"`
				ToolCall
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// StreamTurn accumulates fragmented tool arguments while forwarding text immediately.
func (c *Client) StreamTurn(ctx context.Context, messages []Message, tools []ToolDefinition, onToken func(string) error) (Message, string, error) {
	msg := Message{Role: "assistant"}
	finish := ""
	req, err := c.newRequest(ctx, chatRequest{Model: c.model, Messages: messages, Tools: tools, Stream: true, Thinking: &thinkingCfg{Type: "disabled"}})
	if err != nil {
		return msg, finish, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return msg, finish, fmt.Errorf("llm stream: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return msg, finish, fmt.Errorf("llm stream http %d: %s", res.StatusCode, b)
	}
	// Some compatible gateways ignore stream=true and return a complete JSON message.
	if strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		var response chatResponse
		if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&response); err != nil {
			return msg, finish, err
		}
		if response.Error != nil {
			return msg, finish, fmt.Errorf("llm api: %s", response.Error.Message)
		}
		if len(response.Choices) == 0 {
			return msg, finish, fmt.Errorf("llm api: empty choices")
		}
		msg, finish = response.Choices[0].Message, response.Choices[0].FinishReason
		if msg.Content != "" && onToken != nil {
			err = onToken(msg.Content)
		}
		if err == nil && finish != "stop" && finish != "tool_calls" {
			err = fmt.Errorf("llm incomplete answer: %s", finish)
		}
		return msg, finish, err
	}
	var text, reasoning strings.Builder
	calls := map[int]*ToolCall{}
	argumentBytes := 0
	result := func(err error) (Message, string, error) {
		msg.Content, msg.ReasoningContent = text.String(), reasoning.String()
		indices := make([]int, 0, len(calls))
		for index := range calls {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			msg.ToolCalls = append(msg.ToolCalls, *calls[index])
		}
		return msg, finish, err
	}
	consume := func(data string) (bool, error) {
		if data == "[DONE]" {
			return true, nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return false, fmt.Errorf("llm stream decode: %w", err)
		}
		if chunk.Error != nil {
			return false, fmt.Errorf("llm stream api: %s", chunk.Error.Message)
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.FinishReason != "" {
				finish = choice.FinishReason
			}
			if choice.Delta.Content != "" {
				text.WriteString(choice.Delta.Content)
				if onToken != nil {
					if err := onToken(choice.Delta.Content); err != nil {
						return false, err
					}
				}
			}
			reasoning.WriteString(choice.Delta.ReasoningContent)
			for _, delta := range choice.Delta.ToolCalls {
				if delta.Index < 0 || delta.Index >= 64 {
					return false, fmt.Errorf("llm stream: invalid tool index")
				}
				call := calls[delta.Index]
				if call == nil {
					call = &ToolCall{Type: "function"}
					calls[delta.Index] = call
				}
				if delta.ID != "" {
					call.ID = delta.ID
				}
				if delta.Type != "" {
					call.Type = delta.Type
				}
				call.Function.Name += delta.Function.Name
				call.Function.Arguments += delta.Function.Arguments
				argumentBytes += len(delta.Function.Arguments)
				if len(call.Function.Arguments) > 1<<20 || argumentBytes > 8<<20 || len(call.Function.Name) > 256 || len(call.ID) > 256 {
					return false, fmt.Errorf("llm stream: tool arguments too large")
				}
			}
		}
		if text.Len()+reasoning.Len() > 8<<20 {
			return false, fmt.Errorf("llm stream: response too large")
		}
		return false, nil
	}
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var data []string
	frameBytes := 0
	flush := func() (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		payload := strings.Join(data, "\n")
		data = nil
		frameBytes = 0
		return consume(payload)
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return result(err)
		}
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			done, err := flush()
			if err != nil {
				return result(err)
			}
			if done {
				break
			}
		} else if strings.HasPrefix(line, "data:") {
			frameBytes += len(line)
			if frameBytes > 2<<20 {
				return result(fmt.Errorf("llm stream: frame too large"))
			}
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return result(err)
	}
	if _, err := flush(); err != nil {
		return result(err)
	}
	if err := ctx.Err(); err != nil {
		return result(err)
	}
	if finish == "" {
		return result(fmt.Errorf("llm stream interrupted before finish_reason"))
	}
	if finish != "stop" && finish != "tool_calls" {
		return result(fmt.Errorf("llm incomplete answer: %s", finish))
	}
	for _, call := range calls {
		if call.ID == "" || call.Function.Name == "" {
			return result(fmt.Errorf("llm stream: incomplete tool call"))
		}
	}
	return result(nil)
}
