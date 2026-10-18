package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chromedp/cdproto/cdp"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/store"
)

// driver runs webu the way Bubble Tea would, without a terminal: the lib's
// app takes every message, commands execute in goroutines and their
// messages come back through one channel, batches and sequences are
// unpacked, and a test waits on a condition rather than on a number of
// messages. What is on screen is read off the lib's frame.
type driver struct {
	t    *testing.T
	m    *AppModel
	msgs chan tea.Msg
}

func newDriver(t *testing.T, m *AppModel) *driver {
	d := &driver{t: t, m: m, msgs: make(chan tea.Msg, 256)}
	d.exec(m.a.Init())
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
	if _, quit := msg.(tea.QuitMsg); quit {
		return
	}
	// A capture that failed is worth a line in the log: the app shows it as
	// an error page, and a test that then times out says only "timed out".
	if pm, ok := msg.(pageMsg); ok && pm.err != nil {
		d.t.Logf("%s pageMsg tab %d gen %d: %v", time.Now().Format("15:04:05.000"), pm.tabID, pm.gen, pm.err)
	}
	_, cmd := d.m.a.Update(msg)
	d.exec(cmd)
}

// key sends a keystroke: a key's name, or text typed.
func (d *driver) key(k string) {
	switch k {
	case "enter":
		d.send(tea.KeyMsg{Type: tea.KeyEnter})
	case "esc":
		d.send(tea.KeyMsg{Type: tea.KeyEscape})
	case "backspace":
		d.send(tea.KeyMsg{Type: tea.KeyBackspace})
	case "tab":
		d.send(tea.KeyMsg{Type: tea.KeyTab})
	case " ":
		d.send(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	default:
		d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
}

// press is a key and then the time a popup takes to open: the lib takes
// no keys until it has (Rules F2).
func (d *driver) press(k string) {
	d.key(k)
	d.settle()
}

// settle pumps messages for as long as a popup's animation takes.
func (d *driver) settle() {
	deadline := time.After(400 * time.Millisecond)
	for {
		select {
		case msg := <-d.msgs:
			d.send(msg)
		case <-deadline:
			return
		}
	}
}

// view is the screen as text, styles stripped.
func (d *driver) view() string { return terminu.StripStyles(strings.Join(d.m.a.Frame(), "\n")) }

// shows is a condition: the screen has s on it.
func (d *driver) shows(s string) func() bool {
	return func() bool { return strings.Contains(d.view(), s) }
}

// until pumps messages until cond holds, or fails the test.
func (d *driver) until(what string, cond func() bool) {
	d.t.Helper()
	start := time.Now()
	defer func() {
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
			d.t.Fatalf("timed out waiting for %s\n%s", what, d.view())
		}
	}
}

func (d *driver) page() *tab { return d.m.shownTab() }

// act is Enter on an item, the way a user does it (ux.md §A.0.K): the
// click itself, and for a link — which asks first — Enter again on the
// confirm.
func (d *driver) act() {
	d.t.Helper()
	n := d.page().current()
	d.press("enter")
	if n != nil && n.Kind == ir.Link {
		d.until("open link?", d.shows("OPEN LINK"))
		d.press("enter")
	}
}

func (d *driver) loaded(title string) func() bool {
	return func() bool {
		t := d.page()
		// working, not loading: a page is loaded when webu has stopped
		// looking at it, which is one look after the first (tab.settling).
		return t != nil && !t.working() && t.root != nil && t.title == title
	}
}

// cursorOn puts the cursor on an item whose text contains want.
func (d *driver) cursorOn(kind ir.Kind, want string) {
	d.t.Helper()
	t := d.page()
	for i, it := range t.lay.items {
		if it.node.Kind == kind && strings.Contains(it.node.Text()+it.node.Name, want) {
			t.cursor = i
			d.m.sync()
			return
		}
	}
	d.t.Fatalf("no %s item containing %q in\n%s", kind, want, dumpLayout(t.lay))
}

// nodeNamed is the item on the page whose name has name in it.
func nodeNamed(t *tab, name string) *ir.Node {
	for _, it := range t.lay.items {
		if strings.Contains(it.node.Name, name) {
			return it.node
		}
	}
	return nil
}

// bmSelect puts the bookmarks cursor on a row as a key would: the actions
// that name the row are worked out again (OnMove runs after a message).
func (d *driver) bmSelect(r bmRow) {
	d.m.bmList.Select(bmRowID(r))
	d.m.bmPanel.Actions = d.m.bookmarkActions()
}

// action runs one of the focused panel's actions by its label, as the
// Space menu would.
func (d *driver) action(label string) {
	d.t.Helper()
	p := d.m.a.Screen().Focused()
	for _, a := range p.Actions {
		if a.Label == label {
			if a.Disabled != nil && a.Disabled() {
				d.t.Fatalf("%q is disabled", label)
			}
			d.exec(a.Do())
			d.settle()
			return
		}
	}
	d.t.Fatalf("no action %q on [%s]", label, p.Label)
}

// launch starts the pinned Chromium, or skips the test.
func launch(t *testing.T) *browser.Browser {
	t.Helper()
	t.Setenv("WEBU__CONFIG", t.TempDir()) // the history log goes to a scratch dir, not the user's
	t.Setenv("WEBU__DATA", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}

// fixture is a testdata page as a file URL.
func fixture(name string) string {
	abs, _ := filepath.Abs("testdata/" + name)
	return "file://" + abs
}

func TestAppNavigatesAndFillsAForm(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("nav.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	if v := d.view(); !strings.Contains(v, "Page A") || !strings.Contains(v, "a link to B") {
		t.Fatalf("page not drawn:\n%s", v)
	}

	// Enter on a link asks first — its text and URL — and Esc leaves the
	// page where it is.
	d.cursorOn(ir.Link, "a link to B")
	d.press("enter")
	d.until("open link?", d.shows("OPEN LINK"))
	if v := d.view(); !strings.Contains(v, "nav2.html") {
		t.Errorf("the confirm should show the link's URL:\n%s", v)
	}
	d.press("esc")
	if !strings.Contains(d.page().url, "nav.html") {
		t.Errorf("Esc should stay on page A, is on %s", d.page().url)
	}
	// Enter on the confirm follows it.
	d.press("enter")
	d.until("open link?", d.shows("OPEN LINK"))
	d.press("enter")
	d.until("page B", d.loaded("Page B"))
	if !strings.Contains(d.view(), "You made it.") {
		t.Fatalf("page B not drawn:\n%s", d.view())
	}

	// P goes back. From the keystroke the page shows as on its way, and a
	// second P before it lands is swallowed rather than stacked (ux.md §6).
	d.key("P")
	if !d.page().loading || !d.m.pagePanel.Loading {
		t.Fatal("P should mark the tab loading at once, and [2] say so")
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
	// Nowhere further back: Previous is disabled, and its key does nothing.
	if d.page().canBack {
		t.Error("the entry before this one is about:blank, which does not count")
	}

	// Enter on a textbox is the box itself — the input popup, no menu in
	// between. The title says what the box takes, the prompt which field.
	d.cursorOn(ir.Textbox, "Name")
	d.press("enter")
	d.until("input popup", d.shows("EDIT: Name"))
	if v := d.view(); !strings.Contains(v, "Name") {
		t.Errorf("the field's name over the box:\n%s", v)
	}
	d.key("hi there")
	d.key("enter")
	d.until("value written", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Textbox && n.Value == "hi there"
	})
	d.settle()

	// A password box is known as one while still empty, and is masked.
	d.cursorOn(ir.Textbox, "Secret")
	d.press("enter")
	d.until("password box", d.shows("EDIT: Secret"))
	d.key("s3cret")
	if strings.Contains(d.view(), "s3cret") {
		t.Error("a password is masked from the first keystroke")
	}
	d.key("enter")
	d.until("secret written", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Textbox && n.Protected && n.Value != ""
	})
	if v := d.page().current().Value; strings.Contains(v, "s3cret") {
		t.Errorf("the page should hand back dots, not the secret: %q", v)
	}
	d.settle()

	// Submit is an item operation of a filled field: it presses Enter in
	// the field, and the form's handler sees the value.
	d.cursorOn(ir.Textbox, "Name")
	d.action("Submit")
	d.until("form submitted", func() bool {
		return strings.Contains(dumpLayout(d.page().lay), "submitted:hi there")
	})

	// A select drops its options; j and Enter choose the next one.
	d.cursorOn(ir.Combobox, "Pick")
	d.press("enter")
	d.until("option list", d.shows("CHOOSE"))
	d.key("j")
	d.key("enter")
	d.until("option chosen", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Combobox && n.Value == "Two"
	})
}

// TestScreensAndSession drives the list screens without a browser.
func TestScreensAndSession(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New(nil, "").WithStore(
		[]store.Bookmark{{Title: "Hacker News", URL: "https://news.ycombinator.com/"}, {Title: "Go", URL: "https://go.dev"}},
		nil, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if v := d.view(); !strings.Contains(v, "[W]EB") || !strings.Contains(v, "[S]ETTINGS") || !strings.Contains(v, "[I] DEVTOOLS") {
		t.Errorf("the chip row names every screen:\n%s", v)
	}

	// B is the Bookmarks screen: the whole body, the web kept.
	d.key("B")
	if d.m.a.Screen() != d.m.bmScreen || len(d.m.bmList.Rows()) != 2 {
		t.Fatalf("on the bookmarks screen with both rows: %d", len(d.m.bmList.Rows()))
	}
	if v := d.view(); !strings.Contains(v, "Hacker News") || strings.Contains(v, "[1] TABS") || !strings.Contains(v, "TITLE") {
		t.Errorf("the bookmarks screen should replace the web panels, with a column header:\n%s", v)
	}
	d.key("/")
	d.key("go")
	d.key("enter")
	if d.m.bmList.Current(); !d.m.bmList.Filtering() {
		t.Fatal("/ should filter")
	}
	d.press("x")
	d.until("confirm", d.shows("DELETE"))
	d.press("enter")
	d.until("deleted", func() bool { return len(d.m.bookmarks) == 1 })
	if saved, _, _ := store.LoadBookmarks(); len(saved) != 1 || saved[0].Title != "Hacker News" {
		t.Errorf("bookmarks.yaml after delete: %+v", saved)
	}
	d.key("esc") // the filter
	if d.m.bmList.Filtering() {
		t.Fatal("Esc should clear the filter")
	}

	// A makes a folder — a path makes every level; m moves the bookmark
	// into one; Enter on a folder folds it.
	d.press("A")
	d.until("folder box", d.shows("ADD FOLDER"))
	d.key("dev/sub")
	d.press("enter")
	d.until("folders made", func() bool { return len(d.m.folders) == 2 && d.m.folders[1] == "dev/sub" })
	if r, ok := d.m.bmCurrent(); !ok || !r.isFolder || r.folder != "dev/sub" {
		t.Fatalf("the cursor should land on the new folder: %+v", r)
	}
	d.m.bmList.Select(bmRowID(bmRow{ref: 0}))
	d.press("m")
	d.until("move select", d.shows("MOVE TO"))
	d.key("j")
	d.key("j")
	d.press("enter")
	d.until("moved", func() bool { return d.m.bookmarks[0].Folder == "dev/sub" })
	if r, ok := d.m.bmCurrent(); !ok || r.isFolder || r.ref != 0 {
		t.Errorf("the cursor should follow the moved bookmark: %+v", r)
	}
	if v := d.view(); !strings.Contains(v, glyphFolder+" dev") || !strings.Contains(v, glyphFolder+" sub") {
		t.Errorf("both folders should be rows of their own:\n%s", v)
	}
	d.m.bmList.Select(bmRowID(bmRow{folder: "dev", isFolder: true}))
	d.key("enter")
	if got := len(d.m.bmList.Rows()); got != 1 {
		t.Fatalf("folded dev should leave one row, not %d", got)
	}
	d.key("enter")
	if got := len(d.m.bmList.Rows()); got != 3 {
		t.Fatalf("unfolded dev should show three rows, not %d", got)
	}

	// Downloads: the chip row counts what is in flight, the screen lists
	// it, and the row follows the file as it lands.
	d.send(downloadMsg{guid: "g1", name: "a.zip", url: "https://x/a.zip", begin: true})
	d.send(downloadMsg{guid: "g1", received: 512, total: 1024})
	if v := d.view(); !strings.Contains(v, "1 download in flight") {
		t.Errorf("the chip row should count the download:\n%s", v)
	}
	if pct, moving := d.m.downloadProgress(); pct != 50 || !moving {
		t.Errorf("progress: %d%% moving=%v", pct, moving)
	}
	d.settle()
	d.key("D")
	if d.m.a.Screen() != d.m.dlScreen || !strings.Contains(d.view(), "50%") {
		t.Fatalf("the downloads screen with the row:\n%s", d.view())
	}
	d.send(downloadMsg{guid: "g1", received: 1024, total: 1024, done: true, path: "/tmp/a.zip"})
	if !strings.Contains(d.view(), "/tmp/a.zip") || d.m.downloading() != 0 {
		t.Errorf("the finished row:\n%s", d.view())
	}
	d.settle()
	d.key("C") // clear the done ones
	if len(d.m.dls) != 0 || len(d.m.dlList.Rows()) != 0 {
		t.Errorf("after clear: %+v", d.m.dls)
	}

	// Settings: Enter edits a value, the file follows; a switch flips in
	// place and is written at once.
	d.key("S")
	if d.m.a.Screen() != d.m.setScreen || !strings.Contains(d.view(), "SEARCH_ENGINE") {
		t.Fatalf("the settings screen:\n%s", d.view())
	}
	// download_dir is picked: a path typed lists that directory, and Enter
	// on its "." chooses it.
	dl := filepath.Join(t.TempDir(), "dl")
	if err := os.Mkdir(dl, 0o700); err != nil {
		t.Fatal(err)
	}
	d.key("j")
	d.press("enter")
	d.until("the picker", d.shows("DOWNLOAD TO"))
	d.key("/")
	d.key(dl + "/")
	d.key("enter")
	d.press("enter")
	d.until("saved", func() bool { return d.m.cfg.DownloadDir == dl })
	if cfg, _ := store.LoadConfig(); cfg.DownloadDir != dl {
		t.Errorf("config.yaml after save: %+v", cfg)
	}
	d.key("j")
	d.press("enter")
	d.until("measure box", d.shows("SETTINGS: measure"))
	d.key("backspace")
	d.key("wide")
	d.key("enter")
	if d.m.cfg.Measure != 0 || !strings.Contains(d.view(), "20 or more") {
		t.Fatalf("a bad measure should be refused in the box: %d\n%s", d.m.cfg.Measure, d.view())
	}
	// Esc takes a toast first (tdp K4), then the box.
	for i := 0; i < 3 && strings.Contains(d.view(), "SETTINGS: measure"); i++ {
		d.press("esc")
	}
	d.key("j")
	d.key("enter") // restore_session's switch
	if d.m.cfg.Restore() {
		t.Fatalf("Enter should switch restore_session off:\n%s", d.view())
	}
	if cfg, _ := store.LoadConfig(); cfg.Restore() {
		t.Errorf("config.yaml after the switch: %+v", cfg)
	}
	d.settle()
	d.key("W")
	if !d.m.onWeb() {
		t.Fatal("W should be the web")
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

// TestFrameFitsTheTerminal is tdp L4 across sizes (E6): every line of
// every screen, with a page on the web screen and with popups over it, is
// exactly as wide as the terminal — with icons one cell and two, and the
// powerline symbols one and two (E5).
func TestFrameFitsTheTerminal(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	defer terminu.SetIconWidth(1, 1)
	root := doc(hd(1, "Fitting"), para(text("A paragraph "+glyphLink+" with an icon in it, and some words to wrap around.")),
		link("a link", "https://example.com/x", 7), hd(2, "Second"), para(text("More.")), hd(2, "Third"), para(text("End.")))
	for _, widths := range [][2]int{{1, 1}, {2, 1}, {2, 2}} {
		terminu.SetIconWidth(widths[0], widths[1])
		for _, size := range [][2]int{{120, 40}, {80, 40}, {72, 24}, {60, 20}, {40, 12}} {
			m := New(nil, "")
			d := newDriver(t, m)
			d.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			tb := &tab{id: 1, url: "https://example.com/fitting", title: "Fitting", root: root, cursor: -1}
			m.tabs, m.shown = []*tab{tb}, 0
			tb.relayout(m.pageW())
			m.sync()
			fits := func(where string, sw, sh int) {
				lines := m.a.Frame()
				if len(lines) != sh {
					t.Errorf("%v %dx%d %s: %d lines", widths, sw, sh, where, len(lines))
				}
				for i, l := range lines {
					if w := terminu.Width(l); w != sw {
						t.Errorf("%v %dx%d %s: line %d is %d wide: %q", widths, sw, sh, where, i, w, terminu.StripStyles(l))
					}
				}
			}
			check := func(where string) { fits(where, size[0], size[1]) }
			check("web, [1]")
			d.key("2")
			check("web, [2]")
			d.press(" ")
			check("Space menu")
			d.press("?")
			check("key reference over it")
			d.press("esc")
			d.press("esc")
			for _, k := range []string{"B", "H", "D", "S", "I"} {
				d.key(k)
				check("screen " + k)
			}
			d.key("W")
			d.press("L")
			check("Location")
			d.press("esc")

			long := strings.Repeat("The page has a long thing to say, and says it. ", 12)
			// The page's own dialog floats over the page panel (pagepopup.go).
			dlg := &ir.Node{Kind: ir.Landmark, Role: "dialog", ID: 50, Modal: true,
				Children: []*ir.Node{para(text(long)), link("Accept", "https://example.com/ok", 51)}}
			tb.root, tb.popups = doc(hd(1, "Fitting"), para(text("Behind it.")), dlg), []cdp.BackendNodeID{50}
			tb.relayout(m.pageW())
			m.sync()
			check("the page's dialog")
			tb.root, tb.popups = root, nil
			tb.relayout(m.pageW())
			m.sync()
			ask := &pageAsk{m: m, details: []string{long, "", long}, question: "Answer yes?", verb: "yes",
				accept: func() tea.Cmd { return nil }, cancel: func() tea.Cmd { return nil }}
			d.exec(m.a.Popup(terminu.Popup{Title: "The page asks", Glyph: glyphInfo, Body: ask}))
			d.settle()
			check("the page asks")
			d.exec(m.a.ClosePopup(ask))
			d.settle()
			d.exec(m.a.Confirm(terminu.Ask{Title: "Delete", Subject: "Fitting", Question: "Delete it?", Verb: "delete",
				Details: []string{long}}, func() tea.Cmd { return nil }))
			d.settle()
			check("a question")
			d.press("esc")
			var code []string
			for i := range 60 {
				code = append(code, itoa(i)+" "+long)
			}
			d.exec(m.a.Viewer(&terminu.Viewer{Title: "CODE", Subject: "Fitting", Glyph: glyphCode, Lines: code}))
			d.settle()
			check("a viewer")
			// The screen shrinks under an open popup: the popup shrinks with it.
			d.send(tea.WindowSizeMsg{Width: size[0] / 2, Height: size[1] / 2})
			d.settle()
			fits("a viewer, the screen shrunk under it", size[0]/2, size[1]/2)
			d.press("esc")
			m.Close()
		}
	}
}

// L is Chrome's Cmd+L: the box opens with the page's own URL on offer.
func TestLocationOffersThePageURL(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New(nil, "")
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.tabs = []*tab{{id: 1, url: "https://example.com/a"}}
	m.shown = 0
	m.sync()
	d.key("2")
	d.press("L")
	d.until("location open", d.shows("LOCATION"))
	if v := d.view(); !strings.Contains(v, "https://example.com/a") || !strings.Contains(v, "Tab:edit") {
		t.Fatalf("the page's URL should be offered, Tab taking it:\n%s", v)
	}
	d.press("esc")

	// T on the page is the same new tab as T on the tabs list.
	d.press("T")
	d.until("new tab box", d.shows("NEW TAB"))
	d.press("esc")
}

// importSample is the shape Chrome writes — the Netscape format every
// browser exports: a folder is an H3 and the DL after it.
const importSample = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 PERSONAL_TOOLBAR_FOLDER="true">Bookmarks bar</H3>
    <DL><p>
        <DT><A HREF="https://go.dev/">Go</A>
        <DT><H3>dev</H3>
        <DL><p>
            <DT><A HREF="https://pkg.go.dev/">pkg.go.dev</A>
        </DL><p>
    </DL><p>
    <DT><H3>Other bookmarks</H3>
    <DL><p>
        <DT><H3>empty</H3>
        <DL><p>
        </DL><p>
        <DT><A HREF="https://news.ycombinator.com/">Hacker News</A>
    </DL><p>
</DL><p>
`

// TestImportBookmarks: the file is read when it is picked, the folder name
// is required and must be new, and the whole tree lands under it.
func TestImportBookmarks(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	dir := t.TempDir()
	good, bad := filepath.Join(dir, "bookmarks_9_21_26.html"), filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(good, []byte(importSample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("not an export"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(nil, "").WithStore([]store.Bookmark{{Title: "Go", URL: "https://go.dev"}}, nil, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")

	// A file that is not an export is refused as soon as it is picked.
	d.exec(func() tea.Msg { return nil })
	d.exec(m.importPicked(bad))
	d.settle()
	if !strings.Contains(d.view(), "notes.txt") || strings.Contains(d.view(), "IMPORT BOOKMARKS: ") && strings.Contains(d.view(), "The folder to put") {
		t.Errorf("a file that is not an export should be refused at once:\n%s", d.view())
	}
	d.press("esc")

	d.exec(m.importPicked(good))
	d.settle()
	d.until("name asked", d.shows("3 bookmarks in 4 folders"))
	d.key("enter")
	if !strings.Contains(d.view(), "needs a name") {
		t.Fatalf("the box should say a name is needed, in its error row:\n%s", d.view())
	}
	d.key("chrome")
	d.press("enter")
	d.until("imported", func() bool { return len(d.m.bookmarks) == 4 })
	if r, ok := d.m.bmCurrent(); !ok || !r.isFolder || r.folder != "chrome" {
		t.Errorf("the cursor should be on the chrome folder, is on %+v", r)
	}
	wantFolders := []string{"chrome", "chrome/Bookmarks bar", "chrome/Bookmarks bar/dev", "chrome/Other bookmarks", "chrome/Other bookmarks/empty"}
	if got := strings.Join(d.m.folderNames(), "|"); got != strings.Join(wantFolders, "|") {
		t.Errorf("folders %q", d.m.folderNames())
	}
	if b := d.m.bookmarks[2]; b.Folder != "chrome/Bookmarks bar/dev" || b.URL != "https://pkg.go.dev/" {
		t.Errorf("the nested bookmark: %+v", b)
	}
	if list, folders, err := store.LoadBookmarks(); err != nil || len(list) != 4 || len(folders) != 5 {
		t.Errorf("saved: %d bookmarks, %d folders, %v", len(list), len(folders), err)
	}

	// The same name again is refused: an import never merges into another.
	d.exec(m.importPicked(good))
	d.settle()
	d.until("name asked again", d.shows("3 bookmarks in 4 folders"))
	d.key("chrome")
	d.key("enter")
	if !strings.Contains(d.view(), "is there already") || len(d.m.bookmarks) != 4 {
		t.Errorf("a taken name should keep the box open and import nothing:\n%s", d.view())
	}
}

// TestDeleteFolderTree: x on a folder with anything in it asks first,
// naming what goes, and then takes the whole tree; an empty folder goes at
// once; what is outside the folder stays.
func TestDeleteFolderTree(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New(nil, "").WithStore([]store.Bookmark{
		{Title: "Go", URL: "https://go.dev", Folder: "dev"},
		{Title: "pkg", URL: "https://pkg.go.dev", Folder: "dev/go"},
		{Title: "HN", URL: "https://news.ycombinator.com"},
	}, []string{"dev/empty", "misc"}, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")
	d.bmSelect(bmRow{folder: "dev", isFolder: true})
	d.press("x")
	d.until("asked", d.shows("DELETE FOLDER"))
	if v := d.view(); !strings.Contains(v, "2 bookmarks") || !strings.Contains(v, "2 folders") {
		t.Errorf("the confirm should name what goes:\n%s", v)
	}
	d.press("esc")
	if len(d.m.bookmarks) != 3 {
		t.Fatal("Esc should delete nothing")
	}
	d.press("x")
	d.until("asked again", d.shows("DELETE FOLDER"))
	d.press("enter")
	d.until("tree gone", func() bool { return len(d.m.bookmarks) == 1 })
	if d.m.bookmarks[0].Title != "HN" || strings.Join(d.m.folderNames(), "|") != "misc" {
		t.Errorf("after: %+v %q", d.m.bookmarks, d.m.folderNames())
	}
	// An empty folder asks too — a delete that asks, asks every time (tdp
	// F6) — without the line about what goes with it.
	d.bmSelect(bmRow{folder: "misc", isFolder: true})
	d.press("x")
	d.until("asked about misc", d.shows("DELETE FOLDER: misc"))
	if strings.Contains(d.view(), "everything in it") {
		t.Errorf("an empty folder has nothing going with it:\n%s", d.view())
	}
	d.press("enter")
	if len(d.m.folderNames()) != 0 {
		t.Error("the empty folder should be gone")
	}
	if list, folders, err := store.LoadBookmarks(); err != nil || len(list) != 1 || len(folders) != 0 {
		t.Errorf("saved: %d bookmarks, %d folders, %v", len(list), len(folders), err)
	}
}

// TestRenameBookmarks: r on a bookmark renames it; r on a folder renames
// that level, and everything under it follows; an empty or a taken name
// keeps the box.
func TestRenameBookmarks(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New(nil, "").WithStore([]store.Bookmark{
		{Title: "Go", URL: "https://go.dev", Folder: "dev"},
		{Title: "pkg", URL: "https://pkg.go.dev", Folder: "dev/go"},
	}, []string{"dev/empty", "misc"}, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")

	m.bmList.Select(bmRowID(bmRow{ref: 0}))
	d.press("r")
	d.until("title box", d.shows("RENAME BOOKMARK"))
	d.key("The Go site")
	d.press("enter")
	d.until("renamed", func() bool { return d.m.bookmarks[0].Title == "The Go site" })

	m.bmList.Select(bmRowID(bmRow{folder: "dev", isFolder: true}))
	d.press("r")
	d.until("name box", d.shows("RENAME FOLDER"))
	d.key("misc")
	d.key("enter")
	if !strings.Contains(d.view(), "is there already") {
		t.Fatalf("a taken name should keep the box and say so in it:\n%s", d.view())
	}
	for range 4 {
		d.key("backspace")
	}
	d.key("code")
	d.press("enter")
	d.until("folder renamed", func() bool { return strings.Join(d.m.folderNames(), "|") == "code|code/empty|code/go|misc" })
	if d.m.bookmarks[0].Folder != "code" || d.m.bookmarks[1].Folder != "code/go" {
		t.Errorf("bookmarks should follow the folder: %+v", d.m.bookmarks)
	}
	if r, ok := d.m.bmCurrent(); !ok || !r.isFolder || r.folder != "code" {
		t.Errorf("the cursor should be on the renamed folder, is on %+v", r)
	}
}

// TestPageParts: a page's four parts are panel [2]'s tabs after the whole
// page (user, 2026-09-22). None of it reads a role — the parts are placed
// by WHERE Chromium laid them out.
func TestPageParts(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("parts.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the parts page", d.loaded("Parts"))

	p := d.page()
	var kinds []string
	for _, x := range p.parts {
		kinds = append(kinds, x.kind.word())
	}
	if strings.Join(kinds, " ") != "header body others footer" {
		t.Fatalf("the fixture has all four, in page order: %q", kinds)
	}
	if p.at != partMain || d.m.pagePanel.Tab != 2 {
		t.Errorf("a page opens on its body: at %s, tab %d", p.at.word(), d.m.pagePanel.Tab)
	}
	if v := d.view(); !strings.Contains(v, "PAGE") || !strings.Contains(v, "FOOTER") {
		t.Errorf("[2]'s frame names the page and its parts:\n%s", v)
	}
	if n := p.current(); n == nil || n.Kind != ir.Heading {
		t.Errorf("and on the body's first item, is on %+v", n)
	}
	if v := dumpLayout(p.lay); !strings.Contains(v, "The body itself") || strings.Contains(v, "Top link") {
		t.Errorf("the body, and only the body:\n%s", v)
	}

	// l shows the next part, h the one before; the first tab is the whole
	// page.
	d.key("2")
	d.key("l")
	if p.at != partOthers {
		t.Errorf("l shows the next part: %s", p.at.word())
	}
	if v := dumpLayout(p.lay); strings.Contains(v, "The body itself") {
		t.Errorf("the panel should be showing others:\n%s", v)
	}
	d.key("h")
	d.key("h")
	d.key("h")
	if p.at != partWhole {
		t.Fatalf("the first tab is the whole page: %v", p.at)
	}
	if v := dumpLayout(p.lay); !strings.Contains(v, "The body itself") || !strings.Contains(v, "Top link") {
		t.Errorf("the whole page:\n%s", v)
	}
}

// A page with no one dominant block has no parts to choose between: it
// stays whole, and [2] has no tabs (2026-09-23).
func TestAFlatPageIsOnePart(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("nav.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	p := d.page()
	if len(p.parts) != 0 || len(d.m.pagePanel.Tabs) != 0 {
		t.Errorf("a flat page is not cut up: %d parts, tabs %q", len(p.parts), d.m.pagePanel.Tabs)
	}
	v := dumpLayout(p.lay)
	for _, want := range []string{"Page A", "Ann Example", "About"} {
		if !strings.Contains(v, want) {
			t.Errorf("the whole page is on screen, %q is not:\n%s", want, v)
		}
	}
}

func TestTableCells(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("nav.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	d.key("2")

	d.cursorOn(ir.Cell, "Ann Example")
	d.press("enter")
	d.until("the cell in full", d.shows("CELL"))
	if v := d.view(); !strings.Contains(v, "Name") || !strings.Contains(v, "to be cut by the column") {
		t.Errorf("the viewer should be the whole cell under its column's header:\n%s", v)
	}
	d.press("esc")

	// j is the next item in reading order: the next cell on the row.
	d.key("j")
	if n := d.page().current(); n == nil || n.Kind != ir.Cell || strings.TrimSpace(n.Text()) != "home" {
		t.Fatalf("j should walk to the next cell, is on %+v", n)
	}
	d.press("enter")
	d.until("asks to open", d.shows("OPEN LINK"))
	d.press("esc")

	d.cursorOn(ir.Cell, "none")
	d.press("enter")
	d.until("the other cell", d.shows("CELL — Site"))
}

// Google's search box is a textarea that calls itself a combobox, with no
// options: Enter on it is the input, not "no options to choose from" (user,
// 2026-09-23). A combobox WITH options still drops its list.
func TestAComboboxWithNoOptionsOpensTheInput(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("combo.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("combo page", d.loaded("Combo"))
	d.key("2")

	d.cursorOn(ir.Textbox, "搜尋")
	d.press("enter")
	d.until("the input", d.shows("搜尋"))
	if strings.Contains(d.view(), "No options") {
		t.Error("no message about options")
	}
	d.press("esc")

	d.cursorOn(ir.Combobox, "Pick")
	d.press("enter")
	d.until("the options", d.shows("CHOOSE"))
}

// Going back lands where the reader left, the way a browser restores its
// scroll (user, 2026-09-23).
func TestBackReturnsToThePlaceLeft(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("nav.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	d.key("2")
	p := d.page()

	d.cursorOn(ir.Link, "a link to B")
	leftCursor := p.cursor
	d.press("enter")
	d.until("open link?", d.shows("OPEN LINK"))
	d.press("enter")
	d.until("page B", d.loaded("Page B"))
	if p.entry == 0 || len(p.places) != 1 {
		t.Fatalf("leaving a page keeps its place under its entry: entry %d, places %d", p.entry, len(p.places))
	}
	d.key("P")
	d.until("page A again", d.loaded("Page A"))
	if p.cursor != leftCursor {
		t.Errorf("back lands on the item the reader left: cursor %d, want %d", p.cursor, leftCursor)
	}
	d.key("N")
	d.until("page B again", d.loaded("Page B"))
	if n := p.current(); n == nil || n.Kind != ir.Heading {
		t.Errorf("forward is back on B's start: %+v", n)
	}
}
