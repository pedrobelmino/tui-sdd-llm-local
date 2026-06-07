package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tui"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:   "tsll",
	Short: "Spec-driven development CLI with interactive dashboard",
	Long: `tsll guides solo developers through spec-driven development
with a k9s-like terminal dashboard and local Ollama models.

Run ` + "`tsll`" + ` for the interactive dashboard.
Run ` + "`tsll --help`" + ` for subcommands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if ShouldLaunchTUI() {
			return tui.Run(version)
		}
		fmt.Println("tsll: run in an interactive terminal for the dashboard, or use tsll --help")
		return nil
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.SetVersionTemplate("tsll {{.Version}}\n")
	rootCmd.Version = version
}

// Execute runs the Cobra root command.
func Execute() error {
	return rootCmd.Execute()
}

// ShouldLaunchTUI reports whether the default interactive dashboard should start.
func ShouldLaunchTUI() bool {
	if os.Getenv("TSLL_TUI") == "0" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

// PlainError prints a user-facing error for non-TUI mode.
func PlainError(msg string) {
	fmt.Fprintln(os.Stderr, ui.PlainError(msg))
}
