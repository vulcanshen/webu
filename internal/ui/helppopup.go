package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpPopup is ? (tdp K6, M4): the key reference of the frontmost surface,
// read-only — on a float the keys of THAT float, on a panel the panel's
// keys and the core keys. Nothing in it runs; that is Space's.
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

// helpEntry is one line: a key and what it does, or with no key a group's
// heading. disabled: the key is there but cannot be pressed now, dimmed as
// its Space menu row is (tdp M6 v0.1.14).
type helpEntry struct {
	key, desc string
	disabled  bool
}

// coreKeys close every panel's key reference.
var coreKeys = []helpEntry{
	{key: "Enter", desc: "the item, as a click; where a click means nothing, go in"},
	{key: "Esc", desc: "one step back up"},
	{key: "Space", desc: "what can I do here: the item, the panel, the global operations"},
	{key: "?", desc: "these keys; on a popup, its keys"},
	{key: "Tab/1–2", desc: "next panel / this panel"},
	{key: "j/k", desc: "next / previous item"},
	{key: "h/l", desc: "along a row"},
	{key: "u/d", desc: "half a page"},
	{key: "gg/G", desc: "first / last"},
	{key: "q/Ctrl-C", desc: "quit"},
}

// keyReference is a panel's ? from its Space menu rows: every row that a
// key reaches — a letter, or a core key written into its label — under
// the menu's own region headings, then the core keys. A menu-only row has
// no key to list. When the panel says what its Enter does, the general
// Enter line is not repeated.
func keyReference(items []menuItem) []helpEntry {
	var out []helpEntry
	enter := false
	for _, it := range items {
		if it.header {
			out = append(out, helpEntry{desc: it.label})
			continue
		}
		k, label := rowKey(it)
		if k == "" {
			continue
		}
		enter = enter || k == "Enter"
		if it.hint != "" {
			label += " — " + it.hint
		}
		out = append(out, helpEntry{k, label, it.disabled})
	}
	// A heading with nothing of its own under it goes.
	var kept []helpEntry
	for i, e := range out {
		if e.key == "" && (i+1 == len(out) || out[i+1].key == "") {
			continue
		}
		kept = append(kept, e)
	}
	kept = append(kept, helpEntry{desc: "core keys"})
	for _, e := range coreKeys {
		if !(enter && e.key == "Enter") {
			kept = append(kept, e)
		}
	}
	return kept
}

// rowKey is the key a Space menu row answers to and its label without it:
// "[Enter] Switch to" is Enter, "[go] Go to line" is go, a one-letter key
// is itself. "" for a row only the menu runs.
func rowKey(it menuItem) (string, string) {
	if strings.HasPrefix(it.label, "[") {
		if i := strings.Index(it.label, "] "); i > 0 {
			return it.label[1:i], it.label[i+2:]
		}
	}
	switch {
	case len(it.key) == 1 && strings.HasPrefix(it.label, it.key+" "):
		// "X close others": the key already leads the label.
		return it.key, it.label[len(it.key)+1:]
	case len(it.key) == 1:
		return it.key, it.label
	case it.key == "enter":
		return "Enter", it.label
	}
	return "", ""
}

// Each float's keys, as its ? shows them. A float's border legend is the
// short form of the same list.
var (
	helpMenu = []helpEntry{
		{key: "j/k", desc: "move; off either end wraps"},
		{key: "u/d", desc: "half a window"},
		{key: "gg/G", desc: "first / last row"},
		{key: "Enter", desc: "run the row"},
		{key: "a row's key", desc: "run that row at once"},
		{key: "Esc", desc: "close"},
	}
	helpOptions = []helpEntry{
		{key: "j/k", desc: "move; off either end wraps"},
		{key: "u/d", desc: "half a window"},
		{key: "gg/G", desc: "first / last row"},
		{key: "Enter", desc: "choose the row"},
		{key: "a row's key", desc: "choose that row at once"},
		{key: "Esc", desc: "close; nothing is chosen"},
	}
	helpConfirm = []helpEntry{
		{key: "Enter", desc: "do it"},
		{key: "Esc", desc: "cancel"},
	}
	helpFinder = []helpEntry{
		{key: "j/k/u/d", desc: "move through the hits"},
		{key: "Enter", desc: "go there; nothing is pressed"},
		{key: "Tab", desc: "back to the query"},
		{key: "Esc", desc: "close the search"},
	}
	helpGo = []helpEntry{
		{key: "0–9", desc: "narrow to a line number"},
		{key: "j/k", desc: "move"},
		{key: "Enter", desc: "go to the line"},
		{key: "Esc", desc: "close"},
	}
	helpEditor = []helpEntry{
		{key: "h/j/k/l", desc: "move through the text"},
		{key: "i/a/A/o", desc: "write"},
		{key: "Enter", desc: "set the box to this text"},
		{key: "Esc", desc: "cancel; the box keeps what it had"},
	}
	helpMessage = []helpEntry{
		{key: "j/k", desc: "scroll"},
		{key: "Esc", desc: "close"},
	}
	helpDevtools = []helpEntry{
		{key: "h/l", desc: "Network · Storage · Console · Source"},
		{key: "j/k/u/d", desc: "move"},
		{key: "Enter", desc: "a request's or a message's detail"},
		{key: "/", desc: "filter; in Source, grep"},
		{key: "x/y", desc: "Storage: delete / yank the value"},
		{key: "C", desc: "clear the list; in Storage, the site's data"},
		{key: "i", desc: "Console: evaluate JavaScript in the page"},
		{key: "Esc", desc: "close the detail, then DevTools"},
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
	case m.choices.anim.owns():
		return m.choices.title, helpOptions
	case m.options.anim.owns():
		return m.options.title, helpOptions
	case m.devtools.anim.owns():
		return "DevTools", m.devtools.help()
	case m.message.anim.owns():
		return "Message", m.message.help()
	case m.globalMenu.anim.owns():
		return "Global operation", append(append([]helpEntry{}, helpMenu[:len(helpMenu)-1]...),
			helpEntry{key: "Esc", desc: "back to the Space menu"})
	}
	return m.spaceMenu.title, append(append([]helpEntry{}, helpMenu[:len(helpMenu)-1]...),
		helpEntry{key: "Space/Esc", desc: "close"})
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
	pairs := [][2]string{{"Esc", "close"}}
	if len(m.entries) > m.visible() {
		pairs = append([][2]string{{"j/k", "scroll"}}, pairs...)
	}
	title := " " + glyphHelp + " " + m.title + " · keys "

	// One width for every popup (tdp F7, D4): a description longer than the
	// box is cut at its end, the box is not widened for it.
	keyW := 0
	for _, e := range m.entries {
		keyW = max(keyW, dispW(e.key))
	}
	innerW := popupW(m.screenW)
	hint := fitLegend(pairs, innerW-1)

	dim := lipgloss.NewStyle().Foreground(dimColor)
	key := lipgloss.NewStyle().Foreground(focusColor) // Blue, as in a hint (tdp D2)
	txt := lipgloss.NewStyle().Foreground(textColor)

	vis := m.visible()
	end := min(len(m.entries), m.top+vis)
	rows := make([]string, 0, vis)
	for _, e := range m.entries[m.top:end] {
		if e.key == "" {
			rows = append(rows, dim.Render(padRight(" "+e.desc, innerW)))
			continue
		}
		k, d := key, txt
		if e.disabled {
			k, d = dim, dim // as the Space menu draws a row that cannot run
		}
		rows = append(rows, k.Render(padRight("  "+e.key, keyW+4))+
			d.Render(padRight(truncate(e.desc, innerW-keyW-5), innerW-keyW-4)))
	}
	return drawPopupBox(popupLayerColor(m.layer), title, hint,
		animRows(m.anim, capRows(rows, m.screenH)), innerW)
}

// globalMenuItems are the global operation popup's rows (tdp M4): what acts
// on the app rather than on a panel — the screens and quitting — each run
// like a Space menu's row. Back, forward, location, search and visual mode
// act on the page, so they are [2]'s panel operations and not here (tdp
// M3, 2026-09-27). Every Space menu ends in the one row that opens this.
func globalMenuItems(on screen) []menuItem {
	items := []menuItem{
		{label: "Web", key: "W", hint: "the tabs and the page"},
		{label: "Bookmarks", key: "B", hint: "the folder tree"},
		{label: "History", key: "H", hint: "every page visited"},
		{label: "Downloads", key: "D", hint: "this session's"},
		{label: "Settings", key: "S", hint: "config.yaml, in place"},
		{label: "Quit", key: "q", hint: "[Ctrl-C] too; asks while a download runs"},
	}
	// The screen already up is a row that cannot run (tdp M4, M6): dimmed
	// in its own words, not left out.
	for i := range items {
		if s, ok := screenKeys[items[i].key]; ok && s == on {
			items[i].disabled = true
		}
	}
	return items
}
