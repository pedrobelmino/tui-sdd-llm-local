package templates

import (
	"fmt"
	"strings"
	"time"
)

// ProjectFiles holds generated project bootstrap content.
type ProjectFiles struct {
	Project string
	Roadmap string
	State   string
}

// NewProject generates initial .specs/project/ files from Q&A answers.
func NewProject(name, vision, target, solves, stack, scopeIn, scopeOut string) ProjectFiles {
	now := time.Now().Format("2006-01-02")
	return ProjectFiles{
		Project: fmt.Sprintf(`# %s

**Vision:** %s
**For:** %s
**Solves:** %s

## Goals

- [ ] Deliver v1 capabilities defined below
- [ ] Maintain spec-driven traceability via tui-sdd-llm-local

## Tech Stack

**Core:**

%s

## Scope

**v1 includes:**

%s

**Explicitly out of scope:**

%s

## Constraints

- Platform: Linux
- Model: qwen2.5-coder:3b via Ollama
`, name, vision, target, solves, stack, scopeIn, scopeOut),
		Roadmap: fmt.Sprintf(`# Roadmap

**Current Milestone:** M1 — Foundation
**Status:** In Progress

---

## M1 — Foundation

**Goal:** Project initialized with tui-sdd-llm-local workflow
**Target:** %s

### Features

**Bootstrap** - IN PROGRESS

- Project structure via tsll init
`, name),
		State: fmt.Sprintf(`# State

**Last Updated:** %s
**Current Work:** Project initialized

---

## Recent Decisions (Last 60 days)

### AD-001: Project initialized via tsll init (%s)

**Decision:** Bootstrap .specs/project/ with tui-sdd-llm-local
**Reason:** Start spec-driven workflow
**Trade-off:** —
**Impact:** Ready for tsll specify / tsll tasks / tsll run

---

## Active Blockers

_None._

---

## Lessons Learned

_None._

---

## Todos

- [ ] Specify first feature with tsll specify
`, now, now),
	}
}

// Spec generates a feature spec.md from LLM output (validated wrapper).
func Spec(featureName, body string) string {
	if strings.Contains(body, "# ") {
		return body
	}
	return fmt.Sprintf("# %s Specification\n\n%s\n", featureName, body)
}

// Tasks wraps LLM task breakdown output.
func Tasks(body string) string {
	if strings.HasPrefix(strings.TrimSpace(body), "# ") {
		return body
	}
	return "# Tasks\n\n**Status**: Approved\n\n" + body
}

// Design wraps LLM architecture output.
func Design(featureName, body string) string {
	if strings.HasPrefix(strings.TrimSpace(body), "# ") {
		return body
	}
	return fmt.Sprintf("# %s Design\n\n**Spec**: `.specs/features/%s/spec.md`\n**Status**: Draft\n\n---\n\n%s\n",
		featureName, featureName, body)
}
