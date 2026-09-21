package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpPopup is the §A.2 non-contextual entry point: every global action the app
// has, reachable from any surface. Its completeness is the promise — a user who
// never read a README finds the whole global vocabulary here.
type helpPopup struct {
	anim    popupAnimator
	top     int
	layer   int
	screenW int
	screenH int
}

func newHelpPopup() helpPopup { return helpPopup{anim: newPopupAnimator("help")} }

func (m helpPopup) isActive() bool      { return m.anim.isActive() }
func (m helpPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *helpPopup) open(layer int) tea.Cmd {
	m.layer, m.top = layer, 0
	return m.anim.open()
}
func (m *helpPopup) close() tea.Cmd   { return m.anim.close() }
func (m *helpPopup) setSize(w, h int) { m.screenW, m.screenH = w, h }

// helpEntry is one line: a section header (key == "") or a key/description pair.
type helpEntry struct{ key, desc string }

// helpContent is the whole global vocabulary (ux.md §A.2 and the appendix).
// The core keys come first because they are the five a user has to hold to
// walk the app (§A.0.K).
var helpContent = []helpEntry{
	{"", "Core keys"},
	{"Tab · 1-2", "next panel / this panel"},
	{"Enter", "what can I do with this item (first row: the obvious thing)"},
	{"Esc", "close the top float; on a screen, back to Web"},
	{"Space", "what can I do here: the item and the panel"},
	{"?", "this help"},
	{"", "Global"},
	{"W · B · H · D · S", "the screens: Web · Bookmarks · History · Downloads · Settings"},
	{"P · N", "previous / next page"},
	{"L", "go to a URL: this page's own is offered, Tab edits it"},
	{"/", "search the page (enters selection mode)"},
	{"v", "visual mode: walk the text by character, v/V select, y copy"},
	{"q", "quit"},
	{"Ctrl+C", "force quit"},
	{"", "Navigate"},
	{"j · k", "next / previous item"},
	{"u · d", "half a page"},
	{"gg · G", "first / last"},
}

func (m *helpPopup) update(msg tea.KeyMsg) {
	if !m.anim.isInteractive() {
		return
	}
	m.top = moveScroll(m.top, max(0, len(helpContent)-m.visible()), msg.String(), m.visible())
}

// visible is how many content lines fit; the box costs 4 rows of chrome.
func (m helpPopup) visible() int { return max(1, min(len(helpContent), m.screenH-6)) }

func (m helpPopup) view() string {
	keyW := 0
	for _, e := range helpContent {
		keyW = max(keyW, dispW(e.key))
	}
	innerW := popupInnerW(m.screenW, keyW+44)

	dim := lipgloss.NewStyle().Foreground(dimColor)
	key := lipgloss.NewStyle().Foreground(handColor)
	txt := lipgloss.NewStyle().Foreground(textColor)

	vis := m.visible()
	end := min(len(helpContent), m.top+vis)
	rows := make([]string, 0, vis)
	for _, e := range helpContent[m.top:end] {
		if e.key == "" {
			rows = append(rows, dim.Render(padRight(" "+e.desc, innerW)))
			continue
		}
		rows = append(rows, key.Render(padRight("  "+e.key, keyW+4))+
			txt.Render(padRight(e.desc, innerW-keyW-4)))
	}

	pairs := [][2]string{{"Esc", "close"}}
	if len(helpContent) > vis {
		pairs = append([][2]string{{"j/k", "scroll"}}, pairs...)
	}
	hint := hintLegend(pairs)
	return drawPopupBox(popupLayerColor(m.layer), " "+glyphHelp+" Help ", hint,
		animRows(m.anim, capRows(rows, m.screenH)), innerW)
}
