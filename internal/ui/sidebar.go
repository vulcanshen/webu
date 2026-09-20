package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The side column: panel [1], three fixed rows that each open a global
// popup, and panel [2], the tabs (ui.md §2).

// side1Items is panel [1]'s constant content. Three rows, never more: the
// panel's height is a constant so the tabs below never shift.
var side1Items = []struct {
	glyph, label, key string
}{
	{glyphBookmark, "Bookmarks", "B"},
	{glyphShortcut, "Shortcuts", "S"},
	{glyphHistory, "History", "H"},
}

const panel1Rows = 3

// panel1Body draws the three rows. No "active" state here (ui.md §2): only
// the cursor.
func (m AppModel) panel1Body(innerW int) []string {
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	curOff := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim)
	txt := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimColor)
	out := make([]string, 0, panel1Rows)
	for i, it := range side1Items {
		line := padRight(" "+it.glyph+" "+it.label, innerW)
		switch {
		case i == m.cur1 && m.focus == panel1:
			out = append(out, cur.Render(line))
		case i == m.cur1:
			out = append(out, curOff.Render(line))
		default:
			out = append(out, dim.Render(" "+it.glyph+" ")+txt.Render(padRight(it.label, innerW-4)))
		}
	}
	return out
}

// panel2Body lists the tabs: the cursor says where you are, green says
// which one panel [3] is showing (ui.md §2). The two are independent.
func (m AppModel) panel2Body(innerW, innerH int) []string {
	if len(m.tabs) == 0 {
		return emptyBody(innerW, innerH, "no tabs",
			emptyHint("Press T to open one", "T"))
	}
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	curOff := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim)
	live := lipgloss.NewStyle().Foreground(liveColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimColor)

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
		mark := " "
		if t.loading {
			mark = glyphLive
		}
		line := padRight(mark+" "+oneLine(title), innerW)
		switch {
		case i == m.cur2 && m.focus == panel2:
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
