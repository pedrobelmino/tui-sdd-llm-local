package project

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FeatureEntry describes a feature directory under .specs/features/.
type FeatureEntry struct {
	Name          string
	HasSpec       bool
	HasDesign     bool
	HasTasks      bool
	HasImplement  bool
}

const (
	currentWorkPrefix    = "**Current Work:**"
	currentMilestonePrefix = "**Current Milestone:**"
)

// ParseCurrentWork extracts the Current Work line from STATE.md.
func ParseCurrentWork(statePath string) string {
	return parseMetadataLine(statePath, currentWorkPrefix)
}

// ParseMilestone extracts the Current Milestone line from ROADMAP.md.
func ParseMilestone(roadmapPath string) string {
	return parseMetadataLine(roadmapPath, currentMilestonePrefix)
}

func parseMetadataLine(path, prefix string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// ListFeatures scans .specs/features/ and reports spec/design/tasks/implement status.
func ListFeatures(projectRoot string) ([]FeatureEntry, error) {
	featuresDir := filepath.Join(projectRoot, ".specs/features")
	entries, err := os.ReadDir(featuresDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var features []FeatureEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		featureDir := filepath.Join(featuresDir, name)
		feature := FeatureEntry{
			Name:         name,
			HasSpec:      fileExists(filepath.Join(featureDir, "spec.md")),
			HasDesign:    fileExists(filepath.Join(featureDir, "design.md")),
			HasTasks:     fileExists(filepath.Join(featureDir, "tasks.md")),
			HasImplement: featureImplemented(featureDir),
		}
		features = append(features, feature)
	}

	return features, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func featureImplemented(featureDir string) bool {
	tasksPath := filepath.Join(featureDir, "tasks.md")
	if tasks, err := ParseTasks(tasksPath); err == nil && len(tasks) > 0 {
		done, total := TaskProgress(tasks)
		return done == total
	}
	return fileExists(filepath.Join(featureDir, "implement.done"))
}
