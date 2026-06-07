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

type featureContext struct {
	Feature      string
	FeatureDir   string
	Spec         string
	Design       string
	Tasks        string
	ProjectStack string
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

func (s *Service) toolCtx(ctx context.Context) context.Context {
	return ollama.WithModel(ctx, s.cfg.Model)
}

// implementToolCtx sets the default tool-calling loop budget for implement without a task block.
func (s *Service) implementToolCtx(ctx context.Context) context.Context {
	return ollama.WithToolLoopLimit(s.toolCtx(ctx), estimateTaskLoopLimit("", s.cfg.FastMode))
}

// implementTaskCtx sets loop budget from the task description (scaffolding needs more turns).
func (s *Service) implementTaskCtx(ctx context.Context, taskBlock string) context.Context {
	return ollama.WithToolLoopLimit(s.toolCtx(ctx), estimateTaskLoopLimit(taskBlock, s.cfg.FastMode))
}

// estimateTaskLoopLimit sizes the tool loop for how many files a task likely needs.
func estimateTaskLoopLimit(taskBlock string, fastMode bool) int {
	scope := classifyTaskScope(taskBlock)
	switch scope {
	case ScopeScaffold:
		limit := 60
		if !fastMode {
			limit = 80
		}
		lower := strings.ToLower(taskBlock)
		for _, kw := range []string{"workflow", "ci/cd", ".github", "docker"} {
			if strings.Contains(lower, kw) {
				limit += 8
			}
		}
		if limit > 100 {
			limit = 100
		}
		return limit
	case ScopeFocused:
		if fastMode {
			return 20
		}
		return 28
	case ScopeSection:
		if fastMode {
			return 22
		}
		return 28
	default:
		if fastMode {
			return 30
		}
		return 36
	}
}

func (s *Service) warmModel(ctx context.Context) {
	// Best-effort warm-up to avoid cold-start latency spikes.
	_, _, _ = s.client.Chat(ctx, ollama.ChatRequest{
		Model: s.cfg.Model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: "You are warm-up assistant."},
			{Role: "user", Content: "ok"},
		},
	})
}

// Specify generates spec.md for a feature.
func (s *Service) Specify(ctx context.Context, projectRoot, feature, brief string, onChunk func(string)) (ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}
	if strings.TrimSpace(brief) == "" {
		return ollama.TokenUsage{}, fmt.Errorf("feature description required")
	}

	system := prompts.SpecifySystem(projectRoot)
	featureDir := filepath.Join(projectRoot, ".specs/features", feature)
	specPath := filepath.Join(featureDir, "spec.md")
	var existingSpec string
	if data, err := os.ReadFile(specPath); err == nil {
		existingSpec = string(data)
		if onChunk != nil {
			onChunk("\n📄 existing spec.md found — evolving current document\n")
		}
	}

	user := fmt.Sprintf("Feature name: %s\n\nDescription:\n%s\n\nGenerate complete spec.md now.", feature, brief)
	if existingSpec != "" {
		user += "\n\nCurrent spec.md (evolve this document):\n" + truncate(existingSpec, 7000)
	}

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

	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		return usage, err
	}
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
	designPath := filepath.Join(featureDir, "design.md")
	if existingDesign, err := os.ReadFile(designPath); err == nil {
		if onChunk != nil {
			onChunk("\n📄 existing design.md found — evolving current document\n")
		}
		user += "\n\ndesign.md (current version to evolve):\n" + truncate(string(existingDesign), 7000)
	}
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
	tasksPath := filepath.Join(projectRoot, ".specs/features", feature, "tasks.md")
	if existingTasks, err := os.ReadFile(tasksPath); err == nil {
		if onChunk != nil {
			onChunk("\n📄 existing tasks.md found — evolving current document\n")
		}
		user += "\n\ntasks.md (current version to evolve):\n" + truncate(string(existingTasks), 7000)
	}

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

	fc, err := s.loadFeatureContext(projectRoot, feature)
	if err != nil {
		return ollama.TokenUsage{}, err
	}
	if s.cfg.FastMode && onChunk != nil {
		onChunk("⚡ fast mode enabled (compact context)\n")
	}
	logContextSummary(fc, onChunk)
	s.warmModel(ctx)

	tasksPath := filepath.Join(fc.FeatureDir, "tasks.md")
	allTasks := project.ParseTasksContent(fc.Tasks)
	codeTasks := project.CodeTasks(allTasks)
	if len(codeTasks) > 0 {
		var total ollama.TokenUsage
		for {
			// Re-read tasks.md each iteration so done marks persist and re-runs resume correctly.
			fresh, loadErr := s.loadFeatureContext(projectRoot, feature)
			if loadErr != nil {
				return total, loadErr
			}
			pending := project.ImplementableTasks(project.ParseTasksContent(fresh.Tasks))
			if len(pending) == 0 {
				if onChunk != nil {
					onChunk("\n✓ all code tasks already done — nothing left to implement\n")
				}
				statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
				_ = state.UpdateCurrentWork(statePath, feature+" — implemented")
				return total, nil
			}
			task := pending[0]
			if onChunk != nil {
				onChunk(fmt.Sprintf("\n\n--- Implementing %s: %s ---\n\n", task.ID, task.Title))
			}
			usage, runErr := s.runTaskWithContext(ctx, projectRoot, fresh, task.ID, onChunk)
			total.PromptTokens += usage.PromptTokens
			total.CompletionTokens += usage.CompletionTokens
			if runErr != nil {
				_ = project.UpdateTaskStatus(tasksPath, task.ID, "Pending")
				return total, runErr
			}
		}
	}

	user := buildImplementUserMsg(projectRoot, fc, "", s.cfg.FastMode)
	system := prompts.ImplementSystem(projectRoot)
	msgs := []ollama.ChatMessageWithTools{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	runCtx := ollama.WithMinFileWrites(s.implementToolCtx(ctx), 3)
	runCtx = ollama.WithTaskPlanMode(runCtx, true)
	runCtx = ollama.WithSingleTouch(runCtx, true)
	out, usage, err := s.client.ChatWithTools(
		runCtx, msgs,
		fileops.Definitions(),
		fileops.Executor(projectRoot),
		onChunk,
	)
	if err != nil {
		return usage, err
	}

	if err := os.WriteFile(filepath.Join(fc.FeatureDir, "implement.done"), []byte(out), 0o644); err != nil {
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
	if s.cfg.FastMode && onChunk != nil {
		onChunk("⚡ fast mode enabled (compact context)\n")
	}
	s.warmModel(ctx)
	fc, err := s.loadFeatureContext(projectRoot, feature)
	if err != nil {
		return ollama.TokenUsage{}, err
	}
	logContextSummary(fc, onChunk)
	return s.runTaskWithContext(ctx, projectRoot, fc, taskID, onChunk)
}

func (s *Service) runTaskWithContext(ctx context.Context, projectRoot string, fc featureContext, taskID string, onChunk func(string)) (ollama.TokenUsage, error) {
	block := extractTaskBlock(fc.Tasks, taskID)
	if block == "" {
		return ollama.TokenUsage{}, fmt.Errorf("task %s not found", taskID)
	}

	tasksPath := filepath.Join(fc.FeatureDir, "tasks.md")
	_ = project.UpdateTaskStatus(tasksPath, taskID, "In Progress")

	system := prompts.RunSystem(projectRoot, block, fc.Spec)
	scope := classifyTaskScope(block)
	user := buildImplementUserMsg(projectRoot, fc, block, s.cfg.FastMode)

	msgs := []ollama.ChatMessageWithTools{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	runCtx := ollama.WithMinFileWrites(s.implementTaskCtx(ctx, block), estimateMinFileWrites(block))
	runCtx = ollama.WithTaskPlanMode(runCtx, true)
	runCtx = ollama.WithSingleTouch(runCtx, true)
	runCtx = ollama.WithMaxPlanFiles(runCtx, maxPlanFilesForScope(scope))
	runCtx = ollama.WithFocusedAllowlist(runCtx, focusedAllowedFragments(block))
	if onChunk != nil {
		onChunk(fmt.Sprintf("📎 turn budget: %d · scope: %s · max %d files in plan\n",
			estimateTaskLoopLimit(block, s.cfg.FastMode), scope, maxPlanFilesForScope(scope)))
	}
	out, usage, err := s.client.ChatWithTools(
		runCtx, msgs,
		fileops.Definitions(),
		fileops.Executor(projectRoot),
		onChunk,
	)
	if err != nil {
		_ = project.UpdateTaskStatus(tasksPath, taskID, "Pending")
		return usage, err
	}

	if err := project.UpdateTaskStatus(tasksPath, taskID, "✅ Done"); err != nil {
		if onChunk != nil {
			onChunk(fmt.Sprintf("⚠ could not update tasks.md for %s: %v\n", taskID, err))
		}
	} else if onChunk != nil {
		rel, _ := filepath.Rel(projectRoot, tasksPath)
		onChunk(fmt.Sprintf("✓ marked %s done in %s\n", taskID, rel))
	}
	statePath := filepath.Join(projectRoot, ".specs/project/STATE.md")
	_ = state.UpdateCurrentWork(statePath, fc.Feature+" — "+taskID+" executed")
	_ = state.AppendDecision(statePath, "Task "+taskID+" executed", truncate(out, 200), "tsll run from TUI/CLI")

	return usage, nil
}

// Ask answers a read-only question about a feature using spec/design/tasks context.
func (s *Service) Ask(ctx context.Context, projectRoot, feature, question string, onChunk func(string)) (string, ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return "", ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}
	if strings.TrimSpace(question) == "" {
		return "", ollama.TokenUsage{}, fmt.Errorf("question required")
	}

	fc, err := s.loadFeatureContext(projectRoot, feature)
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}

	system := prompts.AskSystem(projectRoot)
	user := buildAskUserMsg(fc, question, s.cfg.FastMode)
	out, usage, err := s.client.ChatStream(ctx, ollama.ChatRequest{
		Model: s.cfg.Model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}, onChunk)
	if err != nil {
		return out, usage, err
	}
	return out, usage, nil
}

// QuickTask executes an ad-hoc request, optionally scoped to a feature.
// It uses the same file tools loop used by run/implement.
func (s *Service) QuickTask(ctx context.Context, projectRoot, feature, request string, onChunk func(string)) (string, ollama.TokenUsage, error) {
	if !s.Reachable(ctx) {
		return "", ollama.TokenUsage{}, fmt.Errorf("ollama not reachable at %s", s.cfg.OllamaHost)
	}
	if strings.TrimSpace(request) == "" {
		return "", ollama.TokenUsage{}, fmt.Errorf("quick task request required")
	}
	if s.cfg.FastMode && onChunk != nil {
		onChunk("⚡ fast mode enabled (compact context)\n")
	}
	s.warmModel(ctx)

	// Build base prompt; if feature exists, preload its context.
	user := "Quick task request:\n" + request + "\n\nApply changes using file tools."
	if strings.TrimSpace(feature) != "" {
		if fc, err := s.loadFeatureContext(projectRoot, feature); err == nil {
			user = buildImplementUserMsg(projectRoot, fc, "", s.cfg.FastMode) + "\n\n---\n" + user
		}
	}

	msgs := []ollama.ChatMessageWithTools{
		{Role: "system", Content: prompts.ImplementSystem(projectRoot)},
		{Role: "user", Content: user},
	}
	out, usage, err := s.client.ChatWithTools(
		s.toolCtx(ctx), msgs,
		fileops.Definitions(),
		fileops.Executor(projectRoot),
		onChunk,
	)
	return out, usage, err
}

func (s *Service) loadFeatureContext(projectRoot, feature string) (featureContext, error) {
	featureDir := filepath.Join(projectRoot, ".specs/features", feature)
	specPath := filepath.Join(featureDir, "spec.md")
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return featureContext{}, fmt.Errorf("read spec.md: %w (run specify first)", err)
	}
	fc := featureContext{
		Feature:    feature,
		FeatureDir: featureDir,
		Spec:       string(specData),
	}
	if designData, err := os.ReadFile(filepath.Join(featureDir, "design.md")); err == nil {
		fc.Design = string(designData)
	}
	if tasksData, err := os.ReadFile(filepath.Join(featureDir, "tasks.md")); err == nil {
		fc.Tasks = string(tasksData)
	}
	fc.ProjectStack = loadProjectStack(projectRoot)
	return fc, nil
}

func estimateMinFileWrites(taskBlock string) int {
	if taskBlock == "" {
		return 1
	}
	switch classifyTaskScope(taskBlock) {
	case ScopeScaffold:
		return 8
	case ScopeFocused:
		return 2
	case ScopeSection:
		return 3
	default:
		return 2
	}
}

func loadProjectStack(projectRoot string) string {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".specs/project/PROJECT.md"))
	if err != nil {
		return ""
	}
	content := string(data)
	if idx := strings.Index(content, "## Tech Stack"); idx >= 0 {
		content = content[idx:]
	}
	return truncate(content, 1200)
}

// buildImplementUserMsg constructs the user message for Run/Implement.
// taskBlock is empty for full-feature implementation.
func buildAskUserMsg(fc featureContext, question string, fastMode bool) string {
	var b strings.Builder
	b.WriteString("Feature: " + fc.Feature + "\n\n")
	b.WriteString("## Question\n\n")
	b.WriteString(strings.TrimSpace(question) + "\n\n")

	if fc.ProjectStack != "" {
		b.WriteString("## Project tech stack\n\n")
		b.WriteString(fc.ProjectStack + "\n\n")
	}
	if fc.Spec != "" {
		max := 5000
		if fastMode {
			max = 2500
		}
		b.WriteString("## spec.md\n\n")
		b.WriteString(truncate(fc.Spec, max) + "\n\n")
	}
	if fc.Design != "" {
		max := 4000
		if fastMode {
			max = 2000
		}
		b.WriteString("## design.md\n\n")
		b.WriteString(truncate(fc.Design, max) + "\n\n")
	}
	if fc.Tasks != "" {
		max := 3000
		if fastMode {
			max = 1500
		}
		b.WriteString("## tasks.md\n\n")
		b.WriteString(truncate(fc.Tasks, max) + "\n\n")
	}

	b.WriteString("---\n")
	b.WriteString("Answer the question using the documents above. Do not invent requirements not present in the specs.")
	return b.String()
}

func buildImplementUserMsg(projectRoot string, fc featureContext, taskBlock string, fastMode bool) string {
	var b strings.Builder

	b.WriteString("Feature slug: " + fc.Feature + "\n")
	b.WriteString("Feature spec directory: .specs/features/" + fc.Feature + "/\n")
	b.WriteString("IMPORTANT: The feature slug is \"" + fc.Feature + "\" — NOT requirement IDs from spec.md (e.g. FEAT-01).\n")
	b.WriteString("tsll updates .specs/features/" + fc.Feature + "/tasks.md automatically — NEVER write or edit anything under .specs/.\n")
	if fc.ProjectStack != "" {
		b.WriteString("\n## Project tech stack (AUTHORITATIVE — override conflicting design.md frameworks)\n\n")
		b.WriteString(fc.ProjectStack + "\n")
		if guard := stackLanguageGuard(fc.ProjectStack); guard != "" {
			b.WriteString(guard + "\n")
		}
	}

	// List the feature dir so the model knows what files exist there.
	if entries, err := os.ReadDir(fc.FeatureDir); err == nil {
		b.WriteString("Files in .specs/features/" + fc.Feature + "/:\n")
		limit := len(entries)
		if fastMode && limit > 12 {
			limit = 12
		}
		for _, e := range entries[:limit] {
			b.WriteString("  " + e.Name() + "\n")
		}
		if limit < len(entries) {
			b.WriteString("  ...\n")
		}
	}
	b.WriteString("\n")

	if tree := formatExistingSourceTree(projectRoot, 50); tree != "" {
		b.WriteString(tree)
		b.WriteString("\n")
	} else if fc.ProjectStack != "" {
		b.WriteString(GreenfieldPathHint(fc.ProjectStack))
		b.WriteString("\n\n")
	}

	if taskBlock != "" {
		scope := classifyTaskScope(taskBlock)
		b.WriteString(scopeHint(taskBlock, scope))
		b.WriteString("\n")
		b.WriteString("## Task to implement (ONLY this task — follow spec.md + design.md)\n\n")
		b.WriteString(taskBlock + "\n\n")
	} else if pending := project.ImplementableTasks(project.ParseTasksContent(fc.Tasks)); len(pending) > 0 {
		b.WriteString("## Implementation checklist (all must be completed across tool calls)\n\n")
		for _, t := range pending {
			b.WriteString("- [ ] " + t.ID + ": " + t.Title + "\n")
		}
		b.WriteString("\n")
	}

	if fc.Spec != "" {
		specMax := 3000
		if fastMode {
			specMax = 1400
		}
		b.WriteString("## spec.md (already loaded — do NOT read via tool)\n\n")
		b.WriteString(truncate(fc.Spec, specMax) + "\n\n")
	}
	if fc.Design != "" {
		designMax := 2000
		if fastMode {
			designMax = 900
		}
		b.WriteString("## design.md (already loaded — do NOT read via tool)\n\n")
		b.WriteString(truncate(fc.Design, designMax) + "\n\n")
	}
	if taskBlock == "" && fc.Tasks != "" {
		tasksMax := 1500
		if fastMode {
			tasksMax = 700
		}
		b.WriteString("## tasks.md (already loaded — do NOT read via tool)\n\n")
		b.WriteString(truncate(fc.Tasks, tasksMax) + "\n\n")
	}

	b.WriteString("---\n")
	b.WriteString("The spec files above are already in context. Use file tools ONLY for SOURCE CODE files (not spec/design/tasks).\n")
	if taskBlock != "" {
		b.WriteString("Implement ONLY the task above — not other sections of the app. Follow spec.md, design.md, and the project tech stack.\n")
		b.WriteString("## Per-task workflow (one touch per file)\n")
		b.WriteString("1) FIRST response: <file_plan> with ONLY the paths this task needs (one per line).\n")
		b.WriteString("   Derive the plan from this task's \"What/Where\" in tasks.md; add extra files ONLY if spec.md/design.md require them.\n")
		b.WriteString("2) THEN write_file each path ONCE with complete final content — no edit_file, no second write to same path.\n")
		b.WriteString("   Scaffold tasks only: optional <task_plan>{\"files\":[...]}</task_plan> one-shot.\n")
		b.WriteString("3) Plain-text summary only when every planned file is written.\n")
	} else {
		b.WriteString("Implement the complete feature per spec.md, design.md, and tasks.md.\n")
		b.WriteString("Build your <file_plan> from the tasks.md checklist above; add extra files ONLY when spec.md/design.md require them.\n")
		b.WriteString("Create ALL required source files. Do NOT stop after one file or a JSON summary.\n")
	}

	return b.String()
}

func extractTaskBlock(tasks, taskID string) string {
	patterns := []string{
		`(?ms)(### ` + regexp.QuoteMeta(taskID) + `:\s*.*?)(?:\n---|\n### |\z)`,
		`(?ms)(###[^\n]*\(` + regexp.QuoteMeta(taskID) + `\)[^\n]*\n.*?)(?:\n---|\n### |\z)`,
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(tasks); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// logContextSummary makes it explicit, in the run log, which spec documents are
// being considered during implementation (and the authoritative stack).
func logContextSummary(fc featureContext, onChunk func(string)) {
	if onChunk == nil {
		return
	}
	mark := func(name, content string) string {
		if strings.TrimSpace(content) == "" {
			return name + " (ausente)"
		}
		return fmt.Sprintf("%s (%d chars)", name, len(content))
	}
	onChunk(fmt.Sprintf("📚 contexto considerado: %s · %s · %s\n",
		mark("spec.md", fc.Spec), mark("design.md", fc.Design), mark("tasks.md", fc.Tasks)))
	if label := detectStackLabel(fc.ProjectStack); label != "" {
		onChunk("🧱 stack do projeto (autoritativa): " + label + "\n")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
