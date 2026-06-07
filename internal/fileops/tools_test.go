package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFile_CreatesFileAndDirs(t *testing.T) {
	root := t.TempDir()
	exec := Executor(root)

	result := exec("write_file", map[string]any{
		"path":    "internal/foo/bar.go",
		"content": "package foo\n",
	})
	if strings.HasPrefix(result, "ERROR") {
		t.Fatalf("write_file failed: %s", result)
	}
	data, err := os.ReadFile(filepath.Join(root, "internal/foo/bar.go"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "package foo\n" {
		t.Fatalf("content mismatch: %q", data)
	}
}

func TestReadFile_ReturnsContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)
	result := exec("read_file", map[string]any{"path": "hello.txt"})
	if result != "hello world" {
		t.Fatalf("read_file result = %q", result)
	}
}

func TestReadFile_MissingReturnsError(t *testing.T) {
	root := t.TempDir()
	exec := Executor(root)
	result := exec("read_file", map[string]any{"path": "nope.txt"})
	if !strings.HasPrefix(result, "ERROR") {
		t.Fatalf("expected ERROR, got %q", result)
	}
}

func TestReadFile_DirectoryReturnsError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)
	result := exec("read_file", map[string]any{"path": "internal"})
	if !strings.HasPrefix(result, "ERROR") {
		t.Fatalf("expected ERROR for directory, got: %s", result)
	}
	if !strings.Contains(result, "is a directory") || !strings.Contains(result, "list_dir") {
		t.Fatalf("unexpected read_file(directory) result: %q", result)
	}
	if !strings.Contains(result, "ui/") || !strings.Contains(result, "index.html") {
		t.Fatalf("expected directory entries in result: %q", result)
	}
}

func TestWriteFile_BlocksSpecsPath(t *testing.T) {
	root := t.TempDir()
	exec := Executor(root)
	result := exec("write_file", map[string]any{
		"path":    ".specs/features/FEAT-01.md",
		"content": "should not be written",
	})
	if !strings.HasPrefix(result, "ERROR") {
		t.Fatalf("expected specs write blocked, got: %s", result)
	}
	if !strings.Contains(result, "tasks.md") {
		t.Fatalf("expected helpful hint about tasks.md: %s", result)
	}
}

func TestWriteFile_RejectsTruncatedGo(t *testing.T) {
	root := t.TempDir()
	exec := Executor(root)
	result := exec("write_file", map[string]any{
		"path":    "src/main.go",
		"content": "package main\n\nimport (\n",
	})
	if !strings.HasPrefix(result, "ERROR") {
		t.Fatalf("expected truncated Go rejection, got: %s", result)
	}
}

func TestEditFile_ReplacesExactMatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src.go")
	if err := os.WriteFile(path, []byte("func Old() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)
	result := exec("edit_file", map[string]any{
		"path":        "src.go",
		"old_content": "func Old() {}",
		"new_content": "func New() {}",
	})
	if strings.HasPrefix(result, "ERROR") {
		t.Fatalf("edit_file failed: %s", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "func New() {}") {
		t.Fatalf("edit not applied: %q", data)
	}
}

func TestEditFile_AmbiguousReturnsError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "dup.go")
	if err := os.WriteFile(path, []byte("x\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)
	result := exec("edit_file", map[string]any{
		"path":        "dup.go",
		"old_content": "x",
		"new_content": "y",
	})
	if !strings.HasPrefix(result, "ERROR") {
		t.Fatalf("expected ambiguous error, got %q", result)
	}
}

func TestDeleteFile_RemovesFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "del.txt")
	if err := os.WriteFile(path, []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)
	result := exec("delete_file", map[string]any{"path": "del.txt"})
	if strings.HasPrefix(result, "ERROR") {
		t.Fatalf("delete_file failed: %s", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file still exists after delete")
	}
}

func TestCreateDir_MakesDirectory(t *testing.T) {
	root := t.TempDir()
	exec := Executor(root)
	result := exec("create_dir", map[string]any{"path": "a/b/c"})
	if strings.HasPrefix(result, "ERROR") {
		t.Fatalf("create_dir failed: %s", result)
	}
	info, err := os.Stat(filepath.Join(root, "a/b/c"))
	if err != nil || !info.IsDir() {
		t.Fatal("directory not created")
	}
}

func TestListDir_ShowsEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)
	result := exec("list_dir", map[string]any{"path": "."})
	if !strings.Contains(result, "a.go") || !strings.Contains(result, "sub/") {
		t.Fatalf("list_dir result = %q", result)
	}
}

func TestListDir_ShowsHiddenFiles(t *testing.T) {
	root := t.TempDir()
	// Create hidden file and hidden directory
	if err := os.WriteFile(filepath.Join(root, ".hidden-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".specs/features"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	exec := Executor(root)

	result := exec("list_dir", map[string]any{"path": "."})
	if !strings.Contains(result, ".hidden-file") {
		t.Errorf("list_dir did not show hidden file; result = %q", result)
	}
	if !strings.Contains(result, ".specs/") {
		t.Errorf("list_dir did not show hidden dir .specs/; result = %q", result)
	}
	if !strings.Contains(result, "visible.go") {
		t.Errorf("list_dir did not show visible file; result = %q", result)
	}

	// Navigating into hidden directory also works
	result2 := exec("list_dir", map[string]any{"path": ".specs"})
	if !strings.Contains(result2, "features/") {
		t.Errorf("list_dir .specs did not show features/; result = %q", result2)
	}
}

func TestUnknownTool_ReturnsError(t *testing.T) {
	exec := Executor(t.TempDir())
	result := exec("fly_to_moon", map[string]any{})
	if !strings.HasPrefix(result, "ERROR") {
		t.Fatalf("expected ERROR for unknown tool, got %q", result)
	}
}

func TestDefinitions_HasRequiredTools(t *testing.T) {
	defs := Definitions()
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Function.Name] = true
	}
	for _, want := range []string{"write_file", "read_file", "edit_file", "delete_file", "create_dir", "list_dir"} {
		if !names[want] {
			t.Errorf("missing tool definition: %s", want)
		}
	}
}
