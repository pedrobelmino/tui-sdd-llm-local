package tui

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/tui/views"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ui"
)

const (
	topMonitorPanelLines    = 5
	footerMonitorPanelLines = 3
	pageHeaderLines         = 2
	pageFooterLines         = 2
)

func (m RootModel) activeTabIndex() int {
	if m.screen == ScreenFeatureDetail {
		return int(ViewFeatures) - 1
	}
	return int(m.activeView) - 1
}

func (m RootModel) isFeaturesContext() bool {
	switch m.screen {
	case ScreenFeatureDetail, ScreenAction:
		return true
	case ScreenForm:
		return m.formKind == FormNewFeatureName || m.formKind == FormFeatureBrief
	case ScreenDashboard:
		return m.activeView == ViewFeatures
	}
	return false
}

func (m RootModel) monitorPanelLines() int {
	if m.isFeaturesContext() {
		return footerMonitorPanelLines
	}
	return topMonitorPanelLines
}

func (m RootModel) pageOverheadLines(w, h int) int {
	n := pageHeaderLines + m.monitorPanelLines() + pageFooterLines
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

	if !m.isFeaturesContext() {
		parts = append(parts, views.RenderMonitorStrip(views.MonitorData{
			Width: w, Ollama: m.ollama, GPU: m.gpu, Tokens: m.tokens,
		}))
	}

	parts = append(parts, main)

	if m.isFeaturesContext() {
		parts = append(parts, views.RenderMonitorFooter(views.FooterMonitorData{
			Width: w, Ollama: m.ollama, GPU: m.gpu, Tokens: m.tokens, System: m.system,
		}))
	}

	parts = append(parts, ui.Footer(footer, w))

	return strings.Join(parts, "\n")
}
