package project

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleStateMD = `# State

**Last Updated:** 2026-06-06
**Current Work:** sample-feature — design phase

---

## Recent Decisions (Last 60 days)

### AD-001: Sample (2026-06-06)
`

const sampleRoadmapMD = `# Roadmap

**Current Milestone:** M2 — Spec Workflow
**Status:** In Progress

---

## M2 — Spec Workflow

**Goal:** Sample
`

func writeFixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	projDir := filepath.Join(root, ".specs", "project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "PROJECT.md"), []byte("# Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "STATE.md"), []byte(sampleStateMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "ROADMAP.md"), []byte(sampleRoadmapMD), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFixtureFeatures(t *testing.T, root string, features map[string][]string) {
	t.Helper()
	featDir := filepath.Join(root, ".specs", "features")
	for name, files := range features {
		dir := filepath.Join(featDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("# "+f+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestParseCurrentWork_Fixture(t *testing.T) {
	root := writeFixtureProject(t)
	statePath := filepath.Join(root, ".specs", "project", "STATE.md")

	got := ParseCurrentWork(statePath)
	want := "sample-feature — design phase"
	if got != want {
		t.Fatalf("ParseCurrentWork = %q, want %q", got, want)
	}
}

func TestParseMilestone_Fixture(t *testing.T) {
	root := writeFixtureProject(t)
	roadmapPath := filepath.Join(root, ".specs", "project", "ROADMAP.md")

	got := ParseMilestone(roadmapPath)
	want := "M2 — Spec Workflow"
	if got != want {
		t.Fatalf("ParseMilestone = %q, want %q", got, want)
	}
}

func TestParseCurrentWork_MissingFile(t *testing.T) {
	if got := ParseCurrentWork(filepath.Join(t.TempDir(), "STATE.md")); got != "" {
		t.Fatalf("ParseCurrentWork = %q, want empty", got)
	}
}

func TestParseMilestone_MissingFile(t *testing.T) {
	if got := ParseMilestone(filepath.Join(t.TempDir(), "ROADMAP.md")); got != "" {
		t.Fatalf("ParseMilestone = %q, want empty", got)
	}
}

func TestListFeatures_WithBadges(t *testing.T) {
	root := writeFixtureProject(t)
	writeFixtureFeatures(t, root, map[string][]string{
		"alpha": {"spec.md", "tasks.md", "design.md"},
		"bravo": {"spec.md"},
	})

	features, err := ListFeatures(root)
	if err != nil {
		t.Fatalf("ListFeatures: %v", err)
	}
	if len(features) != 2 {
		t.Fatalf("len(features) = %d, want 2", len(features))
	}

	var alpha, bravo FeatureEntry
	for _, f := range features {
		switch f.Name {
		case "alpha":
			alpha = f
		case "bravo":
			bravo = f
		}
	}

	if !alpha.HasSpec || !alpha.HasTasks || !alpha.HasDesign {
		t.Fatalf("alpha badges incomplete: %+v", alpha)
	}
	if !bravo.HasSpec || bravo.HasTasks || bravo.HasDesign {
		t.Fatalf("bravo badges wrong: %+v", bravo)
	}
}

func TestFeatureImplemented_AllTasksDone(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "landing-page")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := `# Tasks

## Status Tracker

| Task | Status |
| ---- | ------ |
| T1   | ✅ Done |
| T2   | ✅ Done |

### T1: First
### T2: Second
`
	if err := os.WriteFile(filepath.Join(featureDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if !featureImplemented(featureDir) {
		t.Fatal("expected implemented when all tasks done")
	}
}

func TestFeatureImplemented_MarkerWithoutTasks(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "auth")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "implement.done"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !featureImplemented(featureDir) {
		t.Fatal("expected implemented via implement.done marker")
	}
}

func TestListFeatures_EmptyDir(t *testing.T) {
	root := t.TempDir()
	features, err := ListFeatures(root)
	if err != nil {
		t.Fatalf("ListFeatures: %v", err)
	}
	if features != nil {
		t.Fatalf("features = %v, want nil", features)
	}
}
