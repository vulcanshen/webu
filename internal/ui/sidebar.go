package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Panel [1], the tabs (ui.md §2). This file also held panel [1] Places —
// three fixed rows for Bookmarks / Shortcuts / History — until those moved
// to the header row (revised 2026-09-20).

// tabsBody lists the tabs: the cursor says where you are, green says
// which one panel [2] is showing (ui.md §2). The two are independent.
func (m AppModel) tabsBody(innerW, innerH int) []string {
	if len(m.tabs) == 0 {
		return emptyBody(innerW, innerH, "no tabs",
			emptyHint("Press T to open one", "T"))
	}
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	curOff := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim)
	live := lipgloss.NewStyle().Foreground(liveColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimColor)

	// Tabs on the same URL are told apart by a number in the order they
	// were OPENED — the tab id — not the order they sit in, which X and
	// undo can change.
	seq := map[*tab]int{}
	byURL := map[string][]*tab{}
	for _, t := range m.tabs {
		if t.url != "" {
			byURL[t.url] = append(byURL[t.url], t)
		}
	}
	for _, same := range byURL {
		if len(same) < 2 {
			continue
		}
		sort.Slice(same, func(i, j int) bool { return same[i].id < same[j].id })
		for i, t := range same {
			seq[t] = i + 1
		}
	}

	top := scrollTo(m.top2, m.cur2, innerH)
	out := make([]string, 0, innerH)
	for i := top; i < len(m.tabs) && len(out) < innerH; i++ {
		t := m.tabs[i]
		title := t.title
		if title == "" {
			title = t.url
		}
		if title == "" {
			title = "new tab"
		}
		if n := seq[t]; n > 0 {
			title = "(" + itoa(n) + ") " + title
		}
		mark := " "
		if t.loading {
			mark = glyphLive
		}
		line := padRight(mark+" "+oneLine(title), innerW)
		switch {
		case i == m.cur2 && m.focus == panelTabs:
			out = append(out, cur.Render(line))
		case i == m.cur2:
			out = append(out, curOff.Render(line))
		case i == m.shown:
			out = append(out, live.Render(line))
		case t.pending:
			out = append(out, dim.Render(line))
		default:
			out = append(out, txt.Render(line))
		}
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
}

// scrollTo keeps cursor inside a window of h rows starting at top.
func scrollTo(top, cursor, h int) int {
	if h <= 0 {
		return 0
	}
	if cursor < top {
		return cursor
	}
	if cursor >= top+h {
		return cursor - h + 1
	}
	return max(0, top)
}
