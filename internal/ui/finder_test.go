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

// The index is every content node of the page, once. A hit is the
// smallest block that holds text: the paragraph carries the link in it,
// a list item that is only a link is one row, and a container with no
// text of its own is not a row at all (user, 2026-09-23).
func TestTheIndexListsEachThingOnce(t *testing.T) {
	li := func(id int, kids ...*ir.Node) *ir.Node {
		return &ir.Node{Kind: ir.ListItem, ID: cdp.BackendNodeID(id), Children: kids}
	}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		hd(1, "Title"),
		para(text("Some text and "), link("a link to B", "https://x.test/b", 2), text(" here.")),
		{Kind: ir.List, Children: []*ir.Node{
			li(10, link("B via nav", "https://x.test/n", 11)),
			li(20, para(text("Card one")), para(text("Card one's body"))),
		}},
		{Kind: ir.Landmark, Role: "navigation", Name: "Breadcrumb", ID: 30,
			Children: []*ir.Node{para(link("Home", "https://x.test/", 31))}},
	}}
	tb := &tab{cursor: -1, root: root}
	tb.relayout(60)
	hits := indexPage(tb)
	var texts []string
	for _, h := range hits {
		texts = append(texts, h.text)
	}
	want := []string{"Title", "Some text and a link to B here.", "B via nav",
		"Card one", "Card one's body", "Breadcrumb", "Home"}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Errorf("hits:\n  %q\nwant\n  %q", texts, want)
	}
	// The paragraphs inside the second item are reached THROUGH it: the
	// chain down to the hit says what to drill into.
	things := func(h hit) []*ir.Node {
		var out []*ir.Node
		chain := tb.chainOf(h)
		for _, a := range chain[:len(chain)-1] {
			if isThing(a) {
				out = append(out, a)
			}
		}
		return out
	}
	for _, h := range hits {
		switch h.text {
		case "Card one", "Card one's body":
			if p := things(h); len(p) != 1 || p[0].ID != 20 {
				t.Errorf("%q sits inside item 20: %v", h.text, p)
			}
		case "B via nav":
			if p := things(h); len(p) != 0 || h.node.Kind != ir.ListItem {
				t.Errorf("an item that is one line is its own hit, not inside itself: %+v", h)
			}
		case "Home":
			if h.under != "Title" {
				t.Errorf("a hit knows the heading over it: %q", h.under)
			}
		}
	}
}

// [/] over a real page: the hit is found in another part, and Enter goes
// there — the part switches, the cursor lands, nothing is pressed.
func TestSearchGoesToAHitInAnotherPart(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
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
	if p.at != partMain {
		t.Fatalf("opens on the body, is on %s", p.at.word())
	}

	d.key("2")
	d.press("/")
	d.until("the finder", d.shows("SEARCH"))
	d.key("bottom")
	d.settle()
	d.key("enter") // to the list
	d.key("enter") // go there
	d.settle()
	if p.at != partFooter {
		t.Errorf("Enter switches to the hit's part: on %s", p.at.word())
	}
	if n := p.current(); n == nil || n.Kind != ir.Link || n.Name != "Bottom link" {
		t.Errorf("and lands on it: %+v", n)
	}
	if !strings.Contains(p.url, "parts.html") || p.loading {
		t.Error("nothing was pressed: still on the page")
	}
}

// [go] over a page asks for a line by its number, and Enter is the line.
func TestGoToLineOnAPage(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("nav.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	d.key("2")
	p := d.page()

	d.key("g")
	d.press("o")
	d.until("the go box", d.shows("GO TO LINE"))
	d.key("16")
	d.key("enter")
	d.settle()
	// Line 16 is the row of buttons: on screen, the cursor on its first.
	if p.top > 15 || 15 >= p.top+d.m.pageVisible() {
		t.Errorf("line 16 should be on screen: top %d", p.top)
	}
	if n := p.current(); n == nil || n.Kind != ir.Button || n.Name != "Alert" {
		t.Errorf("the cursor lands on the first item of the line: %+v", n)
	}
}

// A landing survives the tree being rebuilt under the finder: the page
// settles while the finder is up, and a settle hands back the same page
// under new pointers. The hit is found again by Chromium's id.
func TestALandingSurvivesARebuild(t *testing.T) {
	build := func() *ir.Node {
		return &ir.Node{Kind: ir.Document, Children: []*ir.Node{
			hd(1, "Title"),
			para(text("first")),
			{Kind: ir.List, Children: []*ir.Node{
				{Kind: ir.ListItem, ID: 20, Children: []*ir.Node{
					para(text("Card one")), para(link("deep link", "https://x.test/d", 21))}},
			}},
		}}
	}
	tb := &tab{cursor: -1, root: build()}
	tb.relayout(60)
	hits := indexPage(tb)
	var deep hit
	for _, h := range hits {
		if h.text == "deep link" {
			deep = h
		}
	}
	if deep.node == nil || len(deep.trail) != 3 {
		t.Fatalf("the paragraph inside the item, three deep: %+v", deep)
	}
	// The same page, rebuilt.
	tb.root = build()
	tb.relayout(60)
	m := New(nil, "")
	m.a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.tabs, m.shown = []*tab{tb}, 0
	if !m.goToHit(tb, deep) {
		t.Fatal("the hit is the same node by id, and should land")
	}
	if !tb.drilled() || tb.drill[0].id != 20 {
		t.Errorf("landing drills into the item: %+v", tb.drill)
	}
	if n := tb.current(); n == nil || n.Name != "deep link" {
		t.Errorf("and the cursor is on the link: %+v", n)
	}
}
