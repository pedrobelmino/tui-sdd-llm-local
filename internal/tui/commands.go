package tui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	c := ollama.NewClient("")
	snap := ollama.FetchSnapshot(context.Background(), c)
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
