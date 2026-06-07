package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// toolServer simulates an Ollama /api/chat endpoint that:
//  1. On the first call, returns a tool_call for "write_file"
//  2. On the second call, returns the final text answer
func toolServer(t *testing.T) *httptest.Server {
	t.Helper()
	call := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		call++
		switch call {
		case 1:
			// First call → tool invocation
			resp := ChatResponseWithTools{
				Model: "test",
				Message: ChatMessageWithTools{
					Role:    "assistant",
					Content: "",
					ToolCalls: []ToolCall{{
						Function: ToolCallFunction{
							Name:      "write_file",
							Arguments: map[string]any{"path": "out.txt", "content": "hello"},
						},
					}},
				},
				Done: true,
			}
			json.NewEncoder(w).Encode(resp)
		default:
			// Second call → final answer
			resp := ChatResponseWithTools{
				Model: "test",
				Message: ChatMessageWithTools{
					Role:    "assistant",
					Content: "done writing files",
				},
				Done:            true,
				PromptEvalCount: 5,
				EvalCount:       3,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

func TestChatWithTools_ExecutesToolAndReturnsText(t *testing.T) {
	srv := toolServer(t)
	defer srv.Close()

	client := NewGenerateClient(srv.URL)
	cwt, ok := client.(GenerateClientWithTools)
	if !ok {
		t.Fatal("NewGenerateClient does not implement GenerateClientWithTools")
	}

	var toolCalled bool
	executor := func(name string, args map[string]any) string {
		if name == "write_file" {
			toolCalled = true
			return "wrote out.txt"
		}
		return "unknown"
	}

	var chunks []string
	msgs := []ChatMessageWithTools{
		{Role: "user", Content: "implement task"},
	}
	ctx := WithModel(context.Background(), "test")
	out, usage, err := cwt.ChatWithTools(ctx, msgs, nil, executor, func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatalf("ChatWithTools error: %v", err)
	}
	if !toolCalled {
		t.Fatal("executor was not called")
	}
	if out != "done writing files" {
		t.Fatalf("output = %q", out)
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 3 {
		t.Fatalf("usage = %+v", usage)
	}

	// onChunk should have received the tool notification + final text
	full := strings.Join(chunks, "")
	if !strings.Contains(full, "write_file") {
		t.Errorf("expected tool name in chunks: %q", full)
	}
	if !strings.Contains(full, "done writing files") {
		t.Errorf("expected final text in chunks: %q", full)
	}
}

func TestChatWithTools_CompileTimeInterface(t *testing.T) {
	// compile-time check via var _ is in tools.go; this confirms at runtime
	client := NewGenerateClient("http://127.0.0.1:1")
	if _, ok := client.(GenerateClientWithTools); !ok {
		t.Fatal("genClient does not implement GenerateClientWithTools")
	}
}
