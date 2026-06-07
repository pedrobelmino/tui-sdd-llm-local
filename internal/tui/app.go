package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the interactive tsll dashboard.
func Run(version string) error {
	p := tea.NewProgram(NewRootModel(version), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
