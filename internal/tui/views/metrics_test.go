package views

import (
	"strings"
	"testing"
	"time"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
)

const testMetricsWidth = 80

func TestMetrics_NoRequestsYet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderMetrics(MetricsData{
		Width:  testMetricsWidth,
		Height: 16,
		Tokens: tokens.SessionCounter{},
		GPU:    gpu.Snapshot{Available: false},
	})

	for _, want := range []string{
		"Session Tokens",
		"Prompt:     0",
		"Completion: 0",
		"Total:      0",
		msgNoRequestsYet,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestMetrics_TokenPanelWithCounts(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var counter tokens.SessionCounter
	counter.Add(1240, 890)

	got := RenderMetrics(MetricsData{
		Width:  testMetricsWidth,
		Height: 16,
		Tokens: counter,
		GPU:    gpu.Snapshot{Available: false},
	})

	for _, want := range []string{
		"Prompt:     1,240",
		"Completion: 890",
		"Total:      2,130",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
	if strings.Contains(got, msgNoRequestsYet) {
		t.Fatalf("did not expect %q when requests exist:\n%s", msgNoRequestsYet, got)
	}
}

func TestMetrics_GPUUnavailable(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderMetrics(MetricsData{
		Width:  testMetricsWidth,
		Height: 16,
		Tokens: tokens.SessionCounter{},
		GPU: gpu.Snapshot{
			Available: false,
			Error:     "nvidia-smi not found",
		},
	})

	if !strings.Contains(got, msgGPUUnavailable) {
		t.Fatalf("expected %q in output:\n%s", msgGPUUnavailable, got)
	}
	if !strings.Contains(got, "nvidia-smi not found") {
		t.Fatalf("expected error detail in output:\n%s", got)
	}
}

func TestMetrics_GPUWithDevices(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderMetrics(MetricsData{
		Width:  testMetricsWidth,
		Height: 16,
		Tokens: tokens.SessionCounter{},
		GPU: gpu.Snapshot{
			Available: true,
			Devices: []gpu.Device{
				{
					Index:       0,
					Name:        "NVIDIA GeForce RTX 3090",
					Utilization: 45,
					MemoryUsed:  8192,
					MemoryTotal: 24576,
					Temperature: 65,
				},
				{
					Index:       1,
					Name:        "NVIDIA GeForce GTX 1080 Ti",
					Utilization: 12,
					MemoryUsed:  2048,
					MemoryTotal: 11264,
					Temperature: 55,
				},
			},
			FetchedAt: time.Date(2024, 10, 22, 13, 39, 22, 0, time.UTC),
		},
	})

	for _, want := range []string{
		"[0] NVIDIA GeForce RTX 3090 │ 45% │ 8192/24576 MiB │ 65°C",
		"[1] NVIDIA GeForce GTX 1080 Ti │ 12% │ 2048/11264 MiB │ 55°C",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestMetrics_GPUStaleShowsTimestamp(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	fetchedAt := time.Date(2024, 10, 22, 13, 39, 22, 0, time.UTC)
	got := RenderMetrics(MetricsData{
		Width:  testMetricsWidth,
		Height: 16,
		Tokens: tokens.SessionCounter{},
		GPU: gpu.Snapshot{
			Available: true,
			Stale:     true,
			Error:     "context deadline exceeded",
			Devices: []gpu.Device{
				{
					Index:       0,
					Name:        "NVIDIA GeForce RTX 3060",
					Utilization: 45,
					MemoryUsed:  6349,
					MemoryTotal: 12288,
					Temperature: 62,
				},
			},
			FetchedAt: fetchedAt,
		},
	})

	for _, want := range []string{
		"stale — last updated: 2024-10-22 13:39:22",
		"context deadline exceeded",
		"[0] NVIDIA GeForce RTX 3060 │ 45% │ 6349/12288 MiB │ 62°C",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestMetrics_Snapshot80Cols(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var counter tokens.SessionCounter
	counter.Add(1240, 890)

	got := RenderMetrics(MetricsData{
		Width:  testMetricsWidth,
		Height: 16,
		Tokens: counter,
		GPU: gpu.Snapshot{
			Available: true,
			Devices: []gpu.Device{
				{
					Index:       0,
					Name:        "NVIDIA GeForce RTX 3060",
					Utilization: 45,
					MemoryUsed:  6349,
					MemoryTotal: 12288,
					Temperature: 62,
				},
			},
			FetchedAt: time.Date(2024, 10, 22, 13, 39, 22, 0, time.UTC),
		},
	})

	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "╭") || strings.HasPrefix(line, "│") ||
			strings.HasPrefix(line, "╰") {
			continue
		}
		if line == "" {
			continue
		}
		_ = i
	}

	if !strings.Contains(got, "Session Tokens") {
		t.Fatal("expected Session Tokens panel")
	}
	if !strings.Contains(got, "GPU") {
		t.Fatal("expected GPU panel")
	}
	if !strings.Contains(got, "1,240") {
		t.Fatal("expected formatted prompt tokens")
	}
	if !strings.Contains(got, "RTX 3060") {
		t.Fatal("expected GPU device name")
	}
}
