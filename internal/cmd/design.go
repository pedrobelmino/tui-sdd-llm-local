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

var designCmd = &cobra.Command{
	Use:   "design [feature-name]",
	Short: "Generate design.md for a feature using local model",
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

		featureDir := filepath.Join(ctx.Root, ".specs/features", feature)
		specPath := filepath.Join(featureDir, "spec.md")
		specData, err := os.ReadFile(specPath)
		if err != nil {
			return fmt.Errorf("read spec.md: %w (run tsll specify first)", err)
		}

		user := fmt.Sprintf("Feature: %s\n\nspec.md:\n%s\n\nGenerate complete design.md now.", feature, string(specData))
		if contextData, err := os.ReadFile(filepath.Join(featureDir, "context.md")); err == nil {
			user += "\n\ncontext.md:\n" + string(contextData)
		}
		if concernsData, err := os.ReadFile(filepath.Join(ctx.Root, ".specs/codebase/CONCERNS.md")); err == nil {
			user += "\n\nCONCERNS.md:\n" + string(concernsData)
		}

		system := prompts.DesignSystem(ctx.Root)
		fmt.Fprintln(os.Stderr, "Generating design with", cfg.Model, "...")
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

		designPath := filepath.Join(featureDir, "design.md")
		content := templates.Design(feature, out)
		if err := os.WriteFile(designPath, []byte(content), 0o644); err != nil {
			return err
		}

		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — design generated")
		printOK(fmt.Sprintf("wrote %s (%d+%d tokens)", designPath, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(designCmd)
}
