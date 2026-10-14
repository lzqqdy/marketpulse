package llm

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
)

func TestStreamTurnEmitsBeforeResponseCompletes(t *testing.T) {
	firstReceived := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Stream {
			t.Error("stream=true required")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"第一段\"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-firstReceived:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"第二段\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	var tokens []string
	msg, finish, err := New(server.URL, "test", "test", time.Second).StreamTurn(context.Background(), nil, nil, func(token string) error {
		tokens = append(tokens, token)
		if len(tokens) == 1 {
			close(firstReceived)
		}
		return nil
	})
	if err != nil || finish != "stop" || msg.Content != "第一段第二段" || len(tokens) != 2 {
		t.Fatalf("%+v %s %v tokens=%v", msg, finish, err, tokens)
	}
}

func TestStreamTurnAccumulatesToolCallFragmentsByIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"second","function":{"name":"get_quote","arguments":"{\"symbol\":\"ETH"}},{"index":0,"id":"first","function":{"name":"get_quote","arguments":"{\"symbol\":"}}]}}]}`+"\r\n\r\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"BTC\"}"}},{"index":1,"function":{"arguments":"\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	msg, _, err := New(server.URL, "test", "test", time.Second).StreamTurn(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 2 || msg.ToolCalls[0].ID != "first" || msg.ToolCalls[0].Function.Arguments != `{"symbol":"BTC"}` || msg.ToolCalls[1].Function.Arguments != `{"symbol":"ETH"}` {
		t.Fatalf("%+v", msg)
	}
}

func TestStreamTurnRejectsIncompleteAndInvalidStreams(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"eof", `data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n"},
		{"done-only", "data: [DONE]\n\n"},
		{"bad-json", "data: broken\n\n"},
		{"upstream-error", `data: {"error":{"message":"provider failed"}}` + "\n\n"},
		{"length", `data: {"choices":[{"delta":{"content":"partial"},"finish_reason":"length"}]}` + "\n\n"},
		{"missing-call-id", `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"get_quote"}}]},"finish_reason":"tool_calls"}]}` + "\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			msg, _, err := New(server.URL, "test", "test", time.Second).StreamTurn(context.Background(), nil, nil, nil)
			if err == nil {
				t.Fatal("incomplete stream accepted")
			}
			if strings.Contains(tc.body, "partial") && msg.Content != "partial" {
				t.Fatal("partial answer lost")
			}
		})
	}
}

func TestStreamTurnPropagatesConsumerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"partial"}}]}`+"\n\n")
	}))
	defer server.Close()
	want := errors.New("client disconnected")
	msg, _, err := New(server.URL, "test", "test", time.Second).StreamTurn(context.Background(), nil, nil, func(string) error { return want })
	if !errors.Is(err, want) || msg.Content != "partial" {
		t.Fatalf("%+v %v", msg, err)
	}
}

func TestStreamTurnCancelledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := New("http://127.0.0.1:1", "test", "test", time.Second).StreamTurn(ctx, nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
