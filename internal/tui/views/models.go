package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

// ModelsData is the input subset for the Models view renderer.
type ModelsData struct {
	Width               int
	Height              int
	Tags                []ollama.TagModel
	Running             []ollama.RunningModel
	Reachable           bool
	DefaultModelMissing bool
	Error               string
}

const (
	ollamaUnreachableMsg = "Ollama unreachable — hint: systemctl status ollama"
	defaultModelWarnFmt  = "! model not pulled: %s — run: ollama pull %s"
)

// RenderModels renders installed and running Ollama model tables.
func RenderModels(d ModelsData) string {
	width := d.Width
	if width < 1 {
		width = 80
	}

	var parts []string

	if !d.Reachable {
		parts = append(parts, ui.BannerError(ollamaUnreachableMsg, width))
	}

	if d.DefaultModelMissing && d.Reachable {
		parts = append(parts, formatWarning(fmt.Sprintf(defaultModelWarnFmt, ollama.DefaultModel, ollama.DefaultModel)))
	}

	installed := buildInstalledTable(d.Tags, width)
	running := buildRunningTable(d.Running, width)

	installedLines := tableContentLines(dataRowCount(len(d.Tags)))
	runningLines := tableContentLines(dataRowCount(len(d.Running)))

	parts = append(parts, ui.Panel("Installed", installed.View(), width, panelHeight(installedLines)))
	parts = append(parts, ui.Panel("Running", running.View(), width, panelHeight(runningLines)))

	return strings.Join(parts, "\n")
}

func buildInstalledTable(tags []ollama.TagModel, width int) table.Model {
	cols := []table.Column{
		{Title: "NAME", Width: 28},
		{Title: "SIZE", Width: 10},
		{Title: "PARAMS", Width: 8},
		{Title: "QUANT", Width: 10},
	}
	rows := make([]table.Row, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, table.Row{
			tag.Name,
			formatBytes(tag.Size),
			tag.Details.ParameterSize,
			tag.Details.QuantizationLevel,
		})
	}
	if len(rows) == 0 {
		rows = []table.Row{{"(none)", "", "", ""}}
	}
	return newTable(cols, rows, width, dataRowCount(len(tags)))
}

func buildRunningTable(models []ollama.RunningModel, width int) table.Model {
	cols := []table.Column{
		{Title: "NAME", Width: 28},
		{Title: "VRAM", Width: 10},
		{Title: "EXPIRES", Width: 20},
	}
	rows := make([]table.Row, 0, len(models))
	for _, m := range models {
		rows = append(rows, table.Row{
			m.Name,
			formatBytes(m.SizeVRAM),
			formatExpires(m.ExpiresAt),
		})
	}
	if len(rows) == 0 {
		rows = []table.Row{{"(none)", "", ""}}
	}
	return newTable(cols, rows, width, dataRowCount(len(models)))
}

func newTable(cols []table.Column, rows []table.Row, width, dataRows int) table.Model {
	// WithHeight counts the header row; add one so all data rows are visible.
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(dataRows+1),
		table.WithWidth(width-4),
		table.WithFocused(false),
	)
	t.SetStyles(plainTableStyles())
	return t
}

func plainTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	s.Cell = lipgloss.NewStyle()
	s.Selected = s.Cell
	return s
}

func dataRowCount(n int) int {
	if n == 0 {
		return 1
	}
	return n
}

func tableContentLines(dataRows int) int {
	return dataRows + 1
}

func panelHeight(contentLines int) int {
	return contentLines + 2
}

func formatBytes(b int64) string {
	if b < 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(b) / float64(div)
	suffix := []string{"KB", "MB", "GB", "TB"}[exp]
	return fmt.Sprintf("%.1f %s", value, suffix)
}

func formatExpires(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func formatWarning(msg string) string {
	if ui.NoColor() {
		return msg
	}
	return ui.StyleError.Render(msg)
}
