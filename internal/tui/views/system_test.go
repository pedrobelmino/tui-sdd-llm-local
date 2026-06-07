package views

import (
	"strings"
	"testing"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
)

func TestRenderSystem_Unavailable(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderSystem(SystemData{
		Width:  80,
		Height: 16,
		System: system.Snapshot{Available: false, Error: "boom"},
	})
	if !strings.Contains(out, msgSystemUnavailable) {
		t.Fatalf("expected %q in output: %s", msgSystemUnavailable, out)
	}
	if !strings.Contains(out, "boom") {
		t.Fatalf("expected error detail in output: %s", out)
	}
}

func TestRenderSystem_AllPanelsPresent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	snap := system.Snapshot{
		Available: true,
		CPU: system.CPUInfo{
			Cores: 8, Utilization: 42.5, HasBaseline: true,
		},
		Memory: system.MemoryInfo{
			TotalMiB: 16000, UsedMiB: 8000, AvailableMiB: 8000,
			SwapTotalMiB: 4000, SwapUsedMiB: 500,
		},
		Load: system.LoadAvg{One: 0.42, Five: 0.30, Fifteen: 0.25, Procs: 123},
		Disk: system.DiskInfo{
			Mountpoint: "/", TotalMiB: 200000, UsedMiB: 150000, AvailMiB: 50000,
		},
	}

	out := RenderSystem(SystemData{Width: 80, Height: 20, System: snap})

	for _, want := range []string{
		systemPanelTitle,
		loadPanelTitle,
		diskPanelTitle,
		"CPU:",
		"Memory:",
		"Swap:",
		"cores=8",
		"42.5%",
		"1m: 0.42",
		"procs: 123",
		"/:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestRenderSystem_FirstSampleNoBaseline(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderSystem(SystemData{
		Width: 80, Height: 16,
		System: system.Snapshot{
			Available: true,
			CPU:       system.CPUInfo{Cores: 4, HasBaseline: false},
			Memory:    system.MemoryInfo{TotalMiB: 4000, UsedMiB: 1000, AvailableMiB: 3000},
			Disk:      system.DiskInfo{Mountpoint: "/", TotalMiB: 100, UsedMiB: 50, AvailMiB: 50},
		},
	})
	if !strings.Contains(out, "sampling") {
		t.Fatalf("expected sampling hint when no baseline: %s", out)
	}
}

func TestRenderBar_PercentClamped(t *testing.T) {
	if got := renderBar(150, 10); !strings.Contains(got, "100.0%") {
		t.Errorf("expected clamp to 100, got %q", got)
	}
	if got := renderBar(-10, 10); !strings.Contains(got, "0.0%") {
		t.Errorf("expected clamp to 0, got %q", got)
	}
}

func TestFormatMiB(t *testing.T) {
	if got := formatMiB(512); got != "512 MiB" {
		t.Errorf("formatMiB(512) = %q", got)
	}
	if got := formatMiB(2048); got != "2.0 GiB" {
		t.Errorf("formatMiB(2048) = %q", got)
	}
}
