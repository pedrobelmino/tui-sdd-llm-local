package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveActionLogFile(t *testing.T) {
	path, err := saveActionLogFile("hello implement log")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello implement log" {
		t.Fatalf("content = %q", data)
	}
	home, _ := os.UserHomeDir()
	latest := filepath.Join(home, ".tsll", "last-action.log")
	ld, err := os.ReadFile(latest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ld), "hello implement log") {
		t.Fatalf("latest log missing content: %q", ld)
	}
}
