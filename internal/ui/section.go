package ui

import (
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

// A page cut into pieces.
//
// The page panel draws one very long sheet, and a sheet with no CSS has no
// space to remember a place by: you cannot tell where you are, how much is
// left, or how to get back to the paragraph you just read. Browsers solve
// that with layout; a terminal cannot. What a terminal has always solved it
// with — a BBS, a mail reader, a news reader — is to cut the content into a
// closed set of numbered, named pieces and give each one the whole screen.
//
// A section is one such piece: a heading and everything it holds. The range
// is the one heading folding already uses (render.walkSection): down to the
// next heading of the same level or higher, or the end of the landmark. So
// the cut is not a new guess about where a page divides — it is the cut the
// page already declared, which HTML's own outline algorithm specifies and no
// browser ever implemented.
type section struct {
	// node is the heading; nil for the run of content that comes before
	// the first one, which belongs to nobody.
	node  *ir.Node
	level int // 1-6; 0 for that leading run
	// depth is how deep the section nests, counting from 1 — what the
	// tree draws and what the ink says (theme.levelColor). It is not the
	// level: a page that goes h1 → h3 → h4 nests three deep, because the
	// shape of a hierarchy is the shape, not the tag names.
	depth int
	title string
	// first, last are the section's rows in the layout, inclusive.
	first, last int
	// body is the first row of what the section SAYS — past the landmark
	// rules that open it, past its own heading, past the blank under it.
	// Reading one section draws from here: the heading is the panel's
	// own header row while it is open, and printing it again under
	// itself was the page saying its name three times (2026-09-22).
	body int
	// what is inside, for the list's "is this worth opening" column
	tables, codes, media int
	// prose is how many of its rows carry text that is not a link, and
	// links how many carry one. A page with no main gives its navigation
	// columns headings like any other section — "Top Tutorials" over a
	// stack of links — and prose is what tells the two apart without
	// reading CSS.
	prose, links int
}

func (s section) lines() int { return s.last - s.first + 1 }

// pageShape is which of the closed set of screens a page gets. Measured
// against real pages, not guessed: a page that does not declare enough
// structure to cut is left exactly as it is drawn today.
type pageShape uint8

const (
	// shapeOne: one piece, drawn as the page always was. Forms, apps,
	// search results, anything with fewer than shapeDocMin headings.
	shapeOne pageShape = iota
	// shapeDoc: a document — a list of sections, one of them at a time.
	shapeDoc
)

// shapeDocMin is how many headings a page needs before it is treated as a
// document. Two headings is a page with a subtitle; three is an outline.
const shapeDocMin = 3

// sectionsOf cuts the laid-out page into sections. Only what is drawn is
// cut: a heading inside a shut landmark has no row and so is no section,
// and chrome is not here at all — it left for the pagetab.
func sectionsOf(root *ir.Node, lay layout, pageURL string) []section {
	if root == nil || len(lay.rows) == 0 {
		return nil
	}
	scope := mainOf(root)
	// From the marks, not the items: a heading with a link inside it is
	// not an item (the link is), and it is still a heading. marks holds
	// the first row of every one that was drawn, and the tree walk puts
	// them in reading order.
	type head struct {
		node *ir.Node
		row  int
	}
	var heads []head
	root.Walk(func(n *ir.Node) bool {
		if n.Kind != ir.Heading {
			return true
		}
		if at, ok := lay.marks[n]; ok && (scope == nil || holds(scope, n)) {
			heads = append(heads, head{n, at})
		}
		return true
	})
	if len(heads) == 0 {
		return nil
	}
	top := scopeStart(lay, scope)
	out := make([]section, 0, len(heads)+1)
	// Content before the first heading belongs to no section. It is still
	// content — a lede, an intro table — so it gets a piece of its own
	// rather than being swallowed by the heading that follows it.
	if heads[0].row > top {
		out = append(out, section{title: "(top)", first: top, last: heads[0].row - 1})
	}
	// Each boundary is one row, used by both sides, so the sections tile
	// the page with no gap and no overlap. A landmark's rule sits above
	// the heading it opens — Wikipedia wraps every section in a region —
	// so the boundary is drawn above that rule, not below it.
	start := func(i int) int {
		at := heads[i].row
		for at > 0 && isLandmarkRule(lay.rows[at-1]) {
			at--
		}
		return at
	}
	for i, h := range heads {
		first, last := h.row, len(lay.rows)-1
		if i > 0 {
			first = start(i)
		}
		if i+1 < len(heads) {
			last = max(first, start(i+1)-1)
		}
		out = append(out, section{
			node:  h.node,
			level: h.node.Level,
			title: headingTitle(h.node, pageURL),
			first: first,
			last:  last,
		})
	}
	for i := range out {
		count(&out[i], lay)
		out[i].body = bodyStart(out[i], lay)
	}
	out = pruneNav(out)
	// Depth is taken after the pruning, so a navigation column that left
	// the list does not leave a step in it either.
	for i := range out {
		out[i].depth = 1
	}
	var stack []int
	for i := range out {
		for len(stack) > 0 && stack[len(stack)-1] >= out[i].level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, out[i].level)
		out[i].depth = len(stack)
	}
	return out
}

// pruneNav drops the sections that are navigation columns rather than
// pieces of reading, and gives their rows to the section before them —
// nothing stops being drawn, it just stops being a row in the index.
//
// A page with no main gives its sidebar and footer link columns headings
// like any other section: w3schools' "HTML TUTORIAL", "Top Tutorials",
// "Top References". Nothing in the AX tree says they are furniture. What
// separates them is that they hold no prose of their own, nothing but
// links, and no section beneath them — a heading over a list of places to
// go, not over something to read.
//
// Measured across six pages (2026-09-22): it drops all ten of w3schools'
// navigation columns, nothing at all on go.dev's tutorial or Wikipedia,
// MDN's "Try it" and "Browser compatibility" (both headings over a
// script-driven widget that draws nothing in a terminal), and on
// pkg.go.dev its "Examples" and "Source Files" indexes — right — plus one
// deprecated func's heading, which is wrong. One bad drop in sixteen, and
// the heading it drops is still drawn inside the section above it.
func pruneNav(secs []section) []section {
	out := make([]section, 0, len(secs))
	for i, s := range secs {
		kid := i+1 < len(secs) && secs[i+1].level > s.level
		nav := s.node != nil && s.prose == 0 && s.links > 0 && !kid &&
			s.tables+s.codes+s.media == 0
		if nav && len(out) > 0 {
			out[len(out)-1].last = s.last
			continue
		}
		// The leading run is only a section when it holds something: a
		// page whose first row IS its heading has no lede, and a "(top)"
		// of one blank row at the head of every list is furniture.
		empty := s.node == nil && s.prose+s.links+s.tables+s.codes+s.media == 0
		if (nav || empty) && i+1 < len(secs) {
			// Nothing above it to fold into: the section below takes it.
			secs[i+1].first = s.first
			continue
		}
		out = append(out, s)
	}
	return out
}

// headingTitle is a heading said in one line, with its own anchor link
// taken off the end.
//
// A documentation page hangs a permalink inside its headings —
// <h2>Prerequisites <a href="#prerequisites">Go to prerequisites</a></h2>
// — and Chromium folds that into the accessible name, so go.dev's outline
// reads "Prerequisites Go to prerequisites" and MDN's ends in a pilcrow.
// The link is part of the heading on the page and no part of what it is
// called, so the list drops it. Only a link into this same page, and only
// at the end: a heading whose text genuinely ends in a link elsewhere
// keeps it.
func headingTitle(n *ir.Node, pageURL string) string {
	name := oneLine(nameOr(n.Name, n.Text()))
	for _, c := range n.Children {
		if c.Kind != ir.Link || sameFragment(pageURL, c.URL) == "" {
			continue
		}
		tail := oneLine(nameOr(c.Name, c.Text()))
		if tail == "" || tail == name {
			continue
		}
		if cut := strings.TrimSpace(strings.TrimSuffix(name, tail)); cut != "" {
			name = cut
		}
	}
	return name
}

// bodyStart is where a section's own words begin: past the landmark
// rules it opens with, past its heading row, past one blank under it.
func bodyStart(s section, lay layout) int {
	at := s.first
	for at <= s.last && at < len(lay.rows) && isLandmarkRule(lay.rows[at]) {
		at++
	}
	if s.node != nil && at <= s.last && at < len(lay.rows) {
		at++ // the heading itself
	}
	for at <= s.last && at < len(lay.rows) && strings.TrimSpace(lay.rows[at].plain()) == "" {
		at++
	}
	if at > s.last {
		// A section that is nothing but its heading: it still has to
		// show something, so it shows that.
		return s.first
	}
	return at
}

// scopeStart is the first row the content occupies: main's, or the page's.
func scopeStart(lay layout, scope *ir.Node) int {
	if scope == nil {
		return 0
	}
	if at, ok := lay.marks[scope]; ok {
		return at
	}
	for _, it := range lay.items {
		if holds(scope, it.node) {
			return it.first
		}
	}
	return 0
}

// count fills in what a section holds, off the rows rather than the tree:
// the rows are what the reader will actually meet.
func count(s *section, lay layout) {
	inTable, inCode := false, false
	for i := s.first; i <= s.last && i < len(lay.rows); i++ {
		r := lay.rows[i]
		if r.table && !inTable {
			s.tables++
		}
		if r.code && !inCode {
			s.codes++
		}
		inTable, inCode = r.table, r.code
	}
	for _, it := range lay.items {
		if it.first >= s.first && it.first <= s.last && it.node.Kind == ir.Media {
			s.media++
		}
	}
	for i := s.first; i <= s.last && i < len(lay.rows); i++ {
		hasProse, hasLink := false, false
		for _, g := range lay.rows[i].segs {
			if strings.TrimSpace(g.text) == "" {
				continue
			}
			switch g.kind {
			case segLink:
				hasLink = true
			case segPlain, segCode, segTableHeader:
				hasProse = true
			}
		}
		if hasProse {
			s.prose++
		}
		if hasLink {
			s.links++
		}
	}
}

// isLandmarkRule reports whether a row is the named rule that opens a
// landmark rather than any of its content.
func isLandmarkRule(r row) bool {
	named := false
	for _, g := range r.segs {
		switch {
		case g.kind == segLandmark:
			named = true
		case g.kind == segDim: // the rule the name sits on
		case strings.TrimSpace(g.text) != "":
			return false
		}
	}
	return named
}

// holds reports whether n is a descendant of a.
func holds(a, n *ir.Node) bool {
	found := false
	a.Walk(func(x *ir.Node) bool {
		if x == n {
			found = true
		}
		return !found
	})
	return found
}

// shapeOf says which screen the page gets. Only the leading run is not a
// heading, so it does not count towards the outline.
func shapeOf(secs []section) pageShape {
	n := 0
	for _, s := range secs {
		if s.node != nil {
			n++
		}
	}
	if n >= shapeDocMin {
		return shapeDoc
	}
	return shapeOne
}

// sectionAt is the section holding row r — where the cursor is, said in
// sections.
func sectionAt(secs []section, r int) int {
	for i, s := range secs {
		if r >= s.first && r <= s.last {
			return i
		}
	}
	return 0
}

// sectionAnchor is the fragment for the open section, "#emulators", when
// the page gave its heading an id — the reverse of the anchor table the
// capture carries. Empty otherwise, and empty while the whole page is
// shown: the address is the page's own then.
func (t *tab) sectionAnchor() string {
	if !t.read || t.sec >= len(t.secs) {
		return ""
	}
	n := t.secs[t.sec].node
	if n == nil || n.ID == 0 || strings.Contains(t.url, "#") {
		return ""
	}
	for name, id := range t.anchors {
		if id == n.ID {
			return "#" + name
		}
	}
	return ""
}

// sectionHint is what the panel's bottom border says while a page is being
// read as sections: which one of how many, its name, and — inside one — how
// far down it you are. It is the position sense a sheet of terminal text
// cannot otherwise give, which is the whole reason the cut exists.
func (t *tab) sectionHint(visible int) string {
	if len(t.secs) == 0 {
		return ""
	}
	s := t.secs[clamp(t.sec, 0, len(t.secs)-1)]
	at := itoa(t.sec+1) + "/" + itoa(len(t.secs)) + " · " + s.title
	if pct := t.readPct(visible); pct >= 0 {
		return at + " · " + itoa(pct) + "%"
	}
	return at
}

// readPct is how far through the open section the reader is, or -1 when
// the section fits on screen and there is no progress to report.
func (t *tab) readPct(visible int) int {
	if !t.read || t.sec >= len(t.secs) {
		return -1
	}
	s := t.secs[t.sec]
	if s.lines() <= visible {
		return -1
	}
	return clamp((t.top-s.first+visible)*100/s.lines(), 0, 100)
}

// nodeByID finds the node Chromium gave this backend id, or nil when the
// page has rebuilt that part of itself and it is gone.
func nodeByID(root *ir.Node, id cdp.BackendNodeID) *ir.Node {
	var found *ir.Node
	root.Walk(func(n *ir.Node) bool {
		if n.ID == id {
			found = n
		}
		return found == nil
	})
	return found
}

// isThing reports whether a node is one of the things the panel opens
// into: a list item, an article. Each is a single thing on the page
// however many lines it would take, so it is one row until you go in.
func isThing(n *ir.Node) bool {
	if n == nil || n.ID == 0 {
		return false
	}
	return n.Kind == ir.ListItem || (n.Kind == ir.Landmark && n.Role == "article")
}

// drillNode is the node the panel is showing: the root, or as far down
// the drill path as the page still has. A step the page has rebuilt out
// from under takes the rest of the path with it rather than stranding
// the view somewhere that no longer exists.
func (t *tab) drillNode(base *ir.Node) *ir.Node {
	n := base
	if n == nil {
		t.drill = nil
		return nil
	}
	for i, st := range t.drill {
		c := nodeByID(n, st.id)
		if c == nil {
			t.drill = t.drill[:i]
			return n
		}
		n = c
	}
	return n
}

// drillInto opens one of those things to the whole panel: its contents
// become the page. Enter goes in, Esc comes back out one level, and the
// path has no bound — a card holds a list whose items hold lists.
func (t *tab) drillInto(n *ir.Node, width, visible int) bool {
	if !isThing(n) {
		return false
	}
	title := oneLine(n.Text())
	if t.cursor >= 0 && t.cursor < len(t.lay.items) {
		if it := t.lay.items[t.cursor]; it.node == n && it.first < len(t.lay.rows) {
			title = strings.TrimSpace(t.lay.rows[it.first].plain())
		}
	}
	t.drill = append(t.drill, drillStep{id: n.ID, title: title})
	t.leavePagetab()
	t.relayout(width)
	t.cursor, t.top = t.firstItem(), 0
	t.scrollToCursor(visible)
	return true
}

// leaveDrill comes back out one level, the cursor on the thing just left.
func (t *tab) leaveDrill(width, visible int) {
	if len(t.drill) == 0 {
		return
	}
	was := t.drill[len(t.drill)-1].id
	t.drill = t.drill[:len(t.drill)-1]
	t.relayout(width)
	for i, it := range t.lay.items {
		if it.node.ID == was {
			t.cursor = i
			break
		}
	}
	t.scrollToCursor(visible)
}

// drilled reports whether the panel is inside something.
func (t *tab) drilled() bool { return t != nil && len(t.drill) > 0 }

// drillTitle is what the header row says: the thing you are inside.
func (t *tab) drillTitle() string {
	if len(t.drill) == 0 {
		return ""
	}
	return t.drill[len(t.drill)-1].title
}

// moveSection walks the section list. It does not wrap, and k off the top
// goes up onto the pagetab: the list stands where the page stands, so it
// answers the page's keys the way the page does (tab.moveItem).
//
// h and l do nothing here. In the page they walk the items ALONG a row,
// and a list row holds one thing; opening a section was briefly on l and
// that was wrong twice over — Enter is what opens, and a key does not
// get a second meaning because a surface happens to have room for it
// (user, 2026-09-22).
func (t *tab) moveSection(k string, visible int) {
	if len(t.secs) == 0 {
		return
	}
	half := max(1, visible/2)
	switch k {
	case "j", "down":
		t.sec++
	case "k", "up":
		if t.sec == 0 && t.enterPagetab() {
			return
		}
		t.sec--
	case "d", "ctrl+d":
		t.sec += half
	case "u", "ctrl+u":
		t.sec -= half
	case "gg":
		t.sec = 0
	case "G":
		t.sec = len(t.secs) - 1
	}
	t.sec = clamp(t.sec, 0, len(t.secs)-1)
	t.clampSecTop(visible)
}

// openSection gives the section under the list cursor the whole panel, the
// cursor on its first item.
func (t *tab) openSection(visible int) {
	if len(t.secs) == 0 {
		return
	}
	t.read = true
	t.top = t.secs[t.sec].first
	if first, _ := t.sectionItems(); first >= 0 {
		t.cursor = first
	}
	t.scrollToCursor(visible)
}

// closeSection goes back to the list, its cursor on the section just read.
func (t *tab) closeSection() {
	t.read = false
	t.clampSecTop(0)
}

// stepSection is n and p: the next or previous section at the same depth,
// without going back through the list. Without it every move between two
// sections costs a round trip, which is what makes a list-and-read pair
// tiring to use.
func (t *tab) stepSection(step, visible int) bool {
	if !t.read || len(t.secs) == 0 {
		return false
	}
	at := siblingSection(t.secs, t.sec, step)
	if at == t.sec {
		return false
	}
	t.sec = at
	t.openSection(visible)
	return true
}

// sectionItems is the span of item indexes inside the open section, or
// -1, -1 when it holds none. Items are in row order, so it is a range.
func (t *tab) sectionItems() (int, int) {
	lo, hi := t.rowRange()
	first, last := -1, -1
	for i, it := range t.lay.items {
		if it.first < lo || it.first > hi {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	return first, last
}

// siblings walks the outline at one level: n and p in the reading screen go
// to the next and previous section of the same depth, so walking a page's h2s
// does not fall into every h3 on the way. Nothing at that level (the last h3
// under an h2) falls back to the next section of any level.
func siblingSection(secs []section, at, step int) int {
	if len(secs) == 0 {
		return at
	}
	level := secs[at].level
	for i := at + step; i >= 0 && i < len(secs); i += step {
		if secs[i].level == level {
			return i
		}
	}
	return clamp(at+step, 0, len(secs)-1)
}
