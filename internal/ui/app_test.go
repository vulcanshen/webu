package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/store"
)

// driver runs an AppModel the way Bubble Tea would, without a terminal:
// commands execute in goroutines and their messages come back through one
// channel, batches and sequences are unpacked, and a test waits on a
// condition rather than on a fixed number of messages.
type driver struct {
	t    *testing.T
	m    AppModel
	msgs chan tea.Msg
}

func newDriver(t *testing.T, m AppModel) *driver {
	d := &driver{t: t, m: m, msgs: make(chan tea.Msg, 64)}
	d.exec(m.Init())
	return d
}

func (d *driver) exec(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		if msg := cmd(); msg != nil {
			d.msgs <- msg
		}
	}()
}

func (d *driver) send(msg tea.Msg) {
	// tea.Batch yields a BatchMsg; tea.Sequence a private slice of Cmds.
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			d.exec(c)
		}
		return
	}
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		cmds := make([]tea.Cmd, v.Len())
		for i := range cmds {
			cmds[i], _ = v.Index(i).Interface().(tea.Cmd)
		}
		go func() {
			for _, c := range cmds {
				if c == nil {
					continue
				}
				if m := c(); m != nil {
					d.msgs <- m
				}
			}
		}()
		return
	}
	// A capture that failed is worth a line in the log: the app shows it as
	// an error page, and a test that then times out says only "timed out".
	if pm, ok := msg.(pageMsg); ok && pm.err != nil {
		d.t.Logf("%s pageMsg tab %d gen %d: %v", time.Now().Format("15:04:05.000"), pm.tabID, pm.gen, pm.err)
	}
	model, cmd := d.m.Update(msg)
	d.m = model.(AppModel)
	d.exec(cmd)
}

func (d *driver) key(k string) {
	switch k {
	case "enter":
		d.send(tea.KeyMsg{Type: tea.KeyEnter})
	case "esc":
		d.send(tea.KeyMsg{Type: tea.KeyEscape})
	default:
		d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
}

// until pumps messages until cond holds, or fails the test.
func (d *driver) until(what string, cond func() bool) {
	d.t.Helper()
	start := time.Now()
	defer func() {
		// A step that took seconds is a step worth knowing about, whether
		// or not it eventually passed.
		if el := time.Since(start); el > 2*time.Second {
			d.t.Logf("%s took %s", what, el.Round(time.Millisecond))
		}
	}()
	deadline := time.After(20 * time.Second)
	for !cond() {
		select {
		case msg := <-d.msgs:
			d.send(msg)
		case <-deadline:
			d.t.Fatalf("timed out waiting for %s\n%s", what, d.m.View())
		}
	}
}

func (d *driver) page() *tab { return d.m.shownTab() }

// act is Enter on an item and Enter again on the first row of its menu —
// the obvious thing, done the way a user does it (ux.md §A.0.K).
func (d *driver) act() {
	d.t.Helper()
	d.key("enter")
	d.until("item menu", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optItemMenu })
	d.key("enter")
}

func (d *driver) loaded(title string) func() bool {
	return func() bool {
		t := d.page()
		return t != nil && !t.loading && t.root != nil && t.title == title
	}
}

// cursorOn moves the cursor down until it stands on an item whose text
// contains want.
func (d *driver) cursorOn(kind ir.Kind, want string) {
	d.t.Helper()
	t := d.page()
	for i, it := range t.lay.items {
		if it.node.Kind == kind && strings.Contains(it.node.Text()+it.node.Name, want) {
			t.cursor = i
			return
		}
	}
	d.t.Fatalf("no %s item containing %q in\n%s", kind, want, dumpLayout(t.lay))
}

func TestAppNavigatesAndFillsAForm(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir()) // the history log goes to a scratch dir, not the user's
	t.Setenv("WEBU_DATA", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	abs, _ := filepath.Abs("testdata/nav.html")

	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	if v := d.m.View(); !strings.Contains(v, "Page A") || !strings.Contains(v, "a link to B") {
		t.Fatalf("page not drawn:\n%s", v)
	}

	// Enter on a link opens its menu; Open is the first row, and it navigates.
	d.cursorOn(ir.Link, "a link to B")
	d.key("enter")
	d.until("item menu", func() bool { return d.m.options.isInteractive() })
	if got := d.m.options.items[d.m.options.cursor].label; got != "Open" {
		t.Errorf("first row of a link's menu is %q", got)
	}
	d.key("enter")
	d.until("page B", d.loaded("Page B"))
	if !strings.Contains(d.m.View(), "You made it.") {
		t.Fatalf("page B not drawn:\n%s", d.m.View())
	}

	// P goes back. From the keystroke the page shows as on its way — the
	// live glyph on the URL row — and a second P before it lands is
	// swallowed rather than stacked (ux.md §6).
	d.key("P")
	if !d.page().loading {
		t.Fatal("P should mark the tab loading at once")
	}
	if v := d.m.View(); !strings.Contains(v, glyphLive) {
		t.Errorf("the URL row should show the live glyph while loading:\n%s", v)
	}
	gen := d.page().gen
	d.key("P")
	if d.page().gen != gen {
		t.Error("a second P while the first is in flight was not swallowed")
	}
	d.until("page A again", d.loaded("Page A"))
	if !strings.Contains(d.page().url, "nav.html") {
		t.Errorf("url after back: %s", d.page().url)
	}
	if v := d.m.View(); !strings.Contains(v, glyphWeb) {
		t.Errorf("the URL row should show the web glyph at rest:\n%s", v)
	}
	// Nowhere further back — the entry before this one is the about:blank
	// every tab starts on, which does not count: says so, and the page
	// stays.
	d.key("P")
	d.until("told there is nothing", func() bool {
		return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "nothing to go back")
	})
	if d.page().loading || d.page().root == nil || !strings.Contains(d.page().url, "nav.html") {
		t.Errorf("a failed back should leave the page as it was: loading=%v url=%s", d.page().loading, d.page().url)
	}

	// An empty textbox's menu leads with Edit: the input popup; Enter there
	// writes the value into the page, and the next capture shows it.
	d.cursorOn(ir.Textbox, "Name")
	d.act()
	d.until("input popup", func() bool { return d.m.input.isInteractive() })
	d.key("hi there")
	d.key("enter")
	d.until("value written", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Textbox && n.Value == "hi there"
	})

	// A filled textbox's menu leads with Submit, which presses Enter in the
	// field, and the form's handler sees the value.
	d.key("enter")
	d.until("item menu", func() bool { return d.m.options.isInteractive() })
	if got := d.m.options.items[d.m.options.cursor].label; got != "Submit" {
		t.Errorf("cursor should rest on Submit, is on %q", got)
	}
	d.key("enter")
	d.until("form submitted", func() bool {
		return strings.Contains(dumpLayout(d.page().lay), "submitted:hi there")
	})

	// A select's menu leads with Choose, which swaps in the option list.
	d.cursorOn(ir.Combobox, "Pick")
	d.act()
	d.until("option list", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optSelect })
	d.key("j")
	d.key("enter")
	d.until("option chosen", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Combobox && n.Value == "Two"
	})
}

// TestListPopupsAndSession drives panel [1] without a browser: B opens the
// bookmarks, the filter narrows them, x asks before deleting, and the
// session written on the way out is what was open.
func TestScreensAndSession(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	m := New(nil, "").WithStore(
		[]store.Bookmark{{Title: "Hacker News", URL: "https://news.ycombinator.com/"}, {Title: "Go", URL: "https://go.dev"}},
		nil, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if h := d.m.header(); !strings.Contains(h, "[W]eb") || !strings.Contains(h, "[S]ettings") {
		t.Errorf("header: %q", h)
	}

	// B is the Bookmarks screen: the whole body, the chip lit, the web kept.
	d.key("B")
	if d.m.screen != screenBookmarks || len(d.m.lists.visible()) != 2 {
		t.Fatalf("screen %v, visible %d", d.m.screen, len(d.m.lists.visible()))
	}
	if v := d.m.View(); !strings.Contains(v, "Hacker News") || strings.Contains(v, "[1] Tabs") || !strings.Contains(v, "Title") {
		t.Errorf("the bookmarks screen should replace the web panels, with a column header:\n%s", v)
	}
	d.key("/")
	d.key("go")
	d.key("enter")
	if got := len(d.m.lists.visible()); got != 1 {
		t.Fatalf("filtered %d", got)
	}
	d.key("x")
	d.until("confirm", func() bool { return d.m.confirm.isInteractive() })
	d.key("enter")
	d.until("deleted", func() bool { return len(d.m.bookmarks) == 1 })
	if saved, _, _ := store.LoadBookmarks(); len(saved) != 1 || saved[0].Title != "Hacker News" {
		t.Errorf("bookmarks.yaml after delete: %+v", saved)
	}
	d.key("esc") // the filter
	if d.m.screen != screenBookmarks || d.m.lists.filter != "" {
		t.Fatalf("first Esc should clear the filter: screen %v filter %q", d.m.screen, d.m.lists.filter)
	}

	// F makes a folder; m moves the bookmark into it through a picker; the
	// folder is a row of its own, and x on it only goes when it is empty.
	d.key("F")
	d.until("folder box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputFolder })
	d.key("dev")
	d.key("enter")
	d.until("folder made", func() bool { return len(d.m.folders) == 1 })
	if e, _, ok := d.m.lists.current(); !ok || !e.isFolder || e.folder != "dev" {
		t.Fatalf("the cursor should land on the new folder: %+v", e)
	}
	d.key("esc") // the toast
	d.until("folder toast gone", func() bool { return !d.m.toast.anim.owns() })
	d.m.lists.cursorTo(0) // Hacker News, at the top level
	d.key("m")
	d.until("move picker", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optMoveTo })
	d.key("1")
	d.until("moved", func() bool { return d.m.bookmarks[0].Folder == "dev" })
	if saved, folders, _ := store.LoadBookmarks(); len(saved) != 1 || saved[0].Folder != "dev" || len(folders) != 1 {
		t.Errorf("bookmarks.yaml after move: %+v %v", saved, folders)
	}
	if e, _, ok := d.m.lists.current(); !ok || e.isFolder || e.folder != "dev" {
		t.Errorf("the cursor should follow the moved bookmark: %+v", e)
	}
	if v := d.m.View(); !strings.Contains(v, glyphFolder+" dev") {
		t.Errorf("the folder should be a row of its own:\n%s", v)
	}
	d.key("k") // up, onto the folder row
	if e, _, _ := d.m.lists.current(); !e.isFolder {
		t.Fatalf("k should land on the folder row: %+v", e)
	}
	d.key("x")
	if len(d.m.folders) != 1 {
		t.Error("a folder with a bookmark in it must not go")
	}
	d.key("esc") // the toast that said so
	d.until("toast gone", func() bool { return !d.m.toast.anim.owns() })

	// f on the folder row makes a folder inside it; the tree shows the
	// nesting, and so does the Move picker.
	d.key("f")
	d.until("subfolder box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputFolder })
	if !strings.Contains(d.m.input.prompt, "inside dev") {
		t.Errorf("the box should say where the folder goes: %q", d.m.input.prompt)
	}
	d.key("sub")
	d.key("enter")
	d.until("subfolder made", func() bool { return len(d.m.folders) == 2 && d.m.folders[1] == "dev/sub" })
	if e, _, ok := d.m.lists.current(); !ok || !e.isFolder || e.folder != "dev/sub" || e.depth != 1 {
		t.Fatalf("the cursor should be on the new subfolder, one level in: %+v", e)
	}
	d.key("esc") // the toast
	d.until("toast gone again", func() bool { return !d.m.toast.anim.owns() })
	d.m.lists.cursorTo(0) // Hacker News, in dev
	d.key("m")
	d.until("move picker again", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optMoveTo })
	if got := d.m.options.items[2].label; got != "  sub" {
		t.Errorf("the picker should indent the subfolder: %q", got)
	}
	d.key("2")
	d.until("moved into the subfolder", func() bool { return d.m.bookmarks[0].Folder == "dev/sub" })
	if e, _, ok := d.m.lists.current(); !ok || e.isFolder || e.depth != 2 {
		t.Errorf("the bookmark should sit two levels in: %+v", e)
	}
	d.key("esc") // back to the web
	if d.m.screen != screenWeb {
		t.Fatalf("Esc should go back to the web: %v", d.m.screen)
	}

	// Downloads: the header counts what is in flight, the screen lists it,
	// newest first, and the row follows the file as it lands.
	if h := d.m.header(); strings.Contains(h, "in flight") {
		t.Errorf("header at rest: %q", h)
	}
	d.send(downloadMsg{guid: "g1", name: "a.zip", url: "https://x/a.zip", begin: true})
	d.send(downloadMsg{guid: "g1", received: 512, total: 1024})
	if got := d.m.downloading(); got != 1 {
		t.Fatalf("downloading %d", got)
	}
	if h := d.m.header(); !strings.Contains(h, "1 download in flight") {
		t.Errorf("header: %q", h)
	}
	if pct, moving := d.m.downloadProgress(); pct != 50 || !moving {
		t.Errorf("rule progress: %d%% moving=%v", pct, moving)
	}
	d.key("D")
	if d.m.screen != screenDownloads {
		t.Fatalf("screen %v", d.m.screen)
	}
	if e, _, ok := d.m.lists.current(); !ok || e.title != "a.zip" || !strings.Contains(e.meta, "50%") {
		t.Errorf("download row: %+v %v", e, ok)
	}
	d.send(downloadMsg{guid: "g1", received: 1024, total: 1024, done: true, path: "/tmp/a.zip"})
	if e, _, _ := d.m.lists.current(); e.meta != "/tmp/a.zip" {
		t.Errorf("finished row: %+v", e)
	}
	if d.m.downloading() != 0 || strings.Contains(d.m.header(), "in flight") {
		t.Errorf("header after: %q", d.m.header())
	}
	d.key("C") // clear the done ones
	if len(d.m.dls) != 0 || len(d.m.lists.visible()) != 0 {
		t.Errorf("after clear: %+v", d.m.dls)
	}

	// Settings: one row, Enter edits it, the file and the row follow.
	d.key("S")
	if e, _, ok := d.m.lists.current(); d.m.screen != screenSettings || !ok || e.title != "download_dir" || !strings.HasPrefix(e.meta, "(default) ") {
		t.Fatalf("settings row: %+v %v (screen %v)", e, ok, d.m.screen)
	}
	d.key("enter")
	d.until("setting box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputSetting })
	if d.m.input.value != "" || !strings.HasSuffix(d.m.input.placeholder, "downloads") {
		t.Fatalf("the box should offer the value in force: value %q placeholder %q", d.m.input.value, d.m.input.placeholder)
	}
	d.key("enter") // the offer untouched: nothing changes
	d.until("box closed", func() bool { return !d.m.input.isActive() })
	if d.m.cfg.DownloadDir != "" {
		t.Fatalf("Enter on the untouched offer changed the setting: %q", d.m.cfg.DownloadDir)
	}
	d.key("enter")
	d.until("setting box again", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputSetting })
	d.send(tea.KeyMsg{Type: tea.KeyBackspace}) // decline the offer
	d.key("/tmp/dl")
	d.key("enter")
	d.until("saved", func() bool { return !d.m.input.isActive() })
	if cfg, _ := store.LoadConfig(); cfg.DownloadDir != "/tmp/dl" || d.m.cfg.DownloadDir != "/tmp/dl" {
		t.Errorf("config.yaml after save: %+v", cfg)
	}
	if e, _, _ := d.m.lists.current(); e.meta != "/tmp/dl" {
		t.Errorf("settings row after save: %+v", e)
	}
	d.key("W")
	if d.m.screen != screenWeb {
		t.Fatalf("W should be the web: %v", d.m.screen)
	}

	// No tabs, no browser: the session is empty and does not panic.
	if s := d.m.Session(); len(s.Tabs) != 0 {
		t.Errorf("session: %+v", s)
	}
}

func TestResolveURL(t *testing.T) {
	cases := map[string]string{
		"https://a.b/c":        "https://a.b/c",
		"news.ycombinator.com": "https://news.ycombinator.com",
		"github.com/x/y":       "https://github.com/x/y",
		"localhost:8080":       "http://localhost:8080",
		"what is a tui":        searchEngine + "what+is+a+tui",
		"golang":               searchEngine + "golang",
	}
	for in, want := range cases {
		if got := resolveURL(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestViewFitsTheTerminal(t *testing.T) {
	m := New(nil, "")
	for _, size := range [][2]int{{100, 30}, {72, 20}, {60, 15}, {40, 10}} {
		model, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		mm := model.(AppModel)
		for _, focus := range []panelID{panelTabs, panelPage} {
			mm.focus = focus
			lines := strings.Split(mm.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d focus %d: %d lines", size[0], size[1], focus, len(lines))
			}
			for i, l := range lines {
				if w := dispW(l); w != size[0] {
					t.Errorf("%dx%d focus %d line %d is %d wide: %q", size[0], size[1], focus, i, w, l)
				}
			}
		}
		// The list screens fill the same frame, empty or not.
		for _, s := range []screen{screenBookmarks, screenHistory, screenDownloads, screenSettings} {
			model, _ := mm.switchScreen(map[screen]string{screenBookmarks: "B", screenHistory: "H", screenDownloads: "D", screenSettings: "S"}[s])
			sm := model.(AppModel)
			lines := strings.Split(sm.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d screen %d: %d lines", size[0], size[1], s, len(lines))
			}
			for i, l := range lines {
				if w := dispW(l); w != size[0] {
					t.Errorf("%dx%d screen %d line %d is %d wide: %q", size[0], size[1], s, i, w, l)
				}
			}
		}
	}
}

// L is Chrome's Cmd+L: the box opens with the page's own URL on offer.
func TestGotoOffersThePageURL(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	d := newDriver(t, New(nil, ""))
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/a"}}
	d.m.shown = 0
	d.m.focus = panelTabs // from any panel, like Cmd+L

	d.key("L")
	d.until("goto open", func() bool { return d.m.input.isInteractive() })
	if d.m.input.value != "" || d.m.input.placeholder != "https://example.com/a" {
		t.Fatalf("value %q placeholder %q", d.m.input.value, d.m.input.placeholder)
	}
	if v := d.m.input.view(); !strings.Contains(v, "Tab") || !strings.Contains(v, "edit it") {
		t.Errorf("hint does not offer Tab:\n%s", v)
	}
	// Tab takes the offer into the line; typing then edits it.
	d.send(tea.KeyMsg{Type: tea.KeyTab})
	d.key("b")
	if d.m.input.value != "https://example.com/ab" || d.m.input.placeholder != "" {
		t.Fatalf("after Tab: value %q placeholder %q", d.m.input.value, d.m.input.placeholder)
	}
	d.key("esc")
	d.until("closed", func() bool { return !d.m.input.isActive() })

	// Backspace on the empty line declines it.
	d.key("L")
	d.until("goto open again", func() bool { return d.m.input.isInteractive() })
	d.send(tea.KeyMsg{Type: tea.KeyBackspace})
	if d.m.input.placeholder != "" {
		t.Fatalf("Backspace kept the offer %q", d.m.input.placeholder)
	}
	if v := d.m.input.view(); strings.Contains(v, "edit it") {
		t.Errorf("hint still offers Tab with nothing to take:\n%s", v)
	}
	d.key("esc")

	if got := bracketHotkey("URL", "L"); got != "UR[L]" {
		t.Errorf("bracketHotkey(URL, L) = %q", got)
	}
	if got := bracketHotkey("dir1", "1"); got != "[1] dir1" {
		t.Errorf("a digit key is never bracketed inside the label: %q", got)
	}

	// T on the page is the same new tab as T on the tabs list.
	d.m.focus = panelPage
	d.key("T")
	d.until("new tab box", func() bool { return d.m.input.isInteractive() })
	if d.m.input.action != inputGotoNewTab || d.m.input.placeholder != "" {
		t.Errorf("T on [2]: action %v placeholder %q", d.m.input.action, d.m.input.placeholder)
	}
	d.key("esc")
}
