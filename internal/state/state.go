package state

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	currentWorkRe = regexp.MustCompile(`(?m)^\*\*Last Updated:\*\*.*$`)
	workLineRe    = regexp.MustCompile(`(?m)^\*\*Current Work:\*\*.*$`)
)

// UpdateCurrentWork sets Current Work and Last Updated in STATE.md.
func UpdateCurrentWork(statePath, work string) error {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	content := string(data)
	now := time.Now().Format("2006-01-02")

	if workLineRe.MatchString(content) {
		content = workLineRe.ReplaceAllString(content, "**Current Work:** "+work)
	} else {
		content = strings.Replace(content, "# State\n", "# State\n\n**Current Work:** "+work+"\n", 1)
	}

	if currentWorkRe.MatchString(content) {
		content = regexp.MustCompile(`(?m)^\*\*Last Updated:\*\*.*$`).ReplaceAllString(
			content, "**Last Updated:** "+now)
	}

	return os.WriteFile(statePath, []byte(content), 0o644)
}

// AppendDecision adds an AD-* entry after Recent Decisions header.
func AppendDecision(statePath, title, decision, reason string) error {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	content := string(data)
	n := countADEntries(content) + 1
	entry := fmt.Sprintf(`
### AD-%03d: %s (%s)

**Decision:** %s
**Reason:** %s
**Trade-off:** —
**Impact:** —

`, n, title, time.Now().Format("2006-01-02"), decision, reason)

	marker := "## Recent Decisions"
	idx := strings.Index(content, marker)
	if idx < 0 {
		return fmt.Errorf("STATE.md missing Recent Decisions section")
	}
	insertAt := idx + len(marker)
	// skip "(Last 60 days)" line
	rest := content[insertAt:]
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		insertAt += nl + 1
	}
	newContent := content[:insertAt] + entry + content[insertAt:]
	return os.WriteFile(statePath, []byte(newContent), 0o644)
}

func countADEntries(content string) int {
	return len(regexp.MustCompile(`### AD-\d+`).FindAllString(content, -1))
}

// UpdateTaskStatus replaces task status in tasks.md.
func UpdateTaskStatus(tasksPath, taskID, status string) error {
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.Contains(line, taskID) && strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				parts[len(parts)-2] = " " + status + " "
				lines[i] = strings.Join(parts, "|")
			}
		}
	}
	return os.WriteFile(tasksPath, []byte(strings.Join(lines, "\n")), 0o644)
}
