package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	terminu "github.com/vulcanshen/terminu-go"
)

// The section list: a document's outline as the screen itself, not as a
// popup over it (section.go).
//
// A row is an ordinal, a bar, and a name. The ordinal is what `[go]` jumps
// to — numbers earn their column only when a key takes you to them (user,
// 2026-09-22), so they came back with the chord that uses them, in the
// structural blue every key-shaped thing in this app wears.
//
// The indent is the hierarchy, and nothing else is. Two things were tried
// in front of it and both came out: tree(1)'s rails and elbows, which were
// more skeleton than the shape needed, and then a single bar per row,
// which was worse — a column of ticks down the page that said what the
// indent had already said (user, 2026-09-22).
//
// Depth is on the ink: one hue per level, cycled so it never runs out
// (theme.levelColor). The cursor is the panel cursor, as on any list.

// glyphCode marks a section holding a code block. Read out of the Nerd Font
// cmap, never remembered (theme.go's rule).
const glyphCode = "\U000f0169" // nf-md-code_braces

// sectionRows draws the list at innerW × innerH: its header, then a
// section a row, the way layout/panel draws an ordinary list.
func (m *AppModel) sectionRows(t *tab, innerW, innerH int) []string {
	if innerH <= 0 {
		return nil
	}
	num := lipgloss.NewStyle().Foreground(focusColor)
	meta := lipgloss.NewStyle().Foreground(pageDim)
	// The cursor is the panel cursor: Subtext1, Base in bold, from frame to
	// frame, the arrow at its end; every row keeps the arrow's width.
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor).Bold(true)
	arrow := terminu.PowerlineWidth()
	w := max(innerW-arrow, 1)
	numW := len(itoa(len(t.secs)))
	row := func(ord, stem, name, tail string, gap int) string {
		return ord + stem + name + strings.Repeat(" ", gap) + tail
	}
	out := make([]string, 0, innerH)
	// The header: Blue, upper case, held at the top (layout/panel).
	head := " " + padLeft("#", numW) + "  SECTION"
	out = append(out, num.Render(head+strings.Repeat(" ", max(0, w-dispW(head)-5))+"SIZE ")+spaces(arrow))
	end := min(len(t.secs), t.secTop+innerH-1)
	for i := t.secTop; i < end; i++ {
		s := t.secs[i]
		ord := " " + padLeft(itoa(i+1), numW) + "  "
		stem := strings.Repeat("  ", max(0, s.depth-1))
		tail := sectionTail(s)
		room := w - dispW(ord) - dispW(stem) - dispW(tail) - 2
		if room < 8 {
			// No room for the tail: the name is the only thing that matters.
			tail, room = "", w-dispW(ord)-dispW(stem)-1
		}
		name := truncate(s.title, max(1, room))
		gap := max(0, w-dispW(ord)-dispW(stem)-dispW(name)-dispW(tail))
		if i == t.sec {
			out = append(out, cur.Render(row(ord, stem, name, tail, gap))+
				lipgloss.NewStyle().Foreground(handColor).Render("\U0000E0B0")+spaces(arrow-1))
			continue
		}
		// The name wears its depth's ink: the page's own outline.
		ink := lipgloss.NewStyle().Foreground(levelColor(s.depth))
		out = append(out, num.Render(ord)+ink.Render(stem+name)+strings.Repeat(" ", gap)+meta.Render(tail)+spaces(arrow))
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
}

// sectionTail is the right-hand column: what the section holds, then how
// long it is. A glyph is drawn only when there is something to say.
func sectionTail(s section) string {
	var parts []string
	if s.tables > 0 {
		parts = append(parts, glyphTable+" "+itoa(s.tables))
	}
	if s.codes > 0 {
		parts = append(parts, glyphCode+" "+itoa(s.codes))
	}
	if s.media > 0 {
		parts = append(parts, glyphImage+" "+itoa(s.media))
	}
	return strings.Join(append(parts, itoa(s.lines())+" lines"), "  ") + " "
}
