package views

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const monitorPanelTitle = "Monitor"

// MonitorData is the live Ollama/GPU/token snapshot for the status strip.
type MonitorData struct {
	Width           int
	Ollama          ollama.Snapshot
	GPU             gpu.Snapshot
	Tokens          tokens.SessionCounter
	ConfiguredModel string
}

// RenderMonitorStrip renders a compact always-visible model + GPU status panel.
func RenderMonitorStrip(d MonitorData) string {
	width := d.Width
	if width < 1 {
		width = 80
	}
	return ui.Panel(monitorPanelTitle, monitorContent(d), width, 5)
}

func monitorContent(d MonitorData) string {
	return strings.Join([]string{
		formatOllamaLine(d.Ollama, d.ConfiguredModel),
		formatGPULine(d.GPU),
		formatTokensLine(d.Tokens),
	}, "\n")
}

func formatOllamaLine(snap ollama.Snapshot, configured string) string {
	if !snap.Reachable {
		return "Model: offline — Ollama unreachable"
	}
	if len(snap.Running) == 0 {
		if configured != "" {
			return "Model: · " + configured + " — not loaded in VRAM"
		}
		return "Model: idle — no model loaded in VRAM"
	}
	r := snap.Running[0]
	vram := formatBytes(r.SizeVRAM)
	ctx := ""
	if r.ContextLength > 0 {
		ctx = fmt.Sprintf(" │ ctx %d", r.ContextLength)
	}
	return fmt.Sprintf("Model: ● %s │ VRAM %s%s", r.Name, vram, ctx)
}

func formatGPULine(snap gpu.Snapshot) string {
	if !snap.Available || len(snap.Devices) == 0 {
		msg := "GPU: unavailable"
		if snap.Error != "" {
			msg += " — " + snap.Error
		}
		return msg
	}
	d := snap.Devices[0]
	tag := strings.ToUpper(string(d.Vendor))
	if tag == "" {
		tag = "GPU"
	}
	return fmt.Sprintf("GPU:   %s %.0f%% │ VRAM %d/%d MiB │ %.0f°C",
		tag, d.Utilization, d.MemoryUsed, d.MemoryTotal, d.Temperature)
}

func formatTokensLine(counter tokens.SessionCounter) string {
	if counter.RequestCount == 0 {
		return "Tokens: —"
	}
	return fmt.Sprintf("Tokens: %s prompt + %s completion (%d requests)",
		formatCommaInt(counter.PromptTokens),
		formatCommaInt(counter.CompletionTokens),
		counter.RequestCount)
}
