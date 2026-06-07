package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/system"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tokens"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tui/views"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const actionScrollInterval = 200 * time.Millisecond

// ViewID identifies the active dashboard tab.
type ViewID int

const (
	ViewOverview ViewID = iota + 1
	ViewFeatures
	ViewModels
	ViewMetrics
	ViewSystem

	minWidth  = 80
	minHeight = 24
)

var headerTabs = []ui.Tab{
	{Key: "1", Label: "Overview"},
	{Key: "2", Label: "Features"},
	{Key: "3", Label: "Models"},
	{Key: "4", Label: "Metrics"},
	{Key: "5", Label: "System"},
}

// RootModel is the top-level Bubble Tea model for the tsll dashboard.
type RootModel struct {
	width  int
	height int

	screen     Screen
	activeView ViewID
	showHelp  bool
	statusMsg string

	project  project.ProjectContext
	features []project.FeatureEntry

	featureCursor    int
	selectedFeature  string
	featureTasks     []project.TaskEntry
	taskCursor       int
	pendingFeature string
	pendingInitName string

	formKind         FormKind
	formReturnScreen Screen
	textInput        textinput.Model
	textArea         textarea.Model

	actionRunning      bool
	actionKind         ActionKind
	actionTaskID       string
	actionLog          string
	actionScrollLine   int
	actionFollowTail   bool
	actionCh           <-chan tea.Msg
	actionReturnScreen Screen

	ollama  ollama.Snapshot
	gpu     gpu.Snapshot
	system  system.Snapshot
	tokens  tokens.SessionCounter
	loading map[string]bool

	errBanner string
	keymap    KeyMap
	version   string
}

// NewRootModel constructs the initial TUI state.
func NewRootModel(version string) RootModel {
	ti := newTextInput()
	ta := newTextArea()
	return RootModel{
		screen:     ScreenDashboard,
		activeView: ViewOverview,
		keymap:     DefaultKeyMap(),
		textInput:  ti,
		textArea:   ta,
		loading: map[string]bool{
			"project": true,
			"ollama":  true,
			"gpu":     true,
			"system":  true,
		},
		version: version,
	}
}

// Init loads project, Ollama, GPU, system and starts ticks.
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		loadProjectCmd(),
		fetchOllamaCmd(),
		fetchGPUCmd(),
		fetchSystemCmd(),
		tickGPUCmd(),
		tickSystemCmd(),
		tickOllamaCmd(),
	)
}

// Update routes messages to state changes.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.showHelp && key.Matches(msg, m.keymap.Help) {
			m.showHelp = false
			return m, nil
		}
		if m.showHelp {
			return m, nil
		}

		// Interactive screens take priority
		if m.screen != ScreenDashboard || m.activeView == ViewFeatures {
			if m.screen != ScreenDashboard {
				updated, cmd := m.handleInteractiveKey(msg)
				return updated, cmd
			}
			if m.activeView == ViewFeatures {
				if key.Matches(msg, m.keymap.NewFeature) ||
					key.Matches(msg, m.keymap.Open) ||
					key.Matches(msg, m.keymap.Specify) ||
					key.Matches(msg, m.keymap.GenDesign) ||
					key.Matches(msg, m.keymap.GenTasks) ||
					key.Matches(msg, m.keymap.Implement) ||
					key.Matches(msg, m.keymap.Up) ||
					key.Matches(msg, m.keymap.Down) ||
					msg.String() == "j" || msg.String() == "k" {
					updated, cmd := m.handleInteractiveKey(msg)
					return updated, cmd
				}
			}
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keymap.ForceQuit):
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
		case key.Matches(msg, m.keymap.Overview):
			m.activeView = ViewOverview
		case key.Matches(msg, m.keymap.Features):
			m.activeView = ViewFeatures
		case key.Matches(msg, m.keymap.Models):
			m.activeView = ViewModels
		case key.Matches(msg, m.keymap.Metrics):
			m.activeView = ViewMetrics
		case key.Matches(msg, m.keymap.System):
			m.activeView = ViewSystem
		case key.Matches(msg, m.keymap.Refresh):
			m.loading["project"] = true
			m.loading["ollama"] = true
			m.loading["system"] = true
			return m, refreshCmd()
		case key.Matches(msg, m.keymap.Init):
			if m.activeView == ViewOverview {
				if m.project.Valid {
					m.statusMsg = "Project ready — press 2 for Features"
				} else {
					m = m.openFormWithReturn(FormInitName, ScreenDashboard)
				}
			}
		}

	case ProjectLoadedMsg:
		m.project = msg.Ctx
		m.features = msg.Features
		m.loading["project"] = false
		if m.featureCursor >= len(m.features) {
			m.featureCursor = 0
		}

	case OllamaSnapshotMsg:
		m.ollama = msg.Snapshot
		m.loading["ollama"] = false
		if !msg.Snapshot.Reachable {
			m.errBanner = "Ollama unreachable"
		} else {
			m.errBanner = ""
		}
		return m, tickOllamaCmd()

	case GPUSnapshotMsg:
		m.gpu = msg.Snapshot
		m.loading["gpu"] = false
		return m, tickGPUCmd()

	case SystemSnapshotMsg:
		m.system = msg.Snapshot
		m.loading["system"] = false
		return m, tickSystemCmd()

	case TokenUsageMsg:
		m.tokens.Add(msg.Usage.PromptTokens, msg.Usage.CompletionTokens)

	case FeatureTasksMsg:
		m.featureTasks = msg.Tasks
		m.selectedFeature = msg.Feature

	case ActionChunkMsg:
		m.actionLog += msg.Text
		if m.actionFollowTail {
			m = m.scrollActionToBottom()
		} else {
			m = m.clampActionScroll()
		}
		if m.actionCh != nil {
			return m, waitActionMsg(m.actionCh)
		}

	case ActionFinishedMsg:
		m.actionRunning = false
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
			m.actionLog += "\n\n✗ " + msg.Err.Error()
		} else {
			m.statusMsg = ""
			m.tokens.Add(msg.Usage.PromptTokens, msg.Usage.CompletionTokens)
			m.actionLog += fmt.Sprintf("\n\n✓ done (%d+%d tokens)",
				msg.Usage.PromptTokens, msg.Usage.CompletionTokens)
		}
		m = m.scrollActionToBottom()
		m.selectedFeature = msg.Feature
		cmds := []tea.Cmd{loadProjectCmd()}
		if m.actionReturnScreen == ScreenFeatureDetail {
			cmds = append(cmds, m.loadFeatureTasksCmd(msg.Feature))
		}
		return m, tea.Batch(cmds...)

	case ActionTickMsg:
		if m.actionRunning {
			return m, actionTickCmd()
		}
	}

	return m, nil
}

// View renders the full dashboard.
func (m RootModel) View() string {
	if m.showHelp {
		return m.renderHelp()
	}

	w := m.width
	if w < 1 {
		w = 80
	}
	h := m.height
	if h < 1 {
		h = 24
	}

	mainH := m.mainPanelHeight(w, h)

	var main, footer string
	switch m.screen {
	case ScreenForm:
		main = m.renderForm(w, mainH)
		footer = "enter: submit  esc: cancel"
	case ScreenAction:
		main = m.renderAction(w, mainH)
		footer = "j/k: scroll │ g/G: top/bottom │ esc: back"
	case ScreenFeatureDetail:
		main = m.renderFeatureDetailBody(w, mainH)
		footer = "esc: back │ e: run task │ a: impl all │ s/d/t: spec/design/tasks"
	default:
		main = m.renderBody(w, h)
		footer = FooterBindings()
		if m.activeView == ViewFeatures {
			footer = FeaturesFooter()
		}
	}

	return m.composePage(main, footer, w, h)
}

func (m RootModel) renderFeatureDetailBody(width, height int) string {
	var entry project.FeatureEntry
	for _, f := range m.features {
		if f.Name == m.selectedFeature {
			entry = f
			break
		}
	}
	done, total := project.TaskProgress(m.featureTasks)
	progress := fmt.Sprintf("Progress: %d/%d tasks done", done, total)

	return views.RenderFeatureDetail(views.FeatureDetailData{
		Width: width, Height: height,
		Feature: entry, Tasks: m.featureTasks,
		TaskCursor: m.taskCursor, Progress: progress,
	})
}

func (m RootModel) headerTitle() string {
	proj := "no project"
	if m.project.Valid {
		proj = projectBasename(m.project.Root)
	}
	model := ollama.DefaultModel
	if len(m.ollama.Running) > 0 {
		model = m.ollama.Running[0].Name
	}
	title := fmt.Sprintf("tsll %s ── %s ── %s", m.version, proj, model)
	if mini := systemMiniLine(m.system); mini != "" {
		title += " ── " + mini
	}
	if mini := gpuMiniLine(m.gpu); mini != "" {
		title += " ── " + mini
	}
	return title
}

func systemMiniLine(s system.Snapshot) string {
	if !s.Available {
		return ""
	}
	memPct := 0.0
	if s.Memory.TotalMiB > 0 {
		memPct = float64(s.Memory.UsedMiB) / float64(s.Memory.TotalMiB) * 100
	}
	if !s.CPU.HasBaseline {
		return fmt.Sprintf("CPU --%% │ MEM %.0f%%", memPct)
	}
	return fmt.Sprintf("CPU %.0f%% │ MEM %.0f%%", s.CPU.Utilization, memPct)
}

func gpuMiniLine(s gpu.Snapshot) string {
	if !s.Available || len(s.Devices) == 0 {
		return ""
	}
	d := s.Devices[0]
	tag := strings.ToUpper(string(d.Vendor))
	if tag == "" {
		tag = "GPU"
	}
	return fmt.Sprintf("%s %0.f%% │ VRAM %d/%d MiB", tag, d.Utilization, d.MemoryUsed, d.MemoryTotal)
}

func (m RootModel) renderBody(width, height int) string {
	switch m.activeView {
	case ViewOverview:
		return views.RenderOverview(views.OverviewData{
			Width: width, Height: height,
			Root: m.project.Root, Valid: m.project.Valid, Corrupted: m.project.Corrupted,
			CurrentWork: m.project.CurrentWork, Milestone: m.project.Milestone,
		})
	case ViewFeatures:
		return views.RenderFeatures(views.FeaturesData{
			Width: width, Height: height, Features: m.features, Cursor: m.featureCursor,
		})
	case ViewModels:
		return views.RenderModels(views.ModelsData{
			Width: width, Height: height,
			Tags: m.ollama.Tags, Running: m.ollama.Running,
			Reachable: m.ollama.Reachable, DefaultModelMissing: m.ollama.DefaultModelMissing,
			Error: m.ollama.Error,
		})
	case ViewMetrics:
		return views.RenderMetrics(views.MetricsData{
			Width: width, Height: height, Tokens: m.tokens, GPU: m.gpu,
		})
	case ViewSystem:
		return views.RenderSystem(views.SystemData{
			Width: width, Height: height, System: m.system,
		})
	default:
		return ""
	}
}

func (m RootModel) renderHelp() string {
	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	return ui.Panel("Help", HelpOverlay(), w-4, h-4)
}

func projectBasename(root string) string {
	if root == "" {
		return "no project"
	}
	parts := strings.Split(strings.TrimRight(root, "/"), "/")
	return parts[len(parts)-1]
}
