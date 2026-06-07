package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/prompts"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/state"
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
		cfg := loadCfg()
		client := newGenClient(cfg)

		if !client.Reachable(context.Background()) {
			return fmt.Errorf("ollama not reachable at %s", cfg.OllamaHost)
		}

		featureDir := filepath.Join(ctx.Root, ".specs/features", feature)
		tasksPath := filepath.Join(featureDir, "tasks.md")
		specPath := filepath.Join(featureDir, "spec.md")

		tasksData, err := os.ReadFile(tasksPath)
		if err != nil {
			return fmt.Errorf("read tasks.md: %w", err)
		}
		specData, _ := os.ReadFile(specPath)

		taskID := runTaskID
		if taskID == "" {
			taskID = promptLine("Task ID (e.g. T1)")
		}
		taskBlock := extractTaskBlock(string(tasksData), taskID)
		if taskBlock == "" {
			return fmt.Errorf("task %s not found in tasks.md", taskID)
		}

		system := prompts.RunSystem(ctx.Root, taskBlock, string(specData))
		user := fmt.Sprintf("Implement task %s now. List files to change and provide code changes.", taskID)

		fmt.Fprintln(os.Stderr, "Running", taskID, "with", cfg.Model, "...")
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

		_ = state.UpdateTaskStatus(tasksPath, taskID, "✅ Done")
		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — "+taskID+" executed")
		_ = state.AppendDecision(statePath, "Task "+taskID+" executed",
			"Completed via tsll run", truncateOut(out, 200))

		printOK(fmt.Sprintf("task %s complete (%d+%d tokens)", taskID, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func extractTaskBlock(tasks, taskID string) string {
	re := regexp.MustCompile(`(?ms)(### ` + regexp.QuoteMeta(taskID) + `:.*?)(?:\n---|\n### T|\z)`)
	m := re.FindStringSubmatch(tasks)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// fallback: line containing task ID
	lines := strings.Split(tasks, "\n")
	var block []string
	capture := false
	for _, line := range lines {
		if strings.Contains(line, taskID) && strings.HasPrefix(strings.TrimSpace(line), "###") {
			capture = true
		}
		if capture {
			block = append(block, line)
			if strings.TrimSpace(line) == "---" && len(block) > 2 {
				break
			}
		}
	}
	return strings.Join(block, "\n")
}

func truncateOut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
	runCmd.Flags().StringVar(&runTaskID, "task", "", "task ID (e.g. T1)")
	rootCmd.AddCommand(runCmd)
}
