package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/page"
)

// The Console tab (ui.md §3.2): a viewport of entries, a glyph per level,
// warnings and errors in the override colours (§2.4). An entry is shown
// whole, wrapped onto as many rows as it needs (revised 2026-09-21: it
// was one row, cut with an ellipsis, and a stack trace or a multi-line
// log was unreadable without opening its detail).

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

func (t devConsoleTab) current(filter string) (page.ConsoleEntry, bool) {
	v := t.visible(filter)
	if t.cursor < 0 || t.cursor >= len(v) {
		return page.ConsoleEntry{}, false
	}
	return v[t.cursor], true
}

// detailHead is the facts above a console entry's message in its detail.
func detailHead(e page.ConsoleEntry) []string {
	head := []string{"level    " + e.Level, "at       " + e.At.Local().Format("15:04:05")}
	if e.Where != "" {
		head = append(head, "from     "+e.Where)
	}
	return append(head, "")
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

	// An entry's rows: the time, the level's glyph and the source on the
	// first, the text wrapped under itself on the rest.
	rows := func(e page.ConsoleEntry) ([]string, lipgloss.Style) {
		glyph, style := " ", txt
		switch e.Level {
		case "error":
			glyph, style = glyphWarn, red
		case "warning":
			glyph, style = glyphWarn, peach
		case "info":
			glyph = glyphInfo
		case "input":
			// What was typed at the prompt, and what came back: the same
			// > and < Chrome's console uses.
			glyph, style = ">", dim
		case "result":
			glyph = "<"
		}
		where := ""
		if e.Where != "" {
			where = "  " + e.Where
		}
		at := e.At.Local().Format("15:04:05")
		prefixW := 1 + dispW(at) + 2 + 2
		room := max(4, innerW-prefixW-dispW(where))
		var text []string
		for _, para := range strings.Split(strings.ReplaceAll(e.Text, "\r\n", "\n"), "\n") {
			if para == "" {
				text = append(text, "")
				continue
			}
			text = append(text, wrapWords(para, room)...)
		}
		if len(text) == 0 {
			text = []string{""}
		}
		out := make([]string, len(text))
		for j, l := range text {
			if j == 0 {
				out[j] = padRight(" "+at+"  "+glyph+" "+padRight(l, room)+where, innerW)
			} else {
				out[j] = padRight(strings.Repeat(" ", prefixW)+l, innerW)
			}
		}
		return out, style
	}

	// The cursor's entry is always whole and on screen: the window ends at
	// it and reaches up as far as n rows allow (the same reading as a log
	// that fills from the bottom).
	lines := make([][]string, len(v))
	styles := make([]lipgloss.Style, len(v))
	for i, e := range v {
		lines[i], styles[i] = rows(e)
	}
	top, used := cursor, len(lines[cursor])
	for top > 0 && used+len(lines[top-1]) <= n {
		top--
		used += len(lines[top])
	}
	out := make([]string, 0, n)
	for i := top; i < len(v) && len(out) < n; i++ {
		for _, l := range lines[i] {
			if len(out) >= n {
				break
			}
			if i == cursor {
				out = append(out, cur.Render(l))
			} else {
				out = append(out, styles[i].Render(l))
			}
		}
	}
	return out
}
