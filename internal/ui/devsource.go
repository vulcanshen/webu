package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The Source tab (ui.md §3.2, revised 2026-09-20): the document's HTML as
// the page has it now, a viewport of lines. / keeps the lines that contain
// the filter, which is grep on the page.

// devSourceMsg is the HTML landing.
type devSourceMsg struct {
	tabID int
	html  string
	err   error
}

type devSourceTab struct {
	lines []string
	top   int
	err   string
	// stale: the source belongs to an earlier page; fetched again on show.
	stale bool
}

func (t *devSourceTab) set(html string, err error) {
	t.err, t.stale = "", false
	if err != nil {
		t.err = err.Error()
		t.lines = nil
		return
	}
	t.lines = strings.Split(strings.ReplaceAll(html, "\r\n", "\n"), "\n")
	for i, l := range t.lines {
		t.lines[i] = strings.ReplaceAll(l, "\t", "    ")
	}
	t.top = 0
}

func (t devSourceTab) visible(filter string) []string {
	if filter == "" {
		return t.lines
	}
	var out []string
	for _, l := range t.lines {
		if devMatches(filter, l) {
			out = append(out, l)
		}
	}
	return out
}

// move scrolls: a viewport, no cursor, no wrap.
func (t *devSourceTab) move(k string, page int, filter string) {
	t.top = moveScroll(t.top, max(0, len(t.visible(filter))-page), k, page)
}

func (t devSourceTab) view(innerW, n int, filter string) []string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	red := lipgloss.NewStyle().Foreground(warnColor)
	if t.err != "" {
		return []string{red.Render(padRight(" "+t.err, innerW))}
	}
	v := t.visible(filter)
	if len(v) == 0 {
		if t.lines == nil {
			return []string{dim.Render(padRight("  loading…", innerW))}
		}
		return []string{dim.Render(padRight("  nothing matches", innerW))}
	}
	top := clamp(t.top, 0, max(0, len(v)-1))
	out := make([]string, 0, n)
	numW := len(itoa(len(v)))
	for i := top; i < len(v) && len(out) < n; i++ {
		out = append(out, dim.Render(padLeft(itoa(i+1), numW))+txt.Render(padRight("  "+v[i], innerW-numW)))
	}
	return out
}
