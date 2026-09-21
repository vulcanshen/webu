package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// listPanel is the surface behind each header chip after [W]eb: Bookmarks,
// History, Downloads and Settings (ui.md §2). One model, four contents — a
// row is a title and a line beside it, the operations differ by kind and
// the border hint says which. Typing `/` filters in place.
//
// They were popups until 2026-09-21. With the header being sshu's tab
// strip, each chip is a screen of its own, drawn where the two web panels
// would be; a popup over the page had nothing to do with the page.
type listKind int

// In the header's order after [W]eb: screen n is kind n-1.
const (
	listBookmarks listKind = iota
	listHistory
	listDownloads
	listSettings
)

type listEntry struct {
	title, url string
	// meta is what the row shows beside the title when it is not the URL:
	// a download's progress, a setting's value.
	meta string
	at   time.Time // History only
}

type listPanel struct {
	kind    listKind
	entries []listEntry
	filter  string
	typing  bool
	cursor  int // index into visible()
	top     int
	screenW int
	screenH int
}

func newListPanel() listPanel { return listPanel{} }

func (m *listPanel) setSize(w, h int) { m.screenW, m.screenH = w, h }

// show puts kind on screen from the top, filter cleared.
func (m *listPanel) show(kind listKind, entries []listEntry) {
	m.kind, m.entries = kind, entries
	m.filter, m.typing = "", false
	m.cursor, m.top = 0, 0
}

// setEntries swaps the content in place (after a delete or add), keeping
// the cursor in range.
func (m *listPanel) setEntries(entries []listEntry) {
	m.entries = entries
	m.cursor = clamp(m.cursor, 0, max(0, len(m.visible())-1))
}

func (m listPanel) title() (glyph, text string) {
	switch m.kind {
	case listHistory:
		return glyphHistory, "History"
	case listDownloads:
		return glyphDownload, "Downloads"
	case listSettings:
		return glyphSettings, "Settings"
	}
	return glyphBookmark, "Bookmarks"
}

// visible is the indexes into entries that pass the filter.
func (m listPanel) visible() []int {
	q := strings.ToLower(strings.TrimSpace(m.filter))
	out := make([]int, 0, len(m.entries))
	for i, e := range m.entries {
		if q == "" || strings.Contains(strings.ToLower(e.title), q) || strings.Contains(strings.ToLower(e.url), q) {
			out = append(out, i)
		}
	}
	return out
}

// current is the entry under the cursor.
func (m listPanel) current() (listEntry, int, bool) {
	vis := m.visible()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return listEntry{}, -1, false
	}
	return m.entries[vis[m.cursor]], vis[m.cursor], true
}

// rows is how many entries fit: the screen less the header, its rule, the
// footer and the two borders, and the filter line when there is one.
func (m listPanel) rows() int {
	n := m.screenH - 5
	if m.typing || m.filter != "" {
		n--
	}
	return max(1, n)
}

// update handles one key and returns the key that names an action —
// "enter", "o", "x", "y", "A", "C" — or "": the app runs it, since the
// panel does not know what a URL is for, and the Space menu hands the
// same keys to the same place. Esc is the app's (§4.3); while typing, Esc
// clearing the filter is answered by escTyping.
func (m *listPanel) update(msg tea.KeyMsg) string {
	k := msg.String()
	if m.typing {
		switch msg.Type {
		case tea.KeyEnter:
			m.typing = false
		case tea.KeyBackspace:
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			m.filter += " "
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
		}
		m.cursor = clamp(m.cursor, 0, max(0, len(m.visible())-1))
		return ""
	}
	if navKeys[k] {
		m.cursor = moveCursor(m.cursor, len(m.visible()), k, m.rows())
		m.top = scrollTo(m.top, m.cursor, m.rows())
		return ""
	}
	if m.kind == listSettings {
		if k == "enter" {
			return "enter"
		}
		return ""
	}
	switch k {
	case "/":
		m.typing = true
	case "enter", "x", "y":
		return k
	case "o":
		if m.kind == listDownloads {
			return k
		}
	case "A":
		if m.kind == listBookmarks {
			return k
		}
	case "C":
		if m.kind != listBookmarks {
			return k
		}
	}
	return ""
}

// escTyping is Esc while a filter is being typed or in force: the filter
// goes, the screen stays (the same two-stage Esc as a search, ux.md §1.1).
func (m *listPanel) escTyping() bool {
	if !m.typing && m.filter == "" {
		return false
	}
	m.typing, m.filter = false, ""
	m.cursor, m.top = 0, 0
	return true
}

// menuItems is the screen's Space menu: the same keys update answers to,
// so the two cannot drift (ux.md §A.1).
func (m listPanel) menuItems() []menuItem {
	switch m.kind {
	case listSettings:
		return []menuItem{{label: "Edit", key: "enter", hint: "change this setting; empty means the default"}}
	case listDownloads:
		return []menuItem{
			{header: true, label: "item operation"},
			{label: "Open file", key: "enter", hint: "with what the desktop opens it with"},
			{label: "Source in new tab", key: "o", hint: "where it came from"},
			{label: "Remove", key: "x", hint: "this row; a download still running is stopped"},
			{label: "Yank path", key: "y", hint: "to the clipboard"},
			{separator: true},
			{header: true, label: "panel operation"},
			{label: "Clear done", key: "C", hint: "the finished and cancelled rows"},
			{label: "Filter", key: "/", hint: "type to narrow the list"},
		}
	case listHistory:
		return []menuItem{
			{header: true, label: "item operation"},
			{label: "Open in new tab", key: "enter", hint: "and switch to it"},
			{label: "Delete", key: "x", hint: "this visit"},
			{label: "Yank url", key: "y", hint: "to the clipboard"},
			{separator: true},
			{header: true, label: "panel operation"},
			{label: "Clear", key: "C", hint: "every visit ever recorded"},
			{label: "Filter", key: "/", hint: "type to narrow the list"},
		}
	}
	return []menuItem{
		{header: true, label: "item operation"},
		{label: "Open in new tab", key: "enter", hint: "and switch to it"},
		{label: "Delete", key: "x", hint: "this bookmark"},
		{label: "Yank url", key: "y", hint: "to the clipboard"},
		{separator: true},
		{header: true, label: "panel operation"},
		{label: "Add this page", key: "A", hint: "the one [W]eb is showing"},
		{label: "Filter", key: "/", hint: "type to narrow the list"},
	}
}

// hint is the border legend: bright the key, dim what it does (§4.4).
func (m listPanel) hint() string {
	if m.typing {
		return hintLegend([][2]string{{"Enter", "done"}, {"Esc", "clear"}})
	}
	var pairs [][2]string
	switch m.kind {
	case listSettings:
		pairs = [][2]string{{"Enter", "edit"}}
	case listDownloads:
		pairs = [][2]string{{"Enter", "open file"}, {"o", "source in new tab"}, {"x", "remove"},
			{"y", "yank path"}, {"C", "clear done"}, {"/", "filter"}}
	case listHistory:
		pairs = [][2]string{{"Enter", "open in new tab"}, {"x", "delete"}, {"y", "yank url"}, {"C", "clear"}, {"/", "filter"}}
	default:
		pairs = [][2]string{{"Enter", "open in new tab"}, {"x", "delete"}, {"y", "yank url"}, {"A", "add this page"}, {"/", "filter"}}
	}
	return hintLegend(append(pairs, [2]string{"Esc", "web"}))
}

// panel draws the screen: one framed panel, the keyboard's (focus blue),
// the legend in its bottom border.
func (m listPanel) panel(outerW, outerH int) string {
	glyph, text := m.title()
	innerW, innerH := outerW-2, outerH-2
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	edit := lipgloss.NewStyle().Foreground(editColor)

	rows := []string{}
	if m.typing || m.filter != "" {
		line := " / " + m.filter
		if m.typing {
			rows = append(rows, edit.Render(padRight(line, innerW-1))+cur.Render(" "))
		} else {
			rows = append(rows, dim.Render(padRight(line, innerW)))
		}
	}
	vis := m.visible()
	if len(vis) == 0 {
		fact, hint := m.emptyState()
		if m.filter != "" {
			fact, hint = "nothing matches", emptyHint("Press Esc to clear the filter", "Esc")
		}
		rows = append(rows, emptyBody(innerW, innerH-len(rows), fact, hint)...)
		return panelFrameLegend(innerW, fitLines(rows, innerW, innerH), " "+glyph+" "+text+" ", m.hint(), toneFocus)
	}
	titleW := 0
	for _, i := range vis {
		titleW = max(titleW, dispW(oneLine(m.entries[i].title)))
	}
	titleW = min(titleW, innerW/2)
	end := min(len(vis), m.top+m.rows())
	for r := m.top; r < end; r++ {
		e := m.entries[vis[r]]
		title := padRight(oneLine(nameOr(e.title, e.url)), titleW)
		meta := e.url
		if e.meta != "" {
			meta = e.meta
		}
		if m.kind == listHistory && !e.at.IsZero() {
			meta = e.at.Local().Format("01-02 15:04") + "  " + e.url
		}
		meta = truncate(meta, innerW-titleW-4)
		if r == m.cursor {
			rows = append(rows, cur.Render(padRight(" "+title+"  "+meta, innerW)))
		} else {
			rows = append(rows, txt.Render(" "+title)+dim.Render(padRight("  "+meta, innerW-titleW-1)))
		}
	}
	return panelFrameLegend(innerW, fitLines(rows, innerW, innerH), " "+glyph+" "+text+" ", m.hint(), toneFocus)
}

// emptyState is the fact and the way on when the list has nothing (sshu
// empty.go's shape).
func (m listPanel) emptyState() (string, []hintWord) {
	switch m.kind {
	case listHistory:
		return "no history yet", emptyHint("Pages you visit are listed here; W is the web", "W")
	case listDownloads:
		return "no downloads yet", emptyHint("Files the page saves are listed here; W is the web", "W")
	}
	return "no bookmarks yet", emptyHint("Press A on a page to add it, or W for the web", "A", "W")
}
