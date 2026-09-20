package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/page"
)

// The Console tab (ui.md §3.2): a viewport of lines, a glyph per level,
// warnings and errors in the override colours (§2.4). Eval is v2.

type devConsoleTab struct {
	entries []page.ConsoleEntry
	cursor  int
	top     int
}

func (t devConsoleTab) visible(filter string) []page.ConsoleEntry {
	if filter == "" {
		return t.entries
	}
	var out []page.ConsoleEntry
	for _, e := range t.entries {
		if devMatches(filter, e.Level+" "+e.Text+" "+e.Where) {
			out = append(out, e)
		}
	}
	return out
}

func (t *devConsoleTab) move(k string, page int, filter string) {
	t.cursor = moveCursor(t.cursor, len(t.visible(filter)), k, page)
}

func (t devConsoleTab) view(innerW, n int, filter string) []string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	red := lipgloss.NewStyle().Foreground(warnColor)
	peach := lipgloss.NewStyle().Foreground(peachColor)

	v := t.visible(filter)
	if len(v) == 0 {
		return []string{dim.Render(padRight("  nothing logged since this page loaded", innerW))}
	}
	cursor := clamp(t.cursor, 0, len(v)-1)
	top := clamp(scrollTo(t.top, cursor, n), 0, max(0, len(v)-1))
	out := make([]string, 0, n)
	for i := top; i < len(v) && len(out) < n; i++ {
		e := v[i]
		glyph, style := " ", txt
		switch e.Level {
		case "error":
			glyph, style = glyphWarn, red
		case "warning":
			glyph, style = glyphWarn, peach
		case "info":
			glyph = glyphInfo
		}
		where := ""
		if e.Where != "" {
			where = "  " + e.Where
		}
		at := e.At.Local().Format("15:04:05")
		text := oneLine(e.Text)
		room := innerW - 1 - dispW(at) - 2 - 2 - dispW(where)
		line := " " + at + "  " + glyph + " " + padRight(text, max(4, room)) + where
		if i == cursor {
			out = append(out, cur.Render(padRight(line, innerW)))
		} else {
			out = append(out, style.Render(padRight(line, innerW)))
		}
	}
	return out
}
