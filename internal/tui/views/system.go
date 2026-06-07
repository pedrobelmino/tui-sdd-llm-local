package views

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const (
	systemPanelTitle = "System Resources"
	loadPanelTitle   = "Load Average"
	diskPanelTitle   = "Disk"

	msgSystemUnavailable = "system metrics unavailable"
)

// SystemData is the subset of state needed to render the System view.
type SystemData struct {
	Width  int
	Height int
	System system.Snapshot
}

// RenderSystem renders the System view body with CPU, memory, swap, load and disk.
func RenderSystem(d SystemData) string {
	width := d.Width
	if width < 1 {
		width = 80
	}

	if !d.System.Available {
		lines := []string{msgSystemUnavailable}
		if d.System.Error != "" {
			lines = append(lines, d.System.Error)
		}
		return ui.Panel(systemPanelTitle, strings.Join(lines, "\n"), width, 5)
	}

	cpuMemContent := renderCPUMemoryContent(d.System)
	loadContent := renderLoadContent(d.System.Load)
	diskContent := renderDiskContent(d.System.Disk)

	cpuMemPanel := ui.Panel(systemPanelTitle, cpuMemContent, width, contentPanelHeight(cpuMemContent))
	loadPanel := ui.Panel(loadPanelTitle, loadContent, width, contentPanelHeight(loadContent))
	diskPanel := ui.Panel(diskPanelTitle, diskContent, width, contentPanelHeight(diskContent))

	return strings.Join([]string{cpuMemPanel, loadPanel, diskPanel}, "\n")
}

func renderCPUMemoryContent(snap system.Snapshot) string {
	cpu := snap.CPU
	mem := snap.Memory

	cpuLine := fmt.Sprintf("CPU:     %s  cores=%d", renderBar(cpu.Utilization, 20), cpu.Cores)
	if !cpu.HasBaseline {
		cpuLine = fmt.Sprintf("CPU:     %s  cores=%d  (sampling…)", renderBar(0, 20), cpu.Cores)
	}

	memPct := percent(mem.UsedMiB, mem.TotalMiB)
	memLine := fmt.Sprintf(
		"Memory:  %s  %s / %s used",
		renderBar(memPct, 20),
		formatMiB(mem.UsedMiB), formatMiB(mem.TotalMiB),
	)
	swapPct := percent(mem.SwapUsedMiB, mem.SwapTotalMiB)
	swapLine := fmt.Sprintf(
		"Swap:    %s  %s / %s",
		renderBar(swapPct, 20),
		formatMiB(mem.SwapUsedMiB), formatMiB(mem.SwapTotalMiB),
	)
	return strings.Join([]string{cpuLine, memLine, swapLine}, "\n")
}

func renderLoadContent(load system.LoadAvg) string {
	return fmt.Sprintf(
		"1m: %.2f   5m: %.2f   15m: %.2f   procs: %d",
		load.One, load.Five, load.Fifteen, load.Procs,
	)
}

func renderDiskContent(disk system.DiskInfo) string {
	pct := percent(disk.UsedMiB, disk.TotalMiB)
	mount := disk.Mountpoint
	if mount == "" {
		mount = "/"
	}
	return fmt.Sprintf(
		"%s: %s  %s / %s used  (%s free)",
		mount,
		renderBar(pct, 24),
		formatMiB(disk.UsedMiB),
		formatMiB(disk.TotalMiB),
		formatMiB(disk.AvailMiB),
	)
}

func renderBar(pct float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width) * pct / 100)
	if filled > width {
		filled = width
	}
	empty := width - filled
	return fmt.Sprintf("[%s%s] %5.1f%%",
		strings.Repeat("█", filled),
		strings.Repeat("·", empty),
		pct,
	)
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100.0
}

func formatMiB(mib uint64) string {
	if mib >= 1024 {
		return fmt.Sprintf("%.1f GiB", float64(mib)/1024.0)
	}
	return fmt.Sprintf("%d MiB", mib)
}

func contentPanelHeight(content string) int {
	lines := strings.Count(content, "\n") + 1
	return lines + 2
}
