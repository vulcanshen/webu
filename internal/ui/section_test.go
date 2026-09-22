package ui

import (
	"strings"
	"testing"

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
	secs := sectionsOf(root, l)
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
	secs := sectionsOf(root, render(root, 40))
	if len(secs) != 3 {
		t.Fatalf("a linked heading is still a heading: got %v", titles(secs))
	}
}

// Too few headings to be an outline: the page stays the one sheet it was.
func TestShapeNeedsAnOutline(t *testing.T) {
	root := doc(hd(1, "Title"), para(text("prose")), hd(2, "One"), para(text("more")))
	if got := shapeOf(sectionsOf(root, render(root, 40))); got != shapeOne {
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
	secs := sectionsOf(root, l)
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
	secs := sectionsOf(root, render(root, 40))
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
	lo, hi := tb.rowRange()
	if lo != tb.secs[1].first || hi != tb.secs[1].last {
		t.Errorf("reading shows rows %d-%d, the section is %d-%d", lo, hi, tb.secs[1].first, tb.secs[1].last)
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
	secs := sectionsOf(root, l)
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
