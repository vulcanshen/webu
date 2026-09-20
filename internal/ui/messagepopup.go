package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// messagePopup is the message class with nothing to decide (§6.1): a few
// lines and Esc. Inspect shows a node's facts here; selection mode shows
// its cheatsheet here. Unlike a confirm it asks nothing, so Enter is not
// special.
//
// passKeys is the cheatsheet's rule (ux.md §A.1): a key listed on it is
// pressed THROUGH it — the popup closes and the key runs — so reading the
// sheet and acting on it is one step, not two.
type messagePopup struct {
	anim     popupAnimator
	glyph    string
	title    string
	lines    []string
	passKeys bool
	layer    int
	screenW  int
	screenH  int
}

func newMessagePopup() messagePopup { return messagePopup{anim: newPopupAnimator("message")} }

func (m messagePopup) isActive() bool      { return m.anim.isActive() }
func (m messagePopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *messagePopup) close() tea.Cmd     { return m.anim.close() }
func (m *messagePopup) setSize(w, h int)   { m.screenW, m.screenH = w, h }

func (m *messagePopup) show(glyph, title string, lines []string, passKeys bool, layer int) tea.Cmd {
	m.glyph, m.title, m.lines, m.passKeys, m.layer = glyph, title, lines, passKeys, layer
	return m.anim.open()
}

func (m messagePopup) view() string {
	w := dispW(m.title) + 6
	for _, l := range m.lines {
		w = max(w, dispW(l)+4)
	}
	innerW := popupInnerW(m.screenW, w)
	txt := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimColor)
	rows := make([]string, 0, len(m.lines))
	for _, l := range m.lines {
		// A line starting with two spaces is a key/description pair drawn
		// dim; the rest is content.
		style := txt
		if len(l) > 1 && l[0] == ' ' && l[1] == ' ' {
			style = dim
		}
		rows = append(rows, style.Render(padRight("  "+l, innerW)))
	}
	hint := hintLegend([][2]string{{"Esc", "close"}})
	if m.passKeys {
		hint = hintLegend([][2]string{{"a listed key", "does it"}, {"Esc", "close"}})
	}
	return drawPopupBox(popupLayerColor(m.layer), " "+m.glyph+" "+m.title+" ", hint,
		animRows(m.anim, capRows(rows, m.screenH)), innerW)
}
