package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// listPopup is the menu behind panel [1]'s three rows: Bookmarks,
// Shortcuts and History (ui.md §3.1). One model, three contents — the
// rows are the same shape (a title and a URL), the operations differ by
// kind and the hint says which. Typing `/` filters in place, the History
// popup's "type to find" (a substring match for now; fuzzy later).
type listKind int

const (
	listBookmarks listKind = iota
	listShortcuts
	listHistory
)

type listEntry struct {
	title, url string
	at         time.Time // History only
}

type listPopup struct {
	anim    popupAnimator
	kind    listKind
	entries []listEntry
	filter  string
	typing  bool
	cursor  int // index into visible()
	top     int
	layer   int
	screenW int
	screenH int
}

func newListPopup() listPopup { return listPopup{anim: newPopupAnimator("listpopup")} }

func (m listPopup) isActive() bool      { return m.anim.isActive() }
func (m listPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *listPopup) close() tea.Cmd     { return m.anim.close() }
func (m *listPopup) setSize(w, h int)   { m.screenW, m.screenH = w, h }

func (m *listPopup) open(kind listKind, entries []listEntry, layer int) tea.Cmd {
	m.kind, m.entries, m.layer = kind, entries, layer
	m.filter, m.typing = "", false
	m.cursor, m.top = 0, 0
	return m.anim.open()
}

// setEntries swaps the content in place (after a delete or add), keeping
// the cursor in range.
func (m *listPopup) setEntries(entries []listEntry) {
	m.entries = entries
	m.cursor = clamp(m.cursor, 0, max(0, len(m.visible())-1))
}

func (m listPopup) title() (glyph, text string) {
	switch m.kind {
	case listShortcuts:
		return glyphShortcut, "Shortcuts"
	case listHistory:
		return glyphHistory, "History"
	}
	return glyphBookmark, "Bookmarks"
}

// visible is the indexes into entries that pass the filter.
func (m listPopup) visible() []int {
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
func (m listPopup) current() (listEntry, int, bool) {
	vis := m.visible()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return listEntry{}, -1, false
	}
	return m.entries[vis[m.cursor]], vis[m.cursor], true
}

// rows is how many entries fit.
func (m listPopup) rows() int { return max(1, min(m.screenH-8, 20)) }

// update handles one key. It returns the action committed: "open",
// "newtab", "delete", "add", "clear", or "" — the app runs it, since the
// popup does not know what a URL is for. Esc is the app's (§4.3); while
// typing, Esc clearing the filter is answered by escTyping.
func (m *listPopup) update(msg tea.KeyMsg) string {
	if !m.anim.isInteractive() {
		return ""
	}
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
	switch k {
	case "/":
		m.typing = true
	case "enter":
		return "open"
	case "o":
		return "newtab"
	case "x":
		return "delete"
	case "A":
		if m.kind != listHistory {
			return "add"
		}
	case "C":
		if m.kind == listHistory {
			return "clear"
		}
	}
	return ""
}

// escTyping is Esc while a filter is being typed: the filter goes, the
// popup stays (the same two-stage Esc as a search, ux.md §1.1).
func (m *listPopup) escTyping() bool {
	if !m.typing && m.filter == "" {
		return false
	}
	m.typing, m.filter = false, ""
	m.cursor, m.top = 0, 0
	return true
}

func (m listPopup) view() string {
	glyph, text := m.title()
	innerW := popupInnerW(m.screenW, 72)
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
		fact := "nothing here yet"
		if m.filter != "" {
			fact = "nothing matches"
		}
		rows = append(rows, dim.Render(padRight("  "+fact, innerW)))
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

	var pairs [][2]string
	if m.typing {
		pairs = [][2]string{{"Enter", "done"}, {"Esc", "clear"}}
	} else {
		pairs = [][2]string{{"Enter", "open"}, {"o", "new tab"}, {"x", "delete"}}
		if m.kind == listHistory {
			pairs = append(pairs, [2]string{"C", "clear"})
		} else {
			pairs = append(pairs, [2]string{"A", "add this page"})
		}
		pairs = append(pairs, [2]string{"/", "filter"}, [2]string{"Esc", "close"})
	}
	return drawPopupBox(popupLayerColor(m.layer), " "+glyph+" "+text+" ",
		hintLegend(pairs), animRows(m.anim, capRows(rows, m.screenH)), innerW)
}
