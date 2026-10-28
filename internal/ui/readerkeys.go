package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/reader"
)

// A reader tab's keys (docs/reader.md, the keys): j|k|u|d|gg|G scroll the
// page as less does; / finds text, n|N step through what it found; Focus
// mode (f) walks what can be pressed; v is Visual mode; H|L go back and
// forward; o is the address box; w the full page.

// readerFind is what / found on a reader page: every hit, by row and
// cell columns, and the one the search is on.
type readerFind struct {
	query string
	hits  []findHit
	at    int
}

type findHit struct{ row, c0, c1 int }

// readerScroll is j|k|u|d|gg|G on a reader page: the page moves, nothing
// on it is chosen.
func (m *AppModel) readerScroll(t *tab, k string) {
	vis := max(1, m.pageVisible())
	switch k {
	case "j":
		t.top++
	case "k":
		t.top--
	case "d":
		t.top += max(1, vis/2)
	case "u":
		t.top -= max(1, vis/2)
	case "gg":
		t.top = 0
	case "G":
		t.top = len(t.lay.rows)
	}
	t.top = clamp(t.top, 0, max(0, len(t.lay.rows)-vis))
}

// readerFindBox is / on a reader page: what to find, then every hit
// underlined and the first from the top of the screen on.
func (m *AppModel) readerFindBox() tea.Cmd {
	t := m.shownTab()
	old := ""
	if t.find != nil {
		old = t.find.query
	}
	return m.a.Text(terminu.TextInput{Title: "Find", Glyph: glyphSearch, Name: "The text",
		Prompt: "Text to find on the page", Verb: "find", Old: old,
		Done: m.cmdDone(func(v string) tea.Cmd {
			if strings.TrimSpace(v) == "" {
				t.find = nil
				return nil
			}
			t.find = &readerFind{query: v}
			t.find.hits = findHits(t, v)
			m.did("Searched " + tabName(t) + " for " + v)
			if len(t.find.hits) == 0 {
				t.find = nil
				return m.toast("Not on the page: " + v)
			}
			t.find.at = 0
			for i, h := range t.find.hits {
				if h.row >= t.top {
					t.find.at = i
					break
				}
			}
			m.readerShowHit(t)
			return nil
		})})
}

// readerFindStep is n (1) or N (-1): the next or the previous hit, round
// at the ends.
func (m *AppModel) readerFindStep(step int) tea.Cmd {
	t := m.shownTab()
	if t == nil || t.find == nil || len(t.find.hits) == 0 {
		return nil
	}
	n := len(t.find.hits)
	t.find.at = ((t.find.at+step)%n + n) % n
	m.readerShowHit(t)
	return nil
}

// readerShowHit scrolls the hit the search is on into sight.
func (m *AppModel) readerShowHit(t *tab) {
	h := t.find.hits[t.find.at]
	vis := max(1, m.pageVisible())
	if h.row < t.top || h.row >= t.top+vis {
		t.top = clamp(h.row-vis/3, 0, max(0, len(t.lay.rows)-vis))
	}
}

// findHits is every place q is on the page as drawn, by cell column; a
// query in lower case finds any case, one with a capital its own (smart
// case, as Visual mode's).
func findHits(t *tab, q string) []findHit {
	fold := strings.ToLower(q) == q
	needle := q
	if fold {
		needle = strings.ToLower(q)
	}
	var out []findHit
	for i, r := range t.lay.rows {
		text := r.plain()
		hay := text
		if fold {
			hay = strings.ToLower(text)
		}
		for from := 0; from <= len(hay); {
			j := strings.Index(hay[from:], needle)
			if j < 0 {
				break
			}
			at := from + j
			if len(hay) == len(text) { // columns are only sound when folding kept the bytes
				c0 := dispW(text[:at])
				out = append(out, findHit{row: i, c0: c0, c1: c0 + dispW(text[at:at+len(needle)])})
			}
			from = at + max(1, len(needle))
		}
	}
	return out
}

// foundRange is a stretch of a row the search found, by cell column; cur
// says it is the one the search is on.
type foundRange struct {
	c0, c1 int
	cur    bool
}

// findHits is row's hits on t, for drawing; none outside a reader search,
// and none in Visual mode, which draws its own.
func (m *AppModel) findHits(t *tab, row int) []foundRange {
	if t == nil || !t.reader || t.find == nil || m.sel.on {
		return nil
	}
	var out []foundRange
	for i, h := range t.find.hits {
		if h.row == row {
			out = append(out, foundRange{c0: h.c0, c1: h.c1, cur: i == t.find.at})
		}
	}
	return out
}

// writeFound writes a segment that has hits on it: a hit underlined in its
// own colour, the one the search is on as the cursor block (tdp color.1,
// searching). col is the cell the segment starts at; the row stops at
// innerW. It returns the cells written.
func writeFound(b *strings.Builder, style, cur lipgloss.Style, text string, col, innerW int, hits []foundRange) int {
	used := 0
	var run strings.Builder
	var runStyle lipgloss.Style
	flush := func() {
		if run.Len() > 0 {
			b.WriteString(runStyle.Render(run.String()))
			run.Reset()
		}
	}
	for _, r := range text {
		w := dispW(string(r))
		if col+used+w > innerW {
			break
		}
		at := col + used
		s := style
		for _, h := range hits {
			if at >= h.c0 && at < h.c1 {
				s = style.Underline(true)
				if h.cur {
					s = cur
				}
			}
		}
		if run.Len() > 0 && s.String() != runStyle.String() {
			flush()
		}
		runStyle = s
		run.WriteRune(r)
		used += w
	}
	flush()
	return used
}

// focusMode is Focus mode on a reader page: j|k from one thing that can be
// pressed to the next, Enter presses it, Esc leaves (docs/reader.md).
func (m *AppModel) focusMode() *terminu.Mode {
	key := func(label, desc string, item bool, keys ...string) terminu.ModeKey {
		return terminu.ModeKey{Key: label, Keys: keys, Desc: desc, Item: item, Do: m.focusKey}
	}
	return &terminu.Mode{
		Name: "Focus",
		Keys: []terminu.ModeKey{
			{Key: "j|k", Keys: []string{"j", "k", "↓", "↑"}, Desc: "the next, the previous thing to press", NoFooter: true, Do: m.focusKey},
			{Key: "gg|G", Keys: []string{"g", "G"}, Desc: "the first, the last", NoFooter: true, Do: m.focusKey},
			key("Enter", "press", true, "Enter"),
			key("o", "open in a new tab", true, "o"),
			key("y", "copy its address", true, "y"),
		},
		Exit:   "leave",
		Pause:  true,
		OnExit: func() tea.Cmd { m.leaveFocus(); return nil },
	}
}

// enterFocus is f: Focus mode, on the first thing to press on screen.
func (m *AppModel) enterFocus() tea.Cmd {
	t := m.shownTab()
	if t == nil || len(t.lay.items) == 0 {
		return m.toast("Nothing to press on this page")
	}
	t.cursor = len(t.lay.items) - 1
	for i, it := range t.lay.items {
		if it.first >= t.top {
			t.cursor = i
			break
		}
	}
	t.scrollToCursor(m.pageVisible())
	m.pagePanel.EnterMode(m.focusMode())
	return m.focusPage()
}

func (m *AppModel) leaveFocus() {
	if t := m.shownTab(); t != nil {
		t.cursor = -1
	}
	if md := m.pagePanel.InMode(); md != nil && md.Name == "Focus" {
		m.pagePanel.LeaveMode()
	}
	m.sync()
}

// focusKey is one of Focus mode's keys.
func (m *AppModel) focusKey(k string) tea.Cmd {
	defer m.sync()
	t := m.shownTab()
	if t == nil || len(t.lay.items) == 0 {
		return nil
	}
	n := len(t.lay.items)
	switch moveName(k) {
	case "j":
		t.cursor = min(n-1, t.cursor+1)
	case "k":
		t.cursor = max(0, t.cursor-1)
	case "g", "gg":
		t.cursor = 0
	case "G":
		t.cursor = n - 1
	case "Enter":
		cmd := m.readerEnter(t, t.current())
		if t.navigating {
			// Off to another page: Focus mode was for this one.
			m.leaveFocus()
		}
		return cmd
	case "o":
		if cur := t.current(); cur != nil && cur.URL != "" {
			m.did("Opened " + cur.URL + " in a new tab")
			return m.openTab(cur.URL, true)
		}
		return nil
	case "y":
		if cur := t.current(); cur != nil && cur.URL != "" {
			return copyToClipboard(cur.URL)
		}
		return nil
	}
	t.scrollToCursor(m.pageVisible())
	return nil
}

// readerPanelActions are panel [2]'s on a reader tab (docs/reader.md):
// what it does to the whole page.
func (m *AppModel) readerPanelActions(t *tab) []terminu.Action {
	noPage := func() bool { t := m.shownTab(); return t == nil || t.doc == nil }
	panel := func(key, label, desc string, off func() bool, do func() tea.Cmd) terminu.Action {
		return terminu.Action{Key: key, Label: label, Desc: desc, Scope: terminu.PanelScope, Disabled: off, Do: m.do(do)}
	}
	away := func(a terminu.Action) terminu.Action { a.Clear = true; return a }
	// H and L are on the hint: how to go back and forward is not
	// something to look up (user, 2026-10-28).
	hint := func(a terminu.Action, verb string) terminu.Action { a.Hint = verb; return a }
	noFind := func() bool { t := m.shownTab(); return t == nil || t.find == nil }
	return []terminu.Action{
		panel("R", "Reload", "this page", func() bool { return m.shownTab() == nil || m.busy() }, func() tea.Cmd {
			t := m.shownTab()
			m.did("Reloaded " + tabName(t))
			return t.reload()
		}),
		away(panel("T", "Tab", "a new one, at a URL", nil, m.newTabBox)),
		hint(panel("H", "Back", "in this tab", func() bool { t := m.shownTab(); return t == nil || !t.canBack || m.busy() }, func() tea.Cmd {
			m.did("Went back in " + tabName(m.shownTab()))
			return m.shownTab().readerGo(-1)
		}), "back"),
		hint(panel("L", "Forward", "in this tab", func() bool { t := m.shownTab(); return t == nil || !t.canForward || m.busy() }, func() tea.Cmd {
			m.did("Went forward in " + tabName(m.shownTab()))
			return m.shownTab().readerGo(1)
		}), "forward"),
		away(panel("o", "Open", "a URL or a search; this page's own is offered", nil, m.locationBox)),
		away(panel("/", "Find", "text on the page; n|N step through it", noPage, m.readerFindBox)),
		panel("n", "Next match", "of the text found", noFind, func() tea.Cmd { return m.readerFindStep(1) }),
		panel("N", "Previous match", "of the text found", noFind, func() tea.Cmd { return m.readerFindStep(-1) }),
		panel("f", "Focus mode", "walk what can be pressed: links, images, fields, buttons", noPage, m.enterFocus),
		panel("v", "Visual mode", "walk the text by character, copy some", noPage, m.enterSelect),
		m.wholeAction(t),
		panel("A", "Add bookmark", "this page", noPage, func() tea.Cmd {
			t := m.shownTab()
			return m.addBookmark(t.title, t.url)
		}),
		panel("Z", "Zoom", "the page alone, or the grid back", nil, func() tea.Cmd {
			m.zoom = !m.zoom
			m.relayoutTabs()
			if m.zoom {
				m.did("Zoomed in on the page")
			} else {
				m.did("Zoomed out to the tabs and the page")
			}
			return nil
		}),
		panel("Y", "Yank page url", "to the clipboard", func() bool { return m.shownTab() == nil }, func() tea.Cmd {
			return copyToClipboard(m.shownTab().url)
		}),
		panel("", "Yank markdown", "the page's content, as markdown", noPage, func() tea.Cmd {
			return copyToClipboard(reader.Text(m.shownTab().lines))
		}),
		panel("C", "Close", "this tab", func() bool { return m.shownTab() == nil }, func() tea.Cmd {
			m.did("Closed the tab " + tabName(m.shownTab()))
			return m.closeTab(m.shown)
		}),
	}
}
