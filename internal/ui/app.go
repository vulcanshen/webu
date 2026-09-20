package ui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cdppage "github.com/chromedp/cdproto/page"
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
	// sideW is the side column's outer width, fixed (ui.md §1.2). 28 was
	// the drawing; 24 is what the content needs, and the page gets the rest.
	sideW = 24
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
	events    chan tea.Msg // page and browser events, read by waitEvent
	startURL  string
	session   store.Session // what to restore on the first frame
	// zoom: panel [3] alone fills the screen (ux.md §A.1 [Z]).
	zoom bool
	// sel is selection mode on the shown tab (ux.md §1).
	sel selectMode

	// webu's own files, loaded by main and written back as they change.
	bookmarks []store.Bookmark
	cfg       store.Config
	history   []store.Visit

	// Floats. The Space menu goes down first; a target opened from it stacks
	// above (§6.4). The toast rides on top of everything.
	spaceMenu spaceMenu
	options   spaceMenu // a textbox's Submit/Edit/Clear/Yank, or a select's options
	outline   spaceMenu // the page's landmarks and headings
	lists     listPopup // Bookmarks / Shortcuts / History
	devtools  devtoolsPopup
	viewer    viewerPopup
	message   messagePopup
	help      helpPopup
	confirm   confirmPopup
	input     inputPopup
	toast     toastModel

	// optionsFor is the node the options menu is about, and optionsKind
	// what the menu is: an item's operations, a select's options, or the
	// Add to… picker.
	optionsFor  *ir.Node
	optionsKind optionsKind
	// outlineFor is what the outline's rows stand for.
	outlineFor []outlineEntry
	// dialog is the page's question being asked (function.md §5); the page
	// is stalled until it is answered, and settle captures wait too.
	dialog *dialogMsg
	// auth is the HTTP challenge being answered, and authUser the name
	// typed so far (the password is asked second).
	auth     *authMsg
	authUser string
	// upload is the file chooser waiting on a path.
	upload *fileMsg
	// closed is the tabs closed this session, for [U]ndo close in [2].
	closed []store.SessionTab
	// downloads is how many are in flight: q asks first while any is.
	downloads int
	// pendingG holds the first half of the gg chord.
	pendingG bool
}

// New builds the app over a running browser. startURL, when given, opens as
// a tab in front of whatever the session restores (function.md §12).
func New(b *browser.Browser, startURL string) AppModel {
	m := AppModel{
		focus:     panel3,
		shown:     -1,
		browser:   b,
		events:    make(chan tea.Msg, 16),
		startURL:  startURL,
		spaceMenu: newSpaceMenu(),
		options:   spaceMenu{anim: newPopupAnimator("options")},
		outline:   newOutlineMenu(),
		lists:     newListPopup(),
		devtools:  newDevtoolsPopup(),
		viewer:    newViewerPopup(),
		message:   newMessagePopup(),
		help:      newHelpPopup(),
		confirm:   newConfirmPopup(),
		input:     newInputPopup(),
		toast:     newToast(),
	}
	m.listenBrowser()
	return m
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
	if m.spaceMenu.isActive() || m.options.isActive() || m.lists.isActive() ||
		m.outline.isActive() || m.devtools.isActive() {
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
			&m.spaceMenu, &m.options, &m.outline, &m.lists, &m.devtools, &m.viewer, &m.message,
			&m.help, &m.confirm, &m.input, &m.toast} {
			p.setSize(m.w, m.h)
		}
		m.relayoutTabs()
		if first {
			return m, m.firstFrame()
		}
		return m, nil

	case AnimTickMsg:
		return m, tea.Batch(
			m.spaceMenu.anim.tick(msg), m.options.anim.tick(msg), m.outline.anim.tick(msg),
			m.lists.anim.tick(msg), m.devtools.anim.tick(msg), m.devtools.detail.anim.tick(msg),
			m.viewer.anim.tick(msg), m.message.anim.tick(msg),
			m.help.anim.tick(msg), m.confirm.anim.tick(msg), m.input.anim.tick(msg), m.toast.anim.tick(msg))

	case devTickMsg:
		// The popup redraws from the tab's log while it is open, and the
		// tick re-arms itself only for as long as that is.
		if msg.gen != m.devtools.tickGen || !m.devtools.isActive() {
			return m, nil
		}
		if _, t := m.tabByID(m.devtools.tabID); t != nil {
			m.devtools.refresh(t.dev)
		}
		return m, m.devtools.tick()

	case storageMsg:
		if m.devtools.isActive() && msg.tabID == m.devtools.tabID {
			m.devtools.storage.set(msg.data, msg.err)
		}
		return m, nil

	case bodyMsg:
		if m.devtools.detail.isActive() && msg.tabID == m.devtools.tabID && string(m.devtools.detail.entry.ID) == msg.id {
			m.devtools.detail.setBody(msg.body, msg.err)
		}
		return m, nil

	case evalMsg:
		_, t := m.tabByID(msg.tabID)
		if t == nil {
			return m, nil
		}
		t.dev.Add(msg.entry)
		if m.devtools.isActive() && m.devtools.tabID == msg.tabID {
			m.devtools.refresh(t.dev)
			m.devtools.console.cursor = max(0, len(m.devtools.console.entries)-1)
		}
		return m, nil

	case dialogMsg:
		return m.askDialog(msg)

	case newTargetMsg:
		// A window the page opened becomes a tab at the end of the list, and
		// panel [3] switches to it, the way Chrome does (ux.md §6).
		t := m.newTabFor(msg.id)
		t.url = msg.url
		m.tabs = append(m.tabs, t)
		m.leaveSelect()
		m.shown = len(m.tabs) - 1
		m.cur2 = m.shown
		m.focus = panel3
		return m, tea.Batch(waitEvent(m.events), t.adopt())

	case downloadMsg:
		switch {
		case msg.failed:
			m.downloads = max(0, m.downloads-1)
			return m, tea.Batch(waitEvent(m.events), m.toast.show("download cancelled", toastError))
		case msg.done:
			m.downloads = max(0, m.downloads-1)
			return m, tea.Batch(waitEvent(m.events), m.toast.show("saved "+msg.path, toastInfo))
		}
		m.downloads++
		return m, tea.Batch(waitEvent(m.events), m.toast.show("downloading "+msg.name, toastInfo))

	case authMsg:
		// The name first, the password second (masked); the request waits.
		m.auth, m.authUser = &msg, ""
		where := msg.origin
		if msg.realm != "" {
			where += " — " + msg.realm
		}
		return m, tea.Batch(waitEvent(m.events), m.input.ask(inputPopup{
			title: "Sign in", glyph: glyphPencil, prompt: where + " asks for a name", accept: "next",
			action: inputAuthUser}, m.layer()))

	case fileMsg:
		m.upload = &msg
		prompt := "path of the file to upload"
		if msg.multiple {
			prompt = "paths of the files to upload, separated by spaces"
		}
		return m, tea.Batch(waitEvent(m.events), m.input.ask(inputPopup{
			title: "Upload", glyph: glyphPencil, prompt: prompt, accept: "upload",
			action: inputFile, placeholder: "~/…"}, m.layer()))

	case toastExpireMsg:
		return m, m.toast.expire(msg)

	case clipboardDoneMsg:
		if msg.err != nil {
			return m, m.toast.show(clipboardFailure(msg.err), toastError)
		}
		return m, m.toast.show("copied "+plural(msg.lines, "line"), toastInfo)

	case actionErrMsg:
		return m, m.toast.show(msg.err.Error(), toastError)

	case navFailMsg:
		_, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return m, nil
		}
		t.loading = false
		if errors.Is(msg.err, page.ErrNoEntry) {
			return m, m.toast.show("nothing to go "+msg.what+" to", toastInfo)
		}
		return m, m.toast.show(msg.what+": "+msg.err.Error(), toastError)

	case sourceMsg:
		return m, m.viewer.show(glyphInfo, "Source · "+oneLine(nameOr(msg.title, "page")), msg.html, msg.layer)

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
		// A page with a dialog up answers no CDP call that touches it; the
		// capture would only time out. It is re-asked once the dialog is.
		if m.dialog != nil && m.dialog.tabID == t.id {
			return m, nil
		}
		return m, t.refresh()

	case pageMsg:
		i, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return m, nil
		}
		// Selection mode holds the page still (ux.md §1): the capture is
		// kept and applied when the mode ends.
		if m.sel.on && i == m.shown {
			t.frozen = &msg
			return m, nil
		}
		t.apply(msg, m.pageW())
		t.scrollToCursor(m.pageVisible())
		m.recordVisit(t)
		if i == m.shown && t.certErr && !t.certAsked {
			t.certAsked = true
			return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Certificate error",
				lines: []string{"Chromium does not trust this site's certificate.", fitURL(t.url, 60),
					"Continue anyway, for this tab, for as long as it is open?"},
				accept: "continue", warn: true, action: confirmCert, tabID: t.id}, m.layer())
		}
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
	if m.browser != nil {
		ctx, dir := m.browser.Ctx, m.downloadDir()
		cmds = append(cmds, func() tea.Msg {
			if err := page.SetDownloads(ctx, dir); err != nil {
				return actionErrMsg{err: fmt.Errorf("downloads: %w", err)}
			}
			return nil
		})
	}
	return tea.Batch(cmds...)
}

// downloadDir is config.yaml's download_dir, or ~/Downloads (ui.md §6).
func (m AppModel) downloadDir() string {
	dir := m.cfg.DownloadDir
	if dir == "" {
		dir = "~/Downloads"
	}
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	return dir
}

// relayoutTabs re-lays every tab whose layout is for another width.
func (m *AppModel) relayoutTabs() {
	for i, t := range m.tabs {
		if t.layW != m.pageW() {
			t.relayout(m.pageW())
			t.scrollToCursor(m.pageVisible())
			if m.sel.on && i == m.shown {
				m.sel.resize(t)
			}
		}
	}
}

// ---------------------------------------------------------------- dialogs

// askDialog puts a page's question on screen (function.md §5): alert,
// confirm and beforeunload are a confirm popup, prompt is the input popup.
// The dialog is remembered so that Esc, too, sends an answer — a page left
// waiting on a dialog nobody answers is a page that has stopped.
func (m AppModel) askDialog(msg dialogMsg) (tea.Model, tea.Cmd) {
	_, t := m.tabByID(msg.tabID)
	if t == nil {
		return m, tea.Batch(waitEvent(m.events), m.answerDialogOn(msg.tabID, false, ""))
	}
	m.dialog = &msg
	title := "The page says"
	lines := []string{msg.message}
	accept := "ok"
	switch msg.kind {
	case cdppage.DialogTypeConfirm:
		title = "The page asks"
		accept = "yes"
	case cdppage.DialogTypeBeforeunload:
		title = "Leave this page?"
		lines = []string{"The page says it has unsaved changes."}
		accept = "leave"
	case cdppage.DialogTypePrompt:
		return m, tea.Batch(waitEvent(m.events), m.input.ask(inputPopup{
			title: "The page asks", glyph: glyphPencil, prompt: msg.message,
			value: msg.defaultPrompt, accept: "answer", action: inputPrompt}, m.layer()))
	}
	if strings.TrimSpace(msg.message) == "" {
		lines = []string{"(no message)"}
	}
	return m, tea.Batch(waitEvent(m.events), m.confirm.ask(confirmPopup{glyph: glyphInfo, title: title,
		lines: lines, accept: accept, action: confirmDialog, tabID: msg.tabID}, m.layer()))
}

// answerDialog sends the pending dialog its answer and forgets it. An
// alert has only one answer, so a cancel on it still accepts.
func (m *AppModel) answerDialog(accept bool, text string) tea.Cmd {
	d := m.dialog
	if d == nil {
		return nil
	}
	m.dialog = nil
	if d.kind == cdppage.DialogTypeAlert {
		accept = true
	}
	return m.answerDialogOn(d.tabID, accept, text)
}

func (m AppModel) answerDialogOn(tabID int, accept bool, text string) tea.Cmd {
	_, t := m.tabByID(tabID)
	if t == nil {
		return nil
	}
	return t.act(func(ctx context.Context) error { return page.HandleDialog(ctx, accept, text) })
}

// ---------------------------------------------------------- selection

// enterSelect starts selection mode on the shown tab, typing a search at
// once when asked (ux.md §1: `/` enters the mode and opens the search).
func (m *AppModel) enterSelect(typing bool) tea.Cmd {
	t := m.shownTab()
	if t == nil || t.root == nil {
		return m.toast.show("no page to select from", toastInfo)
	}
	m.focus = panel3
	m.sel.enter(t, typing)
	return nil
}

// leaveSelect ends the mode: the item cursor lands nearest the character
// cursor, and a capture held back while it was on is applied now.
func (m *AppModel) leaveSelect() {
	if !m.sel.on {
		return
	}
	m.sel.on = false
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

func (m AppModel) selectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := m.shownTab()
	if t == nil {
		m.sel.on = false
		return m, nil
	}
	res, text := m.sel.key(msg, m.pageVisible())
	// Keep the character cursor on screen.
	if m.sel.row < t.top {
		t.top = m.sel.row
	}
	if m.sel.row >= t.top+m.pageVisible() {
		t.top = m.sel.row - m.pageVisible() + 1
	}
	switch res {
	case selLeave:
		m.leaveSelect()
	case selYank:
		if text == "" {
			return m, m.toast.show("nothing to copy", toastInfo)
		}
		return m, copyToClipboard(text)
	case selClick:
		i := t.lay.itemAtCol(m.sel.row, m.sel.col)
		if i < 0 {
			return m, m.toast.show("nothing to click here", toastInfo)
		}
		t.cursor = i
		m.leaveSelect()
		return m.enterItem()
	}
	return m, nil
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
	return m.spaceMenu.isActive() || m.options.isActive() || m.outline.isActive() ||
		m.lists.isActive() || m.devtools.isActive() || m.viewer.isActive() || m.message.isActive() ||
		m.help.isActive() || m.confirm.isActive() || m.input.isActive()
}

// typing reports whether a float is taking text: every printable key is a
// character then (§4.5). A search being typed in selection mode counts.
func (m AppModel) typing() bool {
	return m.input.anim.owns() || (m.lists.anim.owns() && m.lists.typing) ||
		(m.devtools.anim.owns() && m.devtools.typing) ||
		(m.sel.on && m.sel.typing && !m.popupOpen())
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc is one role, resolved in one place: close the topmost float (§4.3).
	// With nothing up it belongs to selection mode when that is on, and
	// otherwise does nothing — the previous page is P (ux.md §A.0.K).
	if msg.Type == tea.KeyEscape {
		if m.sel.on && !m.popupOpen() {
			return m.selectKey(msg)
		}
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
	case m.outline.anim.owns():
		return m.outlineKey(msg)
	case m.lists.anim.owns():
		return m.listKey(msg)
	case m.devtools.anim.owns():
		return m.devtoolsKey(msg)
	case m.viewer.anim.owns():
		m.viewer.update(msg)
		return m, nil
	case m.message.anim.owns():
		// The cheatsheet passes its keys through (messagePopup.passKeys):
		// the sheet closes and the key runs, one step.
		if m.message.passKeys && m.sel.on {
			closeCmd := m.message.close()
			mm, cmd := m.selectKey(msg)
			return mm, tea.Batch(closeCmd, cmd)
		}
		return m, nil
	case m.help.anim.owns():
		m.help.update(msg)
		return m, nil
	case m.spaceMenu.anim.owns():
		return m.menuKey(msg)
	}
	if m.sel.on {
		// The mode holds the keyboard (ux.md §1): Space is its cheatsheet,
		// everything else is its own.
		if msg.Type == tea.KeySpace && !m.sel.typing {
			return m, m.message.show(glyphMenu, "Selection mode", selectCheatsheet, true, m.layer())
		}
		return m.selectKey(msg)
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
		// Cancelling a page's prompt is an answer too: "no".
		switch m.input.action {
		case inputPrompt:
			return m, tea.Batch(m.input.close(), m.answerDialog(false, ""))
		case inputAuthUser, inputAuthPass:
			return m, tea.Batch(m.input.close(), m.cancelAuth())
		case inputFile:
			m.upload = nil // the chooser is simply left unanswered: nothing is chosen
		}
		return m, m.input.close()
	case m.confirm.anim.owns():
		if m.confirm.action == confirmDialog {
			return m, tea.Batch(m.confirm.close(), m.answerDialog(false, ""))
		}
		return m, m.confirm.close()
	case m.options.anim.owns():
		return m, m.options.close()
	case m.outline.anim.owns():
		return m, m.outline.close()
	case m.lists.anim.owns():
		// A filter being typed is the innermost thing Esc can drop.
		if m.lists.escTyping() {
			return m, nil
		}
		return m, m.lists.close()
	case m.devtools.anim.owns():
		// Innermost first: the detail, then a filter being typed, then the
		// popup.
		if m.devtools.detail.anim.owns() {
			return m, m.devtools.detail.close()
		}
		if m.devtools.escTyping() {
			return m, nil
		}
		return m, m.devtools.close()
	case m.viewer.anim.owns():
		return m, m.viewer.close()
	case m.message.anim.owns():
		return m, m.message.close()
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
		m.outline.close(), m.lists.close(), m.devtools.close(), m.viewer.close(), m.message.close(),
		m.help.close(), m.spaceMenu.close())
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
		// A download in flight would be cut off (ux.md §5): ask first.
		if m.downloads > 0 {
			return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Quit",
				lines:  []string{plural(m.downloads, "download") + " still in progress.", "Quitting stops it."},
				accept: "quit", warn: true, action: confirmQuit}, m.layer())
		}
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
	case "/":
		return m, m.enterSelect(true)
	case "v":
		// The way into the text itself: the item cursor stops only on
		// items, and a paragraph is reached by character (ux.md §1). vim's
		// letter, and it reads the same from any panel, like /.
		return m, m.enterSelect(false)
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
		case "w", "c", "r", "y", "T", "X", "U":
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
		case "R", "U", "Y", "A", "O", "Z", "V", "D", "W":
			return m.dispatch(k)
		}
	}
	return m, nil
}

// ---------------------------------------------------------------- devtools

// openDevtools shows the popup for the shown tab, on the Storage tab, and
// fetches the storage.
func (m *AppModel) openDevtools() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return m.toast.show("no page for DevTools", toastInfo)
	}
	m.devtools.refresh(t.dev)
	return tea.Batch(m.devtools.open(t.id, m.layer()), m.fetchStorage())
}

func (m AppModel) fetchStorage() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	return m.storageThen(t, nil)
}

// storageThen runs fn on the tab and then re-reads the storage, in ONE
// command. Not tea.Sequence(act, fetch): a nested Sequence is not waited
// for by the outer one — Bubble Tea hands the inner list back as a message
// and moves on — so the fetch would race the change it is meant to show.
func (m AppModel) storageThen(t *tab, fn func(context.Context) error) tea.Cmd {
	ctx, url, id := t.ctx, t.url, t.id
	return func() tea.Msg {
		if fn != nil {
			if err := fn(ctx); err != nil {
				return actionErrMsg{err: err}
			}
		}
		d, err := page.StorageFor(ctx, url)
		return storageMsg{tabID: id, data: d, err: err}
	}
}

func (m AppModel) devtoolsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action, text := m.devtools.update(msg)
	t := m.shownTab()
	if action == devNone || t == nil {
		return m, nil
	}
	filter := m.devtools.filter[m.devtools.tab]
	switch action {
	case devFetchStorage:
		return m, m.fetchStorage()
	case devYank:
		return m, copyToClipboard(text)
	case devDeleteCookie:
		r, _ := m.devtools.storage.current(filter)
		c := r.cookie
		return m, m.storageThen(t, func(ctx context.Context) error { return page.DeleteCookie(ctx, c) })
	case devDeleteItem:
		r, _ := m.devtools.storage.current(filter)
		origin, local, key := m.devtools.storage.data.Origin, r.local, r.key
		return m, m.storageThen(t, func(ctx context.Context) error { return page.RemoveStorageItem(ctx, origin, local, key) })
	case devClearSite:
		origin := m.devtools.storage.data.Origin
		if origin == "" {
			return m, m.toast.show("no origin to clear", toastInfo)
		}
		return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Clear site data",
			lines:  []string{origin, "Cookies, storage, caches — everything Chromium keeps for it.", "Logins there will be gone."},
			accept: "clear", warn: true, action: confirmClearSite}, m.layer()+1)
	case devClearNet:
		t.dev.ClearNet()
		m.devtools.refresh(t.dev)
	case devClearConsole:
		t.dev.ClearConsole()
		m.devtools.refresh(t.dev)
	case devDetail:
		e, _ := m.devtools.network.current(filter)
		ctx, id, reqID := t.ctx, t.id, e.ID
		fetch := func() tea.Msg {
			body, err := page.ResponseBody(ctx, reqID)
			return bodyMsg{tabID: id, id: string(reqID), body: body, err: err}
		}
		return m, tea.Batch(m.devtools.detail.show(e, m.layer()+1), fetch)
	case devEval:
		return m, m.openEvalPrompt()
	}
	return m, nil
}

// openEvalPrompt is the console's prompt (ui.md §3.2 Eval): an input popup
// over the DevTools popup. It stays open after each run, so the console is
// a REPL — Esc is how it ends.
func (m *AppModel) openEvalPrompt() tea.Cmd {
	return m.input.ask(inputPopup{title: "Console", glyph: ">", prompt: "JavaScript, run in the page (Esc ends)",
		accept: "run", action: inputEval}, m.layer()+1)
}

// evalMsg is the console's answer to one expression.
type evalMsg struct {
	tabID int
	entry page.ConsoleEntry
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
			menuItem{label: "Undo close", key: "U", hint: "reopen the last closed tab", disabled: len(m.closed) == 0})
	case panel3:
		title = "Page"
		items = m.pageMenuItems()
	}
	m.spaceMenu.setItems(items, title, 1)
	return m, m.spaceMenu.open()
}

// optionsKind says what the options menu is showing.
type optionsKind int

const (
	optItemMenu optionsKind = iota // an item's operations (Enter on [3])
	optSelect                      // a <select>'s options, keyed by index
	optAddTo                       // Bookmarks / Shortcuts
)

// itemMenuItems is an item's operations by role (menu-only, no letters —
// ux.md §A.1). The first row is the item's main action, so Enter twice
// does the obvious thing: open a link, press a button, edit a field.
// folded is a landmark's state, which decides which way its row reads.
func itemMenuItems(n *ir.Node, folded bool) []menuItem {
	var items []menuItem
	switch n.Kind {
	case ir.Landmark:
		if folded {
			items = append(items, menuItem{label: "Expand", key: "fold", hint: "show what is inside"})
		} else {
			items = append(items, menuItem{label: "Collapse", key: "fold", hint: "one line, out of the way"})
		}
	case ir.Link:
		items = append(items,
			menuItem{label: "Open", key: "click", hint: "click it"},
			menuItem{label: "Open in new tab", key: "newtab", hint: "and switch to it"},
			// "link url", not "url": with the cursor on a link, a bare "Yank
			// url" reads as the page's, which is [Y] in the panel region.
			menuItem{label: "Yank link url", key: "yankurl", hint: oneLine(n.URL)})
	case ir.Button, ir.Check:
		items = append(items, menuItem{label: "Click", key: "click", hint: "press it"})
	case ir.Media:
		items = append(items,
			menuItem{label: "Click", key: "click", hint: "the page decides"},
			menuItem{label: "Yank media url", key: "yankurl", hint: oneLine(n.URL), disabled: n.URL == ""})
	case ir.Textbox:
		if n.Value == "" {
			items = append(items, menuItem{label: "Edit", key: "edit", hint: "type a value"})
		} else {
			items = append(items, textboxItems(n)...)
		}
	case ir.Combobox:
		items = append(items, menuItem{label: "Choose", key: "choose", hint: "pick an option"})
	case ir.Heading:
		items = append(items, menuItem{label: "Fold section", key: "fold", hint: "not in this build yet", disabled: true})
	case ir.Unsupported:
		items = append(items,
			menuItem{label: "role: " + n.Role + ", not supported yet — only click", key: "unsupported", disabled: true},
			menuItem{label: "Click", key: "click", hint: "the page decides"})
	}
	return append(items,
		menuItem{label: "Yank text", key: "yanktext", hint: "what it says"},
		menuItem{label: "Inspect", key: "inspect", hint: "role, name, node id"})
}

// pageMenuItems is panel [3]'s Space menu: the item's operations, then the
// page's — the whole of what can be done here (ux.md §A.1).
func (m AppModel) pageMenuItems() []menuItem {
	var items []menuItem
	t := m.shownTab()
	if n := t.current(); t != nil && n != nil {
		items = append(items, menuItem{header: true, label: "item operation"})
		items = append(items, itemMenuItems(n, t.lay.items[t.cursor].folded)...)
		items = append(items,
			menuItem{separator: true},
			menuItem{header: true, label: "panel operation"})
	}
	items = append(items,
		menuItem{label: "Reload", key: "R", hint: "this page", disabled: t == nil},
		menuItem{label: "Previous", key: "P", hint: "back in this tab", disabled: t == nil},
		menuItem{label: "Next", key: "N", hint: "forward in this tab", disabled: t == nil},
		menuItem{label: "Search", key: "/", hint: "find text on the page", disabled: t == nil},
		menuItem{label: "Select text", key: "select", hint: "walk by character, copy some", disabled: t == nil},
		menuItem{label: "URL", key: "U", hint: "go to one"},
		menuItem{label: "Add to…", key: "A", hint: "Bookmarks or Shortcuts", disabled: t == nil},
		menuItem{label: "Outline", key: "O", hint: "landmarks and headings", disabled: t == nil},
		menuItem{label: "DevTools", key: "D", hint: "storage, network, console", disabled: t == nil},
		menuItem{label: "Zoom", key: "Z", hint: "the page alone, or the grid back"},
		menuItem{label: "View source", key: "V", hint: "the HTML as it is now", disabled: t == nil},
		menuItem{label: "Yank page url", key: "Y", hint: "to the clipboard", disabled: t == nil},
		menuItem{label: "W close", key: "W", hint: "this tab", disabled: t == nil})
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
	if i := m.options.cursor; i < len(m.options.items) && m.options.items[i].disabled && m.options.items[i].key == key {
		return m, m.toast.show(m.options.items[i].hint, toastInfo)
	}
	switch m.optionsKind {
	case optAddTo:
		closeCmd := m.options.close()
		switch key {
		case "b":
			return m, tea.Batch(closeCmd, m.addEntry(listBookmarks, t.title, t.url))
		case "s":
			return m, tea.Batch(closeCmd, m.addEntry(listShortcuts, t.title, t.url))
		}
		return m, closeCmd
	case optSelect:
		idx, err := strconv.Atoi(key)
		if n == nil || err != nil || idx < 0 || idx >= len(n.Children) {
			return m, m.options.close()
		}
		opt := n.Children[idx]
		return m, tea.Batch(m.closeStack(), t.act(func(ctx context.Context) error { return page.Choose(ctx, opt.ID) }))
	}
	// An item's operation. Close BEFORE dispatching: dispatch returns its
	// own copy of the model, and a close applied to this one afterwards
	// would be applied to a model nobody returns. Choose keeps the float
	// open, swapping it for the option list.
	if key == "choose" {
		return m.chooseOptions()
	}
	closeCmd := m.options.close()
	mm, cmd := m.dispatch(key)
	return mm, tea.Batch(closeCmd, cmd)
}

// ---------------------------------------------------------------- actions

// busy reports whether the shown page is on its way somewhere: a
// navigation or history move has been asked for and has not landed.
func (m AppModel) busy() bool {
	t := m.shownTab()
	return t != nil && t.loading
}

// dispatch runs one action by its key — the same function whether the key
// was pressed on the panel or chosen from the menu, so the two cannot drift.
func (m AppModel) dispatch(key string) (tea.Model, tea.Cmd) {
	t := m.shownTab()
	// While the page is on its way, the keys that would act on it — or
	// stack another move on the one in flight — are swallowed (ux.md §6):
	// P pressed three times while the first back is still answering is
	// one back, not three. The dimmed page says why nothing happened.
	if m.busy() {
		switch key {
		case "back", "forward", "R", "click", "submit", "edit", "clear", "choose":
			return m, nil
		}
	}
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
				m.remember(m.tabs[i])
				m.tabs[i].close()
				m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
			}
		}
		m.cur2, m.shown = 0, 0
		return m, nil
	case "U":
		if m.focus == panel2 || m.spaceMenu.isActive() {
			if n := len(m.closed); n > 0 {
				last := m.closed[n-1]
				m.closed = m.closed[:n-1]
				return m, m.openTab(last.URL, true)
			}
			return m, m.toast.show("nothing closed yet", toastInfo)
		}
		return m, m.input.ask(inputPopup{title: "Go to", glyph: glyphSearch,
			prompt: "URL, or words to search for", accept: "open", action: inputGoto}, m.layer())

	// ---- panel [3], page
	case "R":
		if t != nil {
			return m, t.load(t.url)
		}
	case "back":
		if t != nil {
			return m, t.navigate("back", page.Back)
		}
	case "forward":
		if t != nil {
			return m, t.navigate("forward", page.Forward)
		}
	case "Y":
		if t != nil {
			return m, copyToClipboard(t.url)
		}
	case "O":
		return m, m.openOutline()
	case "D":
		return m, m.openDevtools()
	case "W":
		// The page's own close: the tab [3] is showing, wherever [2]'s
		// cursor is. Its lowercase twin in [2] closes the cursor's tab.
		if t != nil {
			return m.closeTab(m.shown)
		}
	case "Z":
		m.zoom = !m.zoom
		m.relayoutTabs()
		return m, nil
	case "V":
		if t != nil {
			ctx, title := t.ctx, t.title
			layer := m.layer()
			return m, func() tea.Msg {
				src, err := page.Source(ctx)
				if err != nil {
					return actionErrMsg{err: fmt.Errorf("view source: %w", err)}
				}
				return sourceMsg{title: title, html: src, layer: layer}
			}
		}
	case "/":
		return m, m.enterSelect(true)
	case "select":
		return m, m.enterSelect(false)
	case "A":
		if t != nil {
			m.optionsFor, m.optionsKind = nil, optAddTo
			m.options.setItems([]menuItem{
				{label: "Bookmarks", key: "b", hint: "the tree you keep"},
				{label: "Shortcuts", key: "s", hint: "the few you reach for"},
			}, "Add to…", m.layer())
			return m, m.options.open()
		}

	// ---- panel [3], item
	case "enter":
		return m.enterItem()
	case "click":
		if n := t.current(); n != nil {
			id := n.ID
			return m, t.act(func(ctx context.Context) error { return page.Click(ctx, id) })
		}
	case "choose":
		return m.chooseOptions()
	case "fold":
		if t != nil {
			t.toggleFold(m.pageW())
			t.scrollToCursor(m.pageVisible())
		}
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
			return m, m.message.show(glyphInfo, "Inspect", inspectLines(n), false, m.layer())
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

// enterItem is Enter on panel [3]: the item's operations, as a menu of
// their own, first row the main one (ux.md §A.0.K as revised 2026-09-20).
// Space is the whole menu — this region and the page's — so Enter is the
// short way to "what can I do with THIS".
func (m AppModel) enterItem() (tea.Model, tea.Cmd) {
	t := m.shownTab()
	n := t.current()
	if t == nil || n == nil {
		return m, nil
	}
	m.optionsFor, m.optionsKind = n, optItemMenu
	title := oneLine(n.Text())
	if title == "" {
		title = oneLine(nameOr(n.Name, n.Kind.String()))
	}
	m.options.setItems(itemMenuItems(n, t.lay.items[t.cursor].folded), truncate(title, 40), m.layer())
	return m, m.options.open()
}

// chooseOptions lists a select's options in the options menu, cursor on
// the current one; picking one sets it (ux.md §2.4).
func (m AppModel) chooseOptions() (tea.Model, tea.Cmd) {
	t := m.shownTab()
	n := t.current()
	if t == nil || n == nil || n.Kind != ir.Combobox {
		return m, m.options.close()
	}
	items := make([]menuItem, 0, len(n.Children))
	for i, o := range n.Children {
		hint := ""
		if o.Selected {
			hint = "current"
		}
		items = append(items, menuItem{label: oneLine(o.Name), key: strconv.Itoa(i), hint: hint})
	}
	if len(items) == 0 {
		return m, tea.Batch(m.options.close(), m.toast.show("no options to choose from", toastInfo))
	}
	m.optionsFor, m.optionsKind = n, optSelect
	m.options.setItems(items, oneLine(nameOr(n.Name, "choose")), m.layer())
	for i, o := range n.Children {
		if o.Selected {
			m.options.cursor = i
		}
	}
	if m.options.isActive() {
		return m, nil // swapped in place under the open float
	}
	return m, m.options.open()
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
	case inputPrompt:
		return m, tea.Batch(m.input.close(), m.answerDialog(true, value))
	case inputEval:
		// The prompt stays; the expression and, when it comes, its result
		// go to the console list behind it.
		_, dt := m.tabByID(m.devtools.tabID)
		expr := strings.TrimSpace(value)
		if dt == nil || expr == "" {
			return m, nil
		}
		m.input.value = ""
		dt.dev.Add(page.ConsoleEntry{Level: "input", Text: expr})
		m.devtools.refresh(dt.dev)
		m.devtools.console.cursor = max(0, len(m.devtools.console.entries)-1)
		ctx, id := dt.ctx, dt.id
		return m, func() tea.Msg { return evalMsg{tabID: id, entry: page.Eval(ctx, expr)} }
	case inputAuthUser:
		if m.auth == nil {
			return m, m.input.close()
		}
		m.authUser = value
		return m, m.input.ask(inputPopup{title: "Sign in", glyph: glyphPencil,
			prompt: "password for " + value, accept: "sign in", action: inputAuthPass, masked: true}, m.layer())
	case inputAuthPass:
		a := m.auth
		m.auth = nil
		user := m.authUser
		if a == nil {
			return m, m.input.close()
		}
		_, at := m.tabByID(a.tabID)
		if at == nil {
			return m, m.input.close()
		}
		return m, tea.Batch(m.input.close(), at.act(func(ctx context.Context) error { return page.Auth(ctx, a.id, user, value) }))
	case inputFile:
		f := m.upload
		m.upload = nil
		if f == nil {
			return m, m.input.close()
		}
		_, ft := m.tabByID(f.tabID)
		if ft == nil {
			return m, m.input.close()
		}
		var files []string
		for _, p := range strings.Fields(value) {
			files = append(files, expandHome(p))
		}
		if len(files) == 0 {
			return m, m.input.close()
		}
		for _, p := range files {
			if _, err := os.Stat(p); err != nil {
				return m, m.toast.show("no such file: "+p, toastError)
			}
		}
		node := f.node
		return m, tea.Batch(m.input.close(), ft.act(func(ctx context.Context) error { return page.SetFiles(ctx, node, files) }))
	}
	return m, m.closeStack()
}

// expandHome turns a leading ~ into the home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// cancelAuth is Esc on a challenge: the page gets its 401.
func (m *AppModel) cancelAuth() tea.Cmd {
	a := m.auth
	m.auth = nil
	if a == nil {
		return nil
	}
	_, t := m.tabByID(a.tabID)
	if t == nil {
		return nil
	}
	return t.act(func(ctx context.Context) error { return page.CancelAuth(ctx, a.id) })
}

// sourceMsg carries the page's HTML to the viewer.
type sourceMsg struct {
	title, html string
	layer       int
}

// inspectLines is what the Inspect popup shows about a node (ux.md §A.1):
// the facts the IR has, nothing invented.
func inspectLines(n *ir.Node) []string {
	lines := []string{
		"role     " + n.Role + " (" + n.Kind.String() + ")",
		"name     " + oneLine(n.Name),
	}
	if n.Value != "" {
		lines = append(lines, "value    "+oneLine(n.Value))
	}
	if n.URL != "" {
		lines = append(lines, "url      "+n.URL)
	}
	var states []string
	for _, f := range []struct {
		on   bool
		name string
	}{{n.Focusable, "focusable"}, {n.Disabled, "disabled"}, {n.Expanded, "expanded"},
		{n.Selected, "selected"}, {n.Multiline, "multiline"}, {n.Protected, "protected"}} {
		if f.on {
			states = append(states, f.name)
		}
	}
	if n.Kind == ir.Check {
		states = append(states, n.Checked.String())
	}
	if len(states) > 0 {
		lines = append(lines, "state    "+strings.Join(states, ", "))
	}
	return append(lines, "node     #"+itoa(int(n.ID)))
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
	case confirmDialog:
		return m, tea.Batch(closeCmd, m.answerDialog(true, ""))
	case confirmCert:
		_, t := m.tabByID(m.confirm.tabID)
		if t == nil {
			return m, closeCmd
		}
		ctx, url := t.ctx, t.url
		ignore := func() tea.Msg {
			if err := page.IgnoreCertErrors(ctx); err != nil {
				return actionErrMsg{err: err}
			}
			return nil
		}
		return m, tea.Batch(closeCmd, tea.Sequence(ignore, t.load(url)))
	case confirmDeleteEntry:
		return m, tea.Batch(closeCmd, m.deleteEntry(m.confirm.at))
	case confirmClearHistory:
		m.history = nil
		if err := store.ClearHistory(); err != nil {
			return m, tea.Batch(closeCmd, m.toast.show(err.Error(), toastError))
		}
		m.lists.setEntries(nil)
		return m, closeCmd
	case confirmClearSite:
		t := m.shownTab()
		origin := m.devtools.storage.data.Origin
		if t == nil || origin == "" {
			return m, closeCmd
		}
		return m, tea.Batch(closeCmd, m.storageThen(t, func(ctx context.Context) error { return page.ClearSiteData(ctx, origin) }))
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

// remember keeps a closing tab's address for [U]ndo close.
func (m *AppModel) remember(t *tab) {
	if t.url == "" || t.url == "about:blank" {
		return
	}
	m.closed = append(m.closed, store.SessionTab{URL: t.url, Title: t.title})
	if len(m.closed) > 20 {
		m.closed = m.closed[1:]
	}
}

// showTab is Enter on panel [2]: panel [3] switches, and the keyboard goes
// with it (ux.md §6). A pending tab loads now.
func (m AppModel) showTab(i int) (tea.Model, tea.Cmd) {
	m.leaveSelect()
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
	if i == m.shown {
		m.sel.on = false
	}
	m.remember(m.tabs[i])
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
	case m.zoom || (m.narrow() && m.focus == panel3):
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
	if m.outline.isActive() {
		out = overlay.Composite(m.outline.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.devtools.isActive() {
		out = overlay.Composite(m.devtools.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.options.isActive() {
		out = overlay.Composite(m.options.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.viewer.isActive() {
		out = overlay.Composite(m.viewer.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.message.isActive() {
		out = overlay.Composite(m.message.view(), out, overlay.Center, overlay.Center, 0, 0)
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
	p1 := panelChrome(innerW, fitLines(m.panel1Body(innerW), innerW, panel1Rows), "[1] Places", m.focus == panel1)
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
	switch {
	case m.sel.on:
		tone = toneSelect // Yellow: the keyboard is here, doing something else (ux.md §B)
	case m.focus == panel3:
		tone = toneFocus
	}
	return panelFrame(innerW, fitLines(m.pageBody(innerW, innerH), innerW, innerH), "[3] Page", hint, tone)
}

// footer is the mandatory disclosure of the entry keys (§A.1 / §A.2): one
// row, locked (ui.md §5). Selection mode replaces it with only the keys
// that work there (ux.md §B: the footer is honest).
func (m AppModel) footer() string {
	if m.sel.on && !m.popupOpen() {
		return keyLegend(selectLegendPairs(m.sel.typing), m.w)
	}
	return keyLegend([][2]string{{"space", "menu"}, {"?", "help"}, {"tab/1-3", "panels"}, {"q", "quit"}}, m.w)
}
