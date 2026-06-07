package project

import (
	"fmt"
	"os"
)

// RequireRoot finds tsll project from cwd or returns error.
func RequireRoot() (ProjectContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return ProjectContext{}, err
	}
	ctx, err := FindProject(cwd)
	if err != nil {
		return ProjectContext{}, err
	}
	if !ctx.Valid {
		return ProjectContext{}, fmt.Errorf("not a tsll project — run tsll init first")
	}
	return ctx, nil
}
