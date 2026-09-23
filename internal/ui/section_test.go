package ui

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

func hd(level int, s string) *ir.Node {
	return &ir.Node{Kind: ir.Heading, Level: level, Children: []*ir.Node{text(s)}}
}

// doc is a page with a main holding kids, so the section cut has the same
// scope a real document gives it.
func doc(kids ...*ir.Node) *ir.Node {
	return &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "main", Children: kids},
	}}
}

func titles(secs []section) []string {
	out := make([]string, len(secs))
	for i, s := range secs {
		out[i] = s.title
	}
	return out
}

// A heading's section runs to the next heading of its level or higher —
// the range folding already uses — and every drawn heading is one row of
// the list.
func TestSectionsCutOnHeadings(t *testing.T) {
	root := doc(
		hd(1, "Title"), para(text("lede")),
		hd(2, "First"), para(text("one")),
		hd(3, "Inside"), para(text("two")),
		hd(2, "Second"), para(text("three")),
	)
	l := render(root, 40)
	secs := sectionsOf(root, l, "")
	want := []string{"Title", "First", "Inside", "Second"}
	if got := titles(secs); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("sections %v, want %v", got, want)
	}
	if shapeOf(secs) != shapeDoc {
		t.Errorf("four headings is a document")
	}
	// The ranges tile the page: no row belongs to two sections, none to
	// none.
	for i := 1; i < len(secs); i++ {
		if secs[i].first != secs[i-1].last+1 {
			t.Errorf("%q ends at %d, %q starts at %d — a gap or an overlap",
				secs[i-1].title, secs[i-1].last, secs[i].title, secs[i].first)
		}
	}
	if last := secs[len(secs)-1].last; last != len(l.rows)-1 {
		t.Errorf("the last section ends at %d, the page at %d", last, len(l.rows)-1)
	}
}

// A heading with a link inside it is not an item — the link is — and it is
// still a heading. Cutting on the items instead of the marks lost every
// such heading, which on pkg.go.dev is most of them.
func TestSectionsCutOnLinkedHeadings(t *testing.T) {
	linked := &ir.Node{Kind: ir.Heading, Level: 2, Children: []*ir.Node{
		text("func Split"), link("¶", "#Split", 7),
	}}
	root := doc(hd(1, "strings"), para(text("lede")), linked, para(text("body")),
		hd(2, "func Join"), para(text("body")))
	secs := sectionsOf(root, render(root, 40), "")
	if len(secs) != 3 {
		t.Fatalf("a linked heading is still a heading: got %v", titles(secs))
	}
}

// Too few headings to be an outline: the page stays the one sheet it was.
func TestShapeNeedsAnOutline(t *testing.T) {
	root := doc(hd(1, "Title"), para(text("prose")), hd(2, "One"), para(text("more")))
	if got := shapeOf(sectionsOf(root, render(root, 40), "")); got != shapeOne {
		t.Errorf("two headings is a page with a subtitle, not a document (got shape %d)", got)
	}
}

// A heading over nothing but links, with no section beneath it, is a
// navigation column — w3schools' sidebar and footer, which no landmark
// marks as chrome. It leaves the list; its rows stay on the page.
func TestNavigationColumnsLeaveTheList(t *testing.T) {
	col := func(name string, n int) []*ir.Node {
		out := []*ir.Node{hd(2, name)}
		for i := 0; i < n; i++ {
			out = append(out, link("go "+itoa(i), "/x", 100+i))
		}
		return out
	}
	kids := []*ir.Node{hd(1, "Real Title"), para(text("prose here"))}
	kids = append(kids, col("TUTORIALS", 4)...)
	kids = append(kids, col("REFERENCES", 4)...)
	kids = append(kids, hd(2, "Also Real"), para(text("more prose")))
	root := doc(kids...)
	l := render(root, 40)
	secs := sectionsOf(root, l, "")
	want := []string{"Real Title", "Also Real"}
	if got := titles(secs); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("sections %v, want %v", got, want)
	}
	// Nothing was hidden: the dropped rows went to the section above.
	if secs[0].last < secs[1].first-1 {
		t.Errorf("the navigation rows fell out of the page as well as the list")
	}
	if !strings.Contains(strings.Join(rowText(l, secs[0]), "\n"), "TUTORIALS") {
		t.Errorf("a pruned heading should still be drawn inside the section above it")
	}
}

// A heading whose own body is empty but which has subheadings is an
// outline level, not a navigation column: MDN's "Accessibility" over its
// h3s has to stay.
func TestAHeadingOverSubheadingsStays(t *testing.T) {
	root := doc(
		hd(1, "Title"), para(text("lede")),
		hd(2, "Accessibility"), link("spec", "/spec", 3),
		hd(3, "Captions"), para(text("prose")),
		hd(2, "Examples"), para(text("prose")),
	)
	secs := sectionsOf(root, render(root, 40), "")
	if got := titles(secs); !strings.Contains(strings.Join(got, "|"), "Accessibility") {
		t.Errorf("a heading with sections under it is not a link column: got %v", got)
	}
}

// Reading one section shows that section and no more, and n walks to the
// next one at the same depth without going back through the list.
func TestReadingOneSection(t *testing.T) {
	root := doc(
		hd(1, "Title"), para(text("lede")),
		hd(2, "First"), para(text("one")),
		hd(3, "Inside"), para(text("two")),
		hd(2, "Second"), para(text("three")),
	)
	tb := &tab{root: root, cursor: -1}
	tb.relayout(40)
	if !tb.listing() {
		t.Fatalf("a document opens on its list")
	}
	tb.sec = 1 // "First"
	tb.openSection(10)
	// The window is the section's BODY: its heading is the panel's own
	// header row while it is open, not the first line under itself.
	lo, hi := tb.rowRange()
	if lo != tb.secs[1].body || hi != tb.secs[1].last {
		t.Errorf("reading shows rows %d-%d, the section's body is %d-%d", lo, hi, tb.secs[1].body, tb.secs[1].last)
	}
	if tb.secs[1].body <= tb.secs[1].first {
		t.Errorf("the body should start past the heading row")
	}
	if strings.Contains(strings.Join(rowText(tb.lay, tb.secs[1])[tb.secs[1].body-tb.secs[1].first:], "\n"), "# First") {
		t.Errorf("the heading should not be drawn inside its own section")
	}
	// n from an h2 goes to the next h2, stepping over the h3 between them.
	if !tb.stepSection(1, 10) {
		t.Fatalf("n should reach the next section")
	}
	if got := tb.secs[tb.sec].title; got != "Second" {
		t.Errorf("n from an h2 lands on %q, want the next h2", got)
	}
	// Esc's first meaning is up one level: out of the section, onto the list.
	tb.closeSection()
	if !tb.listing() {
		t.Errorf("closing a section goes back to the list")
	}
}

// The bottom border says which piece of how many, and inside a long one,
// how far down it you are. That is the position sense the sheet lacked.
func TestSectionHint(t *testing.T) {
	kids := []*ir.Node{hd(1, "Title"), para(text("lede"))}
	for i := 0; i < 3; i++ {
		kids = append(kids, hd(2, "Part "+itoa(i)))
		for j := 0; j < 30; j++ {
			kids = append(kids, para(text("line "+itoa(j))))
		}
	}
	root := doc(kids...)
	tb := &tab{root: root, cursor: -1}
	tb.relayout(40)
	tb.sec = 2
	if got := tb.sectionHint(10); got != "3/4 · Part 1" {
		t.Errorf("list hint is %q", got)
	}
	tb.openSection(10)
	if got := tb.sectionHint(10); !strings.HasPrefix(got, "3/4 · Part 1 · ") || !strings.HasSuffix(got, "%") {
		t.Errorf("reading hint is %q, want a percentage on it", got)
	}
}

// The user's word beats the cut: a page the sections suit badly goes back
// to being one sheet, and the menu row that does it says so.
func TestOneSheetOverrulesTheCut(t *testing.T) {
	root := doc(hd(1, "Title"), para(text("lede")),
		hd(2, "One"), para(text("a")), hd(2, "Two"), para(text("b")))
	tb := &tab{root: root, cursor: -1}
	tb.relayout(40)
	if !tb.listing() {
		t.Fatalf("three headings is a document")
	}
	tb.flat = true
	if tb.listing() {
		t.Errorf("one sheet means the whole page, not the list")
	}
	if lo, hi := tb.rowRange(); lo != 0 || hi != len(tb.lay.rows)-1 {
		t.Errorf("one sheet shows rows %d-%d, want the whole page", lo, hi)
	}
	// And it survives a re-layout: a width change is not a change of mind.
	tb.relayout(60)
	if !tb.flat || tb.listing() {
		t.Errorf("the override should outlive a relayout")
	}
}

// rowText is a section's rows as plain text.
func rowText(l layout, s section) []string {
	var out []string
	for i := s.first; i <= s.last && i < len(l.rows); i++ {
		out = append(out, l.rows[i].plain())
	}
	return out
}

// Wikipedia wraps every section in a region, whose named rule is drawn
// above the heading it opens. The boundary goes above that rule, so the
// rule reads as part of the section it introduces — and the sections still
// tile the page with no row left out of all of them.
func TestSectionsTileAcrossLandmarkRules(t *testing.T) {
	region := func(name string, kids ...*ir.Node) *ir.Node {
		return &ir.Node{Kind: ir.Landmark, Role: "region", Name: name, Children: kids}
	}
	root := doc(
		hd(1, "Title"), para(text("lede")),
		region("Background", hd(2, "Background"), para(text("one"))),
		region("Emulators", hd(2, "Emulators"), para(text("two"))),
	)
	l := render(root, 60)
	secs := sectionsOf(root, l, "")
	if got := titles(secs); strings.Join(got, "|") != "Title|Background|Emulators" {
		t.Fatalf("sections %v", got)
	}
	for i := 1; i < len(secs); i++ {
		if secs[i].first != secs[i-1].last+1 {
			t.Errorf("row %d belongs to no section or to two", secs[i-1].last+1)
		}
	}
	if secs[0].first != 0 || secs[len(secs)-1].last != len(l.rows)-1 {
		t.Errorf("the sections do not cover the page: %d-%d of %d rows",
			secs[0].first, secs[len(secs)-1].last, len(l.rows))
	}
	// The rule naming a region is inside the section it opens, not the
	// tail of the one before it.
	if !strings.Contains(strings.Join(rowText(l, secs[1]), "\n"), "Background") {
		t.Errorf("the region rule should open its own section")
	}
	if strings.Contains(strings.Join(rowText(l, secs[0]), "\n"), "region Background") {
		t.Errorf("the region rule is dangling off the end of the section before it")
	}
}

// A settle capture rebuilds the whole tree, so every node is a new
// pointer. The open section has to be found by the id Chromium gave it,
// or a page that keeps mutating drags the list back to the top on every
// redraw — which is what it did.
func TestSectionSurvivesARecapture(t *testing.T) {
	build := func() *ir.Node {
		h := func(level, id int, s string) *ir.Node {
			return &ir.Node{Kind: ir.Heading, Level: level, Name: s,
				ID: cdp.BackendNodeID(id), Children: []*ir.Node{text(s)}}
		}
		return doc(
			h(1, 10, "Title"), para(text("lede")),
			h(2, 20, "First"), para(text("one")),
			h(2, 30, "Second"), para(text("two")),
			h(2, 40, "Third"), para(text("three")),
		)
	}
	tb := &tab{root: build(), cursor: -1}
	tb.relayout(40)
	tb.sec = 3 // "Third"
	tb.openSection(10)

	// The same page arriving again: a new tree, the same node ids.
	tb.root = build()
	tb.relayout(40)
	if got := tb.secs[tb.sec].title; got != "Third" {
		t.Errorf("after a recapture the reader is on %q, want Third", got)
	}
	if !tb.read {
		t.Errorf("a recapture closed the open section")
	}
	// And with no ids at all, it holds its index rather than jumping home.
	plain := doc(hd(1, "Title"), para(text("lede")),
		hd(2, "First"), para(text("one")), hd(2, "Second"), para(text("two")))
	tb2 := &tab{root: plain, cursor: -1}
	tb2.relayout(40)
	tb2.sec = 2
	tb2.root = doc(hd(1, "Title"), para(text("lede")),
		hd(2, "First"), para(text("one")), hd(2, "Second"), para(text("two")))
	tb2.relayout(40)
	if tb2.sec != 2 {
		t.Errorf("without ids the list should hold its place; went to %d", tb2.sec)
	}
}

// A documentation page hangs a permalink inside its headings, and
// Chromium folds it into the accessible name: go.dev's outline read
// "Prerequisites Go to prerequisites". The list drops that tail — but
// only when it is a link into this same page, and only off the end.
func TestHeadingTitleDropsItsOwnAnchor(t *testing.T) {
	with := func(name string, l *ir.Node) *ir.Node {
		return &ir.Node{Kind: ir.Heading, Level: 2, Name: name,
			Children: []*ir.Node{text(name), l}}
	}
	cases := []struct{ name, want string }{
		{"Prerequisites Go to prerequisites", "Prerequisites"},
		{"Attributes ¶", "Attributes"},
	}
	for _, c := range cases {
		anchor := link(strings.TrimPrefix(c.name, c.want+" "), "https://ex.test/p#x", 5)
		if got := headingTitle(with(c.name, anchor), "https://ex.test/p"); got != c.want {
			t.Errorf("headingTitle(%q) = %q, want %q", c.name, got, c.want)
		}
	}
	// A link out of the page is part of the heading, not furniture.
	out := link("the spec", "https://spec.example/x", 6)
	if got := headingTitle(with("See the spec", out), "https://ex.test/p"); got != "See the spec" {
		t.Errorf("a heading ending in an outside link keeps it: got %q", got)
	}
	// And a heading that is nothing but its link keeps its name.
	only := &ir.Node{Kind: ir.Heading, Level: 2, Name: "func Split",
		Children: []*ir.Node{link("func Split", "https://ex.test/p#Split", 7)}}
	if got := headingTitle(only, "https://ex.test/p"); got != "func Split" {
		t.Errorf("a heading that is one link keeps it: got %q", got)
	}
}

// The panel's bottom border reads to how far through the open section the
// reader is — a progress bar that costs no row, the same one the header
// rule draws for a download. A section that fits on screen draws none.
func TestBorderFillsAsYouRead(t *testing.T) {
	kids := []*ir.Node{hd(1, "Title"), para(text("lede"))}
	for i := 0; i < 3; i++ {
		kids = append(kids, hd(2, "Part "+itoa(i)))
		for j := 0; j < 40; j++ {
			kids = append(kids, para(text("line "+itoa(j))))
		}
	}
	root := doc(kids...)
	tb := &tab{root: root, cursor: -1}
	tb.relayout(60)
	tb.sec, tb.read = 1, true
	tb.top = tb.secs[1].first

	if got := tb.readPct(20); got <= 0 || got >= 100 {
		t.Fatalf("at the top of a long section the reader is part way in: %d%%", got)
	}
	frame := panelFrameFilled(60, []string{}, "[2] Page", "1/4", toneFocus, tb.readPct(20))
	last := frame[strings.LastIndex(frame, "\n"):]
	if !strings.Contains(last, "━") {
		t.Errorf("the border should fill: %q", last)
	}
	// Scrolled to the end, it is full.
	tb.top = tb.secs[1].last
	if got := tb.readPct(20); got != 100 {
		t.Errorf("at the end of a section the border is full, got %d%%", got)
	}
	// A section that fits reports nothing to fill.
	tb.sec, tb.top = 0, tb.secs[0].first
	if got := tb.readPct(200); got != -1 {
		t.Errorf("a section that fits has no progress, got %d", got)
	}
}

// Depth is the nesting, not the tag: a page that skips a level still
// nests by one, and the ink follows the nesting so two depths never share
// a colour until the cycle comes round.
func TestSectionDepth(t *testing.T) {
	root := doc(
		hd(1, "Title"), para(text("lede")),
		hd(2, "First"), para(text("a")),
		hd(3, "Inner one"), para(text("b")),
		hd(3, "Inner two"), para(text("c")),
		hd(2, "Last"), para(text("d")),
	)
	secs := sectionsOf(root, render(root, 60), "")
	for i, d := range []int{1, 2, 3, 3, 2} {
		if secs[i].depth != d {
			t.Errorf("%q: depth %d, want %d", secs[i].title, secs[i].depth, d)
		}
	}
	// A page that skips a level still nests by one.
	skipped := doc(hd(1, "Title"), para(text("a")), hd(4, "Deep"), para(text("b")), hd(4, "Deep two"), para(text("c")))
	ss := sectionsOf(skipped, render(skipped, 60), "")
	if ss[1].depth != 2 {
		t.Errorf("h1 then h4 nests two deep, got %d", ss[1].depth)
	}
	if levelColor(ss[0].depth) == levelColor(ss[1].depth) {
		t.Error("two depths should not share an ink")
	}
	if levelColor(len(levelInk)+1) != levelColor(1) {
		t.Error("the cycle should come round rather than run off the end")
	}
}

// The ordinals came back with the key that uses them: `go`, a number,
// Enter — and the number is a LINE of what is showing (user, 2026-09-23).
// On the section list the lines are the sections; inside one they are
// its own lines, so the same key reaches both without a second meaning.
func TestGoToLineByNumber(t *testing.T) {
	root := doc(hd(1, "Title"), para(text("lede")),
		hd(2, "One"), para(text("a")), hd(2, "Two"), para(text("b")), hd(2, "Three"), para(text("c")))
	tb := &tab{root: root, cursor: -1}
	tb.relayout(60)
	if !tb.listing() {
		t.Fatal("a document opens on its section list")
	}
	if n := tb.lineCount(); n != len(tb.secs) {
		t.Errorf("on the list the lines are the sections: %d, want %d", n, len(tb.secs))
	}
	if !tb.goToLine(3, 10) || tb.sec != 2 {
		t.Errorf("go 3 lands on section 3 (index 2), landed on %d", tb.sec)
	}
	for _, bad := range []int{0, 99} {
		was := tb.sec
		if tb.goToLine(bad, 10) || tb.sec != was {
			t.Errorf("%d should move nothing, moved to %d", bad, tb.sec)
		}
	}
	// Reading one: its lines, from its first.
	tb.sec = 0
	tb.openSection(10)
	lo, hi := tb.rowRange()
	if n := tb.lineCount(); n != hi-lo+1 {
		t.Errorf("inside a section the lines are its own: %d, want %d", n, hi-lo+1)
	}
	// A short section cannot scroll, so the line is on screen rather
	// than at the top of it.
	if !tb.goToLine(2, 10) || tb.top > lo+1 || lo+1 >= tb.top+10 {
		t.Errorf("go 2 is the section's second line, on screen: top %d, line at %d", tb.top, lo+1)
	}
	if tb.lineText(1) == "" {
		t.Error("a line's text is what the list shows for it")
	}
}

// A movement key moves the cursor. It does not change which screen you
// are on: k at the top of an open section stops there, and Esc is the one
// move up (user, 2026-09-22 — the same objection as l opening a section).
func TestMovementDoesNotChangeScreen(t *testing.T) {
	root := doc(hd(1, "Title"), para(text("lede")),
		hd(2, "One"), para(text("prose "), link("a", "https://x.test/a", 1)),
		hd(2, "Two"), para(text("prose "), link("b", "https://x.test/b", 2)),
		hd(2, "Three"), para(text("prose "), link("c", "https://x.test/c", 3)))
	tb := &tab{root: root, cursor: -1}
	tb.relayout(60)
	tb.sec = 1
	tb.openSection(10)
	for _, k := range []string{"k", "up", "k", "h", "l", "gg"} {
		tb.moveItem(k, 10)
		if !tb.read {
			t.Fatalf("%q left the section", k)
		}
	}
	// Esc still does.
	tb.closeSection()
	if !tb.listing() {
		t.Error("Esc leaves the section for the list")
	}
	// A section that is all prose has no item to stop on, so the keys
	// scroll — and that must not leave either.
	prose := doc(hd(1, "Title"), para(text("lede")),
		hd(2, "One"), para(text("a")), hd(2, "Two"), para(text("b")), hd(2, "Three"), para(text("c")))
	tp := &tab{root: prose, cursor: -1}
	tp.relayout(60)
	tp.sec = 1
	tp.openSection(10)
	tp.cursor = -1
	tp.moveItem("k", 10)
	if !tp.read {
		t.Error("k in a section of pure prose left it")
	}
}

// A list item is one thing: one row, its first line, and Enter to go into
// it (user, 2026-09-22). The rule is the same whatever is in it — a
// bullet that already fits on one line simply looks the way it always
// did — and inside, the item's contents ARE the page, so the cursor and
// the scrolling work against them with no second set of rules.
func TestAListItemIsOneThing(t *testing.T) {
	card := func(key, summary string, id int) *ir.Node {
		return &ir.Node{Kind: ir.ListItem, ID: cdp.BackendNodeID(id), Children: []*ir.Node{
			para(link(key, "https://x.test/"+key, id+100)),
			para(text(summary)),
			para(text("Priority: Highest")),
		}}
	}
	root := doc(&ir.Node{Kind: ir.List, Children: []*ir.Node{
		card("AIS-1", "the first card", 10),
		card("AIS-2", "the second card", 20),
	}})
	tb := &tab{root: root, cursor: -1}
	tb.relayout(60)

	if n := len(tb.lay.items); n != 2 {
		t.Fatalf("two cards, two items; got %d:\n%s", n, dumpLayout(tb.lay))
	}
	v := dumpLayout(tb.lay)
	if !strings.Contains(v, "AIS-1") || strings.Contains(v, "the first card") {
		t.Errorf("a shut card shows its first line and no more:\n%s", v)
	}

	// Enter goes in, and the panel becomes that card.
	if !tb.drillInto(tb.lay.items[0].node, 60, 20) {
		t.Fatal("Enter on a list item should go into it")
	}
	if !tb.drilled() {
		t.Fatal("inside a list item")
	}
	in := dumpLayout(tb.lay)
	for _, want := range []string{"AIS-1", "the first card", "Priority"} {
		if !strings.Contains(in, want) {
			t.Errorf("inside, the whole card is the page; %q missing:\n%s", want, in)
		}
	}
	if strings.Contains(in, "AIS-2") {
		t.Errorf("the OTHER card is not on this page:\n%s", in)
	}
	if !strings.Contains(tb.drillTitle(), "AIS-1") {
		t.Errorf("the header row says what you are inside, says %q", tb.drillTitle())
	}
	// Esc comes back out, onto the card it left.
	tb.leaveDrill(60, 20)
	if tb.drilled() {
		t.Error("Esc comes back out")
	}
	if tb.cursor < 0 || tb.lay.items[tb.cursor].node.ID != 10 {
		t.Errorf("back out, the cursor is on the card just left: %d", tb.cursor)
	}
	// A card the page has rebuilt under a new id does not strand the view.
	tb.drill = []drillStep{{id: 999, title: "gone"}}
	tb.relayout(60)
	if tb.drilled() {
		t.Error("a card that is gone lets the page back")
	}
	// The path has no bound: a card holds a list whose items hold lists.
	inner := &ir.Node{Kind: ir.ListItem, ID: 50, Children: []*ir.Node{
		para(text("deep one")), para(text("and more")),
	}}
	outer := &ir.Node{Kind: ir.ListItem, ID: 40, Children: []*ir.Node{
		para(text("the outer card")),
		{Kind: ir.List, Children: []*ir.Node{inner}},
	}}
	tb2 := &tab{root: doc(&ir.Node{Kind: ir.List, Children: []*ir.Node{outer}}), cursor: -1}
	tb2.relayout(60)
	if !tb2.drillInto(tb2.lay.items[0].node, 60, 20) {
		t.Fatal("into the outer card")
	}
	if !tb2.drillInto(tb2.lay.items[0].node, 60, 20) {
		t.Fatalf("into the inner card, from inside the outer:\n%s", dumpLayout(tb2.lay))
	}
	if len(tb2.drill) != 2 {
		t.Errorf("two levels deep, the path has %d", len(tb2.drill))
	}
	if v := dumpLayout(tb2.lay); !strings.Contains(v, "and more") {
		t.Errorf("inside the inner card:\n%s", v)
	}
	tb2.leaveDrill(60, 20)
	if len(tb2.drill) != 1 {
		t.Errorf("Esc comes back ONE level, not all of them: %d", len(tb2.drill))
	}
}
