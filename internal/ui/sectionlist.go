package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The section list: a document's outline as the screen itself, not as a
// popup over it (section.go).
//
// It is drawn the way a file system is drawn — tree(1), a file manager —
// because that is what it is: a hierarchy you walk, where every row has a
// parent and some rows have children. That idiom brings its own answers
// for free. The shape is carried by the connectors, not by indentation
// alone, so a deep row still says what it hangs off. And a row needs no
// ordinal: `tree` does not number its files, because the thing you do with
// a row is walk to it or filter for it, never recite its index (user,
// 2026-09-22 — numbers earn a column only if a key jumps to them, and the
// digits are the panel switches).
//
// Depth is on the ink: one hue per level, cycled so it never runs out
// (theme.levelColor) — and the cursor wears that hue too, so a lit row
// still says how deep it sits. The connectors are one quiet
// colour of their own — they are the skeleton, and a skeleton that
// competes with the names on it is drawn wrong.

// glyphCode marks a section holding a code block. Read out of the Nerd Font
// cmap, never remembered (theme.go's rule).
const glyphCode = "\U000f0169" // nf-md-code_braces

// sectionRows draws the list at innerW × innerH.
func (m AppModel) sectionRows(t *tab, innerW, innerH int) []string {
	if innerH <= 0 {
		return nil
	}
	tree := lipgloss.NewStyle().Foreground(pageTree)
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
	stems := treeStems(t.secs)
	out := make([]string, 0, innerH)
	end := min(len(t.secs), t.secTop+innerH)
	for i := t.secTop; i < end; i++ {
		s := t.secs[i]
		on := i == t.sec
		stem, tail := stems[i], sectionTail(s)
		room := innerW - dispW(stem) - dispW(tail) - 2
		if room < 8 {
			// No room for the tail: the name is the only thing that matters.
			tail, room = "", innerW-dispW(stem)-1
		}
		name := truncate(s.title, max(1, room))
		gap := max(0, innerW-dispW(stem)-dispW(name)-dispW(tail))

		if on {
			out = append(out, cur(s.depth).Render(stem+name+strings.Repeat(" ", gap)+tail))
			continue
		}
		out = append(out, tree.Render(stem)+
			lipgloss.NewStyle().Foreground(levelColor(s.depth)).Render(name)+
			strings.Repeat(" ", gap)+meta.Render(tail))
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
}

// treeStems is the connector drawn in front of each row: for every level
// above it, a rail if that ancestor still has rows to come, blank if it
// does not; then the branch itself, a tee or an elbow depending on whether
// anything follows at its own depth. The same shape tree(1) draws.
//
// The root level takes no connector — a page's top-level sections hang off
// the page, and drawing a stem for that would be drawing the panel.
func treeStems(secs []section) []string {
	out := make([]string, len(secs))
	for i, s := range secs {
		var b strings.Builder
		for d := 2; d < s.depth; d++ {
			if hasMoreAt(secs, i, d) {
				b.WriteString("│  ")
			} else {
				b.WriteString("   ")
			}
		}
		if s.depth > 1 {
			if hasMoreAt(secs, i, s.depth) {
				b.WriteString("├─ ")
			} else {
				b.WriteString("└─ ")
			}
		}
		out[i] = " " + b.String()
	}
	return out
}

// hasMoreAt reports whether another row at depth d follows row i before
// the branch they share ends — whether the rail continues past this row.
func hasMoreAt(secs []section, i, d int) bool {
	for j := i + 1; j < len(secs); j++ {
		switch {
		case secs[j].depth < d:
			return false
		case secs[j].depth == d:
			return true
		}
	}
	return false
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
