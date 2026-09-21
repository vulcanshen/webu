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
	// ref is the row's index in what backs it — m.bookmarks, m.history,
	// m.dls, the settings table — so a delete or a move lands on the right
	// one whatever order the screen shows them in.
	ref int
	// folder is the bookmark's folder (a path, "dev/go"); isFolder marks a
	// folder's own row, which the cursor can land on the way filu's stops
	// on a directory; depth is how far the row is indented.
	folder   string
	isFolder bool
	depth    int
	// folded: a folder row drawn shut; count is what it hides.
	folded bool
	count  int
	// toggle marks a settings row that is a switch: Enter flips it.
	toggle bool
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

// cursorTo puts the cursor on the row backed by ref, if it is visible.
func (m *listPanel) cursorTo(ref int) {
	for p, i := range m.visible() {
		if e := m.entries[i]; !e.isFolder && e.ref == ref {
			m.cursor = p
			m.top = scrollTo(m.top, m.cursor, m.rows())
			return
		}
	}
}

// cursorToFolder puts the cursor on a folder's own row.
func (m *listPanel) cursorToFolder(path string) {
	for p, i := range m.visible() {
		if e := m.entries[i]; e.isFolder && e.folder == path {
			m.cursor = p
			m.top = scrollTo(m.top, m.cursor, m.rows())
			return
		}
	}
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

// columns names the two columns, for the header row over the list: it is
// what keeps the first row off the title chip (revised 2026-09-21).
func (m listPanel) columns() (string, string) {
	switch m.kind {
	case listHistory:
		return "Title", "When  URL"
	case listDownloads:
		return "File", "Progress, or where it landed"
	case listSettings:
		return "Setting", "Value"
	}
	return "Title", "URL"
}

// visible is the indexes into entries that pass the filter. A filter
// flattens the tree: only matches show, and a folder row is not a match.
func (m listPanel) visible() []int {
	q := strings.ToLower(strings.TrimSpace(m.filter))
	out := make([]int, 0, len(m.entries))
	for i, e := range m.entries {
		if q == "" {
			out = append(out, i)
			continue
		}
		if e.isFolder {
			continue
		}
		if strings.Contains(strings.ToLower(e.title), q) || strings.Contains(strings.ToLower(e.url), q) ||
			strings.Contains(strings.ToLower(e.folder), q) {
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
// footer, the two borders and the column header, and the filter line when
// there is one.
func (m listPanel) rows() int {
	n := m.screenH - 6
	if m.typing || m.filter != "" {
		n--
	}
	return max(1, n)
}

// update handles one key and returns the key that names an action —
// "enter", "o", "m", "x", "y", "A", "F", "C" — or "": the app runs it,
// since the panel does not know what a URL is for, and the Space menu
// hands the same keys to the same place. Esc is the app's (§4.3); while
// typing, Esc clearing the filter is answered by escTyping.
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
	case "m", "a", "A":
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
		if e, _, ok := m.current(); ok && e.toggle {
			return []menuItem{{label: "Toggle", key: "enter", hint: "switch it on or off; saved at once"}}
		}
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
	// Bookmarks: what the cursor is on decides the item half — a folder
	// row folds on Enter, a bookmark opens (revised 2026-09-21).
	items := []menuItem{{header: true, label: "item operation"}}
	e, _, ok := m.current()
	switch {
	case ok && e.isFolder && e.folded:
		items = append(items, menuItem{label: "Expand", key: "enter", hint: "show what is inside"})
	case ok && e.isFolder:
		items = append(items, menuItem{label: "Collapse", key: "enter", hint: "one row, out of the way"})
	default:
		items = append(items, menuItem{label: "Open in new tab", key: "enter", hint: "and switch to it"})
	}
	items = append(items, menuItem{label: "Add", key: "a", hint: "a bookmark here: its URL, then its title"})
	if ok && e.isFolder {
		items = append(items, menuItem{label: "Delete", key: "x", hint: "this folder, once it is empty"})
	} else {
		items = append(items,
			menuItem{label: "Move", key: "m", hint: "into a folder, or out to the top"},
			menuItem{label: "Delete", key: "x", hint: "this bookmark"},
			menuItem{label: "Yank url", key: "y", hint: "to the clipboard"})
	}
	return append(items,
		menuItem{separator: true},
		menuItem{header: true, label: "panel operation"},
		menuItem{label: "Add folder", key: "A", hint: "here; a path like a/b/c makes each level"},
		menuItem{label: "Filter", key: "/", hint: "type to narrow the list"})
}

// hintPairs is the border legend: bright the key, dim what it does (§4.4).
func (m listPanel) hintPairs() [][2]string {
	if m.typing {
		return [][2]string{{"Enter", "done"}, {"Esc", "clear"}}
	}
	var pairs [][2]string
	switch m.kind {
	case listSettings:
		pairs = [][2]string{{"Enter", "edit"}}
		if e, _, ok := m.current(); ok && e.toggle {
			pairs = [][2]string{{"Enter", "toggle"}}
		}
	case listDownloads:
		pairs = [][2]string{{"Enter", "open file"}, {"o", "source in new tab"}, {"x", "remove"},
			{"y", "yank path"}, {"C", "clear done"}, {"/", "filter"}}
	case listHistory:
		pairs = [][2]string{{"Enter", "open in new tab"}, {"x", "delete"}, {"y", "yank url"}, {"C", "clear"}, {"/", "filter"}}
	default:
		open := "open in new tab"
		if e, _, ok := m.current(); ok && e.isFolder {
			open = "expand / collapse"
		}
		pairs = [][2]string{{"Enter", open}, {"a", "add"}, {"m", "move"}, {"x", "delete"}, {"y", "yank url"},
			{"A", "add folder"}, {"/", "filter"}}
	}
	return append(pairs, [2]string{"Esc", "web"})
}

// fitLegend keeps as many pairs as fit in w cells, dropped from the right
// the way the footer drops them (chrome.go keyLegend).
func fitLegend(pairs [][2]string, w int) string {
	for n := len(pairs); n > 0; n-- {
		if s := hintLegend(pairs[:n]); dispW(s) <= w {
			return s
		}
	}
	return ""
}

// panel draws the screen: one framed panel, the keyboard's (focus blue),
// a column header over the rows and the legend in the bottom border.
func (m listPanel) panel(outerW, outerH int) string {
	glyph, text := m.title()
	innerW, innerH := outerW-2, outerH-2
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	edit := lipgloss.NewStyle().Foreground(editColor)
	frame := func(rows []string) string {
		return panelFrameLegend(innerW, fitLines(rows, innerW, innerH), " "+glyph+" "+text+" ",
			fitLegend(m.hintPairs(), innerW-4), toneFocus)
	}

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
	h1, h2 := m.columns()
	titleW := dispW(h1)
	for _, i := range vis {
		e := m.entries[i]
		if e.isFolder {
			continue // a folder row spans; it does not set the column
		}
		titleW = max(titleW, dispW(oneLine(nameOr(e.title, e.url)))+2*e.depth)
	}
	titleW = min(titleW, innerW/2)
	rows = append(rows, dim.Render(padRight(" "+padRight(h1, titleW)+"  "+h2, innerW)))
	if len(vis) == 0 {
		fact, hint := m.emptyState()
		if m.filter != "" {
			fact, hint = "nothing matches", emptyHint("Press Esc to clear the filter", "Esc")
		}
		rows = append(rows, emptyBody(innerW, innerH-len(rows), fact, hint)...)
		return frame(rows)
	}
	end := min(len(vis), m.top+m.rows())
	for r := m.top; r < end; r++ {
		e := m.entries[vis[r]]
		if e.isFolder {
			tri, tail := "▾", ""
			if e.folded {
				tri = "▸"
				if e.count > 0 {
					tail = " · " + plural(e.count, "bookmark")
				}
			}
			line := padRight(" "+strings.Repeat("  ", e.depth)+tri+" "+glyphFolder+" "+oneLine(e.title)+tail, innerW)
			if r == m.cursor {
				rows = append(rows, cur.Render(line))
			} else {
				rows = append(rows, txt.Render(line))
			}
			continue
		}
		name := strings.Repeat("  ", e.depth) + oneLine(nameOr(e.title, e.url))
		title := padRight(name, titleW)
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
	return frame(rows)
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
	return "no bookmarks yet", emptyHint("Press a to add one, A for a folder, or W for the web", "a", "A", "W")
}
