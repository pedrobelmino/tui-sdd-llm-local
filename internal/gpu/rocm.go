package gpu

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const rocmBinary = "rocm-smi"

var runROCmSMI = defaultRunROCmSMI

func defaultRunROCmSMI(ctx context.Context) ([]byte, error) {
	path, err := exec.LookPath(rocmBinary)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, path, "--showuse", "--showmeminfo", "vram", "--showtemp", "--json")
	return cmd.Output()
}

// QueryROCm reads Radeon metrics via rocm-smi when amdgpu sysfs is unavailable.
func QueryROCm(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{FetchedAt: time.Now(), Vendor: VendorAMD}

	ctx, cancel := context.WithTimeout(ctx, queryTimeoutAMD)
	defer cancel()

	out, err := runROCmSMI(ctx)
	if err != nil {
		snap.Error = err.Error()
		return snap, nil
	}

	devices, err := parseROCmJSON(out)
	if err != nil {
		snap.Error = err.Error()
		return snap, nil
	}
	if len(devices) == 0 {
		snap.Error = "rocm-smi: no devices"
		return snap, nil
	}

	snap.Devices = devices
	snap.Available = true
	return snap, nil
}

func parseROCmJSON(data []byte) ([]Device, error) {
	// rocm-smi --json returns a map keyed by card id
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var devices []Device
	idx := 0
	for key, val := range raw {
		if !strings.HasPrefix(key, "card") {
			continue
		}
		var card struct {
			GPUUse    json.RawMessage `json:"GPU use (%)"`
			VRAMUsed  json.RawMessage `json:"VRAM Used Memory (B)"`
			VRAMTotal json.RawMessage `json:"VRAM Total Memory (B)"`
			Temp      json.RawMessage `json:"Temperature (Sensor edge) (C)"`
		}
		if err := json.Unmarshal(val, &card); err != nil {
			continue
		}

		util := parseROCmFloat(card.GPUUse)
		used := parseROCmUint(card.VRAMUsed) / (1024 * 1024)
		total := parseROCmUint(card.VRAMTotal) / (1024 * 1024)
		temp := parseROCmFloat(card.Temp)

		devices = append(devices, Device{
			Index:       idx,
			Name:        "AMD Radeon (" + key + ")",
			Vendor:      VendorAMD,
			Utilization: util,
			MemoryUsed:  used,
			MemoryTotal: total,
			Temperature: temp,
		})
		idx++
	}
	return devices, nil
}

func parseROCmFloat(raw json.RawMessage) float64 {
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == "N/A" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseROCmUint(raw json.RawMessage) uint64 {
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == "N/A" {
		return 0
	}
	u, _ := strconv.ParseUint(s, 10, 64)
	return u
}
