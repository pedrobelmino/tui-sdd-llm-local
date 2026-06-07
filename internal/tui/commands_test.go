package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
)

func TestLoadProjectCmd_ReturnsProjectLoadedMsg(t *testing.T) {
	orig := loadProjectFn
	defer func() { loadProjectFn = orig }()

	loadProjectFn = func() tea.Msg {
		return ProjectLoadedMsg{
			Ctx: project.ProjectContext{Valid: true, Root: "/tmp/tsll"},
		}
	}

	cmd := loadProjectCmd()
	msg := cmd()
	if _, ok := msg.(ProjectLoadedMsg); !ok {
		t.Fatalf("expected ProjectLoadedMsg, got %T", msg)
	}
}

func TestFetchOllamaCmd_ReturnsOllamaSnapshotMsg(t *testing.T) {
	orig := fetchOllamaFn
	defer func() { fetchOllamaFn = orig }()

	fetchOllamaFn = func() tea.Msg {
		return OllamaSnapshotMsg{Snapshot: ollama.Snapshot{Reachable: false}}
	}

	msg := fetchOllamaCmd()()
	if m, ok := msg.(OllamaSnapshotMsg); !ok || m.Snapshot.Reachable {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}

func TestFetchGPUCmd_ReturnsGPUSnapshotMsg(t *testing.T) {
	orig := fetchGPUFn
	defer func() { fetchGPUFn = orig }()

	fetchGPUFn = func() tea.Msg {
		return GPUSnapshotMsg{Snapshot: gpu.Snapshot{Available: false}}
	}

	msg := fetchGPUCmd()()
	if m, ok := msg.(GPUSnapshotMsg); !ok || m.Snapshot.Available {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}

func TestRefreshCmd_Batch(t *testing.T) {
	cmd := refreshCmd()
	if cmd == nil {
		t.Fatal("refreshCmd returned nil")
	}
}

func TestTickGPUCmd_ReturnsMsg(t *testing.T) {
	orig := fetchGPUFn
	defer func() { fetchGPUFn = orig }()

	fetchGPUFn = func() tea.Msg {
		return GPUSnapshotMsg{Snapshot: gpu.Snapshot{Available: true}}
	}

	cmd := tickGPUCmd()
	if cmd == nil {
		t.Fatal("tickGPUCmd returned nil")
	}
	_ = time.Second
}

func TestFetchSystemCmd_ReturnsSystemSnapshotMsg(t *testing.T) {
	orig := fetchSystemFn
	defer func() { fetchSystemFn = orig }()

	fetchSystemFn = func() tea.Msg {
		return SystemSnapshotMsg{Snapshot: system.Snapshot{Available: true}}
	}

	msg := fetchSystemCmd()()
	if m, ok := msg.(SystemSnapshotMsg); !ok || !m.Snapshot.Available {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}

func TestTickSystemCmd_NotNil(t *testing.T) {
	orig := fetchSystemFn
	defer func() { fetchSystemFn = orig }()

	fetchSystemFn = func() tea.Msg {
		return SystemSnapshotMsg{Snapshot: system.Snapshot{Available: true}}
	}

	if cmd := tickSystemCmd(); cmd == nil {
		t.Fatal("tickSystemCmd returned nil")
	}
}
