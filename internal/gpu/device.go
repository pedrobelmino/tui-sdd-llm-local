package gpu

import "time"

// Vendor identifies the GPU metrics source.
type Vendor string

const (
	VendorAMD    Vendor = "amd"
	VendorNVIDIA Vendor = "nvidia"
)

// Device holds metrics for a single GPU.
type Device struct {
	Index       int
	Name        string
	Vendor      Vendor
	Utilization float64 // percent
	MemoryUsed  uint64  // MiB
	MemoryTotal uint64  // MiB
	Temperature float64 // Celsius; 0 if unknown
}

// Snapshot is the result of a GPU metrics query.
type Snapshot struct {
	Devices   []Device
	Available bool
	Vendor    Vendor
	Error     string
	Stale     bool
	FetchedAt time.Time
}
