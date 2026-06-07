package workflow

import (
	"strings"
	"testing"
)

func TestDetectStackLabel(t *testing.T) {
	cases := map[string]string{
		"## Tech Stack\n- Language: Go\n- CLI: Cobra":          "Go",
		"## Tech Stack\n- React + Vite + TypeScript":            "React/JS",
		"## Tech Stack\n- Language: React no front e Go no backend": "full-stack (React/JS + Go)",
		"## Tech Stack\n- Python + FastAPI":                     "",
	}
	for stack, want := range cases {
		if got := detectStackLabel(stack); got != want {
			t.Errorf("detectStackLabel(%q)=%q want %q", stack, got, want)
		}
	}
}

// A pure React project (no Go signal) must NOT be flagged as Go, even when a task
// mentions "API" or the stack mentions "backend".
func TestStackLanguageGuard_ReactForbidsGo(t *testing.T) {
	stack := "## Tech Stack\n- React + Vite\n- Talks to a backend API over fetch"
	guard := stackLanguageGuard(stack)
	if !strings.Contains(guard, "JavaScript/React") {
		t.Fatalf("expected React guard, got %q", guard)
	}
	if strings.Contains(strings.ToLower(guard), "go project") {
		t.Fatalf("React project must not be guarded as Go: %q", guard)
	}
}

func TestStackLanguageGuard_GoForbidsJS(t *testing.T) {
	stack := "## Tech Stack\n- Language: Go\n- CLI: Cobra"
	guard := stackLanguageGuard(stack)
	if !strings.Contains(guard, "Go project") {
		t.Fatalf("expected Go guard, got %q", guard)
	}
}

func TestStackLanguageGuard_FullStackNoGuard(t *testing.T) {
	stack := "## Tech Stack\n- Language: React no front e Go no backend"
	if guard := stackLanguageGuard(stack); guard != "" {
		t.Fatalf("full-stack must not force a single language: %q", guard)
	}
}
