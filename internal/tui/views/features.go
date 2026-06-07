package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const (
	featuresPanelWidth = 68
	featuresEmptyMsg   = "No features yet — press n to create"
)

// FeaturesData is the subset of RootModel fields needed to render the Features view.
type FeaturesData struct {
	Width    int
	Height   int
	Features []project.FeatureEntry
	Cursor   int
}

// RenderFeatures renders the Features view with selection cursor.
func RenderFeatures(d FeaturesData) string {
	content := featuresContent(d.Features, d.Cursor)
	lines := strings.Count(content, "\n") + 1
	panelH := lines + 2
	if panelH < 8 {
		panelH = 8
	}

	panel := ui.Panel("Features", content, featuresPanelWidth, panelH)
	help := padLines(ui.Panel("Shortcuts", "n:new  s:spec  d:design  t:tasks  e:impl  j/k:move", featuresPanelWidth, 4), 3)
	return padLines(panel, 3) + "\n" + help
}

func featuresContent(features []project.FeatureEntry, cursor int) string {
	if len(features) == 0 {
		return featuresEmptyMsg
	}

	sorted := append([]project.FeatureEntry(nil), features...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var lines []string
	lines = append(lines, "  NAME            SPEC DSGN TASK IMPL")
	for i, f := range sorted {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%-15s  %s    %s    %s    %s",
			prefix,
			f.Name,
			featureBadge(f.HasSpec),
			featureBadge(f.HasDesign),
			featureBadge(f.HasTasks),
			featureBadge(f.HasImplement),
		))
	}
	return strings.Join(lines, "\n")
}

func featureBadge(present bool) string {
	if present {
		return "✓"
	}
	return "·"
}
