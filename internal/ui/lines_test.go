package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// Every page has a line-number column, not only the section list (user,
// 2026-09-23) — and it counts from the top of what the panel is SHOWING,
// so inside a section line 1 is that section's first line.
func TestEveryPageHasLineNumbers(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Heading, Level: 1, Name: "One", ID: 1},
		para(text("first")),
		{Kind: ir.Heading, Level: 1, Name: "Two", ID: 2},
		para(text("second")),
		para(text("third")),
	}}
	tb := &tab{cursor: -1, root: root}
	tb.relayout(40)
	if tb.gutter != lineNumW(len(tb.lay.rows)) {
		t.Errorf("the gutter is as wide as the last number: %d for %d rows", tb.gutter, len(tb.lay.rows))
	}
	if tb.lineCount() != len(tb.lay.rows) {
		t.Errorf("the whole page is the whole count: %d of %d", tb.lineCount(), len(tb.lay.rows))
	}
	m := AppModel{focus: panelPage, tabs: []*tab{tb}, shown: 0}
	rows := m.pageRows(tb, 40, 6)
	for i, want := range []string{"1", "2", "3"} {
		if !strings.HasPrefix(strings.TrimSpace(rows[i]), want) {
			t.Errorf("row %d should lead with %q: %q", i, want, rows[i])
		}
	}
	// Every drawn row is the panel's full width, numbers included: the
	// gutter comes off the text, it is not added to the panel.
	for i, r := range rows {
		if w := dispW(r); w != 40 {
			t.Fatalf("row %d is %d wide, want 40: %q", i, w, r)
		}
	}

	// The go chord reaches them, which is what earns the column.
	if !tb.goToLine(4, 3) {
		t.Fatal("line 4 exists")
	}
	if tb.goToLine(99, 3) {
		t.Error("line 99 does not")
	}
}

// Inside a section the numbers start again at that section's first line:
// the panel is that section, and "line 5" has to mean the fifth line of
// what is being read.
func TestLineNumbersCountWhatIsShown(t *testing.T) {
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
	whole := p.lineCount()
	if whole != len(p.lay.rows) {
		t.Errorf("the page's own lines: %d of %d rows", whole, len(p.lay.rows))
	}
	// A list item that is nothing but its first line has no inside to
	// go to: Enter on it is Enter on the link it holds (2026-09-23).
	d.cursorOn(ir.ListItem, "B via nav")
	d.key("enter")
	if p.drilled() {
		t.Fatal("a one-line item does not drill")
	}
	d.until("open link?", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmOpenLink })
	d.key("esc")

	// Inside an item with more than its first line, the count is that
	// item's — past the first line, which is the header row now.
	li := &ir.Node{Kind: ir.ListItem, ID: 900, Children: []*ir.Node{
		para(text("Card one")), para(text("first of the body")), para(text("second of the body"))}}
	tb := &tab{cursor: -1, root: &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		hd(1, "Cards"), {Kind: ir.List, Children: []*ir.Node{li}}}}}
	tb.relayout(60)
	if !tb.drillInto(li, 60, 10) {
		t.Fatal("an item with a body drills")
	}
	if tb.drillHead != 0 || tb.drillBody <= 0 {
		t.Errorf("its first line is the header row: head %d body %d", tb.drillHead, tb.drillBody)
	}
	if n := tb.lineCount(); n <= 0 || n != len(tb.lay.rows)-tb.drillBody {
		t.Errorf("inside one item the count is its body's: %d of %d rows, body from %d", n, len(tb.lay.rows), tb.drillBody)
	}
	if !tb.goToLine(1, 10) || tb.top != tb.drillBody {
		t.Errorf("its first line is line 1, the row after the header: top %d", tb.top)
	}
	// And the first line is drawn once: on the header row, not below it.
	m := AppModel{focus: panelPage, tabs: []*tab{tb}, w: 100, h: 30}
	if h := m.pagetabRow(tb, 60); !strings.Contains(h, "Card one") {
		t.Errorf("the header row is the item's first line:\n%s", h)
	}
	for _, r := range m.pageRows(tb, 60, 8) {
		if strings.Contains(r, "Card one") {
			t.Errorf("and the page does not say it again:\n%s", r)
		}
	}
}

// fieldTakes says what a box wants in the page's own word, and falls
// back to the role when the page said nothing.
func TestAFieldSaysWhatItTakes(t *testing.T) {
	f := func(role, typ string, protected, multi bool) string {
		return fieldTakes(&ir.Node{Kind: ir.Textbox, Role: role, InputType: typ,
			Protected: protected, Multiline: multi, ID: cdp.BackendNodeID(1)})
	}
	for _, c := range []struct{ got, want string }{
		{f("textbox", "text", false, false), "text"},
		{f("textbox", "email", false, false), "email"},
		{f("textbox", "date", false, false), "date"},
		{f("textbox", "tel", false, false), "phone number"},
		{f("textbox", "datetime-local", false, false), "date and time"},
		{f("textbox", "password", true, false), "password"},
		{f("searchbox", "search", false, false), "search"},
		// Nothing declared: the role, then the shape, then text.
		{f("searchbox", "", false, false), "search"},
		{f("spinbutton", "", false, false), "number"},
		{f("textbox", "", false, true), "text, several lines"},
		{f("textbox", "", false, false), "text"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
