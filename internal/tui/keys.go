package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap holds global TUI keybindings.
type KeyMap struct {
	Quit       key.Binding
	ForceQuit  key.Binding
	Help       key.Binding
	Overview   key.Binding
	Features   key.Binding
	Models     key.Binding
	Metrics    key.Binding
	System     key.Binding
	Refresh    key.Binding
	Init       key.Binding
	Up         key.Binding
	Down       key.Binding
	NewFeature key.Binding
	Open       key.Binding
	Specify    key.Binding
	GenDesign  key.Binding
	GenTasks   key.Binding
	Implement    key.Binding
	ImplementAll key.Binding
	RunTask      key.Binding
	Back       key.Binding
	Submit     key.Binding
}

// DefaultKeyMap returns the tsll TUI keymap.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Overview: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "overview")),
		Features: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "features")),
		Models: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "models")),
		Metrics: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "metrics")),
		System: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "system")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Init: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "init project")),
		Up: key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k", "up")),
		Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j", "down")),
		NewFeature: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new feature")),
		Open: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Specify: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "specify")),
		GenDesign: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "design")),
		GenTasks: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tasks")),
		Implement:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "implement")),
		ImplementAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "implement all")),
		RunTask:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "run task")),
		Back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Submit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
	}
}

// FooterBindings returns compact footer for dashboard.
func FooterBindings() string {
	return "r: refresh │ 1-5: views │ ?: help │ q: quit"
}

// FeaturesFooter returns footer when on features view.
func FeaturesFooter() string {
	return "n:new │ enter:open │ s:spec │ d:design │ t:tasks │ e:impl │ j/k:nav"
}

// HelpOverlay returns full help text.
func HelpOverlay() string {
	lines := []string{
		"tsll — keyboard shortcuts",
		"",
		"  Global",
		"  q / ctrl+c   quit",
		"  ?            toggle help",
		"  1-5          switch views",
		"  r            refresh",
		"",
		"  Overview (1)",
		"  i            init project (name + vision)",
		"",
		"  Features view (2)",
		"  n            new feature (name + brief → spec)",
		"  j / k        move selection",
		"  enter        open feature detail",
		"  s            specify (spec.md)",
		"  d            design (design.md, needs spec)",
		"  t            tasks (tasks.md, needs spec)",
		"  e            implement feature (needs spec)",
		"",
		"  Feature detail",
		"  j / k        select task",
		"  e / enter    run selected task (needs tasks)",
		"  a            implement all pending tasks (needs spec)",
		"  s            regenerate spec",
		"  d            regenerate design",
		"  t            regenerate tasks",
		"  esc          back to features list",
		"",
		"  Forms & actions",
		"  enter        submit",
		"  j / k        scroll output (during generation)",
		"  g / G        top / bottom of output",
		"  esc          cancel / close",
		"",
		"Press ? to close",
	}
	return strings.Join(lines, "\n")
}
