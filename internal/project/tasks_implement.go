package project

import "strings"

// IsCodeTask reports whether a task is a code implementation task (ignores status).
func IsCodeTask(t TaskEntry) bool {
	lower := strings.ToLower(t.Title)
	skip := []string{
		"analyze", "analysis", "requirement analysis",
		"design ", "mockup", "figma", "sketch",
		"conduct ", "testing", "performance test",
		"deploy ",
	}
	for _, s := range skip {
		if strings.Contains(lower, s) {
			return false
		}
	}
	return strings.Contains(lower, "develop") || strings.Contains(lower, "implement")
}

// IsImplementableTask reports whether a task should run in tsll implement/run automation.
// Skips analysis, design-only, testing, deployment tasks, and tasks already marked done.
func IsImplementableTask(t TaskEntry) bool {
	if t.Status == "Done" {
		return false
	}
	return IsCodeTask(t)
}

// CodeTasks returns all code tasks regardless of status.
func CodeTasks(tasks []TaskEntry) []TaskEntry {
	var out []TaskEntry
	for _, t := range tasks {
		if IsCodeTask(t) {
			out = append(out, t)
		}
	}
	return out
}

// ImplementableTasks returns pending code tasks from parsed tasks.
func ImplementableTasks(tasks []TaskEntry) []TaskEntry {
	var out []TaskEntry
	for _, t := range tasks {
		if IsImplementableTask(t) {
			out = append(out, t)
		}
	}
	return out
}
