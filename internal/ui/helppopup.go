package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpPopup is ? on a float (tdp K6): the keys of THAT float, read-only —
// what can be pressed in this box and what it does. ? on a panel is the
// ? menu instead (globalMenuItems).
type helpPopup struct {
	anim    popupAnimator
	title   string
	entries []helpEntry
	top     int
	layer   int
	screenW int
	screenH int
}

func newHelpPopup() helpPopup { return helpPopup{anim: newPopupAnimator("help")} }

func (m helpPopup) isActive() bool      { return m.anim.isActive() }
func (m helpPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *helpPopup) open(title string, entries []helpEntry, layer int) tea.Cmd {
	m.title, m.entries, m.layer, m.top = title, entries, layer, 0
	return m.anim.open()
}
func (m *helpPopup) close() tea.Cmd   { return m.anim.close() }
func (m *helpPopup) setSize(w, h int) { m.screenW, m.screenH = w, h }

// helpEntry is one line: a key and what it does.
type helpEntry struct{ key, desc string }

// Each float's keys, as its ? shows them. A float's border legend is the
// short form of the same list.
var (
	helpMenu = []helpEntry{
		{"j · k", "move; off either end wraps"},
		{"u · d", "half a window"},
		{"gg · G", "first / last row"},
		{"Enter", "run the row"},
		{"a row's key", "run that row at once"},
		{"Esc", "close"},
	}
	helpOptions = []helpEntry{
		{"j · k", "move; off either end wraps"},
		{"u · d", "half a window"},
		{"gg · G", "first / last row"},
		{"Enter", "choose the row"},
		{"a row's key", "choose that row at once"},
		{"Esc", "close; nothing is chosen"},
	}
	helpConfirm = []helpEntry{
		{"Enter", "do it"},
		{"Esc", "cancel"},
	}
	helpFinder = []helpEntry{
		{"j · k · u · d", "move through the hits"},
		{"Enter", "go there; nothing is pressed"},
		{"Esc", "back to the query"},
	}
	helpGo = []helpEntry{
		{"0-9", "narrow to a line number"},
		{"j · k", "move"},
		{"Enter", "go to the line"},
		{"Esc", "close"},
	}
	helpEditor = []helpEntry{
		{"h j k l", "move through the text"},
		{"i · a · A · o", "write"},
		{"Enter", "set the box to this text"},
		{"Esc", "cancel; the box keeps what it had"},
	}
	helpMessage = []helpEntry{
		{"j · k", "scroll, when it is longer than the box"},
		{"Esc", "close"},
	}
	helpCheatsheet = []helpEntry{
		{"a listed key", "close the sheet and do it"},
		{"Space · Esc", "close"},
	}
	helpDevtools = []helpEntry{
		{"h · l", "Network · Storage · Console · Source"},
		{"j · k · u · d", "move"},
		{"Enter", "a request's or a message's detail"},
		{"/", "filter; in Source, grep"},
		{"x · y", "Storage: delete / yank the value"},
		{"C", "clear the list; in Storage, the site's data"},
		{"i", "Console: evaluate JavaScript in the page"},
		{"Esc", "close the detail, then DevTools"},
	}
)

// floatHelp is the help of the float ? was pressed on: the topmost that
// holds the keyboard, in key routing's order. A float being typed into
// never gets here — ? is a character there.
func (m AppModel) floatHelp() (string, []helpEntry) {
	switch {
	case m.editor.anim.owns():
		return "Text box", helpEditor
	case m.finder.anim.owns() && m.finder.kind == finderGo:
		return "Go to line", helpGo
	case m.finder.anim.owns():
		return "Search", helpFinder
	case m.confirm.anim.owns():
		return m.confirm.title, helpConfirm
	case m.options.anim.owns():
		return m.options.title, helpOptions
	case m.devtools.anim.owns():
		return "DevTools", helpDevtools
	case m.message.anim.owns() && m.message.passKeys:
		return "Visual mode", helpCheatsheet
	case m.message.anim.owns():
		return "Message", helpMessage
	case m.globalMenu.anim.owns():
		return "Global operation", helpMenu
	}
	return m.spaceMenu.title, append(append([]helpEntry{}, helpMenu[:len(helpMenu)-1]...),
		helpEntry{"Space · Esc", "close"})
}

func (m *helpPopup) update(msg tea.KeyMsg) {
	if !m.anim.isInteractive() {
		return
	}
	m.top = moveScroll(m.top, max(0, len(m.entries)-m.visible()), msg.String(), m.visible())
}

// visible is how many content lines fit; the box costs 4 rows of chrome.
func (m helpPopup) visible() int { return max(1, min(len(m.entries), m.screenH-6)) }

func (m helpPopup) view() string {
	keyW := 0
	for _, e := range m.entries {
		keyW = max(keyW, dispW(e.key))
	}
	innerW := popupInnerW(m.screenW, keyW+44)

	key := lipgloss.NewStyle().Foreground(handColor)
	txt := lipgloss.NewStyle().Foreground(textColor)

	vis := m.visible()
	end := min(len(m.entries), m.top+vis)
	rows := make([]string, 0, vis)
	for _, e := range m.entries[m.top:end] {
		rows = append(rows, key.Render(padRight("  "+e.key, keyW+4))+
			txt.Render(padRight(e.desc, innerW-keyW-4)))
	}

	pairs := [][2]string{{"Esc", "close"}}
	if len(m.entries) > vis {
		pairs = append([][2]string{{"j/k", "scroll"}}, pairs...)
	}
	hint := hintLegend(pairs)
	return drawPopupBox(popupLayerColor(m.layer), " "+glyphHelp+" "+m.title+" · keys ", hint,
		animRows(m.anim, capRows(rows, m.screenH)), innerW)
}

// globalMenuItems is the ? menu (tdp M4): on top what the app can do from
// anywhere, each row run like a Space menu's; under it the core keys, to
// read. Every Space menu ends in one row that opens this (dev-remarks.md,
// 偏離 tdp).
func globalMenuItems() []menuItem {
	return []menuItem{
		{header: true, label: "global operation"},
		{label: "Web", key: "W", hint: "the tabs and the page"},
		{label: "Bookmarks", key: "B", hint: "the folder tree"},
		{label: "History", key: "H", hint: "every page visited"},
		{label: "Downloads", key: "D", hint: "this session's"},
		{label: "Settings", key: "S", hint: "config.yaml, in place"},
		{label: "Previous", key: "P", hint: "back in this tab"},
		{label: "Next", key: "N", hint: "forward in this tab"},
		{label: "Location", key: "L", hint: "a URL or a search"},
		{label: "[/] Search", key: "/", hint: "every part of the page; Enter goes there"},
		{label: "Visual mode", key: "v", hint: "walk the text by character, copy some"},
		{label: "Quit", key: "q", hint: "Ctrl+C too; asks while a download runs"},
		{separator: true},
		{header: true, label: "key reference"},
		{note: true, label: "Enter", hint: "the item, as a click; where a click means nothing, go in"},
		{note: true, label: "Esc", hint: "one step back up"},
		{note: true, label: "Space", hint: "what can I do here: the item and the panel"},
		{note: true, label: "?", hint: "on a panel this menu; on a popup, its keys"},
		{note: true, label: "Tab · 1 · 2", hint: "next panel / this panel"},
		{note: true, label: "j · k", hint: "next / previous item"},
		{note: true, label: "h · l", hint: "along a row"},
		{note: true, label: "u · d", hint: "half a page"},
		{note: true, label: "gg · G", hint: "first / last"},
	}
}
