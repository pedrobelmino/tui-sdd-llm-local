package tui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/config"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
)

var (
	loadProjectFn  = defaultLoadProject
	fetchOllamaFn  = defaultFetchOllama
	fetchGPUFn     = defaultFetchGPU
	fetchSystemFn  = defaultFetchSystem
	warmModelFn    = defaultWarmModel
	sharedMonitor  = system.NewMonitor()
)

func defaultLoadProject() tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return ProjectLoadedMsg{Ctx: project.ProjectContext{}, Features: nil}
	}

	ctx, err := project.FindProject(cwd)
	if err != nil {
		return ProjectLoadedMsg{Ctx: project.ProjectContext{}, Features: nil}
	}

	var features []project.FeatureEntry
	if ctx.Valid {
		features, _ = project.ListFeatures(ctx.Root)
	}

	return ProjectLoadedMsg{Ctx: ctx, Features: features}
}

func defaultFetchOllama() tea.Msg {
	cfg := config.Load()
	c := ollama.NewClient(cfg.OllamaHost)
	snap := ollama.FetchSnapshot(context.Background(), c, cfg.Model)
	return OllamaSnapshotMsg{Snapshot: snap}
}

func defaultFetchGPU() tea.Msg {
	snap, err := gpu.Query(context.Background())
	if err != nil {
		return GPUSnapshotMsg{Snapshot: gpu.Snapshot{Available: false, Error: err.Error()}}
	}
	return GPUSnapshotMsg{Snapshot: snap}
}

func defaultFetchSystem() tea.Msg {
	return SystemSnapshotMsg{Snapshot: sharedMonitor.Query(context.Background())}
}

func defaultWarmModel() tea.Msg {
	cfg := config.Load()
	client := ollama.NewGenerateClient(cfg.OllamaHost)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if !client.Reachable(ctx) {
		return ModelWarmupMsg{Err: nil}
	}
	_, _, err := client.Chat(ctx, ollama.ChatRequest{
		Model: cfg.Model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: "Warmup."},
			{Role: "user", Content: "ok"},
		},
	})
	return ModelWarmupMsg{Err: err}
}

func loadProjectCmd() tea.Cmd {
	return func() tea.Msg { return loadProjectFn() }
}

func fetchOllamaCmd() tea.Cmd {
	return func() tea.Msg { return fetchOllamaFn() }
}

func fetchGPUCmd() tea.Cmd {
	return func() tea.Msg { return fetchGPUFn() }
}

func fetchSystemCmd() tea.Cmd {
	return func() tea.Msg { return fetchSystemFn() }
}

func warmModelCmd() tea.Cmd {
	return func() tea.Msg { return warmModelFn() }
}

func refreshCmd() tea.Cmd {
	return tea.Batch(loadProjectCmd(), fetchOllamaCmd(), fetchSystemCmd())
}

func tickGPUCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return fetchGPUFn()
	})
}

func tickSystemCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return fetchSystemFn()
	})
}

func tickOllamaCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return fetchOllamaFn()
	})
}
