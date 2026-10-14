package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an OpenAI-compatible Chat Completions API (DeepSeek).
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func New(baseURL, apiKey, model string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string `json:"type"`
	Function any    `json:"function"`
}

type chatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Thinking *thinkingCfg     `json:"thinking,omitempty"`
}

type thinkingCfg struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat completes one non-streaming turn (used for tool loops).
func (c *Client) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, string, error) {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
		Thinking: &thinkingCfg{Type: "disabled"},
	}
	raw, err := c.doJSON(ctx, reqBody)
	if err != nil {
		return Message{}, "", err
	}
	var resp chatResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Message{}, "", fmt.Errorf("llm decode: %w", err)
	}
	if resp.Error != nil {
		return Message{}, "", fmt.Errorf("llm api: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return Message{}, "", fmt.Errorf("llm api: empty choices")
	}
	return resp.Choices[0].Message, resp.Choices[0].FinishReason, nil
}

// ChatStream streams an assistant answer without tools.
func (c *Client) ChatStream(ctx context.Context, messages []Message, onToken func(string) error) (string, error) {
	msg, _, err := c.StreamTurn(ctx, messages, nil, onToken)
	return msg.Content, err
}

func (c *Client) doJSON(ctx context.Context, body chatRequest) ([]byte, error) {
	httpReq, err := c.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("llm http %d: %s", res.StatusCode, string(raw))
	}
	return raw, nil
}

func (c *Client) newRequest(ctx context.Context, body chatRequest) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	return req, nil
}
