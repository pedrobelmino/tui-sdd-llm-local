package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const (
	tokensPanelTitle = "Session Tokens"
	gpuPanelTitle    = "GPU"

	msgNoRequestsYet       = "no requests yet"
	msgGPUUnavailable      = "GPU metrics unavailable"
	msgStalePrefix         = "stale — last updated:"
)

// MetricsData is the subset of RootModel state needed to render the Metrics view.
type MetricsData struct {
	Width  int
	Height int
	Tokens tokens.SessionCounter
	GPU    gpu.Snapshot
}

// RenderMetrics renders the Metrics view with session token and GPU panels.
func RenderMetrics(m MetricsData) string {
	width := m.Width
	if width < 1 {
		width = 80
	}
	height := m.Height
	if height < 1 {
		height = 20
	}

	tokenH := 8
	gpuH := height - tokenH - 1
	if gpuH < 5 {
		gpuH = 5
	}

	tokensBox := ui.Panel(tokensPanelTitle, renderTokensContent(m.Tokens), width, tokenH)
	gpuBox := ui.Panel(gpuPanelTitle, renderGPUContent(m.GPU), width, gpuH)

	return tokensBox + "\n\n" + gpuBox
}

func renderTokensContent(counter tokens.SessionCounter) string {
	lines := []string{
		fmt.Sprintf("Prompt:     %s", formatCommaInt(counter.PromptTokens)),
		fmt.Sprintf("Completion: %s", formatCommaInt(counter.CompletionTokens)),
		fmt.Sprintf("Total:      %s", formatCommaInt(counter.TotalTokens)),
		fmt.Sprintf("Requests:   %d", counter.RequestCount),
	}
	if counter.RequestCount == 0 {
		lines = append(lines, msgNoRequestsYet)
	} else if counter.LastRequest != nil {
		last := counter.LastRequest
		lines = append(lines, fmt.Sprintf(
			"Last:       %s prompt + %s completion @ %s",
			formatCommaInt(last.PromptTokens),
			formatCommaInt(last.CompletionTokens),
			last.At.Format("15:04:05"),
		))
	}
	return strings.Join(lines, "\n")
}

func renderGPUContent(snap gpu.Snapshot) string {
	if !snap.Available && !snap.Stale {
		lines := []string{msgGPUUnavailable}
		if snap.Error != "" {
			lines = append(lines, snap.Error)
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	if snap.Stale {
		lines = append(lines, fmt.Sprintf("%s %s", msgStalePrefix, snap.FetchedAt.Format("2006-01-02 15:04:05")))
		if snap.Error != "" {
			lines = append(lines, snap.Error)
		}
	}

	for _, dev := range snap.Devices {
		lines = append(lines, formatDeviceLine(dev))
	}

	if len(lines) == 0 {
		return msgGPUUnavailable
	}
	return strings.Join(lines, "\n")
}

func formatDeviceLine(dev gpu.Device) string {
	return fmt.Sprintf(
		"[%d] %s │ %.0f%% │ %s │ %.0f°C",
		dev.Index,
		dev.Name,
		dev.Utilization,
		formatVRAM(dev.MemoryUsed, dev.MemoryTotal),
		dev.Temperature,
	)
}

func formatVRAM(used, total uint64) string {
	return fmt.Sprintf("%d/%d MiB", used, total)
}

func formatCommaInt(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}

	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}

	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}
