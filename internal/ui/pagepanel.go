package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Panel [2]: the page (ui.md §2). One job, always the page: the first row is
// the URL, the second a rule, and the rest is the laid-out IR with the
// cursor on one item.

// pageHeaderRows is the URL row and the rule under it.
const pageHeaderRows = 2

// pageBody draws panel [2]'s inside at innerW × innerH.
func (m AppModel) pageBody(innerW, innerH int) []string {
	t := m.shownTab()
	if t == nil {
		return emptyBody(innerW, innerH, "no page",
			emptyHint("Press L to enter a location, or T for a new tab", "L", "T"))
	}
	dim := lipgloss.NewStyle().Foreground(dimColor)
	out := make([]string, 0, innerH)
	// The first row is the URL, behind a glyph that says what state the
	// page is in: at rest, or on its way (the family's live glyph) — or,
	// while a search is being typed, the query and its count (ux.md
	// §1.1): the page under it does not move.
	if st := m.sel.status(); m.sel.on && st != "" {
		out = append(out, lipgloss.NewStyle().Foreground(selectColor).Render(padRight(" "+st, innerW)))
	} else {
		icon := glyphWeb
		if t.loading {
			icon = glyphLive
		}
		url := lipgloss.NewStyle().Foreground(urlColor).Render(fitURL(t.url, innerW-3))
		out = append(out, dim.Render(" "+icon+" ")+url+strings.Repeat(" ", max(0, innerW-3-dispW(fitURL(t.url, innerW-3)))))
	}
	out = append(out, dim.Render(strings.Repeat("─", innerW)))
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
	codePad := lipgloss.NewStyle().Background(codeBg)
	if t.loading {
		for k := range codeStyles {
			codeStyles[k] = lipgloss.NewStyle().Foreground(dimColor).Background(codeBg)
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
			case s.item >= 0 && s.item == t.cursor && m.focus == panelPage:
				b.WriteString(cur.Render(text))
			case s.item >= 0 && s.item == t.cursor:
				b.WriteString(curOff.Render(text))
			case row.code:
				style, ok := codeStyles[s.kind]
				if !ok {
					style = codeStyles[segCode]
				}
				b.WriteString(style.Render(text))
			default:
				b.WriteString(styles[s.kind].Render(text))
			}
		}
		if row.code {
			span := min(innerW, max(used, t.textWidth()))
			b.WriteString(codePad.Render(strings.Repeat(" ", max(0, span-used))))
			b.WriteString(strings.Repeat(" ", max(0, innerW-span)))
		} else {
			b.WriteString(strings.Repeat(" ", max(0, innerW-used)))
		}
		out = append(out, b.String())
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
}

// segStyles is the colour of each kind of segment: one table, shared by the
// ordinary page and selection mode so the two cannot drift.
func segStyles() map[segKind]lipgloss.Style {
	return map[segKind]lipgloss.Style{
		segPlain:       lipgloss.NewStyle().Foreground(textColor),
		segDim:         lipgloss.NewStyle().Foreground(dimColor),
		segHeading:     lipgloss.NewStyle().Foreground(textColor).Bold(true),
		segLink:        lipgloss.NewStyle().Foreground(linkColor).Underline(true),
		segButton:      lipgloss.NewStyle().Foreground(textColor).Bold(true),
		segInput:       lipgloss.NewStyle().Foreground(editColor),
		segCheck:       lipgloss.NewStyle().Foreground(textColor),
		segMedia:       lipgloss.NewStyle().Foreground(dimColor),
		segCode:        lipgloss.NewStyle().Foreground(codeColor),
		segUnsupported: lipgloss.NewStyle().Foreground(dimColor),
		segLandmark:    lipgloss.NewStyle().Foreground(dimColor).Bold(true),
		segTableHeader: lipgloss.NewStyle().Foreground(headerColor).Bold(true),
	}
}

// codeStyles is the syntax palette inside a code block, every entry on the
// code ground. Keys share mauve with table headers — both are the name of
// a value; strings are the code colour; the rest stay out of the bands the
// app reserves (green for the shown tab, yellow for visual mode, red and
// peach for the override).
func codeStyles() map[segKind]lipgloss.Style {
	on := func(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c).Background(codeBg) }
	return map[segKind]lipgloss.Style{
		segCode:        on(textColor),
		segCodeKey:     on(headerColor),
		segCodeString:  on(codeColor),
		segCodeNumber:  on(lipgloss.Color("#f2cdcd")), // flamingo
		segCodeConst:   on(lipgloss.Color("#89dceb")), // sky: true / false / null
		segCodeKeyword: on(lipgloss.Color("#89dceb")),
		segCodeComment: on(dimColor).Italic(true),
		segCodePunct:   on(lipgloss.Color("#9399b2")), // overlay2
		segCodeHeading: on(textColor).Bold(true),
		segCodeStrong:  on(textColor).Bold(true),
		segCodeEmph:    on(textColor).Italic(true),
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
