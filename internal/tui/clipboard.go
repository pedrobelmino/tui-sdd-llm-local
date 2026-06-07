package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/atotto/clipboard"
)

// copyActionLog copies text to the system clipboard, or saves to ~/.tsll/last-action.log.
func copyActionLog(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("log is empty")
	}
	if err := clipboard.WriteAll(text); err == nil {
		_, _ = saveActionLogFile(text)
		return "log copied to clipboard", nil
	}
	path, err := saveActionLogFile(text)
	if err != nil {
		return "", err
	}
	return "clipboard unavailable — saved to " + path, nil
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
	return path, nil
}
