package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/workflow"
	"github.com/spf13/cobra"
)

var askQuestion string

var askCmd = &cobra.Command{
	Use:   "ask [feature-name]",
	Short: "Ask questions about a feature (read-only, uses spec/design/tasks)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]
		question := strings.TrimSpace(askQuestion)
		if question == "" {
			question = promptLine("Your question about " + feature)
		}
		if question == "" {
			return fmt.Errorf("question required")
		}

		svc := workflow.New()
		if !svc.Reachable(context.Background()) {
			cfg := loadCfg()
			return fmt.Errorf("ollama not reachable at %s", cfg.OllamaHost)
		}

		fmt.Fprintf(os.Stderr, "Asking about %s with %s...\n", feature, svc.Model())
		out, usage, err := svc.Ask(context.Background(), ctx.Root, feature, question, func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		})
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Errorf("model returned empty answer")
		}

		printOK(fmt.Sprintf("answered (%d+%d tokens)", usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	askCmd.Flags().StringVar(&askQuestion, "question", "", "question to ask about the feature")
	rootCmd.AddCommand(askCmd)
}
