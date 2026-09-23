package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chromedp/cdproto/cdp"
)

// editorPopup is the input popup's big sibling: a textarea's box, several
// lines tall, with two modes the way an editor has (user, 2026-09-23).
//
// It opens WRITING: every key is a character, Enter is a new line. Esc
// steps out to the box itself — MOVING — where h/j/k/l walk the cursor,
// u/d page, i and a go back to writing, Enter sets the value and a
// second Esc cancels. Esc is layered, not gone: a box that closed on
// the first Esc would lose a paragraph to a reflex.
type editorMode uint8

const (
	editorWriting editorMode = iota
	editorMoving
)

type editorPopup struct {
	anim    popupAnimator
	title   string // what the box takes, on the border (fieldTakes)
	prompt  string // the field's name, over the text
	lines   [][]rune
	row     int // cursor line
	col     int // cursor column, in runes; may equal len(line)
	top     int // first line shown
	mode    editorMode
	node    cdp.BackendNodeID
	layer   int
	screenW int
	screenH int
}

func newEditorPopup() editorPopup { return editorPopup{anim: newPopupAnimator("editor")} }

func (e editorPopup) isActive() bool      { return e.anim.isActive() }
func (e editorPopup) isInteractive() bool { return e.anim.isInteractive() }
func (e *editorPopup) close() tea.Cmd     { return e.anim.close() }
func (e *editorPopup) setSize(w, h int)   { e.screenW, e.screenH = w, h }

// typing reports whether every printable key is a character: while
// writing. While moving the keys are the editor's own, and Space is
// nothing — a box holding a paragraph is not closed by a stray Space.
func (e editorPopup) typing() bool { return e.anim.owns() }

// ask opens the box on a value, the cursor at its end, writing.
func (e *editorPopup) ask(title, prompt, value string, node cdp.BackendNodeID, layer int) tea.Cmd {
	e.title, e.prompt, e.node, e.layer = title, prompt, node, layer
	e.lines = nil
	for _, l := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		e.lines = append(e.lines, []rune(l))
	}
	e.row = len(e.lines) - 1
	e.col = len(e.lines[e.row])
	e.top, e.mode = 0, editorWriting
	e.follow()
	return e.anim.open()
}

// value is the text as it stands.
func (e editorPopup) value() string {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

// escape is Esc: out of writing into moving, or closed. True when the
// box is now closing.
func (e *editorPopup) escape() (tea.Cmd, bool) {
	if e.mode == editorWriting {
		e.mode = editorMoving
		e.clampCol()
		return nil, false
	}
	return e.close(), true
}

// update takes one keystroke. The returned value is committed when done.
func (e *editorPopup) update(msg tea.KeyMsg) (committed string, done bool) {
	if !e.anim.isInteractive() {
		return "", false
	}
	if e.mode == editorMoving {
		return e.move(msg)
	}
	switch msg.Type {
	case tea.KeyEnter:
		line := e.lines[e.row]
		rest := append([]rune(nil), line[e.col:]...)
		e.lines[e.row] = line[:e.col]
		e.lines = append(e.lines[:e.row+1], append([][]rune{rest}, e.lines[e.row+1:]...)...)
		e.row, e.col = e.row+1, 0
	case tea.KeyBackspace:
		switch {
		case e.col > 0:
			line := e.lines[e.row]
			e.lines[e.row] = append(line[:e.col-1], line[e.col:]...)
			e.col--
		case e.row > 0:
			// At a line's start: the line joins the one above.
			prev := e.lines[e.row-1]
			e.col = len(prev)
			e.lines[e.row-1] = append(prev, e.lines[e.row]...)
			e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
			e.row--
		}
	case tea.KeyLeft:
		if e.col > 0 {
			e.col--
		}
	case tea.KeyRight:
		if e.col < len(e.lines[e.row]) {
			e.col++
		}
	case tea.KeyUp:
		e.step(-1)
	case tea.KeyDown:
		e.step(1)
	case tea.KeyTab:
		e.insert([]rune("    "))
	case tea.KeySpace:
		e.insert([]rune(" "))
	case tea.KeyRunes:
		e.insert(msg.Runes)
	}
	e.follow()
	return "", false
}

// move is a key while moving: the page's vocabulary, i/a back to writing,
// Enter the value.
func (e *editorPopup) move(msg tea.KeyMsg) (string, bool) {
	switch k := msg.String(); k {
	case "enter":
		return e.value(), true
	case "h", "left":
		if e.col > 0 {
			e.col--
		}
	case "l", "right":
		if e.col < max(0, len(e.lines[e.row])-1) {
			e.col++
		}
	case "j", "down":
		e.step(1)
	case "k", "up":
		e.step(-1)
	case "d", "ctrl+d":
		e.step(max(1, e.visible()/2))
	case "u", "ctrl+u":
		e.step(-max(1, e.visible()/2))
	case "g":
		e.row, e.col = 0, 0
	case "G":
		e.row = len(e.lines) - 1
		e.clampCol()
	case "0":
		e.col = 0
	case "$":
		e.col = max(0, len(e.lines[e.row])-1)
	case "i":
		e.mode = editorWriting
	case "a":
		e.mode = editorWriting
		e.col = min(e.col+1, len(e.lines[e.row]))
	case "A":
		e.mode = editorWriting
		e.col = len(e.lines[e.row])
	case "o":
		e.lines = append(e.lines[:e.row+1], append([][]rune{nil}, e.lines[e.row+1:]...)...)
		e.row, e.col, e.mode = e.row+1, 0, editorWriting
	}
	e.follow()
	return "", false
}

func (e *editorPopup) insert(r []rune) {
	line := e.lines[e.row]
	out := make([]rune, 0, len(line)+len(r))
	out = append(out, line[:e.col]...)
	out = append(out, r...)
	out = append(out, line[e.col:]...)
	e.lines[e.row] = out
	e.col += len(r)
}

// step moves the cursor n lines, keeping the column where it can.
func (e *editorPopup) step(n int) {
	e.row = clamp(e.row+n, 0, len(e.lines)-1)
	e.clampCol()
}

// clampCol keeps the column on the line: past its end while writing
// (the caret sits after the last character), on its last character
// while moving.
func (e *editorPopup) clampCol() {
	end := len(e.lines[e.row])
	if e.mode == editorMoving {
		end = max(0, end-1)
	}
	e.col = clamp(e.col, 0, end)
}

// visible is how many lines the box shows.
func (e editorPopup) visible() int { return max(3, min(len(e.lines), e.screenH-10)) }

// follow keeps the cursor's line in the window.
func (e *editorPopup) follow() {
	vis := e.visible()
	if e.row < e.top {
		e.top = e.row
	}
	if e.row >= e.top+vis {
		e.top = e.row - vis + 1
	}
	e.top = max(0, min(e.top, max(0, len(e.lines)-vis)))
}

func (e editorPopup) view() string {
	longest := 0
	for _, l := range e.lines {
		longest = max(longest, dispW(string(l)))
	}
	innerW := popupInnerW(e.screenW, max(60, longest+6, dispW(e.prompt)+3))
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	edit := lipgloss.NewStyle().Foreground(editColor)
	// The caret: lavender while writing, the way the input popup's is;
	// the hand's colour while moving, the way the page's cursor is.
	caret := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(editColor)
	body := edit
	if e.mode == editorMoving {
		caret = lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
		body = txt
	}

	rows := []string{dim.Render(padRight(" "+e.prompt, innerW)), spaces(innerW)}
	vis := e.visible()
	width := innerW - 2 // a cell of margin either side
	for i := e.top; i < min(len(e.lines), e.top+vis); i++ {
		line := e.lines[i]
		// A line wider than the box slides so the cursor stays in it.
		from := 0
		if i == e.row && e.col >= width {
			from = e.col - width + 1
		}
		shown := line[min(from, len(line)):]
		if dispW(string(shown)) > width {
			shown = []rune(truncateNoEllipsis(string(shown), width))
		}
		var b strings.Builder
		b.WriteString(" ")
		if i == e.row {
			c := e.col - from
			before, after := shown, []rune(nil)
			under := " "
			if c < len(shown) {
				before, under, after = shown[:c], string(shown[c]), shown[c+1:]
			}
			b.WriteString(body.Render(string(before)) + caret.Render(under) + body.Render(string(after)))
		} else {
			b.WriteString(body.Render(string(shown)))
		}
		rows = append(rows, clipANSI(b.String(), innerW))
	}
	for len(rows) < vis+2 {
		rows = append(rows, spaces(innerW))
	}
	var hint string
	if e.mode == editorWriting {
		hint = hintLegend([][2]string{{"Enter", "new line"}, {"Esc", "out to the box"}})
	} else {
		hint = hintLegend([][2]string{{"hjkl", "move"}, {"i", "write"}, {"Enter", "set"}, {"Esc", "cancel"}})
	}
	where := ""
	if len(e.lines) > vis {
		where = " " + itoa(e.row+1) + "/" + itoa(len(e.lines)) + " "
	}
	return drawPopupBox(popupLayerColor(e.layer), " "+glyphPencil+" "+e.title+" ",
		hint+where, animRows(e.anim, capRows(rows, e.screenH)), innerW)
}
