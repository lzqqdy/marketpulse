package llm

import (
	"bufio"
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

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
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

// ChatStream streams assistant text tokens (no tools).
func (c *Client) ChatStream(ctx context.Context, messages []Message, onToken func(string) error) (string, error) {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
		Thinking: &thinkingCfg{Type: "disabled"},
	}
	httpReq, err := c.newRequest(ctx, reqBody)
	if err != nil {
		return "", err
	}
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm stream: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("llm stream http %d: %s", res.StatusCode, string(b))
	}
	reader := bufio.NewReader(res.Body)
	var full strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return full.String(), err
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		tok := chunk.Choices[0].Delta.Content
		if tok == "" {
			continue
		}
		full.WriteString(tok)
		if onToken != nil {
			if err := onToken(tok); err != nil {
				return full.String(), err
			}
		}
	}
	return full.String(), nil
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
