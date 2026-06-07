package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/workflow"
	"github.com/spf13/cobra"
)

var runTaskID string

var runCmd = &cobra.Command{
	Use:   "run [feature-name]",
	Short: "Execute a task with the local model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]
		taskID := runTaskID
		if taskID == "" {
			taskID = promptLine("Task ID (e.g. T1)")
		}

		svc := workflow.New()
		if !svc.Reachable(context.Background()) {
			cfg := loadCfg()
			return fmt.Errorf("ollama not reachable at %s", cfg.OllamaHost)
		}
		fmt.Fprintln(os.Stderr, "Running", taskID, "with", svc.Model(), "...")
		usage, err := svc.Run(context.Background(), ctx.Root, feature, taskID, func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		})
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return err
		}

		printOK(fmt.Sprintf("task %s complete (%d+%d tokens)", taskID, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	runCmd.Flags().StringVar(&runTaskID, "task", "", "task ID (e.g. T1)")
	rootCmd.AddCommand(runCmd)
}
