package tui

import "github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"

func (m RootModel) actionPanelWidth() int {
	w := m.width
	if w < 8 {
		return 4
	}
	return w - 4
}

func (m RootModel) actionPanelHeight() int {
	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	return m.mainPanelHeight(w, h)
}

func (m RootModel) actionContentWidth() int {
	w := m.actionPanelWidth() - 3
	if w < 1 {
		return 1
	}
	return w
}

func (m RootModel) actionBodyLines() int {
	h := m.actionPanelHeight() - 2
	if h < 1 {
		return 1
	}
	return h
}

func (m RootModel) actionWrappedLines() []string {
	return ui.WrapLines(m.actionLog, m.actionContentWidth())
}

func (m RootModel) actionMaxScroll() int {
	return ui.MaxScroll(len(m.actionWrappedLines()), m.actionBodyLines())
}

func (m RootModel) clampActionScroll() RootModel {
	max := m.actionMaxScroll()
	if m.actionScrollLine > max {
		m.actionScrollLine = max
	}
	if m.actionScrollLine < 0 {
		m.actionScrollLine = 0
	}
	return m
}

func (m RootModel) scrollActionToBottom() RootModel {
	m.actionScrollLine = m.actionMaxScroll()
	m.actionFollowTail = true
	return m
}

func (m RootModel) actionScrollForRender() int {
	max := m.actionMaxScroll()
	if m.actionScrollLine > max {
		return max
	}
	if m.actionScrollLine < 0 {
		return 0
	}
	return m.actionScrollLine
}
