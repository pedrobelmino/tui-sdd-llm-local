package workflow

import (
	"strings"
	"testing"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
)

func TestExtractTaskBlock_NumberedHeader(t *testing.T) {
	tasks := `### 3. Develop Header Section (T3)
**What**: Implement the header section with company logo and navigation menu.
**Where**: Landing page codebase
`
	block := extractTaskBlock(tasks, "T3")
	if block == "" {
		t.Fatal("expected task block for T3")
	}
	if !strings.Contains(block, "Develop Header Section") {
		t.Fatalf("block: %q", block)
	}
}

func TestBuildAskUserMsg_IncludesQuestionAndSpec(t *testing.T) {
	msg := buildAskUserMsg(featureContext{
		Feature: "landing-page",
		Spec:    "## Goals\nBuild landing page",
		Design:  "## Tech Decisions\nReact",
		Tasks:   "### T3: Develop Header",
		ProjectStack: "## Tech Stack\n- Go",
	}, "Why React in design but Go in project?", false)
	if !strings.Contains(msg, "## Question") || !strings.Contains(msg, "Why React") {
		t.Fatalf("missing question: %q", msg)
	}
	if !strings.Contains(msg, "spec.md") || !strings.Contains(msg, "design.md") {
		t.Fatalf("missing docs: %q", msg)
	}
}

func TestParseLandingPageTasks(t *testing.T) {
	content := `### 1. Analyze Requirements (T1)
x
### 3. Develop Header Section (T3)
x
### 8. Conduct Functional Testing (T8)
x
`
	all := project.ParseTasksContent(content)
	if len(all) != 3 {
		t.Fatalf("parsed %d tasks", len(all))
	}
	impl := project.ImplementableTasks(all)
	if len(impl) != 1 || impl[0].ID != "T3" {
		t.Fatalf("implementable: %+v", impl)
	}
}
