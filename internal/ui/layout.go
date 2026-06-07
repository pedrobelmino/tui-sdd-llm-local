package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tab is a header navigation entry (e.g. key "1", label "Overview").
type Tab struct {
	Key   string
	Label string
}

// Header renders a k9s-style title bar and tab row at the given width.
func Header(title string, tabs []Tab, active int, width int) string {
	if width < 1 {
		return ""
	}

	border := lipgloss.NormalBorder()
	h := string(border.Top)

	titleSegment := "─ " + title + " "
	titlePad := width - lipgloss.Width(titleSegment) - 2
	if titlePad < 0 {
		titleSegment = truncateVisible(titleSegment, width-2)
		titlePad = 0
	}
	topLine := border.TopLeft + titleSegment + strings.Repeat(h, titlePad) + border.TopRight

	var tabParts []string
	for i, tab := range tabs {
		label := fmt.Sprintf("[%s] %s", tab.Key, tab.Label)
		style := StyleHeader.Copy()
		if i == active {
			style = style.Underline(true)
		}
		tabParts = append(tabParts, render(style, label))
	}
	tabRow := " " + strings.Join(tabParts, "  ")
	tabInner := truncateVisible(tabRow, width-2)
	tabLine := border.Left + tabInner + strings.Repeat(" ", width-2-lipgloss.Width(tabInner)) + border.Right

	return topLine + "\n" + tabLine
}

// Footer renders a keybinding hint line truncated to width.
func Footer(keymap string, width int) string {
	if width < 1 {
		return ""
	}

	border := lipgloss.NormalBorder()
	inner := " " + truncateVisible(keymap, width-3)
	innerPad := width - 2 - lipgloss.Width(inner)
	line := border.Left + render(StyleDim, inner) + strings.Repeat(" ", innerPad) + border.Right

	bottomPad := width - 2
	if bottomPad < 0 {
		bottomPad = 0
	}
	bottom := border.BottomLeft + strings.Repeat(string(border.Bottom), bottomPad) + border.BottomRight

	return line + "\n" + bottom
}

// MaxScroll returns the largest scroll offset that still fills the viewport.
func MaxScroll(totalLines, visibleLines int) int {
	if visibleLines < 1 {
		visibleLines = 1
	}
	max := totalLines - visibleLines
	if max < 0 {
		return 0
	}
	return max
}

// WrapLines splits on newlines and wraps long lines to maxWidth (visible runes).
func WrapLines(text string, maxWidth int) []string {
	if maxWidth < 1 {
		maxWidth = 1
	}
	var out []string
	for _, raw := range strings.Split(text, "\n") {
		line := raw
		for lipgloss.Width(line) > maxWidth {
			cut := truncateVisible(line, maxWidth)
			if cut == "" {
				runes := []rune(line)
				out = append(out, string(runes[:1]))
				line = string(runes[1:])
				continue
			}
			out = append(out, cut)
			line = strings.TrimPrefix(line, cut)
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// PanelViewport renders a titled box showing content scrolled by scrollLine.
func PanelViewport(title, content string, width, height, scrollLine int) string {
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}

	border := lipgloss.RoundedBorder()
	innerW := width - 2
	bodyH := height - 2
	if bodyH < 1 {
		bodyH = 1
	}

	titlePrefix := "─ " + title + " "
	titlePad := innerW - lipgloss.Width(titlePrefix)
	if titlePad < 0 {
		titlePrefix = truncateVisible(titlePrefix, innerW)
		titlePad = innerW - lipgloss.Width(titlePrefix)
	}
	top := border.TopLeft + titlePrefix + strings.Repeat(string(border.Top), titlePad) + border.TopRight

	contentLines := WrapLines(content, innerW-1)
	maxScroll := MaxScroll(len(contentLines), bodyH)
	if scrollLine > maxScroll {
		scrollLine = maxScroll
	}
	if scrollLine < 0 {
		scrollLine = 0
	}

	lines := make([]string, bodyH)
	for i := 0; i < bodyH; i++ {
		idx := scrollLine + i
		var line string
		if idx < len(contentLines) {
			line = contentLines[idx]
		}
		line = truncateVisible(line, innerW-1)
		cell := " " + line
		pad := innerW - lipgloss.Width(cell)
		lines[i] = border.Left + cell + strings.Repeat(" ", pad) + border.Right
	}

	bottom := border.BottomLeft + strings.Repeat(string(border.Bottom), innerW) + border.BottomRight
	return render(StylePanel, top) + "\n" + strings.Join(lines, "\n") + "\n" + render(StylePanel, bottom)
}

// Panel renders a titled box with content inside at the given dimensions.
func Panel(title, content string, width, height int) string {
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}

	border := lipgloss.RoundedBorder()
	innerW := width - 2

	titlePrefix := "─ " + title + " "
	titlePad := innerW - lipgloss.Width(titlePrefix)
	if titlePad < 0 {
		titlePrefix = truncateVisible(titlePrefix, innerW)
		titlePad = innerW - lipgloss.Width(titlePrefix)
	}
	top := border.TopLeft + titlePrefix + strings.Repeat(string(border.Top), titlePad) + border.TopRight

	contentLines := strings.Split(content, "\n")
	bodyH := height - 2
	lines := make([]string, bodyH)
	for i := 0; i < bodyH; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		line = truncateVisible(line, innerW-1)
		content := " " + line
		pad := innerW - lipgloss.Width(content)
		lines[i] = border.Left + content + strings.Repeat(" ", pad) + border.Right
	}

	bottom := border.BottomLeft + strings.Repeat(string(border.Bottom), innerW) + border.BottomRight
	return render(StylePanel, top) + "\n" + strings.Join(lines, "\n") + "\n" + render(StylePanel, bottom)
}

// BannerError renders a full-width error banner for the TUI header area.
func BannerError(msg string, width int) string {
	if width < 1 {
		return render(StyleError, msg)
	}
	inner := truncateVisible(msg, width-4)
	pad := width - lipgloss.Width(inner) - 4
	if pad < 0 {
		pad = 0
	}
	banner := "!! " + inner + " " + strings.Repeat("!", pad)
	return render(StyleError, banner)
}

// PlainError formats an error for Cobra/plain mode, respecting NO_COLOR.
func PlainError(msg string) string {
	return render(StyleError, "error: "+msg)
}

func truncateVisible(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	for _, r := range runes {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate) > max {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}
