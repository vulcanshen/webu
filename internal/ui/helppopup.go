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
// heading.
type helpEntry struct{ key, desc string }

// coreKeys close every panel's key reference.
var coreKeys = []helpEntry{
	{"Enter", "the item, as a click; where a click means nothing, go in"},
	{"Esc", "one step back up"},
	{"Space", "what can I do here: the item, the panel, the global operations"},
	{"?", "these keys; on a popup, its keys"},
	{"Tab/1–2", "next panel / this panel"},
	{"j/k", "next / previous item"},
	{"h/l", "along a row"},
	{"u/d", "half a page"},
	{"gg/G", "first / last"},
	{"q/Ctrl-C", "quit"},
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
		out = append(out, helpEntry{k, label})
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
		{"j/k", "move; off either end wraps"},
		{"u/d", "half a window"},
		{"gg/G", "first / last row"},
		{"Enter", "run the row"},
		{"a row's key", "run that row at once"},
		{"Esc", "close"},
	}
	helpOptions = []helpEntry{
		{"j/k", "move; off either end wraps"},
		{"u/d", "half a window"},
		{"gg/G", "first / last row"},
		{"Enter", "choose the row"},
		{"a row's key", "choose that row at once"},
		{"Esc", "close; nothing is chosen"},
	}
	helpConfirm = []helpEntry{
		{"Enter", "do it"},
		{"Esc", "cancel"},
	}
	helpFinder = []helpEntry{
		{"j/k/u/d", "move through the hits"},
		{"Enter", "go there; nothing is pressed"},
		{"Tab", "back to the query"},
		{"Esc", "close the search"},
	}
	helpGo = []helpEntry{
		{"0–9", "narrow to a line number"},
		{"j/k", "move"},
		{"Enter", "go to the line"},
		{"Esc", "close"},
	}
	helpEditor = []helpEntry{
		{"h/j/k/l", "move through the text"},
		{"i/a/A/o", "write"},
		{"Enter", "set the box to this text"},
		{"Esc", "cancel; the box keeps what it had"},
	}
	helpMessage = []helpEntry{
		{"j/k", "scroll, when it is longer than the box"},
		{"Esc", "close"},
	}
	helpDevtools = []helpEntry{
		{"h/l", "Network · Storage · Console · Source"},
		{"j/k/u/d", "move"},
		{"Enter", "a request's or a message's detail"},
		{"/", "filter; in Source, grep"},
		{"x/y", "Storage: delete / yank the value"},
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
	case m.choices.anim.owns():
		return m.choices.title, helpOptions
	case m.options.anim.owns():
		return m.options.title, helpOptions
	case m.devtools.anim.owns():
		return "DevTools", helpDevtools
	case m.message.anim.owns():
		return "Message", helpMessage
	case m.globalMenu.anim.owns():
		return "Global operation", append(append([]helpEntry{}, helpMenu[:len(helpMenu)-1]...),
			helpEntry{"Esc", "back to the Space menu"})
	}
	return m.spaceMenu.title, append(append([]helpEntry{}, helpMenu[:len(helpMenu)-1]...),
		helpEntry{"Space/Esc", "close"})
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
	hint := hintLegend(pairs)
	title := " " + glyphHelp + " " + m.title + " · keys "

	// One width for every popup (tdp F7, D4): a description longer than the
	// box is cut at its end, the box is not widened for it.
	keyW := 0
	for _, e := range m.entries {
		keyW = max(keyW, dispW(e.key))
	}
	innerW := popupW(m.screenW)

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
		rows = append(rows, key.Render(padRight("  "+e.key, keyW+4))+
			txt.Render(padRight(truncate(e.desc, innerW-keyW-5), innerW-keyW-4)))
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
		{label: "Quit", key: "q", hint: "Ctrl+C too; asks while a download runs"},
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
