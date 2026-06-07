package views

import (
	"strings"
	"testing"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
)

func TestRenderFeatures_WithBadgesSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderFeatures(FeaturesData{
		Width:  80,
		Height: 24,
		Features: []project.FeatureEntry{
			{Name: "cli-scaffold", HasSpec: true, HasTasks: true, HasDesign: true},
			{Name: "other-feat", HasSpec: true, HasTasks: false, HasDesign: false},
		},
	})

	if !strings.Contains(got, "cli-scaffold") || !strings.Contains(got, "other-feat") {
		t.Fatalf("missing features:\n%s", got)
	}
	if !strings.Contains(got, "n:new") {
		t.Fatalf("missing shortcuts:\n%s", got)
	}
}

func TestRenderFeatures_EmptyStateSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderFeatures(FeaturesData{Width: 80, Height: 24})
	if !strings.Contains(got, "press n to create") {
		t.Fatalf("empty msg:\n%s", got)
	}
}

func TestRenderFeatures_CursorHighlight(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := RenderFeatures(FeaturesData{
		Width: 80, Features: []project.FeatureEntry{{Name: "alpha"}, {Name: "beta"}}, Cursor: 1,
	})
	if !strings.Contains(got, "> beta") {
		t.Fatalf("cursor:\n%s", got)
	}
}
