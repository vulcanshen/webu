package ui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/paths"
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
// no keys until it has (tdp F2).
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

// TestScreensAndSession drives the list screens without a browser.
func TestScreensAndSession(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New("").WithStore(
		[]store.Bookmark{{Title: "Hacker News", URL: "https://news.ycombinator.com/"}, {Title: "Go", URL: "https://go.dev"}},
		nil, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if v := d.view(); !strings.Contains(v, "[W]EB") || !strings.Contains(v, "[S]ETTINGS") || strings.Contains(v, "DEVTOOLS") {
		t.Errorf("the chip row names every screen, DevTools no longer one:\n%s", v)
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
	// A number carries its unit, written whole (tdp number.2).
	if !strings.Contains(d.view(), "default: 4 spaces") {
		t.Errorf("input_tab_width should read in spaces:\n%s", d.view())
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

// Under webu's least size (tdp L1) only its name and what it needs are
// drawn, each line the terminal's width, down to 1×1 (tdp E5); no key acts,
// and the screen it was on comes back as the terminal grows.
func TestUnderTheLeastSize(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New("")
	d := newDriver(t, m)
	need := "needs a terminal of at least " + itoa(minW) + "×" + itoa(minH)
	for _, s := range [][2]int{{minW - 1, minH}, {minW, minH - 1}, {80, 5}, {40, 12}, {20, 2}, {4, 1}, {1, 1}} {
		d.send(tea.WindowSizeMsg{Width: s[0], Height: s[1]})
		lines := m.a.Frame()
		if len(lines) != s[1] {
			t.Errorf("%dx%d: %d lines", s[0], s[1], len(lines))
		}
		for i, l := range lines {
			if w := terminu.Width(l); w != s[0] {
				t.Errorf("%dx%d: line %d is %d wide", s[0], s[1], i, w)
			}
		}
		v := d.view()
		if !strings.Contains(v, "webu"[:min(4, s[0])]) {
			t.Errorf("%dx%d: no name:\n%s", s[0], s[1], v)
		}
		if s[1] >= 2 && s[0] >= len(need) && !strings.Contains(v, need) {
			t.Errorf("%dx%d: no %q:\n%s", s[0], s[1], need, v)
		}
	}
	d.key("B")
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !d.m.onWeb() {
		t.Error("a key under the least size acted: B switched screens")
	}
	if v := d.view(); !strings.Contains(v, "[W]EB") {
		t.Errorf("the screen should come back as the terminal grows:\n%s", v)
	}
}

// TestEveryPopupFits is tdp E5's popups: each kind webu opens, once over
// the page, the whole frame measured; then, with it open, the screen made
// narrower than it, shorter, and both. A popup's box alone is the lib's to
// measure (tdp E8).
func TestEveryPopupFits(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	defer terminu.SetIconWidth(1, 1)
	long := strings.Repeat("The page has a long thing to say, and says it. ", 12)
	root := doc(hd(1, "Fitting"), para(text(long)), link("a link", "https://example.com/x"))
	var lines, options []string
	for i := range 60 {
		lines = append(lines, itoa(i)+" "+long)
		options = append(options, itoa(i)+" "+long[:70])
	}
	dirSetting := 0
	for i, s := range settings {
		if s.key == "download_dir" {
			dirSetting = i
		}
	}
	none := func() tea.Cmd { return nil }
	done := func(string) tea.Cmd { return nil }
	for _, widths := range [][2]int{{1, 1}, {2, 1}, {2, 2}} {
		terminu.SetIconWidth(widths[0], widths[1])
		for _, size := range [][2]int{{100, 30}, {minW, minH}} {
			m := New("")
			d := newDriver(t, m)
			d.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			tb := &tab{id: 1, url: "https://example.com/fitting", title: "Fitting", root: root, cursor: -1}
			m.tabs, m.shown = []*tab{tb}, 0
			tb.relayout(m.pageW())
			m.sync()
			d.key("2")
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
			// shrink measures the frame at the size and with the popup bigger
			// than the screen: wider, taller, both (tdp E5). Under webu's
			// least size only the name is drawn; it fits all the same.
			shrink := func(where string) {
				for _, s := range [][2]int{size, {size[0] / 2, size[1]}, {size[0], size[1] / 2}, {size[0] / 2, size[1] / 2}} {
					d.send(tea.WindowSizeMsg{Width: s[0], Height: s[1]})
					fits(where, s[0], s[1])
				}
				d.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			}
			for _, p := range []struct {
				name string
				open func() tea.Cmd
			}{
				{"text input", func() tea.Cmd {
					return m.a.Text(terminu.TextInput{Title: "Edit", Subject: "Fitting", Prompt: long, Verb: "save", Old: long, Done: done})
				}},
				{"password", func() tea.Cmd {
					return m.a.Password(terminu.PasswordInput{Title: "Sign in", Prompt: long, Verb: "set", Set: true,
						Done: func(string, bool) tea.Cmd { return nil }})
				}},
				{"number", func() tea.Cmd {
					return m.a.Number(terminu.NumberInput{Title: "Go to", Subject: "Fitting", Name: "The line", Prompt: long,
						Verb: "go", Old: "12", Min: 1, Max: 999, Done: done})
				}},
				{"select", func() tea.Cmd {
					return m.a.Select(terminu.Select{Title: "Choose", Subject: "Fitting", Options: options, Selected: 3,
						Done: func(int) tea.Cmd { return nil }})
				}},
				{"textarea", func() tea.Cmd {
					return m.a.Textarea(terminu.Textarea{Title: "Edit", Subject: "Fitting", Old: strings.Join(lines, "\n"), Done: done})
				}},
				{"form", func() tea.Cmd { return m.addBookmark("Fitting "+long, "https://example.com/"+long) }},
				{"file-picker", func() tea.Cmd { return m.settingBox(dirSetting) }},
				{"menu", func() tea.Cmd {
					return m.a.Menu("Cell", terminu.MenuSection{Title: "Fitting",
						Actions: []terminu.Action{m.item("Content", long, none), m.item(long, long, none)}})
				}},
				{"question", func() tea.Cmd {
					return m.a.Confirm(terminu.Ask{Title: "Delete", Subject: "Fitting", Question: "Delete it?", Verb: "delete",
						Details: []string{long}}, none)
				}},
				{"error", func() tea.Cmd { return m.a.Error("Save failed", long+long) }},
				{"viewer", func() tea.Cmd {
					return m.a.Viewer(&terminu.Viewer{Title: "CODE", Subject: "Fitting", Glyph: glyphCode, Lines: lines})
				}},
			} {
				d.exec(p.open())
				d.settle()
				shrink(p.name)
				d.key("esc")
			}
			m.Close()
		}
	}
}

// L is Chrome's Cmd+L: the box opens with the page's own URL on offer.
func TestLocationOffersThePageURL(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New("")
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.tabs = []*tab{{id: 1, url: "https://example.com/a"}}
	m.shown = 0
	m.sync()
	d.key("2")
	d.press("o")
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
	m := New("").WithStore([]store.Bookmark{{Title: "Go", URL: "https://go.dev"}}, nil, store.Config{}, nil)
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
	m := New("").WithStore([]store.Bookmark{
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

// After a delete the cursor rests on the next bookmark, under the same row
// ID: x's question names that one, not the one just gone.
func TestDeleteAsksAboutTheRowUnderTheCursor(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New("").WithStore([]store.Bookmark{
		{Title: "Alpha", URL: "https://a.example"},
		{Title: "Beta", URL: "https://b.example"},
	}, nil, store.Config{}, nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("B")
	d.bmSelect(bmRow{ref: 0})
	d.press("x")
	d.until("asked about Alpha", d.shows("DELETE: Alpha"))
	d.press("enter")
	d.until("Alpha gone", func() bool { return len(d.m.bookmarks) == 1 })
	d.press("x")
	d.until("asked about Beta", d.shows("DELETE: Beta"))
	if v := d.view(); strings.Contains(v, "a.example") {
		t.Errorf("the question still names the bookmark just deleted:\n%s", v)
	}
}

// webu's own files that will not read are asked about at the start, one
// after the other (tdp E3, when it cannot be read): Enter moves each to
// .bak and starts it new; session.yaml is not written while it is one of
// them.
func TestUnreadableFilesAreMovedAside(t *testing.T) {
	cfg, data := t.TempDir(), t.TempDir()
	t.Setenv("WEBU__CONFIG", cfg)
	t.Setenv("WEBU__DATA", data)
	t.Setenv("WEBU__STATE", "")
	write := func(p string) {
		if err := os.WriteFile(p, []byte("key: [\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(cfg, "config.yaml"))
	write(filepath.Join(cfg, "session.yaml"))
	_, cfgErr := store.LoadConfig()
	_, sessErr := store.LoadSession()
	if cfgErr == nil || sessErr == nil {
		t.Fatal("the files should not read")
	}
	m := New("").WithUnread([]Unread{
		{Name: "config.yaml", Path: store.Path("config.yaml"), Err: cfgErr},
		{Name: "session.yaml", Path: store.Path("session.yaml"), Err: sessErr},
	})
	if m.SessionWritable() {
		t.Error("an unread session.yaml should not be written")
	}
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the first question", d.shows("MOVE ASIDE"))
	if v := d.view(); !strings.Contains(v, "config.yaml can't be read") || !strings.Contains(v, "config.yaml.bak") {
		t.Errorf("the question names the file and where it goes:\n%s", v)
	}
	d.press("enter")
	d.until("the second question", d.shows("session.yaml can't be read"))
	d.press("enter")
	d.until("both moved", func() bool { return !strings.Contains(d.view(), "MOVE ASIDE") })
	for _, p := range []string{filepath.Join(cfg, "config.yaml.bak"), filepath.Join(cfg, "session.yaml.bak")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if !m.SessionWritable() {
		t.Error("a session moved aside starts new, to be written")
	}
}

// What main moved is an Info toast saying from where to where; what it
// could not move is an error popup naming both places (tdp E3).
func TestMovesOnTheFirstScreen(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	t.Setenv("HOME", "/h")
	m := New("").WithMoves([]paths.Move{
		{What: "bookmarks.yaml", From: "/h/.config/webu/bookmarks.yaml", To: "/h/.webu/datas/bookmarks.yaml"},
	})
	d := newDriver(t, m)
	d.settle()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the toast", d.shows("Moved bookmarks.yaml from ~/.config/webu to ~/.webu/datas"))

	m = New("").WithMoves([]paths.Move{
		{What: "session.yaml", From: "/h/.webu/datas/session.yaml", To: "/h/.config/webu/session.yaml", Err: errors.New("permission denied")},
	})
	d = newDriver(t, m)
	d.settle()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the popup", d.shows("MOVE FAILED"))
	v := d.view()
	for _, want := range []string{"~/.webu/datas/session.yaml", "~/.config/webu/session.yaml", "permission denied"} {
		if !strings.Contains(v, want) {
			t.Errorf("the popup should name %q:\n%s", want, v)
		}
	}
}

// TestRenameBookmarks: r on a bookmark renames it; r on a folder renames
// that level, and everything under it follows; an empty or a taken name
// keeps the box.
func TestRenameBookmarks(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	m := New("").WithStore([]store.Bookmark{
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
