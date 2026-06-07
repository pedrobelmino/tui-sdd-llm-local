package gpu

import (
	"context"
	"testing"
)

func TestReadAMDDevice_FromFixture(t *testing.T) {
	origRead := readFileFn
	defer func() { readFileFn = origRead }()

	readFileFn = func(path string) ([]byte, error) {
		switch path {
		case "/fixture/device/gpu_busy_percent":
			return []byte("55\n"), nil
		case "/fixture/device/mem_info_vram_used":
			return []byte("2147483648\n"), nil
		case "/fixture/device/mem_info_vram_total":
			return []byte("8589934592\n"), nil
		case "/fixture/device/product_name":
			return []byte("AMD Radeon RX 6700 XT\n"), nil
		default:
			return nil, errNotFound()
		}
	}

	dev, err := readAMDDevice("/fixture/device", 0)
	if err != nil {
		t.Fatal(err)
	}
	if dev.Vendor != VendorAMD {
		t.Fatalf("vendor: %s", dev.Vendor)
	}
	if dev.Utilization != 55 {
		t.Fatalf("util: %f", dev.Utilization)
	}
	if dev.MemoryUsed != 2048 {
		t.Fatalf("vram used: %d", dev.MemoryUsed)
	}
}

func TestIsAMDGPU_VendorMatch(t *testing.T) {
	origRead := readFileFn
	defer func() { readFileFn = origRead }()

	readFileFn = func(path string) ([]byte, error) {
		if path == "/card/device/vendor" {
			return []byte("0x1002\n"), nil
		}
		return []byte("DRIVER=amdgpu\n"), nil
	}

	if !isAMDGPU("/card/device") {
		t.Fatal("expected amd gpu")
	}
}

func TestQuery_PrefersAMDWhenAvailable(t *testing.T) {
	origAMD := queryAMD
	defer func() { queryAMD = origAMD }()

	queryAMD = func(ctx context.Context) (Snapshot, error) {
		return Snapshot{
			Available: true,
			Vendor:    VendorAMD,
			Devices:   []Device{{Name: "RX 6700", Vendor: VendorAMD, Utilization: 10}},
		}, nil
	}

	snap, err := Query(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Vendor != VendorAMD {
		t.Fatalf("expected amd, got %s", snap.Vendor)
	}
}

type pathError struct{}

func (pathError) Error() string   { return "not found" }
func (pathError) Timeout() bool   { return false }
func (pathError) Temporary() bool { return false }
func errNotFound() error          { return pathError{} }

func TestParseLspciMM_StripsTrailingPCIID(t *testing.T) {
	// "-mm -nn" output has trailing [hex] ID
	sample := `01:00.0 "VGA compatible controller [0300]" "Advanced Micro Devices, Inc. [AMD/ATI] [1002]" "Lexa PRO [Radeon 540/540X/550/550X / RX 540X/550/550X] [699f]" -rc7 -p00 "Advanced Micro Devices, Inc. [AMD/ATI] [1002]" "Device [0b04]"`
	got := parseLspciMM(sample)
	want := "Lexa PRO [Radeon 540/540X/550/550X / RX 540X/550/550X]"
	if got != want {
		t.Fatalf("parseLspciMM = %q, want %q", got, want)
	}
}

func TestParseLspciMM_PreservesDescriptiveBrackets(t *testing.T) {
	// plain "-mm" output (no PCI ID) preserves the bracketed marketing name
	sample := `01:00.0 "VGA compatible controller" "Advanced Micro Devices, Inc. [AMD/ATI]" "Lexa PRO [Radeon 540/540X/550/550X / RX 540X/550/550X]"`
	got := parseLspciMM(sample)
	want := "Lexa PRO [Radeon 540/540X/550/550X / RX 540X/550/550X]"
	if got != want {
		t.Fatalf("parseLspciMM = %q, want %q", got, want)
	}
}

func TestParseLspciMM_EmptyInput(t *testing.T) {
	if got := parseLspciMM(""); got != "" {
		t.Errorf("expected empty for empty input, got %q", got)
	}
	if got := parseLspciMM("bogus"); got != "" {
		t.Errorf("expected empty for malformed input, got %q", got)
	}
}

func TestReadAMDName_FallsBackToLspci(t *testing.T) {
	origRead := readFileFn
	origLspci := runLspci
	origEval := evalSymlinks
	defer func() {
		readFileFn = origRead
		runLspci = origLspci
		evalSymlinks = origEval
	}()

	readFileFn = func(path string) ([]byte, error) {
		// product_name absent, name absent → forces lspci path
		return nil, errNotFound()
	}
	evalSymlinks = func(p string) (string, error) {
		return "/sys/devices/pci0000:00/0000:01:00.0", nil
	}
	runLspci = func(ctx context.Context, slot string) (string, error) {
		if slot != "0000:01:00.0" {
			t.Fatalf("unexpected slot: %s", slot)
		}
		return `01:00.0 "VGA compatible controller [0300]" "AMD [1002]" "Navi 23 [Radeon RX 6600] [73ff]"`, nil
	}

	got := readAMDName("/sys/class/drm/card1/device")
	want := "Navi 23 [Radeon RX 6600]"
	if got != want {
		t.Fatalf("readAMDName = %q, want %q", got, want)
	}
}

func TestIsHex4(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"699f", true},
		{"73FF", true},
		{"0000", true},
		{"abcz", false},
		{"abc", false},
		{"12345", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isHex4(c.in); got != c.want {
			t.Errorf("isHex4(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
