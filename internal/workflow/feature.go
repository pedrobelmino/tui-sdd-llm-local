package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/config"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/fileops"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/prompts"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/state"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/templates"
)

// Service runs tsll feature workflows (specify, tasks, run).
type Service struct {
	cfg    config.Config
	client ollama.GenerateClientWithTools
}

// New creates a workflow service with config defaults.
func New() *Service {
	cfg := config.Load()
	return &Service{
		cfg:    cfg,
		client: ollama.NewGenerateClient(cfg.OllamaHost).(ollama.GenerateClientWithTools),
	}
}

// Reachable reports if Ollama is up.
func (s *Service) Reachable(ctx context.Context) bool {
	return s.client.Reachable(ctx)
}

// Model returns configured model name.
func (s *Service) Model() string { return s.cfg.Model }

// Specify generates spec.md for a feature.
func (s *Service) Specify(ctx context.Context, projectRoot, feature, brief string, onChunk func(string)) (ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}
	if strings.TrimSpace(brief) == "" {
		return ollama.TokenUsage{}, fmt.Errorf("feature description required")
	}

	system := prompts.SpecifySystem(projectRoot)
	user := fmt.Sprintf("Feature name: %s\n\nDescription:\n%s\n\nGenerate complete spec.md now.", feature, brief)

	out, usage, err := s.client.ChatStream(ctx, ollama.ChatRequest{
		Model: s.cfg.Model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}, onChunk)
	if err != nil {
		return usage, err
	}

	featureDir := filepath.Join(projectRoot, ".specs/features", feature)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		return usage, err
	}
	specPath := filepath.Join(featureDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(templates.Spec(feature, out)), 0o644); err != nil {
		return usage, err
	}

	statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
	_ = state.UpdateCurrentWork(statePath, feature+" — spec generated")
	return usage, nil
}

// Design generates design.md from spec.md.
func (s *Service) Design(ctx context.Context, projectRoot, feature string, onChunk func(string)) (ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}

	featureDir := filepath.Join(projectRoot, ".specs/features", feature)
	specPath := filepath.Join(featureDir, "spec.md")
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return ollama.TokenUsage{}, fmt.Errorf("read spec.md: %w (run specify first)", err)
	}

	user := fmt.Sprintf("Feature: %s\n\nspec.md:\n%s\n\nGenerate complete design.md now.", feature, string(specData))
	if contextData, err := os.ReadFile(filepath.Join(featureDir, "context.md")); err == nil {
		user += "\n\ncontext.md:\n" + string(contextData)
	}
	if concernsData, err := os.ReadFile(filepath.Join(projectRoot, ".specs/codebase/CONCERNS.md")); err == nil {
		user += "\n\nCONCERNS.md:\n" + truncate(string(concernsData), 4000)
	}

	system := prompts.DesignSystem(projectRoot)
	out, usage, err := s.client.ChatStream(ctx, ollama.ChatRequest{
		Model: s.cfg.Model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}, onChunk)
	if err != nil {
		return usage, err
	}

	designPath := filepath.Join(featureDir, "design.md")
	if err := os.WriteFile(designPath, []byte(templates.Design(feature, out)), 0o644); err != nil {
		return usage, err
	}

	statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
	_ = state.UpdateCurrentWork(statePath, feature+" — design generated")
	return usage, nil
}

// Tasks generates tasks.md from spec.md.
func (s *Service) Tasks(ctx context.Context, projectRoot, feature string, onChunk func(string)) (ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}

	specPath := filepath.Join(projectRoot, ".specs/features", feature, "spec.md")
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return ollama.TokenUsage{}, fmt.Errorf("read spec.md: %w (run specify first)", err)
	}

	system := prompts.TasksSystem(projectRoot)
	user := fmt.Sprintf("Feature: %s\n\nspec.md:\n%s\n\nGenerate complete tasks.md now.", feature, string(specData))

	out, usage, err := s.client.ChatStream(ctx, ollama.ChatRequest{
		Model: s.cfg.Model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}, onChunk)
	if err != nil {
		return usage, err
	}

	tasksPath := filepath.Join(projectRoot, ".specs/features", feature, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(templates.Tasks(out)), 0o644); err != nil {
		return usage, err
	}

	statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
	_ = state.UpdateCurrentWork(statePath, feature+" — tasks generated")
	return usage, nil
}

// Implement executes all pending tasks or, without tasks.md, implements the feature from spec.md.
func (s *Service) Implement(ctx context.Context, projectRoot, feature string, onChunk func(string)) (ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}

	featureDir := filepath.Join(projectRoot, ".specs/features", feature)
	specPath := filepath.Join(featureDir, "spec.md")
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return ollama.TokenUsage{}, fmt.Errorf("read spec.md: %w (run specify first)", err)
	}

	tasksPath := filepath.Join(featureDir, "tasks.md")
	if tasksData, err := os.ReadFile(tasksPath); err == nil {
		tasks := project.ParseTasksContent(string(tasksData))
		if len(tasks) > 0 {
			var total ollama.TokenUsage
			var ran bool
			for _, task := range tasks {
				if task.Status == "Done" {
					continue
				}
				ran = true
				if onChunk != nil {
					onChunk(fmt.Sprintf("\n\n--- Implementing %s: %s ---\n\n", task.ID, task.Title))
				}
				usage, err := s.Run(ctx, projectRoot, feature, task.ID, onChunk)
				total.PromptTokens += usage.PromptTokens
				total.CompletionTokens += usage.CompletionTokens
				if err != nil {
					return total, err
				}
			}
			if !ran {
				return total, fmt.Errorf("feature already fully implemented")
			}
			statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
			_ = state.UpdateCurrentWork(statePath, feature+" — implemented")
			return total, nil
		}
	}

	user := buildImplementUserMsg(projectRoot, feature, featureDir, "", specData, nil)

	system := prompts.ImplementSystem(projectRoot)
	msgs := []ollama.ChatMessageWithTools{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	toolCtx := ollama.WithModel(ctx, s.cfg.Model)
	out, usage, err := s.client.ChatWithTools(
		toolCtx, msgs,
		fileops.Definitions(),
		fileops.Executor(projectRoot),
		onChunk,
	)
	if err != nil {
		return usage, err
	}

	if err := os.WriteFile(filepath.Join(featureDir, "implement.done"), []byte(out), 0o644); err != nil {
		return usage, err
	}

	statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
	_ = state.UpdateCurrentWork(statePath, feature+" — implemented")
	_ = state.AppendDecision(statePath, "Feature "+feature+" implemented",
		"Completed via tsll implement", truncate(out, 200))
	return usage, nil
}

// Run executes a single task with the local model, using file-operation tools
// so the model can create/edit/delete files directly on disk.
func (s *Service) Run(ctx context.Context, projectRoot, feature, taskID string, onChunk func(string)) (ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}

	featureDir := filepath.Join(projectRoot, ".specs/features", feature)
	tasksPath := filepath.Join(featureDir, "tasks.md")
	specPath := filepath.Join(featureDir, "spec.md")

	tasksData, err := os.ReadFile(tasksPath)
	if err != nil {
		return ollama.TokenUsage{}, fmt.Errorf("read tasks.md: %w", err)
	}
	specData, _ := os.ReadFile(specPath)

	block := extractTaskBlock(string(tasksData), taskID)
	if block == "" {
		return ollama.TokenUsage{}, fmt.Errorf("task %s not found", taskID)
	}

	system := prompts.RunSystem(projectRoot, block, string(specData))
	user := buildImplementUserMsg(projectRoot, feature, featureDir, block, specData, nil)

	msgs := []ollama.ChatMessageWithTools{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	toolCtx := ollama.WithModel(ctx, s.cfg.Model)
	out, usage, err := s.client.ChatWithTools(
		toolCtx, msgs,
		fileops.Definitions(),
		fileops.Executor(projectRoot),
		onChunk,
	)
	if err != nil {
		return usage, err
	}

	_ = state.UpdateTaskStatus(tasksPath, taskID, "✅ Done")
	statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
	_ = state.UpdateCurrentWork(statePath, feature+" — "+taskID+" executed")
	_ = state.AppendDecision(statePath, "Task "+taskID+" executed", truncate(out, 200), "tsll run from TUI/CLI")

	return usage, nil
}

// buildImplementUserMsg constructs the user message for Run/Implement, providing
// all spec artefacts inline so the model does not try to re-read them via tools.
// taskBlock is empty for full-feature implementation (Implement without tasks.md).
func buildImplementUserMsg(projectRoot, feature, featureDir, taskBlock string, specData []byte, _ []byte) string {
	var b strings.Builder

	b.WriteString("Feature: " + feature + "\n")
	b.WriteString("Feature spec directory: .specs/features/" + feature + "/\n")

	// List the feature dir so the model knows what files exist there.
	if entries, err := os.ReadDir(featureDir); err == nil {
		b.WriteString("Files in .specs/features/" + feature + "/:\n")
		for _, e := range entries {
			b.WriteString("  " + e.Name() + "\n")
		}
	}
	b.WriteString("\n")

	if taskBlock != "" {
		b.WriteString("## Task to implement\n\n")
		b.WriteString(taskBlock + "\n\n")
	}

	if len(specData) > 0 {
		b.WriteString("## spec.md (already loaded — do NOT read via tool)\n\n")
		b.WriteString(truncate(string(specData), 3000) + "\n\n")
	}
	if designData, err := os.ReadFile(filepath.Join(featureDir, "design.md")); err == nil {
		b.WriteString("## design.md (already loaded — do NOT read via tool)\n\n")
		b.WriteString(truncate(string(designData), 2000) + "\n\n")
	}
	if tasksData, err := os.ReadFile(filepath.Join(featureDir, "tasks.md")); err == nil {
		b.WriteString("## tasks.md (already loaded — do NOT read via tool)\n\n")
		b.WriteString(truncate(string(tasksData), 1500) + "\n\n")
	}

	b.WriteString("---\n")
	b.WriteString("The spec files above are already in context. Use file tools ONLY for SOURCE CODE files (not spec/design/tasks).\n")
	if taskBlock != "" {
		b.WriteString("Implement the task above: read any relevant source files, then write/edit/create the necessary code files.\n")
	} else {
		b.WriteString("Implement the complete feature: read any relevant source files, then write/edit/create all necessary code files.\n")
	}

	return b.String()
}

func extractTaskBlock(tasks, taskID string) string {
	re := regexp.MustCompile(`(?ms)(### ` + regexp.QuoteMeta(taskID) + `:.*?)(?:\n---|\n### T|\z)`)
	if m := re.FindStringSubmatch(tasks); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
