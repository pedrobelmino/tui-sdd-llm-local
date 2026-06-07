package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
)

func newTestModel() RootModel {
	m := NewRootModel("0.1.0-test")
	m.width = 100
	m.height = 30
	return m
}

func TestModel_ViewOverview(t *testing.T) {
	m := newTestModel()
	m.project = project.ProjectContext{Valid: true, Root: "/tmp/tui-sdd-llm-local", CurrentWork: "test", Milestone: "M1"}
	out := m.View()
	if !strings.Contains(out, "Overview") {
		t.Fatalf("expected Overview tab in view: %s", out)
	}
}

func TestModel_SwitchView(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m2 := updated.(RootModel)
	if m2.activeView != ViewModels {
		t.Fatalf("expected ViewModels, got %v", m2.activeView)
	}
}

func TestModel_FeaturesImplementKeyStartsAction(t *testing.T) {
	m := newTestModel()
	m.activeView = ViewFeatures
	m.project = project.ProjectContext{Valid: true, Root: t.TempDir()}
	m.features = []project.FeatureEntry{{Name: "landing-page", HasSpec: true}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m2 := updated.(RootModel)
	if m2.screen != ScreenAction || m2.actionKind != ActionImplement {
		t.Fatalf("expected implement action, got screen=%v kind=%v", m2.screen, m2.actionKind)
	}
}

func TestModel_FeaturesDesignKeyStartsAction(t *testing.T) {
	m := newTestModel()
	m.activeView = ViewFeatures
	m.project = project.ProjectContext{Valid: true, Root: t.TempDir()}
	m.features = []project.FeatureEntry{{Name: "landing-page", HasSpec: true}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m2 := updated.(RootModel)
	if m2.screen != ScreenAction || m2.actionKind != ActionDesign {
		t.Fatalf("expected design action, got screen=%v kind=%v", m2.screen, m2.actionKind)
	}
	if m2.selectedFeature != "landing-page" {
		t.Fatalf("expected landing-page, got %q", m2.selectedFeature)
	}
}

func TestModel_ToggleHelp(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(RootModel)
	if !m2.showHelp {
		t.Fatal("expected showHelp true")
	}
	out := m2.View()
	if !strings.Contains(out, "keyboard shortcuts") {
		t.Fatalf("help overlay missing: %s", out)
	}
}

func TestModel_QuitKey(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestModel_OllamaUnreachableBanner(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(OllamaSnapshotMsg{Snapshot: ollama.Snapshot{Reachable: false}})
	m2 := updated.(RootModel)
	if m2.errBanner == "" {
		t.Fatal("expected err banner")
	}
}

func TestModel_TokenUsage(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(TokenUsageMsg{Usage: tokens.FromOllamaResponse(10, 5)})
	m2 := updated.(RootModel)
	if m2.tokens.PromptTokens != 10 || m2.tokens.CompletionTokens != 5 {
		t.Fatalf("tokens not accumulated: %+v", m2.tokens)
	}
}

func TestModel_InitOpensForm(t *testing.T) {
	m := newTestModel()
	m.activeView = ViewOverview
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m2 := updated.(RootModel)
	if m2.screen != ScreenForm || m2.formKind != FormInitName {
		t.Fatalf("expected init form, got screen=%v form=%v", m2.screen, m2.formKind)
	}
}

func TestModel_InitWhenProjectValid(t *testing.T) {
	m := newTestModel()
	m.activeView = ViewOverview
	m.project = project.ProjectContext{Valid: true, Root: "/tmp/tsll"}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m2 := updated.(RootModel)
	if m2.statusMsg == "" {
		t.Fatal("expected status when project already valid")
	}
}

func TestModel_GPUMiniInHeader(t *testing.T) {
	m := newTestModel()
	m.gpu = gpu.Snapshot{
		Available: true,
		Devices: []gpu.Device{{
			Utilization: 42, MemoryUsed: 1000, MemoryTotal: 2000,
		}},
	}
	title := m.headerTitle()
	if !strings.Contains(title, "GPU") {
		t.Fatalf("expected GPU in header: %s", title)
	}
}

func TestModel_WindowSize(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := updated.(RootModel)
	if m2.width != 120 || m2.height != 40 {
		t.Fatalf("size not updated: %dx%d", m2.width, m2.height)
	}
}

func TestModel_RefreshKey(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh cmd")
	}
}

func TestModel_SystemKeySwitchesView(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m2 := updated.(RootModel)
	if m2.activeView != ViewSystem {
		t.Fatalf("expected ViewSystem, got %v", m2.activeView)
	}
}

func TestModel_SystemSnapshotMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(SystemSnapshotMsg{Snapshot: system.Snapshot{
		Available: true,
		CPU:       system.CPUInfo{Cores: 8, Utilization: 33.3, HasBaseline: true},
		Memory:    system.MemoryInfo{TotalMiB: 16000, UsedMiB: 4000},
	}})
	m2 := updated.(RootModel)
	if !m2.system.Available || m2.system.CPU.Cores != 8 {
		t.Fatalf("system not stored: %+v", m2.system)
	}
}

func TestModel_ActionScreenShowsFooterMonitor(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenAction
	m.actionRunning = true
	m.actionKind = ActionSpecify
	m.pendingFeature = "landing-page"
	m.ollama = ollama.Snapshot{
		Reachable: true,
		Running: []ollama.RunningModel{{Name: "qwen2.5-coder:latest", SizeVRAM: 2408937472}},
	}
	m.gpu = gpu.Snapshot{
		Available: true,
		Devices: []gpu.Device{{Vendor: gpu.VendorAMD, Utilization: 40, MemoryUsed: 3000, MemoryTotal: 4096}},
	}
	m.system = system.Snapshot{
		Available: true,
		CPU:       system.CPUInfo{Utilization: 22, HasBaseline: true},
		Memory:    system.MemoryInfo{TotalMiB: 16000, UsedMiB: 8000},
		Load:      system.LoadAvg{One: 1.2},
	}

	out := m.View()
	for _, want := range []string{
		"Monitor", "qwen2.5-coder", "GPU AMD 40%", "CPU 22%", "MEM 50%", "Ld 1.2",
		"Generating spec: landing-page",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("action view missing %q:\n%s", want, out)
		}
	}
	genIdx := strings.Index(out, "Generating spec")
	monIdx := strings.LastIndex(out, "Monitor")
	if genIdx < 0 || monIdx < 0 || genIdx > monIdx {
		t.Fatalf("footer monitor should appear after main content (gen=%d mon=%d)", genIdx, monIdx)
	}
}

func TestModel_FeaturesViewShowsFooterMonitor(t *testing.T) {
	m := newTestModel()
	m.activeView = ViewFeatures
	m.ollama = ollama.Snapshot{Reachable: true, Running: []ollama.RunningModel{{Name: "phi3:mini"}}}
	m.gpu = gpu.Snapshot{
		Available: true,
		Devices:   []gpu.Device{{Vendor: gpu.VendorAMD, Utilization: 10, MemoryUsed: 512, MemoryTotal: 4096}},
	}
	m.tokens = tokens.SessionCounter{PromptTokens: 500, CompletionTokens: 200, RequestCount: 2}
	m.system = system.Snapshot{
		Available: true,
		CPU:       system.CPUInfo{Utilization: 15, HasBaseline: true},
		Memory:    system.MemoryInfo{TotalMiB: 8000, UsedMiB: 3200},
		Load:      system.LoadAvg{One: 0.5},
	}

	out := m.View()
	for _, want := range []string{"Monitor", "phi3", "GPU AMD", "Tok 500+200", "CPU 15%", "MEM 40%"} {
		if !strings.Contains(out, want) {
			t.Fatalf("features view missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "Monitor") != 1 {
		t.Fatalf("expected single footer monitor panel, got:\n%s", out)
	}
}

func TestModel_OverviewShowsTopMonitorStrip(t *testing.T) {
	m := newTestModel()
	m.activeView = ViewOverview
	m.ollama = ollama.Snapshot{Reachable: true, Running: []ollama.RunningModel{{Name: "llama3:8b"}}}
	m.gpu = gpu.Snapshot{
		Available: true,
		Devices:   []gpu.Device{{Vendor: gpu.VendorAMD, Utilization: 5, MemoryUsed: 100, MemoryTotal: 8000}},
	}

	out := m.View()
	if !strings.Contains(out, "Monitor") {
		t.Fatal("overview should show top monitor strip")
	}
	// Top strip has separate Token line (not compact Tok prefix).
	if !strings.Contains(out, "Tokens:") {
		t.Fatalf("top monitor should use multi-line format:\n%s", out)
	}
}

func TestModel_FormScreenShowsFooterMonitor(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenForm
	m.formKind = FormFeatureBrief
	m.pendingFeature = "auth"
	m.ollama = ollama.Snapshot{Reachable: true}

	out := m.View()
	if !strings.Contains(out, "Monitor") {
		t.Fatalf("form view missing footer monitor: %s", out)
	}
	if !strings.Contains(out, "Describe: auth") {
		t.Fatalf("form view missing form title: %s", out)
	}
}

func TestModel_ActionScrollWhileRunning(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 20
	m.screen = ScreenAction
	m.actionRunning = true
	m.actionLog = strings.Repeat("line ", 500)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(RootModel)
	if m.actionScrollLine == 0 {
		t.Fatal("expected scroll line to advance on j")
	}
	if m.actionFollowTail {
		t.Fatal("expected follow tail disabled after manual scroll")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = updated.(RootModel)
	if !m.actionFollowTail {
		t.Fatal("expected follow tail enabled after G")
	}
	if m.actionScrollLine != m.actionMaxScroll() {
		t.Fatalf("scroll = %d, want max %d", m.actionScrollLine, m.actionMaxScroll())
	}
}

func TestModel_ActionLogAppendsAcrossUpdates(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenAction
	m.actionRunning = true

	updated, _ := m.Update(ActionChunkMsg{Text: "hello "})
	m = updated.(RootModel)
	updated, _ = m.Update(ActionChunkMsg{Text: "world"})
	m = updated.(RootModel)

	if m.actionLog != "hello world" {
		t.Fatalf("actionLog = %q, want %q", m.actionLog, "hello world")
	}
}

func TestModel_HeaderShowsSystemAndGPU(t *testing.T) {
	m := newTestModel()
	m.system = system.Snapshot{
		Available: true,
		CPU:       system.CPUInfo{Utilization: 33, HasBaseline: true},
		Memory:    system.MemoryInfo{TotalMiB: 1000, UsedMiB: 500},
	}
	m.gpu = gpu.Snapshot{
		Available: true,
		Devices: []gpu.Device{{
			Vendor: gpu.VendorAMD, Utilization: 22, MemoryUsed: 500, MemoryTotal: 4000,
		}},
	}
	title := m.headerTitle()
	if !strings.Contains(title, "CPU 33%") {
		t.Errorf("expected CPU 33%% in header: %s", title)
	}
	if !strings.Contains(title, "AMD") {
		t.Errorf("expected AMD vendor in header: %s", title)
	}
}
