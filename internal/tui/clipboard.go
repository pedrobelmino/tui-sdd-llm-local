package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

func lastActionLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tsll", "last-action.log")
}

// readLastActionLogFile returns the most recently saved action log from disk.
func readLastActionLogFile() (string, error) {
	path := lastActionLogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// copyActionLog saves the full log to disk first, then copies to clipboard when possible.
func copyActionLog(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		var err error
		text, err = readLastActionLogFile()
		if err != nil || strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("log is empty — run an action first")
		}
	}
	path, err := saveActionLogFile(text)
	if err != nil {
		return "", err
	}
	lines := strings.Count(text, "\n") + 1
	nbytes := len(text)
	meta := fmt.Sprintf("%d lines, %d bytes", lines, nbytes)
	if err := clipboard.WriteAll(text); err == nil {
		return fmt.Sprintf("copied full log (%s) — backup at %s", meta, path), nil
	}
	return fmt.Sprintf("clipboard unavailable — saved full log (%s) to %s", meta, path), nil
}

func saveActionLogFile(text string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tsll")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	path := filepath.Join(dir, "last-action-"+stamp+".log")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	latest := filepath.Join(dir, "last-action.log")
	_ = os.WriteFile(latest, []byte(text), 0o644)
	return latest, nil
}

// persistActionLogIncremental writes the in-progress log so it survives crashes and y-after-esc.
func persistActionLogIncremental(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	_, _ = saveActionLogFile(text)
}
