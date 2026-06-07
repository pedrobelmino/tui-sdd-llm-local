package gpu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	amdVendorID     = "0x1002"
	amdgpuDriver    = "amdgpu"
	sysfsDRMRelay   = "/sys/class/drm"
	queryTimeoutAMD = 3 * time.Second
)

var (
	readFileFn   = os.ReadFile
	globFn       = filepath.Glob
	evalSymlinks = filepath.EvalSymlinks
	runLspci     = defaultRunLspci
)

func defaultRunLspci(ctx context.Context, slot string) (string, error) {
	path, err := exec.LookPath("lspci")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, path, "-s", slot, "-mm")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// QueryAMD reads Radeon GPU metrics via amdgpu sysfs (primary for Linux Radeon).
func QueryAMD(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{FetchedAt: time.Now(), Vendor: VendorAMD}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, queryTimeoutAMD)
		defer cancel()
	}

	cards, err := discoverAMDGPUs()
	if err != nil {
		snap.Error = err.Error()
		return snap, nil
	}
	if len(cards) == 0 {
		snap.Error = "no amdgpu devices found"
		return snap, nil
	}

	devices := make([]Device, 0, len(cards))
	for i, cardPath := range cards {
		select {
		case <-ctx.Done():
			snap.Error = ctx.Err().Error()
			return snap, nil
		default:
		}

		dev, err := readAMDDevice(cardPath, i)
		if err != nil {
			snap.Error = err.Error()
			continue
		}
		devices = append(devices, dev)
	}

	if len(devices) == 0 {
		return snap, nil
	}

	snap.Devices = devices
	snap.Available = true
	return snap, nil
}

func discoverAMDGPUs() ([]string, error) {
	matches, err := globFn(filepath.Join(sysfsDRMRelay, "card[0-9]"))
	if err != nil {
		return nil, err
	}

	var cards []string
	for _, card := range matches {
		deviceDir := filepath.Join(card, "device")
		if !isAMDGPU(deviceDir) {
			continue
		}
		cards = append(cards, deviceDir)
	}
	return cards, nil
}

func isAMDGPU(deviceDir string) bool {
	vendorPath := filepath.Join(deviceDir, "vendor")
	data, err := readFileFn(vendorPath)
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(data)) != amdVendorID {
		return false
	}

	uevent, err := readFileFn(filepath.Join(deviceDir, "uevent"))
	if err == nil && strings.Contains(string(uevent), "DRIVER="+amdgpuDriver) {
		return true
	}

	// vendor match is enough for most Radeon on amdgpu
	return true
}

func readAMDDevice(deviceDir string, index int) (Device, error) {
	name := readAMDName(deviceDir)

	util, _ := readUintFromFile(filepath.Join(deviceDir, "gpu_busy_percent"))
	memUsed, _ := readUintFromFile(filepath.Join(deviceDir, "mem_info_vram_used"))
	memTotal, _ := readUintFromFile(filepath.Join(deviceDir, "mem_info_vram_total"))

	temp := readAMDTemp(deviceDir)

	return Device{
		Index:       index,
		Name:        name,
		Vendor:      VendorAMD,
		Utilization: float64(util),
		MemoryUsed:  memUsed / (1024 * 1024),
		MemoryTotal: memTotal / (1024 * 1024),
		Temperature: temp,
	}, nil
}

func readAMDName(deviceDir string) string {
	if data, err := readFileFn(filepath.Join(deviceDir, "product_name")); err == nil {
		name := strings.TrimSpace(string(data))
		if name != "" {
			return name
		}
	}
	if name := lspciDeviceName(deviceDir); name != "" {
		return name
	}
	if data, err := readFileFn(filepath.Join(deviceDir, "name")); err == nil {
		if n := strings.TrimSpace(string(data)); n != "" {
			return n
		}
	}
	return "AMD Radeon"
}

// lspciDeviceName runs `lspci -s <slot> -mm` to recover the marketing name.
// Returns "" when lspci is unavailable.
func lspciDeviceName(deviceDir string) string {
	slot := pciSlotFromSysfs(deviceDir)
	if slot == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := runLspci(ctx, slot)
	if err != nil {
		return ""
	}
	return parseLspciMM(out)
}

func pciSlotFromSysfs(deviceDir string) string {
	resolved, err := evalSymlinks(deviceDir)
	if err != nil {
		return ""
	}
	// e.g. /sys/devices/pci0000:00/0000:00:01.0/0000:01:00.0
	base := filepath.Base(resolved)
	if len(base) >= 7 && strings.Count(base, ":") >= 1 {
		return base
	}
	return ""
}

// parseLspciMM extracts the third quoted field (device name) from
// `lspci -mm` output. It strips a trailing 4-hex PCI ID (e.g. "[699f]")
// while preserving descriptive brackets like "[Radeon RX 6700]".
func parseLspciMM(out string) string {
	line := strings.TrimSpace(out)
	if line == "" {
		return ""
	}
	fields := splitLspciFields(line)
	if len(fields) < 3 {
		return ""
	}
	return stripPCIID(fields[2])
}

func stripPCIID(name string) string {
	name = strings.TrimSpace(name)
	if !strings.HasSuffix(name, "]") {
		return name
	}
	open := strings.LastIndex(name, "[")
	if open < 0 {
		return name
	}
	id := name[open+1 : len(name)-1]
	if !isHex4(id) {
		return name
	}
	return strings.TrimSpace(name[:open])
}

func isHex4(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'f',
			r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// splitLspciFields tokenizes the -mm format which uses quoted strings.
func splitLspciFields(s string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	for _, r := range s {
		switch r {
		case '"':
			if inQuote {
				fields = append(fields, current.String())
				current.Reset()
			}
			inQuote = !inQuote
		default:
			if inQuote {
				current.WriteRune(r)
			}
		}
	}
	return fields
}

func readAMDTemp(deviceDir string) float64 {
	matches, _ := globFn(filepath.Join(deviceDir, "hwmon", "hwmon*", "temp1_input"))
	for _, p := range matches {
		if v, err := readUintFromFile(p); err == nil && v > 0 {
			return float64(v) / 1000.0
		}
	}
	return 0
}

func readUintFromFile(path string) (uint64, error) {
	data, err := readFileFn(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	if s == "" || s == "N/A" {
		return 0, fmt.Errorf("empty value")
	}
	return strconv.ParseUint(s, 10, 64)
}
