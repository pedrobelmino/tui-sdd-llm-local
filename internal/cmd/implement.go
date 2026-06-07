package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/workflow"
	"github.com/spf13/cobra"
)

var implementCmd = &cobra.Command{
	Use:   "implement [feature-name]",
	Short: "Implement a feature from spec (and tasks when present)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]
		svc := workflow.New()
		if !svc.Reachable(context.Background()) {
			cfg := loadCfg()
			return fmt.Errorf("ollama not reachable at %s", cfg.OllamaHost)
		}

		fmt.Fprintln(os.Stderr, "Implementing", feature, "with", svc.Model(), "...")
		usage, err := svc.Implement(context.Background(), ctx.Root, feature, func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		})
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return err
		}

		printOK(fmt.Sprintf("feature %s implemented (%d+%d tokens)", feature, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(implementCmd)
}
