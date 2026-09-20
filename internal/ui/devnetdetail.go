package ui

import (
	"sort"
	"strings"

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
	lines   []string
	top     int
	layer   int
	screenW int
	screenH int
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
	m.lines = m.build("(loading body…)")
	return m.anim.open()
}

func (m *devDetailPopup) setBody(body string, err error) {
	if err != nil {
		body = "(no body: " + err.Error() + ")"
	}
	m.lines = m.build(body)
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

func headerLines(h map[string]string) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, "  "+k+": "+h[k])
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
	innerW := popupInnerW(m.screenW, m.screenW-8)
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
	return drawPopupBoxPad(popupLayerColor(m.layer), " "+glyphDevTools+" "+oneLine(fitURL(m.entry.URL, innerW-12))+" ",
		hintLegend(pairs), animRows(m.anim, rows), innerW, false)
}

// over composites the detail on top of the DevTools frame.
func (m devDetailPopup) over(frame string) string {
	return overlay.Composite(m.view(), frame, overlay.Center, overlay.Center, 0, 0)
}
