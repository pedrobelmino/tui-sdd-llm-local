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
			return m.startAction(ActionDesign, name, "", "", "")
		}
	case key.Matches(msg, m.keymap.GenTasks):
		if len(m.features) > 0 {
			name := m.sortedFeatureName()
			return m.startAction(ActionTasks, name, "", "", "")
		}
	case key.Matches(msg, m.keymap.QuickTask):
		if len(m.features) > 0 {
			m.pendingFeature = m.sortedFeatureName()
		}
		m = m.openFormWithReturn(FormQuickTask, ScreenDashboard)
	case key.Matches(msg, m.keymap.Ask):
		if len(m.features) > 0 {
			m.pendingFeature = m.sortedFeatureName()
			m = m.openFormWithReturn(FormAsk, ScreenDashboard)
		}
	case key.Matches(msg, m.keymap.Implement):
		if len(m.features) > 0 {
			name := m.sortedFeatureName()
			return m.startAction(ActionImplement, name, "", "", "")
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
		return m.startAction(ActionDesign, m.selectedFeature, "", "", "")
	case key.Matches(msg, m.keymap.GenTasks):
		return m.startAction(ActionTasks, m.selectedFeature, "", "", "")
	case key.Matches(msg, m.keymap.QuickTask):
		m.pendingFeature = m.selectedFeature
		m = m.openFormWithReturn(FormQuickTask, ScreenFeatureDetail)
		return m, nil
	case key.Matches(msg, m.keymap.Ask):
		m.pendingFeature = m.selectedFeature
		m = m.openFormWithReturn(FormAsk, ScreenFeatureDetail)
		return m, nil
	case key.Matches(msg, m.keymap.ImplementAll):
		return m.startAction(ActionImplement, m.selectedFeature, "", "", "")
	case key.Matches(msg, m.keymap.RunTask), key.Matches(msg, m.keymap.Open):
		if len(m.featureTasks) > 0 {
			id := m.featureTasks[m.taskCursor].ID
			return m.startAction(ActionRun, m.selectedFeature, "", id, "")
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
	case key.Matches(msg, m.keymap.CopyLog):
		if msg, err := copyActionLog(m.actionLog); err != nil {
			m.statusMsg = err.Error()
		} else {
			m.statusMsg = msg
		}
		return m, nil
	}

	// Cancel running action with ESC or x.
	if m.actionRunning {
		if key.Matches(msg, m.keymap.Back) || key.Matches(msg, m.keymap.CancelAction) {
			if m.actionCancel != nil {
				m.actionCancel()
				m.actionCancel = nil
			}
			m.actionCancelled = true
			m.actionRunning = false
			m.actionPhase = "cancelled"
			m.actionLog += "\n\n✗ cancelled by user"
			m = m.scrollActionToBottom()
		}
		return m, nil
	}

	if m.actionNeedsInput && key.Matches(msg, m.keymap.Submit) {
		m = m.openFormWithReturn(FormActionReply, ScreenAction)
		return m, nil
	}

	if key.Matches(msg, m.keymap.Back) {
		if m.actionCancel != nil {
			m.actionCancel()
			m.actionCancel = nil
		}
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
		return m.startAction(ActionSpecify, feature, brief, "", "")

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

	case FormQuickTask:
		req := strings.TrimSpace(m.textArea.Value())
		if req == "" {
			m.statusMsg = "quick task request required"
			return m, nil
		}
		feature := strings.TrimSpace(m.pendingFeature)
		m.screen = m.formReturnScreen
		m.formKind = FormNone
		return m.startAction(ActionQuickTask, feature, "", "", req)

	case FormAsk:
		question := strings.TrimSpace(m.textArea.Value())
		if question == "" {
			m.statusMsg = "question required"
			return m, nil
		}
		feature := strings.TrimSpace(m.pendingFeature)
		if feature == "" {
			m.statusMsg = "select a feature first"
			return m, nil
		}
		m.screen = m.formReturnScreen
		m.formKind = FormNone
		return m.startAction(ActionAsk, feature, "", "", question)

	case FormActionReply:
		answer := strings.TrimSpace(m.textArea.Value())
		if answer == "" {
			m.statusMsg = "reply required"
			return m, nil
		}
		m.screen = ScreenAction
		m.formKind = FormNone
		prompt := "Model asked:\n" + m.actionQuestion + "\n\nUser answer:\n" + answer + "\n\nContinue with the implementation/workflow now."
		return m.startAction(ActionQuickTask, m.selectedFeature, "", "", prompt)
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
	case FormQuickTask:
		m.textArea.Focus()
		m.textArea.SetValue("")
		if m.pendingFeature != "" {
			m.textArea.Placeholder = "Describe quick task for feature " + m.pendingFeature + "..."
		} else {
			m.textArea.Placeholder = "Describe a quick task..."
		}
	case FormAsk:
		m.textArea.Focus()
		m.textArea.SetValue("")
		m.textArea.Placeholder = "Ask about " + m.pendingFeature + " (spec, design, tasks, tech stack)..."
	case FormActionReply:
		m.textArea.Focus()
		m.textArea.SetValue("")
		m.textArea.Placeholder = "Answer model question..."
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

func (m RootModel) startAction(kind ActionKind, feature, brief, taskID, prompt string) (RootModel, tea.Cmd) {
	if !m.project.Valid {
		m.statusMsg = "run tsll init first"
		return m, nil
	}
	// Cancel any previous action still running.
	if m.actionCancel != nil {
		m.actionCancel()
	}
	returnScreen := m.screen
	if returnScreen == ScreenForm {
		returnScreen = ScreenDashboard
	}
	m.actionReturnScreen = returnScreen
	m.screen = ScreenAction
	m.actionRunning = true
	m.actionCancelled = false
	m.actionKind = kind
	m.actionTaskID = taskID
	m.actionPrompt = prompt
	m.pendingFeature = feature
	m.selectedFeature = feature
	m.actionLog = ""
	m.actionNeedsInput = false
	m.actionQuestion = ""
	m.actionPhase = "waiting"
	m.actionScrollLine = 0
	m.actionFollowTail = true
	m.actionSpinner = 0

	ctx, cancel := context.WithCancel(context.Background())
	m.actionCancel = cancel

	ch := make(chan tea.Msg, 64)
	m.actionCh = ch

	go runWorkflow(ctx, kind, m.project.Root, feature, brief, taskID, prompt, ch)

	return m, tea.Batch(waitActionMsg(ch), actionTickCmd())
}

func runWorkflow(ctx context.Context, kind ActionKind, root, feature, brief, taskID, prompt string, ch chan<- tea.Msg) {
	defer close(ch)
	svc := workflow.New()

	var usage ollama.TokenUsage
	var output string
	var err error
	onChunk := func(s string) {
		select {
		case ch <- ActionChunkMsg{Text: s}:
		case <-ctx.Done():
		}
	}

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
	case ActionQuickTask:
		output, usage, err = svc.QuickTask(ctx, root, feature, prompt, onChunk)
	case ActionAsk:
		output, usage, err = svc.Ask(ctx, root, feature, prompt, onChunk)
	}

	ch <- ActionFinishedMsg{
		Kind:    kind,
		Feature: feature,
		TaskID:  taskID,
		Output:  output,
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
	case FormQuickTask:
		if m.pendingFeature != "" {
			title = "Quick task: " + m.pendingFeature
		} else {
			title = "Quick task"
		}
		body = m.textArea.View()
	case FormAsk:
		title = "Ask: " + m.pendingFeature
		body = m.textArea.View()
	case FormActionReply:
		title = "Reply to model"
		body = m.textArea.View()
	}
	help := "enter: submit  esc: cancel"
	return ui.Panel(title, body+"\n\n"+help, width-4, height)
}

func (m RootModel) renderAction(width, height int) string {
	feature := m.selectedFeature
	if feature == "" {
		feature = m.pendingFeature
	}

	var baseTitle string
	switch m.actionKind {
	case ActionSpecify:
		baseTitle = "spec: " + feature
	case ActionDesign:
		baseTitle = "design: " + feature
	case ActionTasks:
		baseTitle = "tasks: " + feature
	case ActionImplement:
		baseTitle = "implement: " + feature
	case ActionRun:
		baseTitle = fmt.Sprintf("run %s · %s", m.actionTaskID, feature)
	case ActionAsk:
		baseTitle = "ask: " + feature
	default:
		baseTitle = feature
	}

	var title string
	switch {
	case m.actionCancelled:
		title = "✗ cancelled — " + baseTitle
	case !m.actionRunning && m.actionPhase == "done":
		title = "✓ done — esc to close"
	case !m.actionRunning && m.actionPhase == "error":
		title = "✗ error — esc to close"
	case !m.actionRunning:
		title = "Done — esc to close"
	default:
		spin := spinnerFrames[m.actionSpinner%len(spinnerFrames)]
		phaseLabel := phaseLabel(m.actionPhase)
		title = spin + " " + baseTitle + " · " + phaseLabel
	}

	log := m.actionLog
	if log == "" && m.actionRunning {
		log = "Waiting for model..."
	}
	return ui.PanelViewport(title, log, width-4, height, m.actionScrollForRender())
}

// detectPhase infers the current workflow phase from a streaming chunk.
func detectPhase(chunk, current string) string {
	t := strings.TrimSpace(chunk)
	switch {
	case strings.HasPrefix(t, "🔧"):
		return "tool-call"
	case strings.HasPrefix(t, "   ✓"):
		return "tool-done"
	case strings.HasPrefix(t, "✓"):
		return "tool-done"
	case strings.HasPrefix(t, "📂"):
		return "loading"
	case strings.HasPrefix(t, "⚠"):
		return "generating"
	case strings.HasPrefix(t, "❌"):
		return "error"
	case strings.HasPrefix(t, "---"):
		return "task-start"
	case t != "":
		return "generating"
	}
	return current
}

// phaseLabel returns a short human-readable label for an action phase.
func phaseLabel(phase string) string {
	switch phase {
	case "waiting":
		return "waiting…"
	case "loading":
		return "loading layout…"
	case "generating":
		return "generating"
	case "tool-call":
		return "calling tool"
	case "tool-done":
		return "tool done"
	case "task-start":
		return "next task"
	case "done":
		return "done"
	case "error":
		return "error"
	case "cancelled":
		return "cancelled"
	case "awaiting-input":
		return "awaiting input"
	default:
		return phase
	}
}

// detectQuestionPrompt tries to detect when the model finished by asking for user input.
func detectQuestionPrompt(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return "", false
	}
	l := strings.ToLower(t)
	markers := []string{
		"what should", "what would you like", "please provide",
		"which file", "which path", "can you clarify", "could you clarify",
		"how would you like", "what should be the name and location",
		"to proceed", "i need",
	}
	for _, m := range markers {
		if strings.Contains(l, m) && strings.Contains(t, "?") {
			return t, true
		}
	}
	return "", false
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
