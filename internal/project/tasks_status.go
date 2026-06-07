package project

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var taskBlockStatusRe = regexp.MustCompile(`(?m)^\*\*Status\*\*:\s*.+$`)

// UpdateTaskStatus writes task status into tasks.md (Status Tracker table + optional **Status** line).
func UpdateTaskStatus(tasksPath, taskID, status string) error {
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return err
	}
	content := updateStatusTracker(string(data), taskID, status)
	content = updateTaskBlockStatus(content, taskID, status)
	return os.WriteFile(tasksPath, []byte(content), 0o644)
}

func updateStatusTracker(content, taskID, status string) string {
	lines := strings.Split(content, "\n")
	trackerIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "## Status Tracker") {
			trackerIdx = i
			break
		}
	}

	if trackerIdx < 0 {
		section := buildStatusTrackerSection(content, taskID, status)
		content = strings.TrimRight(content, "\n") + "\n\n" + section + "\n"
		return content
	}

	headerIdx, statusCol := -1, 1
	for i := trackerIdx + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "## ") {
			break
		}
		if !strings.Contains(trim, "|") {
			continue
		}
		cells := splitTableRow(trim)
		if headerIdx < 0 && strings.Contains(strings.ToLower(trim), "task") {
			headerIdx = i
			for j, c := range cells {
				if strings.Contains(strings.ToLower(c), "status") {
					statusCol = j
					break
				}
			}
			continue
		}
		if headerIdx >= 0 && isSeparatorRow(trim) {
			continue
		}
		if headerIdx >= 0 && rowMatchesTaskID(cells, taskID) {
			lines[i] = setTableStatus(lines[i], statusCol, status)
			return strings.Join(lines, "\n")
		}
	}

	// Tracker exists but row missing — insert after header/separator.
	insertAt := trackerIdx + 1
	for i := trackerIdx + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "## ") {
			insertAt = i
			break
		}
		if strings.Contains(trim, "|") {
			insertAt = i + 1
		}
	}
	newRow := fmt.Sprintf("| %s | %s |", taskID, status)
	if headerIdx >= 0 {
		headerCells := splitTableRow(strings.TrimSpace(lines[headerIdx]))
		if len(headerCells) > 2 {
			for len(strings.Split(newRow, "|"))-2 < len(headerCells) {
				newRow = strings.TrimSuffix(newRow, "|") + " — |"
			}
		}
	}
	lines = append(lines[:insertAt], append([]string{newRow}, lines[insertAt:]...)...)
	return strings.Join(lines, "\n")
}

func buildStatusTrackerSection(content, taskID, status string) string {
	ids := collectTaskIDsFromContent(content)
	if len(ids) == 0 {
		ids = []string{taskID}
	}
	var b strings.Builder
	b.WriteString("## Status Tracker\n\n")
	b.WriteString("| Task | Status |\n")
	b.WriteString("| ---- | ------ |\n")
	for _, id := range ids {
		st := "Pending"
		if id == taskID {
			st = status
		}
		b.WriteString(fmt.Sprintf("| %s | %s |\n", id, st))
	}
	return strings.TrimRight(b.String(), "\n")
}

func collectTaskIDsFromContent(content string) []string {
	tasks := ParseTasksContent(content)
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

func updateTaskBlockStatus(content, taskID, status string) string {
	lines := strings.Split(content, "\n")
	headerIdx := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if m := taskHeaderRe.FindStringSubmatch(trim); len(m) == 3 && m[1] == taskID {
			headerIdx = i
			break
		}
		if m := taskHeaderAltRe.FindStringSubmatch(trim); len(m) == 3 && m[2] == taskID {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return content
	}

	end := len(lines)
	for i := headerIdx + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "### ") || strings.HasPrefix(trim, "## ") {
			end = i
			break
		}
	}

	statusLine := "**Status**: " + status
	updated := false
	for i := headerIdx + 1; i < end; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "**Status**:") {
			lines[i] = statusLine
			updated = true
			break
		}
	}
	if !updated {
		insert := append([]string{lines[headerIdx], "", statusLine}, lines[headerIdx+1:end]...)
		lines = append(lines[:headerIdx], append(insert, lines[end:]...)...)
	}
	return strings.Join(lines, "\n")
}

func splitTableRow(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	if len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func isSeparatorRow(line string) bool {
	trim := strings.TrimSpace(line)
	if !strings.Contains(trim, "|") {
		return false
	}
	for _, c := range trim {
		if c != '|' && c != '-' && c != ':' && c != ' ' {
			return false
		}
	}
	return strings.Contains(trim, "-")
}

func rowMatchesTaskID(cells []string, taskID string) bool {
	if len(cells) == 0 {
		return false
	}
	return strings.EqualFold(cells[0], taskID)
}

func setTableStatus(row string, statusCol int, status string) string {
	cells := splitTableRow(row)
	if statusCol < 0 || statusCol >= len(cells) {
		if len(cells) >= 2 {
			statusCol = 1
		} else {
			return row
		}
	}
	cells[statusCol] = status
	var b strings.Builder
	b.WriteString("|")
	for _, c := range cells {
		b.WriteString(" ")
		b.WriteString(c)
		b.WriteString(" |")
	}
	return b.String()
}
