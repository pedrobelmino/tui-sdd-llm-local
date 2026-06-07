package tokens

import "testing"

func TestSessionCounter_ZeroState(t *testing.T) {
	var s SessionCounter

	if s.PromptTokens != 0 || s.CompletionTokens != 0 || s.TotalTokens != 0 {
		t.Fatalf("expected zero token counts, got prompt=%d completion=%d total=%d",
			s.PromptTokens, s.CompletionTokens, s.TotalTokens)
	}
	if s.RequestCount != 0 {
		t.Fatalf("expected RequestCount 0, got %d", s.RequestCount)
	}
	if s.LastRequest != nil {
		t.Fatal("expected LastRequest nil in zero state")
	}
}

func TestSessionCounter_AddAccumulates(t *testing.T) {
	var s SessionCounter

	s.Add(100, 50)
	if s.PromptTokens != 100 || s.CompletionTokens != 50 || s.TotalTokens != 150 {
		t.Fatalf("after first add: prompt=%d completion=%d total=%d", s.PromptTokens, s.CompletionTokens, s.TotalTokens)
	}
	if s.RequestCount != 1 {
		t.Fatalf("expected RequestCount 1, got %d", s.RequestCount)
	}
	if s.LastRequest == nil || s.LastRequest.PromptTokens != 100 || s.LastRequest.CompletionTokens != 50 {
		t.Fatal("LastRequest should reflect the most recent add")
	}

	s.Add(30, 20)
	if s.PromptTokens != 130 || s.CompletionTokens != 70 || s.TotalTokens != 200 {
		t.Fatalf("after second add: prompt=%d completion=%d total=%d", s.PromptTokens, s.CompletionTokens, s.TotalTokens)
	}
	if s.RequestCount != 2 {
		t.Fatalf("expected RequestCount 2, got %d", s.RequestCount)
	}
	if s.LastRequest.PromptTokens != 30 || s.LastRequest.CompletionTokens != 20 {
		t.Fatal("LastRequest should reflect the second add only")
	}
}

func TestFromOllamaResponse_MapsCounts(t *testing.T) {
	snapshot := FromOllamaResponse(42, 17)

	if snapshot.PromptTokens != 42 {
		t.Fatalf("expected PromptTokens 42, got %d", snapshot.PromptTokens)
	}
	if snapshot.CompletionTokens != 17 {
		t.Fatalf("expected CompletionTokens 17, got %d", snapshot.CompletionTokens)
	}
	if snapshot.At.IsZero() {
		t.Fatal("expected non-zero At timestamp")
	}
}

func TestSessionCounter_TotalTokensEqualsSum(t *testing.T) {
	var s SessionCounter
	s.Add(10, 5)
	s.Add(3, 7)

	want := s.PromptTokens + s.CompletionTokens
	if s.TotalTokens != want {
		t.Fatalf("TotalTokens=%d, want Prompt+Completion=%d", s.TotalTokens, want)
	}
}
