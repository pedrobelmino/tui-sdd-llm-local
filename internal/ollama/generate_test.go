package ollama

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChatStream_AllowsSlowChunks(t *testing.T) {
	// Old clientTimeout was 5s; gaps between chunks must exceed that.
	const delay = 6 * time.Second

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not a Flusher")
		}

		fmt.Fprintf(w, `{"message":{"role":"assistant","content":"hel"},"done":false}`+"\n")
		flusher.Flush()

		time.Sleep(delay)
		fmt.Fprintf(w, `{"message":{"role":"assistant","content":"lo"},"done":true,"prompt_eval_count":1,"eval_count":2}`+"\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client := NewGenerateClient(srv.URL)
	text, usage, err := client.ChatStream(context.Background(), ChatRequest{
		Model: "test",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if text != "hello" {
		t.Fatalf("text = %q, want hello", text)
	}
	if usage.PromptTokens != 1 || usage.CompletionTokens != 2 {
		t.Fatalf("usage = %+v, want prompt=1 completion=2", usage)
	}
}
