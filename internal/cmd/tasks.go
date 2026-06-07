package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/prompts"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/state"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/templates"
	"github.com/spf13/cobra"
)

var tasksCmd = &cobra.Command{
	Use:   "tasks [feature-name]",
	Short: "Break a feature into atomic tasks using local model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]
		cfg := loadCfg()
		client := newGenClient(cfg)

		if !client.Reachable(context.Background()) {
			return fmt.Errorf("ollama not reachable at %s", cfg.OllamaHost)
		}

		specPath := filepath.Join(ctx.Root, ".specs/features", feature, "spec.md")
		specData, err := os.ReadFile(specPath)
		if err != nil {
			return fmt.Errorf("read spec.md: %w (run tsll specify first)", err)
		}

		system := prompts.TasksSystem(ctx.Root)
		user := fmt.Sprintf("Feature: %s\n\nspec.md:\n%s\n\nGenerate complete tasks.md now.", feature, string(specData))

		fmt.Fprintln(os.Stderr, "Generating tasks with", cfg.Model, "...")
		out, usage, err := client.ChatStream(context.Background(), ollama.ChatRequest{
			Model: cfg.Model,
			Messages: []ollama.ChatMessage{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		}, func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		})
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return err
		}

		tasksPath := filepath.Join(ctx.Root, ".specs/features", feature, "tasks.md")
		content := templates.Tasks(out)
		if err := os.WriteFile(tasksPath, []byte(content), 0o644); err != nil {
			return err
		}

		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — tasks generated")
		printOK(fmt.Sprintf("wrote %s (%d+%d tokens)", tasksPath, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tasksCmd)
}
