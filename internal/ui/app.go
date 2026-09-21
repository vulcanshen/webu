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
	"github.com/vulcanshen/webu/internal/paths"
	"github.com/vulcanshen/webu/internal/store"
)

// The two panels, left and right (ui.md §1.1). What used to be a third —
// Bookmarks / Shortcuts / History as a three-row panel — is the header row
// now: those are global popups, and a panel that never changes and is never
// the thing you are doing is a bar, not a panel (revised 2026-09-20).
type panelID int

const (
	panelTabs panelID = iota // [1] the tabs
	panelPage                // [2] the page
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

// screen is which header chip is lit: the web, or one of the lists that
// take the whole body (ui.md §1.1, revised 2026-09-21).
type screen int

const (
	screenWeb screen = iota // [1] Tabs and [2] Page
	screenBookmarks
	screenHistory
	screenDownloads
	screenSettings
)

// screenKeys are the header's letters, global from any screen.
var screenKeys = map[string]screen{"W": screenWeb, "B": screenBookmarks, "H": screenHistory, "D": screenDownloads, "S": screenSettings}

type AppModel struct {
	w, h   int
	screen screen
	focus  panelID
	cur2   int
	top2   int

	tabs      []*tab
	shown     int // index into tabs panel [2] displays; -1 for none
	nextTabID int
	browser   *browser.Browser
	events    chan tea.Msg  // page and browser events, read by waitEvent
	startURLs []string      // what the command line asked for, a tab each (function.md §12)
	session   store.Session // what to restore on the first frame
	// zoom: panel [2] alone fills the screen (ux.md §A.1 [Z]).
	zoom bool
	// sel is selection mode on the shown tab (ux.md §1).
	sel selectMode

	// webu's own files, loaded by main and written back as they change.
	bookmarks []store.Bookmark
	folders   []string // bookmark folders declared on their own (bookmarks.go)
	cfg       store.Config
	history   []store.Visit

	// Floats. The Space menu goes down first; a target opened from it stacks
	// above (§6.4). The toast rides on top of everything.
	spaceMenu spaceMenu
	options   spaceMenu   // a textbox's Submit/Edit/Clear/Yank, or a select's options
	outline   spaceMenu   // the page's landmarks and headings
	lists     listPanel   // the screen behind a header chip after [W]eb
	splash    splashModel // the easter egg (splash.go)
	devtools  devtoolsPopup
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
	// closed is the tabs closed this session, for [U]ndo close in [1].
	closed []store.SessionTab
	// dls is this session's downloads, oldest first: the Downloads screen
	// lists them, the header counts the ones still running.
	dls []download
	// moveRef is the bookmark a Move picker is about; settingRef the row a
	// Settings box is editing; folderParent where a folder being named goes;
	// newBookmark the one being typed in, between its two boxes;
	// foldedFolders the folder rows shut with Enter (bookmarks.go).
	moveRef, settingRef int
	folderParent        string
	newBookmark         store.Bookmark
	foldedFolders       map[string]bool
	// pendingG holds the first half of the gg chord.
	pendingG bool
}

// New builds the app over a running browser. Every start argument — a
// URL, or words to search for, the same reading as the Location box —
// opens as a new tab after whatever the session restores, the first of
// them shown (function.md §12). Empty strings are ignored.
func New(b *browser.Browser, start ...string) AppModel {
	var urls []string
	for _, s := range start {
		if strings.TrimSpace(s) != "" {
			urls = append(urls, strings.TrimSpace(s))
		}
	}
	m := AppModel{
		focus:     panelPage,
		shown:     -1,
		browser:   b,
		events:    make(chan tea.Msg, 16),
		startURLs: urls,
		spaceMenu: newSpaceMenu(),
		splash:    newSplashModel(),
		options:   spaceMenu{anim: newPopupAnimator("options")},
		outline:   newOutlineMenu(),
		lists:     newListPanel(),
		devtools:  newDevtoolsPopup(),
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
func (m AppModel) WithStore(bookmarks []store.Bookmark, folders []string, cfg store.Config, history []store.Visit) AppModel {
	m.bookmarks, m.folders, m.cfg, m.history = bookmarks, folders, cfg, history
	return m
}

// WithSession is what was open last time; restored on the first frame,
// without loading (ux.md §6).
func (m AppModel) WithSession(s store.Session) AppModel {
	m.session = s
	return m
}

// Session is what to write down on the way out: every tab's URL, and which
// one panel [2] was on.
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
func (m AppModel) panelH() int  { return m.h - 3 } // the header row, its rule, and the footer row
func (m AppModel) layer() int {
	if m.spaceMenu.isActive() || m.options.isActive() ||
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
			&m.spaceMenu, &m.options, &m.outline, &m.lists, &m.devtools, &m.message,
			&m.help, &m.confirm, &m.input, &m.toast} {
			p.setSize(m.w, m.h)
		}
		m.relayoutTabs()
		if first {
			return m, m.firstFrame()
		}
		return m, nil

	case splashTickMsg, splashIdentityMsg, splashHintMsg:
		var cmd tea.Cmd
		m.splash, cmd = m.splash.update(msg)
		return m, cmd

	case AnimTickMsg:
		return m, tea.Batch(
			m.spaceMenu.anim.tick(msg), m.options.anim.tick(msg), m.outline.anim.tick(msg),
			m.devtools.anim.tick(msg), m.devtools.detail.anim.tick(msg),
			m.message.anim.tick(msg),
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
		// panel [2] switches to it, the way Chrome does (ux.md §6).
		t := m.newTabFor(msg.id)
		t.url = msg.url
		m.tabs = append(m.tabs, t)
		m.leaveSelect()
		m.shown = len(m.tabs) - 1
		m.cur2 = m.shown
		m.focus = panelPage
		return m, tea.Batch(waitEvent(m.events), t.adopt())

	case downloadMsg:
		return m, tea.Batch(waitEvent(m.events), m.noteDownload(msg))

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
			title: "Upload", glyph: glyphPencil, prompt: prompt + " (~ is home)",
			accept: "upload", action: inputFile}, m.layer()))

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

	case devSourceMsg:
		if m.devtools.isActive() && msg.tabID == m.devtools.tabID {
			m.devtools.source.set(msg.html, msg.err)
		}
		return m, nil

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
		fresh := msg.url != "" && msg.url != t.lastVisit
		t.apply(msg, m.pageW())
		if !fresh {
			t.scrollToCursor(m.pageVisible())
		}
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
// except the one that was showing; and the command line's URLs, a new tab
// each, the first of them in front, when there are any.
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
	if len(m.startURLs) > 0 {
		for i, u := range m.startURLs {
			cmds = append(cmds, m.openTab(m.resolveURL(u), i == 0))
		}
	} else if t := m.shownTab(); t != nil && t.pending {
		cmds = append(cmds, t.load(t.url))
	}
	if m.browser != nil {
		cmds = append(cmds, m.pointDownloads())
	}
	return tea.Batch(cmds...)
}

// pointDownloads tells the browser where downloads go (ui.md §6): on the
// first frame, and again when the setting changes.
func (m AppModel) pointDownloads() tea.Cmd {
	if m.browser == nil {
		return nil
	}
	ctx, dir := m.browser.Ctx, m.downloadDir()
	return func() tea.Msg {
		// Chromium does not create the directory it is pointed at.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return actionErrMsg{err: fmt.Errorf("downloads: %w", err)}
		}
		if err := page.SetDownloads(ctx, dir); err != nil {
			return actionErrMsg{err: fmt.Errorf("downloads: %w", err)}
		}
		return nil
	}
}

// downloadDir is config.yaml's download_dir, or ~/.webu/datas/downloads
// (ui.md §6; revised 2026-09-21 — it was ~/Downloads). ~ is expanded.
func (m AppModel) downloadDir() string {
	dir := m.cfg.DownloadDir
	if dir == "" {
		if d, err := paths.Downloads(); err == nil {
			dir = d
		} else {
			dir = "~/Downloads"
		}
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
	m.focus = panelPage
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
		m.devtools.isActive() || m.message.isActive() ||
		m.help.isActive() || m.confirm.isActive() || m.input.isActive()
}

// floatOwned reports whether some float still holds the keyboard — not
// merely is on screen: one that is closing has let go (§6.2), and an Esc
// that arrived during its animation belongs to whatever is under it.
func (m AppModel) floatOwned() bool {
	return m.toast.anim.owns() || m.input.anim.owns() || m.confirm.anim.owns() ||
		m.options.anim.owns() || m.outline.anim.owns() || m.devtools.anim.owns() ||
		m.message.anim.owns() || m.help.anim.owns() || m.spaceMenu.anim.owns()
}

// typing reports whether a float is taking text: every printable key is a
// character then (§4.5). A search being typed in selection mode counts.
func (m AppModel) typing() bool {
	return m.input.anim.owns() || (m.screen != screenWeb && m.lists.typing) ||
		(m.devtools.anim.owns() && m.devtools.typing) ||
		(m.sel.on && m.sel.typing && !m.popupOpen())
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The easter-egg splash owns the keyboard until dismissed — any key
	// closes it, and nothing underneath sees the press.
	if m.splash.isActive() {
		var cmd tea.Cmd
		m.splash, cmd = m.splash.update(msg)
		return m, cmd
	}
	// Esc is one role, resolved in one place: close the topmost float (§4.3).
	// With nothing up it belongs to selection mode when that is on, and
	// otherwise does nothing — the previous page is P (ux.md §A.0.K).
	if msg.Type == tea.KeyEscape {
		switch {
		case m.floatOwned():
			return m.closeTop()
		case m.screen != screenWeb:
			// A screen is not a float, but Esc unwinds it the same way: the
			// filter first, then back to the web it was opened from.
			if !m.lists.escTyping() {
				m.screen = screenWeb
			}
			return m, nil
		case m.sel.on:
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
	case m.devtools.anim.owns():
		return m.devtoolsKey(msg)
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
	if m.screen != screenWeb {
		return m.screenKey(msg)
	}
	if m.sel.on {
		// The mode holds the keyboard (ux.md §1): Space is its cheatsheet,
		// everything else is its own.
		if msg.Type == tea.KeySpace && !m.sel.typing {
			return m, m.message.show(glyphMenu, "Visual mode", selectCheatsheet, true, m.layer())
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
		m.outline.close(), m.devtools.close(), m.message.close(),
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
		m.focus = (m.focus + 1) % 2
		return m, nil
	case "1", "2":
		m.focus = panelID(k[0] - '1')
		return m, nil
	case "q":
		return m.askQuit()
	case "W", "B", "H", "D", "S":
		return m.switchScreen(k)
	case "L":
		return m.dispatch("L")
	case "P":
		return m.dispatch("back")
	case "N":
		return m.dispatch("forward")
	case " ":
		return m.openMenu()
	case "/":
		return m, m.enterSelect(true)
	case "V":
		// The family's easter egg, on the family's key (splash.go).
		return m, m.splash.show()
	case "v":
		// Visual mode, the way into the text itself: the item cursor stops
		// only on items, and a paragraph is reached by character (ux.md
		// §1). Lower case as the one exception to "panel operations are
		// upper case" (revised 2026-09-21): V is the family's egg, and a
		// mode is not a panel operation. It reads the same from any panel,
		// like /.
		return m, m.enterSelect(false)
	}

	switch m.focus {
	case panelTabs:
		if navKeys[k] {
			m.cur2 = moveCursor(m.cur2, len(m.tabs), k, m.panelH()-2)
			return m, nil
		}
		switch k {
		case "enter":
			return m.dispatch("show")
		case "w", "c", "r", "y", "T", "X", "U":
			return m.dispatch(k)
		}
	case panelPage:
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
		case "R", "T", "Y", "A", "O", "Z", "I", "C":
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
	var fetch tea.Cmd
	switch m.devtools.tab {
	case devStorage:
		fetch = m.fetchStorage()
	case devSource:
		fetch = m.fetchSource()
	}
	return tea.Batch(m.devtools.open(t.id, m.layer()), fetch)
}

// fetchSource reads the shown page's HTML for the Source tab.
func (m *AppModel) fetchSource() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	m.devtools.source.lines = nil
	ctx, id := t.ctx, t.id
	return func() tea.Msg {
		html, err := page.Source(ctx)
		return devSourceMsg{tabID: id, html: html, err: err}
	}
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
	case devFetchSource:
		return m, m.fetchSource()
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
	case devConsoleDetail:
		e, _ := m.devtools.console.current(filter)
		text := e.Text
		if e.Detail != "" {
			text = e.Detail // the whole object, not its one-line preview
		}
		return m, m.devtools.detail.showText("console · "+e.Level, detailHead(e), text, m.layer()+1)
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

// askQuit is q: a download in flight would be cut off (ux.md §5), so it
// asks first; otherwise it goes.
func (m AppModel) askQuit() (tea.Model, tea.Cmd) {
	if n := m.downloading(); n > 0 {
		return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Quit",
			lines:  []string{plural(n, "download") + " still in progress.", "Quitting stops it."},
			accept: "quit", warn: true, action: confirmQuit}, m.layer())
	}
	return m.quit()
}

// ----------------------------------------------------------------- screens

// switchScreen is a header letter: the chip lights and its screen takes
// the body. A list screen opens from the top, filter cleared, on what is
// there now. The web keeps its tabs and page exactly as they were.
func (m AppModel) switchScreen(k string) (tea.Model, tea.Cmd) {
	s, ok := screenKeys[k]
	if !ok {
		return m, nil
	}
	m.screen = s
	if s != screenWeb {
		kind := listKind(s - 1)
		m.lists.show(kind, m.listEntries(kind))
	}
	return m, nil
}

// screenKey is a key on a list screen with no float up: the header's
// letters and q are global there too, Space is the screen's menu, and the
// rest is the list's own.
func (m AppModel) screenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if !m.lists.typing {
		switch k {
		case "q":
			return m.askQuit()
		case "W", "B", "H", "D", "S":
			return m.switchScreen(k)
		case " ":
			return m.openMenu()
		case "V":
			// The easter egg, the same letter as on the web (splash.go).
			return m, m.splash.show()
		}
	}
	return m.listAction(m.lists.update(msg))
}

func (m AppModel) listEntries(kind listKind) []listEntry {
	var out []listEntry
	switch kind {
	case listBookmarks:
		return m.bookmarkEntries()
	case listDownloads:
		for i := len(m.dls) - 1; i >= 0; i-- { // newest first, as Chrome lists them
			d := m.dls[i]
			out = append(out, listEntry{title: d.name, url: d.url, meta: d.describe(), at: d.at, ref: i})
		}
	case listHistory:
		for i, v := range m.history {
			out = append(out, listEntry{title: v.Title, url: v.URL, at: v.At, ref: i})
		}
	case listSettings:
		return m.settingEntries()
	}
	return out
}

// listAction runs one of a list screen's operations by its key — from the
// key itself or from the Space menu, the same place either way.
func (m AppModel) listAction(key string) (tea.Model, tea.Cmd) {
	if key == "" {
		return m, nil
	}
	e, _, ok := m.lists.current()
	switch key {
	case "enter":
		if !ok {
			return m, nil
		}
		switch {
		case m.lists.kind == listDownloads:
			return m, m.openDownload(e.ref)
		case m.lists.kind == listSettings:
			return m, m.changeSetting(e.ref)
		case e.isFolder:
			m.toggleFolder(e.folder)
			return m, nil
		}
		// A bookmark or a visit opens in a NEW tab and never over the one
		// [W]eb was showing (revised 2026-09-21).
		m.screen = screenWeb
		return m, m.openTab(e.url, true)
	case "o":
		if !ok || e.isFolder {
			return m, nil
		}
		m.screen = screenWeb
		return m, m.openTab(e.url, true)
	case "m":
		if !ok || e.isFolder {
			return m, m.toast.show("m moves a bookmark; put the cursor on one", toastInfo)
		}
		return m, m.movePicker(e.ref)
	case "a":
		// A bookmark typed in, where the cursor is: inside the folder under
		// it, or beside the bookmark under it.
		folder := ""
		if ok {
			folder = e.folder
		}
		return m, m.startAddBookmark(folder)
	case "A":
		// A folder where the cursor is; a path makes every level at once.
		m.folderParent = ""
		prompt := "folder to add; a path like a/b/c makes each level"
		if ok && e.folder != "" {
			m.folderParent = e.folder
			prompt += ", inside " + e.folder
		}
		return m, m.input.ask(inputPopup{title: "Add folder", glyph: glyphFolder,
			prompt: prompt, accept: "create", action: inputFolder}, m.layer())
	case "y":
		if !ok || e.isFolder {
			return m, nil
		}
		if m.lists.kind == listDownloads {
			return m, m.yankDownload(e.ref)
		}
		return m, copyToClipboard(e.url)
	case "x":
		if !ok {
			return m, nil
		}
		switch {
		case e.isFolder:
			// The path, not the row's own name: "sub" is "dev/sub" (a folder
			// inside another could not be deleted until 2026-09-21).
			return m, m.deleteFolder(e.folder)
		case m.lists.kind == listDownloads:
			// Not destructive: the file stays; only the row goes (and a
			// download still running is stopped, which is what x on it says).
			return m, m.deleteEntry(e.ref)
		}
		return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Delete",
			lines: []string{nameOr(e.title, e.url), e.url}, accept: "delete", warn: true,
			action: confirmDeleteEntry, at: e.ref}, m.layer()+1)
	case "C":
		if m.lists.kind == listDownloads {
			m.clearDownloads()
			return m, nil
		}
		return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Clear history",
			lines:  []string{"Every visit ever recorded goes.", "This is the only way the log shrinks."},
			accept: "clear", warn: true, action: confirmClearHistory}, m.layer()+1)
	case "/":
		m.lists.typing = true
	}
	return m, nil
}

// addEntry puts the current page in Bookmarks and writes the file; a page
// already there is not added twice. (Shortcuts, the second list a page
// could be added to, went with the Places panel — revised 2026-09-20.)
func (m *AppModel) addEntry(kind listKind, title, url string) tea.Cmd {
	switch kind {
	case listBookmarks:
		for _, b := range m.bookmarks {
			if b.URL == url {
				return m.toast.show("already bookmarked", toastInfo)
			}
		}
		m.bookmarks = append(m.bookmarks, store.Bookmark{Title: title, URL: url})
		if err := m.saveBookmarks(); err != nil {
			return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
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
			err = m.saveBookmarks()
		}
	case listDownloads:
		return m.removeDownload(at)
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
// §A.1).
func (m AppModel) openMenu() (tea.Model, tea.Cmd) {
	var items []menuItem
	title := ""
	if m.screen != screenWeb {
		_, title = m.lists.title()
		m.spaceMenu.setItems(m.lists.menuItems(), title, 1)
		return m, m.spaceMenu.open()
	}
	switch m.focus {
	case panelTabs:
		title = "Tabs"
		if len(m.tabs) > 0 {
			items = append(items,
				menuItem{header: true, label: "item operation"},
				menuItem{label: "Switch to", key: "enter", hint: "show this tab in [2]"},
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
	case panelPage:
		title = "Page"
		items = m.pageMenuItems()
	}
	m.spaceMenu.setItems(items, title, 1)
	return m, m.spaceMenu.open()
}

// optionsKind says what the options menu is showing.
type optionsKind int

const (
	optSelect optionsKind = iota // a <select>'s options, keyed by index
	optMoveTo                    // a bookmark's folder, keyed by index (bookmarks.go)
)

// itemMenuItems is an item's operations by role (menu-only, no letters —
// ux.md §A.1): what a right click would list. The first row is the item's
// main action — the one Enter does outright, or after asking, for a link.
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
		// The same two words as a landmark's row and a bookmark folder's.
		if folded {
			items = append(items, menuItem{label: "Expand", key: "fold", hint: "show the section again"})
		} else {
			items = append(items, menuItem{label: "Collapse", key: "fold", hint: "the section, up to the next heading of its level"})
		}
	case ir.Unsupported:
		items = append(items,
			menuItem{label: "role: " + n.Role + ", not supported yet — only click", key: "unsupported", disabled: true},
			menuItem{label: "Click", key: "click", hint: "the page decides"})
	}
	return append(items,
		menuItem{label: "Yank text", key: "yanktext", hint: "what it says"},
		menuItem{label: "Inspect", key: "inspect", hint: "role, name, node id"})
}

// pageMenuItems is panel [2]'s Space menu: the item's operations, then the
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
		// The same T as [1]'s: a new tab is wanted from the page as often as
		// from the list (revised 2026-09-20).
		menuItem{label: "Tab", key: "T", hint: "a new one, at a URL"},
		menuItem{label: "Previous", key: "P", hint: "back in this tab", disabled: t == nil},
		menuItem{label: "Next", key: "N", hint: "forward in this tab", disabled: t == nil},
		menuItem{label: "Search", key: "/", hint: "find text on the page", disabled: t == nil},
		menuItem{label: "Visual mode", key: "v", hint: "walk the text by character, copy some", disabled: t == nil},
		menuItem{label: "Location", key: "L", hint: "a URL or a search; this page's own is offered"},
		menuItem{label: "Add bookmark", key: "A", hint: "this page", disabled: t == nil},
		menuItem{label: "Outline", key: "O", hint: "landmarks and headings", disabled: t == nil},
		menuItem{label: "Inspect", key: "I", hint: "DevTools: network, storage, console, source", disabled: t == nil},
		menuItem{label: "Zoom", key: "Z", hint: "the page alone, or the grid back"},
		menuItem{label: "Yank page url", key: "Y", hint: "to the clipboard", disabled: t == nil},
		menuItem{label: "Close", key: "C", hint: "this tab", disabled: t == nil})
	return items
}

// textboxItems is a filled textbox's rows in the Space menu (ux.md §2.2):
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

// optionsKey drives the second-level menu: a select's options (whose keys
// are their index), or the Move to… picker.
func (m AppModel) optionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var key string
	m.options, key, _ = m.options.update(msg)
	if key == "" {
		return m, nil
	}
	if m.optionsKind == optMoveTo {
		// The Bookmarks screen's picker: no page is involved.
		closeCmd := m.options.close()
		idx, err := strconv.Atoi(key)
		if err != nil {
			return m, closeCmd
		}
		return m, tea.Batch(closeCmd, m.moveBookmark(m.moveRef, idx))
	}
	n := m.optionsFor
	t := m.shownTab()
	if t == nil {
		return m, m.options.close()
	}
	if i := m.options.cursor; i < len(m.options.items) && m.options.items[i].disabled && m.options.items[i].key == key {
		return m, m.toast.show(m.options.items[i].hint, toastInfo)
	}
	idx, err := strconv.Atoi(key)
	if n == nil || err != nil || idx < 0 || idx >= len(n.Children) {
		return m, m.options.close()
	}
	opt := n.Children[idx]
	return m, tea.Batch(m.closeStack(), t.act(func(ctx context.Context) error { return page.Choose(ctx, opt.ID) }))
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
	if m.screen != screenWeb {
		return m.listAction(key)
	}
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
	// ---- panel [1]
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
		if n := len(m.closed); n > 0 {
			last := m.closed[n-1]
			m.closed = m.closed[:n-1]
			return m, m.openTab(last.URL, true)
		}
		return m, m.toast.show("nothing closed yet", toastInfo)
	case "L":
		// Chrome's Cmd+L, from any panel: the box opens with the page's own
		// URL on offer — Tab takes it into the line to edit, Backspace
		// clears it, typing over it starts fresh (ux.md §7).
		p := inputPopup{title: "Location", glyph: glyphSearch,
			prompt: "URL, or words to search for", accept: "open", action: inputGoto}
		if t != nil && t.url != "" && t.url != "about:blank" {
			p.placeholder = t.url
		}
		return m, m.input.ask(p, m.layer())

	// ---- panel [2], page
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
	case "I":
		// Inspect, Chrome's word for it (Cmd+Opt+I); D is the header's
		// Downloads from any panel (revised 2026-09-20).
		return m, m.openDevtools()
	case "C":
		// The page's own close: the tab [2] is showing, wherever [1]'s
		// cursor is. Its lowercase twin in [1] closes the cursor's tab. It
		// was W until W became the header's Web (2026-09-21).
		if t != nil {
			return m.closeTab(m.shown)
		}
	case "Z":
		m.zoom = !m.zoom
		m.relayoutTabs()
		return m, nil
	case "/":
		return m, m.enterSelect(true)
	case "v", "select":
		return m, m.enterSelect(false)
	case "A":
		if t != nil {
			return m, m.addEntry(listBookmarks, t.title, t.url)
		}

	// ---- panel [2], item
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

// enterItem is Enter on panel [2]: what a left click on the item would
// do, in terminal terms (ux.md §A.0.K, settled 2026-09-21). Space is the
// right-click menu, and the two do not mix — Enter never opens a menu.
//
//   - a textbox is focused to type into: the input popup, a password's
//     masked
//   - a select drops its list: the option menu
//   - a button, a check box, a media box, an unsupported node: the click
//   - a link would leave the page for somewhere the screen does not show,
//     so it asks first — text and URL in a confirm — and opens on Enter
//   - a landmark's or heading's row has no click to map; its own action
//     is to collapse or expand
//   - anything else says that Enter has nothing defined for it yet; the
//     Space menu still lists what it can do
func (m AppModel) enterItem() (tea.Model, tea.Cmd) {
	t := m.shownTab()
	n := t.current()
	if t == nil || n == nil {
		return m, nil
	}
	switch n.Kind {
	case ir.Textbox:
		return m, m.editField(n)
	case ir.Combobox:
		return m.chooseOptions()
	case ir.Button, ir.Check, ir.Media, ir.Unsupported:
		return m.dispatch("click")
	case ir.Link:
		return m, m.askOpenLink(n)
	case ir.Landmark, ir.Heading:
		return m.dispatch("fold")
	}
	return m, m.message.show(glyphInfo, "Enter", []string{
		"Nothing is defined for Enter on this item yet.",
		"Space lists what can be done with it."}, false, m.layer())
}

// askOpenLink puts a link's text and URL up before following it: a click
// would leave the page, and nothing on the screen says where to. Enter
// opens it in this tab; Open in new tab is in the Space menu.
func (m *AppModel) askOpenLink(n *ir.Node) tea.Cmd {
	lines := []string{oneLine(n.URL)}
	if text := oneLine(n.Text()); text != "" {
		lines = append([]string{text}, lines...)
	}
	return m.confirm.ask(confirmPopup{glyph: glyphLink, title: "Open link", lines: lines,
		accept: "open", action: confirmOpenLink, node: n.ID}, m.layer())
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
	value, prompt := n.Value, "value"
	if n.Protected {
		// Never the old value: Chromium hands over dots, not the secret.
		value, prompt = "", "password"
	}
	return m.input.ask(inputPopup{title: oneLine(nameOr(n.Name, "field")), glyph: glyphPencil,
		prompt: prompt, accept: "set", action: inputField, node: n.ID, value: value,
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
		m.focus = panelPage
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
	case inputSetting:
		// An offer still standing means nothing was typed or declined.
		return m, m.saveSetting(value, value == "" && m.input.placeholder != "")
	case inputFolder:
		return m, tea.Batch(m.input.close(), m.addFolder(m.folderParent, value))
	case inputBookmarkURL:
		return m, m.bookmarkURLGiven(value)
	case inputBookmarkTitle:
		return m, m.bookmarkTitleGiven(value)
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
	case confirmOpenLink:
		t := m.shownTab()
		if t == nil {
			return m, closeCmd
		}
		id := m.confirm.node
		return m, tea.Batch(closeCmd, t.act(func(ctx context.Context) error { return page.Click(ctx, id) }))
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

// openTab creates a tab at url; show says whether panel [2] switches to it
// (a new tab does, ux.md §6).
func (m *AppModel) openTab(url string, show bool) tea.Cmd {
	t := m.newTab()
	m.tabs = append(m.tabs, t)
	if show || m.shown < 0 {
		m.shown = len(m.tabs) - 1
		m.cur2 = m.shown
		m.focus = panelPage
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

// showTab is Enter on panel [1]: panel [2] switches, and the keyboard goes
// with it (ux.md §6). A pending tab loads now.
func (m AppModel) showTab(i int) (tea.Model, tea.Cmd) {
	m.leaveSelect()
	m.shown = i
	m.focus = panelPage
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
	// The easter-egg splash replaces the whole frame while it plays.
	if m.splash.isActive() {
		return m.splash.render(m.w, m.h)
	}
	ph := m.panelH()
	var out string
	switch {
	case m.screen != screenWeb:
		out = m.lists.panel(m.w, ph)
	case m.zoom || (m.narrow() && m.focus == panelPage):
		out = m.pagePanel(m.w, ph)
	case m.narrow():
		out = m.tabsPanel(m.w, ph)
	default:
		out = joinHorizontal(m.tabsPanel(sideW, ph), m.pagePanel(m.w-sideW, ph))
	}
	out = m.header() + "\n" + m.headerRule() + "\n" + out + "\n" + m.footer()

	// Bottom to top: the menu first so what it opened lands above it.
	if m.spaceMenu.isActive() {
		out = overlay.Composite(m.spaceMenu.view(), out, overlay.Center, overlay.Center, 0, 0)
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

// header is the top row (ui.md §1.1): the screens as one chain of chips,
// the current one lit, and on the right the downloads still in flight.
// sshu's tab row, reused for the same reason: chrome above the surfaces
// that never moves, and the lit chip is what says which surface you are
// on (revised 2026-09-21: [W]eb and [S]ettings joined the three lists,
// which became screens rather than popups).
var headerLabels = []string{"[W]eb", "[B]ookmarks", "[H]istory", "[D]ownloads", "[S]ettings"}

func (m AppModel) header() string {
	active := int(m.screen)
	status, live := "", false
	if n := m.downloading(); n > 0 {
		status, live = plural(n, "download")+" in flight", true
	}
	return tabRow(m.w, headerLabels, active, status, live)
}

// headerRule is the line under the header (ui.md §1.1): it keeps the
// chips from reading as one strip with the panel titles below, and while
// a download runs it is the thinnest progress bar there is — sshu's
// tabRule, doing for downloads what it does there for transfers.
func (m AppModel) headerRule() string {
	pct, moving := m.downloadProgress()
	return tabRule(m.w, pct, moving)
}

// tabsPanel is panel [1], outerW wide and outerH tall.
func (m AppModel) tabsPanel(outerW, outerH int) string {
	innerW, innerH := outerW-2, outerH-2
	return panelChrome(innerW, fitLines(m.tabsBody(innerW, innerH), innerW, innerH), "[1] Tabs", m.focus == panelTabs)
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
	case m.focus == panelPage:
		tone = toneFocus
	}
	return panelFrame(innerW, fitLines(m.pageBody(innerW, innerH), innerW, innerH), "[2] Page", hint, tone)
}

// footer is the mandatory disclosure of the entry keys (§A.1 / §A.2): one
// row, locked (ui.md §5). Selection mode replaces it with only the keys
// that work there (ux.md §B: the footer is honest).
func (m AppModel) footer() string {
	if m.screen != screenWeb {
		return keyLegend([][2]string{{"space", "menu"}, {"?", "help"}, {"esc", "web"}, {"q", "quit"}}, m.w)
	}
	if m.sel.on && !m.popupOpen() {
		return keyLegend(selectLegendPairs(m.sel.typing), m.w)
	}
	return keyLegend([][2]string{{"space", "menu"}, {"?", "help"}, {"tab/1-2", "panels"}, {"q", "quit"}}, m.w)
}
