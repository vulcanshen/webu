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
		{Kind: ir.Landmark, Role: "banner", ID: 1, Children: []*ir.Node{link("Home", "/", 2), link("About", "/a", 3)}},
		{Kind: ir.Landmark, Role: "main", ID: 4, Children: []*ir.Node{
			para(text("body text")),
			{Kind: ir.Landmark, Role: "navigation", Name: "Repository", ID: 6, Children: []*ir.Node{link("Code", "/c", 7)}},
		}},
		{Kind: ir.Landmark, Role: "contentinfo", ID: 5, Children: []*ir.Node{text("footer")}},
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
	for _, want := range []string{"▸ banner · 2 items", "▾ main", "body text", "▾ navigation Repository", "Code", "▸ contentinfo"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "Home") || strings.Contains(out, "footer") {
		t.Errorf("a folded landmark's content is drawn:\n%s", out)
	}
	// Items: three rules on the top level, the nav rule, and Code.
	if len(tb.lay.items) != 5 || tb.lay.items[0].node.Role != "banner" || !tb.lay.items[0].folded {
		t.Fatalf("items: %s", out)
	}
	tb.cursor = 0
	tb.toggleFold(60)
	out = rows()
	if !strings.Contains(out, "▾ banner") || !strings.Contains(out, "Home") {
		t.Errorf("Enter on a folded landmark opens it:\n%s", out)
	}
	if tb.cursor != 0 || tb.lay.items[0].node.Role != "banner" {
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
		{Kind: ir.Landmark, Role: "contentinfo", ID: 8, Children: []*ir.Node{link("F", "/f", 9)}},
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

func TestNavigationListFlowsOnOneLine(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "navigation", ID: 1, Children: []*ir.Node{{Kind: ir.List, Children: []*ir.Node{
			{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Platform", "/p", 2)}},
			{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Solutions", "/s", 3)}},
			{Kind: ir.ListItem, Marker: "• ", Children: []*ir.Node{link("Resources", "/r", 4)}},
		}}}},
	}}
	// GitHub styles its tab links display:block; the list is still a row.
	for _, li := range root.Children[0].Children[0].Children {
		li.Children[0].Block = true
	}
	l := render(root, 80)
	if len(l.rows) < 2 {
		t.Fatalf("rows:\n%s", dumpLayout(l))
	}
	if got := l.rows[1].plain(); !strings.Contains(got, "Platform · ") || !strings.Contains(got, "Solutions · ") || !strings.Contains(got, "Resources") {
		t.Errorf("nav list should be one line: %q", got)
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
