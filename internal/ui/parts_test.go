package ui

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

// A page's parts are WHERE things are, not what they are called. None of
// this reads a tag, an attribute or a role — only the rectangles Chromium
// laid the page out into (user, 2026-09-22).
func TestAPageHasFourParts(t *testing.T) {
	box := func(x, y, w, h float64) ir.Box { return ir.Box{X: x, Y: y, W: w, H: h} }
	// The window the page was laid out in: every page below is taller
	// than it, so every page below scrolls.
	screen := ir.Box{W: 1000, H: 600}
	node := func(role string, id int) *ir.Node {
		return &ir.Node{Kind: ir.Landmark, Role: role, ID: cdp.BackendNodeID(id)}
	}
	// The shape every application has: a bar across the top, a menu down
	// one side, the page beside it, a bar across the bottom.
	head, side, body, foot := node("banner", 1), node("navigation", 2), node("main", 3), node("contentinfo", 4)
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{head, side, body, foot}}
	boxes := map[cdp.BackendNodeID]ir.Box{
		1: box(0, 0, 1000, 60),
		2: box(0, 60, 200, 900),
		3: box(200, 60, 800, 900),
		4: box(0, 960, 1000, 80),
	}
	got := map[partKind][]string{}
	for _, p := range splitParts(root, boxes, screen) {
		for _, n := range p.nodes {
			got[p.kind] = append(got[p.kind], n.Role)
		}
	}
	for kind, want := range map[partKind]string{
		partHeader: "banner", partOthers: "navigation", partMain: "main", partFooter: "contentinfo",
	} {
		if strings.Join(got[kind], ",") != want {
			t.Errorf("%s: %v, want %s", kind.word(), got[kind], want)
		}
	}

	// The side menu on the OTHER side lands in the same part: how the
	// page chose to place it is not something webu has an opinion about.
	boxes[2], boxes[3] = box(800, 60, 200, 900), box(0, 60, 800, 900)
	got = map[partKind][]string{}
	for _, p := range splitParts(root, boxes, screen) {
		for _, n := range p.nodes {
			got[p.kind] = append(got[p.kind], n.Role)
		}
	}
	if strings.Join(got[partOthers], ",") != "navigation" {
		t.Errorf("a menu on the right is still beside the body: %v", got)
	}

	// A page that arrives whole has no parts: there is no wading through
	// chrome to reach content that is already on screen.
	if ps := splitParts(root, boxes, ir.Box{W: 1000, H: 2000}); ps != nil {
		t.Errorf("a page inside its window is one page: %+v", ps)
	}

	// Nor has a page where nothing dominates. A plain document is a stack
	// of paragraphs, each as wide as the page and shorter than it, and
	// any line drawn through it is webu's line, not the page's.
	var flat []*ir.Node
	stack := map[cdp.BackendNodeID]ir.Box{}
	for i := 0; i < 40; i++ {
		flat = append(flat, node("paragraph", 100+i))
		stack[cdp.BackendNodeID(100+i)] = box(0, float64(30*i), 1000, 28)
	}
	if ps := splitParts(&ir.Node{Kind: ir.Document, Children: flat}, stack, screen); ps != nil {
		t.Errorf("a flat page is not cut up: %+v", ps)
	}
}

// The anchor is the biggest block by AREA, not by how much content it
// holds. Google's home page is a search form with almost no text on it,
// and every measure of content picks its "other languages" line over the
// box the page exists for (2026-09-23, measured).
func TestTheBiggestBlockIsTheBody(t *testing.T) {
	box := func(x, y, w, h float64) ir.Box { return ir.Box{X: x, Y: y, W: w, H: h} }
	text := func(s string, id int) *ir.Node {
		return &ir.Node{Kind: ir.Group, ID: cdp.BackendNodeID(id),
			Children: []*ir.Node{{Kind: ir.Text, Name: s}}}
	}
	search := &ir.Node{Kind: ir.Landmark, Role: "search", ID: 2,
		Children: []*ir.Node{{Kind: ir.Textbox, ID: 20, Focusable: true}}}
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		&ir.Node{Kind: ir.Landmark, Role: "navigation", ID: 1},
		search,
		text("Google offered in: English Deutsch Francais", 3),
		&ir.Node{Kind: ir.Landmark, Role: "contentinfo", ID: 4},
	}}
	boxes := map[cdp.BackendNodeID]ir.Box{
		1: box(0, 0, 1000, 40),
		2: box(200, 200, 600, 400), 20: box(200, 300, 600, 40),
		3: box(0, 650, 1000, 30),
		4: box(0, 900, 1000, 60),
	}
	for _, p := range splitParts(root, boxes, ir.Box{W: 1000, H: 600}) {
		if p.kind == partMain {
			if len(p.nodes) != 1 || p.nodes[0] != search {
				t.Errorf("the body is the box the page exists for, is %+v", p.nodes)
			}
			return
		}
	}
	t.Error("every page has a body")
}
