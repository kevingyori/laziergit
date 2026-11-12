package app

import (
	"fmt"
	"strings"
)

// itemLines follows MenuViewModel.GetNonModelItems: prompt plus blank,
// then section headers (separated by blanks), unless a fuzzy filter is on.
func (m *menuOverlay) itemLines(a *App) (lines, headers []int) {
	line := 0
	if m.body != "" {
		line = len(strings.Split(m.body, "\n")) + 1
	}
	previous := ""
	for _, it := range m.visible() {
		header := -1
		if !(m.filter != "" && a.repo.Config.Gui.UseFuzzySearch()) && it.section != "" && it.section != previous {
			if previous != "" {
				line++
			}
			header = line
			line++
			previous = it.section
		}
		headers = append(headers, header)
		lines = append(lines, line)
		line++
	}
	return
}

func (m *menuOverlay) filterShown() bool { return m.filtering || m.filter != "" }

func (m *menuOverlay) footer() string {
	n := len(m.visible())
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", m.cursor+1, n)
}

func (m *menuOverlay) filterPrompt(a *App) string {
	if m.allowKeyFilter {
		return a.repo.Common.Tr.FilterPrefixMenu
	}
	return a.repo.Common.Tr.FilterPrefix
}

func (m *menuOverlay) tooltip(a *App) string {
	items := m.visible()
	if m.cursor < 0 || m.cursor >= len(items) {
		return ""
	}
	it := items[m.cursor]
	text := it.tooltip
	if it.disabled != "" {
		if text != "" {
			text += "\n\n"
		}
		text += a.repo.Common.Tr.DisabledMenuItemPrefix + it.disabled
	}
	return text
}

// textCursorVisible describes the caret actually drawn by the overlay. Menus
// keep root key dispatch even while their filter has a text caret.
func (a *App) textCursorVisible() bool {
	if m, ok := a.topOverlay().(*menuOverlay); ok && m.filterAsYouType {
		return m.filterShown()
	}
	return a.wantsTextFocus()
}
