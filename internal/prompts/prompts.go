package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillDir returns the tlc-spec-driven skill path in the project or home.
func SkillDir(projectRoot string) string {
	candidates := []string{
		filepath.Join(projectRoot, ".cursor/skills/tlc-spec-driven"),
		filepath.Join(projectRoot, ".agents/skills/tlc-spec-driven"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

// LoadReference reads a skill reference file if present.
func LoadReference(projectRoot, name string) string {
	dir := SkillDir(projectRoot)
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "references", name))
	if err != nil {
		return ""
	}
	return string(data)
}

// SpecifySystem builds the system prompt for tsll specify.
func SpecifySystem(projectRoot string) string {
	specRef := LoadReference(projectRoot, "specify.md")
	return strings.TrimSpace(fmt.Sprintf(`You are tui-sdd-llm-local, a spec-driven development assistant.
Follow tlc-spec-driven specify conventions. Output a complete spec.md in markdown.
Use WHEN/THEN/SHALL acceptance criteria, P1/P2/P3 user stories, requirement IDs like FEAT-01.
Include Problem Statement, Goals, Out of Scope, User Stories, Edge Cases, Requirement Traceability, Success Criteria.

Reference:
%s`, truncate(specRef, 4000)))
}

// TasksSystem builds the system prompt for tsll tasks.
func TasksSystem(projectRoot string) string {
	tasksRef := LoadReference(projectRoot, "tasks.md")
	return strings.TrimSpace(fmt.Sprintf(`You are tui-sdd-llm-local. Break the feature spec into atomic tasks in tasks.md format.
Each task: What, Where, Depends on, Done when, Tests, Gate. Use T1, T2... numbering.
Include Execution Plan phases and Requirement Traceability.

Reference:
%s`, truncate(tasksRef, 4000)))
}

// RunSystem builds the system prompt for tsll run task execution.
func RunSystem(projectRoot, taskDesc, specContext string) string {
	principles := LoadReference(projectRoot, "coding-principles.md")
	return strings.TrimSpace(fmt.Sprintf(`You are tui-sdd-llm-local implementing a single atomic task.
Follow the spec and design. Make minimal, surgical changes. Match existing code style.
Output only the implementation plan and unified diff style changes when asked.

Coding principles:
%s

Task:
%s

Spec context:
%s`, truncate(principles, 2000), taskDesc, truncate(specContext, 6000)))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...(truncated)"
}
