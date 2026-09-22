package ui

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

var updateRender = flag.Bool("update", false, "rewrite testdata/*.render from the current renderer")

// TestRenderFixtures draws every IR fixture at a fixed width and checks the
// plain text against a golden. The goldens are the page as a user would see
// it, minus colour — the fastest way to review a layout change is to read
// the diff.
func TestRenderFixtures(t *testing.T) {
	jsons, _ := filepath.Glob("../ir/testdata/*.json")
	if len(jsons) == 0 {
		t.Fatal("no IR fixtures")
	}
	sort.Strings(jsons)
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range jsons {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var c ir.Capture
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			got := dumpLayout(render(ir.Build(c), 60))
			golden := filepath.Join("testdata", name+".render")
			if *updateRender {
				os.WriteFile(golden, []byte(got), 0o644)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("no golden: run with -update")
			}
			if string(want) != got {
				t.Errorf("render differs\n--- want\n%s--- got\n%s", want, got)
			}
			for i, r := range render(ir.Build(c), 60).rows {
				if w := dispW(r.plain()); w > 60 {
					t.Errorf("row %d is %d cells wide: %q", i, w, r.plain())
				}
			}
		})
	}
}

func dumpLayout(l layout) string {
	var b strings.Builder
	if len(l.pagetab) > 0 {
		fmt.Fprintf(&b, "-- pagetab (%d of %d fit) --\n", l.fit, len(l.pagetab))
		for _, c := range l.pagetab {
			b.WriteString(c.label)
			b.WriteString("\n")
		}
	}
	for _, r := range l.rows {
		b.WriteString(r.plain())
		b.WriteString("\n")
	}
	b.WriteString("-- items --\n")
	for i, it := range l.items {
		fmt.Fprintf(&b, "%d %s %q rows %d-%d\n", i, it.node.Kind, oneLine(it.node.Text()), it.first, it.last)
	}
	return b.String()
}

func TestWrapKeepsItemSpans(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{{
		Kind: ir.Paragraph, Children: []*ir.Node{
			{Kind: ir.Text, Name: "before "},
			{Kind: ir.Link, Name: "a link whose text is long enough to wrap onto the next row", URL: "u"},
			{Kind: ir.Text, Name: " after"},
		},
	}}}
	l := render(root, 30)
	if len(l.items) != 1 {
		t.Fatalf("items %d", len(l.items))
	}
	it := l.items[0]
	if it.first != 0 || it.last < 1 {
		t.Errorf("link should span from row 0 onto a later row, got %d-%d", it.first, it.last)
	}
	if got := l.itemAt(it.last); got != 0 {
		t.Errorf("itemAt(last) = %d", got)
	}
	for i, r := range l.rows {
		if w := dispW(r.plain()); w > 30 {
			t.Errorf("row %d overflows: %d", i, w)
		}
	}
}

func link(name, url string, id int) *ir.Node {
	return &ir.Node{Kind: ir.Link, Name: name, URL: url, ID: cdpID(id)}
}

func para(children ...*ir.Node) *ir.Node { return &ir.Node{Kind: ir.Paragraph, Children: children} }
func text(s string) *ir.Node             { return &ir.Node{Kind: ir.Text, Name: s} }

func TestLandmarksFoldOnlyWhenTold(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "region", Name: "Header", ID: 1, Children: []*ir.Node{link("Home", "/", 2), link("About", "/a", 3)}},
		{Kind: ir.Landmark, Role: "main", ID: 4, Children: []*ir.Node{
			para(text("body text")),
			{Kind: ir.Landmark, Role: "navigation", Name: "Repository", ID: 6, Children: []*ir.Node{link("Code", "/c", 7)}},
		}},
		{Kind: ir.Landmark, Role: "region", Name: "Footer", ID: 5, Children: []*ir.Node{text("footer")}},
	}}
	// Nothing folds by default (revised 2026-09-21), main or no main.
	fresh := &tab{cursor: -1, root: root}
	fresh.relayout(60)
	if out := dumpLayout(fresh.lay); !strings.Contains(out, "Home") || !strings.Contains(out, "footer") || strings.Contains(out, "▸") {
		t.Errorf("a fresh page should be all open:\n%s", out)
	}

	// The user's word folds, and survives a re-layout.
	tb := &tab{cursor: -1, root: root, fold: map[cdp.BackendNodeID]bool{1: true, 5: true}}
	tb.relayout(60)
	// The rows alone: the item list names a landmark by its whole text,
	// folded or not, which is not what is on screen.
	rows := func() string {
		var b strings.Builder
		for _, r := range tb.lay.rows {
			b.WriteString(r.plain() + "\n")
		}
		return b.String()
	}
	out := rows()
	for _, want := range []string{"▸ region Header · 2 items", "body text", "▸ region Footer"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	// A folded landmark's content stays off; so does a navigation's —
	// it is a capsule on the pagetab, its links behind Enter.
	if strings.Contains(out, "Home") || strings.Contains(out, "footer") || strings.Contains(out, "Code") {
		t.Errorf("hidden content is drawn:\n%s", out)
	}
	if len(tb.lay.pagetab) != 1 || !strings.Contains(tb.lay.pagetab[0].label, "nav +1") || strings.Contains(out, "▾ main") {
		t.Errorf("the navigation inside main should be a capsule on the pagetab, and main has no rule:\n%s", dumpLayout(tb.lay))
	}
	// Items: the two regions' rules; main has none, the navigation is on
	// the pagetab.
	if len(tb.lay.items) != 2 || tb.lay.items[0].node.Role != "region" || !tb.lay.items[0].folded {
		t.Fatalf("items: %s", out)
	}
	tb.cursor = 0
	tb.toggleFold(60)
	out = rows()
	if !strings.Contains(out, "▾ region Header") || !strings.Contains(out, "Home") {
		t.Errorf("Enter on a folded landmark opens it:\n%s", out)
	}
	if tb.cursor != 0 || tb.lay.items[0].node.Role != "region" {
		t.Errorf("the cursor should stay on the landmark: %d", tb.cursor)
	}
	// The Outline reaching into a folded landmark opens the way.
	tb.toggleFold(60)
	if strings.Contains(rows(), "About") {
		t.Fatal("the banner should be shut again")
	}
	tb.reveal(root.Children[0].Children[1], 60)
	if !strings.Contains(rows(), "About") {
		t.Error("reveal should open the banner")
	}
}

// A heading collapses its section: what follows it up to the next heading
// of its level or higher, or the end of its landmark (2026-09-21).
func TestHeadingCollapsesItsSection(t *testing.T) {
	heading := func(level int, id cdp.BackendNodeID, s string) *ir.Node {
		return &ir.Node{Kind: ir.Heading, Level: level, ID: id, Children: []*ir.Node{text(s)}}
	}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "main", ID: 1, Children: []*ir.Node{
			heading(1, 2, "Top"),
			para(text("intro")),
			link("A", "/a", 3),
			heading(2, 4, "Sub"),
			link("B", "/b", 5),
			heading(1, 6, "Next"),
			link("C", "/c", 7),
		}},
		{Kind: ir.Landmark, Role: "region", Name: "Tail", ID: 8, Children: []*ir.Node{link("F", "/f", 9)}},
	}}
	layout := func(folded ...cdp.BackendNodeID) (*tab, string) {
		tb := &tab{cursor: -1, root: root, fold: map[cdp.BackendNodeID]bool{}}
		for _, id := range folded {
			tb.fold[id] = true
		}
		tb.relayout(60)
		var b strings.Builder
		for _, r := range tb.lay.rows {
			b.WriteString(r.plain() + "\n")
		}
		return tb, b.String()
	}
	check := func(what, out string, present, absent []string) {
		t.Helper()
		for _, w := range present {
			if !strings.Contains(out, w) {
				t.Errorf("%s: missing %q in\n%s", what, w, out)
			}
		}
		for _, w := range absent {
			if strings.Contains(out, w) {
				t.Errorf("%s: %q should be hidden in\n%s", what, w, out)
			}
		}
	}
	_, out := layout()
	check("all open", out, []string{"# Top", "intro", "A", "## Sub", "B", "# Next", "C", "F"}, nil)
	_, out = layout(2)
	check("Top collapsed", out, []string{"▸ # Top · 3 items", "# Next", "C", "F"}, []string{"intro", "Sub", "B", " A"})
	_, out = layout(4)
	check("Sub collapsed", out, []string{"intro", "A", "▸ ## Sub · 1 item", "# Next", "C"}, []string{"B"})
	_, out = layout(6)
	check("Next collapsed, its landmark ends the section", out, []string{"▸ # Next · 1 item", "F"}, []string{"C"})

	// Enter on the collapsed heading opens it and the cursor stays there.
	tb, _ := layout(2)
	for i, it := range tb.lay.items {
		if it.node.ID == 2 {
			tb.cursor = i
		}
	}
	if !tb.lay.items[tb.cursor].folded {
		t.Fatal("the heading's item should say it is folded")
	}
	tb.toggleFold(60)
	if out := dumpLayout(tb.lay); !strings.Contains(out, "intro") || tb.lay.items[tb.cursor].node.ID != 2 {
		t.Errorf("toggleFold should open the section and keep the cursor:\n%s", out)
	}

	// A jump to something the section hides opens the heading first.
	tb, _ = layout(2)
	tb.reveal(root.Children[0].Children[4], 60) // B
	if out := dumpLayout(tb.lay); !strings.Contains(out, "B") || tb.fold[2] {
		t.Errorf("reveal should expand the heading over B:\n%s", out)
	}
}

// TestNavigationIsOneRow: a navigation landmark is an entry — one row
// naming where the user is in it and how many links are behind it, none
// of them items — and navTargets is what Enter lists, the nesting of its
// lists as depth. Where the user is: aria-current first, else the link
// whose URL is the page's or its longest prefix; a breadcrumb's last
// crumb.
func TestChromeIsACapsule(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, URL: "https://x.test/docs/api", Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "navigation", Name: "Main", ID: 1, Children: []*ir.Node{{Kind: ir.List, Children: []*ir.Node{
			{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Platform", "https://x.test/platform", 2)}},
			{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Solutions", "https://x.test/docs", 3), {Kind: ir.List, Children: []*ir.Node{
				{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Enterprise", "https://x.test/docs/enterprise", 5)}},
			}}}},
			{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Resources", "https://x.test/resources", 4)}},
		}}}},
	}}
	l := render(root, 80)
	if len(l.rows) != 0 || len(l.pagetab) != 1 || !strings.Contains(l.pagetab[0].label, "nav +4") {
		t.Errorf("the capsule should be the kind's word with the count, and leave the page no row:\n%s", dumpLayout(l))
	}
	// Where the user is — the section the page is under — is the hint's.
	if h := capsuleHint(l.pagetab[0], root.URL); !strings.Contains(h, "navigation Main") || !strings.Contains(h, "at Solutions") || !strings.Contains(h, "4 inside") {
		t.Errorf("the hint should say what it is, where the user is in it, and how much is inside: %q", h)
	}
	// The page's own word wins over the URL.
	root.Children[0].Children[0].Children[2].Children[0].Current = true
	if l := render(root, 80); !strings.Contains(capsuleHint(l.pagetab[0], root.URL), "at Resources") {
		t.Errorf("aria-current should name where the user is: %q", capsuleHint(l.pagetab[0], root.URL))
	}
	// A breadcrumb's row is its last crumb, link or not.
	trail := &ir.Node{Kind: ir.Document, URL: "https://x.test/lib/a", Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "navigation", Name: "Breadcrumb", Breadcrumb: true, ID: 9, Children: []*ir.Node{{Kind: ir.List, Children: []*ir.Node{
			{Kind: ir.ListItem, Children: []*ir.Node{link("Home", "https://x.test/", 10)}},
			{Kind: ir.ListItem, Children: []*ir.Node{link("Library", "https://x.test/lib", 11)}},
			{Kind: ir.ListItem, Children: []*ir.Node{text("Article A")}},
		}}}},
	}}
	if l := render(trail, 80); len(l.pagetab) != 1 || !strings.Contains(capsuleHint(l.pagetab[0], trail.URL), "at Article A") {
		t.Errorf("a breadcrumb's hint should be its last crumb: %q", capsuleHint(l.pagetab[0], trail.URL))
	}
	// The rest of the chrome: one capsule per kind, the kind's word, a
	// field counting among what is behind it; the skip link is its own
	// capsule, first.
	chrome := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "banner", ID: 20, Children: []*ir.Node{
			link("Skip to content", "https://x.test/page#main", 21), link("Acme", "https://x.test/", 22), link("Sign in", "https://x.test/login", 23)}},
		{Kind: ir.Landmark, Role: "search", ID: 30, Children: []*ir.Node{
			{Kind: ir.Textbox, Role: "searchbox", Name: "Search Acme", ID: 31}, {Kind: ir.Button, Role: "button", Name: "Go", ID: 32}}},
		{Kind: ir.Landmark, Role: "contentinfo", Name: "Site footer", ID: 40, Children: []*ir.Node{text("© Acme")}},
	}}
	// The skip link inside the banner is chrome inside chrome: not a
	// capsule, and not among the header's rows either.
	cl := render(chrome, 80)
	for _, want := range []string{"header +2", "search +2", "footer\n"} {
		if !strings.Contains(dumpLayout(cl), want) {
			t.Errorf("missing the capsule %q in:\n%s", want, dumpLayout(cl))
		}
	}
	if len(cl.pagetab) != 3 || cl.pagetab[0].kind != pagetabHeader || cl.pagetab[2].kind != pagetabFooter || len(cl.items) != 0 {
		t.Errorf("three capsules in the pagetab's order, and no item on the page:\n%s", dumpLayout(cl))
	}
	if h := capsuleHint(cl.pagetab[2], ""); h != "contentinfo Site footer" {
		t.Errorf("a footer's hint is its role and name: %q", h)
	}
	if len(l.items) != 0 {
		t.Errorf("a navigation should leave the page no item:\n%s", dumpLayout(l))
	}
	ts := entryTargets(root.Children[0])
	if len(ts) != 4 || ts[0].node.Text() != "Platform" || ts[2].node.Text() != "Enterprise" || ts[2].depth != 1 || ts[3].depth != 0 {
		t.Errorf("targets: %+v", ts)
	}
}

func TestMeasureCapsTextNotTables(t *testing.T) {
	long := strings.Repeat("word ", 60)
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		para(text(long)),
		{Kind: ir.Table, Children: []*ir.Node{{Kind: ir.Row, Children: []*ir.Node{
			{Kind: ir.Cell, Header: true, Children: []*ir.Node{text(strings.Repeat("h", 30))}},
			{Kind: ir.Cell, Header: true, Children: []*ir.Node{text(strings.Repeat("i", 30))}},
		}}}},
	}}
	l := renderWith(root, renderOpts{width: 200, measure: 40})
	wideTable := false
	for i, r := range l.rows {
		w := dispW(r.plain())
		if strings.HasPrefix(r.plain(), "word") && w > 40 {
			t.Errorf("row %d of text is %d wide, measure is 40", i, w)
		}
		if strings.HasPrefix(r.plain(), "hhh") && w > 40 {
			wideTable = true
		}
	}
	if !wideTable {
		t.Error("the table should still take the width")
	}
}

func TestCodeBlockFoldsAtTheMeasure(t *testing.T) {
	long := `"body": "` + strings.Repeat("quia et suscipit ", 8) + `"`
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Code, Children: []*ir.Node{text("{\n  " + long + "\n}")}},
	}}
	l := renderWith(root, renderOpts{width: 200, measure: 40})
	if len(l.rows) < 5 {
		t.Fatalf("the long line should fold onto several rows:\n%s", dumpLayout(l))
	}
	joined := ""
	for i, r := range l.rows {
		if w := dispW(r.plain()); w > 40 {
			t.Errorf("row %d is %d wide, measure is 40", i, w)
		}
		if !r.code {
			t.Errorf("row %d lost its code ground", i)
		}
		if strings.Contains(r.plain(), "…") {
			t.Errorf("row %d was cut instead of folded: %q", i, r.plain())
		}
		joined += r.plain()
	}
	if !strings.Contains(joined, strings.Repeat("quia et suscipit ", 8)) {
		t.Error("text was lost in the fold")
	}
}

func TestRowNavigation(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		para(link("A", "/a", 1), text(" "), link("B", "/b", 2), text(" "), link("C", "/c", 3)),
		para(link("D", "/d", 4)),
	}}
	tb := &tab{cursor: 0, root: root}
	tb.relayout(60)
	if len(tb.lay.items) != 4 {
		t.Fatalf("items:\n%s", dumpLayout(tb.lay))
	}
	name := func() string { return tb.current().Name }
	tb.moveItem("l", 10)
	tb.moveItem("l", 10)
	if name() != "C" {
		t.Errorf("l l → %s", name())
	}
	tb.moveItem("l", 10)
	if name() != "C" {
		t.Errorf("l at the row's end stays: %s", name())
	}
	tb.moveItem("j", 10)
	if name() != "D" {
		t.Errorf("j → %s", name())
	}
	tb.moveItem("k", 10)
	if name() != "A" {
		t.Errorf("k lands nearest the column (D starts at 0): %s", name())
	}
	tb.moveItem("h", 10)
	tb.moveItem("k", 10)
	if name() != "A" {
		t.Errorf("h and k at the top stay: %s", name())
	}
}

func cdpID(n int) cdp.BackendNodeID { return cdp.BackendNodeID(n) }

func TestLongWordIsSplit(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{{
		Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Text, Name: strings.Repeat("x", 25)}},
	}}}
	l := render(root, 10)
	if len(l.rows) != 3 {
		t.Fatalf("rows %d: %q", len(l.rows), dumpLayout(l))
	}
	for _, r := range l.rows {
		if dispW(r.plain()) > 10 {
			t.Errorf("overflow: %q", r.plain())
		}
	}
}

// TestTableCellsAreItems: in a data table every cell is a stop — the
// header's too — and what a cell holds is behind it, not a stop of its
// own; the rows carry the table's ground, the header row the deeper
// one; l walks the row; the column's header names a cell's popup.
func TestTableCellsAreItems(t *testing.T) {
	cell := func(header bool, kids ...*ir.Node) *ir.Node {
		return &ir.Node{Kind: ir.Cell, Role: "cell", Header: header, Children: kids}
	}
	rowOf := func(cells ...*ir.Node) *ir.Node { return &ir.Node{Kind: ir.Row, Role: "row", Children: cells} }
	table := &ir.Node{Kind: ir.Table, Role: "table", Children: []*ir.Node{
		rowOf(cell(true, text("Name")), cell(true, text("Site"))),
		rowOf(cell(false, text("Ann Example, whose description runs on")), cell(false, link("home", "https://x.test/", 7))),
		rowOf(cell(false, text("Bob")), cell(false, text("none"))),
	}}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{table}}
	tb := &tab{cursor: 0, root: root}
	tb.relayout(30)
	l := tb.lay
	if len(l.items) != 6 {
		t.Fatalf("six cells, six items:\n%s", dumpLayout(l))
	}
	for _, it := range l.items {
		if it.node.Kind != ir.Cell {
			t.Errorf("a %s is a stop; only cells should be:\n%s", it.node.Kind, dumpLayout(l))
		}
	}
	if len(l.rows) < 3 || !l.rows[0].table || !l.rows[0].header || !l.rows[1].table || l.rows[1].header {
		t.Errorf("the rows should carry the table's grounds, the header row its own:\n%s", dumpLayout(l))
	}
	// The cell is cut to its column; l moves along the row.
	tb.cursor = 2
	if tb.moveItem("l", 20); tb.lay.items[tb.cursor].node.Text() != "home" {
		t.Errorf("l should walk to the next cell, is on %q", tb.lay.items[tb.cursor].node.Text())
	}
	if got := columnHeader(root, tb.lay.items[tb.cursor].node); got != "Site" {
		t.Errorf("the column's header names the cell: %q", got)
	}
}

// TestSkipLinkIsARow: a skip link is chrome — one row of the entry
// style, an item — and a new page never starts on it: main's first
// item, else the item after it.
// A skip link is dropped: not drawn, not an item, and on no capsule
// (user, 2026-09-22). It exists to jump a screen reader past the
// navigation to the content — and webu has already taken the navigation
// off the page and starts the cursor at main. It is furniture for a
// problem this browser does not have.
func TestSkipLinkIsDropped(t *testing.T) {
	skip := link("Skip to main content", "https://x.test/page#main", 1)
	root := &ir.Node{Kind: ir.Document, URL: "https://x.test/page", Children: []*ir.Node{
		skip,
		{Kind: ir.Landmark, Role: "main", ID: 2, Children: []*ir.Node{para(link("First", "https://x.test/first", 3))}},
	}}
	tb := &tab{cursor: -1, root: root}
	tb.relayout(60)
	l := tb.lay
	if len(l.pagetab) != 0 {
		t.Errorf("a skip link is on no capsule:\n%s", dumpLayout(l))
	}
	if len(l.items) != 1 || l.items[0].node.Text() != "First" {
		t.Errorf("the skip link should leave the page no item, and main has no rule:\n%s", dumpLayout(l))
	}
	if at := tb.firstItem(); at < 0 || l.items[at].node.Text() != "First" {
		t.Errorf("a new page starts inside main: %d", at)
	}
	// No main: the first item, the link being off the page.
	bare := &ir.Node{Kind: ir.Document, URL: "https://x.test/page", Children: []*ir.Node{
		link("Skip navigation", "https://x.test/page#content", 1), para(link("Next", "https://x.test/next", 3))}}
	tb = &tab{cursor: -1, root: bare}
	tb.relayout(60)
	if at := tb.firstItem(); at < 0 || tb.lay.items[at].node.Text() != "Next" {
		t.Errorf("without main, a new page starts on its first item: %d", at)
	}
}

// TestPagetabHand: k from the top of the page puts the hand on the pagetab, h/l
// walk it and wrap, j comes back to the item the hand left; a width that
// holds only some capsules ends the pagetab in a +N, and a capsule chosen
// from behind it takes the last slot while the hand is on it.
func TestPagetabHand(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, URL: "https://x.test/", Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "banner", ID: 1, Children: []*ir.Node{link("Acme", "https://x.test/", 2)}},
		{Kind: ir.Landmark, Role: "navigation", Name: "Main", ID: 3, Children: []*ir.Node{link("Docs", "https://x.test/docs", 4)}},
		{Kind: ir.Landmark, Role: "search", ID: 5, Children: []*ir.Node{{Kind: ir.Textbox, Role: "searchbox", Name: "Search", ID: 6}}},
		{Kind: ir.Landmark, Role: "main", ID: 7, Children: []*ir.Node{para(link("First", "https://x.test/1", 8)), para(link("Second", "https://x.test/2", 9))}},
		{Kind: ir.Landmark, Role: "contentinfo", Name: "Footer", ID: 10, Children: []*ir.Node{text("©")}},
	}}
	tb := &tab{cursor: -1, root: root}
	tb.relayout(80)
	if len(tb.lay.pagetab) != 4 || tb.lay.fit != 4 || tb.onPagetab() {
		t.Fatalf("four capsules, all fitting, and the hand in the page:\n%s", dumpLayout(tb.lay))
	}
	tb.cursor = 0 // First, the top row of the page (main has no rule)
	tb.moveItem("k", 20)
	if !tb.onPagetab() || tb.current() != root.Children[0] {
		t.Errorf("k from the top of the page should put the hand on the first capsule, is on %+v", tb.current())
	}
	tb.moveItem("k", 20)
	if tb.current() != root.Children[0] {
		t.Errorf("k on the pagetab should stay: %+v", tb.current())
	}
	tb.moveItem("h", 20)
	if tb.current() != root.Children[4] {
		t.Errorf("h at the first capsule should wrap to the last, is on %+v", tb.current())
	}
	tb.moveItem("l", 20)
	tb.moveItem("l", 20)
	if tb.current() != root.Children[1] {
		t.Errorf("l should walk on, wrapping at the end, is on %+v", tb.current())
	}
	tb.moveItem("j", 20)
	if tb.onPagetab() || tb.cursor != 0 {
		t.Errorf("j should leave the pagetab for the item the hand left: pagetab %d cursor %d", tb.pagetab, tb.cursor)
	}
	// 32 cells hold two capsules and a +2.
	tb.relayout(32)
	if tb.lay.fit != 2 {
		t.Fatalf("at 32 cells two capsules should fit, then +2: fit %d", tb.lay.fit)
	}
	tb.moveItem("k", 20)
	tb.moveItem("h", 20)
	if !tb.onMore() || tb.current() != nil {
		t.Errorf("h from the first capsule should wrap onto the +N, which stands for no node: pagetab %d", tb.pagetab)
	}
	tb.focusPagetab(3) // chosen from behind the +N
	if slots := tb.pagetabSlots(); len(slots) != 3 || slots[0] != 0 || slots[1] != 3 || slots[2] != pagetabMore {
		t.Errorf("a capsule chosen from behind +N should take the last slot: %v", slots)
	}
	tb.moveItem("l", 20)
	if !tb.onMore() {
		t.Errorf("l from the last slot should be the +N: pagetab %d", tb.pagetab)
	}
	tb.moveItem("l", 20)
	if tb.current() != root.Children[0] {
		t.Errorf("l from the +N should wrap to the first capsule, is on %+v", tb.current())
	}
	tb.moveItem("G", 20)
	if tb.onPagetab() || tb.cursor != len(tb.lay.items)-1 {
		t.Errorf("G from the pagetab should come down and go to the end: pagetab %d cursor %d", tb.pagetab, tb.cursor)
	}
}

// TestJumpToAnchor: a fragment lands the cursor on the element it names
// when that is an item, else on the first item inside it, else on the
// item it sits inside, by the DOM's parents; an id the page lacks, or a
// target with nothing to stop on, is no jump.
func TestJumpToAnchor(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		para(link("Top", "https://x.test/p", 1)),
		{Kind: ir.Landmark, Role: "region", Name: "Sec", ID: 10, Children: []*ir.Node{para(link("Inside", "https://x.test/q", 11))}},
		para(text("plain")),
	}}
	tb := &tab{cursor: 0, root: root,
		anchors: map[string]cdp.BackendNodeID{"sec": 10, "inside": 11, "deep": 12, "plain": 13},
		parents: map[cdp.BackendNodeID]cdp.BackendNodeID{11: 10, 12: 11, 13: 0}}
	tb.relayout(60)
	if !tb.jumpToAnchor("sec", 20) || tb.lay.items[tb.cursor].node.ID != 10 {
		t.Errorf("the region itself is an item: cursor %d", tb.cursor)
	}
	tb.cursor = 0
	if !tb.jumpToAnchor("deep", 20) || tb.lay.items[tb.cursor].node.ID != 11 {
		t.Errorf("an id inside a link lands on the link, by parents: cursor %d", tb.cursor)
	}
	tb.cursor = 0
	if tb.jumpToAnchor("plain", 20) || tb.jumpToAnchor("missing", 20) || tb.cursor != 0 {
		t.Errorf("nothing to stop on is no jump: cursor %d", tb.cursor)
	}
	// The window follows: the target is the first row on screen, the way
	// following an anchor scrolls a browser (2026-09-22). A long page,
	// so there is somewhere to scroll to.
	long := &ir.Node{Kind: ir.Document, Children: []*ir.Node{}}
	for i := 0; i < 40; i++ {
		long.Children = append(long.Children, para(text("filler line")))
	}
	long.Children = append(long.Children,
		&ir.Node{Kind: ir.Landmark, Role: "region", Name: "Far", ID: 99, Children: []*ir.Node{para(link("There", "u", 98))}})
	tb = &tab{cursor: 0, root: long, anchors: map[string]cdp.BackendNodeID{"far": 99},
		parents: map[cdp.BackendNodeID]cdp.BackendNodeID{}}
	tb.relayout(60)
	tb.pagetab = 1 // and the hand was on the pagetab when it chose the anchor
	if !tb.jumpToAnchor("far", 10) {
		t.Fatal("the anchor should be found")
	}
	if row := tb.lay.items[tb.cursor].first; tb.top != row {
		t.Errorf("the target should be the first row on screen: top %d, target row %d", tb.top, row)
	}
	if tb.onPagetab() {
		t.Error("landing in the page takes the hand off the pagetab")
	}
}

// TestSpinnerWhileFetching: panel [2]'s URL row turns while the page is
// on its way and shows the web glyph at rest, and the glyph wears the
// same blue as the URL beside it (2026-09-22).
func TestSpinnerWhileFetching(t *testing.T) {
	tb := &tab{id: 1, url: "https://x.test/", cursor: -1}
	m := AppModel{w: 100, h: 30, tabs: []*tab{tb}}
	row := func() string { return m.pageBody(80, 12)[0] }
	if !strings.Contains(row(), glyphWeb) {
		t.Errorf("at rest the row wears the web glyph: %q", row())
	}
	if m.fetching() {
		t.Error("nothing is being fetched yet")
	}
	tb.loading = true
	if !m.fetching() {
		t.Error("a loading tab is a fetch in flight: what keeps the spinner armed")
	}
	spun := false
	for _, f := range spinnerFrames {
		if strings.Contains(row(), f) {
			spun = true
		}
	}
	if !spun || strings.Contains(row(), glyphWeb) {
		t.Errorf("while fetching the glyph is replaced by a spinner frame: %q", row())
	}
	if len(spinnerFrames) < 4 {
		t.Error("a spinner needs frames to spin")
	}
}

// TestOtherGoesToThePagetab: with a main on the page, what lies outside it
// in no landmark — a promo strip above main — is the pagetab's "other"
// capsule, not rows above the content; a wrapper around main is walked
// through; a page with no main keeps everything.
func TestOtherGoesToThePagetab(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, URL: "https://x.test/", Children: []*ir.Node{
		{Kind: ir.Group, Block: true, Children: []*ir.Node{
			para(text("Learn from our partner "), link("Scrimba", "https://s.test/", 1)),
			{Kind: ir.Landmark, Role: "main", ID: 2, Children: []*ir.Node{para(link("First", "https://x.test/1", 3))}},
		}},
		para(text("© 2026")),
	}}
	tb := &tab{cursor: -1, root: root}
	tb.relayout(60)
	l := tb.lay
	if len(l.pagetab) != 1 || l.pagetab[0].kind != pagetabOther || !strings.Contains(l.pagetab[0].label, "other +1") {
		t.Fatalf("the promo and the copyright should be one other capsule:\n%s", dumpLayout(l))
	}
	if len(l.items) != 1 || l.items[0].node.Text() != "First" || len(l.rows) != 1 {
		t.Errorf("the page should be main alone:\n%s", dumpLayout(l))
	}
	if h := capsuleHint(l.pagetab[0], ""); !strings.Contains(h, "outside main") || !strings.Contains(h, "1 inside") {
		t.Errorf("hint: %q", h)
	}
	if ts := capsuleTargets(l.pagetab[0]); len(ts) != 1 || ts[0].node.Text() != "Scrimba" {
		t.Errorf("targets: %+v", ts)
	}
	if at := tb.firstItem(); at != 0 {
		t.Errorf("a new page starts on main's first item: %d", at)
	}
	// Outside main, a skip link is dropped and blank text is nothing: no
	// capsule for either, and no row.
	tidy := &ir.Node{Kind: ir.Document, URL: "https://x.test/page", Children: []*ir.Node{
		link("Jump to content", "https://x.test/page#main", 1), text(" \n "),
		{Kind: ir.Landmark, Role: "main", ID: 2, Children: []*ir.Node{para(link("First", "https://x.test/1", 3))}},
	}}
	if l := render(tidy, 60); len(l.pagetab) != 0 || strings.Contains(dumpLayout(l), "Jump to") {
		t.Errorf("a skip link is furniture for a problem webu does not have:\n%s", dumpLayout(l))
	}
	// No main: nothing is other.
	bare := &ir.Node{Kind: ir.Document, Children: []*ir.Node{para(text("promo")), para(link("First", "u", 3))}}
	if l := render(bare, 60); len(l.pagetab) != 0 || len(l.items) != 1 || !strings.Contains(dumpLayout(l), "promo") {
		t.Errorf("without main the page keeps everything:\n%s", dumpLayout(l))
	}
}

// TestInlineMarkup: the page's own markup is drawn with the terminal's
// text attributes, composed with whatever the run already is — bold
// inside a heading is both — and <del> / <ins> are drawn at all, having
// fallen through to "unsupported" until 2026-09-22. A link is the one
// place markup does not reach: a link draws its accessible name, which
// Chromium has already flattened.
func TestInlineMarkup(t *testing.T) {
	span := func(role string, kids ...*ir.Node) *ir.Node {
		return &ir.Node{Kind: ir.Span, Role: role, Children: kids}
	}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		para(text("plain "), span("strong", text("bold")), text(" "),
			span("emphasis", text("ital")), text(" "),
			span("deletion", text("gone")), text(" "),
			span("insertion", text("new")), text(" "),
			span("mark", text("hit"))),
		{Kind: ir.Heading, Level: 2, Children: []*ir.Node{
			text("a "), span("strong", text("loud")), text(" title")}},
	}}
	l := render(root, 60)
	want := map[string]textAttr{
		"bold": attrBold, "ital": attrItalic, "gone": attrStrike,
		"new": attrUnderline, "hit": attrReverse, "plain": 0,
	}
	got := map[string]textAttr{}
	var headAttr textAttr
	var headKind segKind
	for _, r := range l.rows {
		for _, s := range r.segs {
			word := strings.TrimSpace(s.text)
			if _, ok := want[word]; ok {
				got[word] = s.attr
			}
			if word == "loud" {
				headAttr, headKind = s.attr, s.kind
			}
		}
	}
	for word, attr := range want {
		if got[word] != attr {
			t.Errorf("%q should be drawn with attr %b, is %b", word, attr, got[word])
		}
	}
	if headAttr&attrBold == 0 || headKind != segHeading {
		t.Errorf("markup inside a heading keeps both: attr %b kind %v", headAttr, headKind)
	}
	// The attribute reaches the paint rather than stopping at the layout.
	if st := withAttr(lipgloss.NewStyle(), attrStrike); !st.GetStrikethrough() {
		t.Error("withAttr should put the attribute on the style")
	}
}

// TestHeadingGround: a heading's rows carry its level, which is the
// ground they are painted on — brightest at h1, the crust at h6
// (theme.headingBg, 2026-09-22). The ramp is the VTP's lerp between two
// anchors, so the six are distinct and in order.
func TestHeadingGround(t *testing.T) {
	head := func(level int, s string) *ir.Node {
		return &ir.Node{Kind: ir.Heading, Level: level, Children: []*ir.Node{text(s)}}
	}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		head(1, "one"), para(text("prose")), head(3, "three"), head(6, "six"),
	}}
	l := render(root, 40)
	got := map[string]int{}
	for _, r := range l.rows {
		if w := strings.TrimSpace(r.plain()); w != "" {
			got[w] = r.heading
		}
	}
	// The row carries the heading's DEPTH, not its level: h1 then h3 then
	// h6 nests three deep, because a hierarchy's shape is the shape.
	for word, depth := range map[string]int{"# one": 1, "### three": 2, "###### six": 3, "prose": 0} {
		if got[word] != depth {
			t.Errorf("%q should carry depth %d, carries %d", word, depth, got[word])
		}
	}
	// Seven distinct inks, cycling rather than running out.
	seen := map[string]bool{}
	for depth := 1; depth <= len(levelInk); depth++ {
		ink := string(levelColor(depth))
		if seen[ink] {
			t.Errorf("depth %d repeats an ink: %s", depth, ink)
		}
		seen[ink] = true
	}
	if levelColor(len(levelInk)+1) != levelColor(1) {
		t.Error("the eighth level should start the cycle over, not run off the end")
	}
	if levelColor(0) != levelColor(1) {
		t.Error("a depth below one should clamp")
	}
}

// A label the page laid out as a block was printed twice: once as the
// page's own text, once as the field's accessible name, which IS that
// label. dropLabel only looked at the run being gathered, and a block
// label has already been flushed to a row by the time the field arrives
// (GitHub's sign-in, 2026-09-22).
func TestABlockLabelIsNotPrintedTwice(t *testing.T) {
	field := func(name string, id int) *ir.Node {
		return &ir.Node{Kind: ir.Textbox, Name: name, ID: cdp.BackendNodeID(id), Focusable: true}
	}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "form", ID: 1, Children: []*ir.Node{
			para(text("Username or email address")), field("Username or email address", 2),
			para(text("Password")), field("Password", 3),
		}},
	}}
	l := render(root, 60)
	for _, name := range []string{"Username or email address", "Password"} {
		n := 0
		for _, r := range l.rows {
			if strings.Contains(r.plain(), name) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%q is drawn on %d rows, want 1:\n%s", name, n, dumpLayout(l))
		}
	}
	// A paragraph that merely mentions the name is not a label, and stays.
	kept := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		para(text("Type your Password below, carefully")), field("Password", 4),
	}}
	if k := render(kept, 60); !strings.Contains(dumpLayout(k), "carefully") {
		t.Errorf("only an exact label is dropped:\n%s", dumpLayout(k))
	}
	// Nor is a row anything else points at: an item's row survives.
	held := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		para(link("Password", "https://x.test/p", 9)), field("Password", 5),
	}}
	if h := render(held, 60); len(h.items) != 2 {
		t.Errorf("a row an item is on is never dropped:\n%s", dumpLayout(h))
	}
}

// The hand parked on the pagetab has to survive a recapture. It was
// matched only on the node behind the capsule, and "other" holds whatever
// lies outside main in no landmark — nodes whose ids Chromium reassigns
// when the page rebuilds that part of its DOM. So a hand on the last
// capsule of a page that keeps settling fell back into the page, which
// reads as Esc undoing itself (user, 2026-09-22).
func TestTheHandStaysOnThePagetab(t *testing.T) {
	build := func(id int) *ir.Node {
		return &ir.Node{Kind: ir.Document, Children: []*ir.Node{
			{Kind: ir.Landmark, Role: "navigation", ID: 2, Children: []*ir.Node{link("Home", "/", 3)}},
			// Outside main, in no landmark: the "other" capsule, whose
			// id moves when the page rebuilds it.
			para(text("a promo"), link("Buy", "/buy", id)),
			{Kind: ir.Landmark, Role: "main", ID: 9, Children: []*ir.Node{para(text("body"))}},
		}}
	}
	tb := &tab{cursor: -1, root: build(100)}
	tb.relayout(80)
	last := len(tb.lay.pagetab) - 1
	if last < 1 {
		t.Fatalf("want a nav and an other capsule, got %d", len(tb.lay.pagetab))
	}
	tb.focusPagetab(last)
	kind := tb.lay.pagetab[last].kind

	// The same page again, that part of its DOM rebuilt under new ids.
	tb.apply(pageMsg{url: "https://x.test/p", cap: ir.Capture{}}, 80)
	tb.root = build(777)
	tb.relayout(80)
	tb.focusCapsuleLike(0, kind, last)
	if !tb.onPagetab() {
		t.Fatal("the hand fell back into the page")
	}
	if got := tb.lay.pagetab[tb.pagetabIndex()].kind; got != kind {
		t.Errorf("the hand moved to kind %v, want %v", got, kind)
	}
	// And with nothing to go back to, it does leave.
	empty := &tab{cursor: -1, root: &ir.Node{Kind: ir.Document, Children: []*ir.Node{para(text("bare"))}}}
	empty.relayout(80)
	empty.focusCapsuleLike(0, kind, 0)
	if empty.onPagetab() {
		t.Error("a page with no chrome has no capsule to hold the hand")
	}
}
