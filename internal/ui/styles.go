package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// NoColor reports whether ANSI styling should be suppressed (NO_COLOR convention).
func NoColor() bool {
	v := strings.TrimSpace(os.Getenv("NO_COLOR"))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "0", "false", "off":
		return false
	default:
		return true
	}
}

// k9s-inspired palette (design.md).
var (
	StyleHeader  = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("255")).Bold(true)
	StyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	StyleDim     = lipgloss.NewStyle().Faint(true)
	StylePanel   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
)

func render(style lipgloss.Style, text string) string {
	if NoColor() {
		return text
	}
	return style.Render(text)
}
