package views

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

// FeatureDetailData renders a single feature and its tasks.
type FeatureDetailData struct {
	Width       int
	Height      int
	Feature     project.FeatureEntry
	Tasks       []project.TaskEntry
	TaskCursor  int
	Progress    string
}

// RenderFeatureDetail shows feature metadata and task list.
func RenderFeatureDetail(d FeatureDetailData) string {
	var b strings.Builder

	meta := fmt.Sprintf(
		"Feature: %s\nSpec: %s  Design: %s  Tasks: %s  Impl: %s\n%s\n",
		d.Feature.Name,
		badge(d.Feature.HasSpec),
		badge(d.Feature.HasDesign),
		badge(d.Feature.HasTasks),
		badge(d.Feature.HasImplement),
		d.Progress,
	)
	b.WriteString(ui.Panel("Feature", meta, d.Width-6, 7))

	if len(d.Tasks) == 0 {
		b.WriteString("\n")
		b.WriteString(padLines(ui.Panel("Tasks", "No tasks yet — press t to generate", d.Width-6, 5), 3))
		return b.String()
	}

	var lines []string
	lines = append(lines, "ID    STATUS        TITLE")
	for i, t := range d.Tasks {
		prefix := "  "
		if i == d.TaskCursor {
			prefix = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%-5s %-13s %s",
			prefix, t.ID, truncate(t.Status, 12), truncate(t.Title, 40)))
	}
	body := strings.Join(lines, "\n")
	h := len(d.Tasks) + 4
	b.WriteString("\n")
	b.WriteString(padLines(ui.Panel("Tasks", body, d.Width-6, h), 3))

	help := "s:spec  d:design  t:tasks  e:run  a:impl  j/k:nav  esc:back"
	b.WriteString("\n")
	b.WriteString(padLines(ui.Panel("Actions", help, d.Width-6, 4), 3))

	return b.String()
}

func badge(ok bool) string {
	if ok {
		return "✓"
	}
	return "·"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
