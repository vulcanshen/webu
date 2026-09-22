package ui

import (
	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

// A page has four parts, and only one of them is required (user,
// 2026-09-22):
//
//	topheader     what comes before the body
//	mainbody      the page itself
//	bottomfooter  what comes after it
//	others        what sits beside it — a side menu, a drawer, a rail
//
// None of them is a tag and none of them is a role. The seven roles that
// used to decide this — banner, navigation, search, complementary,
// contentinfo, dialog, alertdialog — have no special standing any more,
// because deciding by role failed twice in a row on exactly the pages
// where it mattered: Google's search form and Jira's board were the whole
// of their pages and both were carried off as furniture. A page's parts
// are WHERE things are, which every page has whether or not its author
// marked anything up. Hacker News has no landmark at all and still has
// four parts.
//
// Where things are comes from the layout webu already captures, not from
// CSS it refuses to read: the split needs no stylesheet, only the
// rectangles Chromium laid the page out into.
type partKind uint8

const (
	// The body is FIRST so that it is the zero value: a page opens on
	// its body (user, 2026-09-22), and a tab that has never been told
	// otherwise is on it. The order they are drawn in is not this one —
	// that is the order of the page, and splitParts lists it.
	partMain partKind = iota
	partHeader
	partOthers
	partFooter
)

func (k partKind) word() string {
	switch k {
	case partHeader:
		return "header"
	case partOthers:
		return "others"
	case partFooter:
		return "footer"
	}
	return "body"
}

// part is one of them: the nodes it holds, in reading order.
type part struct {
	kind  partKind
	nodes []*ir.Node
	box   ir.Box
}

// splitParts cuts a page into its parts.
//
// The anchor is the biggest block on the page by AREA, and everything
// else is placed against it. Area rather than weight: Google's home page
// is a search form with almost no text on it, and every measure of
// content picks its "other languages" line over the box the page exists
// for. Area picks the box.
//
// It is the page's top-level blocks that are measured, and in the IR that
// is already a short list — three to eight on every page tried. The
// wrapper chains that make this hard in the DOM are not here: Chromium
// leaves them out of the accessibility tree, and what it keeps webu
// dissolves (ir.Build). Reasoning about this in HTML was reasoning about
// a problem we do not have (user, 2026-09-23).
//
// Some pages have no parts at all, and say so by returning none: the
// pagetab does not appear and the panel is the whole page.
func splitParts(root *ir.Node, boxes map[cdp.BackendNodeID]ir.Box, view ir.Box) []part {
	if root == nil || len(boxes) == 0 {
		return nil
	}
	type block struct {
		node *ir.Node
		box  ir.Box
	}
	var blocks []block
	for _, c := range root.Children {
		if b := boxOf(c, boxes); b.Area() > 0 {
			blocks = append(blocks, block{c, b})
		}
	}
	if len(blocks) == 0 {
		return nil
	}
	anchor, ink := 0, 0.0
	for i, b := range blocks {
		ink += b.box.Area()
		if b.box.Area() > blocks[anchor].box.Area() {
			anchor = i
		}
	}
	// Two ways a page has no parts, and either is enough.
	//
	// It fits the window. The four parts exist so the reader does not
	// wade through chrome to reach the content, and there is no wading on
	// a page that arrives whole — a heading and a paragraph have a
	// biggest block by arithmetic and a body by nothing.
	//
	// Or nothing dominates it. A plain document is a stack of paragraphs,
	// every one as wide as the page and shorter than it; cut that stack
	// anywhere and the cut is webu's, not the page's.
	var page ir.Box
	for _, b := range blocks {
		page = page.Union(b.box)
	}
	if view.H > 0 && page.H <= view.H {
		return nil
	}
	if blocks[anchor].box.Area() < ink*bodyShare {
		return nil
	}
	main := blocks[anchor].box

	byKind := map[partKind]*part{}
	add := func(k partKind, n *ir.Node, b ir.Box) {
		p, ok := byKind[k]
		if !ok {
			p = &part{kind: k}
			byKind[k] = p
		}
		p.nodes = append(p.nodes, n)
		p.box = p.box.Union(b)
	}
	for i, b := range blocks {
		switch {
		case i == anchor:
			add(partMain, b.node, b.box)
		case beside(b.box, main):
			// Level with the body but not over it: a side menu however
			// the page chose to place it — left, right, a drawer, a rail
			// that follows the scroll. "Beside" is the only thing they
			// have in common and it is the only thing webu can see.
			add(partOthers, b.node, b.box)
		case i < anchor:
			add(partHeader, b.node, b.box)
		default:
			add(partFooter, b.node, b.box)
		}
	}
	out := make([]part, 0, 4)
	for _, k := range []partKind{partHeader, partMain, partOthers, partFooter} {
		if p, ok := byKind[k]; ok {
			out = append(out, *p)
		}
	}
	return out
}

// boxOf is where a node was laid out: its own rectangle, or the one
// around everything under it when the node itself has none. A landmark
// whose children are positioned has no box of its own.
func boxOf(n *ir.Node, boxes map[cdp.BackendNodeID]ir.Box) ir.Box {
	var b ir.Box
	n.Walk(func(x *ir.Node) bool {
		b = b.Union(boxes[x.ID])
		return true
	})
	return b
}

// bodyShare is how much of a page's laid-out area the body has to be
// before webu believes in it. Measured, not chosen: the biggest block is
// 94% of Wikipedia, 90% of Hacker News and 70% of MDN — and 16% of a
// page with no structure at all, where the blocks are fifteen paragraphs
// of much the same size. A quarter sits in the gap (2026-09-23).
const bodyShare = 0.25

// beside reports whether a sits level with b but not over it: their rows
// overlap and their columns do not.
func beside(a, b ir.Box) bool {
	return a.Y < b.Y+b.H && b.Y < a.Y+a.H && // level with it
		(a.X+a.W <= b.X || b.X+b.W <= a.X) // and out of its way
}

// activePart is the part the panel is showing, or nil when the page has
// none to show.
func (t *tab) activePart() *part {
	for i := range t.parts {
		if t.parts[i].kind == t.at {
			return &t.parts[i]
		}
	}
	// The part the tab was on is gone — a page that used to have a side
	// menu and does not any more. The body always exists when anything
	// does, so it is where the hand lands.
	for i := range t.parts {
		if t.parts[i].kind == partMain {
			t.at = partMain
			return &t.parts[i]
		}
	}
	if len(t.parts) > 0 {
		t.at = t.parts[0].kind
		return &t.parts[0]
	}
	return nil
}

// partIndex is where a part sits on the pagetab, or 0.
func (t *tab) partIndex(k partKind) int {
	for i, p := range t.parts {
		if p.kind == k {
			return i
		}
	}
	return 0
}

// showPart switches the panel to a part, at its start — the way opening
// anything else does. Where the hand is is not its business: on the
// pagetab the hand stays up there and shows each part as it reaches it
// (tab.stepPagetab).
func (t *tab) showPart(k partKind, width, visible int) {
	t.at = k
	t.drill = nil
	t.relayout(width)
	t.cursor, t.top = t.firstItem(), 0
	if t.cursor >= 0 {
		t.top = clamp(t.lay.items[t.cursor].first, 0, max(0, len(t.lay.rows)-1))
	}
	t.scrollToCursor(visible)
}

// partHint is what the panel's bottom border says while the hand is on
// the pagetab: how much is in the part under it. It no longer says
// whether that part is the one on screen, because it always is now — the
// hand and the panel move together (pagepanel.partChain).
func partHint(t *tab) string {
	i := t.pagetabIndex()
	if i < 0 || i >= len(t.parts) {
		return ""
	}
	n := 0
	for _, node := range t.parts[i].nodes {
		n += countItems(node)
	}
	return plural(n, "item") + " · Enter to stay"
}

// withLead puts the menu glyph on the chain's first segment. The pagetab
// is one strip, so it says so once, at its head (user, 2026-09-22).
func withLead(labels []string) []string {
	if len(labels) == 0 {
		return labels
	}
	out := append([]string(nil), labels...)
	out[0] = glyphMenu + " " + out[0]
	return out
}
