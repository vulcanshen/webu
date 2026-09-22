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

func spinCmd() tea.Cmd {
	return tea.Tick(spinStep, func(time.Time) tea.Msg { return spinTickMsg{} })
}

// fetching reports whether any tab is on its way somewhere: what keeps
// the spinner turning.
func (m AppModel) fetching() bool {
	for _, t := range m.tabs {
		if t.loading {
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
		// where: one pair, one colour (2026-09-22).
		icon := glyphWeb
		if t.loading {
			icon = spinnerFrame()
		}
		blue := lipgloss.NewStyle().Foreground(urlColor)
		url := blue.Render(fitURL(t.url, innerW-3))
		out = append(out, blue.Render(" "+icon+" ")+url+strings.Repeat(" ", max(0, innerW-3-dispW(fitURL(t.url, innerW-3)))))
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
	default:
		out = append(out, m.pageRows(t, innerW, rest)...)
	}
	return out
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
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	curOff := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(borderDim)
	styles := segStyles()
	if t.loading {
		// The page on screen is the one being LEFT: it dims until the next
		// one lands, so a key pressed now is visibly pressed on nothing —
		// and the navigation keys are swallowed meanwhile (AppModel.busy).
		for k := range styles {
			styles[k] = lipgloss.NewStyle().Foreground(dimColor)
		}
		cur = curOff
	}
	// A code block's rows sit on their own background, padding included,
	// so the block reads as one thing; the text width is what the block
	// spans, not the panel, so it does not run under the side of the page.
	codeStyles := codeStyles()
	codePad := lipgloss.NewStyle().Background(pageCodeBg)
	if t.loading {
		for k := range codeStyles {
			codeStyles[k] = lipgloss.NewStyle().Foreground(pageDim).Background(pageCodeBg)
		}
	}
	out := make([]string, 0, innerH)
	end := min(len(t.lay.rows), t.top+innerH)
	for i := t.top; i < end; i++ {
		var b strings.Builder
		used := 0
		row := t.lay.rows[i]
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
				b.WriteString(withAttr(cur, s.attr).Render(text))
			case s.item >= 0 && s.item == t.cursor:
				b.WriteString(withAttr(curOff, s.attr).Render(text))
			case row.code:
				style, ok := codeStyles[s.kind]
				if !ok {
					style = codeStyles[segCode]
				}
				b.WriteString(withAttr(style, s.attr).Render(text))
			case row.heading > 0:
				// The page's own outline, drawn as weight: h1 on the
				// brightest ground, h6 on the crust (theme.headingBg).
				b.WriteString(withAttr(styles[s.kind].Background(headingBg(row.heading)), s.attr).Render(text))
			case row.table:
				bg := pageTableBg
				if row.header {
					bg = pageTableHeadBg
				}
				b.WriteString(withAttr(styles[s.kind].Background(bg), s.attr).Render(text))
			default:
				b.WriteString(withAttr(styles[s.kind], s.attr).Render(text))
			}
		}
		switch {
		case row.code:
			span := min(innerW, max(used, t.textWidth()))
			b.WriteString(codePad.Render(strings.Repeat(" ", max(0, span-used))))
			b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
		case row.heading > 0:
			// The ground runs to the text width, so a heading is a band
			// rather than a tinted word.
			span := min(innerW, max(used, t.textWidth()))
			pad := lipgloss.NewStyle().Background(headingBg(row.heading))
			b.WriteString(pad.Render(strings.Repeat(" ", max(0, span-used))))
			b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
		default:
			b.WriteString(strings.Repeat(" ", max(0, innerW-used)))
		}
		out = append(out, b.String())
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
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
		segLink:        lipgloss.NewStyle().Foreground(pageLink).Underline(true),
		segButton:      lipgloss.NewStyle().Foreground(pageText).Bold(true),
		segInput:       lipgloss.NewStyle().Foreground(pageField),
		segCheck:       lipgloss.NewStyle().Foreground(pageText),
		segMedia:       lipgloss.NewStyle().Foreground(pageDim),
		segCode:        lipgloss.NewStyle().Foreground(pageCode),
		segUnsupported: lipgloss.NewStyle().Foreground(pageDim),
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
		segCodeKey:     on(pageKey),
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
	out := panelChromeTone(innerW, body, title, tone)
	lines := strings.Split(out, "\n")
	bs := lipgloss.NewStyle().Foreground(toneColor(tone))
	lw := dispW(legend)
	if lw+4 > innerW {
		return out
	}
	lines[len(lines)-1] = bs.Render("╰"+strings.Repeat("─", innerW-lw-1)) + legend + bs.Render("─╯")
	return strings.Join(lines, "\n")
}

// pagetabRow is the pagetab: the rule under the URL, and on it the page's
// chrome as capsules (ux.md §A.0.K, 2026-09-22) — one per kind, in a
// fixed order: skip, header, nav, search, sidebar, footer, each dialog,
// other; the menu glyph, the kind's word, +N for what is behind it —
// and a "+N" for the ones the width left out. Off the page, so the page
// starts at its content. The hand comes up here on Esc, or on k from
// the page's top, walks the capsules on h/l, goes back down on j or
// Esc; Enter on one is its list, Space its menu (tab.moveItem). A bare
// rule when the page has no chrome.
func (m AppModel) pagetabRow(t *tab, innerW int) string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	slots := t.pagetabSlots()
	if len(slots) == 0 {
		return dim.Render(strings.Repeat("─", innerW))
	}
	labels := make([]string, 0, len(slots))
	active := -1
	for i, s := range slots {
		if s == pagetabMore {
			labels = append(labels, "+"+itoa(len(t.lay.pagetab)-t.lay.fit))
		} else {
			labels = append(labels, t.lay.pagetab[s].label)
		}
		if s == t.pagetabSlot() && !m.sel.on {
			active = i
		}
	}
	for len(labels) > 1 && chainW(withLead(labels)) > innerW-1 {
		labels = labels[:len(labels)-1]
		if active >= len(labels) {
			active = len(labels) - 1
		}
	}
	labels = withLead(labels)
	chain := pagetabChain(labels, active, m.focus == panelPage, t.loading)
	return chain + dim.Render(strings.Repeat("─", max(0, innerW-chainW(labels))))
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
	lit, unlit := headerColor, lipgloss.Color(baseHex)
	ink := headerColor
	switch {
	case dimmed:
		lit, ink = borderDim, dimColor
	case !focused:
		lit = borderDim
	}
	fill := func(i int) lipgloss.Color {
		if i == active {
			return lit
		}
		return unlit
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(fill(0)).Render(capLeft))
	for i, lab := range labels {
		if i > 0 {
			div, fg, bg := divider(fill(i-1), fill(i))
			b.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bg).Render(div))
		}
		seg := " " + lab + " "
		if i == active {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).
				Background(lit).Bold(true).Render(seg))
			continue
		}
		b.WriteString(lipgloss.NewStyle().Foreground(ink).Background(unlit).Render(seg))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(fill(len(labels) - 1)).Render(capRight))
	return b.String()
}
