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

func TestParseToolCall_TagFormat(t *testing.T) {
	text := "preamble\n<tool_call>\n{\"tool\":\"write_file\",\"args\":{\"path\":\"foo.go\",\"content\":\"hello\"}}\n</tool_call>"
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

func TestParseToolCall_CodeBlockJSON(t *testing.T) {
	// Model outputs tool call in a ```json code block (observed in the wild)
	text := "```json\n{\"tool\":\"read_file\",\"args\":{\"path\":\".specs/spec.md\"}}\n```"
	tc, ok := parseToolCall(text)
	if !ok {
		t.Fatal("expected code-block tool call to be found")
	}
	if tc.Tool != "read_file" {
		t.Errorf("tool = %q", tc.Tool)
	}
}

func TestParseToolCall_CodeBlockNoLang(t *testing.T) {
	// No language hint
	text := "```{\"tool\":\"list_dir\",\"args\":{\"path\":\".\"}}```"
	tc, ok := parseToolCall(text)
	if !ok {
		t.Fatal("expected bare code-block tool call to be found")
	}
	if tc.Tool != "list_dir" {
		t.Errorf("tool = %q", tc.Tool)
	}
}

func TestParseToolCall_BareJSON(t *testing.T) {
	// Bare JSON object in the response
	text := "Here is the call: {\"tool\":\"create_dir\",\"args\":{\"path\":\"internal/foo\"}}"
	tc, ok := parseToolCall(text)
	if !ok {
		t.Fatal("expected bare JSON tool call to be found")
	}
	if tc.Tool != "create_dir" {
		t.Errorf("tool = %q", tc.Tool)
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

func TestParseToolCall_BracesInContent(t *testing.T) {
	text := `<tool_call>{"tool":"write_file","args":{"path":"app/x.js","content":"function init() { return 1; }\n"}}</tool_call>`
	tc, ok := parseToolCall(text)
	if !ok {
		t.Fatal("expected brace-aware parse")
	}
	if tc.Tool != "write_file" {
		t.Fatalf("tool = %q", tc.Tool)
	}
	content, _ := tc.Args["content"].(string)
	if !strings.Contains(content, "return 1") {
		t.Fatalf("content = %q", content)
	}
}

func TestParseToolCall_LiteralNewlinesInContent(t *testing.T) {
	text := "<tool_call>\n{\"tool\":\"write_file\",\"args\":{\"path\":\"app/Footer.go\",\"content\":\"package app\n\nfunc Footer() {}\n\"}}\n</tool_call>"
	tc, ok := parseToolCall(text)
	if !ok {
		t.Fatal("expected lenient parse for multiline content")
	}
	if tc.Args["path"] != "app/Footer.go" {
		t.Fatalf("path = %v", tc.Args["path"])
	}
	content, _ := tc.Args["content"].(string)
	if !strings.Contains(content, "package app") {
		t.Fatalf("content = %q", content)
	}
}

func TestLooksLikeMalformedToolCall_Narrow(t *testing.T) {
	if looksLikeMalformedToolCall("I will use { braces } in prose without tools") {
		t.Fatal("prose with braces should not look malformed")
	}
	if !looksLikeMalformedToolCall(`<tool_call>{"tool":"list_dir"`) {
		t.Fatal("unclosed tool_call tag should look malformed")
	}
	if !looksLikeMalformedToolCall(`{"tool":"read_file","args":{"path":"x"}}`) {
		t.Fatal("bare tool JSON should look malformed when parse fails elsewhere")
	}
}

func TestFormatToolInvocation_WriteFileHidesContent(t *testing.T) {
	got := formatToolInvocation(toolCallJSON{
		Tool: "write_file",
		Args: map[string]any{"path": "src/main.go", "content": "package main\n\nfunc main() {}\n"},
	})
	if strings.Contains(got, "package main") {
		t.Fatalf("should not show file content in log: %q", got)
	}
	if !strings.Contains(got, "src/main.go") || !strings.Contains(got, "bytes") {
		t.Fatalf("unexpected invocation format: %q", got)
	}
}

func TestPreflightToolBlock_PlaceholderDir(t *testing.T) {
	blocked, reason := preflightToolBlock(toolCallJSON{
		Tool: "create_dir",
		Args: map[string]any{"path": "internal/foo"},
	}, loopPreflightState{})
	if !blocked || !strings.Contains(reason, "placeholder") {
		t.Fatalf("expected placeholder block, got blocked=%v reason=%q", blocked, reason)
	}
}

func TestPreflightToolBlock_ReadAfterCreate(t *testing.T) {
	blocked, _ := preflightToolBlock(toolCallJSON{
		Tool: "read_file",
		Args: map[string]any{"path": "internal/bar"},
	}, loopPreflightState{lastCreateDirPath: "internal/bar"})
	if !blocked {
		t.Fatal("expected read_file after create_dir to be blocked")
	}
}

func TestIsGreenfieldLayout(t *testing.T) {
	if !isGreenfieldLayout(".specs/\n") {
		t.Fatal("expected greenfield for specs-only root")
	}
	if isGreenfieldLayout("internal/\n.specs/\n") {
		t.Fatal("expected non-greenfield when internal/ exists")
	}
}

func TestLooksLikePrematureDone_JSONSummary(t *testing.T) {
	if !looksLikePrematureDone(`{"summary":"Created models/product.go"}`, 1, 2) {
		t.Fatal("expected JSON summary with insufficient writes to be premature")
	}
	if looksLikePrematureDone("Implemented header, footer, and models.", 2, 2) {
		t.Fatal("expected valid plain summary to be accepted")
	}
}

func TestValidateToolCallArgs_EmptyWriteFile(t *testing.T) {
	reason := validateToolCallArgs(toolCallJSON{
		Tool: "write_file",
		Args: map[string]any{"path": "app/landing-page/styles.css", "content": ""},
	})
	if reason == "" || !strings.Contains(reason, "empty") {
		t.Fatalf("expected empty content error, got %q", reason)
	}
	if !strings.Contains(reason, "body { margin") {
		t.Fatalf("expected CSS hint, got %q", reason)
	}
}

func TestPreflightToolBlock_RepeatFailedWrite(t *testing.T) {
	tc := toolCallJSON{Tool: "write_file", Args: map[string]any{"path": "app/x.css", "content": ""}}
	sig := toolFailureSignature(tc)
	blocked, reason := preflightToolBlock(tc, loopPreflightState{lastFailedSig: sig})
	if !blocked || !strings.Contains(reason, "blocked repeat") {
		t.Fatalf("expected repeat block, got blocked=%v reason=%q", blocked, reason)
	}
}

func TestMarksSourceInspection_AppPath(t *testing.T) {
	tc := toolCallJSON{Tool: "list_dir", Args: map[string]any{"path": "app/components"}}
	if !marksSourceInspection(tc) {
		t.Fatal("expected app/ path to count as source inspection")
	}
}

func TestSourceInspectPathFor_AppComponent(t *testing.T) {
	got := sourceInspectPathFor("app/components/Footer.js")
	if got != "app" {
		t.Fatalf("got %q, want app", got)
	}
}

func TestBootstrapSatisfiesInspection_AppListed(t *testing.T) {
	root := "app/\n.specs/\n"
	layout := "Project root layout (list_dir \".\"):\napp/\n\napp/:\ncomponents/\n"
	if !bootstrapSatisfiesInspection(root, layout) {
		t.Fatal("expected bootstrap app/ listing to satisfy inspection")
	}
}

func TestPreflightToolBlock_RepeatRootListDir(t *testing.T) {
	tc := toolCallJSON{Tool: "list_dir", Args: map[string]any{"path": "."}}
	blocked, reason := preflightToolBlock(tc, loopPreflightState{rootListed: true})
	if !blocked || !strings.Contains(reason, "already loaded") {
		t.Fatalf("expected root list_dir block, got blocked=%v reason=%q", blocked, reason)
	}
}
