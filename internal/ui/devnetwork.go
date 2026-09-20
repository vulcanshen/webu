package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/page"
)

// The Network tab (ui.md §3.2): one row per request — method, status,
// type, URL, size, time — newest at the bottom, the way the log grows.

type devNetworkTab struct {
	entries []page.NetEntry
	cursor  int
	top     int
}

func (t devNetworkTab) visible(filter string) []page.NetEntry {
	if filter == "" {
		return t.entries
	}
	var out []page.NetEntry
	for _, e := range t.entries {
		if devMatches(filter, e.Method+" "+e.URL+" "+e.Type+" "+e.Mime) {
			out = append(out, e)
		}
	}
	return out
}

func (t devNetworkTab) current(filter string) (page.NetEntry, bool) {
	v := t.visible(filter)
	if t.cursor < 0 || t.cursor >= len(v) {
		return page.NetEntry{}, false
	}
	return v[t.cursor], true
}

func (t *devNetworkTab) move(k string, page int, filter string) {
	t.cursor = moveCursor(t.cursor, len(t.visible(filter)), k, page)
}

func (t devNetworkTab) view(innerW, n int, filter string) []string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	red := lipgloss.NewStyle().Foreground(warnColor)
	peach := lipgloss.NewStyle().Foreground(peachColor)

	v := t.visible(filter)
	if len(v) == 0 {
		return []string{dim.Render(padRight("  no requests since this page loaded", innerW))}
	}
	cursor := clamp(t.cursor, 0, len(v)-1)
	top := clamp(scrollTo(t.top, cursor, n), 0, max(0, len(v)-1))
	const methodW, statusW, typeW, sizeW, timeW = 7, 4, 10, 8, 7
	urlW := max(10, innerW-1-methodW-1-statusW-1-typeW-1-sizeW-1-timeW)
	out := make([]string, 0, n)
	for i := top; i < len(v) && len(out) < n; i++ {
		e := v[i]
		status := "…"
		if e.Status > 0 {
			status = itoa(int(e.Status))
		} else if e.Error != "" {
			status = "err"
		}
		size, dur := "", ""
		if e.Done {
			size = humanBytes(e.Size)
			dur = fmt.Sprintf("%dms", e.Duration.Milliseconds())
		}
		line := " " + padRight(e.Method, methodW) + " " + padRight(status, statusW) + " " +
			padRight(strings.ToLower(e.Type), typeW) + " " + padRight(fitURL(e.URL, urlW), urlW) + " " +
			padLeft(size, sizeW) + " " + padLeft(dur, timeW)
		switch {
		case i == cursor:
			out = append(out, cur.Render(padRight(line, innerW)))
		case e.Error != "":
			out = append(out, red.Render(padRight(line, innerW)))
		case e.Status >= 400:
			out = append(out, peach.Render(padRight(line, innerW)))
		default:
			out = append(out, txt.Render(padRight(line, innerW)))
		}
	}
	return out
}

func humanBytes(n float64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%.0f B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", n/1024)
	}
	return fmt.Sprintf("%.1f MB", n/(1024*1024))
}
