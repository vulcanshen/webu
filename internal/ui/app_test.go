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

// pagetabOn puts the hand on the capsule whose label contains want. One the
// width left out is behind the +N, and reaching it that way is a test
// of its own — so that is a fail here.
func (d *driver) pagetabOn(want string) {
	d.t.Helper()
	t := d.page()
	for i, c := range t.lay.pagetab {
		if strings.Contains(c.label, want) {
			if i >= t.lay.fit {
				d.t.Fatalf("capsule %q is behind the +N at this width:\n%s", want, dumpLayout(t.lay))
			}
			t.focusPagetab(i)
			return
		}
	}
	d.t.Fatalf("no capsule containing %q in\n%s", want, dumpLayout(t.lay))
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
	// measure takes a number of cells and refuses anything else, keeping
	// the box open with the reason.
	d.key("j")
	if e, _, ok := d.m.lists.current(); !ok || e.title != "measure" || !strings.HasPrefix(e.meta, "(default) 100") {
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

// TestNavigationEntry: a navigation is one row — an entry with a count,
// its links not items — and Enter on it lists them, Enter on one opens
// it; the Space menu lists the same rows as its item operations.
func TestNavigationEntry(t *testing.T) {
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
	// The page's chrome is on the pagetab under the URL, off the page, one
	// capsule per kind in the pagetab's order: the skip link and the skip
	// block are one "skip", the three navigations one "nav", then the
	// search, then the dialog by its name. The page starts on its
	// content.
	if n := d.page().current(); n == nil || n.Kind != ir.Heading || d.page().onPagetab() {
		t.Errorf("a new page should start on its content, starts on %+v", n)
	}
	v := dumpLayout(d.page().lay)
	for _, want := range []string{"skip +3", "nav +4", "search +2", "Cookies +1"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing the capsule %q in:\n%s", want, v)
		}
	}
	if l := d.page().lay; len(l.pagetab) != 4 || strings.Contains(v, "▎") {
		t.Errorf("four capsules and nothing of the chrome on the page:\n%s", v)
	}
	for _, it := range d.page().lay.items {
		if it.node.Kind == ir.Link && strings.Contains(it.node.Text(), "B via nav") {
			t.Error("a navigation's links should not be items of the page")
		}
	}
	// Esc goes up onto the pagetab, Esc again comes back; so do k from the
	// top of the page and j; h/l walk the capsules.
	d.key("esc")
	if n := d.page().current(); !d.page().onPagetab() || n == nil || !isSkipLink(n) {
		t.Errorf("Esc should put the hand on the first capsule, is on %+v", n)
	}
	d.key("esc")
	if n := d.page().current(); d.page().onPagetab() || n == nil || n.Kind != ir.Heading {
		t.Errorf("Esc on the pagetab should come back to the item the hand left, is on %+v", n)
	}
	d.key("k")
	if !d.page().onPagetab() {
		t.Error("k from the top of the page should go up onto the pagetab")
	}
	d.key("l")
	if n := d.page().current(); n == nil || n.Role != "navigation" {
		t.Errorf("l should walk to the next capsule, nav, is on %+v", n)
	}
	if !strings.Contains(d.m.View(), "3 navigations") || !strings.Contains(d.m.View(), "at Anchor") {
		t.Errorf("the panel's hint should say what the capsule is and where the user is in it:\n%s", d.m.View())
	}
	d.key("j")
	if n := d.page().current(); d.page().onPagetab() || n == nil || n.Kind != ir.Heading {
		t.Errorf("j should leave the pagetab for the item the hand left, is on %+v", n)
	}
	// The skip capsule's Enter lists the skip link and, under its own
	// header, the block's anchors. The link does what it says: the
	// content. An anchor lands the cursor on what it names, the form
	// here, without following anything.
	d.pagetabOn("skip")
	d.key("enter")
	d.until("the skip list", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optItemMenu })
	if got := d.m.options.items[d.m.options.cursor].label; got != "Skip to content" {
		t.Errorf("the first row should be the skip link, is %q", got)
	}
	d.key("enter")
	if n := d.page().current(); n == nil || n.Kind != ir.Heading || d.page().onPagetab() {
		t.Errorf("the skip link should land on the content, landed on %+v", n)
	}
	d.pagetabOn("skip")
	d.key("enter")
	d.until("the skip list again", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optItemMenu })
	d.key("j")
	if got := d.m.options.items[d.m.options.cursor].label; got != "Form" {
		t.Errorf("the row after the link should be the block's first anchor, is %q", got)
	}
	d.key("enter")
	if n := d.page().current(); n == nil || n.Kind != ir.Landmark || n.Role != "form" || d.m.confirm.isActive() || d.page().onPagetab() {
		t.Errorf("Form should land the cursor on the form, no confirm; landed on %+v", n)
	}
	// So does any link into the page: a table of contents entry.
	d.cursorOn(ir.Link, "to the form")
	d.key("enter")
	if n := d.page().current(); n == nil || n.Kind != ir.Landmark || n.Role != "form" || d.m.confirm.isActive() {
		t.Errorf("a link into the page should land the cursor, no confirm; landed on %+v", n)
	}

	// The nav capsule's list: each navigation under its name — the
	// class-marked trail among them — and the rows run across them.
	d.pagetabOn("nav")
	d.key("enter")
	d.until("its links", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optItemMenu })
	if got := d.m.options.items[d.m.options.cursor].label; got != "B via nav" {
		t.Errorf("the first row should be the first link, is %q", got)
	}
	headers, root := 0, false
	for _, it := range d.m.options.items {
		if it.header {
			headers++
		}
		if it.label == "Root" {
			root = true
		}
	}
	if headers != 3 || !root {
		t.Errorf("three navigations, each under a header, the trail's Root among the rows: %+v", d.m.options.items)
	}
	d.key("esc")
	d.until("list gone", func() bool { return !d.m.options.isActive() })

	d.key(" ")
	d.until("space menu", func() bool { return d.m.spaceMenu.isInteractive() })
	found, back := false, false
	for _, it := range d.m.spaceMenu.items {
		if it.label == "Anchor" && it.key == "entry:1" && strings.HasPrefix(it.hint, "here") {
			found = true
		}
		// The hand is on the pagetab here, so the menu offers the way back.
		if it.key == "pagetab" && it.label == "Back to the page" && strings.Contains(it.hint, "Esc") {
			back = true
		}
	}
	if !found {
		t.Error("the Space menu should list the navigations' links as its item operations, the current one marked")
	}
	if !back {
		t.Error("the Space menu should disclose the way off the pagetab")
	}
	d.key("esc")
	d.until("space menu gone", func() bool { return !d.m.spaceMenu.isActive() })

	// And from the page, the way onto it — the same row, the same key,
	// which does what Esc does.
	d.cursorOn(ir.Textbox, "Look")
	d.key(" ")
	d.until("space menu again", func() bool { return d.m.spaceMenu.isInteractive() })
	var row menuItem
	for _, it := range d.m.spaceMenu.items {
		if it.key == "pagetab" {
			row = it
		}
	}
	if row.label != "Page chrome" || !strings.Contains(row.hint, "Esc") || row.disabled {
		t.Errorf("the page's Space menu should offer the pagetab: %+v", row)
	}
	d.key("esc")
	d.until("space menu gone again", func() bool { return !d.m.spaceMenu.isActive() })
	if mm, _ := d.m.dispatch("pagetab"); !mm.(AppModel).shownTab().onPagetab() {
		t.Error("the menu row should put the hand on the pagetab")
	}
	d.page().leavePagetab()

	// A search with one box: Enter is the box itself; what is typed lands
	// in the page's field — behind the capsule, not an item — and is
	// offered to the page's Enter at once; Enter on that submits.
	d.pagetabOn("search")
	d.key("enter")
	d.until("the search box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputField })
	d.key("webu")
	d.key("enter")
	d.until("offered to search", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmSubmitField })
	if l := d.m.confirm.lines; len(l) == 0 || l[0] != "webu" {
		t.Errorf("the confirm should show what would be searched: %q", l)
	}
	d.key("enter")
	d.until("searched", func() bool { return strings.Contains(dumpLayout(d.page().lay), "searched:webu") })

	// A search box that is an item (type=search) is the same: the value
	// is written, the offer made; Esc keeps the value and sends nothing.
	d.cursorOn(ir.Textbox, "Look")
	d.key("enter")
	d.until("the look box", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputField && d.m.input.search })
	d.key("abc")
	d.key("enter")
	d.until("offered again", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmSubmitField })
	d.key("esc")
	d.until("kept, unsent", func() bool {
		n := d.page().current()
		return !d.m.confirm.isActive() && n != nil && n.Kind == ir.Textbox && n.Value == "abc"
	})
	if strings.Contains(dumpLayout(d.page().lay), "searched:abc") {
		t.Error("Esc on the offer should not submit")
	}

	// Narrower, the pagetab holds only some of the four capsules and ends
	// in a +N for the rest; the hand wraps onto it from the first, Enter
	// lists them, and the one chosen takes the last slot and opens its
	// list: the dialog — a cookie banner, chrome too — whose Accept
	// takes it off the page.
	d.send(tea.WindowSizeMsg{Width: 46, Height: 30})
	l := d.page().lay
	hidden := len(l.pagetab) - l.fit
	if len(l.pagetab) != 4 || l.fit == 0 || hidden == 0 {
		t.Fatalf("at 46 cells some of the four capsules should be behind a +N: %d of %d fit\n%s", l.fit, len(l.pagetab), dumpLayout(l))
	}
	more := " +" + itoa(hidden) + " "
	if !strings.Contains(d.m.View(), more) {
		t.Errorf("the pagetab should end in %q:\n%s", more, d.m.View())
	}
	d.pagetabOn("skip")
	d.key("h")
	if !d.page().onMore() {
		t.Errorf("h from the first capsule should wrap onto the +N: pagetab %d", d.page().pagetab)
	}
	d.key("enter")
	d.until("the capsules behind +N", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optPagetabMore })
	if len(d.m.options.items) != hidden || !strings.Contains(d.m.options.items[hidden-1].label, "Cookies +1") {
		t.Fatalf("the list should be the capsules the width left out, the dialog last: %+v", d.m.options.items)
	}
	for i := 1; i < hidden; i++ {
		d.key("j")
	}
	d.key("enter")
	d.until("its buttons", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optItemMenu })
	if n := d.page().current(); n == nil || n.Role != "dialog" || d.page().pagetabSlots()[l.fit-1] != 3 {
		t.Errorf("the chosen capsule should be under the hand, in the pagetab's last slot: %+v %v", n, d.page().pagetabSlots())
	}
	if got := d.m.options.items[d.m.options.cursor].label; got != "Accept" {
		t.Errorf("the dialog's row should be its button, is %q", got)
	}
	d.key("enter")
	d.until("dialog gone", func() bool { return !strings.Contains(dumpLayout(d.page().lay), "Cookies +1") })
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})

	d.pagetabOn("nav")
	d.key("enter")
	d.until("its links again", func() bool { return d.m.options.isInteractive() && d.m.optionsKind == optItemMenu })
	d.key("enter")
	d.until("page B", d.loaded("Page B"))
}

// TestTableCells: a data table's cells are stops; Enter on a text cell
// is its content in full under the column's header; Enter on a cell that
// is one link asks to open it, as the link would; l walks the row.
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
