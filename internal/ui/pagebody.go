package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/page"
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

// headRows is how many rows sit above the page: the URL row, and while a
// section or a thing is being read, the row that names it.
func (m *AppModel) headRows(t *tab) int {
	if t != nil && t.popupNode() == nil && (t.drilled() || (t.read && t.sec < len(t.secs))) {
		return 2
	}
	return 1
}

// Draw draws panel [2]'s inside at w × h.
func (b *pageBody) Draw(w, h int, focused bool) []string {
	m := b.m
	b.h = h
	m.relayoutTabs()
	m.syncPage()
	t := m.shownTab()
	if t == nil {
		return terminu.EmptyState(w, h, "No page", "Press [L] to enter a location, or [T] for a new tab")
	}
	out := make([]string, 0, h)
	out = append(out, m.urlRow(t, w))
	if m.headRows(t) == 2 {
		out = append(out, m.headRowOf(t, w))
	}
	rest := h - len(out)
	switch {
	case m.sel.on && t.root != nil:
		out = append(out, m.selectRows(t, w, rest)...)
	case t.errText != "":
		// What went wrong, in Red, and the key that tries again
		// (layout/panel, loading and errors).
		out = append(out, stateRows(w, rest, terminu.Fg(terminu.Red).Render(t.errText),
			terminu.Fg(terminu.Overlay0).Render("Press [R] to retry"))...)
	case t.root == nil && t.working():
		out = append(out, stateRows(w, rest, terminu.Fg(terminu.Overlay0).Render("loading…"), "")...)
	case t.root == nil:
		out = append(out, terminu.EmptyState(w, rest, "Nothing here yet", "Press [R] to load it")...)
	case t.popupNode() != nil:
		// The page under a popup: as it was, no cursor — the cursor is in
		// the float — faded the way everything under a popup is.
		back := m.pageRows(t.backdrop(), w, rest)
		for i := range back {
			back[i] = terminu.Dim(back[i])
		}
		for _, f := range m.pagePopupFloats(t, w) {
			x := max(0, (w-f.w)/2) + f.dx
			y := max(0, (rest-len(f.lines))/2) + f.dy
			back = terminu.Composite(back, f.lines, x, y, w)
		}
		out = append(out, back...)
	case t.listing():
		// A document is its sections before it is a sheet (section.go).
		out = append(out, m.sectionRows(t, w, rest)...)
	default:
		out = append(out, m.pageRows(t, w, rest)...)
	}
	return terminu.Pad2(out, w, h)
}

// stateRows is a fact and a hint, centred: an empty panel's two sentences
// (layout/screen, empty states), already coloured.
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

// urlRow is the page's address, behind the web glyph; while a section is
// open it wears the section's anchor, a section being a place. In visual
// mode a search shows its query and where it is instead.
func (m *AppModel) urlRow(t *tab, w int) string {
	if st := m.sel.status(); m.sel.on && st != "" {
		sel := lipgloss.NewStyle().Foreground(selectColor)
		return " " + sel.Render(truncate(st, w-1)) + spaces(w-1-dispW(truncate(st, w-1)))
	}
	blue := lipgloss.NewStyle().Foreground(urlColor)
	lead := " " + glyphWeb + " "
	shown := fitURL(t.url+t.sectionAnchor(), w-dispW(lead))
	return blue.Render(lead) + blue.Render(shown) + spaces(w-dispW(lead)-dispW(shown))
}

// headRowOf is the row under the URL while a section or a thing is being
// read: which one, and its name.
func (m *AppModel) headRowOf(t *tab, w int) string {
	if t.drilled() {
		// How deep, when it is more than one: the path has no bound and
		// Esc comes back one level at a time.
		var depth []string
		if n := len(t.drill); n > 1 {
			depth = append(depth, itoa(n)+"  ")
		}
		if t.drillHead >= 0 && t.drillHead < len(t.lay.rows) {
			// The thing's first line IS the header row, drawn as the page
			// draws it — its link a link, its cursor a cursor.
			return m.headRow(t, w, depth...)
		}
		return m.insideRow(t, t.drillTitle(), pageClick, w, depth...)
	}
	s := t.secs[t.sec]
	at := itoa(t.sec+1) + "/" + itoa(len(t.secs)) + "  "
	return m.insideRow(t, s.title, levelColor(s.depth), w, at)
}

// insideRow names what panel [2] is showing a piece of: the menu glyph,
// where it is, its name, and a rule to the edge.
func (m *AppModel) insideRow(t *tab, title string, ink lipgloss.Color, w int, before ...string) string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	lead := " " + glyphMenu + " " + strings.Join(before, "")
	nameStyle := lipgloss.NewStyle().Foreground(ink).Bold(true)
	if t.loading {
		nameStyle = dim
	}
	name := truncate(title, max(1, w-dispW(lead)-2))
	return dim.Render(lead) + nameStyle.Render(name) +
		dim.Render(" "+strings.Repeat("─", max(0, w-dispW(lead)-dispW(name)-1)))
}

// headRow is insideRow with a row of the page for its name: a drilled
// thing's first line, live, so the cursor can stop on what it holds.
func (m *AppModel) headRow(t *tab, w int, before ...string) string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	lead := " " + glyphMenu + " " + strings.Join(before, "")
	return dim.Render(lead) + m.rowLine(t, t.drillHead, max(1, w-dispW(lead)), m.rowStyles(t))
}

// Key moves the cursor (Rules K12); the panel's actions take the rest.
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
	if t.listing() {
		t.moveSection(k, m.pageVisible())
		m.syncPage()
		return true, nil
	}
	t.moveItem(k, m.pageVisible())
	m.syncPage()
	if n := t.current(); n != nil && n.ID != 0 {
		id := n.ID
		return true, t.act(func(ctx context.Context) error { return page.Reveal(ctx, id) })
	}
	return true, nil
}

// moveName is a key as the movement code knows it: the arrows are j and k
// (Rules K12).
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
// icon and the panel's LoadingText in its middle (layout/panel).
func (b *pageBody) Empty() bool {
	t := b.m.shownTab()
	return t != nil && t.root == nil && t.working()
}

// HasItem reports whether the cursor is on something item operations
// apply to: a section on the list, an item on the page.
func (b *pageBody) HasItem() bool {
	t := b.m.shownTab()
	if t == nil || t.root == nil {
		return false
	}
	if t.listing() {
		return len(t.secs) > 0
	}
	return t.current() != nil
}

// Scroll is where the panel is (layout/panel): the section under the
// cursor on the list, the rows in sight on the page.
func (b *pageBody) Scroll() string {
	m := b.m
	t := m.shownTab()
	if t == nil || t.root == nil || m.sel.on {
		return ""
	}
	vis := max(1, b.h-m.headRows(t))
	if t.listing() {
		vis-- // the header
		if len(t.secs) <= vis {
			return ""
		}
		return itoa(t.sec+1) + " of " + itoa(len(t.secs))
	}
	lo, hi := t.rowRange()
	total := hi - lo + 1
	if total <= vis {
		return ""
	}
	from := t.top - lo + 1
	return itoa(from) + "-" + itoa(min(total, from+vis-1)) + " of " + itoa(total)
}

// MoveKeys are the page's movement keys (Rules K6, K12).
func (b *pageBody) MoveKeys() []terminu.KeyRow {
	if t := b.m.shownTab(); t != nil && t.listing() {
		return []terminu.KeyRow{
			{Key: "j|k", Desc: "next section, previous section"},
			{Key: "u|d", Desc: "half a page up, down (PgUp, PgDn too)"},
			{Key: "gg|G", Desc: "first section, last section"},
		}
	}
	return []terminu.KeyRow{
		{Key: "j|k", Desc: "next item, previous item, in reading order"},
		{Key: "u|d", Desc: "half a page up, down (PgUp, PgDn too)"},
		{Key: "gg|G", Desc: "first item, last item"},
	}
}

// popupWidth is how wide a page's popup is laid out: a dialog's width,
// inside the panel, never the panel's — a float as wide as what it
// floats over is not a float.
func popupWidth(panelW int) int {
	return max(20, min(panelW-8, 76))
}

// popupFloat is one of the page's popups drawn as a float, and where it
// sits: the stack cascades down and to the right, three cells and a row a
// level, so the one under is still seen.
type popupFloat struct {
	lines  []string
	w      int
	dx, dy int
}

// pagePopupFloats is the page's popups as floats over the page, bottom to
// top: the one on top with the cursor and a hint, the ones under it
// dimmed, drawn as they were when they were on top (pagepopup.go). They
// are the page's content, not webu's popups (dev-remarks, 偏離 tdp).
func (m *AppModel) pagePopupFloats(t *tab, panelW int) []popupFloat {
	if len(t.popups) == 0 || t.root == nil {
		return nil
	}
	w := popupWidth(panelW)
	vis := m.popupVisible()
	var out []popupFloat
	for i, id := range t.popups {
		top := i == len(t.popups)-1
		bt := t
		if !top {
			// Under the top one: the layout it had when it was on top,
			// dimmed, no cursor — Chromium may have pruned it out of the
			// tree behind the one over it.
			lay, ok := t.popupLays[id]
			if !ok {
				continue
			}
			cp := *t
			cp.lay, cp.gutter = lay, 0
			cp.cursor, cp.top = -1, 0
			cp.read, cp.drill = false, nil
			bt = &cp
		}
		p := nodeByID(t.root, id)
		st := m.rowStyles(bt)
		rows := make([]string, 0, vis)
		lo, hi := bt.rowRange()
		end := min(hi+1, bt.top+vis)
		for r := max(bt.top, lo); r < end; r++ {
			// No line numbers: nothing in a dialog is reached by number.
			rows = append(rows, m.rowLine(bt, r, w, st))
		}
		hint := ""
		if top {
			hint = "Enter:act Space:menu"
		}
		box := floatBox(terminu.LayerColor(i+1), glyphPopup+" "+bt.popupTitleOf(p), hint, rows, w)
		if !top {
			box = terminu.DimLines(box)
		}
		out = append(out, popupFloat{lines: box, w: w + 2, dx: 3 * i, dy: i})
	}
	return out
}

// floatBox frames a page popup's rows: a rounded box in its layer colour,
// the title on the top edge, the hint on the bottom one.
func floatBox(color terminu.Color, title, hint string, rows []string, w int) []string {
	fg := terminu.Fg(color)
	title = truncate(" "+title+" ", max(1, w-2))
	out := []string{fg.Render("╭─" + title + strings.Repeat("─", max(0, w-1-dispW(title))) + "╮")}
	for _, r := range rows {
		out = append(out, fg.Render("│")+terminu.Pad(r, w)+fg.Render("│"))
	}
	bottom := "╰" + strings.Repeat("─", w) + "╯"
	if hint != "" && dispW(hint)+4 <= w {
		bottom = "╰─ " + hint + " " + strings.Repeat("─", max(0, w-3-dispW(hint))) + "╯"
	}
	return append(out, fg.Render(bottom))
}

// fitURL shrinks a URL to w cells. The host is kept whole; the path gives
// way from its middle, so the tail (usually the interesting part) survives.
func fitURL(u string, w int) string {
	if dispW(u) <= w {
		return u
	}
	scheme, rest := "", u
	if i := strings.Index(u, "://"); i >= 0 {
		scheme, rest = u[:i+3], u[i+3:]
	}
	host, path := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		host, path = rest[:i], rest[i:]
	}
	head := scheme + host
	room := w - dispW(head)
	if room < 4 {
		return truncate(head, w)
	}
	return head + truncateHead(path, room)
}

// pageRows colours the visible window of the layout. The cursor's item is
// lit across every row it spans; with the focus elsewhere the lib dims the
// whole panel, cursor and all (Principle P6).
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
// is. The panel's rows and the header row a drilled thing's first line
// becomes (headRow) are drawn by the one function, so they cannot
// disagree about what a link or a cursor looks like.
func (m *AppModel) rowLine(t *tab, i, innerW int, st rowStyles) string {
	var b strings.Builder
	used := 0
	row := t.lay.rows[i]
	// A framed block — a form — is drawn by the panel, because only
	// the panel knows how wide the row ended up (render.boxPart).
	span := min(innerW, max(1, row.boxW))
	if row.box == boxTop || row.box == boxBottom {
		b.WriteString(st.frame.Render(boxRule(row, span)))
		b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
		return b.String()
	}
	if row.box == boxSide {
		b.WriteString(st.frame.Render("│"))
		used++
	}
	for _, s := range row.segs {
		text := s.text
		if used+dispW(text) > innerW {
			text = truncate(text, innerW-used)
		}
		if text == "" {
			continue
		}
		used += dispW(text)
		switch {
		case s.item >= 0 && s.item == t.cursor:
			// On a heading the cursor wears that level's colour, the
			// way the section list's does: being under the hand must
			// not cost a row the one thing it was saying. Not on a
			// tab: the strip's chosen segment already wears it.
			b.WriteString(withAttr(st.cur, s.attr).Render(text))
		case row.code:
			style, ok := st.codeStyles[s.kind]
			if !ok {
				style = st.codeStyles[segCode]
			}
			b.WriteString(withAttr(style, s.attr).Render(text))
		case s.kind == segTabOn || s.kind == segTabOff || s.kind == segTabCap:
			// A tablist's strip, in the colour of the section it sits in,
			// the chosen tab filled with it. Dimmed with the page.
			ink := levelColor(max(1, row.heading))
			if t.loading {
				ink = dimColor
			}
			switch s.kind {
			case segTabOn:
				b.WriteString(withAttr(lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(ink).Bold(true), s.attr).Render(text))
			default:
				b.WriteString(withAttr(lipgloss.NewStyle().Foreground(ink), s.attr).Render(text))
			}
		case row.heading > 0 && !t.loading:
			// The page's own outline, drawn on the ink: one bright hue
			// per depth, cycling (theme.levelColor).
			b.WriteString(withAttr(st.styles[s.kind].Foreground(levelColor(row.heading)), s.attr).Render(text))
		case row.table:
			bg := pageTableBg
			if row.header {
				bg = pageTableHeadBg
			}
			b.WriteString(withAttr(st.styles[s.kind].Background(bg), s.attr).Render(text))
		default:
			b.WriteString(withAttr(st.styles[s.kind], s.attr).Render(text))
		}
	}
	switch {
	case row.code:
		span := min(innerW, max(used, t.textWidth()))
		b.WriteString(st.codePad.Render(strings.Repeat(" ", max(0, span-used))))
		b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
	case row.box == boxSide:
		b.WriteString(strings.Repeat(" ", max(0, span-used-1)))
		b.WriteString(st.frame.Render("│"))
		b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
	default:
		b.WriteString(strings.Repeat(" ", max(0, innerW-used)))
	}
	return b.String()
}

// boxRule is a framed block's top or bottom edge, the name of the block
// set into the top one when it has one.
func boxRule(r row, span int) string {
	left, right := "╭", "╮"
	if r.box == boxBottom {
		left, right = "╰", "╯"
	}
	name := strings.TrimSpace(r.plain())
	if name != "" {
		name = " " + truncate(name, max(1, span-6)) + " "
	}
	return left + name + strings.Repeat("─", max(0, span-2-dispW(name))) + right
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
		segPlain:       lipgloss.NewStyle().Foreground(pageText),
		segDim:         lipgloss.NewStyle().Foreground(pageDim),
		segHeading:     lipgloss.NewStyle().Foreground(pageText).Bold(true),
		segLink:        lipgloss.NewStyle().Foreground(pageClick).Underline(true),
		segButton:      lipgloss.NewStyle().Foreground(pageClick),
		segInput:       lipgloss.NewStyle().Foreground(pageInput),
		segCaret:       lipgloss.NewStyle().Foreground(pageInput).Background(pageInputBg),
		segCheck:       lipgloss.NewStyle().Foreground(pageInput),
		segMedia:       lipgloss.NewStyle().Foreground(pageMedia),
		segCode:        lipgloss.NewStyle().Foreground(pageCode),
		segUnsupported: lipgloss.NewStyle().Foreground(pageDim),
		segInvalid:     lipgloss.NewStyle().Foreground(pageInvalid),
		segTabOff:      lipgloss.NewStyle().Foreground(pageText),
		segTabOn:       lipgloss.NewStyle().Foreground(pageText).Bold(true),
		segTabCap:      lipgloss.NewStyle().Foreground(pageDim),
		segLandmark:    lipgloss.NewStyle().Foreground(pageDim).Bold(true),
		segTableHeader: lipgloss.NewStyle().Foreground(pageText).Bold(true),
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
// the digits, and one space to keep the number off the text. Every page
// has one: [go] takes you to a line by it.
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
	ink := focusColor
	if dimmed {
		ink = dimColor
	}
	return lipgloss.NewStyle().Foreground(ink).Render(" " + padLeft(itoa(n), w-2) + " ")
}
