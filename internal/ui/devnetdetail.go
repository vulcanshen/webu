package ui

import (
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/vulcanshen/webu/internal/page"
)

// The Network detail (ui.md §3.2): one request's headers and body, a
// viewport stacked over the DevTools popup. Its own file and animator
// (kbu §6.0.4); the DevTools popup composites it inside its own view.

// bodyMsg is the response body landing.
type bodyMsg struct {
	tabID int
	id    string
	body  string
	err   error
}

type devDetailPopup struct {
	anim    popupAnimator
	entry   page.NetEntry
	title   string
	lines   []string
	top     int
	layer   int
	screenW int
	screenH int
}

// innerW is the box's inside; what showText wraps to.
func (m devDetailPopup) innerW() int { return popupInnerW(m.screenW, m.screenW-8) }

// showText opens on lines of plain text — a console entry's whole message
// — wrapped to the box, since a line the list had to cut is the reason
// the popup exists.
func (m *devDetailPopup) showText(title string, head []string, text string, layer int) tea.Cmd {
	m.title, m.layer, m.top = title, layer, 0
	m.entry = page.NetEntry{}
	m.lines = append([]string(nil), head...)
	width := max(10, m.innerW()-3)
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(para) == "" {
			m.lines = append(m.lines, "")
			continue
		}
		// A line's indent is kept on every row it wraps to: a property
		// listing stays a listing.
		body := strings.TrimLeft(para, " ")
		indent := para[:len(para)-len(body)]
		for _, line := range wrapWords(body, max(4, width-dispW(indent))) {
			m.lines = append(m.lines, "  "+indent+line)
		}
	}
	return m.anim.open()
}

// wrapWords breaks text at spaces to at most width cells per line; a word
// longer than the line is cut where it must be.
func wrapWords(text string, width int) []string {
	var out []string
	line, used := "", 0
	for _, w := range strings.Fields(text) {
		for dispW(w) > width {
			if used > 0 {
				out = append(out, line)
				line, used = "", 0
			}
			head := truncateNoEllipsis(w, width)
			out = append(out, head)
			w = strings.TrimPrefix(w, head)
		}
		if w == "" {
			continue
		}
		if used > 0 && used+1+dispW(w) > width {
			out = append(out, line)
			line, used = "", 0
		}
		if used > 0 {
			line += " "
			used++
		}
		line += w
		used += dispW(w)
	}
	if used > 0 || len(out) == 0 {
		out = append(out, line)
	}
	return out
}

func newDevDetailPopup() devDetailPopup {
	return devDetailPopup{anim: newPopupAnimator("devtools_detail")}
}

func (m devDetailPopup) isActive() bool    { return m.anim.isActive() }
func (m *devDetailPopup) close() tea.Cmd   { return m.anim.close() }
func (m *devDetailPopup) setSize(w, h int) { m.screenW, m.screenH = w, h }

// show opens on the headers at once; the body arrives by bodyMsg.
func (m *devDetailPopup) show(e page.NetEntry, layer int) tea.Cmd {
	m.entry, m.layer, m.top = e, layer, 0
	m.title = oneLine(fitURL(e.URL, m.innerW()-12))
	m.lines = m.build("(loading body…)")
	return m.anim.open()
}

func (m *devDetailPopup) setBody(body string, err error) {
	switch text, ok := textBody(body, m.entry.Mime); {
	case err != nil:
		body = "(no body: " + err.Error() + ")"
	case !ok:
		// An image, a font, a zip: bytes that are not text are not shown
		// as text — they would be a screen of replacement glyphs and
		// control characters that shear the frame.
		body = "(binary: " + humanBytes(float64(len(body))) + " of " + nameOr(m.entry.Mime, "unknown type") + ")"
	default:
		body = text
	}
	m.lines = m.build(body)
}

// textBody says whether a body is text a terminal can show, and returns
// it cleaned: valid UTF-8, no control characters but newline and tab.
// The mime type decides first; a body that claims to be text but is not
// valid UTF-8 or carries a NUL is treated as binary all the same.
//
// Nothing is cut: the viewer draws only the rows on screen, so a body of
// a hundred thousand lines scrolls at the cost of ten.
func textBody(body, mime string) (string, bool) {
	if !textMime(mime) || strings.ContainsRune(body, 0) || !utf8.ValidString(body) {
		return "", false
	}
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return -1
	}, body), true
}

func textMime(mime string) bool {
	m := strings.ToLower(mime)
	if m == "" || strings.HasPrefix(m, "text/") {
		return true
	}
	for _, s := range []string{"json", "javascript", "ecmascript", "xml", "svg", "x-www-form-urlencoded", "graphql", "yaml", "csv"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

func (m devDetailPopup) build(body string) []string {
	e := m.entry
	lines := []string{e.Method + " " + e.URL, ""}
	if e.Status > 0 {
		lines = append(lines, "status   "+itoa(int(e.Status))+"  "+e.Mime)
	}
	if e.Error != "" {
		lines = append(lines, "error    "+e.Error)
	}
	if e.Done {
		lines = append(lines, "size     "+humanBytes(e.Size)+"  in "+itoa(int(e.Duration.Milliseconds()))+"ms")
	}
	lines = append(lines, "", "request headers")
	lines = append(lines, headerLines(e.ReqHdr)...)
	lines = append(lines, "", "response headers")
	lines = append(lines, headerLines(e.RespHdr)...)
	lines = append(lines, "", "body")
	for _, l := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		lines = append(lines, "  "+strings.ReplaceAll(l, "\t", "    "))
	}
	return lines
}

// headerValue keeps a header's value on one line: a set-cookie with a
// newline in it would be two rows, the second of them unlabelled.
func headerValue(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return ' '
	}, strings.ToValidUTF8(v, "�"))
}

func headerLines(h map[string]string) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, "  "+k+": "+headerValue(h[k]))
	}
	if len(out) == 0 {
		out = append(out, "  (none)")
	}
	return out
}

func (m devDetailPopup) visible() int { return max(1, m.screenH-6) }

func (m *devDetailPopup) update(msg tea.KeyMsg) {
	if !m.anim.isInteractive() {
		return
	}
	m.top = moveScroll(m.top, max(0, len(m.lines)-m.visible()), msg.String(), m.visible())
}

func (m devDetailPopup) view() string {
	innerW := m.innerW()
	txt := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimColor)
	vis := m.visible()
	end := min(len(m.lines), m.top+vis)
	rows := make([]string, 0, vis)
	for _, l := range m.lines[m.top:end] {
		style := txt
		if !strings.HasPrefix(l, "  ") && (l == "request headers" || l == "response headers" || l == "body") {
			style = dim
		}
		rows = append(rows, style.Render(padRight(" "+l, innerW)))
	}
	pairs := [][2]string{{"j/k", "scroll"}, {"u/d", "half page"}, {"Esc", "close"}}
	return drawPopupBoxPad(popupLayerColor(m.layer), " "+glyphDevTools+" "+m.title+" ",
		hintLegend(pairs), animRows(m.anim, rows), innerW, false)
}

// over composites the detail on top of the DevTools frame.
func (m devDetailPopup) over(frame string) string {
	return overlay.Composite(m.view(), frame, overlay.Center, overlay.Center, 0, 0)
}
