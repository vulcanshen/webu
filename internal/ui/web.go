package ui

import (
	"sort"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// The web screen (ui.md §1): [1] the tabs, [2] the page. [2] is [1]'s
// details (components/layout/panel): it shows the tab under [1]'s cursor,
// and Enter on [1] takes the keyboard there (Rules K3).

func (m *AppModel) buildWeb() {
	m.tabsList = &terminu.List{
		Columns:   []terminu.Column{{Title: "Tab", Flex: true}},
		EmptyFact: "No tabs",
		EmptyHint: "Press [T] to open one, or [Space] to see what you can do here",
		OnMove:    m.tabMoved,
	}
	m.page = &pageBody{m: m}
	m.pagePanel = &terminu.Panel{Label: "Page", Body: m.page, OnTab: m.partTab}
	m.tabsPanel = &terminu.Panel{Label: "Tabs", Body: m.tabsList, Details: m.pagePanel}
	m.tabsPanel.Actions = m.tabActions()
	m.syncPage()
	m.web = m.a.AddScreen(&terminu.Screen{
		Name: "Web", Key: "W", Desc: "the tabs and the page",
		Panels:      []*terminu.Panel{m.tabsPanel, m.pagePanel},
		Layout:      m.webLayout,
		NarrowBelow: narrowW,
	})
}

// webLayout puts [1] at its fixed width and gives [2] the rest; zoomed,
// [2] takes the whole width (ui.md §1.2). The zoom is the page's: with the
// focus on [1] the grid is drawn, so the focused panel is on screen (tdp
// L5).
func (m *AppModel) webLayout(w, h int) []terminu.Rect {
	if m.zoom {
		// Zoomed, the focused panel takes the whole width, the way a narrow
		// screen draws it (layout/screen): Tab shows the other one.
		if m.web.Focused() == m.pagePanel {
			return []terminu.Rect{{X: 0, Y: 0, W: 0, H: h}, {X: 0, Y: 0, W: w, H: h}}
		}
		return []terminu.Rect{{X: 0, Y: 0, W: w, H: h}, {X: w, Y: 0, W: 0, H: h}}
	}
	side := min(sideW, w)
	return []terminu.Rect{{X: 0, Y: 0, W: side, H: h}, {X: side, Y: 0, W: w - side, H: h}}
}

// onWeb reports whether the web screen is the one up.
func (m *AppModel) onWeb() bool { return m.a.Screen() == m.web }

// pageFocused reports whether the keyboard is on panel [2].
func (m *AppModel) pageFocused() bool { return m.onWeb() && m.web.Focused() == m.pagePanel }

// narrow reports whether the web screen draws one panel at a time.
func (m *AppModel) narrow() bool {
	w, _ := m.a.Size()
	return w < narrowW
}

// pageW is panel [2]'s inner width at the current size.
func (m *AppModel) pageW() int {
	w, _ := m.a.Size()
	if m.narrow() || m.zoom {
		return max(1, w-2)
	}
	return max(1, w-sideW-2)
}

// pageVisible is how many page rows panel [2] shows at the current size:
// the terminal less the chip row, its rule, the footer, the frame and the
// rows above the page.
func (m *AppModel) pageVisible() int {
	_, h := m.a.Size()
	t := m.shownTab()
	if t != nil && t.popupNode() != nil {
		return m.popupVisible()
	}
	header := 0
	if t != nil && t.listing() {
		header = 1 // the section list's header
	}
	return max(1, h-3-2-m.headRows(t)-header)
}

// popupVisible is how many of the page popup's rows the float shows.
func (m *AppModel) popupVisible() int {
	_, h := m.a.Size()
	return max(3, h-3-10)
}

// goWeb brings the web screen up, from a list that opened a page in it.
//
// TODO(lib WB6): terminu-go has no call that switches screens; the web
// screen's own hotkey is sent as a key instead.
func (m *AppModel) goWeb() tea.Cmd {
	if m.onWeb() {
		return nil
	}
	return pressKey("W")
}

// focusPage takes the keyboard to panel [2], where a page that was just
// opened is.
//
// TODO(lib WB6): terminu-go has no call that moves the focus; the panel's
// digit is sent as a key instead.
func (m *AppModel) focusPage() tea.Cmd {
	if m.pageFocused() || m.web.Focused() == nil {
		return nil
	}
	return pressKey("2")
}

// pressKey is a key as if the user had pressed it.
func pressKey(k string) tea.Cmd {
	return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)} }
}

// ------------------------------------------------------------- panel [1]

// tabMoved is [1]'s cursor reaching a tab: [2] shows it (details).
func (m *AppModel) tabMoved(r terminu.Row) tea.Cmd {
	id, err := strconv.Atoi(r.ID)
	if err != nil {
		return nil
	}
	i, _ := m.tabByID(id)
	if i < 0 || i == m.shown {
		return nil
	}
	cmd := m.showTab(i)
	m.syncPage()
	return cmd
}

// cursorTab is the tab under [1]'s cursor.
func (m *AppModel) cursorTab() (int, *tab) {
	r, ok := m.tabsList.Current()
	if !ok {
		return -1, nil
	}
	id, err := strconv.Atoi(r.ID)
	if err != nil {
		return -1, nil
	}
	return m.tabByID(id)
}

// syncTabs lists the tabs in [1]: green is the one [2] shows, a live glyph
// one still loading, grey one restored but not loaded yet (ui.md §2). Tabs
// on the same URL are told apart by a number in the order they were opened.
func (m *AppModel) syncTabs() {
	seq := map[*tab]int{}
	byURL := map[string][]*tab{}
	for _, t := range m.tabs {
		if t.url != "" {
			byURL[t.url] = append(byURL[t.url], t)
		}
	}
	for _, same := range byURL {
		if len(same) < 2 {
			continue
		}
		sort.Slice(same, func(i, j int) bool { return same[i].id < same[j].id })
		for i, t := range same {
			seq[t] = i + 1
		}
	}
	rows := make([]terminu.Row, 0, len(m.tabs))
	for i, t := range m.tabs {
		title := t.title
		if title == "" {
			title = t.url
		}
		if title == "" {
			title = "new tab"
		}
		if n := seq[t]; n > 0 {
			title = "(" + itoa(n) + ") " + title
		}
		mark := " "
		if t.working() {
			// Still filling in counts: a tab in the background that says
			// nothing looks the same as one that has finished.
			mark = glyphLive
		}
		cell := mark + " " + oneLine(title)
		switch {
		case i == m.shown:
			cell = terminu.Fg(terminu.Green).Render(cell)
		case t.pending:
			cell = terminu.Fg(terminu.Overlay0).Render(cell)
		}
		rows = append(rows, terminu.Row{ID: strconv.Itoa(t.id), Cells: []string{cell}})
	}
	m.tabsList.SetRows(rows)
	if t := m.shownTab(); t != nil {
		m.tabsList.Select(strconv.Itoa(t.id))
	}
}

// tabActions are panel [1]'s (ux.md §A.1).
func (m *AppModel) tabActions() []terminu.Action {
	none := func() bool { return len(m.tabs) == 0 }
	return []terminu.Action{
		{Key: "Enter", Label: "Read", Desc: "this tab's page, in [2]", Scope: terminu.ItemScope, Hint: "read",
			ToDetails: func() bool { return true },
			Do:        m.do(func() tea.Cmd { i, _ := m.cursorTab(); return m.showTab(i) })},
		// c, the page panel's C in lower case.
		{Key: "c", Label: "close", Desc: "this tab", Scope: terminu.ItemScope, Hint: "close",
			Do: m.do(func() tea.Cmd { i, _ := m.cursorTab(); return m.closeTab(i) })},
		// o as on a download's row: this row's URL, in a new tab.
		{Key: "o", Label: "open in new tab", Desc: "the same page again", Scope: terminu.ItemScope,
			Do: m.do(func() tea.Cmd {
				if _, t := m.cursorTab(); t != nil {
					return m.openTab(t.url, true)
				}
				return nil
			})},
		{Key: "r", Label: "reload", Desc: "this tab", Scope: terminu.ItemScope,
			Do: m.do(func() tea.Cmd {
				if _, t := m.cursorTab(); t != nil {
					return t.load(t.url)
				}
				return nil
			})},
		{Key: "y", Label: "yank url", Desc: "to the clipboard", Scope: terminu.ItemScope,
			Do: func() tea.Cmd {
				if _, t := m.cursorTab(); t != nil {
					return copyToClipboard(t.url)
				}
				return nil
			}},
		{Key: "T", Label: "Tab", Desc: "a new one, at a URL", Scope: terminu.PanelScope, Hint: "new tab", Do: m.newTabBox},
		{Key: "X", Label: "Close others", Desc: "every tab but this one", Scope: terminu.PanelScope,
			Disabled: func() bool { return len(m.tabs) < 2 }, Hidden: none,
			Do: m.do(m.closeOthers)},
		{Key: "U", Label: "Undo close", Desc: "reopen the last closed tab", Scope: terminu.PanelScope,
			Disabled: func() bool { return len(m.closed) == 0 },
			Do: m.do(func() tea.Cmd {
				n := len(m.closed)
				if n == 0 {
					return nil
				}
				last := m.closed[n-1]
				m.closed = m.closed[:n-1]
				return m.openTab(last.URL, true)
			})},
	}
}

// closeOthers is [X]: every tab but the one under [1]'s cursor.
func (m *AppModel) closeOthers() tea.Cmd {
	keep, _ := m.cursorTab()
	if keep < 0 {
		return nil
	}
	for i := len(m.tabs) - 1; i >= 0; i-- {
		if i != keep {
			m.remember(m.tabs[i])
			m.tabs[i].close()
			m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
		}
	}
	m.leaveSelect()
	m.shown = 0
	return nil
}

// newTabBox is [T]: a URL, or words to search for, in a new tab.
func (m *AppModel) newTabBox() tea.Cmd {
	return m.a.Text(terminu.TextInput{Title: "New tab", Glyph: glyphSearch, Name: "The address",
		Prompt: "URL, or words to search for", Verb: "open",
		Done: m.cmdDone(func(v string) tea.Cmd {
			if v == "" {
				return nil
			}
			return m.openTab(m.resolveURL(v), true)
		})})
}
