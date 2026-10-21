package ui

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
	"github.com/vulcanshen/webu/internal/paths"
	"github.com/vulcanshen/webu/internal/store"
)

// webu's screen is terminu-go's (tdp E8): the lib draws the frames, the
// chip row, the footer, every popup, and handles the core keys; webu
// registers its screens, panels and actions, and draws what is inside
// panel [2] — the page — itself. AppModel is everything webu knows: the
// tabs, the files, the screens it registered.

// searchEngine is the default for a goto that is not a URL (ux.md §2.2);
// config.yaml can change it.
const searchEngine = store.DefaultSearch

// sideW is panel [1]'s outer width on the web screen, fixed (ui.md §1.2).
const sideW = 24

// narrowW is the width under which the web screen draws only the focused
// panel (layout/screen, narrow).
const narrowW = 72

// minW and minH are webu's least size (tdp L1); under it the lib draws
// only the name and the size it needs.
const minW, minH = 60, 20

// AppModel is webu: the lib's app, and webu's own state behind it.
type AppModel struct {
	a *terminu.App

	tabs      []*tab
	shown     int // index into tabs panel [2] displays; -1 for none
	nextTabID int
	browser   *browser.Browser
	events    chan tea.Msg  // page and browser events, read by waitEvent
	startURLs []string      // what the command line asked for, a tab each (function.md §12)
	session   store.Session // what to restore on the first frame
	started   bool
	// zoom: panel [2] alone fills the screen (ux.md §A.1 [Z]).
	zoom bool
	// sel is visual mode on the shown tab (ux.md §1; visual.go).
	sel selectMode

	// webu's own files, loaded by main and written back as they change.
	bookmarks []store.Bookmark
	folders   []string // bookmark folders declared on their own (bookmarks.go)
	cfg       store.Config
	history   []store.Visit

	// dialog is the page's question being asked (function.md §5); the page
	// is stalled until it is answered, and settle captures wait too.
	dialog *dialogMsg
	// auth is the HTTP challenge being answered.
	auth *authMsg
	// upload is the file chooser waiting on a path.
	upload *fileMsg
	// closed is the tabs closed this session, for [U]ndo close in [1].
	closed []store.SessionTab
	// dls is this session's downloads, oldest first: the Downloads screen
	// lists them, the chip row counts the ones still running.
	dls []download
	// foldedFolders are the bookmark folder rows shut with Enter.
	foldedFolders map[string]bool
	// unread are webu's own files main could not read: each asked about at
	// the start, moved aside or webu left (tdp E3). sessionUnread holds
	// session.yaml back from being written while it is one of them.
	unread        []Unread
	sessionUnread bool
	// moves are what main moved to where tdp E3 puts it now, or could not:
	// said on the first frame.
	moves []paths.Move

	// The screens (web.go, bookmarks.go, history.go, downloads.go,
	// settings.go, devtools.go).
	web       *terminu.Screen
	tabsList  *terminu.List
	tabsPanel *terminu.Panel
	pagePanel *terminu.Panel
	page      *pageBody

	bmScreen   *terminu.Screen
	bmList     *terminu.List
	bmPanel    *terminu.Panel
	histScreen *terminu.Screen
	histList   *terminu.List
	histPanel  *terminu.Panel
	dlScreen   *terminu.Screen
	dlList     *terminu.List
	dlPanel    *terminu.Panel
	setScreen  *terminu.Screen
	setValues  *terminu.Values
	setPanel   *terminu.Panel
	dev        *devtools
}

// New builds the app over a running browser. Every start argument — a
// URL, or words to search for, the same reading as the Location box —
// opens as a new tab after whatever the session restores, the first of
// them shown (function.md §12). Empty strings are ignored.
func New(b *browser.Browser, start ...string) *AppModel {
	var urls []string
	for _, s := range start {
		if strings.TrimSpace(s) != "" {
			urls = append(urls, strings.TrimSpace(s))
		}
	}
	m := &AppModel{
		shown:     -1,
		browser:   b,
		events:    make(chan tea.Msg, 16),
		startURLs: urls,
	}
	m.build()
	m.listenBrowser()
	return m
}

// WithStore hands the app its files. The UI never reads them itself.
func (m *AppModel) WithStore(bookmarks []store.Bookmark, folders []string, cfg store.Config, history []store.Visit) *AppModel {
	m.bookmarks, m.folders, m.cfg, m.history = bookmarks, folders, cfg, history
	m.applyConfig()
	m.refreshLists()
	return m
}

// WithSession is what was open last time; restored on the first frame,
// without loading (ux.md §6).
func (m *AppModel) WithSession(s store.Session) *AppModel {
	m.session = s
	return m
}

// Session is what to write down on the way out: every tab's URL, and which
// one panel [2] was on.
func (m *AppModel) Session() store.Session {
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

// Run runs the screen until the user leaves (tdp K9).
func (m *AppModel) Run() error { return m.a.Run() }

// Program is the lib's app, for a program webu does not start itself (the
// tests drive it message by message).
func (m *AppModel) Program() *terminu.App { return m.a }

// Close releases every tab. Chromium itself is the caller's to stop.
func (m *AppModel) Close() {
	for _, t := range m.tabs {
		t.close()
	}
}

// build registers webu with the lib: the five screens and DevTools, the
// quit flow, the splash, what to do with webu's own messages.
func (m *AppModel) build() {
	a := terminu.New("webu")
	m.a = a
	a.QuitDesc = "leave webu; the tabs are kept for next time"
	// webu's least size (tdp L1): 60 × 20, the narrowest its two panels
	// were laid out for (one at a time under narrowW) and room for its
	// forms; smaller, only the name is drawn.
	a.MinWidth, a.MinHeight = minW, minH
	a.Splash = splash()
	a.Status = m.status
	a.Progress = m.progress
	a.OnMsg = m.onMsg
	a.Start = m.start
	m.buildWeb()
	m.buildBookmarks()
	m.buildHistory()
	m.buildDownloads()
	m.buildSettings()
	m.buildDevtools()
	m.sync()
}

// start is the first frame: the event listener and what the app opens
// with — not while a file of webu's is still asked about (tdp E3): the
// last Enter opens it, as on the first start.
func (m *AppModel) start() tea.Cmd {
	cmds := []tea.Cmd{waitEvent(m.events)}
	if len(m.unread) == 0 {
		cmds = append(cmds, m.firstFrame())
	}
	return tea.Batch(append(cmds, m.devTick(), m.startNews())...)
}

// startNews says on the first screen what main found: the things it moved
// (tdp E3), the ones it could not, the files it could not read. The lib
// folds a popup opened before the terminal's size once the size is known.
func (m *AppModel) startNews() tea.Cmd {
	var cmds []tea.Cmd
	var moved, stuck []string
	for _, mv := range m.moves {
		if mv.Both {
			stuck = append(stuck, mv.What+" is in both "+paths.Short(mv.From)+" and "+paths.Short(mv.To)+
				". webu uses "+paths.Short(mv.To)+" and leaves the other as it is: merge them yourself.")
			continue
		}
		if mv.Err == nil {
			// A file is said by its directory, a directory by itself.
			from, to := mv.From, mv.To
			if filepath.Base(from) == mv.What {
				from, to = filepath.Dir(from), filepath.Dir(to)
			}
			moved = append(moved, mv.What+" from "+paths.Short(from)+" to "+paths.Short(to))
			continue
		}
		stuck = append(stuck, mv.What+" could not be moved from "+paths.Short(mv.From)+" to "+paths.Short(mv.To)+": "+
			mv.Err.Error()+". webu keeps using it where it is; move it yourself.")
	}
	if len(moved) > 0 {
		// The toast goes into the App Log by itself (components/dialog/note).
		cmds = append(cmds, m.toast("Moved "+strings.Join(moved, "; ")))
	}
	if len(stuck) > 0 {
		cmds = append(cmds, m.a.Error("Move failed", strings.Join(stuck, "\n")))
	}
	if len(m.unread) > 0 {
		cmds = append(cmds, m.moveAside(0))
	}
	return tea.Batch(cmds...)
}

// Unread is one of webu's own files that could not be read (tdp E3, when
// it cannot be read): its name, where it is, why.
type Unread struct {
	Name, Path string
	Err        error
}

// moveAside asks about the unread file i, then the next (tdp E3): Enter
// moves it to .bak and starts that file new, Esc leaves webu, the lib
// handing main the path and the reason for stderr. Past the last one,
// webu opens what it starts with.
func (m *AppModel) moveAside(i int) tea.Cmd {
	u := m.unread[i]
	return m.a.MoveAside(u.Path, u.Err, func() tea.Cmd {
		m.fresh(u.Name)
		m.sync()
		if i+1 < len(m.unread) {
			return m.moveAside(i + 1)
		}
		return m.firstFrame()
	})
}

// fresh starts one of webu's files new, after the unreadable one was moved
// aside: settings at their defaults, the rest empty.
func (m *AppModel) fresh(name string) {
	switch name {
	case "config.yaml":
		m.cfg = store.Config{}
		m.applyConfig()
		m.refreshSettings()
	case "bookmarks.yaml":
		m.bookmarks, m.folders = nil, nil
		m.refreshBookmarks()
	case "history.yaml":
		m.history = nil
		m.refreshHistory()
	case "session.yaml":
		m.sessionUnread = false
	}
}

// SessionWritable says whether session.yaml may be written on the way out:
// not while it could not be read and was not moved aside, so that leaving
// from its question writes nothing over it.
func (m *AppModel) SessionWritable() bool { return !m.sessionUnread }

// sync puts webu's state where the lib reads it: panel [1]'s rows, panel
// [2]'s name, tabs, loading icon and actions, the quit flow's question.
// It runs after every message webu handles and every action it runs, and
// again as panel [2] is drawn.
func (m *AppModel) sync() {
	m.syncTabs()
	m.syncPage()
	m.syncQuit()
}

// do wraps an action so that what it changed reaches the screen.
func (m *AppModel) do(f func() tea.Cmd) func() tea.Cmd {
	return func() tea.Cmd {
		cmd := f()
		m.sync()
		return cmd
	}
}

// toast says one short thing at the bottom (components/dialog/toast).
func (m *AppModel) toast(text string) tea.Cmd { return m.a.Toast(terminu.Info, text) }

// toastOK says an action is done: a Success toast (components/dialog/toast).
func (m *AppModel) toastOK(text string) tea.Cmd { return m.a.Toast(terminu.Success, text) }

// toastErr says something went wrong (tdp F5); the App Log keeps it after
// the toast goes.
func (m *AppModel) toastErr(text string) tea.Cmd { return m.a.Toast(terminu.Error, text) }

// saveFailed is one of webu's files not written: an error popup, the whole
// of the reason readable (components/dialog/note).
func (m *AppModel) saveFailed(file string, err error) tea.Cmd {
	return m.a.Error("Save failed", file+": "+err.Error())
}

// status is the chip row's right end (layout/screen, statusbar): the
// downloads still running, in Green.
func (m *AppModel) status(w int) []terminu.Span {
	// A field of its own width, whatever the count (tdp L2).
	const field = 22
	n := m.downloading()
	if n == 0 {
		return []terminu.Span{{Text: spaces(min(field, w))}}
	}
	return []terminu.Span{{Text: padLeft(plural(n, "download")+" in flight", min(field, w)), Style: terminu.Fg(terminu.Green)}}
}

// progress is the downloads' progress for the line under the chips.
func (m *AppModel) progress() (float64, bool) {
	pct, moving := m.downloadProgress()
	if !moving {
		return 0, false
	}
	return float64(pct) / 100, true
}

// syncQuit asks before quitting only when a download would be cut off
// (ux.md §5).
func (m *AppModel) syncQuit() {
	n := m.downloading()
	if n == 0 {
		m.a.Quit = nil
		return
	}
	m.a.Quit = &terminu.Ask{Title: "Quit", Question: "Quit webu?", Verb: "quit",
		Details: []string{plural(n, "download") + " still in progress. Quitting stops it."}, Warn: []int{0}}
}

// ------------------------------------------------------------------ update

// onMsg handles webu's own messages: the page, the browser, the files.
func (m *AppModel) onMsg(msg tea.Msg) tea.Cmd {
	cmd := m.handle(msg)
	m.sync()
	return cmd
}

func (m *AppModel) handle(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case devTickMsg, storageMsg, bodyMsg, evalMsg, devSourceMsg:
		return m.devMsg(msg)

	case dialogMsg:
		return tea.Batch(waitEvent(m.events), m.askDialog(msg))

	case newTargetMsg:
		// A window the page opened becomes a tab at the end of the list, and
		// panel [2] switches to it, the way Chrome does (ux.md §6).
		t := m.newTabFor(msg.id)
		t.url = msg.url
		m.tabs = append(m.tabs, t)
		left := m.leaveSelect()
		m.shown = len(m.tabs) - 1
		return tea.Batch(left, waitEvent(m.events), t.adopt(), m.goWeb(), m.focusPage())

	case downloadMsg:
		return tea.Batch(waitEvent(m.events), m.noteDownload(msg))

	case authMsg:
		// Name and password; the request waits.
		m.auth = &msg
		return tea.Batch(waitEvent(m.events), m.askAuth(msg))

	case fileMsg:
		// The page asks for a file: the file-picker, walked rather than
		// typed (user, 2026-09-23). One file at a time, even where the page
		// would take several.
		m.upload = &msg
		return tea.Batch(waitEvent(m.events), m.a.PickFile(terminu.FilePicker{
			Title: "Upload", Glyph: glyphUpload, Root: importDir(),
			Done: m.cmdDone(m.uploadPicked)}))

	case clipboardDoneMsg:
		if msg.err != nil {
			return m.toastErr(clipboardFailure(msg.err))
		}
		return m.toastOK("Copied " + plural(msg.lines, "line"))

	case actionErrMsg:
		// A closed tab's work comes back cancelled: the tab is gone, and
		// so is anything to say about it.
		if errors.Is(msg.err, context.Canceled) {
			return nil
		}
		return m.a.Error(msg.what+" failed", msg.err.Error())

	case navFailMsg:
		_, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return nil
		}
		t.loading, t.navigating = false, false
		if errors.Is(msg.err, page.ErrNoEntry) {
			return m.toast("Nothing to go " + msg.what + " to")
		}
		return m.a.Error("Going "+msg.what+" failed", msg.err.Error())

	case pageEventMsg:
		// Chromium says the tab moved or changed; look again shortly. Always
		// re-arm the listener, or the next event has nowhere to go.
		_, t := m.tabByID(msg.tabID)
		if t == nil {
			return waitEvent(m.events)
		}
		return tea.Batch(waitEvent(m.events), t.settle(250*time.Millisecond))

	case settleMsg:
		_, t := m.tabByID(msg.tabID)
		if t == nil || msg.gen != t.gen {
			return nil
		}
		// A page with a dialog up answers no CDP call that touches it; the
		// capture would only time out. It is re-asked once the dialog is.
		if m.dialog != nil && m.dialog.tabID == t.id {
			t.loading = false
			return nil
		}
		return t.refresh()

	case pageMsg:
		return m.applyPage(msg)
	}
	return nil
}

// cmdDone adapts a func(string) tea.Cmd to a Done that syncs after it.
func (m *AppModel) cmdDone(f func(string) tea.Cmd) func(string) tea.Cmd {
	return func(v string) tea.Cmd {
		cmd := f(v)
		m.sync()
		return cmd
	}
}

// applyPage is a capture landing on its tab.
func (m *AppModel) applyPage(msg pageMsg) tea.Cmd {
	i, t := m.tabByID(msg.tabID)
	if t == nil || msg.gen != t.gen {
		return nil
	}
	// Visual mode holds the page still (ux.md §1): the capture is kept and
	// applied when the mode ends.
	if m.sel.on && i == m.shown {
		t.frozen = &msg
		return nil
	}
	fresh := msg.url != "" && msg.url != t.lastVisit
	if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
		// The page panel says it failed; the App Log keeps what and why
		// (tdp E3: what is outside webu, a panel error with details logged).
		m.a.Log(terminu.Error, "Loading "+nameOr(msg.url, t.url)+": "+msg.err.Error())
	}
	t.apply(msg, m.pageW())
	if !fresh {
		t.scrollToCursor(m.pageVisible())
	}
	if id := t.wantDrill; id != 0 && !t.working() {
		// The frame Enter asked for: in, now that its document is here —
		// or not, when the frame answered nothing.
		t.wantDrill = 0
		if n := nodeByID(t.root, id); n != nil && len(n.Children) > 0 {
			t.drillInto(n, m.pageW(), m.pageVisible())
		} else {
			// A failure, so an Error toast (components/dialog/toast).
			return m.toastErr("The frame could not be entered; its address is a [Space] › Yank away")
		}
	}
	if t.stillComing() {
		// An application's shell: <body> exists and nothing is drawn yet.
		// Keep saying so, and look again.
		t.loading = true
		return t.settle(300 * time.Millisecond)
	}
	if t.settling() {
		// The page is still being built: this look does not agree with the
		// last one. Look again, and go on saying so.
		return t.settle(300 * time.Millisecond)
	}
	visited := m.recordVisit(t)
	if t.refreshErr != "" {
		// The page looked at again and could not be had: the one on screen
		// stays, and a toast says so (layout/panel, loading and errors).
		e := t.refreshErr
		t.refreshErr = ""
		return tea.Batch(visited, m.toastErr("Could not look at the page again: "+e))
	}
	if visited != nil {
		return visited
	}
	if i == m.shown && t.certErr && !t.certAsked {
		t.certAsked = true
		// The address that failed, kept now: the error page Chromium draws
		// instead is captured too, and by the time the question is answered
		// the tab's URL is chrome-error://.
		id, failed := t.id, t.url
		return m.a.Confirm(terminu.Ask{Title: "Continue",
			Details:  []string{"Chromium does not trust this site's certificate.", failed},
			Question: "Continue anyway, for this tab, for as long as it is open?",
			Verb:     "continue", Warn: []int{0}}, m.do(func() tea.Cmd { return m.trustCert(id, failed) }))
	}
	if n := t.current(); i == m.shown && msg.err == nil && n != nil {
		id := n.ID
		return t.act(func(ctx context.Context) error { return page.Reveal(ctx, id) })
	}
	return nil
}

// trustCert is the certificate confirm's Enter: the tab ignores
// certificate errors from now on and loads the address that failed again.
func (m *AppModel) trustCert(tabID int, url string) tea.Cmd {
	_, t := m.tabByID(tabID)
	if t == nil {
		return nil
	}
	ctx := t.ctx
	ignore := func() tea.Msg {
		if err := page.IgnoreCertErrors(ctx); err != nil {
			return actionErrMsg{what: "Continue", err: err}
		}
		return nil
	}
	return tea.Sequence(ignore, t.load(url))
}

// firstFrame opens what the app starts with: the session's tabs, unloaded
// except the one that was showing; and the command line's URLs, a new tab
// each, the first of them in front, when there are any.
func (m *AppModel) firstFrame() tea.Cmd {
	if m.started {
		return nil
	}
	m.started = true
	var cmds []tea.Cmd
	for _, st := range m.session.Tabs {
		t := m.newTab()
		t.url, t.title, t.pending = st.URL, st.Title, true
		m.tabs = append(m.tabs, t)
	}
	if len(m.tabs) > 0 {
		m.shown = clamp(m.session.Shown, 0, len(m.tabs)-1)
		// Done in the background as webu starts: the App Log says so.
		m.a.Log(terminu.Info, "Reopened "+plural(len(m.tabs), "tab")+" from the last session")
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
	m.sync()
	return tea.Batch(cmds...)
}

// pointDownloads tells the browser where downloads go (ui.md §6): on the
// first frame, and again when the setting changes.
func (m *AppModel) pointDownloads() tea.Cmd {
	if m.browser == nil {
		return nil
	}
	ctx, dir := m.browser.Ctx, m.downloadDir()
	return func() tea.Msg {
		// Chromium does not create the directory it is pointed at.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return actionErrMsg{what: "Download folder", err: err}
		}
		if err := page.SetDownloads(ctx, dir); err != nil {
			return actionErrMsg{what: "Download folder", err: err}
		}
		return nil
	}
}

// downloadDir is config.yaml's download_dir, or ~/.webu/datas/downloads
// (ui.md §6). ~ is expanded.
func (m *AppModel) downloadDir() string {
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

// recordVisit appends a loaded page to the history log (ux.md §6): the
// final URL and title after the load, once per page, never about:blank or
// an error page. Written synchronously — one line, a small file.
func (m *AppModel) recordVisit(t *tab) tea.Cmd {
	if t.errText != "" || t.root == nil || t.url == "" || t.url == "about:blank" || t.url == t.lastVisit {
		return nil
	}
	t.lastVisit = t.url
	v := store.Visit{At: time.Now(), URL: t.url, Title: t.title}
	if err := store.AppendVisit(v); err != nil {
		return m.toastErr("Save failed: history.yaml: " + err.Error())
	}
	m.history = append([]store.Visit{v}, m.history...)
	m.refreshHistory()
	return nil
}

// WithUnread are webu's own files main could not read (tdp E3): each is
// asked about on the first frame, MOVE ASIDE, rather than webu going on
// with it and a later save writing over it.
func (m *AppModel) WithUnread(files []Unread) *AppModel {
	m.unread = files
	for _, u := range files {
		if u.Name == "session.yaml" {
			m.sessionUnread = true
		}
	}
	return m
}

// WithMoves are what main moved from where an older webu kept it (tdp E3):
// an Info toast on the first frame says from where to where, and an error
// popup names both places of one it could not move.
func (m *AppModel) WithMoves(moves []paths.Move) *AppModel {
	m.moves = moves
	return m
}

// ------------------------------------------------------------------- tabs

func (m *AppModel) shownTab() *tab {
	if m.shown < 0 || m.shown >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.shown]
}

func (m *AppModel) tabByID(id int) (int, *tab) {
	for i, t := range m.tabs {
		if t.id == id {
			return i, t
		}
	}
	return -1, nil
}

// openTab creates a tab at url; show says whether panel [2] switches to it
// (a new tab does, ux.md §6).
func (m *AppModel) openTab(url string, show bool) tea.Cmd {
	t := m.newTab()
	m.tabs = append(m.tabs, t)
	if show || m.shown < 0 {
		left := m.leaveSelect()
		m.shown = len(m.tabs) - 1
		return tea.Batch(left, t.load(url), m.goWeb(), m.focusPage())
	}
	return t.load(url)
}

// openTabBehind opens url in a new tab, which [2] shows, and leaves the
// screen and the focus where they are: Enter on a list does not move the
// focus (tdp K3), so the toast says where the page went.
func (m *AppModel) openTabBehind(url string) tea.Cmd {
	t := m.newTab()
	m.tabs = append(m.tabs, t)
	left := m.leaveSelect()
	m.shown = len(m.tabs) - 1
	return tea.Batch(left, t.load(url), m.toast("Opened in a new tab; press [W] to see it"))
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
func (m *AppModel) showTab(i int) tea.Cmd {
	if i < 0 || i >= len(m.tabs) {
		return nil
	}
	left := m.leaveSelect()
	m.shown = i
	t := m.tabs[i]
	if t.pending {
		return tea.Batch(left, t.load(t.url))
	}
	if t.layW != m.pageW() {
		t.relayout(m.pageW())
	}
	return left
}

func (m *AppModel) closeTab(i int) tea.Cmd {
	if i < 0 || i >= len(m.tabs) {
		return nil
	}
	// The shown tab's held capture is for a tab going away: dropped.
	if i == m.shown {
		_ = m.leaveSelect()
	}
	m.remember(m.tabs[i])
	m.tabs[i].close()
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	switch {
	case len(m.tabs) == 0:
		m.shown = -1
	case m.shown == i:
		m.shown = min(i, len(m.tabs)-1)
	case m.shown > i:
		m.shown--
	}
	if t := m.shownTab(); t != nil && t.pending {
		return t.load(t.url)
	}
	return nil
}

// resolveURL turns what was typed into somewhere to go (ux.md §2.2): a URL
// gets its scheme, anything else is a search.
func (m *AppModel) resolveURL(s string) string {
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

// inspectLines is what the Inspect viewer shows about a node (ux.md §A.1):
// the facts the IR has, nothing invented.
func inspectLines(n *ir.Node) []string {
	lines := []string{
		"role     " + n.Role + " (" + n.Kind.String() + ")",
		"name     " + strings.TrimSpace(n.Name),
	}
	if n.Value != "" {
		lines = append(lines, "value    "+strings.TrimSpace(n.Value))
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
