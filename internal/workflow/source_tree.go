package workflow

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var sourceRoots = []string{"internal", "src", "app", "cmd", "pkg", "web", "frontend"}

// formatExistingSourceTree lists source files already on disk (skip .specs).
func formatExistingSourceTree(projectRoot string, maxFiles int) string {
	if maxFiles <= 0 {
		maxFiles = 60
	}
	var files []string
	for _, root := range sourceRoots {
		abs := filepath.Join(projectRoot, root)
		_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(projectRoot, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, ".specs/") {
				return nil
			}
			files = append(files, rel)
			if len(files) >= maxFiles {
				return filepath.SkipAll
			}
			return nil
		})
	}
	for _, extra := range []string{"package.json", "go.mod", "Makefile"} {
		if _, err := os.Stat(filepath.Join(projectRoot, extra)); err == nil {
			files = append(files, extra)
		}
	}
	if len(files) == 0 {
		return ""
	}
	sort.Strings(files)
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	var b strings.Builder
	b.WriteString("## Existing source files (already on disk — do NOT recreate)\n\n")
	for _, f := range files {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	b.WriteString("\nOnly add or edit files required by THIS task. Reuse existing files.\n")
	return b.String()
}
