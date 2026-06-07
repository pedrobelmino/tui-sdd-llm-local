package views

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

// OverviewData is the subset of RootModel fields needed to render the Overview view.
type OverviewData struct {
	Width       int
	Height      int
	Root        string
	Valid       bool
	Corrupted   bool
	CurrentWork string
	Milestone   string
}

const overviewPanelWidth = 36

// RenderOverview renders the Overview view body for the given dimensions and project state.
func RenderOverview(d OverviewData) string {
	content := overviewContent(d)
	lines := strings.Count(content, "\n") + 1
	panelH := lines + 2
	if panelH < 5 {
		panelH = 5
	}

	panel := ui.Panel("Project", content, overviewPanelWidth, panelH)
	return padLines(panel, 3)
}

func overviewContent(d OverviewData) string {
	switch {
	case d.Corrupted:
		return "! corrupted project\nRun: tsll init --force"
	case !d.Valid:
		return "No .specs/project/ found\n\nPress i to init project"
	default:
		root := d.Root
		if root == "" {
			root = ".specs/project/"
		}
		current := d.CurrentWork
		if current == "" {
			current = "—"
		}
		milestone := d.Milestone
		if milestone == "" {
			milestone = "—"
		}
		return fmt.Sprintf(
			"✓ %s\nCurrent: %s\nMilestone: %s\n\nWorkflow (TUI):\n  2 → Features → n new feature\n  enter → task detail → e run task\n  s/t → evolve spec & tasks",
			root, current, milestone,
		)
	}
}

func padLines(s string, left int) string {
	if left <= 0 {
		return s
	}
	pad := strings.Repeat(" ", left)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
