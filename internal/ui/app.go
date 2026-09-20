package ui

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
	"github.com/vulcanshen/webu/internal/store"
)

// The three panels, numbered left to right, top to bottom (ui.md §1.1).
type panelID int

const (
	panel1 panelID = iota // Bookmarks / Shortcuts / History
	panel2                // Tabs
	panel3                // the page
)

const (
	// sideW is the side column's outer width, fixed (ui.md §1.2).
	sideW = 28
	// narrowW is where the grid gives up and only the focused side is drawn.
	narrowW = 72
	// minAppH is the least height at which the frame is worth drawing.
	minAppH = 10
)

// searchEngine is the default for a goto that is not a URL (ux.md §7);
// config.yaml can change it.
const searchEngine = store.DefaultSearch

type AppModel struct {
	w, h  int
	focus panelID
	cur1  int
	cur2  int
	top2  int

	tabs      []*tab
	shown     int // index into tabs panel [3] displays; -1 for none
	nextTabID int
	browser   *browser.Browser
	events    chan pageEventMsg
	startURL  string
	session   store.Session // what to restore on the first frame

	// webu's own files, loaded by main and written back as they change.
	bookmarks []store.Bookmark
	cfg       store.Config
	history   []store.Visit

	// Floats. The Space menu goes down first; a target opened from it stacks
	// above (§6.4). The toast rides on top of everything.
	spaceMenu spaceMenu
	options   spaceMenu // a textbox's Submit/Edit/Clear/Yank, or a select's options
	lists     listPopup // Bookmarks / Shortcuts / History
	help      helpPopup
	confirm   confirmPopup
	input     inputPopup
	toast     toastModel

	// optionsFor is the node the options menu is about.
	optionsFor *ir.Node
	// pendingG holds the first half of the gg chord.
	pendingG bool
}

// New builds the app over a running browser. startURL, when given, opens as
// a tab in front of whatever the session restores (function.md §12).
func New(b *browser.Browser, startURL string) AppModel {
	return AppModel{
		focus:     panel3,
		shown:     -1,
		browser:   b,
		events:    make(chan pageEventMsg, 8),
		startURL:  startURL,
		spaceMenu: newSpaceMenu(),
		options:   spaceMenu{anim: newPopupAnimator("options")},
		lists:     newListPopup(),
		help:      newHelpPopup(),
		confirm:   newConfirmPopup(),
		input:     newInputPopup(),
		toast:     newToast(),
	}
}

// WithStore hands the app its files. The UI never reads them itself.
func (m AppModel) WithStore(bookmarks []store.Bookmark, cfg store.Config, history []store.Visit) AppModel {
	m.bookmarks, m.cfg, m.history = bookmarks, cfg, history
	return m
}

// WithSession is what was open last time; restored on the first frame,
// without loading (ux.md §6).
func (m AppModel) WithSession(s store.Session) AppModel {
	m.session = s
	return m
}

// Session is what to write down on the way out: every tab's URL, and which
// one panel [3] was on.
func (m AppModel) Session() store.Session {
	s := store.Session{Shown: max(0, m.shown)}
	for _, t := range m.tabs {
		if t.url == "" || t.url == "about:blank" {
			continue
		}
		s.Tabs = append(s.Tabs, store.SessionTab{URL: t.url, Title: t.title})
	}
	if s.Shown >= len(s.Tabs) {
		s.Shown = max(0, len(s.Tabs)-1)
	}
	return s
}

func (m AppModel) Init() tea.Cmd {
	return waitEvent(m.events)
}

// Close releases every tab. Chromium itself is the caller's to stop.
func (m AppModel) Close() {
	for _, t := range m.tabs {
		t.close()
	}
}

func (m AppModel) narrow() bool { return m.w < narrowW }
func (m AppModel) panelH() int  { return m.h - 1 } // one footer row
func (m AppModel) layer() int {
	if m.spaceMenu.isActive() || m.options.isActive() || m.lists.isActive() {
		return 2
	}
	return 1
}

func (m AppModel) shownTab() *tab {
	if m.shown < 0 || m.shown >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.shown]
}

func (m AppModel) tabByID(id int) (int, *tab) {
	for i, t := range m.tabs {
		if t.id == id {
			return i, t
		}
	}
	return -1, nil
}

// ------------------------------------------------------------------ update

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		first := m.w == 0
		m.w, m.h = msg.Width, msg.Height
		for _, p := range []interface{ setSize(int, int) }{
			&m.spaceMenu, &m.options, &m.lists, &m.help, &m.confirm, &m.input, &m.toast} {
			p.setSize(m.w, m.h)
		}
		for _, t := range m.tabs {
			if t.layW != m.pageW() {
				t.relayout(m.pageW())
				t.scrollToCursor(m.pageVisible())
			}
		}
		if first {
			return m, m.firstFrame()
		}
		return m, nil

	case AnimTickMsg:
		return m, tea.Batch(
			m.spaceMenu.anim.tick(msg), m.options.anim.tick(msg), m.lists.anim.tick(msg),
			m.help.anim.tick(msg), m.confirm.anim.tick(msg), m.input.anim.tick(msg), m.toast.anim.tick(msg))

	case toastExpireMsg:
		return m, m.toast.expire(msg)

	case clipboardDoneMsg:
		if msg.err != nil {
			return m, m.toast.show(clipboardFailure(msg.err), toastError)
		}
		return m, m.toast.show("copied "+plural(msg.lines, "line"), toastInfo)

	case actionErrMsg:
		return m, m.toast.show(msg.err.Error(), toastError)

	case pageEventMsg:
		// Chromium says the tab moved or changed; look again shortly. Always
		// re-arm the listener, or the next event has nowhere to go.
		_, t := m.tabByID(msg.tabID)
		if t == nil {
			return m, waitEvent(m.events)
		}
		return m, tea.Batch(waitEvent(m.events), t.settle(250*time.Millisecond))

	case settleMsg:
		_, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return m, nil
		}
		return m, t.refresh()

	case pageMsg:
		i, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return m, nil
		}
		t.apply(msg, m.pageW())
		t.scrollToCursor(m.pageVisible())
		m.recordVisit(t)
		if i == m.shown && msg.err == nil && t.cursor >= 0 {
			id := t.current().ID
			return m, t.act(func(ctx context.Context) error { return page.Reveal(ctx, id) })
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// firstFrame opens what the app starts with: the session's tabs, unloaded
// except the one that was showing; and the URL from the command line, in
// front, when there is one.
func (m *AppModel) firstFrame() tea.Cmd {
	var cmds []tea.Cmd
	for _, st := range m.session.Tabs {
		t := m.newTab()
		t.url, t.title, t.pending = st.URL, st.Title, true
		m.tabs = append(m.tabs, t)
	}
	if len(m.tabs) > 0 {
		m.shown = clamp(m.session.Shown, 0, len(m.tabs)-1)
		m.cur2 = m.shown
	}
	if m.startURL != "" {
		cmds = append(cmds, m.openTab(m.startURL, true))
	} else if t := m.shownTab(); t != nil && t.pending {
		cmds = append(cmds, t.load(t.url))
	}
	return tea.Batch(cmds...)
}

// recordVisit appends a loaded page to the history log (ux.md §6): the
// final URL and title after the load, once per page, never about:blank or
// an error page. Written synchronously — one line, a small file.
func (m *AppModel) recordVisit(t *tab) {
	if t.errText != "" || t.root == nil || t.url == "" || t.url == "about:blank" || t.url == t.lastVisit {
		return
	}
	t.lastVisit = t.url
	v := store.Visit{At: time.Now(), URL: t.url, Title: t.title}
	if err := store.AppendVisit(v); err == nil {
		m.history = append([]store.Visit{v}, m.history...)
	}
}

// ------------------------------------------------------------------- keys

func (m AppModel) popupOpen() bool {
	return m.spaceMenu.isActive() || m.options.isActive() || m.lists.isActive() ||
		m.help.isActive() || m.confirm.isActive() || m.input.isActive()
}

// typing reports whether a float is taking text: every printable key is a
// character then (§4.5).
func (m AppModel) typing() bool {
	return m.input.anim.owns() || (m.lists.anim.owns() && m.lists.typing)
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc is one role, resolved in one place: close the topmost float (§4.3).
	// With nothing up it does nothing — the previous page is P (ux.md §A.0.K).
	if msg.Type == tea.KeyEscape {
		return m.closeTop()
	}
	if msg.Type == tea.KeyCtrlC {
		return m.quit()
	}
	// Space and ? close what they open (§A.1 / §A.2), unless a float is
	// being typed into.
	if msg.Type == tea.KeySpace && m.popupOpen() && !m.typing() {
		return m.closeTop()
	}
	if msg.String() == "?" && !m.typing() {
		if m.help.anim.owns() {
			return m, m.help.close()
		}
		return m, m.help.open(m.layer())
	}

	switch {
	case m.input.anim.owns():
		return m.inputKey(msg)
	case m.confirm.anim.owns():
		return m.confirmKey(msg)
	case m.options.anim.owns():
		return m.optionsKey(msg)
	case m.lists.anim.owns():
		return m.listKey(msg)
	case m.help.anim.owns():
		m.help.update(msg)
		return m, nil
	case m.spaceMenu.anim.owns():
		return m.menuKey(msg)
	}
	return m.panelKey(msg)
}

// closeTop pops one level off the float stack; the source under a target
// stays (§6.4).
//
// A float that is already closing is not "the top": its keyboard is gone
// (§6.2), and an Esc that landed on it would do nothing while the float
// underneath waited — which is exactly what happens when two keys arrive
// inside one closing animation.
func (m AppModel) closeTop() (tea.Model, tea.Cmd) {
	switch {
	case m.toast.anim.owns():
		return m, m.toast.close()
	case m.input.anim.owns():
		return m, m.input.close()
	case m.confirm.anim.owns():
		return m, m.confirm.close()
	case m.options.anim.owns():
		return m, m.options.close()
	case m.lists.anim.owns():
		// A filter being typed is the innermost thing Esc can drop.
		if m.lists.escTyping() {
			return m, nil
		}
		return m, m.lists.close()
	case m.help.anim.owns():
		return m, m.help.close()
	case m.spaceMenu.anim.owns():
		return m, m.spaceMenu.close()
	}
	return m, nil
}

// closeStack tears every float down: an errand that ended in an action is
// over, and the user is back on the panel (§7.1).
func (m *AppModel) closeStack() tea.Cmd {
	return tea.Batch(m.input.close(), m.confirm.close(), m.options.close(),
		m.lists.close(), m.help.close(), m.spaceMenu.close())
}

func (m AppModel) quit() (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

// panelKey is a key with no float up: the global letters first, then the
// focused panel's own.
func (m AppModel) panelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	// The gg chord: g then g.
	if m.pendingG {
		m.pendingG = false
		if k == "g" {
			k = "gg"
		}
	} else if k == "g" {
		m.pendingG = true
		return m, nil
	}

	switch k {
	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "1", "2", "3":
		m.focus = panelID(k[0] - '1')
		return m, nil
	case "q":
		return m.quit()
	case "B":
		return m, m.openList(listBookmarks)
	case "S":
		return m, m.openList(listShortcuts)
	case "H":
		return m, m.openList(listHistory)
	case "P":
		return m.dispatch("back")
	case "N":
		return m.dispatch("forward")
	case " ":
		return m.openMenu()
	}

	switch m.focus {
	case panel1:
		if navKeys[k] {
			m.cur1 = moveCursor(m.cur1, len(side1Items), k, panel1Rows)
			return m, nil
		}
		if k == "enter" {
			return m, m.openList(listKind(m.cur1))
		}
	case panel2:
		if navKeys[k] {
			m.cur2 = moveCursor(m.cur2, len(m.tabs), k, m.panelH()-panel1Rows-4)
			return m, nil
		}
		switch k {
		case "enter":
			return m.dispatch("show")
		case "w", "c", "r", "y", "T", "X":
			return m.dispatch(k)
		}
	case panel3:
		t := m.shownTab()
		if navKeys[k] && t != nil {
			t.moveItem(k, m.pageVisible())
			if n := t.current(); n != nil {
				id := n.ID
				return m, t.act(func(ctx context.Context) error { return page.Reveal(ctx, id) })
			}
			return m, nil
		}
		switch k {
		case "enter":
			return m.dispatch("enter")
		case "R", "U", "Y", "A":
			return m.dispatch(k)
		case "O", "D", "Z", "V", "/":
			return m, m.toast.show("not in this build yet", toastInfo)
		}
	}
	return m, nil
}

// ------------------------------------------------------------------- lists

// openList shows one of panel [1]'s popups with its current content.
func (m *AppModel) openList(kind listKind) tea.Cmd {
	return m.lists.open(kind, m.listEntries(kind), m.layer())
}

func (m AppModel) listEntries(kind listKind) []listEntry {
	var out []listEntry
	switch kind {
	case listBookmarks:
		for _, b := range m.bookmarks {
			out = append(out, listEntry{title: b.Title, url: b.URL})
		}
	case listShortcuts:
		for _, s := range m.cfg.Shortcuts {
			out = append(out, listEntry{title: s.Title, url: s.URL})
		}
	case listHistory:
		for _, v := range m.history {
			out = append(out, listEntry{title: v.Title, url: v.URL, at: v.At})
		}
	}
	return out
}

func (m AppModel) listKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.lists.update(msg)
	if action == "" {
		return m, nil
	}
	e, at, ok := m.lists.current()
	switch action {
	case "open":
		if !ok {
			return m, nil
		}
		if t := m.shownTab(); t != nil {
			m.focus = panel3
			return m, tea.Batch(m.closeStack(), t.load(e.url))
		}
		return m, tea.Batch(m.closeStack(), m.openTab(e.url, true))
	case "newtab":
		if !ok {
			return m, nil
		}
		return m, tea.Batch(m.closeStack(), m.openTab(e.url, true))
	case "add":
		t := m.shownTab()
		if t == nil || t.url == "" {
			return m, m.toast.show("no page to add", toastInfo)
		}
		return m, m.addEntry(m.lists.kind, t.title, t.url)
	case "delete":
		if !ok {
			return m, nil
		}
		return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Delete",
			lines: []string{nameOr(e.title, e.url), e.url}, accept: "delete", warn: true,
			action: confirmDeleteEntry, at: at}, m.layer()+1)
	case "clear":
		return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Clear history",
			lines:  []string{"Every visit ever recorded goes.", "This is the only way the log shrinks."},
			accept: "clear", warn: true, action: confirmClearHistory}, m.layer()+1)
	}
	return m, nil
}

// addEntry puts the current page in Bookmarks or Shortcuts and writes the
// file; a page already there is not added twice.
func (m *AppModel) addEntry(kind listKind, title, url string) tea.Cmd {
	switch kind {
	case listBookmarks:
		for _, b := range m.bookmarks {
			if b.URL == url {
				return m.toast.show("already bookmarked", toastInfo)
			}
		}
		m.bookmarks = append(m.bookmarks, store.Bookmark{Title: title, URL: url})
		if err := store.SaveBookmarks(m.bookmarks); err != nil {
			return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
		}
	case listShortcuts:
		for _, s := range m.cfg.Shortcuts {
			if s.URL == url {
				return m.toast.show("already a shortcut", toastInfo)
			}
		}
		m.cfg.Shortcuts = append(m.cfg.Shortcuts, store.Shortcut{Title: title, URL: url})
		if err := store.SaveConfig(m.cfg); err != nil {
			return m.toast.show("config.yaml: "+err.Error(), toastError)
		}
	default:
		return nil
	}
	m.lists.setEntries(m.listEntries(kind))
	return m.toast.show("added "+oneLine(nameOr(title, url)), toastInfo)
}

// deleteEntry removes entry at of the open list and writes the file.
func (m *AppModel) deleteEntry(at int) tea.Cmd {
	var err error
	switch m.lists.kind {
	case listBookmarks:
		if at < len(m.bookmarks) {
			m.bookmarks = append(m.bookmarks[:at], m.bookmarks[at+1:]...)
			err = store.SaveBookmarks(m.bookmarks)
		}
	case listShortcuts:
		if at < len(m.cfg.Shortcuts) {
			m.cfg.Shortcuts = append(m.cfg.Shortcuts[:at], m.cfg.Shortcuts[at+1:]...)
			err = store.SaveConfig(m.cfg)
		}
	case listHistory:
		if at < len(m.history) {
			v := m.history[at]
			m.history = append(m.history[:at], m.history[at+1:]...)
			err = store.DeleteVisit(v)
		}
	}
	m.lists.setEntries(m.listEntries(m.lists.kind))
	if err != nil {
		return m.toast.show(err.Error(), toastError)
	}
	return nil
}

// ------------------------------------------------------------------- menus

// openMenu is Space: the contextual list for the focused panel (ux.md
// §A.1). Panel [1] has one action per row, so Space is Enter there.
func (m AppModel) openMenu() (tea.Model, tea.Cmd) {
	var items []menuItem
	title := ""
	switch m.focus {
	case panel1:
		return m, m.openList(listKind(m.cur1))
	case panel2:
		title = "Tabs"
		if len(m.tabs) > 0 {
			items = append(items,
				menuItem{header: true, label: "item operation"},
				menuItem{label: "Switch to", key: "enter", hint: "show this tab in [3]"},
				menuItem{label: "Close", key: "w", hint: "this tab"},
				menuItem{label: "Clone", key: "c", hint: "the same page, a new tab"},
				menuItem{label: "Reload", key: "r", hint: "this tab"},
				menuItem{label: "Yank url", key: "y", hint: "to the clipboard"},
				menuItem{separator: true},
				menuItem{header: true, label: "panel operation"})
		}
		items = append(items,
			menuItem{label: "Tab", key: "T", hint: "a new one, at a URL"},
			menuItem{label: "X close others", key: "X", hint: "every tab but this one", disabled: len(m.tabs) < 2},
			menuItem{label: "Undo close", key: "U", hint: "not in this build yet", disabled: true})
	case panel3:
		title = "Page"
		items = m.pageMenuItems()
	}
	m.spaceMenu.setItems(items, title, 1)
	return m, m.spaceMenu.open()
}

// pageMenuItems is panel [3]'s Space menu: the item's operations by role
// (menu-only, no letters — ux.md §A.1), then the page's.
func (m AppModel) pageMenuItems() []menuItem {
	var items []menuItem
	t := m.shownTab()
	if n := t.current(); t != nil && n != nil {
		items = append(items, menuItem{header: true, label: "item operation"})
		switch n.Kind {
		case ir.Link:
			items = append(items,
				menuItem{label: "Open", key: "enter", hint: "click it"},
				menuItem{label: "Open in new tab", key: "newtab", hint: "and switch to it"},
				menuItem{label: "Yank url", key: "yankurl", hint: oneLine(n.URL)})
		case ir.Button, ir.Check:
			items = append(items, menuItem{label: "Click", key: "enter", hint: "press it"})
		case ir.Media:
			items = append(items,
				menuItem{label: "Click", key: "enter", hint: "the page decides"},
				menuItem{label: "Yank url", key: "yankurl", hint: oneLine(n.URL), disabled: n.URL == ""})
		case ir.Textbox:
			if n.Value == "" {
				items = append(items, menuItem{label: "Edit", key: "enter", hint: "type a value"})
			} else {
				items = append(items, textboxItems(n)...)
			}
		case ir.Combobox:
			items = append(items, menuItem{label: "Choose", key: "enter", hint: "pick an option"})
		case ir.Heading:
			items = append(items, menuItem{label: "Fold section", key: "fold", hint: "not in this build yet", disabled: true})
		case ir.Unsupported:
			items = append(items,
				menuItem{label: "role: " + n.Role + ", not supported yet — only click", key: "enter", disabled: true},
				menuItem{label: "Click", key: "enter", hint: "the page decides"})
		}
		items = append(items,
			menuItem{label: "Yank text", key: "yanktext", hint: "what it says"},
			menuItem{label: "Inspect", key: "inspect", hint: "role, name, node id"},
			menuItem{separator: true},
			menuItem{header: true, label: "panel operation"})
	}
	items = append(items,
		menuItem{label: "Reload", key: "R", hint: "this page", disabled: t == nil},
		menuItem{label: "Previous", key: "P", hint: "back in this tab", disabled: t == nil},
		menuItem{label: "Next", key: "N", hint: "forward in this tab", disabled: t == nil},
		menuItem{label: "Search", key: "/", hint: "not in this build yet", disabled: true},
		menuItem{label: "Select mode", key: "alt+v", hint: "not in this build yet", disabled: true},
		menuItem{label: "URL", key: "U", hint: "go to one"},
		menuItem{label: "Add to…", key: "A", hint: "Bookmarks or Shortcuts", disabled: t == nil},
		menuItem{label: "Outline", key: "O", hint: "not in this build yet", disabled: true},
		menuItem{label: "DevTools", key: "D", hint: "not in this build yet", disabled: true},
		menuItem{label: "Zoom", key: "Z", hint: "not in this build yet", disabled: true},
		menuItem{label: "View source", key: "V", hint: "not in this build yet", disabled: true},
		menuItem{label: "Yank page url", key: "Y", hint: "to the clipboard", disabled: t == nil})
	return items
}

// textboxItems is the menu a filled textbox opens on Enter (ux.md §2.2):
// Submit first, because that is what filling it was for.
func textboxItems(n *ir.Node) []menuItem {
	items := []menuItem{}
	if !n.Multiline {
		items = append(items, menuItem{label: "Submit", key: "submit", hint: "press Enter in the field"})
	}
	return append(items,
		menuItem{label: "Edit", key: "edit", hint: "change the value"},
		menuItem{label: "Clear", key: "clear", hint: "empty the field"},
		menuItem{label: "Yank", key: "yankvalue", hint: "copy the value"})
}

func (m AppModel) menuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var key string
	m.spaceMenu, key, _ = m.spaceMenu.update(msg)
	if key == "" {
		return m, nil
	}
	if i := m.spaceMenu.cursor; i < len(m.spaceMenu.items) && m.spaceMenu.items[i].disabled && m.spaceMenu.items[i].key == key {
		return m, m.toast.show(m.spaceMenu.items[i].hint, toastInfo)
	}
	closeCmd := m.spaceMenu.close()
	mm, cmd := m.dispatch(key)
	return mm, tea.Batch(closeCmd, cmd)
}

// optionsKey drives the second-level menu: a filled textbox's actions, a
// select's options (whose keys are their index), or the Add to… picker.
func (m AppModel) optionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var key string
	m.options, key, _ = m.options.update(msg)
	if key == "" {
		return m, nil
	}
	n := m.optionsFor
	t := m.shownTab()
	if t == nil {
		return m, m.options.close()
	}
	if n == nil {
		// The Add to… picker: keys name the list.
		closeCmd := m.options.close()
		switch key {
		case "b":
			return m, tea.Batch(closeCmd, m.addEntry(listBookmarks, t.title, t.url))
		case "s":
			return m, tea.Batch(closeCmd, m.addEntry(listShortcuts, t.title, t.url))
		}
		return m, closeCmd
	}
	if n.Kind == ir.Combobox {
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(n.Children) {
			return m, m.options.close()
		}
		opt := n.Children[idx]
		return m, tea.Batch(m.closeStack(), t.act(func(ctx context.Context) error { return page.Choose(ctx, opt.ID) }))
	}
	// Close BEFORE dispatching: dispatch returns its own copy of the model,
	// and a close applied to this one afterwards would be applied to a model
	// nobody returns.
	closeCmd := m.options.close()
	mm, cmd := m.dispatch(key)
	return mm, tea.Batch(closeCmd, cmd)
}

// ---------------------------------------------------------------- actions

// dispatch runs one action by its key — the same function whether the key
// was pressed on the panel or chosen from the menu, so the two cannot drift.
func (m AppModel) dispatch(key string) (tea.Model, tea.Cmd) {
	t := m.shownTab()
	switch key {
	// ---- panel [2]
	case "show":
		if m.cur2 >= 0 && m.cur2 < len(m.tabs) {
			return m.showTab(m.cur2)
		}
	case "w":
		return m.closeTab(m.cur2)
	case "c":
		if m.cur2 >= 0 && m.cur2 < len(m.tabs) {
			return m, m.openTab(m.tabs[m.cur2].url, true)
		}
	case "r":
		if m.cur2 >= 0 && m.cur2 < len(m.tabs) {
			return m, m.tabs[m.cur2].load(m.tabs[m.cur2].url)
		}
	case "y":
		if m.cur2 >= 0 && m.cur2 < len(m.tabs) {
			return m, copyToClipboard(m.tabs[m.cur2].url)
		}
	case "T":
		return m, m.input.ask(inputPopup{title: "New tab", glyph: glyphSearch,
			prompt: "URL, or words to search for", accept: "open", action: inputGotoNewTab}, m.layer())
	case "X":
		for i := len(m.tabs) - 1; i >= 0; i-- {
			if i != m.cur2 {
				m.tabs[i].close()
				m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
			}
		}
		m.cur2, m.shown = 0, 0
		return m, nil

	// ---- panel [3], page
	case "R":
		if t != nil {
			return m, t.load(t.url)
		}
	case "back":
		if t != nil {
			return m, t.act(page.Back)
		}
	case "forward":
		if t != nil {
			return m, t.act(page.Forward)
		}
	case "U":
		return m, m.input.ask(inputPopup{title: "Go to", glyph: glyphSearch,
			prompt: "URL, or words to search for", accept: "open", action: inputGoto}, m.layer())
	case "Y":
		if t != nil {
			return m, copyToClipboard(t.url)
		}
	case "A":
		if t != nil {
			m.optionsFor = nil
			m.options.setItems([]menuItem{
				{label: "Bookmarks", key: "b", hint: "the tree you keep"},
				{label: "Shortcuts", key: "s", hint: "the few you reach for"},
			}, "Add to…", m.layer())
			return m, m.options.open()
		}

	// ---- panel [3], item
	case "enter":
		return m.enterItem()
	case "newtab":
		if n := t.current(); n != nil && n.URL != "" {
			return m, m.openTab(n.URL, true)
		}
	case "yankurl":
		if n := t.current(); n != nil {
			return m, copyToClipboard(n.URL)
		}
	case "yanktext":
		if n := t.current(); n != nil {
			return m, copyToClipboard(n.Text())
		}
	case "yankvalue":
		if n := t.current(); n != nil {
			return m, copyToClipboard(n.Value)
		}
	case "inspect":
		if n := t.current(); n != nil {
			return m, m.toast.show(fmt.Sprintf("%s %q #%d %s", n.Role, oneLine(n.Name), n.ID, n.URL), toastInfo)
		}
	case "submit":
		if n := t.current(); n != nil {
			id := n.ID
			return m, t.act(func(ctx context.Context) error { return page.Submit(ctx, id) })
		}
	case "edit":
		if n := t.current(); n != nil {
			return m, m.editField(n)
		}
	case "clear":
		if n := t.current(); n != nil {
			id := n.ID
			return m, t.act(func(ctx context.Context) error { return page.Type(ctx, id, "") })
		}
	}
	return m, nil
}

// enterItem is Enter on panel [3]: a click, with the two exceptions that
// need a second step — a textbox and a select (ux.md §A.0.K, §2).
func (m AppModel) enterItem() (tea.Model, tea.Cmd) {
	t := m.shownTab()
	n := t.current()
	if t == nil || n == nil {
		return m, nil
	}
	switch n.Kind {
	case ir.Textbox:
		if n.Value == "" {
			return m, m.editField(n)
		}
		m.optionsFor = n
		m.options.setItems(textboxItems(n), oneLine(nameOr(n.Name, "field")), m.layer())
		return m, m.options.open()
	case ir.Combobox:
		items := make([]menuItem, 0, len(n.Children))
		for i, o := range n.Children {
			hint := ""
			if o.Selected {
				hint = "current"
			}
			items = append(items, menuItem{label: oneLine(o.Name), key: strconv.Itoa(i), hint: hint})
		}
		if len(items) == 0 {
			return m, m.toast.show("no options to choose from", toastInfo)
		}
		m.optionsFor = n
		m.options.setItems(items, oneLine(nameOr(n.Name, "choose")), m.layer())
		for i, o := range n.Children {
			if o.Selected {
				m.options.cursor = i
			}
		}
		return m, m.options.open()
	}
	id := n.ID
	return m, t.act(func(ctx context.Context) error { return page.Click(ctx, id) })
}

// editField opens the input popup on a textbox. A pointer receiver on
// purpose: called from a value-receiver method, it edits that method's copy,
// which is the model that gets returned — a value receiver here would open
// the popup on a copy nobody keeps.
func (m *AppModel) editField(n *ir.Node) tea.Cmd {
	value := n.Value
	if n.Protected {
		value = ""
	}
	return m.input.ask(inputPopup{title: oneLine(nameOr(n.Name, "field")), glyph: glyphPencil,
		prompt: "value", accept: "set", action: inputField, node: n.ID, value: value,
		masked: n.Protected}, m.layer())
}

func (m AppModel) inputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	value, done := m.input.update(msg)
	if !done {
		return m, nil
	}
	t := m.shownTab()
	switch m.input.action {
	case inputGoto:
		if strings.TrimSpace(value) == "" {
			return m, m.input.close()
		}
		if t == nil {
			return m, tea.Batch(m.closeStack(), m.openTab(m.resolveURL(value), true))
		}
		m.focus = panel3
		return m, tea.Batch(m.closeStack(), t.load(m.resolveURL(value)))
	case inputGotoNewTab:
		if strings.TrimSpace(value) == "" {
			return m, m.input.close()
		}
		return m, tea.Batch(m.closeStack(), m.openTab(m.resolveURL(value), true))
	case inputField:
		id := m.input.node
		if t == nil {
			return m, m.closeStack()
		}
		return m, tea.Batch(m.closeStack(), t.act(func(ctx context.Context) error { return page.Type(ctx, id, value) }))
	}
	return m, m.closeStack()
}

func (m AppModel) confirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.confirm.commit(msg) {
		return m, nil
	}
	closeCmd := m.confirm.close()
	switch m.confirm.action {
	case confirmQuit:
		return m.quit()
	case confirmCloseTab:
		i, _ := m.tabByID(m.confirm.tabID)
		mm, cmd := m.closeTab(i)
		return mm, tea.Batch(closeCmd, cmd)
	case confirmDeleteEntry:
		return m, tea.Batch(closeCmd, m.deleteEntry(m.confirm.at))
	case confirmClearHistory:
		m.history = nil
		if err := store.ClearHistory(); err != nil {
			return m, tea.Batch(closeCmd, m.toast.show(err.Error(), toastError))
		}
		m.lists.setEntries(nil)
		return m, closeCmd
	}
	return m, closeCmd
}

// resolveURL turns what was typed into somewhere to go (ux.md §7): a URL
// gets its scheme, anything else is a search.
func (m AppModel) resolveURL(s string) string {
	return resolveURLWith(s, m.cfg.Search())
}

func resolveURL(s string) string { return resolveURLWith(s, searchEngine) }

func resolveURLWith(s, search string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "://") {
		return s
	}
	if strings.HasPrefix(s, "localhost") || strings.HasPrefix(s, "127.") {
		return "http://" + s
	}
	head := s
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		head = s[:i]
	}
	if !strings.ContainsAny(head, " ") && strings.Contains(head, ".") && !strings.HasSuffix(head, ".") {
		return "https://" + s
	}
	return search + url.QueryEscape(s)
}

// ------------------------------------------------------------------- tabs

// openTab creates a tab at url; show says whether panel [3] switches to it
// (a new tab does, ux.md §6).
func (m *AppModel) openTab(url string, show bool) tea.Cmd {
	t := m.newTab()
	m.tabs = append(m.tabs, t)
	if show || m.shown < 0 {
		m.shown = len(m.tabs) - 1
		m.cur2 = m.shown
		m.focus = panel3
	}
	return t.load(url)
}

// showTab is Enter on panel [2]: panel [3] switches, and the keyboard goes
// with it (ux.md §6). A pending tab loads now.
func (m AppModel) showTab(i int) (tea.Model, tea.Cmd) {
	m.shown = i
	m.focus = panel3
	t := m.tabs[i]
	if t.pending {
		return m, t.load(t.url)
	}
	if t.layW != m.pageW() {
		t.relayout(m.pageW())
	}
	return m, nil
}

func (m AppModel) closeTab(i int) (tea.Model, tea.Cmd) {
	if i < 0 || i >= len(m.tabs) {
		return m, nil
	}
	m.tabs[i].close()
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	switch {
	case len(m.tabs) == 0:
		m.shown, m.cur2 = -1, 0
	case m.shown == i:
		m.shown = min(i, len(m.tabs)-1)
	case m.shown > i:
		m.shown--
	}
	m.cur2 = clamp(m.cur2, 0, max(0, len(m.tabs)-1))
	if t := m.shownTab(); t != nil && t.pending {
		return m, t.load(t.url)
	}
	return m, nil
}

// -------------------------------------------------------------------- view

func (m AppModel) View() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	if m.w < 40 || m.h < minAppH {
		return "terminal too small"
	}
	ph := m.panelH()
	var out string
	switch {
	case m.narrow() && m.focus == panel3:
		out = m.pagePanel(m.w, ph)
	case m.narrow():
		out = m.sideColumn(m.w, ph)
	default:
		out = joinHorizontal(m.sideColumn(sideW, ph), m.pagePanel(m.w-sideW, ph))
	}
	out += "\n" + m.footer()

	// Bottom to top: the menu first so what it opened lands above it.
	if m.spaceMenu.isActive() {
		out = overlay.Composite(m.spaceMenu.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.lists.isActive() {
		out = overlay.Composite(m.lists.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.options.isActive() {
		out = overlay.Composite(m.options.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.help.isActive() {
		out = overlay.Composite(m.help.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.confirm.isActive() {
		out = overlay.Composite(m.confirm.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.input.isActive() {
		out = overlay.Composite(m.input.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.toast.isActive() {
		out = overlay.Composite(m.toast.view(), out, overlay.Center, overlay.Bottom, 0, -2)
	}
	return out
}

// sideColumn is panel [1] over panel [2], outerW wide and outerH tall.
func (m AppModel) sideColumn(outerW, outerH int) string {
	innerW := outerW - 2
	p1 := panelChrome(innerW, fitLines(m.panel1Body(innerW), innerW, panel1Rows), "[1]", m.focus == panel1)
	h2 := outerH - (panel1Rows + 2) - 2
	p2 := panelChrome(innerW, fitLines(m.panel2Body(innerW, h2), innerW, h2), "[2] Tabs", m.focus == panel2)
	return joinVertical(p1, p2)
}

func (m AppModel) pagePanel(outerW, outerH int) string {
	innerW, innerH := outerW-2, outerH-2
	hint := ""
	if t := m.shownTab(); t != nil {
		switch {
		case t.loading:
			hint = "loading"
		case len(t.lay.rows) > innerH-pageHeaderRows:
			hint = fmt.Sprintf("%d-%d of %d", t.top+1, min(len(t.lay.rows), t.top+innerH-pageHeaderRows), len(t.lay.rows))
		}
	}
	tone := toneIdle
	if m.focus == panel3 {
		tone = toneFocus
	}
	return panelFrame(innerW, fitLines(m.pageBody(innerW, innerH), innerW, innerH), "[3]", hint, tone)
}

// footer is the mandatory disclosure of the entry keys (§A.1 / §A.2): one
// row, locked (ui.md §5).
func (m AppModel) footer() string {
	return keyLegend([][2]string{{"space", "menu"}, {"?", "help"}, {"tab/1-3", "panels"}, {"q", "quit"}}, m.w)
}
