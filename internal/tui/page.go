package tui

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tui/views"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const (
	monitorPanelLines = 5
	pageHeaderLines   = 2
	pageFooterLines   = 2
)

func (m RootModel) activeTabIndex() int {
	if m.screen == ScreenFeatureDetail {
		return int(ViewFeatures) - 1
	}
	return int(m.activeView) - 1
}

func (m RootModel) pageOverheadLines(w, h int) int {
	n := pageHeaderLines + monitorPanelLines + pageFooterLines
	if w < minWidth || h < minHeight {
		n++
	}
	if m.errBanner != "" {
		n++
	}
	if m.statusMsg != "" {
		n++
	}
	return n
}

func (m RootModel) mainPanelHeight(w, h int) int {
	mh := h - m.pageOverheadLines(w, h)
	if mh < 5 {
		return 5
	}
	return mh
}

func (m RootModel) composePage(main, footer string, w, h int) string {
	var parts []string
	parts = append(parts, ui.Header(m.headerTitle(), headerTabs, m.activeTabIndex(), w))

	if w < minWidth || h < minHeight {
		parts = append(parts, ui.BannerError(
			fmt.Sprintf("Terminal small (%dx%d); min %dx%d", w, h, minWidth, minHeight), w))
	}
	if m.errBanner != "" {
		parts = append(parts, ui.BannerError(m.errBanner, w))
	}
	if banner := m.renderStatusBanner(w); banner != "" {
		parts = append(parts, banner)
	}

	parts = append(parts, views.RenderMonitorStrip(views.MonitorData{
		Width: w, Ollama: m.ollama, GPU: m.gpu, Tokens: m.tokens,
	}))
	parts = append(parts, main)
	parts = append(parts, ui.Footer(footer, w))

	return strings.Join(parts, "\n")
}
