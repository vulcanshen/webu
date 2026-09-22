package ui

import (
	"os"
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
	case "backspace":
		d.send(tea.KeyMsg{Type: tea.KeyBackspace})
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

// act is Enter on an item, the way a user does it (ux.md §A.0.K): the
// click itself, and for a link — which asks first — Enter again on the
// confirm.
func (d *driver) act() {
	d.t.Helper()
	n := d.page().current()
	d.key("enter")
	if n != nil && n.Kind == ir.Link {
		d.until("open link?", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmOpenLink })
		d.key("enter")
	}
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
			t.leavePagetab()
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

	// Enter on a link asks first — its text and URL — and Esc leaves the
	// page where it is.
	d.cursorOn(ir.Link, "a link to B")
	d.key("enter")
	d.until("open link?", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmOpenLink })
	if l := d.m.confirm.lines; len(l) != 2 || l[0] != "a link to B" || !strings.HasSuffix(l[1], "nav2.html") {
		t.Errorf("the confirm should show the link's text and URL, shows %q", l)
	}
	d.key("esc")
	d.until("confirm gone", func() bool { return !d.m.confirm.isActive() })
	if !strings.Contains(d.page().url, "nav.html") {
		t.Errorf("Esc should stay on page A, is on %s", d.page().url)
	}
	// Enter on the confirm follows it.
	d.key("enter")
	d.until("open link?", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmOpenLink })
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

	// Enter on a textbox is the box itself — the input popup, no menu in
	// between: a click on a field focuses it and nothing more. Enter there
	// writes the value into the page, and the next capture shows it.
	d.cursorOn(ir.Textbox, "Name")
	d.key("enter")
	d.until("input popup", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputField })
	d.key("hi there")
	d.key("enter")
	d.until("value written", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Textbox && n.Value == "hi there"
	})

	// Filled, Enter is still the box, with the value in it to change.
	d.key("enter")
	d.until("input popup again", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputField })
	if d.m.input.value != "hi there" {
		t.Errorf("the box should hold the value, holds %q", d.m.input.value)
	}
	// Esc closes the topmost float, and the toast from the failed back
	// above may still be it: let it go first.
	d.until("toast gone", func() bool { return !d.m.toast.anim.owns() })
	d.key("esc")
	d.until("box gone", func() bool { return !d.m.input.isActive() })

	// A password box is known as one while still empty — the DOM says so,
	// not the dots — and its box is masked from the first keystroke.
	d.cursorOn(ir.Textbox, "Secret")
	d.key("enter")
	d.until("password box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputField })
	if !d.m.input.masked || d.m.input.prompt != "password" {
		t.Errorf("an empty password field's box should be masked and say so: masked=%v prompt=%q", d.m.input.masked, d.m.input.prompt)
	}
	d.key("s3cret")
	d.key("enter")
	d.until("secret written", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Textbox && n.Protected && n.Value != ""
	})
	if v := d.page().current().Value; strings.Contains(v, "s3cret") {
		t.Errorf("the page should hand back dots, not the secret: %q", v)
	}

	// Submit is in the Space menu, first row: it presses Enter in the
	// field, and the form's handler sees the value.
	d.cursorOn(ir.Textbox, "Name")
	d.key(" ")
	d.until("space menu", func() bool { return d.m.spaceMenu.isInteractive() })
	if got := d.m.spaceMenu.items[d.m.spaceMenu.cursor].label; got != "Submit" {
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

	// A makes a folder — a path makes every level; m moves the bookmark
	// into one through a picker; the folder is a row of its own, Enter
	// folds it, and x on it only goes when it is empty.
	d.key("A")
	d.until("folder box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputFolder })
	d.key("dev/sub")
	d.key("enter")
	d.until("folders made", func() bool { return len(d.m.folders) == 2 && d.m.folders[1] == "dev/sub" })
	if e, _, ok := d.m.lists.current(); !ok || !e.isFolder || e.folder != "dev/sub" || e.depth != 1 {
		t.Fatalf("the cursor should land on the new folder, one level in: %+v", e)
	}
	d.key("esc") // the toast
	d.until("folder toast gone", func() bool { return !d.m.toast.anim.owns() })
	d.m.lists.cursorTo(0) // Hacker News, at the top level
	d.key("m")
	d.until("move picker", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optMoveTo })
	if got := d.m.options.items[2].label; got != "  sub" {
		t.Errorf("the picker should indent the subfolder: %q", got)
	}
	d.key("2")
	d.until("moved", func() bool { return d.m.bookmarks[0].Folder == "dev/sub" })
	if saved, folders, _ := store.LoadBookmarks(); len(saved) != 1 || saved[0].Folder != "dev/sub" || len(folders) != 2 {
		t.Errorf("bookmarks.yaml after move: %+v %v", saved, folders)
	}
	if e, _, ok := d.m.lists.current(); !ok || e.isFolder || e.depth != 2 {
		t.Errorf("the cursor should follow the moved bookmark, two levels in: %+v", e)
	}
	if v := d.m.View(); !strings.Contains(v, glyphFolder+" dev") || !strings.Contains(v, glyphFolder+" sub") {
		t.Errorf("both folders should be rows of their own:\n%s", v)
	}

	// Enter on the top folder folds everything under it into one row.
	d.m.lists.cursorToFolder("dev")
	d.key("enter")
	if got := len(d.m.lists.visible()); got != 1 {
		t.Fatalf("folded dev should leave one row, not %d", got)
	}
	if e, _, _ := d.m.lists.current(); !e.isFolder || !e.folded || e.count != 1 {
		t.Errorf("the folded row should count what it hides: %+v", e)
	}
	d.key("enter")
	if got := len(d.m.lists.visible()); got != 3 {
		t.Fatalf("unfolded dev should show three rows, not %d", got)
	}
	d.key("x")
	if len(d.m.folders) != 2 {
		t.Error("a folder with something in it must not go")
	}
	d.key("esc") // the toast that said so
	d.until("toast gone", func() bool { return !d.m.toast.anim.owns() })

	// A folder inside another can be deleted from the inside out: x on
	// the innermost row removes that path, and the parent it implied goes
	// with it once nothing else needs it.
	d.m.lists.cursorTo(0) // Hacker News, in dev/sub
	d.key("A")
	d.until("nested folder box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputFolder })
	d.key("x/y")
	d.key("enter")
	d.until("nested folders made", func() bool { return len(d.m.folders) == 4 && d.m.folders[3] == "dev/sub/x/y" })
	d.key("esc") // the toast
	d.until("nested toast gone", func() bool { return !d.m.toast.anim.owns() })
	d.m.lists.cursorToFolder("dev/sub/x")
	d.key("x")
	if len(d.m.folders) != 4 {
		t.Error("x must not go while y is inside it")
	}
	d.key("esc")
	d.until("refusal gone", func() bool { return !d.m.toast.anim.owns() })
	d.m.lists.cursorToFolder("dev/sub/x/y")
	d.key("x")
	d.until("y removed", func() bool { return len(d.m.folders) == 3 })
	if names := d.m.folderNames(); len(names) != 3 || names[2] != "dev/sub/x" {
		t.Errorf("x must stay when y goes, empty or not: %v", names)
	}
	if _, folders, _ := store.LoadBookmarks(); len(folders) != 3 {
		t.Errorf("every folder is written down, not only the hand-made ones: %v", folders)
	}
	d.key("esc")
	d.until("removal toast gone", func() bool { return !d.m.toast.anim.owns() })

	// a adds a bookmark where the cursor is, in two boxes; with no page
	// up, the title box offers the URL, and Enter takes the offer.
	d.m.lists.cursorToFolder("dev")
	d.key("a")
	d.until("url box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputBookmarkURL })
	d.key("go.dev")
	d.key("enter")
	d.until("title box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputBookmarkTitle })
	if d.m.input.placeholder != "https://go.dev" {
		t.Errorf("the title box should offer the URL: %q", d.m.input.placeholder)
	}
	d.key("enter")
	d.until("added", func() bool { return len(d.m.bookmarks) == 2 })
	if b := d.m.bookmarks[1]; b.URL != "https://go.dev" || b.Title != "https://go.dev" || b.Folder != "dev" {
		t.Errorf("the typed bookmark: %+v", b)
	}
	d.key("esc") // the toast
	d.until("added toast gone", func() bool { return !d.m.toast.anim.owns() })
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
	if e, _, ok := d.m.lists.current(); d.m.screen != screenSettings || !ok || e.title != "search_engine" || !strings.Contains(e.meta, "google.com") {
		t.Fatalf("first settings row: %+v %v (screen %v)", e, ok, d.m.screen)
	}
	d.key("j")
	if e, _, ok := d.m.lists.current(); !ok || e.title != "download_dir" || !strings.HasPrefix(e.meta, "(default) ") {
		t.Fatalf("settings row: %+v %v", e, ok)
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
	if e, _, _ := d.m.lists.current(); !strings.HasPrefix(e.meta, "/tmp/dl  ") {
		t.Errorf("settings row after save: %+v", e)
	}
	// measure takes full or a number of cells and refuses anything else,
	// keeping the box open with the reason. Full is the default.
	d.key("j")
	if e, _, ok := d.m.lists.current(); !ok || e.title != "measure" || !strings.HasPrefix(e.meta, "(default) full") {
		t.Fatalf("measure row: %+v %v", e, ok)
	}
	d.key("enter")
	d.until("measure box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputSetting })
	d.send(tea.KeyMsg{Type: tea.KeyBackspace})
	d.key("wide")
	d.key("enter")
	d.until("refused", func() bool { return d.m.toast.isActive() })
	if !d.m.input.isActive() || d.m.cfg.Measure != 0 {
		t.Fatalf("a bad measure should keep the box open and the setting as it was: %d", d.m.cfg.Measure)
	}
	d.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("80")
	d.key("enter")
	d.until("measure saved", func() bool { return d.m.cfg.Measure == 80 && !d.m.input.isActive() })
	if cfg, _ := store.LoadConfig(); cfg.Measure != 80 {
		t.Errorf("config.yaml after measure: %+v", cfg)
	}
	d.key("esc")
	d.until("measure toast gone", func() bool { return !d.m.toast.anim.owns() })

	// A switch flips on Enter and is written at once.
	d.key("j")
	if e, _, ok := d.m.lists.current(); !ok || e.title != "restore_session" || !e.toggle || !strings.HasPrefix(e.meta, "(default) on") {
		t.Fatalf("restore_session row: %+v %v", e, ok)
	}
	d.key("enter")
	if d.m.cfg.Restore() {
		t.Fatal("Enter should switch restore_session off")
	}
	if cfg, _ := store.LoadConfig(); cfg.Restore() {
		t.Errorf("config.yaml after the switch: %+v", cfg)
	}
	if e, _, _ := d.m.lists.current(); !strings.HasPrefix(e.meta, "off  ") {
		t.Errorf("the row should read off: %+v", e)
	}
	d.key("esc") // the toast
	d.until("switch toast gone", func() bool { return !d.m.toast.anim.owns() })
	d.key("enter")
	if !d.m.cfg.Restore() {
		t.Fatal("Enter again should switch it back on")
	}
	d.key("esc")
	d.until("switch toast gone again", func() bool { return !d.m.toast.anim.owns() })
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

// TestImportBookmarks drives I on the Bookmarks screen without a browser:
// the picker opens in Downloads, a file that is not an export is refused
// as soon as it is picked, the folder name is required and must be new,
// and the whole tree lands under it.
func TestImportBookmarks(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	dl := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"bookmarks_9_21_26.html": importSample, "notes.txt": "not an export"} {
		if err := os.WriteFile(filepath.Join(dl, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := New(nil, "").WithStore([]store.Bookmark{{Title: "Go", URL: "https://go.dev"}}, nil, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")
	d.key("I")
	d.until("picker", func() bool { return d.m.picker.isInteractive() })
	if d.m.picker.dir != dl {
		t.Errorf("the picker should open in Downloads, opened in %s", d.m.picker.dir)
	}

	// Typing narrows; a file that is not an export is refused as soon as
	// it is picked, before any name is asked.
	d.key("notes")
	d.key("enter")
	d.until("refused", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "notes.txt") })
	if d.m.input.isActive() {
		t.Error("no name should be asked for a file that is not an export")
	}
	d.until("picker gone", func() bool { return !d.m.picker.isActive() })

	d.key("I")
	d.until("picker again", func() bool { return d.m.picker.isInteractive() })
	d.key("book")
	d.key("enter")
	d.until("name asked", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputImportName })
	if p := d.m.input.prompt; !strings.Contains(p, "3 bookmarks") || !strings.Contains(p, "4 folders") {
		t.Errorf("the prompt should count what was read: %q", p)
	}
	// The name is required: Enter on nothing keeps the box.
	d.key("enter")
	d.until("told to name it", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "folder name") })
	if !d.m.input.isInteractive() {
		t.Fatal("the box should stay open without a name")
	}
	d.key("chrome")
	d.key("enter")
	d.until("imported", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "imported 3 bookmarks") })
	d.until("box gone", func() bool { return !d.m.input.isActive() })

	// The whole tree is under chrome, the empty folder too; what was
	// there before is untouched; the cursor is on the new root.
	if e, _, ok := d.m.lists.current(); !ok || !e.isFolder || e.folder != "chrome" {
		t.Errorf("the cursor should be on the chrome folder, is on %+v", e)
	}
	wantFolders := []string{"chrome", "chrome/Bookmarks bar", "chrome/Bookmarks bar/dev", "chrome/Other bookmarks", "chrome/Other bookmarks/empty"}
	if got := strings.Join(d.m.folderNames(), "|"); got != strings.Join(wantFolders, "|") {
		t.Errorf("folders %q", d.m.folderNames())
	}
	if len(d.m.bookmarks) != 4 || d.m.bookmarks[0] != (store.Bookmark{Title: "Go", URL: "https://go.dev"}) {
		t.Errorf("bookmarks %+v", d.m.bookmarks)
	}
	if b := d.m.bookmarks[2]; b.Folder != "chrome/Bookmarks bar/dev" || b.URL != "https://pkg.go.dev/" {
		t.Errorf("the nested bookmark: %+v", b)
	}
	if list, folders, err := store.LoadBookmarks(); err != nil || len(list) != 4 || len(folders) != 5 {
		t.Errorf("saved: %d bookmarks, %d folders, %v", len(list), len(folders), err)
	}

	// The same name again is refused: an import never merges into another.
	d.key("I")
	d.until("picker once more", func() bool { return d.m.picker.isInteractive() })
	d.key("book")
	d.key("enter")
	d.until("name asked again", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputImportName })
	d.key("chrome")
	d.key("enter")
	d.until("refused the name", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "exists") })
	if !d.m.input.isInteractive() || len(d.m.bookmarks) != 4 {
		t.Error("a taken name should keep the box open and import nothing")
	}
}

// TestDeleteFolderTree: x on a folder with anything in it asks first,
// naming what goes, and then takes the whole tree; an empty folder goes
// at once; what is outside the folder stays.
func TestDeleteFolderTree(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	m := New(nil, "").WithStore([]store.Bookmark{
		{Title: "Go", URL: "https://go.dev", Folder: "dev"},
		{Title: "pkg", URL: "https://pkg.go.dev", Folder: "dev/go"},
		{Title: "HN", URL: "https://news.ycombinator.com"},
	}, []string{"dev/empty", "misc"}, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")
	d.m.lists.cursorToFolder("dev")
	d.key("x")
	d.until("asked", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmDeleteFolder })
	if l := d.m.confirm.lines; len(l) != 2 || l[0] != "dev" || !strings.Contains(l[1], "2 bookmarks") || !strings.Contains(l[1], "2 folders") {
		t.Errorf("the confirm should name what goes: %q", l)
	}
	d.key("esc")
	d.until("kept", func() bool { return !d.m.confirm.isActive() })
	if len(d.m.bookmarks) != 3 {
		t.Fatal("Esc should delete nothing")
	}

	d.m.lists.cursorToFolder("dev")
	d.key("x")
	d.until("asked again", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmDeleteFolder })
	d.key("enter")
	d.until("tree gone", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "2 bookmarks") })
	if len(d.m.bookmarks) != 1 || d.m.bookmarks[0].Title != "HN" {
		t.Errorf("bookmarks after: %+v", d.m.bookmarks)
	}
	if got := strings.Join(d.m.folderNames(), "|"); got != "misc" {
		t.Errorf("folders after: %q", d.m.folderNames())
	}

	// An empty folder goes without a question. (The confirm above is
	// still animating shut: let it finish, so a new one would show.)
	d.until("confirm shut", func() bool { return !d.m.confirm.isActive() })
	d.m.lists.cursorToFolder("misc")
	d.key("x")
	d.until("misc gone", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "misc") })
	if d.m.confirm.isActive() || len(d.m.folderNames()) != 0 {
		t.Error("an empty folder should go at once")
	}
	if list, folders, err := store.LoadBookmarks(); err != nil || len(list) != 1 || len(folders) != 0 {
		t.Errorf("saved: %d bookmarks, %d folders, %v", len(list), len(folders), err)
	}
}

// TestRenameBookmarks: r on a bookmark edits its title in a box holding
// the current one; r on a folder renames that level, and everything
// under it follows; an empty or a taken name keeps the box.
func TestRenameBookmarks(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	m := New(nil, "").WithStore([]store.Bookmark{
		{Title: "Go", URL: "https://go.dev", Folder: "dev"},
		{Title: "pkg", URL: "https://pkg.go.dev", Folder: "dev/go"},
	}, []string{"dev/empty", "misc"}, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")

	d.m.lists.cursorTo(0)
	d.key("r")
	d.until("title box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputRename })
	if d.m.input.value != "Go" {
		t.Errorf("the box should hold the title, holds %q", d.m.input.value)
	}
	d.key("backspace")
	d.key("backspace")
	d.key("enter")
	d.until("told to name it", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "name") })
	if !d.m.input.isInteractive() {
		t.Fatal("an empty name should keep the box")
	}
	d.key("The Go site")
	d.key("enter")
	d.until("renamed", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "renamed") })
	if d.m.bookmarks[0].Title != "The Go site" || d.m.bookmarks[0].URL != "https://go.dev" {
		t.Errorf("bookmark after: %+v", d.m.bookmarks[0])
	}
	d.until("box gone", func() bool { return !d.m.input.isActive() })

	// A folder: its own name only, and a taken one is refused.
	d.m.lists.cursorToFolder("dev")
	d.key("r")
	d.until("name box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputRename })
	if d.m.input.value != "dev" {
		t.Errorf("the box should hold the folder's name, holds %q", d.m.input.value)
	}
	for range 3 {
		d.key("backspace")
	}
	d.key("misc")
	d.key("enter")
	d.until("refused", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "exists") })
	if !d.m.input.isInteractive() {
		t.Fatal("a taken name should keep the box")
	}
	for range 4 {
		d.key("backspace")
	}
	d.key("code")
	d.key("enter")
	d.until("folder renamed", func() bool { return d.m.toast.isActive() && strings.Contains(d.m.toast.msg, "now code") })
	if got := strings.Join(d.m.folderNames(), "|"); got != "code|code/empty|code/go|misc" {
		t.Errorf("folders after: %q", d.m.folderNames())
	}
	if d.m.bookmarks[0].Folder != "code" || d.m.bookmarks[1].Folder != "code/go" {
		t.Errorf("bookmarks should follow the folder: %+v", d.m.bookmarks)
	}
	if e, _, ok := d.m.lists.current(); !ok || !e.isFolder || e.folder != "code" {
		t.Errorf("the cursor should be on the renamed folder, is on %+v", e)
	}
	if _, folders, err := store.LoadBookmarks(); err != nil || len(folders) != 4 {
		t.Errorf("saved folders: %q, %v", folders, err)
	}
}

// TestPageParts: a page is four parts and the pagetab is those parts
// (user, 2026-09-22). None of it reads a role — the fixture's header,
// nav, main and footer are placed by WHERE Chromium laid them out, and
// the same four come out of a page that marks up none of them.
func TestPageParts(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
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
	abs, _ := filepath.Abs("testdata/parts.html")
	d := newDriver(t, New(b, "file://"+abs))
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
	if p.at != partMain {
		t.Errorf("a page opens on its body, opened on %s", p.at.word())
	}
	if n := p.current(); n == nil || n.Kind != ir.Heading {
		t.Errorf("and on the body's first item, is on %+v", n)
	}
	if v := dumpLayout(p.lay); !strings.Contains(v, "The body itself") || strings.Contains(v, "Top link") {
		t.Errorf("the body, and only the body:\n%s", v)
	}

	// Esc puts the hand on the part being shown — which is where the eye
	// already is — and h/l walk the others without changing what is on
	// screen.
	d.key("esc")
	if !p.onPagetab() || p.parts[p.pagetabIndex()].kind != partMain {
		t.Errorf("Esc goes to the part on screen: %d", p.pagetabIndex())
	}
	d.key("l")
	if p.at != partMain {
		t.Error("walking the pagetab does not change what is shown")
	}
	if p.parts[p.pagetabIndex()].kind == partMain {
		t.Error("l moves the hand")
	}

	// Enter shows the part the hand is on, and the panel becomes it.
	want := p.parts[p.pagetabIndex()].kind
	d.key("enter")
	if p.at != want || p.onPagetab() {
		t.Errorf("Enter shows the part and comes back down: at=%s onPagetab=%v", p.at.word(), p.onPagetab())
	}
	if v := dumpLayout(p.lay); strings.Contains(v, "The body itself") {
		t.Errorf("the panel should be showing %s:\n%s", want.word(), v)
	}

	// Esc again, back to the body, and the page is whole again.
	d.key("esc")
	for p.parts[p.pagetabIndex()].kind != partMain {
		d.key("l")
	}
	d.key("enter")
	if !strings.Contains(dumpLayout(p.lay), "The body itself") {
		t.Errorf("back on the body:\n%s", dumpLayout(p.lay))
	}
}

// A page with no one dominant block has no parts to choose between: the
// flat fixture is fifteen paragraphs of much the same size, and cutting
// it anywhere would invent a header. It stays whole, and the pagetab
// does not appear (2026-09-23).
func TestAFlatPageIsOnePart(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
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

	p := d.page()
	if len(p.parts) != 0 {
		t.Errorf("a flat page is not cut up: %d parts", len(p.parts))
	}
	v := dumpLayout(p.lay)
	for _, want := range []string{"Page A", "Ann Example", "About"} {
		if !strings.Contains(v, want) {
			t.Errorf("the whole page is on screen, %q is not:\n%s", want, v)
		}
	}
	d.key("esc")
	if p.onPagetab() {
		t.Error("with one part there is nowhere for Esc to go")
	}
}

func TestTableCells(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
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

	d.cursorOn(ir.Cell, "Ann Example")
	d.key("enter")
	d.until("the cell in full", func() bool { return d.m.message.isInteractive() })
	if d.m.message.title != "Name" || !strings.Contains(strings.Join(d.m.message.lines, " "), "to be cut by the column") {
		t.Errorf("the popup should be the whole cell under its column's header: %q %q", d.m.message.title, d.m.message.lines)
	}
	d.key("esc")
	d.until("popup gone", func() bool { return !d.m.message.isActive() })

	d.key("l")
	if n := d.page().current(); n == nil || n.Kind != ir.Cell || strings.TrimSpace(n.Text()) != "home" {
		t.Fatalf("l should walk to the next cell, is on %+v", n)
	}
	d.key("enter")
	d.until("asks to open", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmOpenLink })
	d.key("esc")
	d.until("confirm gone", func() bool { return !d.m.confirm.isActive() })

	d.cursorOn(ir.Cell, "none")
	d.key("enter")
	d.until("the other cell", func() bool { return d.m.message.isInteractive() && d.m.message.title == "Site" })
}
