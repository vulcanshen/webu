package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewerPopup is the viewport class (§6.1): long text, every row content,
// scrolled with the navigation vocabulary and never wrapping round. View
// source lives here; so would anything else that is pages of text.
type viewerPopup struct {
	anim    popupAnimator
	glyph   string
	title   string
	lines   []string
	top     int
	layer   int
	screenW int
	screenH int
}

func newViewerPopup() viewerPopup { return viewerPopup{anim: newPopupAnimator("viewer")} }

func (m viewerPopup) isActive() bool      { return m.anim.isActive() }
func (m viewerPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *viewerPopup) close() tea.Cmd     { return m.anim.close() }
func (m *viewerPopup) setSize(w, h int)   { m.screenW, m.screenH = w, h }

func (m *viewerPopup) show(glyph, title, text string, layer int) tea.Cmd {
	m.glyph, m.title, m.layer = glyph, title, layer
	m.lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, l := range m.lines {
		m.lines[i] = strings.ReplaceAll(l, "\t", "    ")
	}
	m.top = 0
	return m.anim.open()
}

// visible is how many rows fit: the box costs two borders, no padRow.
func (m viewerPopup) visible() int { return max(1, m.screenH-4) }

func (m *viewerPopup) update(msg tea.KeyMsg) {
	if !m.anim.isInteractive() {
		return
	}
	m.top = moveScroll(m.top, max(0, len(m.lines)-m.visible()), msg.String(), m.visible())
}

func (m viewerPopup) view() string {
	innerW := popupInnerW(m.screenW, m.screenW-4)
	txt := lipgloss.NewStyle().Foreground(textColor)
	vis := m.visible()
	end := min(len(m.lines), m.top+vis)
	rows := make([]string, 0, vis)
	for _, l := range m.lines[m.top:end] {
		rows = append(rows, txt.Render(padRight(" "+l, innerW)))
	}
	pos := ""
	if len(m.lines) > vis {
		pos = itoa(m.top+1) + "-" + itoa(end) + " of " + itoa(len(m.lines))
	}
	pairs := [][2]string{{"j/k", "scroll"}, {"u/d", "half page"}, {"gg/G", "ends"}, {"Esc", "close"}}
	if pos != "" {
		pairs = append([][2]string{{pos, ""}}, pairs...)
	}
	return drawPopupBoxPad(popupLayerColor(m.layer), " "+m.glyph+" "+m.title+" ",
		hintLegend(pairs), animRows(m.anim, rows), innerW, false)
}
