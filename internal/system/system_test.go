package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const sampleStat = `cpu  100 0 200 700 0 0 0 0 0 0
cpu0 50 0 100 350 0 0 0 0 0 0
intr 12345
ctxt 9876
`

const sampleStat2 = `cpu  200 0 400 1000 0 0 0 0 0 0
cpu0 100 0 200 500 0 0 0 0 0 0
intr 12500
ctxt 10000
`

const sampleMeminfo = `MemTotal:       16000000 kB
MemFree:         1000000 kB
MemAvailable:   10000000 kB
Buffers:          200000 kB
Cached:          5000000 kB
SwapCached:         3000 kB
SwapTotal:       8000000 kB
SwapFree:        7500000 kB
`

const sampleLoad = "0.42 0.30 0.25 3/123 7777\n"

func writeProcFiles(t *testing.T, stat, meminfo, load string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	statPath := filepath.Join(dir, "stat")
	memPath := filepath.Join(dir, "meminfo")
	loadPath := filepath.Join(dir, "loadavg")

	if err := os.WriteFile(statPath, []byte(stat), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memPath, []byte(meminfo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loadPath, []byte(load), 0o644); err != nil {
		t.Fatal(err)
	}
	return statPath, memPath, loadPath
}

func fakeStatfs(_ string, stat *syscall.Statfs_t) error {
	stat.Blocks = 1000 // 1000 blocks
	stat.Bavail = 250  // 250 free
	stat.Bsize = 4096  // 4 KiB blocks
	return nil
}

func errorStatfs(_ string, _ *syscall.Statfs_t) error {
	return errors.New("statfs failed")
}

func TestMonitor_QueryFirstCallNoBaseline(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)

	m := &Monitor{
		DiskTarget:  "/",
		procStat:    stat,
		procMeminfo: mem,
		procLoad:    load,
		statfsFn:    fakeStatfs,
	}

	snap := m.Query(context.Background())
	if !snap.Available {
		t.Fatalf("expected Available=true, got error=%q", snap.Error)
	}
	if snap.CPU.HasBaseline {
		t.Error("first Query should not have CPU baseline")
	}
	if snap.CPU.Utilization != 0 {
		t.Errorf("CPU Util = %f, want 0 on first call", snap.CPU.Utilization)
	}
	if snap.CPU.Cores <= 0 {
		t.Error("expected positive Cores")
	}
}

func TestMonitor_QuerySecondCallComputesUtilization(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)

	m := &Monitor{
		DiskTarget:  "/",
		procStat:    stat,
		procMeminfo: mem,
		procLoad:    load,
		statfsFn:    fakeStatfs,
	}

	_ = m.Query(context.Background())

	if err := os.WriteFile(stat, []byte(sampleStat2), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := m.Query(context.Background())
	if !snap.CPU.HasBaseline {
		t.Fatal("expected HasBaseline=true on second call")
	}
	// total delta = (200+400+1000) - (100+200+700) = 1600-1000 = 600
	// idle delta = 1000 - 700 = 300
	// busy = 300 ⇒ util = 50%
	if snap.CPU.Utilization < 49 || snap.CPU.Utilization > 51 {
		t.Errorf("CPU Util = %f, want ~50", snap.CPU.Utilization)
	}
}

func TestMonitor_MemoryParsing(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)
	m := &Monitor{procStat: stat, procMeminfo: mem, procLoad: load, statfsFn: fakeStatfs}

	snap := m.Query(context.Background())
	if !snap.Available {
		t.Fatalf("not available: %s", snap.Error)
	}
	if snap.Memory.TotalMiB == 0 || snap.Memory.AvailableMiB == 0 {
		t.Fatalf("expected non-zero memory: %+v", snap.Memory)
	}
	if snap.Memory.SwapTotalMiB == 0 || snap.Memory.SwapUsedMiB == 0 {
		t.Fatalf("expected swap stats: %+v", snap.Memory)
	}
}

func TestMonitor_LoadAvg(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)
	m := &Monitor{procStat: stat, procMeminfo: mem, procLoad: load, statfsFn: fakeStatfs}

	snap := m.Query(context.Background())
	if snap.Load.One < 0.41 || snap.Load.One > 0.43 {
		t.Errorf("Load.One = %f, want ~0.42", snap.Load.One)
	}
	if snap.Load.Procs != 123 {
		t.Errorf("Load.Procs = %d, want 123", snap.Load.Procs)
	}
}

func TestMonitor_DiskUsage(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)
	m := &Monitor{procStat: stat, procMeminfo: mem, procLoad: load, statfsFn: fakeStatfs}

	snap := m.Query(context.Background())
	// 1000 blocks * 4096 = 4_096_000 bytes total ⇒ <4 MiB
	if snap.Disk.TotalMiB != 4096000/(1024*1024) {
		t.Errorf("Disk.TotalMiB = %d", snap.Disk.TotalMiB)
	}
}

func TestMonitor_DiskError(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)
	m := &Monitor{procStat: stat, procMeminfo: mem, procLoad: load, statfsFn: errorStatfs}

	snap := m.Query(context.Background())
	if snap.Available {
		t.Fatal("expected Available=false on disk error")
	}
	if snap.Error == "" {
		t.Fatal("expected Error on disk failure")
	}
}

func TestMonitor_ContextCancelled(t *testing.T) {
	stat, mem, load := writeProcFiles(t, sampleStat, sampleMeminfo, sampleLoad)
	m := &Monitor{procStat: stat, procMeminfo: mem, procLoad: load, statfsFn: fakeStatfs}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := m.Query(ctx)
	if snap.Available {
		t.Fatal("expected Available=false on cancellation")
	}
}

func TestNewMonitor_DefaultsLiveHost(t *testing.T) {
	m := NewMonitor()
	if m.procStat != "/proc/stat" || m.DiskTarget != "/" {
		t.Fatalf("unexpected defaults: %+v", m)
	}

	// best-effort live query — should not panic; may report unavailable in sandboxes
	snap := m.Query(context.Background())
	_ = snap // intentionally ignore Available; live host may vary
	if !snap.FetchedAt.After(time.Time{}) {
		t.Fatal("expected FetchedAt populated")
	}
}
