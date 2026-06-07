package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindProject_InCwd(t *testing.T) {
	root := repoRoot(t)
	ctx, err := FindProject(root)
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if !ctx.Valid {
		t.Fatal("expected Valid=true")
	}
	if ctx.Root != root {
		t.Fatalf("Root = %q, want %q", ctx.Root, root)
	}
	if ctx.Corrupted {
		t.Fatal("expected Corrupted=false")
	}
}

func TestFindProject_InSubdirWalkUp(t *testing.T) {
	root := repoRoot(t)
	sub := filepath.Join(root, "internal", "project")
	ctx, err := FindProject(sub)
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if !ctx.Valid {
		t.Fatal("expected Valid=true from ancestor walk-up")
	}
	if ctx.Root != root {
		t.Fatalf("Root = %q, want %q", ctx.Root, root)
	}
}

func TestFindProject_NotFound(t *testing.T) {
	dir := t.TempDir()
	ctx, err := FindProject(dir)
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if ctx.Valid {
		t.Fatal("expected Valid=false")
	}
	if ctx.Corrupted {
		t.Fatal("expected Corrupted=false")
	}
	if ctx.Root != "" {
		t.Fatalf("Root = %q, want empty", ctx.Root)
	}
}

func TestFindProject_Corrupted(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, ".specs", "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// STATE.md without PROJECT.md triggers corrupted detection.
	if err := os.WriteFile(filepath.Join(projectDir, "STATE.md"), []byte("# State\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, err := FindProject(dir)
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if ctx.Valid {
		t.Fatal("expected Valid=false")
	}
	if !ctx.Corrupted {
		t.Fatal("expected Corrupted=true")
	}
}

func TestFindProject_ValidInAncestorOverridesCorruptedChild(t *testing.T) {
	root := t.TempDir()
	validDir := filepath.Join(root, "valid")
	corruptDir := filepath.Join(validDir, "sub", "corrupt")

	for _, d := range []struct {
		dir   string
		valid bool
	}{
		{validDir, true},
		{corruptDir, false},
	} {
		projectDir := filepath.Join(d.dir, ".specs", "project")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if d.valid {
			if err := os.WriteFile(filepath.Join(projectDir, "PROJECT.md"), []byte("# Project\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	ctx, err := FindProject(corruptDir)
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if !ctx.Valid {
		t.Fatal("expected Valid=true from ancestor")
	}
	if ctx.Root != validDir {
		t.Fatalf("Root = %q, want %q", ctx.Root, validDir)
	}
	if !ctx.Corrupted {
		t.Fatal("expected Corrupted=true from child dir without PROJECT.md")
	}
}

func TestFindProject_EvalSymlinks(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, ".specs", "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "PROJECT.md"), []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(root, "linked")
	if err := os.Symlink(root, linkDir); err != nil {
		t.Skip("symlink not supported:", err)
	}

	ctx, err := FindProject(linkDir)
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if !ctx.Valid {
		t.Fatal("expected Valid=true through symlink")
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Root != wantRoot {
		t.Fatalf("Root = %q, want %q", ctx.Root, wantRoot)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/project/*_test.go → repo root is two levels up.
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
