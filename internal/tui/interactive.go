package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/workflow"
)

func (m RootModel) handleInteractiveKey(msg tea.KeyMsg) (RootModel, tea.Cmd) {
	switch m.screen {
	case ScreenForm:
		return m.handleFormKey(msg)
	case ScreenAction:
		return m.handleActionKey(msg)
	case ScreenFeatureDetail:
		return m.handleDetailKey(msg)
	case ScreenDashboard:
		if m.activeView == ViewFeatures {
			return m.handleFeaturesKey(msg)
		}
	}
	return m, nil
}

func (m RootModel) handleFeaturesKey(msg tea.KeyMsg) (RootModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Down), msg.String() == "j":
		if len(m.features) > 0 && m.featureCursor < len(m.features)-1 {
			m.featureCursor++
		}
	case key.Matches(msg, m.keymap.Up), msg.String() == "k":
		if m.featureCursor > 0 {
			m.featureCursor--
		}
	case key.Matches(msg, m.keymap.NewFeature):
		m = m.openFormWithReturn(FormNewFeatureName, ScreenDashboard)
	case key.Matches(msg, m.keymap.Open):
		if len(m.features) > 0 {
			return m.openFeatureDetail()
		}
	case key.Matches(msg, m.keymap.Specify):
		if len(m.features) > 0 {
			m.pendingFeature = m.sortedFeatureName()
			m = m.openFormWithReturn(FormFeatureBrief, ScreenDashboard)
		}
	case key.Matches(msg, m.keymap.GenDesign):
		if len(m.features) > 0 {
			name := m.sortedFeatureName()
			return m.startAction(ActionDesign, name, "", "")
		}
	case key.Matches(msg, m.keymap.GenTasks):
		if len(m.features) > 0 {
			name := m.sortedFeatureName()
			return m.startAction(ActionTasks, name, "", "")
		}
	case key.Matches(msg, m.keymap.Implement):
		if len(m.features) > 0 {
			name := m.sortedFeatureName()
			return m.startAction(ActionImplement, name, "", "")
		}
	}
	return m, nil
}

func (m RootModel) handleDetailKey(msg tea.KeyMsg) (RootModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Back):
		m.screen = ScreenDashboard
		m.activeView = ViewFeatures
	case key.Matches(msg, m.keymap.Down), msg.String() == "j":
		if m.taskCursor < len(m.featureTasks)-1 {
			m.taskCursor++
		}
	case key.Matches(msg, m.keymap.Up), msg.String() == "k":
		if m.taskCursor > 0 {
			m.taskCursor--
		}
	case key.Matches(msg, m.keymap.Specify):
		m.pendingFeature = m.selectedFeature
		m = m.openFormWithReturn(FormFeatureBrief, ScreenFeatureDetail)
	case key.Matches(msg, m.keymap.GenDesign):
		return m.startAction(ActionDesign, m.selectedFeature, "", "")
	case key.Matches(msg, m.keymap.GenTasks):
		return m.startAction(ActionTasks, m.selectedFeature, "", "")
	case key.Matches(msg, m.keymap.ImplementAll):
		return m.startAction(ActionImplement, m.selectedFeature, "", "")
	case key.Matches(msg, m.keymap.RunTask), key.Matches(msg, m.keymap.Open):
		if len(m.featureTasks) > 0 {
			id := m.featureTasks[m.taskCursor].ID
			return m.startAction(ActionRun, m.selectedFeature, "", id)
		}
	}
	return m, nil
}

func (m RootModel) handleFormKey(msg tea.KeyMsg) (RootModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Back):
		m.screen = m.formReturnScreen
		m.formKind = FormNone
	case key.Matches(msg, m.keymap.Submit):
		return m.submitForm()
	}

	var cmd tea.Cmd
	if m.formKind == FormFeatureBrief || m.formKind == FormInitVision {
		m.textArea, cmd = m.textArea.Update(msg)
	} else {
		m.textInput, cmd = m.textInput.Update(msg)
	}
	return m, cmd
}

func (m RootModel) handleActionKey(msg tea.KeyMsg) (RootModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keymap.Down), msg.String() == "j":
		m.actionFollowTail = false
		if m.actionScrollLine < m.actionMaxScroll() {
			m.actionScrollLine++
		}
		return m, nil
	case key.Matches(msg, m.keymap.Up), msg.String() == "k":
		if m.actionScrollLine > 0 {
			m.actionScrollLine--
		}
		if m.actionScrollLine >= m.actionMaxScroll() {
			m = m.scrollActionToBottom()
		}
		return m, nil
	case msg.String() == "g":
		m.actionScrollLine = 0
		m.actionFollowTail = false
		return m, nil
	case msg.String() == "G":
		return m.scrollActionToBottom(), nil
	}

	if m.actionRunning {
		return m, nil
	}
	if key.Matches(msg, m.keymap.Back) {
		m.screen = m.actionReturnScreen
		if m.actionReturnScreen == ScreenFeatureDetail {
			return m, m.loadFeatureTasksCmd(m.selectedFeature)
		}
		return m, loadProjectCmd()
	}
	return m, nil
}

func (m RootModel) submitForm() (RootModel, tea.Cmd) {
	switch m.formKind {
	case FormNewFeatureName:
		name := strings.TrimSpace(m.textInput.Value())
		if name == "" {
			m.statusMsg = "feature name required"
			return m, nil
		}
		m.pendingFeature = name
		m = m.openFormWithReturn(FormFeatureBrief, m.formReturnScreen)
		return m, nil

	case FormFeatureBrief:
		brief := strings.TrimSpace(m.textArea.Value())
		if brief == "" {
			m.statusMsg = "description required"
			return m, nil
		}
		feature := m.pendingFeature
		m.screen = ScreenDashboard
		m.formKind = FormNone
		return m.startAction(ActionSpecify, feature, brief, "")

	case FormInitName:
		name := strings.TrimSpace(m.textInput.Value())
		if name == "" {
			m.statusMsg = "project name required"
			return m, nil
		}
		m.pendingInitName = name
		m = m.openFormWithReturn(FormInitVision, m.formReturnScreen)
		return m, nil

	case FormInitVision:
		vision := strings.TrimSpace(m.textArea.Value())
		if vision == "" {
			m.statusMsg = "vision required"
			return m, nil
		}
		cwd, err := os.Getwd()
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		if err := workflow.InitProject(workflow.InitParams{
			Root: cwd, Name: m.pendingInitName, Vision: vision,
		}); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.screen = ScreenDashboard
		m.formKind = FormNone
		m.statusMsg = "Project initialized — press 2 for Features"
		m.activeView = ViewOverview
		return m, loadProjectCmd()
	}
	return m, nil
}

func (m RootModel) openForm(kind FormKind) RootModel {
	return m.openFormWithReturn(kind, m.screen)
}

func (m RootModel) openFormWithReturn(kind FormKind, returnScreen Screen) RootModel {
	m.formReturnScreen = returnScreen
	m.screen = ScreenForm
	m.formKind = kind

	switch kind {
	case FormNewFeatureName:
		m.textInput.Focus()
		m.textInput.SetValue("")
		m.textInput.Placeholder = "feature-name (e.g. user-auth)"
		m.textInput.CharLimit = 64
		m.textInput.Width = 40
	case FormFeatureBrief:
		m.textArea.Focus()
		m.textArea.SetValue("")
		m.textArea.Placeholder = "Describe the feature..."
	case FormInitName:
		m.textInput.Focus()
		m.textInput.SetValue("")
		m.textInput.Placeholder = "project-name"
		m.textInput.CharLimit = 64
		m.textInput.Width = 40
	case FormInitVision:
		m.textArea.Focus()
		m.textArea.SetValue("")
		m.textArea.Placeholder = "Vision (1-2 sentences)..."
	}
	return m
}

func (m RootModel) openFeatureDetail() (RootModel, tea.Cmd) {
	if len(m.features) == 0 {
		return m, nil
	}
	name := m.sortedFeatureName()
	m.screen = ScreenFeatureDetail
	m.selectedFeature = name
	m.taskCursor = 0
	return m, m.loadFeatureTasksCmd(name)
}

func (m RootModel) sortedFeatureName() string {
	sorted := sortedFeatures(m.features)
	if m.featureCursor < 0 || m.featureCursor >= len(sorted) {
		return ""
	}
	return sorted[m.featureCursor].Name
}

func sortedFeatures(in []project.FeatureEntry) []project.FeatureEntry {
	out := append([]project.FeatureEntry(nil), in...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func (m RootModel) loadFeatureTasksCmd(feature string) tea.Cmd {
	return func() tea.Msg {
		if !m.project.Valid {
			return FeatureTasksMsg{Feature: feature}
		}
		path := filepath.Join(m.project.Root, ".specs/features", feature, "tasks.md")
		tasks, err := project.ParseTasks(path)
		if err != nil {
			return FeatureTasksMsg{Feature: feature}
		}
		return FeatureTasksMsg{Feature: feature, Tasks: tasks}
	}
}

func (m RootModel) startAction(kind ActionKind, feature, brief, taskID string) (RootModel, tea.Cmd) {
	if !m.project.Valid {
		m.statusMsg = "run tsll init first"
		return m, nil
	}
	returnScreen := m.screen
	if returnScreen == ScreenForm {
		returnScreen = ScreenDashboard
	}
	m.actionReturnScreen = returnScreen
	m.screen = ScreenAction
	m.actionRunning = true
	m.actionKind = kind
	m.actionTaskID = taskID
	m.pendingFeature = feature
	m.selectedFeature = feature
	m.actionLog = ""
	m.actionScrollLine = 0
	m.actionFollowTail = true

	ch := make(chan tea.Msg, 64)
	m.actionCh = ch

	go runWorkflow(kind, m.project.Root, feature, brief, taskID, ch)

	return m, tea.Batch(waitActionMsg(ch), actionTickCmd())
}

func runWorkflow(kind ActionKind, root, feature, brief, taskID string, ch chan<- tea.Msg) {
	defer close(ch)
	svc := workflow.New()
	ctx := context.Background()

	var usage ollama.TokenUsage
	var err error
	onChunk := func(s string) { ch <- ActionChunkMsg{Text: s} }

	switch kind {
	case ActionSpecify:
		usage, err = svc.Specify(ctx, root, feature, brief, onChunk)
	case ActionDesign:
		usage, err = svc.Design(ctx, root, feature, onChunk)
	case ActionTasks:
		usage, err = svc.Tasks(ctx, root, feature, onChunk)
	case ActionImplement:
		usage, err = svc.Implement(ctx, root, feature, onChunk)
	case ActionRun:
		usage, err = svc.Run(ctx, root, feature, taskID, onChunk)
	}

	ch <- ActionFinishedMsg{
		Kind:    kind,
		Feature: feature,
		TaskID:  taskID,
		Usage:   usage,
		Err:     err,
	}
}

func waitActionMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func actionTickCmd() tea.Cmd {
	return tea.Tick(actionScrollInterval, func(time.Time) tea.Msg { return ActionTickMsg{} })
}

func (m RootModel) renderForm(width, height int) string {
	title := "New feature"
	body := m.textInput.View()
	switch m.formKind {
	case FormFeatureBrief:
		title = "Describe: " + m.pendingFeature
		body = m.textArea.View()
	case FormInitName:
		title = "Init project — name"
	case FormInitVision:
		title = "Init project — vision: " + m.pendingInitName
		body = m.textArea.View()
	}
	help := "enter: submit  esc: cancel"
	return ui.Panel(title, body+"\n\n"+help, width-4, height)
}

func (m RootModel) renderAction(width, height int) string {
	title := "Working..."
	switch m.actionKind {
	case ActionSpecify:
		title = "Generating spec: " + m.pendingFeature
	case ActionDesign:
		title = "Generating design: " + m.selectedFeature
	case ActionTasks:
		title = "Generating tasks: " + m.selectedFeature
	case ActionImplement:
		title = "Implementing: " + m.selectedFeature
	case ActionRun:
		title = fmt.Sprintf("Running %s on %s", m.actionTaskID, m.selectedFeature)
	}
	if !m.actionRunning {
		title = "Done — esc to close"
	}
	log := m.actionLog
	if log == "" && m.actionRunning {
		log = "Waiting for model..."
	}
	return ui.PanelViewport(title, log, width-4, height, m.actionScrollForRender())
}

func (m RootModel) renderStatusBanner(width int) string {
	if m.statusMsg == "" {
		return ""
	}
	return ui.BannerError(m.statusMsg, width)
}

func newTextInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	return ti
}

func newTextArea() textarea.Model {
	ta := textarea.New()
	ta.SetHeight(6)
	ta.SetWidth(60)
	ta.ShowLineNumbers = false
	return ta
}
