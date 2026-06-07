package project

import (
	"os"
	"path/filepath"
)

// ProjectContext holds detected .specs/ project metadata.
type ProjectContext struct {
	Root        string // absolute path to project root
	Valid       bool   // PROJECT.md exists
	Corrupted   bool   // .specs/project/ exists but PROJECT.md missing
	CurrentWork string // from STATE.md
	Milestone   string // from ROADMAP.md "Current Milestone"
}

const (
	projectMarker    = ".specs/project/PROJECT.md"
	projectDirMarker = ".specs/project"
)

// FindProject walks up from cwd looking for .specs/project/PROJECT.md.
// cwd is resolved via filepath.EvalSymlinks before the walk.
func FindProject(cwd string) (ProjectContext, error) {
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return ProjectContext{}, err
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		return ProjectContext{}, err
	}

	ctx := ProjectContext{}
	dir := abs

	for {
		projectMD := filepath.Join(dir, projectMarker)
		projectDir := filepath.Join(dir, projectDirMarker)

		if _, err := os.Stat(projectMD); err == nil {
			ctx.Root = dir
			ctx.Valid = true
			ctx.CurrentWork = ParseCurrentWork(filepath.Join(dir, ".specs/project/STATE.md"))
			ctx.Milestone = ParseMilestone(filepath.Join(dir, ".specs/project/ROADMAP.md"))
			return ctx, nil
		}

		if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
			ctx.Corrupted = true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ctx, nil
}
