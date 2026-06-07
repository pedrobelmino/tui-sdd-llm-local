package cmd

import (
	"os"
	"testing"
)

func TestShouldLaunchTUI_TSLL_TUI_Zero(t *testing.T) {
	t.Setenv("TSLL_TUI", "0")
	if ShouldLaunchTUI() {
		t.Fatal("expected false when TSLL_TUI=0")
	}
}

func TestShouldLaunchTUI_Default(t *testing.T) {
	t.Setenv("TSLL_TUI", "")
	// stdout in tests is usually not a TTY
	_ = ShouldLaunchTUI()
}

func TestVersionSet(t *testing.T) {
	if rootCmd.Version == "" {
		t.Fatal("version not set")
	}
}

func TestRootCmd_HasSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"init", "specify", "tasks", "run"} {
		if !names[want] {
			t.Fatalf("missing subcommand %s", want)
		}
	}
}

func TestPlainError_WritesToStderr(t *testing.T) {
	// smoke: does not panic
	PlainError("test error")
}

func TestExecute_Help(t *testing.T) {
	t.Setenv("TSLL_TUI", "0")
	rootCmd.SetArgs([]string{"--help"})
	// help prints and may return nil
	_ = Execute()
}

func TestExecute_Version(t *testing.T) {
	rootCmd.SetArgs([]string{"--version"})
	if err := Execute(); err != nil {
		t.Fatalf("version: %v", err)
	}
}

func init() {
	// ensure non-interactive in package tests unless overridden
	if os.Getenv("TSLL_TUI") == "" {
		os.Setenv("TSLL_TUI", "0")
	}
}
