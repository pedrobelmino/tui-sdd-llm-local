package gpu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func loadFixture(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "nvidia_smi.csv"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func TestParseCSV_FromFixture(t *testing.T) {
	devices, err := parseCSV(loadFixture(t))
	if err != nil {
		t.Fatalf("parseCSV: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	want0 := Device{
		Index:       0,
		Name:        "NVIDIA GeForce RTX 3090",
		Vendor:      VendorNVIDIA,
		Utilization: 45,
		MemoryUsed:  8192,
		MemoryTotal: 24576,
		Temperature: 65,
	}
	if devices[0] != want0 {
		t.Errorf("device[0] = %+v, want %+v", devices[0], want0)
	}

	want1 := Device{
		Index:       1,
		Name:        "NVIDIA GeForce GTX 1080 Ti",
		Vendor:      VendorNVIDIA,
		Utilization: 12,
		MemoryUsed:  2048,
		MemoryTotal: 11264,
		Temperature: 55,
	}
	if devices[1] != want1 {
		t.Errorf("device[1] = %+v, want %+v", devices[1], want1)
	}
}

func TestParseCSV_EmptyInput(t *testing.T) {
	devices, err := parseCSV("  \n  ")
	if err != nil {
		t.Fatalf("parseCSV: %v", err)
	}
	if devices != nil {
		t.Fatalf("expected nil devices, got %#v", devices)
	}
}

func TestParseCSV_InvalidFieldCount(t *testing.T) {
	_, err := parseCSV("0, GPU Name, 10, 1000")
	if err == nil {
		t.Fatal("expected parse error for short row")
	}
}

func TestQueryNVIDIA_LookPathFailure(t *testing.T) {
	orig := lookPath
	lookPath = func(string) (string, error) {
		return "", errors.New("nvidia-smi not found")
	}
	t.Cleanup(func() { lookPath = orig })

	snap, err := queryNVIDIALegacy(context.Background())
	if err != nil {
		t.Fatalf("queryNVIDIALegacy: %v", err)
	}
	if snap.Available {
		t.Error("expected Available=false when nvidia-smi is missing")
	}
	if snap.Error != "" {
		t.Errorf("expected empty Error, got %q", snap.Error)
	}
}

func TestQueryNVIDIA_CommandFailure(t *testing.T) {
	origLook := lookPath
	origRun := runNvidiaSMI
	lookPath = func(string) (string, error) { return "/usr/bin/nvidia-smi", nil }
	runNvidiaSMI = func(context.Context, string) ([]byte, error) {
		return nil, errors.New("driver error")
	}
	t.Cleanup(func() {
		lookPath = origLook
		runNvidiaSMI = origRun
	})

	snap, err := queryNVIDIALegacy(context.Background())
	if err != nil {
		t.Fatalf("queryNVIDIALegacy: %v", err)
	}
	if snap.Available {
		t.Error("expected Available=false on command failure")
	}
	if snap.Error == "" {
		t.Error("expected Error to describe command failure")
	}
}

func TestQueryNVIDIA_SuccessFromFixture(t *testing.T) {
	fixture := loadFixture(t)

	origLook := lookPath
	origRun := runNvidiaSMI
	lookPath = func(string) (string, error) { return "/usr/bin/nvidia-smi", nil }
	runNvidiaSMI = func(context.Context, string) ([]byte, error) {
		return []byte(fixture), nil
	}
	t.Cleanup(func() {
		lookPath = origLook
		runNvidiaSMI = origRun
	})

	snap, err := queryNVIDIALegacy(context.Background())
	if err != nil {
		t.Fatalf("queryNVIDIALegacy: %v", err)
	}
	if !snap.Available {
		t.Fatal("expected Available=true")
	}
	if len(snap.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(snap.Devices))
	}
	if snap.Error != "" {
		t.Errorf("expected empty Error, got %q", snap.Error)
	}
	if snap.FetchedAt.IsZero() {
		t.Error("expected FetchedAt to be set")
	}
}

func TestQueryNVIDIA_HonorsThreeSecondTimeout(t *testing.T) {
	origLook := lookPath
	origRun := runNvidiaSMI
	lookPath = func(string) (string, error) { return "/usr/bin/nvidia-smi", nil }
	runNvidiaSMI = func(ctx context.Context, _ string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("missing deadline")
		}

		remaining := time.Until(deadline)
		if remaining < 2*time.Second || remaining > queryTimeout {
			return nil, errors.New("unexpected deadline")
		}

		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() {
		lookPath = origLook
		runNvidiaSMI = origRun
	})

	start := time.Now()
	snap, err := queryNVIDIALegacy(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("queryNVIDIALegacy: %v", err)
	}
	if snap.Available {
		t.Error("expected Available=false on timeout")
	}
	if snap.Error == "" {
		t.Error("expected Error on timeout")
	}
	if elapsed < 2*time.Second || elapsed > 4*time.Second {
		t.Errorf("expected ~3s timeout, elapsed %v", elapsed)
	}
}
