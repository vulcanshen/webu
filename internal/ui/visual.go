package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// Visual mode (ux.md §1) is a mode of panel [2] (tdp K11): the lib
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
	// What acts on the range or on the item under the cursor is an item
	// operation in the mode's Space menu (tdp K11, menu.4).
	item := func(k terminu.ModeKey) terminu.ModeKey { k.Item = true; return k }
	md := &terminu.Mode{
		Name: "Visual",
		Keys: []terminu.ModeKey{
			key("h|j|k|l", "move by character and row", false, "h", "j", "k", "l", "←", "↓", "↑", "→"),
			key("w|b|e", "next word, previous word, end of the word", false, "w", "b", "e"),
			key("0|$", "start, end of the row", false, "0", "$"),
			key("u|d", "half a page up, down (PgUp, PgDn too)", false, "u", "d", "PgUp", "PgDn"),
			key("gg|G", "top, bottom of the page", false, "g", "G"),
			item(key("v|V", "select", true, "v", "V")),
			// Named as the lib's own selection mode names it.
			item(terminu.ModeKey{Key: "y", Desc: "copy", Label: "[y] Copy", Do: m.visualKey}),
			{Key: "/", Desc: "find", Do: func(string) tea.Cmd { return m.visualSearch() }},
			key("n|N", "next match", true, "n", "N"),
			item(key("Enter", "press", true, "Enter")),
		},
		Exit:  "leave",
		Pause: true,
	}
	md.OnExit = func() tea.Cmd {
		if m.sel.selecting {
			// Esc while selecting goes back a layer (tdp K11): the
			// selection goes, the cursor stays, and the lib, which has
			// left the mode, is put back in it.
			m.sel.selecting = false
			m.visualLayer(md)
			m.pagePanel.EnterMode(md)
			m.sync()
			return nil
		}
		cmd := m.leaveSelect()
		m.sync()
		return cmd
	}
	return md
}

// visualLayer names the mode's layer (tdp K11): once selecting it is
// Visual>select, and Esc unselects.
func (m *AppModel) visualLayer(md *terminu.Mode) {
	md.Name, md.Exit = "Visual", "leave"
	if m.sel.selecting {
		md.Name, md.Exit = "Visual>select", "unselect"
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
// cursor, and a capture held back while it was on comes back as the
// message it was, for applyPage to take like any other — the visit written,
// a failed write or look said.
func (m *AppModel) leaveSelect() tea.Cmd {
	if !m.sel.on {
		return nil
	}
	m.sel.on = false
	if m.pagePanel.InMode() != nil {
		m.pagePanel.LeaveMode()
	}
	t := m.shownTab()
	if t == nil {
		return nil
	}
	if i := t.lay.nearestItem(m.sel.row); i >= 0 {
		t.cursor = i
	}
	t.scrollToCursor(m.pageVisible())
	if t.frozen != nil {
		msg := *t.frozen
		t.frozen = nil
		return func() tea.Msg { return msg }
	}
	return nil
}

// visualKey is one of the mode's keys.
func (m *AppModel) visualKey(k string) tea.Cmd {
	defer m.sync()
	t := m.shownTab()
	if t == nil {
		return m.leaveSelect()
	}
	vis := m.pageVisible()
	res, text := m.sel.key(k, vis)
	if md := m.pagePanel.InMode(); md != nil {
		m.visualLayer(md)
	}
	// Keep the character cursor on screen.
	if m.sel.row < t.top {
		t.top = m.sel.row
	}
	if m.sel.row >= t.top+vis {
		t.top = m.sel.row - vis + 1
	}
	switch res {
	case selYank:
		left := m.leaveSelect()
		if text == "" {
			return tea.Batch(left, m.toast("Nothing to copy"))
		}
		return tea.Batch(left, copyToClipboard(text))
	case selClick:
		i := t.lay.itemAtCol(m.sel.row, m.sel.col)
		if i < 0 {
			return m.toast("Nothing to press here")
		}
		left := m.leaveSelect()
		t.cursor = i
		return tea.Batch(left, m.enterItem())
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
