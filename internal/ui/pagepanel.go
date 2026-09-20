package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Panel [3]: the page (ui.md §2). One job, always the page: the first row is
// the URL, the second a rule, and the rest is the laid-out IR with the
// cursor on one item.

// pageHeaderRows is the URL row and the rule under it.
const pageHeaderRows = 2

// pageBody draws panel [3]'s inside at innerW × innerH.
func (m AppModel) pageBody(innerW, innerH int) []string {
	t := m.shownTab()
	if t == nil {
		return emptyBody(innerW, innerH, "no page",
			emptyHint("Press U to go to a URL, or T in [2] for a new tab", "U", "T"))
	}
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	out := make([]string, 0, innerH)
	out = append(out, txt.Render(padRight(" "+fitURL(t.url, innerW-1), innerW)))
	out = append(out, dim.Render(strings.Repeat("─", innerW)))
	rest := innerH - pageHeaderRows
	switch {
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
	styles := map[segKind]lipgloss.Style{
		segPlain:       lipgloss.NewStyle().Foreground(textColor),
		segDim:         lipgloss.NewStyle().Foreground(dimColor),
		segHeading:     lipgloss.NewStyle().Foreground(textColor).Bold(true),
		segLink:        lipgloss.NewStyle().Foreground(linkColor),
		segButton:      lipgloss.NewStyle().Foreground(textColor).Bold(true),
		segInput:       lipgloss.NewStyle().Foreground(editColor),
		segCheck:       lipgloss.NewStyle().Foreground(textColor),
		segMedia:       lipgloss.NewStyle().Foreground(dimColor),
		segCode:        lipgloss.NewStyle().Foreground(peachColor),
		segUnsupported: lipgloss.NewStyle().Foreground(dimColor),
	}
	out := make([]string, 0, innerH)
	end := min(len(t.lay.rows), t.top+innerH)
	for i := t.top; i < end; i++ {
		var b strings.Builder
		used := 0
		for _, s := range t.lay.rows[i].segs {
			text := s.text
			if used+dispW(text) > innerW {
				text = truncate(text, innerW-used)
			}
			if text == "" {
				continue
			}
			used += dispW(text)
			switch {
			case s.item >= 0 && s.item == t.cursor && m.focus == panel3:
				b.WriteString(cur.Render(text))
			case s.item >= 0 && s.item == t.cursor:
				b.WriteString(curOff.Render(text))
			default:
				b.WriteString(styles[s.kind].Render(text))
			}
		}
		b.WriteString(strings.Repeat(" ", max(0, innerW-used)))
		out = append(out, b.String())
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
}

// pageVisible is how many page rows panel [3] shows at the current size.
func (m AppModel) pageVisible() int {
	return max(1, m.panelH()-2-pageHeaderRows)
}

// pageW is panel [3]'s inner width at the current size.
func (m AppModel) pageW() int {
	if m.narrow() {
		return max(1, m.w-2)
	}
	return max(1, m.w-sideW-2)
}

// panelFrame is panelChromeTone with a hint in the bottom border — where
// panel [3] says "loading" and "12 of 40" (ui.md §5).
func panelFrame(innerW int, body []string, title, hint string, tone borderTone) string {
	out := panelChromeTone(innerW, body, title, tone)
	if hint == "" {
		return out
	}
	lines := strings.Split(out, "\n")
	bs := lipgloss.NewStyle().Foreground(toneColor(tone))
	legend := hintLegend([][2]string{{hint, ""}})
	legend = strings.TrimRight(legend, " ")
	lw := dispW(legend)
	if lw+4 > innerW {
		return out
	}
	lines[len(lines)-1] = bs.Render("╰"+strings.Repeat("─", innerW-lw-1)) + legend + bs.Render("─╯")
	return strings.Join(lines, "\n")
}
