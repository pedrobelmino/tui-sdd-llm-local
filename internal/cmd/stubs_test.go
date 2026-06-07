package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetRootArgs prepares rootCmd for a deterministic test run.
func resetRootArgs(args ...string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	rootCmd.SetArgs(args)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	return buf
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

func TestInit_CreatesProjectFiles(t *testing.T) {
	dir := chdirTemp(t)

	resetRootArgs("init", "--yes", "--name", "testproj", "--vision", "test vision")
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	for _, f := range []string{".specs/project/PROJECT.md", ".specs/project/ROADMAP.md", ".specs/project/STATE.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
}

func TestSpecify_RequiresProject(t *testing.T) {
	chdirTemp(t)

	resetRootArgs("specify", "myfeature", "--brief", "test")
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "tsll init") {
		t.Fatalf("expected project error, got %v", err)
	}
}

func TestTasks_RequiresProject(t *testing.T) {
	chdirTemp(t)

	resetRootArgs("tasks", "myfeature")
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "tsll init") {
		t.Fatalf("expected project error, got %v", err)
	}
}

func TestRun_RequiresProject(t *testing.T) {
	chdirTemp(t)

	resetRootArgs("run", "myfeature", "--task", "T1")
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "tsll init") {
		t.Fatalf("expected project error, got %v", err)
	}
}

func TestValidate_RequiresProject(t *testing.T) {
	chdirTemp(t)

	resetRootArgs("validate", "feat")
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
