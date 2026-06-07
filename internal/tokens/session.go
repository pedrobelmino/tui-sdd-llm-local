package tokens

import "time"

// UsageSnapshot captures token usage for a single request.
type UsageSnapshot struct {
	PromptTokens     int
	CompletionTokens int
	At               time.Time
}

// SessionCounter accumulates token usage across a TUI session.
type SessionCounter struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	RequestCount     int
	LastRequest      *UsageSnapshot
}

// FromOllamaResponse maps Ollama prompt_eval_count and eval_count to a snapshot.
func FromOllamaResponse(promptEvalCount, evalCount int) UsageSnapshot {
	return UsageSnapshot{
		PromptTokens:     promptEvalCount,
		CompletionTokens: evalCount,
		At:               time.Now(),
	}
}

// Add accumulates token counts from one request into the session totals.
func (s *SessionCounter) Add(promptEval, evalCount int) {
	snapshot := FromOllamaResponse(promptEval, evalCount)
	s.PromptTokens += snapshot.PromptTokens
	s.CompletionTokens += snapshot.CompletionTokens
	s.TotalTokens = s.PromptTokens + s.CompletionTokens
	s.RequestCount++
	s.LastRequest = &snapshot
}
