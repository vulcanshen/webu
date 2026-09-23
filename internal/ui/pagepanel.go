package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Panel [2]: the page (ui.md §2). One job, always the page: the first row is
// the URL, the second a rule, and the rest is the laid-out IR with the
// cursor on one item.

// pageHeaderRows is the URL row and the rule under it, which carries
// the page's chrome as capsules (pagetabRow).
const pageHeaderRows = 2

// spinTickMsg asks for a redraw while a page is on its way: the spinner
// in the URL row reads the clock, so the tick carries nothing and two of
// them in flight cost a redraw rather than a wrong frame (theme
// spinnerFrame).
type spinTickMsg struct{}

// spinCmd asks for the next frame, at the moment that frame is due
// rather than one step from now. The frame is read from the clock
// (theme.spinnerFrame), so a tick that lands late lands in the NEXT
// frame's window and the one it was for is never drawn — which is how
// eight slices came out looking like three (user, 2026-09-22).
func spinCmd() tea.Cmd {
	now := time.Now().UnixNano()
	due := (now/int64(spinStep) + 1) * int64(spinStep)
	return tea.Tick(time.Duration(due-now), func(time.Time) tea.Msg { return spinTickMsg{} })
}

// fetching reports whether any tab is on its way somewhere: what keeps
// the spinner turning.
func (m AppModel) fetching() bool {
	for _, t := range m.tabs {
		if t.working() {
			return true
		}
	}
	return false
}

// pageBody draws panel [2]'s inside at innerW × innerH.
func (m AppModel) pageBody(innerW, innerH int) []string {
	t := m.shownTab()
	if t == nil {
		return emptyBody(innerW, innerH, "no page",
			emptyHint("Press L to enter a location, or T for a new tab", "L", "T"))
	}
	out := make([]string, 0, innerH)
	// The first row is the URL, behind a glyph that says what state the
	// page is in: at rest, or turning while it is on its way — or, while
	// a search is being typed, the query and its count (ux.md §1.1): the
	// page under it does not move.
	if st := m.sel.status(); m.sel.on && st != "" {
		out = append(out, lipgloss.NewStyle().Foreground(selectColor).Render(padRight(" "+st, innerW)))
	} else {
		// The glyph says what state the fetch is in and the URL says
		// where: one pair, one colour (2026-09-22). The glyph alone —
		// colouring the whole row while the page came was tried and it
		// took the eye off the page (user, 2026-09-23).
		icon := glyphWeb
		if t.working() {
			icon = spinnerFrame()
		}
		blue := lipgloss.NewStyle().Foreground(urlColor)
		// While a section is open the URL wears its anchor: a section is
		// a place, and the address bar is where a place is named.
		shown := fitURL(t.url+t.sectionAnchor(), innerW-3)
		out = append(out, blue.Render(" "+icon+" ")+blue.Render(shown)+
			strings.Repeat(" ", max(0, innerW-3-dispW(shown))))
	}
	out = append(out, m.pagetabRow(t, innerW))
	rest := innerH - pageHeaderRows
	switch {
	case m.sel.on && t.root != nil:
		out = append(out, m.selectRows(t, innerW, rest)...)
	case t.errText != "":
		out = append(out, emptyBody(innerW, rest, "could not load",
			emptyHint(t.errText+" — press R to retry", "R"))...)
	case t.root == nil && t.loading:
		out = append(out, emptyBody(innerW, rest, "loading…", nil)...)
	case t.root == nil:
		out = append(out, emptyBody(innerW, rest, "nothing here yet",
			emptyHint("Press R to load it", "R"))...)
	case t.popupNode() != nil:
		// The page under a popup: as it was, dimmed the way a page being
		// left is, no cursor — the cursor is in the float (pagePopupView).
		out = append(out, m.pageRows(t.backdrop(), innerW, rest)...)
	case t.listing():
		// A document is its sections before it is a sheet (section.go):
		// the panel lists them, and Enter gives one the whole panel.
		out = append(out, m.sectionRows(t, innerW, rest)...)
	default:
		out = append(out, m.pageRows(t, innerW, rest)...)
	}
	return out
}

// popupWidth is how wide a page's popup is laid out: a dialog's width,
// inside the panel, never the panel's — a float as wide as what it
// floats over is not a float.
func popupWidth(panelW int) int {
	return max(20, min(panelW-8, 76))
}

// popupVisible is how many of the popup's rows the float shows.
func (m AppModel) popupVisible() int {
	return max(3, m.panelH()-10)
}

// pagePopupView is the page's popup as a float over the page: the same
// box every popup of webu's wears, the popup's rows inside it with the
// cursor, and a hint that says the one thing it needs said — it is
// answered, not left (pagepopup.go).
func (m AppModel) pagePopupView(t *tab) string {
	p := t.popupNode()
	if p == nil {
		return ""
	}
	w := popupWidth(m.pageW())
	title := truncate(t.popupTitle(), w-6)
	vis := m.popupVisible()
	st := m.rowStyles(t)
	gut := t.gutter
	rows := make([]string, 0, vis)
	lo, hi := t.rowRange()
	end := min(hi+1, t.top+vis)
	for i := max(t.top, lo); i < end; i++ {
		rows = append(rows, lineNum(i-lo+1, gut, false)+m.rowLine(t, i, w-gut, st))
	}
	hint := hintLegend([][2]string{{"Enter", "act"}, {"Space", "menu"}, {"Esc", "does not close it"}})
	return drawPopupBox(popupLayerColor(1), " "+glyphPopup+" "+title+" ", hint, rows, w)
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
// lit across every row it spans; on an unfocused panel it drops to the
// register an unfocused chip wears, so the eye can still find it.
func (m AppModel) pageRows(t *tab, innerW, innerH int) []string {
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
	cur, curOff lipgloss.Style
	styles      map[segKind]lipgloss.Style
	codeStyles  map[segKind]lipgloss.Style
	codePad     lipgloss.Style
	frame       lipgloss.Style
}

func (m AppModel) rowStyles(t *tab) rowStyles {
	st := rowStyles{
		cur:    lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor),
		curOff: lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim),
		styles: segStyles(),
		// A code block's rows sit on their own background, padding
		// included, so the block reads as one thing; the text width is
		// what the block spans, not the panel, so it does not run under
		// the side of the page.
		codeStyles: codeStyles(),
		codePad:    lipgloss.NewStyle().Background(pageCodeBg),
		frame:      lipgloss.NewStyle().Foreground(pageDim),
	}
	if t.loading {
		// The page on screen is the one being LEFT: it dims until the next
		// one lands, so a key pressed now is visibly pressed on nothing —
		// and the navigation keys are swallowed meanwhile (AppModel.busy).
		for k := range st.styles {
			st.styles[k] = lipgloss.NewStyle().Foreground(dimColor)
		}
		st.cur = st.curOff
		for k := range st.codeStyles {
			st.codeStyles[k] = lipgloss.NewStyle().Foreground(pageDim).Background(pageCodeBg)
		}
		st.frame = lipgloss.NewStyle().Foreground(dimColor)
	}
	return st
}

// rowLine draws one row of the layout at innerW, the cursor lit where it
// is. The panel's rows and the header row a drilled thing's first line
// becomes (insideRow) are drawn by the one function, so they cannot
// disagree about what a link or a cursor looks like.
func (m AppModel) rowLine(t *tab, i, innerW int, st rowStyles) string {
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
		case s.item >= 0 && s.item == t.cursor && m.focus == panelPage && !t.onPagetab():
			// On a heading the cursor wears that level's colour, the
			// way the section list's does: being under the hand must
			// not cost a row the one thing it was saying.
			lit := st.cur
			if row.heading > 0 && !t.loading {
				lit = st.cur.Background(levelColor(row.heading))
			}
			b.WriteString(withAttr(lit, s.attr).Render(text))
		case s.item >= 0 && s.item == t.cursor:
			b.WriteString(withAttr(st.curOff, s.attr).Render(text))
		case row.code:
			style, ok := st.codeStyles[s.kind]
			if !ok {
				style = st.codeStyles[segCode]
			}
			b.WriteString(withAttr(style, s.attr).Render(text))
		case row.heading > 0 && !t.loading:
			// The page's own outline, drawn on the ink: one bright hue
			// per depth, cycling (theme.levelColor). It used to be a
			// grey ground per level, which made every heading a band
			// and asked the eye to rank six greys (v0.2.1, replaced
			// 2026-09-22).
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
// set into the top one when it has one. A form usually has none, and a
// bare frame says all that is needed: these rows belong to each other.
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
// ordinary page and selection mode so the two cannot drift.
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
		segLandmark:    lipgloss.NewStyle().Foreground(pageDim).Bold(true),
		segTableHeader: lipgloss.NewStyle().Foreground(pageText).Bold(true), // the header row's ground tells it apart (pagepanel)
	}
}

// codeStyles is the syntax palette inside a code block, every entry on the
// code ground. Keys are mauve — the name of a value; strings are the code
// colour; the rest stay out of the bands the app reserves (green for the
// shown tab, yellow for visual mode, red and peach for the override).
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

// pageVisible is how many page rows panel [2] shows at the current size.
func (m AppModel) pageVisible() int {
	if t := m.shownTab(); t != nil && t.popupNode() != nil {
		return m.popupVisible()
	}
	return max(1, m.panelH()-2-pageHeaderRows)
}

// pageW is panel [2]'s inner width at the current size.
func (m AppModel) pageW() int {
	if m.narrow() || m.zoom {
		return max(1, m.w-2)
	}
	return max(1, m.w-sideW-2)
}

// panelFrame is panelChromeTone with a hint in the bottom border — where
// panel [2] says "loading" and "12 of 40" (ui.md §5).
func panelFrame(innerW int, body []string, title, hint string, tone borderTone) string {
	if hint == "" {
		return panelChromeTone(innerW, body, title, tone)
	}
	return panelFrameLegend(innerW, body, title, hintLegend([][2]string{{hint, ""}}), tone)
}

// panelFrameLegend is the same frame with a whole key legend in the bottom
// border: a screen's operations, listed the way a popup's hint lists them.
func panelFrameLegend(innerW int, body []string, title, legend string, tone borderTone) string {
	return panelFrameFilled(innerW, body, title, legend, tone, -1)
}

// panelFrameFilled is that frame with the border itself reading from left
// to right up to pct — how far through the open section the reader is
// (section.go). It is the same trick the header rule plays for a download
// in flight (chrome.tabRule): a progress bar that costs no row, on the
// edge that already says where you are. pct below zero draws none.
func panelFrameFilled(innerW int, body []string, title, legend string, tone borderTone, pct int) string {
	out := panelChromeTone(innerW, body, title, tone)
	lines := strings.Split(out, "\n")
	bs := lipgloss.NewStyle().Foreground(toneColor(tone))
	lw := dispW(legend)
	if lw+4 > innerW {
		return out
	}
	run := innerW - lw - 1
	rule := bs.Render(strings.Repeat("─", run))
	if pct >= 0 {
		on := clamp(run*pct/100, 0, run)
		rule = lipgloss.NewStyle().Foreground(toneColor(toneFocus)).Render(strings.Repeat("━", on)) +
			bs.Render(strings.Repeat("─", run-on))
	}
	lines[len(lines)-1] = bs.Render("╰") + rule + legend + bs.Render("─╯")
	return strings.Join(lines, "\n")
}

// pagetabRow is the pagetab: the rule under the URL, and on it the
// page's four parts (parts.go) — header, body, others, footer, whichever
// of them this page has, behind one menu glyph at the head of the strip.
// Off the page, so the page below starts at its content.
//
// The hand comes up here on Esc, or on k from the page's top; h and l
// walk the parts and the panel follows them; Enter or j takes the hand
// back down, leaving the part it walked to on screen. A bare rule when
// the page is all one part, where there is nothing to choose between.
func (m AppModel) pagetabRow(t *tab, innerW int) string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	if t.drilled() {
		// How deep, when it is more than one: the path has no bound and
		// Esc comes back one level at a time, so the row has to say how
		// many are left.
		var depth []string
		if n := len(t.drill); n > 1 {
			depth = append(depth, itoa(n)+"  ")
		}
		if t.drillHead >= 0 && t.drillHead < len(t.lay.rows) {
			// The thing's first line IS the header row, drawn as the
			// page draws it — its link a link, its cursor a cursor —
			// and not again below (tab.relayout).
			return m.headRow(t, innerW, depth...)
		}
		return m.insideRow(t, t.drillTitle(), pageClick, innerW, depth...)
	}
	if t.read && t.sec < len(t.secs) {
		return m.sectionHeadRow(t, innerW)
	}
	if len(t.parts) < 2 {
		// One part is the whole page: there is nothing to choose between,
		// so the row is a rule and Esc has nowhere to go.
		return dim.Render(strings.Repeat("─", innerW))
	}
	labels := partLabels(t.parts)
	hand := t.onPagetab() && m.focus == panelPage && !m.sel.on
	chain := partChain(labels, t.partIndex(t.at)+1, t.onPagetab(),
		m.focus == panelPage && !m.sel.on, t.loading)
	// The rule to the panel's edge belongs to the strip: with the hand up
	// here the whole row lights, edge included, which is what makes the
	// state readable from across the screen (user, 2026-09-23).
	rule := dim
	if hand && !t.loading {
		rule = lipgloss.NewStyle().Foreground(headerColor)
	}
	return chain + rule.Render(strings.Repeat("─", max(0, innerW-chainW(labels))))
}

// sectionHeadRow is the pagetab's row while one section is open: the
// menu glyph alone, then which section of how many and its name (user's
// proposal, 2026-09-22).
//
// Panel [2] is three screens now, not one — an index, a document, the
// whole sheet — and chrome identical across all three wastes the
// strongest signal the panel has. While a section is open the chrome
// that matters is which piece you are in, so the row says that; the
// pagetab shrinks to the handle that still reaches it, and Esc still
// goes up. It also lets the section's own heading come off the page
// below, where it was the third printing of the same name.
func (m AppModel) sectionHeadRow(t *tab, innerW int) string {
	s := t.secs[t.sec]
	at := itoa(t.sec+1) + "/" + itoa(len(t.secs)) + "  "
	return m.insideRow(t, s.title, levelColor(s.depth), innerW, at)
}

// insideRow is the row under the URL while the panel is showing one piece
// of the page rather than the page: the menu glyph alone, then what you
// are inside. Panel [2] is several screens now, and chrome identical
// across all of them wastes the strongest signal it has.
func (m AppModel) insideRow(t *tab, title string, ink lipgloss.Color, innerW int, before ...string) string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	lead := pagetabChain([]string{glyphMenu}, -1, m.focus == panelPage, t.loading)
	at := " " + strings.Join(before, "")
	nameStyle, atStyle := lipgloss.NewStyle().Foreground(ink).Bold(true), dim
	if t.loading {
		nameStyle = dim
	}
	used := chainW([]string{glyphMenu}) + dispW(at)
	name := truncate(title, max(1, innerW-used-2))
	return lead + atStyle.Render(at) + nameStyle.Render(name) +
		dim.Render(" "+strings.Repeat("─", max(0, innerW-used-dispW(name)-1)))
}

// headRow is insideRow with a row of the page for its name: a drilled
// thing's first line, live. What that line held — a link, a button —
// stays on the page, up here, and the cursor can stop on it.
func (m AppModel) headRow(t *tab, innerW int, before ...string) string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	lead := pagetabChain([]string{glyphMenu}, -1, m.focus == panelPage, t.loading)
	at := " " + strings.Join(before, "")
	used := chainW([]string{glyphMenu}) + dispW(at)
	return lead + dim.Render(at) + m.rowLine(t, t.drillHead, max(1, innerW-used), m.rowStyles(t))
}

// chainW is the width of a chain of labels: two caps, a space either
// side of every label, one divider between neighbours.
func chainW(labels []string) int {
	w := 2
	for i, l := range labels {
		if i > 0 {
			w++
		}
		w += dispW(l) + 2
	}
	return w
}

// pagetabChain draws the capsules as ONE powerline strip, the way the
// header draws its screens (chrome.tabChain): round cap, segments run
// together, a slanted seam between neighbours, round cap. Loose chips
// with a rule between them read as a row of buttons; a chain reads as
// one object — which the page's chrome is (2026-09-22).
//
// The whole chain is mauve (2026-09-22): the ground stays the page's,
// so it does not read as a band across the panel, and the ink says at a
// glance that this row is the page's chrome rather than its first line
// of text. At most one segment is lit — the one the hand is on — and it
// is the same mauve, filled, with the canvas for ink: the row wears one
// colour whether the hand is on it or not, rather than borrowing the
// page cursor's. Unfocused it drops to the register an unfocused chip
// wears; all of it dims while the page is on its way.
func pagetabChain(labels []string, active int, focused, dimmed bool) string {
	unlit, ink := lipgloss.Color(baseHex), headerColor
	fill := unlit
	if active >= 0 {
		fill = headerColor
	}
	if dimmed {
		ink = dimColor
		if active >= 0 {
			fill = borderDim
		}
	} else if !focused && active >= 0 {
		fill = borderDim
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(fill).Render(capLeft))
	for i, lab := range labels {
		if i > 0 {
			div, fg, bg := divider(fill, fill)
			b.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bg).Render(div))
		}
		seg := " " + lab + " "
		if fill != unlit {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).
				Background(fill).Bold(true).Render(seg))
			continue
		}
		b.WriteString(lipgloss.NewStyle().Foreground(ink).Background(unlit).Render(seg))
		_ = i
	}
	b.WriteString(lipgloss.NewStyle().Foreground(fill).Render(capRight))
	return b.String()
}

// partChain draws the parts as one powerline strip, in one of two looks
// — and which look it wears IS the answer to "where is the hand?" (user,
// 2026-09-23).
//
// At rest the strip is dim and the part on screen is the one lit thing
// on it: rosewater, filled, with the canvas for ink.
//
// With the hand on it the WHOLE strip lights — every segment filled
// rosewater, and the rule that runs from it to the panel's edge with
// them — and the part on screen inverts inside that: the canvas for
// ground, rosewater for ink. A whole row changing state is a far
// louder signal than one segment changing hue, and it buys the keys
// back: because the row says where the hand is, h and l can show the
// part as they reach it, and Enter is left to mean "yes, that one",
// which is to take the hand back down. Moving a hand and then
// confirming the move was two steps for one decision, and the lavender
// that marked the difference between them is gone with it.
//
// at is one-based, so zero is a strip with nothing on screen.
func partChain(labels []string, at int, hand, focused, dimmed bool) string {
	canvas := lipgloss.Color(baseHex)
	lit := hand && focused && !dimmed
	// ground and ink, per segment.
	dress := func(i int) (lipgloss.Color, lipgloss.Color) {
		here := i+1 == at
		switch {
		case dimmed:
			if here {
				return borderDim, canvas
			}
			return canvas, dimColor
		case lit && here:
			return canvas, headerColor
		case lit:
			return headerColor, canvas
		case here:
			return headerColor, canvas
		}
		return canvas, dimColor
	}
	ground := func(i int) lipgloss.Color { g, _ := dress(i); return g }
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ground(0)).Render(capLeft))
	for i, lab := range labels {
		if i > 0 {
			div, fg, bg := divider(ground(i-1), ground(i))
			b.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bg).Render(div))
		}
		g, fg := dress(i)
		// Bold is for the segments that carry a state: the one on screen,
		// and every one of them while the strip is lit. A dim label in
		// bold is just a heavier dim label.
		b.WriteString(lipgloss.NewStyle().Foreground(fg).Background(g).
			Bold(lit || i+1 == at).Render(" " + lab + " "))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ground(len(labels) - 1)).Render(capRight))
	return b.String()
}

// lineNumW is how wide the line-number column is for a page of n rows:
// the digits, and one space to keep the number off the text.
//
// Every page has one, not only the section list (user, 2026-09-23). A
// number earns its column when a key takes you to it, and [go] is that
// key here as it is there — on a page it asks for a line rather than a
// section (app.dispatch).
func lineNumW(n int) int {
	if n <= 0 {
		return 0
	}
	return len(itoa(n)) + 1
}

// lineNum draws one: right-aligned in the gutter, in the structural blue
// every key-shaped thing in this app wears, dim while the page is on its
// way like everything else on it.
func lineNum(n, w int, dimmed bool) string {
	if w <= 0 {
		return ""
	}
	ink := focusColor
	if dimmed {
		ink = dimColor
	}
	return lipgloss.NewStyle().Foreground(ink).Render(padLeft(itoa(n), w-1) + " ")
}
