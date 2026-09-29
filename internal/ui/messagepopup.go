package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// messagePopup is the message class with nothing to decide (tdp F1): a few
// lines and Esc. Inspect shows a node's facts here, a cell or a code block
// in full. Unlike a confirm it asks nothing, so Enter is not special.
type messagePopup struct {
	anim    popupAnimator
	glyph   string
	title   string
	lines   []string
	top     int // first line shown: j/k scroll a long message (a cell in full)
	layer   int
	screenW int
	screenH int
}

func newMessagePopup() messagePopup { return messagePopup{anim: newPopupAnimator("message")} }

func (m messagePopup) isActive() bool      { return m.anim.isActive() }
func (m messagePopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *messagePopup) close() tea.Cmd     { return m.anim.close() }
func (m *messagePopup) setSize(w, h int)   { m.screenW, m.screenH = w, h }

func (m *messagePopup) show(glyph, title string, lines []string, layer int) tea.Cmd {
	m.glyph, m.title, m.lines, m.layer = glyph, title, lines, layer
	m.top = 0
	return m.anim.open()
}

// help is helpMessage with j/k dimmed when the text fits and there is
// nothing to scroll (tdp M6).
func (m messagePopup) help() []helpEntry {
	out := append([]helpEntry{}, helpMessage...)
	for i := range out {
		if out[i].key == "j/k" {
			out[i].disabled = len(m.lines) <= m.visible()
		}
	}
	return out
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
	pairs := [][2]string{{"Esc", "close"}}
	if long {
		// Longer than the box: the keys that move the window.
		pairs = [][2]string{{"j/k", "scroll"}, {"Esc", "close"}}
	}
	innerW := popupW(m.screenW)
	hint := fitLegend(pairs, innerW-1)
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
