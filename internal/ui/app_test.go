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
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir())
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

	// P goes back; the URL row follows.
	d.key("P")
	d.until("page A again", d.loaded("Page A"))
	if !strings.Contains(d.page().url, "nav.html") {
		t.Errorf("url after back: %s", d.page().url)
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
func TestListPopupsAndSession(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	m := New(nil, "").WithStore(
		[]store.Bookmark{{Title: "Hacker News", URL: "https://news.ycombinator.com/"}, {Title: "Go", URL: "https://go.dev"}},
		store.Config{Shortcuts: []store.Shortcut{{Title: "Mail", URL: "https://mail.example"}}},
		nil)
	d := newDriver(t, m)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})

	d.key("B")
	d.until("bookmarks open", func() bool { return d.m.lists.isInteractive() })
	if got := len(d.m.lists.visible()); got != 2 {
		t.Fatalf("visible %d", got)
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
	if saved, _ := store.LoadBookmarks(); len(saved) != 1 || saved[0].Title != "Hacker News" {
		t.Errorf("bookmarks.yaml after delete: %+v", saved)
	}
	d.key("esc") // the filter
	d.key("esc") // the popup
	d.until("closed", func() bool { return !d.m.lists.isActive() })

	d.key("S")
	d.until("shortcuts open", func() bool { return d.m.lists.isInteractive() })
	if e, _, ok := d.m.lists.current(); !ok || e.title != "Mail" {
		t.Errorf("shortcut row: %+v %v", e, ok)
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
		for _, focus := range []panelID{panel1, panel2, panel3} {
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
	}
}
