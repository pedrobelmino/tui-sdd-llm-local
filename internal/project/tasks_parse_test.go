package project

import "testing"

func TestParseTasksContent(t *testing.T) {
	content := `### T1: First task

text

### T2: Second task

## Status Tracker

| Task | Status | Commit |
| T1 | ✅ Done | abc |
| T2 | Pending | — |
`
	tasks := ParseTasksContent(content)
	if len(tasks) != 2 {
		t.Fatalf("len=%d", len(tasks))
	}
	if tasks[0].ID != "T1" || tasks[0].Status != "Done" {
		t.Fatalf("t1: %+v", tasks[0])
	}
	if tasks[1].Status != "Pending" {
		t.Fatalf("t2 status: %s", tasks[1].Status)
	}
}

// TestParseTasksContent_NewTaskFormat covers the real generated format:
// "### Status Tracker" (3 hashes) and "#### New Task: N. Title (TN)" headers.
func TestParseTasksContent_NewTaskFormat(t *testing.T) {
	content := `# Tasks

### Status Tracker

| Task | Status |
| ---- | ------ |
| T2 | Pending |
| T3 | Pending |

### Update to Tasks.md

#### New Task: 2. Design Landing Page Layout (T2)
**What**: mockup.

#### New Task: 3. Develop Header Section (T3)

**Status**: Pending
**What**: Implement the header section.
`
	tasks := ParseTasksContent(content)
	byID := map[string]TaskEntry{}
	for _, tk := range tasks {
		byID[tk.ID] = tk
	}
	t3, ok := byID["T3"]
	if !ok {
		t.Fatalf("T3 not parsed; got %+v", tasks)
	}
	if t3.Title != "Develop Header Section" {
		t.Fatalf("T3 title=%q", t3.Title)
	}
	if !IsCodeTask(t3) {
		t.Fatalf("T3 should be a code task: %+v", t3)
	}
	if t2, ok := byID["T2"]; ok && IsCodeTask(t2) {
		t.Fatalf("T2 (design) should not be a code task: %+v", t2)
	}
	code := CodeTasks(tasks)
	if len(code) == 0 {
		t.Fatalf("expected at least one code task, got none from %+v", tasks)
	}
}
