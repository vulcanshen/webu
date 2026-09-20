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

	// Enter on a link is a click, and the click navigates.
	d.cursorOn(ir.Link, "a link to B")
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

	// Enter on an empty textbox opens the input popup; Enter there writes
	// the value into the page, and the next capture shows it.
	d.cursorOn(ir.Textbox, "Name")
	d.key("enter")
	d.until("input popup", func() bool { return d.m.input.isInteractive() })
	d.key("hi there")
	d.key("enter")
	d.until("value written", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Textbox && n.Value == "hi there"
	})

	// A filled textbox's Enter is the Submit/Edit/Clear/Yank menu; Submit
	// presses Enter in the field, and the form's handler sees the value.
	d.key("enter")
	d.until("options menu", func() bool { return d.m.options.isInteractive() })
	if got := d.m.options.items[d.m.options.cursor].label; got != "Submit" {
		t.Errorf("cursor should rest on Submit, is on %q", got)
	}
	d.key("enter")
	d.until("form submitted", func() bool {
		return strings.Contains(dumpLayout(d.page().lay), "submitted:hi there")
	})

	// A select lists its options; choosing one sets the value.
	d.cursorOn(ir.Combobox, "Pick")
	d.key("enter")
	d.until("options menu", func() bool { return d.m.options.isInteractive() })
	d.key("j")
	d.key("enter")
	d.until("option chosen", func() bool {
		n := d.page().current()
		return n != nil && n.Kind == ir.Combobox && n.Value == "Two"
	})
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
