package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chromedp/cdproto/cdp"
)

// inputAction says what to do with the answer. It exists so the popup itself
// stays a text box and nothing else — the same reason confirmPopup carries an
// action rather than a closure.
type inputAction int

const (
	inputNone       inputAction = iota
	inputGoto                   // open a URL in the shown tab (ux.md §7)
	inputGotoNewTab             // open a URL in a new tab
	inputField                  // write a textbox's value back to the page (ux.md §2)
	inputPrompt                 // answer a page's prompt() (function.md §5)
	inputAuthUser               // an HTTP challenge: the name, then…
	inputAuthPass               // …the password, masked
	inputFile                   // a path for a file chooser
)

// inputPopup is one line of text with a question above it — the message
// class's sibling (§6.1). It is NOT a form: a form is several fields and one
// submit, and blurring the two would make Enter mean different things on
// different floats.
//
// While it is up every printable key is a character: Space is a space and ?
// is a question mark (§4.5).
type inputPopup struct {
	anim   popupAnimator
	title  string
	glyph  string
	prompt string
	value  string
	action inputAction
	// accept is the verb on the Enter hint. The box is the same box; what
	// pressing Enter DOES is not, and the hint has to say which.
	accept string
	// node is the field the answer goes to, for inputField.
	node cdp.BackendNodeID
	// masked draws the value as dots: a password field's edit.
	masked bool
	// placeholder is shown in the empty box and vanishes at the first keystroke.
	placeholder string

	layer   int
	screenW int
	screenH int
}

func newInputPopup() inputPopup { return inputPopup{anim: newPopupAnimator("input")} }

func (m inputPopup) isActive() bool      { return m.anim.isActive() }
func (m inputPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *inputPopup) close() tea.Cmd     { return m.anim.close() }
func (m *inputPopup) setSize(w, h int)   { m.screenW, m.screenH = w, h }

// ask opens the box with value already filled in and the cursor at its end.
// Pre-filling matters for an edit: most edits change part of a value, and
// starting from empty makes the common case retype the whole thing.
func (m *inputPopup) ask(p inputPopup, layer int) tea.Cmd {
	p.anim, p.layer = m.anim, layer
	p.screenW, p.screenH = m.screenW, m.screenH
	*m = p
	return m.anim.open()
}

// update edits the line. It reports the committed value, or "" — Esc is not
// handled here, because cancelling is resolved in one place for every float
// (§4.3). Tab is a no-op on purpose (ux.md §2.1): there is one field.
func (m *inputPopup) update(msg tea.KeyMsg) (committed string, done bool) {
	if !m.anim.isInteractive() {
		return "", false
	}
	switch msg.Type {
	case tea.KeyEnter:
		return m.value, true
	case tea.KeyBackspace:
		if r := []rune(m.value); len(r) > 0 {
			m.value = string(r[:len(r)-1])
		}
	case tea.KeyCtrlU:
		m.value = ""
	case tea.KeySpace:
		m.value += " "
	case tea.KeyRunes:
		m.value += string(msg.Runes)
	}
	return "", false
}

func (m inputPopup) view() string {
	innerW := popupInnerW(m.screenW, max(44, dispW(m.value)+8, dispW(m.prompt)+3))
	dim := lipgloss.NewStyle().Foreground(dimColor)
	edit := lipgloss.NewStyle().Foreground(editColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(editColor)

	shown := m.value
	if m.masked {
		shown = ""
		for range m.value {
			shown += "•"
		}
	}
	// Lavender, because this is the field being edited (§B). Long values
	// keep their END in view: that is where the cursor is.
	value := truncateHead(shown, innerW-3)
	line := " " + edit.Render(value) + cur.Render(" ") +
		spaces(max(0, innerW-2-dispW(value)))
	if m.value == "" && m.placeholder != "" {
		ph := truncate(m.placeholder, innerW-3)
		line = " " + cur.Render(" ") + dim.Render(ph) +
			spaces(max(0, innerW-2-dispW(ph)))
	}

	rows := []string{
		dim.Render(padRight(" "+m.prompt, innerW)),
		spaces(innerW),
		line,
	}
	hint := hintLegend([][2]string{{"Enter", m.accept}, {"Esc", "cancel"}})
	return drawPopupBox(popupLayerColor(m.layer), " "+m.glyph+" "+m.title+" ",
		hint, animRows(m.anim, rows), innerW)
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
