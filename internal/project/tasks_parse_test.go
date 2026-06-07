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
