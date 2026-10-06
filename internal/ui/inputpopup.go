package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chromedp/cdproto/cdp"
	"strings"
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
	inputFill                   // a date, a time, a colour: the value set whole (page.Fill)
	inputPrompt                 // answer a page's prompt() (function.md §5)
	inputAuth                   // an HTTP challenge: name and password, one group
	inputEval                   // the console prompt: JavaScript, run in the page
	inputSetting                // a value for config.yaml, from the Settings screen
	inputFolder                 // a new bookmark folder's name, or a path of them
	inputBookmark               // a bookmark typed in: URL and title, one group (bookmarks.go)
	inputImportName             // the folder a browser's export goes under (bookmarks.go)
	inputRename                 // a bookmark's title, or a folder's name (bookmarks.go)
)

// inputPopup is one line of text with a question above it — the message
// class's sibling (tdp F1). It is NOT a form: a form is several fields and one
// submit, and blurring the two would make Enter mean different things on
// different floats.
//
// While it is up every printable key is a character: Space is a space and ?
// is a question mark (tdp K8).
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
	// shape is what an inputFill value has to look like — "YYYY-MM-DD",
	// "#rrggbb" — checked before it is sent (fill.go).
	shape string
	// search: the box is a search box, so the value written is then
	// offered to the page's Enter (inputKey).
	search bool
	// placeholder is shown dim in the empty box: an offer Tab takes and
	// Backspace declines (update).
	placeholder string
	// more are the fields after the first, for an input group (tdp K3):
	// the popup's own prompt, value, placeholder and masked are field 0.
	// at is the field the keys go to; Tab moves it, Enter submits them all.
	more []groupField
	at   int
	// refused says why the last Enter did not go through: the box stays,
	// on the field at fault.
	refused string

	layer   int
	screenW int
	screenH int
}

func newInputPopup() inputPopup { return inputPopup{anim: newPopupAnimator("input")} }

// groupField is one field of an input group after the first.
type groupField struct {
	prompt      string
	value       string
	placeholder string
	masked      bool
}

// fields is every field of the popup, the first one included, as copies.
func (m inputPopup) fields() []groupField {
	return append([]groupField{{m.prompt, m.value, m.placeholder, m.masked}}, m.more...)
}

// focused is the value and the offer of the field the keys go to.
func (m *inputPopup) focused() (*string, *string) {
	if m.at == 0 || m.at > len(m.more) {
		return &m.value, &m.placeholder
	}
	f := &m.more[m.at-1]
	return &f.value, &f.placeholder
}

// canFail reports whether a submit of this box can be refused: those boxes
// open with an error row (tdp F7), blank until a refusal writes in it.
func (a inputAction) canFail() bool {
	switch a {
	case inputBookmark, inputFill, inputSetting, inputFolder, inputImportName, inputRename:
		return true
	}
	return false
}

// refuse keeps the box up on field i, saying why (tdp K3).
func (m *inputPopup) refuse(i int, why string) {
	m.at, m.refused = i, why
}

func (m inputPopup) isActive() bool      { return m.anim.isActive() }
func (m inputPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *inputPopup) close() tea.Cmd     { return m.anim.close() }
func (m *inputPopup) setSize(w, h int)   { m.screenW, m.screenH = w, h }

// ask opens the box with value already filled in and the cursor at its end.
// Pre-filling matters for an edit: most edits change part of a value, and
// starting from empty makes the common case retype the whole thing.
// What is filled in goes through the paste's filter (oneline.go).
func (m *inputPopup) ask(p inputPopup, layer int) tea.Cmd {
	p.value, p.placeholder = cleanValue(p.value), cleanValue(p.placeholder)
	for i := range p.more {
		p.more[i].value, p.more[i].placeholder = cleanValue(p.more[i].value), cleanValue(p.more[i].placeholder)
	}
	p.anim, p.layer = m.anim, layer
	p.screenW, p.screenH = m.screenW, m.screenH
	*m = p
	return m.anim.open()
}

// update edits the line. It reports the committed value, or "" — Esc is not
// handled here, because cancelling is resolved in one place for every float
// (tdp K4).
//
// The placeholder is an offer, not a value (ux.md §2.1, revised
// 2026-09-20): the go-to box opens on the page's own URL the way Chrome's
// Cmd+L does. Tab takes the offer into the line to edit; Backspace on an
// empty line declines it outright; typing starts fresh over it. Enter
// commits the line as typed — an offer nobody took is not sent anywhere.
func (m *inputPopup) update(msg tea.KeyMsg) (committed string, done bool) {
	if !m.anim.isInteractive() {
		return "", false
	}
	value, offer := m.focused()
	switch msg.Type {
	case tea.KeyEnter:
		// Always the whole box, one field or a group (tdp K3).
		return m.value, true
	case tea.KeyTab, tea.KeyShiftTab:
		// In a group, field to field and nothing else; in a single box,
		// which has no field to go to, Tab accepts the offer (tdp K2,
		// v0.1.6). Shift-Tab only ever moves.
		if n := len(m.more) + 1; n > 1 {
			d := 1
			if msg.Type == tea.KeyShiftTab {
				d = n - 1
			}
			m.at = (m.at + d) % n
			return "", false
		}
		if msg.Type == tea.KeyShiftTab {
			return "", false
		}
		if *value == "" && *offer != "" {
			*value, *offer = *offer, ""
		}
	case tea.KeyRight:
		// → accepts the offer in any box: in a group it is the only key
		// that does, since Tab moves between fields there (tdp K2).
		if *value == "" && *offer != "" {
			*value, *offer = *offer, ""
		}
	case tea.KeyBackspace:
		if r := []rune(*value); len(r) > 0 {
			*value = string(r[:len(r)-1])
		} else {
			*offer = ""
		}
	case tea.KeyCtrlU:
		*value = ""
	case tea.KeySpace:
		*value += " "
	case tea.KeyRunes:
		*value += takeText(msg.Runes)
	default:
		return "", false
	}
	m.refused = ""
	return "", false
}

func (m inputPopup) view() string {
	fields := m.fields()
	innerW := popupW(m.screenW) // one width for every popup (tdp F7)
	dim := lipgloss.NewStyle().Foreground(dimColor)
	edit := lipgloss.NewStyle().Foreground(editColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(editColor)
	warn := lipgloss.NewStyle().Foreground(warnColor)

	var rows []string
	for i, f := range fields {
		if i > 0 {
			rows = append(rows, spaces(innerW))
		}
		on := i == m.at
		shown := f.value
		if f.masked {
			shown = strings.Repeat("•", len([]rune(f.value)))
		}
		// Lavender, because this is the field being edited (tdp P4). Long
		// values keep their END in view: that is where the cursor is. In a
		// group only the field the keys go to wears the cursor.
		caret := " "
		if on {
			caret = cur.Render(" ")
		}
		value, vw := valueView(shown, innerW-3, edit, warn, true)
		line := " " + value + caret + spaces(max(0, innerW-2-vw))
		if f.value == "" && f.placeholder != "" {
			// An offer is grey throughout, its `\n` too: not a value yet.
			ph, pw := valueView(f.placeholder, innerW-3, lipgloss.NewStyle(), lipgloss.NewStyle(), false)
			line = " " + caret + dim.Render(ph) + spaces(max(0, innerW-2-pw))
		}
		prompt := dim.Render(padRight(" "+f.prompt, innerW))
		if on && len(fields) > 1 {
			prompt = edit.Render(padRight(" "+f.prompt, innerW))
		}
		rows = append(rows, prompt, spaces(innerW), line)
	}
	// A box whose submit can fail has its error row from the start, so a
	// refusal writes into it and the box keeps its height (tdp F7, K3).
	if m.action.canFail() {
		rows = append(rows, spaces(innerW), warn.Render(padRight(" "+truncate(m.refused, innerW-2), innerW)))
	}

	hint := fitLegend(m.legend(fields, false), innerW-1)
	return drawPopupBox(popupLayerColor(m.layer), " "+m.glyph+" "+m.title+" ",
		hint, animRows(m.anim, rows), innerW)
}

// legend is the bottom border's keys: the offer's two only while the field
// the keys go to has one standing — or, widest, while any field does.
func (m inputPopup) legend(fields []groupField, widest bool) [][2]string {
	pairs := [][2]string{{"Enter", m.accept}}
	if len(fields) > 1 {
		pairs = append(pairs, [2]string{"Tab", "next field"})
	}
	offer := false
	for i, f := range fields {
		if (widest || i == m.at) && f.value == "" && f.placeholder != "" {
			offer = true
		}
	}
	// One key per action on the legend: a single box accepts with Tab, a
	// group with → (→ works in a single box too, unlisted). Backspace declines.
	if offer {
		take := "Tab"
		if len(fields) > 1 {
			take = "→"
		}
		pairs = append(pairs, [2]string{take, "accept"}, [2]string{"Backspace", "decline"})
	}
	return append(pairs, [2]string{"Esc", "cancel"})
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
