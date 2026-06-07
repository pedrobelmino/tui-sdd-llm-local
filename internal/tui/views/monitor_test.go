package views

import (
	"strings"
	"testing"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
)

func TestRenderMonitorStrip_RunningModelAndGPU(t *testing.T) {
	out := RenderMonitorStrip(MonitorData{
		Width: 100,
		Ollama: ollama.Snapshot{
			Reachable: true,
			Running: []ollama.RunningModel{{
				Name: "qwen2.5-coder:latest", SizeVRAM: 2408937472, ContextLength: 4096,
			}},
		},
		GPU: gpu.Snapshot{
			Available: true,
			Devices: []gpu.Device{{
				Vendor: gpu.VendorAMD, Utilization: 55, MemoryUsed: 3456, MemoryTotal: 4096, Temperature: 62,
			}},
		},
		Tokens: tokens.SessionCounter{PromptTokens: 100, CompletionTokens: 50, RequestCount: 2},
	})

	for _, want := range []string{
		"Monitor",
		"qwen2.5-coder:latest",
		"AMD 55%",
		"VRAM 3456/4096 MiB",
		"100 prompt + 50 completion",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMonitorStrip_Offline(t *testing.T) {
	out := RenderMonitorStrip(MonitorData{
		Width:  80,
		Ollama: ollama.Snapshot{Reachable: false},
	})
	if !strings.Contains(out, "offline") {
		t.Fatalf("expected offline message: %s", out)
	}
}
