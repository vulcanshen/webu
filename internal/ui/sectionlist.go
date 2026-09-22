package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The section list: a document's outline as the screen itself, not as a
// popup over it (section.go).
//
// A row is an ordinal, a bar, and a name. The ordinal is what `[go]` jumps
// to — numbers earn their column only when a key takes you to them (user,
// 2026-09-22), so they came back with the chord that uses them, in the
// structural blue every key-shaped thing in this app wears.
//
// The bar is the hierarchy. Connectors were tried first — tree(1)'s rails
// and elbows — and they were more skeleton than the thing needed: the
// indent already says how deep a row sits, so all the shape needs is a
// left edge to sit against. One bar, in the depth's own colour, gives the
// indent an edge and says the level twice over.
//
// Depth is on the ink: one hue per level, cycled so it never runs out
// (theme.levelColor) — and the cursor wears that hue too, so a lit row
// still says how deep it sits.

// glyphCode marks a section holding a code block. Read out of the Nerd Font
// cmap, never remembered (theme.go's rule).
const glyphCode = "\U000f0169" // nf-md-code_braces

// levelBar is the stroke a row sits against, drawn at its depth in its
// own colour.
const levelBar = "▏"

// sectionRows draws the list at innerW × innerH.
func (m AppModel) sectionRows(t *tab, innerW, innerH int) []string {
	if innerH <= 0 {
		return nil
	}
	num := lipgloss.NewStyle().Foreground(focusColor)
	meta := lipgloss.NewStyle().Foreground(pageDim)
	// The cursor takes the colour of the level it is on rather than the
	// page cursor's own grey (user, 2026-09-22): the lit row still says
	// how deep it sits, which is the one thing a highlight would
	// otherwise take away. Off focus it drops to the register an
	// unfocused chip wears, where depth is the border's business.
	focused := m.focus == panelPage && !t.onPagetab()
	cur := func(depth int) lipgloss.Style {
		bg := levelColor(depth)
		if !focused {
			bg = borderDim
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(bg).Bold(true)
	}
	numW := len(itoa(len(t.secs)))
	out := make([]string, 0, innerH)
	end := min(len(t.secs), t.secTop+innerH)
	for i := t.secTop; i < end; i++ {
		s := t.secs[i]
		ord := " " + padLeft(itoa(i+1), numW) + "  "
		// The bar sits at the row's own depth and wears its colour: the
		// indent gets an edge, and the level is said twice.
		stem := strings.Repeat("  ", max(0, s.depth-1)) + levelBar + " "
		tail := sectionTail(s)
		room := innerW - dispW(ord) - dispW(stem) - dispW(tail) - 2
		if room < 8 {
			// No room for the tail: the name is the only thing that matters.
			tail, room = "", innerW-dispW(ord)-dispW(stem)-1
		}
		name := truncate(s.title, max(1, room))
		gap := max(0, innerW-dispW(ord)-dispW(stem)-dispW(name)-dispW(tail))

		ink := lipgloss.NewStyle().Foreground(levelColor(s.depth))
		if i == t.sec {
			lit := cur(s.depth)
			out = append(out, num.Render(ord)+lit.Render(stem+name+strings.Repeat(" ", gap)+tail))
			continue
		}
		out = append(out, num.Render(ord)+ink.Render(stem+name)+
			strings.Repeat(" ", gap)+meta.Render(tail))
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
