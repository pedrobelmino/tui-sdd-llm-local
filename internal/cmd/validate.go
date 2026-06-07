package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [feature-name]",
	Short: "Validate feature spec and tasks completeness",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]
		dir := filepath.Join(ctx.Root, ".specs/features", feature)
		checks := []struct{ file, label string }{
			{"spec.md", "specification"},
			{"tasks.md", "tasks"},
			{"design.md", "design (optional)"},
		}

		allOK := true
		for _, c := range checks {
			path := filepath.Join(dir, c.file)
			if _, err := os.Stat(path); err != nil {
				if c.file == "design.md" {
					fmt.Fprintf(os.Stderr, "○ %s missing (optional)\n", c.file)
					continue
				}
				fmt.Fprintf(os.Stderr, "✗ %s missing\n", c.file)
				allOK = false
				continue
			}
			printOK(c.label + " present: " + path)
		}

		if !allOK {
			return fmt.Errorf("validation failed for %s", feature)
		}
		fmt.Fprintln(os.Stderr, "\nValidation passed. Run tasks manually or via tsll run.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
