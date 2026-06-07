package project

import "strings"

// IsImplementableTask reports whether a task should run in tsll implement/run automation.
// Skips analysis, design-only, testing, and deployment tasks.
func IsImplementableTask(t TaskEntry) bool {
	if t.Status == "Done" {
		return false
	}
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
