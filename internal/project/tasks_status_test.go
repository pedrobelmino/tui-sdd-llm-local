package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateTaskStatus_CreatesTrackerWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	content := `### 3. Develop Header Section (T3)

**What**: header

### 5. Develop Contact (T5)

**What**: contact
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateTaskStatus(path, "T3", "✅ Done"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "## Status Tracker") {
		t.Fatalf("missing tracker: %s", s)
	}
	tasks := ParseTasksContent(s)
	for _, task := range tasks {
		if task.ID == "T3" && task.Status != "Done" {
			t.Fatalf("T3 status = %q", task.Status)
		}
		if task.ID == "T5" && task.Status != "Pending" {
			t.Fatalf("T5 should stay pending, got %q", task.Status)
		}
	}
}

func TestUpdateTaskStatus_FixesStatusColumnWithCommitColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	content := `## Status Tracker

| Task | Status | Commit |
| ---- | ------ | ------ |
| T3 | Pending | — |
| T4 | Pending | — |

### T3: Header
### T4: Footer
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateTaskStatus(path, "T3", "✅ Done"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if strings.Contains(s, "| T3 | — |") || strings.Contains(s, "| T3 | Pending |") {
		t.Fatalf("status not updated in Status column: %s", s)
	}
	if !strings.Contains(s, "| T3 | ✅ Done |") {
		t.Fatalf("expected T3 done row: %s", s)
	}
	tasks := ParseTasksContent(s)
	if tasks[0].ID != "T3" || tasks[0].Status != "Done" {
		t.Fatalf("parsed T3: %+v", tasks[0])
	}
}

func TestUpdateTaskStatus_AddsInlineStatusLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	content := `### 5. Develop Contact (T5)

**What**: contact section
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateTaskStatus(path, "T5", "✅ Done"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "**Status**: ✅ Done") {
		t.Fatalf("missing inline status: %s", s)
	}
}

func TestParseTasksContent_InlineStatus(t *testing.T) {
	content := `### T3: Header

**Status**: ✅ Done

### T4: Footer

**Status**: Pending
`
	tasks := ParseTasksContent(content)
	if len(tasks) != 2 {
		t.Fatalf("len=%d", len(tasks))
	}
	if tasks[0].Status != "Done" || tasks[1].Status != "Pending" {
		t.Fatalf("statuses: %+v", tasks)
	}
}

func TestImplementableTasks_SkipsDone(t *testing.T) {
	tasks := []TaskEntry{
		{ID: "T3", Title: "Develop Header", Status: "Done"},
		{ID: "T4", Title: "Develop Footer", Status: "Done"},
		{ID: "T5", Title: "Develop Contact", Status: "Pending"},
	}
	got := ImplementableTasks(tasks)
	if len(got) != 1 || got[0].ID != "T5" {
		t.Fatalf("got %+v", got)
	}
}
