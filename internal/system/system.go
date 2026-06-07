// Package system reads local machine resource metrics from /proc and statfs.
// Linux only. Pure stdlib — no cgo, no external deps.
package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Snapshot is a point-in-time view of host resources.
type Snapshot struct {
	CPU       CPUInfo
	Memory    MemoryInfo
	Disk      DiskInfo
	Load      LoadAvg
	Available bool
	Error     string
	FetchedAt time.Time
}

// CPUInfo aggregates current CPU utilization in percent.
// Utilization is measured between two consecutive Query calls; the first call
// returns Utilization=0 with HasBaseline=false.
type CPUInfo struct {
	Cores       int
	Utilization float64
	HasBaseline bool
}

// MemoryInfo holds RAM and swap usage in MiB.
type MemoryInfo struct {
	TotalMiB     uint64
	UsedMiB      uint64
	AvailableMiB uint64
	SwapTotalMiB uint64
	SwapUsedMiB  uint64
}

// DiskInfo describes a single mountpoint usage in MiB.
type DiskInfo struct {
	Mountpoint string
	TotalMiB   uint64
	UsedMiB    uint64
	AvailMiB   uint64
}

// LoadAvg captures 1/5/15-minute load.
type LoadAvg struct {
	One     float64
	Five    float64
	Fifteen float64
	Procs   int
}

// Monitor is a stateful sampler that derives CPU utilization between calls.
type Monitor struct {
	prevTotal uint64
	prevIdle  uint64
	hasPrev   bool

	// DiskTarget is the mountpoint to report. Default: "/".
	DiskTarget string

	// procStat / procMeminfo / procLoad are overridable for tests.
	procStat    string
	procMeminfo string
	procLoad    string
	statfsFn    func(path string, stat *syscall.Statfs_t) error
}

// NewMonitor returns a Monitor configured for the live Linux host.
func NewMonitor() *Monitor {
	return &Monitor{
		DiskTarget:  "/",
		procStat:    "/proc/stat",
		procMeminfo: "/proc/meminfo",
		procLoad:    "/proc/loadavg",
		statfsFn:    syscall.Statfs,
	}
}

// Query collects a new system snapshot.
func (m *Monitor) Query(ctx context.Context) Snapshot {
	snap := Snapshot{FetchedAt: time.Now()}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		snap.Error = ctx.Err().Error()
		return snap
	default:
	}

	cpu, err := m.readCPU()
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.CPU = cpu

	mem, err := readMeminfo(m.procMeminfo)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.Memory = mem

	load, err := readLoad(m.procLoad)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.Load = load

	disk, err := readDisk(m.statfsFn, m.DiskTarget)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.Disk = disk

	snap.Available = true
	return snap
}

func (m *Monitor) readCPU() (CPUInfo, error) {
	total, idle, err := readCPUTotals(m.procStat)
	if err != nil {
		return CPUInfo{Cores: runtime.NumCPU()}, err
	}

	info := CPUInfo{Cores: runtime.NumCPU()}
	if m.hasPrev {
		deltaTotal := total - m.prevTotal
		deltaIdle := idle - m.prevIdle
		if deltaTotal > 0 {
			busy := deltaTotal - deltaIdle
			info.Utilization = float64(busy) / float64(deltaTotal) * 100.0
		}
		info.HasBaseline = true
	}
	m.prevTotal = total
	m.prevIdle = idle
	m.hasPrev = true
	return info, nil
}

func readCPUTotals(path string) (uint64, uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, fmt.Errorf("%s: empty", path)
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") && !strings.HasPrefix(line, "cpu\t") {
		return 0, 0, fmt.Errorf("%s: missing aggregate cpu line", path)
	}

	fields := strings.Fields(line)[1:]
	var total uint64
	var idle uint64
	for i, f := range fields {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("%s: parse field %d: %w", path, i, err)
		}
		total += v
		// idle = fields[3] (idle) + fields[4] (iowait, optional)
		if i == 3 {
			idle += v
		}
		if i == 4 {
			idle += v
		}
	}
	return total, idle, nil
}

func readMeminfo(path string) (MemoryInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return MemoryInfo{}, err
	}
	defer f.Close()

	values := map[string]uint64{}
	wanted := map[string]bool{
		"MemTotal":     true,
		"MemAvailable": true,
		"MemFree":      true,
		"Buffers":      true,
		"Cached":       true,
		"SwapTotal":    true,
		"SwapFree":     true,
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		if !wanted[key] {
			continue
		}
		rest := strings.TrimSpace(line[idx+1:])
		rest = strings.TrimSuffix(rest, " kB")
		v, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			continue
		}
		values[key] = v
	}
	if err := scanner.Err(); err != nil {
		return MemoryInfo{}, err
	}

	total := kibToMiB(values["MemTotal"])
	avail := kibToMiB(values["MemAvailable"])
	if avail == 0 {
		// Older kernels: fallback approximation.
		avail = kibToMiB(values["MemFree"] + values["Buffers"] + values["Cached"])
	}
	used := uint64(0)
	if total > avail {
		used = total - avail
	}

	swapTotal := kibToMiB(values["SwapTotal"])
	swapFree := kibToMiB(values["SwapFree"])
	swapUsed := uint64(0)
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}

	return MemoryInfo{
		TotalMiB:     total,
		UsedMiB:      used,
		AvailableMiB: avail,
		SwapTotalMiB: swapTotal,
		SwapUsedMiB:  swapUsed,
	}, nil
}

func readLoad(path string) (LoadAvg, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LoadAvg{}, err
	}
	parts := strings.Fields(string(data))
	if len(parts) < 4 {
		return LoadAvg{}, fmt.Errorf("%s: malformed loadavg", path)
	}
	la := LoadAvg{}
	la.One, _ = strconv.ParseFloat(parts[0], 64)
	la.Five, _ = strconv.ParseFloat(parts[1], 64)
	la.Fifteen, _ = strconv.ParseFloat(parts[2], 64)
	// parts[3] is "running/total"; we take total.
	if slash := strings.Index(parts[3], "/"); slash >= 0 {
		la.Procs, _ = strconv.Atoi(parts[3][slash+1:])
	}
	return la, nil
}

func readDisk(statfs func(string, *syscall.Statfs_t) error, target string) (DiskInfo, error) {
	if target == "" {
		target = "/"
	}
	var stat syscall.Statfs_t
	if err := statfs(target, &stat); err != nil {
		return DiskInfo{Mountpoint: target}, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	used := total - avail
	return DiskInfo{
		Mountpoint: target,
		TotalMiB:   total / (1024 * 1024),
		UsedMiB:    used / (1024 * 1024),
		AvailMiB:   avail / (1024 * 1024),
	}, nil
}

func kibToMiB(kib uint64) uint64 {
	return kib / 1024
}
