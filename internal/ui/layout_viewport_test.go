package ui

import (
	"strings"
	"testing"
)

func TestWrapLines_LongLine(t *testing.T) {
	lines := WrapLines("abcdefghij", 4)
	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3: %v", len(lines), lines)
	}
}

func TestPanelViewport_ScrollsDown(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	content := strings.Join([]string{"line-1", "line-2", "line-3", "line-4", "line-5"}, "\n")
	top := PanelViewport("Log", content, 30, 5, 0)
	bottom := PanelViewport("Log", content, 30, 5, 3)

	if !strings.Contains(top, "line-1") {
		t.Fatalf("top viewport should show line-1:\n%s", top)
	}
	if strings.Contains(top, "line-5") {
		t.Fatalf("top viewport should not show line-5:\n%s", top)
	}
	if !strings.Contains(bottom, "line-5") {
		t.Fatalf("bottom viewport should show line-5:\n%s", bottom)
	}
}
