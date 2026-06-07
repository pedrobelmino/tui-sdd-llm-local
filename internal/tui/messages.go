package tui

import (
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
)

// OllamaSnapshotMsg delivers fetched Ollama state into the TUI loop.
type OllamaSnapshotMsg struct {
	Snapshot ollama.Snapshot
}

// GPUSnapshotMsg delivers GPU metrics into the TUI loop.
type GPUSnapshotMsg struct {
	Snapshot gpu.Snapshot
}

// ProjectLoadedMsg delivers project context and feature list.
type ProjectLoadedMsg struct {
	Ctx      project.ProjectContext
	Features []project.FeatureEntry
}

// TokenUsageMsg records token usage from a completed request.
type TokenUsageMsg struct {
	Usage tokens.UsageSnapshot
}

// RefreshRequestedMsg is sent when the user presses r.
type RefreshRequestedMsg struct{}

// SystemSnapshotMsg delivers host CPU/RAM/disk metrics into the TUI loop.
type SystemSnapshotMsg struct {
	Snapshot system.Snapshot
}

// ActionChunkMsg streams LLM output during specify/tasks/run.
type ActionChunkMsg struct {
	Text string
}

// ActionFinishedMsg completes an async workflow action.
type ActionFinishedMsg struct {
	Kind    ActionKind
	Feature string
	TaskID  string
	Output  string
	Usage   ollama.TokenUsage
	Err error
}

// FeatureTasksMsg delivers parsed tasks for the detail view.
type FeatureTasksMsg struct {
	Feature string
	Tasks   []project.TaskEntry
}

// ActionTickMsg polls streaming buffer (internal).
type ActionTickMsg struct{}

// ModelWarmupMsg reports best-effort model warm-up at TUI startup.
type ModelWarmupMsg struct {
	Err error
}
