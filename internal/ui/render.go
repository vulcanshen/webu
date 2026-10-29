package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/ir"
)

// render lays an IR tree out for one width: rows of segments, and the list
// of items the cursor can stop on, in reading order. It is the whole of
// "draw the page" — the panel only colours what comes out and picks a
// window of rows to show.
//
// Blocks break lines; inline nodes flow together and wrap at spaces. An item
// (a link, a button, a field) is a run of segments that may span several
// rows, and every segment remembers which item it belongs to so the cursor
// can light the whole run.

type segKind uint8

const (
	segPlain segKind = iota
	segDim
	segLink
	segButton
	segInput
	segCheck
	segMedia
	segCode
	// Inside a code block with a language (highlight.go).
	segCodeKey
	segCodeString
	segCodeNumber
	segCodeConst
	segCodeKeyword
	segCodeComment
	segCodePunct
	segCodeHeading
	segCodeStrong
	segCodeEmph
	// The reader's elements (docs/reader.md): a bulleted list's icon, a
	// quote's bar, code inside a line.
	segBullet
	segQuoteBar
	segInlineCode
	// A heading's chip (docs/reader.md): its words on its level's colour.
	segChip
)

type seg struct {
	text string
	item int // index into layout.items, or -1
	kind segKind
	// attr is what the page marked this run up as — bold, italic, struck
	// through — on top of whatever its kind draws (textAttr).
	attr textAttr
}

// textAttr is the terminal's own text attributes, which is how webu draws
// the inline markup a page carries: <strong>, <em>, <del>, <ins>, <mark>
// (2026-09-22). A bitfield rather than more segKinds, because they
// compose — bold inside a link is both — and because a kind already says
// what a run IS while an attribute says how it is marked up.
type textAttr uint8

const (
	attrBold textAttr = 1 << iota
	attrItalic
	attrStrike
	attrUnderline
	attrReverse
)

// attrOf is the attribute an inline run's role draws with.
func attrOf(role string) textAttr {
	switch role {
	case "strong":
		return attrBold
	case "emphasis":
		return attrItalic
	case "deletion":
		return attrStrike
	case "insertion":
		return attrUnderline
	case "mark":
		// Reverse video: a highlight that borrows no colour band, so it
		// cannot be read as a link, as code, or as the cursor (theme).
		return attrReverse
	}
	return 0
}

type row struct {
	segs []seg
	// ground is a reader row's background across the whole row — a
	// heading's level colour, a code block's or a quote's Mantle, a
	// table's header — and ink its text colour when the ground needs one
	// (a heading's Base). Empty for none.
	ground, ink lipgloss.Color
	// chip is the colour of a heading's chip on this row (segChip),
	// empty for none.
	chip lipgloss.Color
	// code: a row of a code block, drawn on the code background across
	// the row, padding included.
	code bool
}

// plain is the row's text with no styling — what tests and the search read.
func (r row) plain() string {
	var b strings.Builder
	for _, s := range r.segs {
		b.WriteString(s.text)
	}
	return b.String()
}

type item struct {
	node        *ir.Node
	first, last int // row span, inclusive
	col         int // cell column where it starts on its first row: h/l walk a row by it
}

type layout struct {
	rows  []row
	items []item
	// marks is the first row of every landmark, heading and block — a
	// paragraph, a list item, a table — so a jump to one (the section
	// list, a search hit) has a row to land on.
	marks map[*ir.Node]int
}

// itemAt is the first item whose span includes row, or -1.
func (l layout) itemAt(row int) int {
	for i, it := range l.items {
		if row >= it.first && row <= it.last {
			return i
		}
	}
	return -1
}

// itemAtCol is the item under the character at row/col (a rune index), or
// -1: what Enter clicks in selection mode.
func (l layout) itemAtCol(row, col int) int {
	if row < 0 || row >= len(l.rows) {
		return -1
	}
	at := 0
	for _, s := range l.rows[row].segs {
		n := len([]rune(s.text))
		if col >= at && col < at+n {
			return s.item
		}
		at += n
	}
	return -1
}

// oneLine folds a value into one line for a field or a chip: the first line,
// with an ellipsis when there is more.
func oneLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + "…"
	}
	return strings.TrimSpace(s)
}

// ----------------------------------------------------------------- blocks

// ------------------------------------------------------------------ flow
