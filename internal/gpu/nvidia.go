package gpu

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	nvidiaBinary  = "nvidia-smi"
	queryTimeout  = 3 * time.Second
	nvidiaGPUArgs = "index,name,utilization.gpu,memory.used,memory.total,temperature.gpu"
)

var (
	lookPath     = exec.LookPath
	runNvidiaSMI = defaultRunNvidiaSMI
)

func defaultRunNvidiaSMI(ctx context.Context, path string) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		path,
		"--query-gpu="+nvidiaGPUArgs,
		"--format=csv,noheader,nounits",
	)
	return cmd.Output()
}

// queryNVIDIALegacy runs nvidia-smi and parses CSV output into a Snapshot.
func queryNVIDIALegacy(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{FetchedAt: time.Now(), Vendor: VendorNVIDIA}

	path, err := lookPath(nvidiaBinary)
	if err != nil {
		snap.Available = false
		return snap, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	out, err := runNvidiaSMI(queryCtx, path)
	if err != nil {
		snap.Available = false
		snap.Error = err.Error()
		return snap, nil
	}

	devices, err := parseCSV(string(out))
	if err != nil {
		snap.Available = false
		snap.Error = err.Error()
		return snap, nil
	}

	snap.Devices = devices
	snap.Available = true
	return snap, nil
}

func parseCSV(data string) ([]Device, error) {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil, nil
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse nvidia-smi CSV: %w", err)
	}

	devices := make([]Device, 0, len(records))
	for i, record := range records {
		if len(record) != 6 {
			return nil, fmt.Errorf("line %d: expected 6 fields, got %d", i+1, len(record))
		}

		device, err := parseDevice(record)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func parseDevice(fields []string) (Device, error) {
	index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil {
		return Device{}, fmt.Errorf("index: %w", err)
	}

	utilization, err := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
	if err != nil {
		return Device{}, fmt.Errorf("utilization: %w", err)
	}

	memoryUsed, err := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64)
	if err != nil {
		return Device{}, fmt.Errorf("memory.used: %w", err)
	}

	memoryTotal, err := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 64)
	if err != nil {
		return Device{}, fmt.Errorf("memory.total: %w", err)
	}

	temperature, err := strconv.ParseFloat(strings.TrimSpace(fields[5]), 64)
	if err != nil {
		return Device{}, fmt.Errorf("temperature: %w", err)
	}

	return Device{
		Index:       index,
		Name:        strings.TrimSpace(fields[1]),
		Vendor:      VendorNVIDIA,
		Utilization: utilization,
		MemoryUsed:  memoryUsed,
		MemoryTotal: memoryTotal,
		Temperature: temperature,
	}, nil
}
