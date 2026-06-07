package views

import (
	"strings"
	"testing"
	"time"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
)

const testWidth = 80

func testTag(name string, size int64) ollama.TagModel {
	return ollama.TagModel{
		Name:  name,
		Model: name,
		Size:  size,
		Details: ollama.ModelDetails{
			ParameterSize:     "7.6B",
			QuantizationLevel: "Q4_K_M",
		},
	}
}

func testRunning(name string, vram int64, expires time.Time) ollama.RunningModel {
	return ollama.RunningModel{
		Name:      name,
		Model:     name,
		SizeVRAM:  vram,
		ExpiresAt: expires,
		Details: ollama.ModelDetails{
			ParameterSize:     "7.6B",
			QuantizationLevel: "Q4_K_M",
		},
	}
}

func TestRenderModels_UnreachableBanner(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := RenderModels(ModelsData{
		Width:     testWidth,
		Reachable: false,
	})

	want := "!! Ollama unreachable — hint: systemctl status ollama"
	if !strings.Contains(got, want) {
		t.Fatalf("expected unreachable banner %q in output:\n%s", want, got)
	}
	if strings.Contains(got, "model not pulled") {
		t.Fatalf("default model warning should not appear when unreachable:\n%s", got)
	}
}

func TestRenderModels_InstalledTable(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := RenderModels(ModelsData{
		Width:     testWidth,
		Reachable: true,
		Tags: []ollama.TagModel{
			testTag("qwen2.5-coder:latest", 4683087332),
			testTag("llama3.2:latest", 2019393189),
		},
	})

	for _, want := range []string{
		"Installed",
		"NAME",
		"qwen2.5-coder:latest",
		"llama3.2:latest",
		"4.4 GB",
		"1.9 GB",
		"7.6B",
		"Q4_K_M",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in installed table output:\n%s", want, got)
		}
	}
}

func TestRenderModels_RunningTable(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	expires, err := time.Parse(time.RFC3339, "2024-11-11T10:14:17Z")
	if err != nil {
		t.Fatalf("parse expires: %v", err)
	}

	got := RenderModels(ModelsData{
		Width:     testWidth,
		Reachable: true,
		Running: []ollama.RunningModel{
			testRunning("qwen2.5-coder:latest", 5368709120, expires),
		},
	})

	for _, want := range []string{
		"Running",
		"VRAM",
		"EXPIRES",
		"qwen2.5-coder:latest",
		"5.0 GB",
		"2024-11-11 10:14:17",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in running table output:\n%s", want, got)
		}
	}
}

func TestRenderModels_DefaultModelWarning(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := RenderModels(ModelsData{
		Width:               testWidth,
		Reachable:           true,
		DefaultModelMissing: true,
		Tags: []ollama.TagModel{
			testTag("llama3.2:latest", 2019393189),
		},
	})

	want := "! model not pulled: qwen2.5-coder — run: ollama pull qwen2.5-coder"
	if !strings.Contains(got, want) {
		t.Fatalf("expected default model warning %q in output:\n%s", want, got)
	}
}

func TestRenderModels_EmptyTables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := RenderModels(ModelsData{
		Width:     testWidth,
		Reachable: true,
		Tags:      nil,
		Running:   nil,
	})

	if strings.Count(got, "(none)") != 2 {
		t.Fatalf("expected two empty-state rows, got:\n%s", got)
	}
}
