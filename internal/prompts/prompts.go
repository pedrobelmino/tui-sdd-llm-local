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

// DesignSystem builds the system prompt for tsll design.
func DesignSystem(projectRoot string) string {
	designRef := LoadReference(projectRoot, "design.md")
	return strings.TrimSpace(fmt.Sprintf(`You are tui-sdd-llm-local. Define HOW to build the feature in design.md format.
Include Architecture Overview (mermaid when helpful), Code Reuse Analysis, Components, Data Models if applicable,
Error Handling Strategy, and Tech Decisions. Reuse existing codebase patterns. Never fabricate APIs or behaviors.

Reference:
%s`, truncate(designRef, 4000)))
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

// ImplementSystem builds the system prompt for full-feature implementation.
func ImplementSystem(projectRoot string) string {
	implRef := LoadReference(projectRoot, "implement.md")
	principles := LoadReference(projectRoot, "coding-principles.md")
	return strings.TrimSpace(fmt.Sprintf(`You are tui-sdd-llm-local implementing a complete feature from its specification.
Follow tlc-spec-driven execute conventions. You have file tools available — USE THEM to actually create and edit
files on disk. Do not just describe changes; call write_file, edit_file, create_dir, etc. to apply them.

Workflow:
1. Call list_dir(".") to understand the project structure.
2. Call read_file for files you need to understand before editing.
3. Call write_file / edit_file / create_dir / delete_file to make the changes.
4. After all changes, summarise what was done.

Constraints: touch only files listed in the task; make minimum changes; match existing code style.
Never fabricate imports, APIs, or package names — read the existing files first.

Coding principles:
%s

Reference:
%s`, truncate(principles, 2000), truncate(implRef, 4000)))
}

// RunSystem builds the system prompt for tsll run task execution.
func RunSystem(projectRoot, taskDesc, specContext string) string {
	principles := LoadReference(projectRoot, "coding-principles.md")
	return strings.TrimSpace(fmt.Sprintf(`You are tui-sdd-llm-local implementing a single atomic task.
You have file tools — USE THEM to actually create and edit files on disk.

Workflow:
1. Call list_dir(".") or read relevant files to understand the codebase first.
2. Use write_file to create new files, edit_file to modify existing ones.
3. After all changes, briefly summarise what was done.

Constraints: touch only files listed in the task; minimum changes; match existing code style.
Never fabricate imports, APIs, or package names — read existing files first.

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
