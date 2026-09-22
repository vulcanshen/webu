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
	partHeader partKind = iota
	partMain
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
func splitParts(root *ir.Node, boxes map[cdp.BackendNodeID]ir.Box) []part {
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
	anchor := 0
	for i, b := range blocks {
		if b.box.Area() > blocks[anchor].box.Area() {
			anchor = i
		}
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

// beside reports whether a sits level with b but not over it: their rows
// overlap and their columns do not.
func beside(a, b ir.Box) bool {
	return a.Y < b.Y+b.H && b.Y < a.Y+a.H && // level with it
		(a.X+a.W <= b.X || b.X+b.W <= a.X) // and out of its way
}
