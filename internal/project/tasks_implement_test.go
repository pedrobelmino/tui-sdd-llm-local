package project

import "testing"

func TestParseTasksContent_NumberedHeaders(t *testing.T) {
	content := `### 3. Develop Header Section (T3)

text

### 4. Develop Footer (T4)
`
	tasks := ParseTasksContent(content)
	if len(tasks) != 2 {
		t.Fatalf("len=%d, want 2", len(tasks))
	}
	if tasks[0].ID != "T3" || tasks[0].Title != "Develop Header Section" {
		t.Fatalf("t3: %+v", tasks[0])
	}
}

func TestImplementableTasks_FiltersNonCode(t *testing.T) {
	tasks := []TaskEntry{
		{ID: "T1", Title: "Analyze Requirements", Status: "Pending"},
		{ID: "T2", Title: "Design Landing Page Layout", Status: "Pending"},
		{ID: "T3", Title: "Develop Header Section", Status: "Pending"},
		{ID: "T7", Title: "Implement Offline Handling", Status: "Pending"},
		{ID: "T8", Title: "Conduct Functional Testing", Status: "Pending"},
	}
	got := ImplementableTasks(tasks)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2 (T3,T7)", len(got))
	}
	if got[0].ID != "T3" || got[1].ID != "T7" {
		t.Fatalf("got %+v", got)
	}
}
