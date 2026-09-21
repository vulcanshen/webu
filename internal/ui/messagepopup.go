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
	top      int // first line shown: j/k scroll a long message (a cell in full)
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
	m.top = 0
	return m.anim.open()
}

// visible is how many lines the box shows: capRows' budget.
func (m messagePopup) visible() int { return max(1, m.screenH-6) }

// scroll moves the window by the page's own keys — j/k, u/d, G — when
// the message is longer than the box; other keys do nothing.
func (m *messagePopup) scroll(k string) {
	vis := m.visible()
	half := max(1, vis/2)
	switch k {
	case "j", "down":
		m.top++
	case "k", "up":
		m.top--
	case "d", "ctrl+d":
		m.top += half
	case "u", "ctrl+u":
		m.top -= half
	case "G":
		m.top = len(m.lines)
	case "g":
		m.top = 0
	}
	m.top = max(0, min(m.top, max(0, len(m.lines)-vis)))
}

func (m messagePopup) view() string {
	vis := m.visible()
	long := len(m.lines) > vis
	hint := hintLegend([][2]string{{"Esc", "close"}})
	switch {
	case m.passKeys:
		hint = hintLegend([][2]string{{"a listed key", "does it"}, {"Esc", "close"}})
	case long:
		// Longer than the box: the keys that move the window.
		hint = hintLegend([][2]string{{"j/k", "scroll"}, {"Esc", "close"}})
	}
	w := max(dispW(m.title)+6, dispW(hint)+1)
	for _, l := range m.lines {
		w = max(w, dispW(l)+4)
	}
	innerW := popupInnerW(m.screenW, w)
	txt := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimColor)
	lines := m.lines
	if long {
		top := max(0, min(m.top, len(lines)-vis))
		lines = lines[top : top+vis]
	}
	rows := make([]string, 0, len(lines))
	for _, l := range lines {
		// A line starting with two spaces is a key/description pair drawn
		// dim; the rest is content.
		style := txt
		if len(l) > 1 && l[0] == ' ' && l[1] == ' ' {
			style = dim
		}
		rows = append(rows, style.Render(padRight("  "+l, innerW)))
	}
	return drawPopupBox(popupLayerColor(m.layer), " "+m.glyph+" "+m.title+" ", hint,
		animRows(m.anim, capRows(rows, m.screenH)), innerW)
}
