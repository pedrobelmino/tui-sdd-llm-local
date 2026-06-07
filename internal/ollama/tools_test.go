package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// toolStreamServer simulates /api/chat with text-based tool calling:
//  1. First call returns a <tool_call> block for "write_file"
//  2. Second call returns a plain final answer "done writing files"
//
// Note: ChatWithTools now pre-loads list_dir before the first model turn,
// so the executor will also be called for list_dir. The server only sees
// /api/chat calls — list_dir is executed before any HTTP request.
func toolStreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	call := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		call++

		writeChunks := func(content string, promptTok, evalTok int) {
			enc := json.NewEncoder(w)
			_ = enc.Encode(ChatResponse{
				Model:   "test",
				Message: ChatMessage{Role: "assistant", Content: content},
				Done:    false,
			})
			_ = enc.Encode(ChatResponse{
				Model:           "test",
				Message:         ChatMessage{Role: "assistant", Content: ""},
				Done:            true,
				PromptEvalCount: promptTok,
				EvalCount:       evalTok,
			})
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		switch call {
		case 1:
			writeChunks(`<tool_call>{"tool":"write_file","args":{"path":"out.txt","content":"hello"}}</tool_call>`, 5, 10)
		default:
			writeChunks("done writing files", 3, 5)
		}
	}))
}

func TestChatWithTools_ExecutesToolAndReturnsText(t *testing.T) {
	srv := toolStreamServer(t)
	defer srv.Close()

	client := NewGenerateClient(srv.URL)
	cwt, ok := client.(GenerateClientWithTools)
	if !ok {
		t.Fatal("NewGenerateClient does not implement GenerateClientWithTools")
	}

	var toolCalled bool
	executor := func(name string, args map[string]any) string {
		switch name {
		case "list_dir":
			return "cmd/ internal/ go.mod" // pre-load bootstrap
		case "write_file":
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
	_ = usage // token counts are accumulated but not critical to assert here

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
	client := NewGenerateClient("http://127.0.0.1:1")
	if _, ok := client.(GenerateClientWithTools); !ok {
		t.Fatal("genClient does not implement GenerateClientWithTools")
	}
}

func TestParseToolCall_ValidJSON(t *testing.T) {
	text := "some preamble\n<tool_call>\n{\"tool\":\"write_file\",\"args\":{\"path\":\"foo.go\",\"content\":\"hello\"}}\n</tool_call>"
	tc, ok := parseToolCall(text)
	if !ok {
		t.Fatal("expected tool call to be found")
	}
	if tc.Tool != "write_file" {
		t.Errorf("tool = %q", tc.Tool)
	}
	if tc.Args["path"] != "foo.go" {
		t.Errorf("path = %v", tc.Args["path"])
	}
}

func TestParseToolCall_NoBlock(t *testing.T) {
	_, ok := parseToolCall("just plain text, no tool call here")
	if ok {
		t.Fatal("expected no tool call")
	}
}

func TestParseToolCall_MalformedJSON(t *testing.T) {
	text := "<tool_call>not json</tool_call>"
	_, ok := parseToolCall(text)
	if ok {
		t.Fatal("expected parse failure on bad JSON")
	}
}
