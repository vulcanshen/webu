package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	terminu "github.com/vulcanshen/terminu-go"
)

// Panel [2]: the page (ui.md §2). The frame, its name and tabs, the
// loading icon and the dimming when the focus is elsewhere are the lib's;
// what is inside the frame is webu's own content: the URL row, the row
// that names the section or the thing being read, and the laid-out IR
// with the cursor on one item.

// pageBody is panel [2]'s inside (terminu.Body).
type pageBody struct {
	m *AppModel
	// h is the height it was last drawn at, for the scroll position.
	h int
}

// Draw draws panel [2]'s inside at w × h.
func (b *pageBody) Draw(w, h int, focused bool) []string {
	m := b.m
	b.h = h
	m.relayoutTabs()
	m.syncPage()
	t := m.shownTab()
	if t == nil {
		return terminu.EmptyState(w, h, "No page", "Press [o] to enter a location, or [T] for a new tab")
	}
	out := make([]string, 0, h)
	out = append(out, m.urlRow(t, w))
	rest := h - len(out)
	switch {
	case m.sel.on && t.root != nil:
		out = append(out, m.selectRows(t, w, rest)...)
	case t.errText != "":
		// What went wrong, in Red, and the key that tries again
		// (tdp panel.15).
		out = append(out, stateRows(w, rest, terminu.Fg(terminu.Red).Render(t.errText),
			terminu.Fg(terminu.Overlay0).Render("Press [R] to retry"))...)
	case t.root == nil && t.working():
		out = append(out, stateRows(w, rest, terminu.Fg(terminu.Overlay0).Render("loading…"), "")...)
	case t.root == nil:
		out = append(out, terminu.EmptyState(w, rest, "Nothing here yet", "Press [R] to load it")...)
	default:
		out = append(out, m.pageRows(t, w, rest)...)
	}
	return terminu.Pad2(out, w, h)
}

// stateRows is a fact and a hint, centred: an empty panel's two sentences
// (tdp screen.8), already coloured.
func stateRows(w, h int, fact, hint string) []string {
	out := make([]string, max(h, 0))
	for i := range out {
		out[i] = spaces(w)
	}
	lines := []string{fact}
	if hint != "" {
		lines = append(lines, "", hint)
	}
	top := max((h-len(lines))/2, 0)
	for i, l := range lines {
		if top+i < h {
			out[top+i] = terminu.Center(clipANSI(l, w), w)
		}
	}
	return out
}

// urlRow is the page's address, behind the web glyph: where the panel is,
// its location row (tdp panel.10), Lavender and cut at the front. In visual mode a search shows its query and where it
// is instead.
func (m *AppModel) urlRow(t *tab, w int) string {
	if st := m.sel.status(); m.sel.on && st != "" {
		sel := lipgloss.NewStyle().Foreground(selectColor)
		return " " + sel.Render(truncate(st, w-1)) + spaces(w-1-dispW(truncate(st, w-1)))
	}
	lead := " " + glyphWeb + " "
	shown := terminu.TruncateLeft(terminu.Escape(t.url), max(1, w-dispW(lead)))
	return terminu.Pad(terminu.Fg(terminu.Lavender).Render(lead+shown), w)
}

// Key moves the cursor (tdp K12); the panel's actions take the rest.
func (b *pageBody) Key(key string, _ tea.KeyMsg) (bool, tea.Cmd) {
	m := b.m
	t := m.shownTab()
	if t == nil {
		return false, nil
	}
	k := moveName(key)
	switch k {
	case "j", "k", "u", "d", "gg", "G":
	default:
		return false, nil
	}
	// Reading: the page scrolls, as in less (docs/reader.md).
	m.readerScroll(t, k)
	m.syncPage()
	return true, nil
}

// moveName is a key as the movement code knows it: the arrows are j and k
// (tdp K12).
func moveName(key string) string {
	switch key {
	case "↓":
		return "j"
	case "↑":
		return "k"
	case "PgDn":
		return "d"
	case "PgUp":
		return "u"
	}
	return key
}

// Empty reports a page that has not come yet: the lib draws the loading
// icon and the panel's LoadingText in its middle (tdp panel.15).
func (b *pageBody) Empty() bool {
	t := b.m.shownTab()
	return t != nil && t.root == nil && t.working()
}

// HasItem reports whether the cursor is on something item operations
// apply to: an item on the page, in Focus mode.
func (b *pageBody) HasItem() bool {
	t := b.m.shownTab()
	if t == nil || t.root == nil {
		return false
	}
	return t.current() != nil
}

// Scroll is where the panel is (tdp panel.1): the rows in sight. Where a
// focused link goes is the inspect toast's (focusInspect).
func (b *pageBody) Scroll() string {
	m := b.m
	t := m.shownTab()
	if t == nil || t.root == nil || m.sel.on {
		return ""
	}
	vis := max(1, b.h-1) // under the URL row
	lo, hi := t.rowRange()
	total := hi - lo + 1
	if total <= vis {
		return ""
	}
	from := t.top - lo + 1
	return itoa(from) + "-" + itoa(min(total, from+vis-1)) + " of " + itoa(total)
}

// Uncover keeps the item under Focus mode's cursor above the rows at the
// bottom the inspect toast covers, which the lib says before each Draw
// (tdp toast.4); 0 is none. Covered, the page may scroll past its end by
// those rows, so its last item still shows above the toast; uncovered, it
// sits on its end again — as terminu-go's List does.
func (b *pageBody) Uncover(rows int) {
	t := b.m.shownTab()
	if t == nil {
		return
	}
	vis := b.m.pageVisible()
	if rows > 0 && t.cursor >= 0 {
		t.scrollToCursor(max(1, vis-rows))
		return
	}
	t.top = clamp(t.top, 0, max(0, len(t.lay.rows)-vis))
}

// MoveKeys are the page's movement keys (tdp K6, tdp K12): reading, they
// scroll, as in less (docs/reader.md).
func (b *pageBody) MoveKeys() []terminu.KeyRow {
	return []terminu.KeyRow{
		{Key: "j|k", Desc: "a row down, up"},
		{Key: "u|d", Desc: "half a page up, down (PgUp, PgDn too)"},
		{Key: "gg|G", Desc: "top, bottom of the page"},
	}
}

// pageRows colours the visible window of the layout. The cursor's item is
// lit across every row it spans; with the focus elsewhere the lib dims the
// whole panel, cursor and all (tdp P6).
func (m *AppModel) pageRows(t *tab, innerW, innerH int) []string {
	if innerH <= 0 {
		return nil
	}
	st := m.rowStyles(t)
	out := make([]string, 0, innerH)
	// While one section is open the panel shows only its rows: the page
	// does not run on past the end of what is being read (section.go).
	lo, hi := t.rowRange()
	end := min(hi+1, t.top+innerH)
	// The line numbers count from what the panel is SHOWING, not from the
	// page behind it: inside a section, or inside a list item, the panel
	// is that thing, and "line 5" has to mean the fifth line of what is
	// being read (2026-09-23).
	gut := t.gutter
	innerW -= gut
	for i := max(t.top, lo); i < end; i++ {
		out = append(out, lineNum(i-lo+1, gut, t.loading)+m.rowLine(t, i, innerW, st))
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW+gut))
	}
	return out
}

// rowStyles is the dress every row of a page shares: the cursor, the
// text kinds, a code block's ground, a form's frame — all of them dimmed
// together while the page is on its way.
type rowStyles struct {
	cur        lipgloss.Style
	styles     map[segKind]lipgloss.Style
	codeStyles map[segKind]lipgloss.Style
	codePad    lipgloss.Style
	frame      lipgloss.Style
}

func (m *AppModel) rowStyles(t *tab) rowStyles {
	st := rowStyles{
		cur:    lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor).Bold(true),
		styles: segStyles(),
		// A code block's rows sit on their own background, padding
		// included, so the block reads as one thing.
		codeStyles: codeStyles(),
		codePad:    lipgloss.NewStyle().Background(pageCodeBg),
		frame:      lipgloss.NewStyle().Foreground(pageDim),
	}
	if t.loading {
		// The page on screen is the one being LEFT: it dims until the next
		// one lands, so a key pressed now is visibly pressed on nothing —
		// and the navigation keys are swallowed meanwhile (busy).
		for k := range st.styles {
			st.styles[k] = lipgloss.NewStyle().Foreground(dimColor)
		}
		st.cur = lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim)
		for k := range st.codeStyles {
			st.codeStyles[k] = lipgloss.NewStyle().Foreground(pageDim).Background(pageCodeBg)
		}
		st.frame = lipgloss.NewStyle().Foreground(dimColor)
	}
	return st
}

// rowLine draws one row of the layout at innerW, the cursor lit where it
// is.
func (m *AppModel) rowLine(t *tab, i, innerW int, st rowStyles) string {
	var b strings.Builder
	used := 0
	row := t.lay.rows[i]
	for _, s := range row.segs {
		var style lipgloss.Style
		cursor := s.item >= 0 && s.item == t.cursor
		switch {
		case cursor:
			// The cursor is the panel cursor on every item, a heading too
			// (tdp panel.5).
			style = st.cur
		case row.chip != "" && s.kind == segChip:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(row.chip)
		case row.code:
			var ok bool
			if style, ok = st.codeStyles[s.kind]; !ok {
				style = st.codeStyles[segCode]
			}
		default:
			style = st.styles[s.kind]
		}
		// A reader row on its ground: the whole row, its ink with it. Code
		// in a line keeps its own chip, but not in a heading, whose row is
		// one bar of its colour (user, 2026-10-28).
		if row.ground != "" && !cursor && (s.kind != segInlineCode || row.ink != "") {
			style = style.Background(row.ground)
			if row.ink != "" {
				style = style.Foreground(row.ink)
			}
		}
		style = withAttr(style, s.attr)
		if hits := m.findHits(t, i); len(hits) > 0 && !strings.ContainsFunc(s.text, isControl) {
			used += writeFound(&b, style, st.cur, s.text, used, innerW, hits)
			continue
		}
		if strings.ContainsFunc(s.text, isControl) {
			// What the page wrote never reaches the terminal as it is: a
			// control character is its name, in Red (tdp
			// components.5). The row is still cut to the panel.
			used += writeNamed(&b, style, controlStyle(cursor), s.text, innerW-used)
			continue
		}
		text := s.text
		if used+dispW(text) > innerW {
			text = truncate(text, innerW-used)
		}
		if text == "" {
			continue
		}
		used += dispW(text)
		b.WriteString(style.Render(text))
	}
	switch {
	case row.ground != "":
		b.WriteString(lipgloss.NewStyle().Background(row.ground).Render(strings.Repeat(" ", max(0, innerW-used))))
	case row.code:
		span := min(innerW, max(used, t.textWidth()))
		b.WriteString(st.codePad.Render(strings.Repeat(" ", max(0, span-used))))
		b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
	default:
		b.WriteString(strings.Repeat(" ", max(0, innerW-used)))
	}
	return b.String()
}

// isControl is a control character (C0, DEL, C1): never sent to the
// terminal as it is (tdp components.5).
func isControl(r rune) bool { return r < 0x20 || r >= 0x7f && r <= 0x9f }

// controlName is how a control character is shown: its name, \t, \n,
// \x1b, as the lib writes it (terminu.Escape, its colour left out).
func controlName(r rune) string { return terminu.StripStyles(terminu.Escape(string(r))) }

// controlStyle is a control character's name: Red, or Red under Base on
// the cursor (tdp color.1, the error colour).
func controlStyle(onCursor bool) lipgloss.Style {
	if onCursor {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(warnColor)
	}
	return lipgloss.NewStyle().Foreground(warnColor)
}

// writeNamed writes text in style, each control character in it by its
// name in name, no wider than room; it returns the cells it used.
func writeNamed(b *strings.Builder, style, name lipgloss.Style, text string, room int) int {
	used := 0
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			b.WriteString(style.Render(run.String()))
			run.Reset()
		}
	}
	for _, r := range text {
		piece := string(r)
		if isControl(r) {
			piece = controlName(r)
		}
		w := dispW(piece)
		if used+w > room {
			break
		}
		used += w
		if isControl(r) {
			flush()
			b.WriteString(name.Render(piece))
			continue
		}
		run.WriteString(piece)
	}
	flush()
	return used
}

// withAttr puts the page's own markup on top of whatever the run is
// already drawn as: <strong> stays bold inside a link, <del> stays
// struck through inside a table cell (render textAttr, 2026-09-22).
func withAttr(st lipgloss.Style, a textAttr) lipgloss.Style {
	if a == 0 {
		return st
	}
	if a&attrBold != 0 {
		st = st.Bold(true)
	}
	if a&attrItalic != 0 {
		st = st.Italic(true)
	}
	if a&attrStrike != 0 {
		st = st.Strikethrough(true)
	}
	if a&attrUnderline != 0 {
		st = st.Underline(true)
	}
	if a&attrReverse != 0 {
		st = st.Reverse(true)
	}
	return st
}

// segStyles is the colour of each kind of segment: one table, shared by the
// ordinary page and visual mode so the two cannot drift.
func segStyles() map[segKind]lipgloss.Style {
	return map[segKind]lipgloss.Style{
		segPlain:      lipgloss.NewStyle().Foreground(pageText),
		segDim:        lipgloss.NewStyle().Foreground(pageDim),
		segLink:       lipgloss.NewStyle().Foreground(pageClick).Underline(true),
		segButton:     lipgloss.NewStyle().Foreground(pageClick),
		segInput:      lipgloss.NewStyle().Foreground(pageInput),
		segCheck:      lipgloss.NewStyle().Foreground(pageInput),
		segMedia:      lipgloss.NewStyle().Foreground(pageMedia),
		segBullet:     lipgloss.NewStyle().Foreground(lipgloss.Color(blueHex)),
		segQuoteBar:   lipgloss.NewStyle().Foreground(lipgloss.Color(lavenderHex)),
		segInlineCode: lipgloss.NewStyle().Foreground(pageCode).Background(lipgloss.Color(surface0Hex)),
		segCode:       lipgloss.NewStyle().Foreground(pageCode),
	}
}

// codeStyles is the syntax palette inside a code block, every entry on the
// code ground.
func codeStyles() map[segKind]lipgloss.Style {
	on := func(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c).Background(pageCodeBg) }
	return map[segKind]lipgloss.Style{
		segCode:        on(pageText),
		segCodeKey:     on(pageCode),
		segCodeString:  on(pageCode),
		segCodeNumber:  on(lipgloss.Color("#f2cdcd")), // flamingo
		segCodeConst:   on(lipgloss.Color("#89dceb")), // sky: true / false / null
		segCodeKeyword: on(lipgloss.Color("#89dceb")),
		segCodeComment: on(pageDim).Italic(true),
		segCodePunct:   on(lipgloss.Color("#9399b2")), // overlay2
		segCodeHeading: on(pageText).Bold(true),
		segCodeStrong:  on(pageText).Bold(true),
		segCodeEmph:    on(pageText).Italic(true),
	}
}

// lineNumW is how wide the line-number column is for a page of n rows:
// the digits, a space to keep the number off the frame and one to keep it
// off the text. Every page has one: [go] takes you to a line by it.
func lineNumW(n int) int {
	if n <= 0 {
		return 0
	}
	return len(itoa(n)) + 2 // a cell off the frame, a cell off the text
}

// lineNum draws one: right-aligned in the gutter, dim while the page is on
// its way like everything else on it.
func lineNum(n, w int, dimmed bool) string {
	if w <= 0 {
		return ""
	}
	// Dim: the numbers are where the text is, not the text (user,
	// 2026-10-28).
	ink := pageDim
	if dimmed {
		ink = dimColor
	}
	return lipgloss.NewStyle().Foreground(ink).Render(" " + padLeft(itoa(n), w-2) + " ")
}
