package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const (
	footerMonitorTitle = "Monitor"
	footerMonitorLines = 3 // title row + 1 content line + bottom border
)

// FooterMonitorData is a single-line Metrics+System summary for the features footer.
type FooterMonitorData struct {
	Width  int
	Ollama ollama.Snapshot
	GPU    gpu.Snapshot
	Tokens tokens.SessionCounter
	System system.Snapshot
}

// RenderMonitorFooter renders a one-line monitor bar (tabs 4+5 summary) for the page footer.
func RenderMonitorFooter(d FooterMonitorData) string {
	width := d.Width
	if width < 1 {
		width = 80
	}
	return ui.Panel(footerMonitorTitle, formatFooterMonitorLine(d), width, footerMonitorLines)
}

func formatFooterMonitorLine(d FooterMonitorData) string {
	segments := []string{
		compactModelSegment(d.Ollama),
		compactGPUSegment(d.GPU),
		compactTokensSegment(d.Tokens),
		compactSystemSegment(d.System),
	}
	line := strings.Join(segments, " │ ")
	max := d.Width - 6
	if max < 20 {
		max = 20
	}
	return truncateRunes(line, max)
}

func compactModelSegment(snap ollama.Snapshot) string {
	if !snap.Reachable {
		return "Model offline"
	}
	if len(snap.Running) == 0 {
		return "Model idle"
	}
	name := snap.Running[0].Name
	if i := strings.Index(name, ":"); i > 0 {
		name = name[:i]
	}
	return "Model ● " + name
}

func compactGPUSegment(snap gpu.Snapshot) string {
	if !snap.Available || len(snap.Devices) == 0 {
		return "GPU —"
	}
	d := snap.Devices[0]
	tag := strings.ToUpper(string(d.Vendor))
	if tag == "" {
		tag = "GPU"
	}
	return fmt.Sprintf("GPU %s %.0f%% %d/%dMiB %.0f°C",
		tag, d.Utilization, d.MemoryUsed, d.MemoryTotal, d.Temperature)
}

func compactTokensSegment(c tokens.SessionCounter) string {
	if c.RequestCount == 0 {
		return "Tok —"
	}
	return fmt.Sprintf("Tok %s+%s (%d)",
		compactTokenCount(c.PromptTokens),
		compactTokenCount(c.CompletionTokens),
		c.RequestCount)
}

func compactTokenCount(n int) string {
	if n < 1000 {
		return formatCommaInt(n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

func compactSystemSegment(snap system.Snapshot) string {
	if !snap.Available {
		return "CPU — MEM —"
	}
	cpu := "--"
	if snap.CPU.HasBaseline {
		cpu = fmt.Sprintf("%.0f", snap.CPU.Utilization)
	}
	memPct := 0.0
	if snap.Memory.TotalMiB > 0 {
		memPct = float64(snap.Memory.UsedMiB) / float64(snap.Memory.TotalMiB) * 100
	}
	return fmt.Sprintf("CPU %s%% MEM %.0f%% Ld %.1f", cpu, memPct, snap.Load.One)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	for _, r := range runes {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate) > max {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}
