package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The section list: a document's outline as the screen itself, not as a
// popup over it (section.go). One row per heading — number, title indented
// by level, what it holds, how long it is — and Enter opens one to the whole
// panel.
//
// The ground is the one a heading wears in the page (theme.headingBg), so a
// level reads the same in the list as it does when you are inside it: h1 on
// the brightest, h6 on the crust. Indent says the same thing a second way,
// for a terminal with the colours flattened.

// glyphCode marks a section holding a code block. Read out of the Nerd Font
// cmap, never remembered (theme.go's rule).
const glyphCode = "\U000f0169" // nf-md-code_braces

// sectionRows draws the list at innerW × innerH.
func (m AppModel) sectionRows(t *tab, innerW, innerH int) []string {
	if innerH <= 0 {
		return nil
	}
	num := lipgloss.NewStyle().Foreground(pageDim)
	title := lipgloss.NewStyle().Foreground(pageText)
	meta := lipgloss.NewStyle().Foreground(pageDim)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	if m.focus != panelPage || t.onPagetab() {
		cur = lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim)
	}
	numW := len(itoa(len(t.secs)))
	out := make([]string, 0, innerH)
	end := min(len(t.secs), t.secTop+innerH)
	for i := t.secTop; i < end; i++ {
		s := t.secs[i]
		on := i == t.sec
		// The columns, right to left: the tail is fixed, the title takes
		// what is left of the row.
		tail := sectionTail(s)
		head := "  " + padLeft(itoa(i+1), numW) + "  " + strings.Repeat("  ", max(0, s.level-1))
		room := innerW - dispW(head) - dispW(tail) - 2
		if room < 8 {
			// No room for the tail: the title is the only thing that matters.
			tail, room = "", innerW-dispW(head)-1
		}
		name := truncate(s.title, max(1, room))
		gap := max(0, innerW-dispW(head)-dispW(name)-dispW(tail))

		var b strings.Builder
		switch {
		case on:
			b.WriteString(cur.Render(head + name + strings.Repeat(" ", gap) + tail))
		default:
			bg := lipgloss.NewStyle()
			if s.level > 0 {
				bg = bg.Background(headingBg(s.level))
			}
			b.WriteString(bg.Inherit(num).Render(head))
			b.WriteString(bg.Inherit(title).Render(name))
			b.WriteString(bg.Render(strings.Repeat(" ", gap)))
			b.WriteString(bg.Inherit(meta).Render(tail))
		}
		out = append(out, b.String())
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
