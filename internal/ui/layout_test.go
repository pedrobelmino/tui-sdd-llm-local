package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

const testWidth = 80

func testTabs() []Tab {
	return []Tab{
		{Key: "1", Label: "Overview"},
		{Key: "2", Label: "Features"},
		{Key: "3", Label: "Models"},
		{Key: "4", Label: "Metrics"},
	}
}

func TestHeader_Snapshot80Cols(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := Header("tsll v0.1.0 ── tui-sdd-llm-local ── qwen2.5-coder", testTabs(), 0, testWidth)

	want := strings.Join([]string{
		"┌─ tsll v0.1.0 ── tui-sdd-llm-local ── qwen2.5-coder ──────────────────────────┐",
		"│ [1] Overview  [2] Features  [3] Models  [4] Metrics                          │",
	}, "\n")

	if got != want {
		t.Fatalf("header snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestHeader_ActiveTabSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := Header("tsll", testTabs(), 2, testWidth)
	if !strings.Contains(got, "[3] Models") {
		t.Fatalf("expected active tab in output, got:\n%s", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lipgloss.Width(lines[1]) != testWidth {
		t.Fatalf("tab line width = %d, want %d", lipgloss.Width(lines[1]), testWidth)
	}
}

func TestFooter_Snapshot80Cols(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	keymap := "r: refresh │ 1-4: views │ ?: help │ q: quit"
	got := Footer(keymap, testWidth)

	want := strings.Join([]string{
		"│ r: refresh │ 1-4: views │ ?: help │ q: quit                                  │",
		"└──────────────────────────────────────────────────────────────────────────────┘",
	}, "\n")

	if got != want {
		t.Fatalf("footer snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFooter_TruncatesToWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	long := strings.Repeat("x", 200)
	got := Footer(long, testWidth)
	lines := strings.Split(got, "\n")
	if lipgloss.Width(lines[0]) != testWidth {
		t.Fatalf("footer line width = %d, want %d", lipgloss.Width(lines[0]), testWidth)
	}
}

func TestPanel_Snapshot80Cols(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	content := "✓ .specs/project/\nCurrent: cli-scaffold\nMilestone: M1"
	got := Panel("Project", content, 30, 5)

	want := strings.Join([]string{
		"╭─ Project ──────────────────╮",
		"│ ✓ .specs/project/          │",
		"│ Current: cli-scaffold      │",
		"│ Milestone: M1              │",
		"╰────────────────────────────╯",
	}, "\n")

	if got != want {
		t.Fatalf("panel snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBannerError_Snapshot80Cols(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := BannerError("Ollama unreachable", testWidth)
	want := "!! Ollama unreachable !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
	if got != want {
		t.Fatalf("banner snapshot mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPlainError_RespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := PlainError("unknown command")
	want := "error: unknown command"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
