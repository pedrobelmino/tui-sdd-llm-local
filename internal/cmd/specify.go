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

var specifyBrief string

var specifyCmd = &cobra.Command{
	Use:   "specify [feature-name]",
	Short: "Generate spec.md for a feature using local model",
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

		brief := specifyBrief
		if brief == "" {
			brief = promptMultiline("Describe the feature")
		}
		if brief == "" {
			return fmt.Errorf("feature description required")
		}

		system := prompts.SpecifySystem(ctx.Root)
		user := fmt.Sprintf("Feature name: %s\n\nDescription:\n%s\n\nGenerate complete spec.md now.", feature, brief)

		fmt.Fprintln(os.Stderr, "Generating spec with", cfg.Model, "...")
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

		featureDir := filepath.Join(ctx.Root, ".specs/features", feature)
		if err := os.MkdirAll(featureDir, 0o755); err != nil {
			return err
		}

		specPath := filepath.Join(featureDir, "spec.md")
		content := templates.Spec(feature, out)
		if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
			return err
		}

		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — spec generated")
		printOK(fmt.Sprintf("wrote %s (%d+%d tokens)", specPath, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	specifyCmd.Flags().StringVar(&specifyBrief, "brief", "", "feature description (skip prompt)")
	rootCmd.AddCommand(specifyCmd)
}
