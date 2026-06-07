package views

import (
	"strings"
	"testing"
)

const overviewTestWidth = 80

func TestRenderOverview_ValidProjectSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderOverview(OverviewData{
		Width:       overviewTestWidth,
		Height:      24,
		Root:        "/home/pedro/dev/tsll",
		Valid:       true,
		CurrentWork: "cli-scaffold",
		Milestone:   "M1",
	})

	want := strings.Join([]string{
		"   ╭─ Project ────────────────────────╮",
		"   │ ✓ /home/pedro/dev/tsll           │",
		"   │ Current: cli-scaffold            │",
		"   │ Milestone: M1                    │",
		"   │                                  │",
		"   │ Workflow (TUI):                  │",
		"   │   2 → Features → n new feature   │",
		"   │   enter → task detail → e run tas│",
		"   │   s/t → evolve spec & tasks      │",
		"   ╰──────────────────────────────────╯",
	}, "\n")

	if got != want {
		t.Fatalf("valid project snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderOverview_NoProjectSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderOverview(OverviewData{
		Width:  overviewTestWidth,
		Height: 24,
	})

	want := strings.Join([]string{
		"   ╭─ Project ────────────────────────╮",
		"   │ No .specs/project/ found         │",
		"   │                                  │",
		"   │ Press i to init project          │",
		"   ╰──────────────────────────────────╯",
	}, "\n")

	if got != want {
		t.Fatalf("no project snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderOverview_CorruptedProjectSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderOverview(OverviewData{
		Width:     overviewTestWidth,
		Height:    24,
		Corrupted: true,
		Root:      "/tmp/broken",
	})

	want := strings.Join([]string{
		"   ╭─ Project ────────────────────────╮",
		"   │ ! corrupted project              │",
		"   │ Run: tsll init --force           │",
		"   │                                  │",
		"   ╰──────────────────────────────────╯",
	}, "\n")

	if got != want {
		t.Fatalf("corrupted project snapshot mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
