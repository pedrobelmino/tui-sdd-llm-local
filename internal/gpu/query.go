package gpu

import (
	"context"
	"os"
	"strings"
)

var (
	queryAMD    = QueryAMD
	queryNVIDIA = queryNVIDIAInner
)

// Query returns GPU metrics. Default order: AMD/Radeon (amdgpu → rocm-smi) then NVIDIA.
// Set TSLL_GPU_PREFER=nvidia to try NVIDIA first.
func Query(ctx context.Context) (Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	prefer := strings.ToLower(os.Getenv("TSLL_GPU_PREFER"))
	if prefer == "" {
		prefer = "amd"
	}

	if prefer == "nvidia" {
		if snap, err := tryNVIDIA(ctx); snap.Available {
			return snap, err
		}
		return tryAMD(ctx)
	}
	return tryAMD(ctx)
}

func tryAMD(ctx context.Context) (Snapshot, error) {
	if amd, err := queryAMD(ctx); err == nil && amd.Available && len(amd.Devices) > 0 {
		return amd, nil
	}
	if rocm, err := QueryROCm(ctx); err == nil && rocm.Available && len(rocm.Devices) > 0 {
		return rocm, nil
	}
	return queryNVIDIA(ctx)
}

func tryNVIDIA(ctx context.Context) (Snapshot, error) {
	return queryNVIDIA(ctx)
}

func queryNVIDIAInner(ctx context.Context) (Snapshot, error) {
	return queryNVIDIALegacy(ctx)
}
