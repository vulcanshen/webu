package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A paste keeps its line breaks and tabs, "\r\n" one "\n"; every other
// control character goes (terminu, 2026-10-06).
func TestTakeText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"plain é 中", "plain é 中"},
		{"a\r\nb", "a\nb"},
		{"a\nb", "a\nb"},
		{"a\rb", "a\rb"},
		{"a\tb", "a\tb"},
		{"a\r\r\nb", "a\r\nb"},
		{"a\x1b[31mb", "a[31mb"},
		{"\x00\x07\x7f\u0085\u009bx", "x"},
		{" x", " x"},
	} {
		if got := takeText([]rune(c.in)); got != c.want {
			t.Errorf("takeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A line break or a tab is drawn as a two-cell `\n` / `\t` in its own
// style, and a cut takes it whole or not at all, from whichever end.
func TestValueView(t *testing.T) {
	withColour(t)
	st := lipgloss.NewStyle().Foreground(textColor)
	esc := lipgloss.NewStyle().Foreground(warnColor)
	for _, c := range []struct {
		v     string
		w     int
		head  bool
		shown string
	}{
		{"a\tb", 10, true, `a\tb`},
		{"a\rb", 10, false, `a\nb`},
		{"abc\ndef", 5, true, "… def"},
		{"abc\ndef", 6, true, `…\ndef`},
		{"ab\ncd", 4, false, "ab …"},
		{"ab\ncd", 5, false, `ab\n…`},
		{"abc", 1, false, "…"},
		{"abc", 0, false, ""},
	} {
		got, gw := valueView(c.v, c.w, st, esc, c.head)
		if ansi.Strip(got) != c.shown || gw != dispW(got) {
			t.Errorf("valueView(%q, %d, head %v) = %q (%d cells), want %q", c.v, c.w, c.head, ansi.Strip(got), gw, c.shown)
		}
	}
	got, _ := valueView("x\ny", 10, st, esc, true)
	if !strings.Contains(got, esc.Render(`\n`)) || !strings.Contains(got, st.Render("x")) {
		t.Errorf("the `\\n` takes esc's colour, the rest st's: %q", got)
	}
}

// Every one-line value webu takes — the input box, the file picker's
// query, the finder's, a list screen's filter, DevTools' filter, visual
// mode's search — keeps a pasted line break and tab and drops the rest:
// the row it is drawn on stays one row, the box keeps its shape, the
// `\n` is red, and Backspace takes a "\r\n" pasted as one.
func TestAPasteStaysOnOneRowEverywhere(t *testing.T) {
	withColour(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	paste := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("12\r\n34\tz\x1b\u009bq"), Paste: true}
	red := lipgloss.NewStyle().Foreground(warnColor).Render(`\n`)
	for _, s := range []struct {
		name  string
		open  func(d *driver)
		value func(d *driver) string
	}{
		{"input", func(d *driver) {
			d.key("L")
			d.until("the input", func() bool { return d.m.input.anim.isInteractive() })
		}, func(d *driver) string { return d.m.input.value }},
		{"picker", func(d *driver) {
			d.exec(d.m.picker.open(glyphFolder, "Import", dir, 1))
			d.until("the picker", func() bool { return d.m.picker.anim.isInteractive() })
		}, func(d *driver) string { return d.m.picker.query }},
		{"finder", func(d *driver) {
			d.key("/")
			d.until("the finder", func() bool { return d.m.finder.anim.isInteractive() })
		}, func(d *driver) string { return d.m.finder.query }},
		{"list filter", func(d *driver) {
			d.key("H")
			d.key("/")
		}, func(d *driver) string { return d.m.lists.filter }},
		{"devtools filter", func(d *driver) {
			d.m.devtools.tab = devNetwork
			d.exec(d.m.devtools.anim.open())
			d.until("devtools", func() bool { return d.m.devtools.anim.isInteractive() })
			d.key("/")
		}, func(d *driver) string { return d.m.devtools.filter[devNetwork] }},
		{"visual search", func(d *driver) {
			d.key("v")
			d.key("/")
		}, func(d *driver) string { return d.m.sel.query }},
	} {
		d := d6Driver(t)
		s.open(d)
		d.send(paste)
		if got := s.value(d); got != "12\n34\tzq" {
			t.Errorf("%s: the value is %q, want %q", s.name, got, "12\n34\tzq")
		}
		v := d.m.View()
		exactRows(t, s.name, v, 100, 30)
		if !strings.Contains(ansi.Strip(v), `12\n34\tzq`) {
			t.Errorf("%s: the value should be drawn as 12\\n34\\tzq:\n%s", s.name, ansi.Strip(v))
		}
		if !strings.Contains(v, red) {
			t.Errorf("%s: the `\\n` should be red", s.name)
		}
		for range 6 {
			d.key("backspace")
		}
		if got := s.value(d); got != "12" {
			t.Errorf("%s: six Backspaces leave %q, want %q: the \\r\\n is one", s.name, got, "12")
		}
	}
}

// inGrey reports whether s is drawn inside one run of Overlay0 in v.
func inGrey(v, s string) bool {
	grey := lipgloss.NewStyle().Foreground(dimColor).Render("x")
	return regexp.MustCompile(regexp.QuoteMeta(grey[:strings.Index(grey, "x")]) + `[^\x1b]*` + regexp.QuoteMeta(s)).MatchString(v)
}

// A filter left standing is grey throughout, its `\n` too: the keys are
// not on it (oneline.go).
func TestAStandingFilterIsGreyThroughout(t *testing.T) {
	withColour(t)
	d := d6Driver(t)
	d.key("H")
	d.key("/")
	d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true})
	d.key("enter")
	v := d.m.View()
	if !inGrey(v, `/ a\nb`) || strings.Contains(v, lipgloss.NewStyle().Foreground(warnColor).Render(`\n`)) {
		t.Errorf("a standing filter's `\\n` should be grey, not red:\n%s", ansi.Strip(v))
	}
}

// What a box opens with goes through the paste's filter: a page's
// prompt() default, a field's text, a bookmark's title, a setting.
// The offer is grey, its `\n` too.
func TestABoxOpensWithAFilteredValue(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	d.exec(d.m.input.ask(inputPopup{title: "The page asks", prompt: "name?", value: "a\r\nb\x1bc", placeholder: "o\x07k",
		more: []groupField{{prompt: "two", placeholder: "x\ty\x07"}}}, 1))
	d.until("the input", func() bool { return d.m.input.anim.isInteractive() })
	if d.m.input.value != "a\nbc" || d.m.input.placeholder != "ok" || d.m.input.more[0].placeholder != "x\ty" {
		t.Errorf("filled in: %q, %q and %q, want %q, %q and %q", d.m.input.value, d.m.input.placeholder,
			d.m.input.more[0].placeholder, "a\nbc", "ok", "x\ty")
	}
	v := d.m.input.view()
	exactRows(t, "the box", v, dispW(strings.Split(v, "\n")[0]), 0)
	if !inGrey(v, `x\ty`) || strings.Contains(v, lipgloss.NewStyle().Foreground(warnColor).Render(`\t`)) {
		t.Errorf("the offer should be grey throughout, its `\\t` too:\n%s", ansi.Strip(v))
	}

	// Add bookmark's title offer follows the URL field key by key: the
	// page's title, filtered each time.
	d = d6Driver(t)
	d.page().title = "Ho\x1bme"
	d.exec(d.m.startAddBookmark(""))
	d.until("the input", func() bool { return d.m.input.anim.isInteractive() })
	d.key("x")
	d.key("backspace")
	if got := d.m.input.more[0].placeholder; got != "Home" {
		t.Errorf("the title offer after a key is %q, want %q", got, "Home")
	}
}

// The finder's query keeps its `\t` red while it is typed, and grey with
// the rest of its row once the list has the keys (tdp D3): a code line
// with a tab in it is a hit for a query with one.
func TestTheFinderQueryIsGreyOnItsList(t *testing.T) {
	withColour(t)
	f := newFinder()
	f.kind, f.layer, f.query = finderSearch, 1, "a\tb"
	f.hits = []hit{{part: partMain, text: "a\tb"}}
	red := lipgloss.NewStyle().Foreground(warnColor).Render(`\t`)
	f.mode = finderInput
	if rows := f.listColumn(40, 6); !strings.Contains(rows[0], red) {
		t.Errorf("typing: the `\\t` should be red: %q", rows[0])
	}
	f.mode = finderNav
	if rows := f.listColumn(40, 6); !inGrey(rows[0], `a\tb`) || strings.Contains(rows[0], red) {
		t.Errorf("on the list: the `\\t` should be grey with its row: %q", rows[0])
	}
}

// No box takes a value with a line break or a tab, whatever it is for:
// Enter keeps it up, on the first field with one, the error row saying
// which field and why; nothing is sent. In a group the field at fault
// has its name red (tdp K3).
func TestNoBoxTakesALineBreak(t *testing.T) {
	withColour(t)
	red := lipgloss.NewStyle().Foreground(warnColor).Render("x")
	red = red[:strings.Index(red, "x")]
	for _, c := range []struct {
		p    inputPopup
		at   int
		says string
	}{
		{inputPopup{action: inputGoto, value: "a\nb"}, 0, "a URL or a search"},
		{inputPopup{action: inputGotoNewTab, value: "a\tb"}, 0, "a URL or a search"},
		{inputPopup{action: inputField, value: "a\nb"}, 0, "the field"},
		{inputPopup{action: inputFill, value: "1\t2", shape: "YYYY-MM-DD"}, 0, "the field"},
		{inputPopup{action: inputPrompt, value: "a\rb"}, 0, "the answer"},
		{inputPopup{action: inputAuth, prompt: "name", value: "me", more: []groupField{{prompt: "password", value: "p\tq", masked: true}}}, 1, "the password"},
		{inputPopup{action: inputAuth, prompt: "name", value: "m\ne", more: []groupField{{prompt: "password", value: "p\tq", masked: true}}}, 0, "the name"},
		{inputPopup{action: inputEval, value: "1 +\n2"}, 0, "a console line"},
		{inputPopup{action: inputSetting, value: "a\nb"}, 0, "a setting"},
		{inputPopup{action: inputFolder, value: "a\tb"}, 0, "a folder name"},
		{inputPopup{action: inputBookmark, prompt: "URL", value: "https://example.com", more: []groupField{{prompt: "title", value: "a\nb"}}}, 1, "the title"},
		{inputPopup{action: inputImportName, value: "a\nb"}, 0, "a folder name"},
		{inputPopup{action: inputRename, prompt: "title", value: "a\nb"}, 0, "a title"},
	} {
		d := keysDriver(t)
		c.p.title, c.p.accept = "Box", "go"
		if c.p.prompt == "" {
			c.p.prompt = "value"
		}
		d.exec(d.m.input.ask(c.p, 1))
		d.until("the box", func() bool { return d.m.input.anim.isInteractive() })
		h := rowsOf(d.m.input.view())
		before := d.m.input.fields()
		d.key("enter")
		want := c.says + " can't have line breaks or tabs"
		v := d.m.input.view()
		if !d.m.input.anim.owns() || d.m.input.refused != want || d.m.input.at != c.at || rowsOf(v) != h ||
			!strings.Contains(ansi.Strip(v), want) {
			t.Errorf("action %d: open %v, refused %q on field %d, %d rows then %d; want %q on field %d",
				c.p.action, d.m.input.anim.owns(), d.m.input.refused, d.m.input.at, h, rowsOf(v), want, c.at)
		}
		for i, f := range d.m.input.fields() {
			if f.value != before[i].value {
				t.Errorf("action %d: field %d is now %q, was %q", c.p.action, i, f.value, before[i].value)
			}
		}
		if len(c.p.more) > 0 && !strings.Contains(v, red+" "+d.m.input.fields()[c.at].prompt) {
			t.Errorf("action %d: the field at fault should have its name red:\n%q", c.p.action, v)
		}
	}
}
