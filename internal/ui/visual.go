package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// Visual mode (ux.md §1) is a mode of panel [2] (Rules K11): the lib
// draws its name on the frame and the frame in the mode's colour, takes
// Esc out of it, lists its keys in ? and the footer, and keeps every other
// key of the panel and the app away while it is on. What the mode does —
// a character cursor over the frozen page, a selection, a search — is
// selectMode's.

// visualMode is the mode as the lib knows it.
func (m *AppModel) visualMode() *terminu.Mode {
	key := func(label, desc string, footer bool, keys ...string) terminu.ModeKey {
		return terminu.ModeKey{Key: label, Keys: keys, Desc: desc, NoFooter: !footer, Do: m.visualKey}
	}
	return &terminu.Mode{
		Name: "Visual",
		Keys: []terminu.ModeKey{
			key("h|j|k|l", "move by character and row", false, "h", "j", "k", "l", "←", "↓", "↑", "→"),
			key("w|b|e", "next word, previous word, end of the word", false, "w", "b", "e"),
			key("0|$", "start, end of the row", false, "0", "$"),
			key("u|d", "half a page up, down", false, "u", "d"),
			key("gg|G", "top, bottom of the page", false, "g", "G"),
			key("v|V", "select by character, by row", true, "v", "V"),
			key("y", "copy the selection, or the row, and leave", true, "y"),
			{Key: "/", Desc: "search the text", Do: func(string) tea.Cmd { return m.visualSearch() }},
			key("n|N", "next match, previous match", true, "n", "N"),
			key("Enter", "press what the cursor is on, and leave", true, "Enter"),
		},
		Exit:   "leave",
		Pause:  true,
		OnExit: func() tea.Cmd { m.leaveSelect(); m.sync(); return nil },
	}
}

// enterSelect starts visual mode on the shown tab (ux.md §1).
func (m *AppModel) enterSelect() tea.Cmd {
	t := m.shownTab()
	if t == nil || t.root == nil {
		return m.toast("No page to select from")
	}
	m.sel.enter(t)
	m.pagePanel.EnterMode(m.visualMode())
	return m.focusPage()
}

// leaveSelect ends the mode: the item cursor lands nearest the character
// cursor, and a capture held back while it was on is applied now.
func (m *AppModel) leaveSelect() {
	if !m.sel.on {
		return
	}
	m.sel.on = false
	if m.pagePanel.InMode() != nil {
		m.pagePanel.LeaveMode()
	}
	t := m.shownTab()
	if t == nil {
		return
	}
	if i := t.lay.nearestItem(m.sel.row); i >= 0 {
		t.cursor = i
	}
	if t.frozen != nil {
		msg := *t.frozen
		t.frozen = nil
		t.apply(msg, m.pageW())
		m.recordVisit(t)
	}
	t.scrollToCursor(m.pageVisible())
}

// visualKey is one of the mode's keys.
func (m *AppModel) visualKey(k string) tea.Cmd {
	defer m.sync()
	t := m.shownTab()
	if t == nil {
		m.leaveSelect()
		return nil
	}
	vis := m.pageVisible()
	res, text := m.sel.key(k, vis)
	// Keep the character cursor on screen.
	if m.sel.row < t.top {
		t.top = m.sel.row
	}
	if m.sel.row >= t.top+vis {
		t.top = m.sel.row - vis + 1
	}
	switch res {
	case selYank:
		m.leaveSelect()
		if text == "" {
			return m.toast("Nothing to copy")
		}
		return copyToClipboard(text)
	case selClick:
		i := t.lay.itemAtCol(m.sel.row, m.sel.col)
		if i < 0 {
			return m.toast("Nothing to press here")
		}
		m.leaveSelect()
		t.cursor = i
		return m.enterItem()
	}
	return nil
}

// visualSearch is / in the mode: what to find, typed in a box; n and N
// step through what it found.
func (m *AppModel) visualSearch() tea.Cmd {
	return m.a.Text(terminu.TextInput{Title: "Find", Glyph: glyphSearch, Name: "The text",
		Prompt: "Text to find on the page", Verb: "find", Old: m.sel.query,
		Done: func(v string) tea.Cmd {
			m.sel.query = v
			m.sel.search()
			m.sel.jumpMatch(m.sel.firstMatchFrom(m.sel.row, m.sel.col))
			if t := m.shownTab(); t != nil {
				vis := m.pageVisible()
				if m.sel.row < t.top || m.sel.row >= t.top+vis {
					t.top = max(0, m.sel.row-vis/2)
				}
			}
			return nil
		}})
}
