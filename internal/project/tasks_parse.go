package project

import (
	"os"
	"regexp"
	"strings"
)

// TaskEntry is a parsed task from tasks.md.
type TaskEntry struct {
	ID     string
	Title  string
	Status string // Pending, In Progress, Done
}

var (
	taskHeaderRe    = regexp.MustCompile(`^### (T\d+):\s*(.+)$`)
	taskHeaderAltRe = regexp.MustCompile(`^### (?:\d+\.\s*)?(.+?)\s*\((T\d+)\)\s*$`)
	statusRowRe     = regexp.MustCompile(`^\|\s*(T\d+)\s*\|\s*([^|]+)\|`)
)

// ParseTasks reads task IDs, titles and status from tasks.md.
func ParseTasks(tasksPath string) ([]TaskEntry, error) {
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return nil, err
	}
	return ParseTasksContent(string(data)), nil
}

// ParseTasksContent parses tasks from markdown content.
func ParseTasksContent(content string) []TaskEntry {
	titles := map[string]string{}
	statuses := map[string]string{}

	lines := strings.Split(content, "\n")
	inTracker := false

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## Status Tracker") {
			inTracker = true
			continue
		}
		if inTracker && strings.HasPrefix(trim, "## ") {
			inTracker = false
		}

		if m := taskHeaderRe.FindStringSubmatch(trim); len(m) == 3 {
			titles[m[1]] = strings.TrimSpace(m[2])
			if _, ok := statuses[m[1]]; !ok {
				statuses[m[1]] = "Pending"
			}
			continue
		}
		if m := taskHeaderAltRe.FindStringSubmatch(trim); len(m) == 3 {
			titles[m[2]] = strings.TrimSpace(m[1])
			if _, ok := statuses[m[2]]; !ok {
				statuses[m[2]] = "Pending"
			}
			continue
		}

		if inTracker {
			if m := statusRowRe.FindStringSubmatch(line); len(m) == 3 {
				id := strings.TrimSpace(m[1])
				raw := strings.TrimSpace(m[2])
				statuses[id] = normalizeTaskStatus(raw)
			}
		}
	}

	// preserve T1, T2... order from titles map keys sorted
	ids := sortedTaskIDs(titles)
	out := make([]TaskEntry, 0, len(ids))
	for _, id := range ids {
		st := statuses[id]
		if st == "" {
			st = "Pending"
		}
		out = append(out, TaskEntry{ID: id, Title: titles[id], Status: st})
	}
	return out
}

func sortedTaskIDs(titles map[string]string) []string {
	var ids []string
	for id := range titles {
		ids = append(ids, id)
	}
	// simple numeric sort T1, T2, ... T10
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if taskNum(ids[j]) < taskNum(ids[i]) {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

func taskNum(id string) int {
	n := 0
	for _, c := range id {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func normalizeTaskStatus(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "done") || strings.Contains(raw, "✅"):
		return "Done"
	case strings.Contains(lower, "progress"):
		return "In Progress"
	default:
		return "Pending"
	}
}

// TaskProgress returns done/total counts.
func TaskProgress(tasks []TaskEntry) (done, total int) {
	total = len(tasks)
	for _, t := range tasks {
		if t.Status == "Done" {
			done++
		}
	}
	return done, total
}
