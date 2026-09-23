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
	lists     listPanel   // the screen behind a header chip after [W]eb
	splash    splashModel // the easter egg (splash.go)
	devtools  devtoolsPopup
	message   messagePopup
	finder    finder // [/] over the page's nodes, [go] over its lines (finder.go)
	help      helpPopup
	confirm   confirmPopup
	input     inputPopup
	editor    editorPopup // a textarea's box: several lines, two modes (editorpopup.go)
	picker    filePicker  // a file, picked rather than typed (filepicker.go)
	toast     toastModel

	// optionsFor is the node the options menu is about, and optionsKind
	// what the menu is: an item's operations, a select's options, or the
	// Add to… picker.
	optionsFor  *ir.Node
	optionsKind optionsKind
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
	// renameRef is the bookmark a Rename box is about, renameFolder the
	// folder — one or the other (bookmarks.go).
	renameRef     int
	renameFolder  string
	newBookmark   store.Bookmark
	foldedFolders map[string]bool
	// pendingImport is a browser's export the picker read, waiting for the
	// folder name it goes under (bookmarks.go).
	pendingImport *store.Import
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
		lists:     newListPanel(),
		devtools:  newDevtoolsPopup(),
		message:   newMessagePopup(),
		finder:    newFinder(),
		help:      newHelpPopup(),
		confirm:   newConfirmPopup(),
		input:     newInputPopup(),
		picker:    newFilePicker(),
		editor:    newEditorPopup(),
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
	// The spinner is armed on a key, because a key is where a fetch
	// usually begins — but the page webu opens with has no key behind
	// it, so its chain was never started and the first page of all drew
	// one frozen frame (2026-09-22).
	return tea.Batch(waitEvent(m.events), spinCmd())
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
		m.devtools.isActive() {
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
			&m.spaceMenu, &m.options, &m.lists, &m.devtools, &m.message,
			&m.finder, &m.help, &m.confirm, &m.input, &m.editor, &m.picker, &m.toast} {
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

	case spinTickMsg:
		// The chain lives exactly as long as the fetch does.
		if m.fetching() {
			return m, spinCmd()
		}
		return m, nil

	case AnimTickMsg:
		return m, tea.Batch(
			m.spaceMenu.anim.tick(msg), m.options.anim.tick(msg),
			m.devtools.anim.tick(msg), m.devtools.detail.anim.tick(msg),
			m.message.anim.tick(msg), m.finder.anim.tick(msg),
			m.help.anim.tick(msg), m.confirm.anim.tick(msg), m.input.anim.tick(msg), m.editor.anim.tick(msg),
			m.picker.anim.tick(msg), m.toast.anim.tick(msg))

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
		// The page asks for a file: the file picker, the way the
		// Bookmarks import chooses one — walked, not typed (user,
		// 2026-09-23). One file at a time, even where the page would
		// take several.
		m.upload = &msg
		return m, tea.Batch(waitEvent(m.events),
			m.picker.open(glyphUpload, "Upload", importDir(), m.layer()))

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
		t.loading, t.navigating = false, false
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
		cmds := []tea.Cmd{waitEvent(m.events), t.settle(250 * time.Millisecond)}
		if m.fetching() {
			// A navigation the page started rather than the user: the
			// spinner is armed here instead.
			cmds = append(cmds, spinCmd())
		}
		return m, tea.Batch(cmds...)

	case settleMsg:
		_, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return m, nil
		}
		// A page with a dialog up answers no CDP call that touches it; the
		// capture would only time out. It is re-asked once the dialog is.
		if m.dialog != nil && m.dialog.tabID == t.id {
			t.loading = false
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
		if id := t.wantDrill; id != 0 && !t.working() {
			// The frame Enter asked for: in, now that its document is
			// here — or not, when it is another site's and could not be.
			t.wantDrill = 0
			if n := nodeByID(t.root, id); n != nil && len(n.Children) > 0 {
				t.drillInto(n, m.pageW(), m.pageVisible())
			} else {
				return m, m.toast.show("a frame from another site: webu cannot enter it; its address is a Yank away", toastInfo)
			}
		}
		if t.stillComing() {
			// An application's shell: <body> exists and nothing is drawn
			// yet. Keep saying so, and look again — the observer will
			// also fire, and a second settle costs a redraw.
			t.loading = true
			return m, tea.Batch(t.settle(300*time.Millisecond), spinCmd())
		}
		if t.settling() {
			// The page is still being built: this look does not agree
			// with the last one. Look again, and go on saying so — but
			// without loading's dim, because what is drawn is this page,
			// not the one being left (tab.working).
			return m, tea.Batch(t.settle(300*time.Millisecond), spinCmd())
		}
		m.recordVisit(t)
		if i == m.shown && t.certErr && !t.certAsked {
			t.certAsked = true
			return m, m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Certificate error",
				lines: []string{"Chromium does not trust this site's certificate.", fitURL(t.url, 60),
					"Continue anyway, for this tab, for as long as it is open?"},
				accept: "continue", warn: true, action: confirmCert, tabID: t.id}, m.layer())
		}
		if n := t.current(); i == m.shown && msg.err == nil && n != nil {
			id := n.ID
			return m, t.act(func(ctx context.Context) error { return page.Reveal(ctx, id) })
		}
		return m, nil

	case tea.KeyMsg:
		// A key is where a fetch begins, so it is where the spinner in
		// panel [2] is armed; the chain then keeps itself alive for as
		// long as the fetch does (pagepanel spinTickMsg). Arming twice
		// costs a redraw, never a wrong frame: the frame is read from
		// the clock, not counted.
		mm, cmd := m.handleKey(msg)
		if am, ok := mm.(AppModel); ok && am.fetching() {
			cmd = tea.Batch(cmd, spinCmd())
		}
		return mm, cmd
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
	// The click that raised this has not returned and will not until the
	// dialog is answered, so the settle behind it never runs: the tab
	// stops saying it is busy here instead (2026-09-22). What is waiting
	// on the user is the dialog, and the dialog is on screen.
	t.loading = false
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
	return t.unblock(func(ctx context.Context) error { return page.HandleDialog(ctx, accept, text) })
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
	t.leavePagetab()
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
		t.leavePagetab()
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
	return m.spaceMenu.isActive() || m.options.isActive() ||
		m.devtools.isActive() || m.message.isActive() || m.finder.isActive() ||
		m.help.isActive() || m.confirm.isActive() || m.input.isActive() || m.editor.isActive() || m.picker.isActive()
}

// floatOwned reports whether some float still holds the keyboard — not
// merely is on screen: one that is closing has let go (§6.2), and an Esc
// that arrived during its animation belongs to whatever is under it.
func (m AppModel) floatOwned() bool {
	return m.toast.anim.owns() || m.input.anim.owns() || m.editor.anim.owns() || m.picker.anim.owns() || m.confirm.anim.owns() ||
		m.options.anim.owns() || m.devtools.anim.owns() || m.finder.anim.owns() ||
		m.message.anim.owns() || m.help.anim.owns() || m.spaceMenu.anim.owns()
}

// typing reports whether a float is taking text: every printable key is a
// character then (§4.5). A search being typed in selection mode counts.
func (m AppModel) typing() bool {
	return m.input.anim.owns() || m.editor.typing() || m.picker.anim.owns() || m.finder.typing() || (m.screen != screenWeb && m.lists.typing) ||
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
		case m.focus == panelPage:
			// Nothing up, no mode on: Esc is the way onto the pagetab under
			// the URL — the page's chrome — and back off it (2026-09-22).
			if m.toast.anim.owns() {
				return m, m.toast.close()
			}
			return m.togglePagetab()
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
	case m.editor.anim.owns():
		return m.editorKey(msg)
	case m.picker.anim.owns():
		return m.pickerKey(msg)
	case m.finder.anim.owns():
		return m.finderKey(msg)
	case m.confirm.anim.owns():
		return m.confirmKey(msg)
	case m.options.anim.owns():
		return m.optionsKey(msg)
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
		// A long message — a cell in full — scrolls by the page's keys.
		m.message.scroll(msg.String())
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
		}
		return m, m.input.close()
	case m.editor.anim.owns():
		// Layered: out of writing into the box, and out of the box.
		cmd, _ := m.editor.escape()
		return m, cmd
	case m.picker.anim.owns():
		m.upload = nil // a page's chooser is simply left unanswered: nothing is chosen
		return m, m.picker.close()
	case m.finder.anim.owns():
		// Layered: from the list Esc goes back up to the query, and from
		// the query it closes.
		cmd, _ := m.finder.escape()
		return m, cmd
	case m.confirm.anim.owns():
		if m.confirm.action == confirmDialog {
			return m, tea.Batch(m.confirm.close(), m.answerDialog(false, ""))
		}
		return m, m.confirm.close()
	case m.options.anim.owns():
		return m, m.options.close()
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

// togglePagetab moves the hand between the page and the pagetab under
// the URL — its chrome (pagepanel.pagetabRow). Esc does it, and so does
// the row the page's Space menu lists for it: a panel operation is
// disclosed in the menu or it does not exist (ux.md §A.1).
func (m AppModel) togglePagetab() (tea.Model, tea.Cmd) {
	t := m.shownTab()
	if t == nil {
		return m, nil
	}
	if t.onPagetab() {
		t.leavePagetab()
		return m, nil
	}
	// Esc is one move: up a level. Inside a list item that is the page it
	// was on; inside a section, the section list; on the list, or on a
	// page with neither, the chrome above both (section.go).
	if t.drilled() {
		t.leaveDrill(m.pageW(), m.pageVisible())
		return m, nil
	}
	if t.popupNode() != nil {
		// A popup is not left, it is answered (popup.go): the page put it
		// up for a decision, and Esc would be that decision put off.
		return m, m.toast.show("this popup wants an answer: Esc does not close it", toastInfo)
	}
	if t.read {
		t.closeSection()
		return m, nil
	}
	if !t.enterPagetab() {
		return m, m.toast.show("this page has no chrome", toastInfo)
	}
	return m, nil
}

// closeStack tears every float down: an errand that ended in an action is
// over, and the user is back on the panel (§7.1).
func (m *AppModel) closeStack() tea.Cmd {
	return tea.Batch(m.input.close(), m.editor.close(), m.picker.close(), m.confirm.close(), m.options.close(),
		m.devtools.close(), m.message.close(), m.finder.close(),
		m.help.close(), m.spaceMenu.close())
}

func (m AppModel) quit() (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

// panelKey is a key with no float up: the global letters first, then the
// focused panel's own.
func (m AppModel) panelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	// The g chords: g then g is the top, g then o is a section's number.
	if m.pendingG {
		m.pendingG = false
		switch k {
		case "g":
			k = "gg"
		case "o":
			k = "go"
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
		return m.openFinder(finderSearch)
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
		case "c", "o", "r", "y", "T", "X", "U":
			return m.dispatch(k)
		}
	case panelPage:
		t := m.shownTab()
		if navKeys[k] && t != nil {
			if t.listing() && !t.onPagetab() {
				t.moveSection(k, m.pageVisible())
				return m, nil
			}
			t.moveItem(k, m.pageVisible())
			if n := t.current(); n != nil && n.ID != 0 {
				id := n.ID
				return m, t.act(func(ctx context.Context) error { return page.Reveal(ctx, id) })
			}
			return m, nil
		}
		switch k {
		case "enter":
			return m.dispatch("enter")
		case "n", "p", "go":
			return m.dispatch(k)
		case "R", "T", "Y", "A", "Z", "I", "C":
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
	case "r":
		if !ok {
			return m, nil
		}
		return m, m.startRename(e)
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
	case "I":
		return m, m.startImport()
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
				menuItem{label: "[Enter] Switch to", key: "enter", hint: "show this tab in [2]"},
				// c, the page panel's C in lower case; it was w, which is a
				// window's letter, and a terminal has none (2026-09-21).
				menuItem{label: "Close", key: "c", hint: "this tab"},
				// o as on a download's row: this row's URL, in a new tab.
				menuItem{label: "Open in new tab", key: "o", hint: "the same page again"},
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
	optItemMenu optionsKind = iota // an item's operations on their own
	optSelect                      // a <select>'s options, keyed by index
	optMoveTo                      // a bookmark's folder, keyed by index (bookmarks.go)
)

// itemMenuItems is an item's operations by role (menu-only, no letters —
// ux.md §A.1): what a right click would list. The first row is the item's
// main action — the one Enter does outright, or after asking, for a link.
// folded is a landmark's state, which decides which way its row reads.
func itemMenuItems(n *ir.Node, folded bool) []menuItem {
	var items []menuItem
	switch n.Kind {
	case ir.Cell:
		// A cell cut to its column: the content in full, then whatever
		// it holds (the same rows an entry lists).
		items = append(items, menuItem{label: "Content", key: "cell:content", hint: "the cell in full"})
		items = append(items, targetItems(n)...)
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
	case ir.Code:
		items = append(items, menuItem{label: "[Enter] Read", key: "enter", hint: "the block on its own, lines unfolded"})
	}
	return append(items,
		menuItem{label: "Yank text", key: "yanktext", hint: "what it says"},
		menuItem{label: "Inspect", key: "inspect", hint: "role, name, node id"})
}

// targetItems is what a table cell (or a piece of chrome) holds, one row
// each — a link, a button, a field, a check box, a select — with the
// nesting of its lists as indent; the key is the target's place
// (dispatch).
func targetItems(n *ir.Node) []menuItem {
	var items []menuItem
	for i, t := range entryTargets(n) {
		items = append(items, targetRow(t, i))
	}
	return items
}

// targetRow is one target's row: its name, what it is as the hint —
// and "here" first when it is where the user is (aria-current) — its
// place as the key.
func targetRow(t entryTarget, i int) menuItem {
	label := oneLine(t.node.Name)
	if label == "" {
		label = oneLine(nameOr(t.node.Text(), t.node.URL))
	}
	hint := oneLine(t.node.URL)
	switch t.node.Kind {
	case ir.Button:
		hint = "button"
	case ir.Textbox:
		hint = "a field: type into it"
	case ir.Check:
		hint = "check box"
	case ir.Combobox:
		hint = "select"
	}
	if t.node.Current {
		hint = "here · " + hint
	}
	return menuItem{label: strings.Repeat("  ", t.depth) + truncate(label, 60),
		key: "entry:" + itoa(i), hint: hint}
}

// pageMenuItems is panel [2]'s Space menu: the item's operations, then the
// page's — the whole of what can be done here (ux.md §A.1).
func (m AppModel) pageMenuItems() []menuItem {
	var items []menuItem
	t := m.shownTab()
	if t.listing() {
		// On the section list the item is a section, and the one thing to
		// do to it is open it.
		items = append(items,
			menuItem{header: true, label: "item operation"},
			menuItem{label: "[Enter] Open section", key: "enter",
				hint: sectionOpenHint(t)},
			menuItem{separator: true},
			menuItem{header: true, label: "panel operation"})
	} else if n := t.current(); t != nil && n != nil {
		items = append(items, menuItem{header: true, label: "item operation"})
		items = append(items, itemMenuItems(n, t.curFolded())...)
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
		menuItem{label: "[/] Search", key: "/", hint: "every part of the page; Enter goes there",
			disabled: t == nil || t.popupNode() != nil},
		menuItem{label: "Visual mode", key: "v", hint: "walk the text by character, copy some", disabled: t == nil},
		menuItem{label: "Location", key: "L", hint: "a URL or a search; this page's own is offered"},
		menuItem{label: "Add bookmark", key: "A", hint: "this page", disabled: t == nil},
		pagetabItem(t),
		sectionsItem(t),
		goItem(t),
		menuItem{label: "[n] Next section", key: "n", hint: "the one after this, at the same depth",
			disabled: t == nil || !t.read},
		menuItem{label: "[p] Previous section", key: "p", hint: "the one before this, at the same depth",
			disabled: t == nil || !t.read},
		menuItem{label: "Inspect", key: "I", hint: "DevTools: network, storage, console, source", disabled: t == nil},
		menuItem{label: "Zoom", key: "Z", hint: "the page alone, or the grid back"},
		menuItem{label: "Yank page url", key: "Y", hint: "to the clipboard", disabled: t == nil},
		menuItem{label: "Yank markdown", key: "yankmd", hint: "the page itself, as markdown",
			disabled: t == nil || t.root == nil},
		menuItem{label: "Close", key: "C", hint: "this tab", disabled: t == nil})
	return items
}

// pagetabItem is the page's Space menu row for the pagetab: the way onto
// the page's chrome and back. Esc is the key, and it is written into the
// label — bracketHotkey brackets a single letter in place, and a core
// key has no letter to bracket, so the row says it itself (user,
// 2026-09-22).
func pagetabItem(t *tab) menuItem {
	switch {
	case t == nil || len(t.parts) < 2:
		return menuItem{label: "[Esc] Page parts", key: "pagetab",
			hint: "this page is all one part", disabled: true}
	case t.onPagetab():
		return menuItem{label: "[Esc] Back to the page", key: "pagetab",
			hint: "h/l show a part, Enter stays on it"}
	}
	return menuItem{label: "[Esc] Page parts", key: "pagetab",
		hint: "header, body, others, footer"}
}

// goItem is the go chord's row: a line of what is on screen, by the
// number in the gutter (user, 2026-09-23).
func goItem(t *tab) menuItem {
	n := 0
	if t != nil && t.popupNode() == nil {
		n = t.lineCount()
	}
	return menuItem{label: "[go] Go to line", key: "go",
		hint: "by its number, 1 to " + itoa(n), disabled: n == 0}
}

// sectionOpenHint says what opening the section under the list cursor
// will give: its size, so the row is a decision and not a leap.
func sectionOpenHint(t *tab) string {
	if t.sec >= len(t.secs) {
		return ""
	}
	s := t.secs[t.sec]
	return truncate(s.title, 40) + " · " + plural(s.lines(), "line")
}

// sectionsItem is the page's Space menu row for the cut into sections
// (section.go): a document is read one section at a time, and this is how
// that is turned off for a page the cut does not suit — or on for one it
// was not offered for.
func sectionsItem(t *tab) menuItem {
	switch {
	case t == nil || len(t.secs) == 0:
		return menuItem{label: "Sections", key: "sections",
			hint: "this page has no headings to cut on", disabled: true}
	case t.flat:
		return menuItem{label: "Sections", key: "sections",
			hint: "read this page one section at a time"}
	}
	return menuItem{label: "One sheet", key: "sections",
		hint: "read the whole page in one run"}
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

// optionsKey drives the second-level menu: an item's own operation list
// (what an entry row holds), a select's options (whose keys are their
// index), or the Move to… picker.
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
	if m.optionsKind == optItemMenu {
		// Close BEFORE dispatching: dispatch returns its own copy of the
		// model, and a close applied to this one afterwards would land on
		// a model nobody returns.
		closeCmd := m.options.close()
		mm, cmd := m.dispatch(key)
		return mm, tea.Batch(closeCmd, cmd)
	}
	idx, err := strconv.Atoi(key)
	if n == nil || err != nil || idx < 0 || idx >= len(n.Children) {
		return m, m.options.close()
	}
	opt := n.Children[idx]
	return m, tea.Batch(m.closeStack(), t.press(func(ctx context.Context) error { return page.Choose(ctx, opt.ID) }))
}

// ---------------------------------------------------------------- actions

// busy reports whether the shown page is on its way somewhere: a
// navigation or history move has been asked for and has not landed.
// busy is a NAVIGATION in flight, and it is what swallows the keys that
// would stack another one on it (ux.md §6). A click on the page is not
// this: it says the page is loading, which dims it and turns the
// spinner, but the keyboard stays live.
func (m AppModel) busy() bool {
	t := m.shownTab()
	return t != nil && t.navigating
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
	if key == "cell:content" {
		if n := t.current(); t != nil && n != nil && n.Kind == ir.Cell {
			return m, m.showCell(t, n)
		}
		return m, nil
	}
	if strings.HasPrefix(key, "entry:") {
		// Something inside a capsule, or a table cell, by its place in
		// its operation list (capsuleMenuItems, targetItems).
		if t != nil && !m.busy() {
			i, err := strconv.Atoi(strings.TrimPrefix(key, "entry:"))
			if ts, search := t.currentTargets(); err == nil && i >= 0 && i < len(ts) {
				return m.actOn(t, ts[i].node, search)
			}
		}
		return m, nil
	}
	switch key {
	// ---- panel [1]
	case "show":
		if m.cur2 >= 0 && m.cur2 < len(m.tabs) {
			return m.showTab(m.cur2)
		}
	case "c":
		return m.closeTab(m.cur2)
	case "o":
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
		return m.openFinder(finderSearch)
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
			return m, t.press(func(ctx context.Context) error { return page.Click(ctx, id) })
		}
	case "choose":
		return m.chooseOptions()
	case "pagetab":
		return m.togglePagetab()
	case "n", "p":
		// The next and previous section at the same depth, without going
		// back through the list (section.stepSection).
		if t == nil || !t.read {
			return m, nil
		}
		step := 1
		if key == "p" {
			step = -1
		}
		if !t.stepSection(step, m.pageVisible()) {
			return m, m.toast.show("no section that way", toastInfo)
		}
		return m, nil
	case "go":
		// A number earns its column only when a key takes you to it
		// (user, 2026-09-22); this is that key. It asks for a line of
		// what is on screen — on the section list the lines are the
		// sections, so the same number reaches both (user, 2026-09-23).
		return m.openFinder(finderGo)
	case "sections":
		// The page as one sheet, or cut into sections: the way out when
		// the cut is wrong for this page, and the way in when a page was
		// not cut but you want it to be.
		if t == nil || len(t.secs) == 0 {
			return m, m.toast.show("this page has no headings to cut on", toastInfo)
		}
		t.flat, t.read = !t.flat, false
		if t.flat {
			t.scrollToCursor(m.pageVisible())
			return m, nil
		}
		t.sec = sectionAt(t.secs, t.top)
		t.clampSecTop(m.pageVisible())
		return m, nil
	case "yankmd":
		// The page itself rather than its URL: what webu draws, said in
		// the form the rest of the family passes around (ir.Markdown).
		if t != nil && t.root != nil {
			return m, copyToClipboard(ir.Markdown(t.root))
		}
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
			return m, t.press(func(ctx context.Context) error { return page.Submit(ctx, id) })
		}
	case "edit":
		if n := t.current(); n != nil {
			return m, m.editField(n)
		}
	case "clear":
		if n := t.current(); n != nil {
			id := n.ID
			return m, t.press(func(ctx context.Context) error { return page.Type(ctx, id, "") })
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
//   - a chrome row — banner, navigation, breadcrumb, search, sidebar,
//     footer — is an entry: its own operation is the list of what it
//     holds, the same rows the Space menu's item half shows; a search
//     with one box opens the box outright
//   - a content landmark's row, or a heading's, has no click to map; its
//     own action is to collapse or expand
//   - anything else says that Enter has nothing defined for it yet; the
//     Space menu still lists what it can do
func (m AppModel) enterItem() (tea.Model, tea.Cmd) {
	t := m.shownTab()
	if t != nil {
		if t.onPagetab() {
			// The pagetab: h and l have been showing each part as they
			// reached it, so the part under the hand is already the one
			// on screen. Enter is the word for "yes, this one" — it takes
			// the hand back down to the page (user, 2026-09-23).
			t.leavePagetab()
			return m, nil
		}
		if t.listing() {
			// The section list: Enter opens one to the whole panel
			// (section.go).
			t.openSection(m.pageVisible())
			return m, nil
		}
	}
	n := t.current()
	if t == nil || n == nil {
		return m, nil
	}
	switch n.Kind {
	case ir.ListItem:
		// A list item is one thing; Enter is how you go into it
		// (section.drillInto). One that is nothing but its first line
		// has no inside to go to — Enter is Enter on what that line
		// holds, a link or a button, and nothing when it holds only
		// text (2026-09-23).
		w, vis := m.pageW(), m.pageVisible()
		if t.drillInto(n, w, vis) && t.drillBody >= len(t.lay.rows) {
			t.leaveDrill(w, vis)
			if x := firstItemIn(n); x != nil {
				return m.enterOn(t, x)
			}
		}
		return m, nil
	case ir.Cell:
		return m.enterCell(t, n)
	case ir.Media:
		if n.Frame != "" {
			// An iframe is one thing until you go in (user, 2026-09-23):
			// its document, when it has come; asked for, when not.
			if len(n.Children) > 0 {
				t.drillInto(n, m.pageW(), m.pageVisible())
				return m, nil
			}
			return m, tea.Batch(t.openFrame(n), spinCmd())
		}
	case ir.Code:
		// A block of code on its own: the lines unfolded, as far as the
		// terminal is wide, the page's keys to scroll (showCell's
		// sibling — the smallest unit of content is read, not opened).
		return m, m.showCode(n)
	case ir.Landmark:
		if n.Role == "search" {
			// A search with one box has one obvious operation: the box
			// (2026-09-21). It is a landmark like any other otherwise.
			var boxes []*ir.Node
			for _, x := range entryTargets(n) {
				if x.node.Kind == ir.Textbox {
					boxes = append(boxes, x.node)
				}
			}
			if len(boxes) == 1 {
				return m, m.editFieldAs(boxes[0], true)
			}
		}
		return m.dispatch("fold")
	case ir.Heading:
		return m.dispatch("fold")
	}
	return m.enterOn(t, n)
}

// firstItemIn is the first thing the cursor could stop on inside n, or
// nil: what Enter on a one-line list item is Enter on.
func firstItemIn(n *ir.Node) *ir.Node {
	var found *ir.Node
	n.Walk(func(x *ir.Node) bool {
		if found == nil && x != n && x.ID != 0 && x.IsItem() {
			found = x
		}
		return found == nil
	})
	return found
}

// enterOn is Enter on an interactive node: the item under the cursor,
// or the one thing a table cell holds. A field opens to type, a select
// drops its list, a link asks first, the rest is a click; anything else
// says nothing is defined.
func (m AppModel) enterOn(t *tab, n *ir.Node) (tea.Model, tea.Cmd) {
	if m.busy() {
		return m, nil
	}
	if n.Disabled {
		// A click on a disabled control does nothing in the browser
		// either, but the browser shows it greyed and a terminal's grey
		// is easy to miss: say why nothing happened (2026-09-23 — the
		// APG alert example disables Discard when the notes are empty).
		return m, m.toast.show(oneLine(nameOr(n.Name, "this"))+" is disabled on the page", toastInfo)
	}
	switch n.Kind {
	case ir.Textbox:
		return m, m.editField(n)
	case ir.Combobox:
		return m.chooseOptionsFor(n)
	case ir.Button, ir.Check, ir.Media, ir.Unsupported, ir.Option:
		id := n.ID
		return m, t.press(func(ctx context.Context) error { return page.Click(ctx, id) })
	case ir.Group:
		if n.Role == "treeitem" {
			// A tree's item: the page's click opens a branch or picks a
			// leaf, whichever it is.
			id := n.ID
			return m, t.press(func(ctx context.Context) error { return page.Click(ctx, id) })
		}
	case ir.Link:
		// A link into the page lands the cursor, no confirm: nothing is
		// left.
		if frag := sameFragment(t.url, n.URL); frag != "" && t.jumpToAnchor(frag, m.pageVisible()) {
			return m, nil
		}
		return m, m.askOpenLink(n)
	}
	return m, m.message.show(glyphInfo, "Enter", []string{
		"Nothing is defined for Enter on this item yet.",
		"Space lists what can be done with it."}, false, m.layer())
}

// enterCell is Enter on a data table's cell, drawn cut to its column
// (table): the content in full, a popup that scrolls — unless the cell
// is one link, one button, one field and nothing else, in which case the
// cell IS that thing and Enter is its Enter; a cell holding text and
// links both opens its operation list, Content first (2026-09-21).
func (m AppModel) enterCell(t *tab, n *ir.Node) (tea.Model, tea.Cmd) {
	ts := entryTargets(n)
	switch {
	case len(ts) == 0:
		return m, m.showCell(t, n)
	case len(ts) == 1 && oneLine(n.Text()) == oneLine(ts[0].node.Text()):
		return m.enterOn(t, ts[0].node)
	}
	return m.openItemMenu(n)
}

// showCell is a cell's content in full: the column's header as the
// title, the text wrapped, the page's keys to scroll when it is long.
func (m *AppModel) showCell(t *tab, n *ir.Node) tea.Cmd {
	title := nameOr(columnHeader(t.root, n), "cell")
	text := strings.TrimSpace(n.Text())
	if text == "" {
		text = "(empty)"
	}
	return m.message.show(glyphTable, title, wrapWords(text, min(72, max(20, m.w-12))), false, m.layer())
}

// showCode is a code block in full: every line as the page wrote it,
// tabs as four spaces, titled by its language when the page named one.
func (m *AppModel) showCode(n *ir.Node) tea.Cmd {
	title := "code"
	if n.Lang != "" {
		title += " · " + n.Lang
	}
	text := strings.ReplaceAll(strings.TrimRight(n.Text(), "\n"), "\t", "    ")
	return m.message.show(glyphCode, title, strings.Split(text, "\n"), false, m.layer())
}

// openItemMenu is Enter on an item whose own operation IS its operation
// list — a capsule, whose rows are what it holds. The same rows the
// Space menu's item half shows, on their own (ux.md §A.0.K).
func (m AppModel) openItemMenu(n *ir.Node) (tea.Model, tea.Cmd) {
	return m.openItemMenuAt(n, m.layer())
}

// openItemMenuAt is openItemMenu on a given layer: the one the list
// replaces, when it opens in place of another (openPagetabMore).
func (m AppModel) openItemMenuAt(n *ir.Node, layer int) (tea.Model, tea.Cmd) {
	t := m.shownTab()
	items, title := itemMenuItems(n, t.curFolded()), truncate(oneLine(nameOr(n.Name, n.Role)), 40)
	m.optionsFor, m.optionsKind = n, optItemMenu
	m.options.setItems(items, title, layer)
	return m, m.options.open()
}

// actOn does to a node inside an entry what Enter does to it as an item
// (enterItem): a field opens to type, a select drops its list, the rest
// is a click — no confirm for a link, the list having been the look.
// search says the entry is a search landmark, whose every box is one.
func (m AppModel) actOn(t *tab, x *ir.Node, search bool) (tea.Model, tea.Cmd) {
	switch x.Kind {
	case ir.Textbox:
		return m, m.editFieldAs(x, search || x.Role == "searchbox")
	case ir.Combobox:
		return m.chooseOptionsFor(x)
	case ir.Link:
		if frag := sameFragment(t.url, x.URL); frag != "" && t.jumpToAnchor(frag, m.pageVisible()) {
			return m, nil
		}
	}
	id := x.ID
	return m, t.press(func(ctx context.Context) error { return page.Click(ctx, id) })
}

// sameFragment is the fragment of a link that points into the page it
// is on — "#main", or the page's own URL with a fragment — else "".
func sameFragment(pageURL, url string) string {
	i := strings.Index(url, "#")
	if i < 0 {
		return ""
	}
	base, frag := url[:i], url[i+1:]
	if j := strings.Index(pageURL, "#"); j >= 0 {
		pageURL = pageURL[:j]
	}
	if frag == "" || (base != "" && base != pageURL) {
		return ""
	}
	return frag
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
	if t == nil {
		return m, m.options.close()
	}
	return m.chooseOptionsFor(t.current())
}

// chooseOptionsFor is chooseOptions on a given select: the one under the
// cursor, or one inside an entry row (actOn).
func (m AppModel) chooseOptionsFor(n *ir.Node) (tea.Model, tea.Cmd) {
	if n == nil || n.Kind != ir.Combobox {
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
	return m.editFieldAs(n, n.Role == "searchbox" || m.inSearch(n))
}

// inSearch reports whether a node sits inside a search landmark: a box
// there is a search box whatever it calls itself — Google's is a
// combobox — and what is typed into it is what the user came to submit.
func (m AppModel) inSearch(n *ir.Node) bool {
	t := m.shownTab()
	if t == nil {
		return false
	}
	for _, a := range chainTo(t.root, n) {
		if a.Kind == ir.Landmark && a.Role == "search" {
			return true
		}
	}
	return false
}

// editFieldAs is editField told whether the box is a search: one by its
// role (type=search), or any box inside a search landmark. What is
// written into a search box is then offered to the page's Enter
// (inputKey) — that being what typing into one is for.
func (m *AppModel) editFieldAs(n *ir.Node, search bool) tea.Cmd {
	value := n.Value
	if n.Protected {
		// Never the old value: Chromium hands over dots, not the secret.
		value = ""
	}
	// The type on the border, the field's own name over the box (user,
	// 2026-09-23). The border is chrome and says what KIND of box this
	// is; the line inside it is about this one field, and the name is
	// the thing that tells one field from the next.
	title := fieldTakes(n)
	if n.Invalid {
		// The page marked what is there wrong: the one thing the box
		// should say about the value it is about to replace.
		title += " · invalid"
	}
	if n.Multiline {
		// A textarea: the big box, several lines, two modes
		// (editorpopup.go).
		return m.editor.ask(title, oneLine(nameOr(n.Name, "field")), value, n.ID, m.layer())
	}
	return m.input.ask(inputPopup{title: title, glyph: glyphPencil,
		prompt: oneLine(nameOr(n.Name, "field")), accept: "set", action: inputField,
		node: n.ID, value: value, masked: n.Protected, search: search}, m.layer())
}

// fieldTakes is what the box over a field says it wants. The page's own
// word for it, where the page gave one: "email" over an email box, not
// "value" (user, 2026-09-23) — a terminal shows nothing of a field's
// type, where a browser shows a date picker or a number stepper, so the
// one line above the box is where it has to be said.
//
// Chromium's own default is text, and "text" is worth saying: it is the
// answer to "what does this want?", and a blank there would read as
// webu not knowing.
func fieldTakes(n *ir.Node) string {
	if n.Protected {
		return "password"
	}
	switch t := n.InputType; t {
	case "":
		// Not an <input>, or one that declared nothing.
		switch {
		case n.Multiline:
			return "text, several lines"
		case n.Role == "searchbox":
			return "search"
		case n.Role == "spinbutton":
			return "number"
		}
		return "text"
	case "tel":
		return "phone number"
	case "datetime-local":
		return "date and time"
	default:
		return t
	}
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
		write := t.press(func(ctx context.Context) error { return page.Type(ctx, id, value) })
		if m.input.search && strings.TrimSpace(value) != "" {
			// A search box: what was typed is what the user came to
			// submit, so the box's Enter offers that at once — a
			// confirm, then the page's own Enter in the field
			// (2026-09-21). Esc keeps the value, unsent.
			ask := m.confirm.ask(confirmPopup{glyph: glyphSearch, title: "Search",
				lines:  []string{oneLine(value), "Enter in the field: the page searches"},
				accept: "search", action: confirmSubmitField, node: id}, m.layer())
			return m, tea.Batch(m.input.close(), write, ask)
		}
		return m, tea.Batch(m.closeStack(), write)
	case inputPrompt:
		return m, tea.Batch(m.input.close(), m.answerDialog(true, value))
	case inputSetting:
		// An offer still standing means nothing was typed or declined.
		return m, m.saveSetting(value, value == "" && m.input.placeholder != "")
	case inputFolder:
		return m, tea.Batch(m.input.close(), m.addFolder(m.folderParent, value))
	case inputImportName:
		return m, m.importBookmarks(value)
	case inputBookmarkURL:
		return m, m.bookmarkURLGiven(value)
	case inputBookmarkTitle:
		return m, m.bookmarkTitleGiven(value)
	case inputRename:
		return m, m.renameGiven(value)
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
		return m, tea.Batch(m.input.close(), at.unblock(func(ctx context.Context) error { return page.Auth(ctx, a.id, user, value) }))
	}
	return m, m.closeStack()
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
	return t.unblock(func(ctx context.Context) error { return page.CancelAuth(ctx, a.id) })
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
		{n.Selected, "selected"}, {n.Multiline, "multiline"}, {n.Protected, "protected"}, {n.Current, "current"}, {n.Breadcrumb, "breadcrumb"}} {
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
	case confirmSubmitField:
		t := m.shownTab()
		if t == nil {
			return m, closeCmd
		}
		id := m.confirm.node
		return m, tea.Batch(closeCmd, t.press(func(ctx context.Context) error { return page.Submit(ctx, id) }))
	case confirmDeleteFolder:
		return m, tea.Batch(closeCmd, m.deleteFolderTree(m.confirm.folder))
	case confirmOpenLink:
		t := m.shownTab()
		if t == nil {
			return m, closeCmd
		}
		id := m.confirm.node
		return m, tea.Batch(closeCmd, t.press(func(ctx context.Context) error { return page.Click(ctx, id) }))
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

	// Bottom to top. The page's own popup first, under everything of
	// webu's: a menu opened on it lands above it (pagepopup.go).
	if t := m.shownTab(); t != nil && m.screen == screenWeb && t.popupNode() != nil {
		for _, f := range m.pagePopupFloats(t) {
			out = overlay.Composite(f.box, out, overlay.Center, overlay.Center, f.dx, f.dy)
		}
	}
	// Then the menu, so what it opened lands above it.
	if m.spaceMenu.isActive() {
		out = overlay.Composite(m.spaceMenu.view(), out, overlay.Center, overlay.Center, 0, 0)
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
	if m.finder.isActive() {
		out = overlay.Composite(m.finder.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.help.isActive() {
		out = overlay.Composite(m.help.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.confirm.isActive() {
		out = overlay.Composite(m.confirm.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.picker.isActive() {
		out = overlay.Composite(m.picker.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.input.isActive() {
		out = overlay.Composite(m.input.view(), out, overlay.Center, overlay.Center, 0, 0)
	}
	if m.editor.isActive() {
		out = overlay.Composite(m.editor.view(), out, overlay.Center, overlay.Center, 0, 0)
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
		case t.working():
			hint = "loading"
		case t.popupNode() != nil:
			hint = "a popup is up: answer it"
		case t.onPagetab():
			// The hand on the pagetab: what that part holds, and whether
			// it is the one being shown.
			hint = truncate(partHint(t), max(1, innerW-8))
		case t.listing(), t.read:
			// Which piece of how many, and what it is called — the one
			// thing a sheet of text cannot say about itself (section.go).
			hint = truncate(t.sectionHint(innerH-pageHeaderRows), max(1, innerW-8))
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
	// Inside an open section the border itself reads to how far down it
	// the reader is: the progress bar costs no row (pagepanel).
	body := fitLines(m.pageBody(innerW, innerH), innerW, innerH)
	if t := m.shownTab(); t != nil && t.read && hint != "" && !t.loading {
		return panelFrameFilled(innerW, body, "[2] Page",
			hintLegend([][2]string{{hint, ""}}), tone, t.readPct(innerH-pageHeaderRows))
	}
	return panelFrame(innerW, body, "[2] Page", hint, tone)
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

// openFinder opens [/] or [go] over the shown tab.
func (m AppModel) openFinder(kind finderKind) (tea.Model, tea.Cmd) {
	t := m.shownTab()
	if t == nil || t.root == nil {
		return m, m.toast.show("no page to search", toastInfo)
	}
	if t.popupNode() != nil {
		// A dialog is a few lines to answer: nothing in it is reached by
		// number or by search (user, 2026-09-23).
		return m, m.toast.show("a popup is answered, not searched", toastInfo)
	}
	if kind == finderGo {
		if t.lineCount() == 0 {
			return m, m.toast.show("this page has no lines to go to", toastInfo)
		}
		return m, m.finder.openGo(t, m.layer())
	}
	return m, m.finder.openSearch(t, m.layer())
}

// finderKey is a keystroke while the finder is up. Enter on a hit lands
// on it and closes the finder — in that order, so a landing that fails
// can say so with the finder still there.
func (m AppModel) finderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	h, ok := m.finder.update(msg)
	if !ok {
		return m, nil
	}
	t := m.shownTab()
	if t == nil {
		return m, m.finder.close()
	}
	if m.finder.kind == finderGo {
		t.goToLine(h.line, m.pageVisible())
		return m, m.finder.close()
	}
	if !m.goToHit(t, h) {
		return m, m.toast.show("that is not on the page any more", toastInfo)
	}
	return m, m.finder.close()
}

// goToHit takes the page to a search hit: the part it is in, the things
// it is inside drilled into, the section that holds it opened, and the
// window and cursor on its row. Nothing is pressed (finder.go).
//
// Everything about WHERE is read off the tree as it is now — the hit
// says which node, the tree says what it is inside (tab.chainOf): the
// page is recaptured while the finder is up, and a path remembered at
// index time would be a path through a tree that is gone.
func (m *AppModel) goToHit(t *tab, h hit) bool {
	w, vis := m.pageW(), m.pageVisible()
	chain := t.chainOf(h)
	if chain == nil {
		return false
	}
	node := chain[len(chain)-1]
	m.focus = panelPage
	t.showPart(h.part, w, vis)
	// Each list item or article on the way down is one thing until you
	// go in: go in.
	for _, a := range chain[:len(chain)-1] {
		if isThing(a) && !t.drillInto(a, w, vis) {
			return false
		}
	}
	t.reveal(node, w)
	// The node's own row, or the nearest ancestor's that has one — a
	// table's cell is the item, and what is in the cell lands on it.
	row := -1
	for i := len(chain) - 1; i >= 0 && row < 0; i-- {
		row = t.rowOf(chain[i])
	}
	if row < 0 {
		return false
	}
	if t.listing() || t.read {
		// A document: the section holding the row is the one to read.
		t.sec = sectionAt(t.secs, row)
		t.read = true
	}
	t.landRow(row, vis)
	return true
}

// editorKey is a keystroke while the textarea's box is up: the value it
// commits is typed into the field, the way a line's is.
func (m AppModel) editorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	value, done := m.editor.update(msg)
	if !done {
		return m, nil
	}
	t := m.shownTab()
	id := m.editor.node
	if t == nil {
		return m, m.editor.close()
	}
	return m, tea.Batch(m.editor.close(), t.press(func(ctx context.Context) error { return page.Type(ctx, id, value) }))
}
