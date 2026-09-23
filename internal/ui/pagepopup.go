package ui

import (
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

// A page's popup is something that was not shown, that appeared after
// the user did something, and that wants an answer (user, 2026-09-23).
// It is not one of the page's four parts: while it is up it IS the
// panel, and Esc does not leave it — a popup exists to be decided, and a
// decision put off with Esc is a window minimised and forgotten. The
// global keys stay live (another tab, a URL, quit); only the page under
// it is out of reach, which it is in the browser too. (webu's own
// floats — the menus, the boxes — are popup.go; this is the page's.)
//
// Told apart by behaviour, never by tag: a block at the top of the tree
// that the last capture did not have, that the page put the keyboard
// into or declared modal, and that holds something to press. A toast
// that appears after a save has nothing to press and is content; a chat
// bubble that appears on its own took no keyboard and is content. A
// block with none of that is left where the page put it, and the parts
// place it like anything else.
//
// popupGrace is how long after a press — or a navigation — an appearing
// block can still be the answer to it. A page that keeps building itself
// long after would otherwise turn its every late arrival into a demand.
const popupGrace = settleGrace

// findPopup is the block the page just put up, or nil. prev is every
// node the last capture had, by Chromium's id — empty right after a
// navigation, when everything is new and a dialog the page opens on
// load is a popup like any other (user, 2026-09-23). skip is what is
// already taken: the popups on the stack. A block that is not a popup
// is looked into, for the one it may be wrapped around — a dialog is as
// often inside main as beside it, and as often inside a wrapper the
// page made for it.
func findPopup(prev, skip map[cdp.BackendNodeID]bool, root *ir.Node, boxes map[cdp.BackendNodeID]ir.Box) *ir.Node {
	return findPopupIn(prev, skip, root, boxes, nil)
}

// findPopupIn is findPopup told the DOM's parents (Capture.Parents): what
// a block is over is judged against every box the page laid out, and
// the boxes of the block's own DOM — the text inside its buttons, the
// html and body around it — are not what it is over. The IR alone
// cannot say: a button is a leaf in it, its text is not.
func findPopupIn(prev, skip map[cdp.BackendNodeID]bool, root *ir.Node, boxes map[cdp.BackendNodeID]ir.Box, parents map[cdp.BackendNodeID]cdp.BackendNodeID) *ir.Node {
	if root == nil {
		return nil
	}
	var found *ir.Node
	var walk func(n *ir.Node, chain []*ir.Node)
	walk = func(n *ir.Node, chain []*ir.Node) {
		chain = append(chain, n)
		for _, c := range n.Children {
			if found != nil {
				return
			}
			if c.ID != 0 && !skip[c.ID] && (len(prev) == 0 || !prev[c.ID]) && isPopup(c, chain, boxes, parents) {
				found = c
				return
			}
			walk(c, chain)
		}
	}
	walk(root, nil)
	return found
}

// isPopup is the test on one block: it wants an answer — the page put
// the keyboard into it, or called it a dialog or a menu — it has
// something to press, and unless it is declared modal it is OVER
// something: some element the page laid out, not its own and not one
// of the ancestors holding it, is half covered by it, or half of it
// lies on such an element. A block the page laid out in its flow covers
// nothing and is covered by nothing but what holds it.
func isPopup(c *ir.Node, chain []*ir.Node, boxes map[cdp.BackendNodeID]ir.Box, parents map[cdp.BackendNodeID]cdp.BackendNodeID) bool {
	if countItems(c) == 0 {
		return false
	}
	declared := c.Modal || (c.Kind == ir.Landmark && (c.Role == "dialog" || c.Role == "alertdialog")) ||
		c.Role == "menu"
	if !declared && !hasFocus(c) {
		return false
	}
	if c.Modal {
		// Declared modal: the declaration is the whole of the evidence,
		// and it is enough — the page said this is over everything.
		return true
	}
	b := boxOf(c, boxes)
	if b.Area() <= 0 {
		return false
	}
	// Its own: the IR under it and over it, and — where the DOM's
	// parents are known — every DOM node under it and over it too.
	own := map[cdp.BackendNodeID]bool{}
	c.Walk(func(x *ir.Node) bool { own[x.ID] = true; return true })
	for _, a := range chain {
		own[a.ID] = true
	}
	for id, hop := c.ID, 0; id != 0 && hop < 256; id, hop = parents[id], hop+1 {
		own[id] = true
	}
	related := func(id cdp.BackendNodeID) bool {
		if own[id] {
			return true
		}
		for x, hop := id, 0; x != 0 && hop < 256; x, hop = parents[x], hop+1 {
			if x == c.ID {
				return true
			}
		}
		return false
	}
	for id, ob := range boxes {
		if ob.Area() <= 0 || related(id) {
			continue
		}
		if o := overlap(b, ob); o >= ob.Area()/2 || o >= b.Area()/2 {
			return true
		}
	}
	return false
}

// hasFocus reports whether the page's keyboard is somewhere under n.
func hasFocus(n *ir.Node) bool {
	found := false
	n.Walk(func(x *ir.Node) bool {
		if x.Focused {
			found = true
		}
		return !found
	})
	return found
}

// overlap is the area two boxes share.
func overlap(a, b ir.Box) float64 {
	w := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
	h := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// allIDs is every node of the tree by Chromium's id: what the next
// capture is diffed against.
func allIDs(root *ir.Node) map[cdp.BackendNodeID]bool {
	out := map[cdp.BackendNodeID]bool{}
	if root == nil {
		return out
	}
	root.Walk(func(n *ir.Node) bool {
		if n.ID != 0 {
			out[n.ID] = true
		}
		return true
	})
	return out
}

// popupNode is the popup on top of the stack, as the current tree has
// it, or nil — the page's popups stack the way webu's own do (VTP's
// z-axis): a dialog opened from a dialog sits over it, and answering
// the top one uncovers the one below.
func (t *tab) popupNode() *ir.Node {
	if len(t.popups) == 0 || t.root == nil {
		return nil
	}
	return nodeByID(t.root, t.popups[len(t.popups)-1])
}

// popupSet is the stack as a set, for what the diff must not take twice.
func (t *tab) popupSet() map[cdp.BackendNodeID]bool {
	out := map[cdp.BackendNodeID]bool{}
	for _, id := range t.popups {
		out[id] = true
	}
	return out
}

// noticePopup runs after a capture is built: the popups the page has
// taken down are let go of, and one it has just put up is taken, on
// top. True when the popup on top changed, so the caller can start the
// cursor over.
func (t *tab) noticePopup() bool {
	was := cdp.BackendNodeID(0)
	if len(t.popups) > 0 {
		was = t.popups[len(t.popups)-1]
	}
	var p *ir.Node
	if time.Now().Before(t.popupUntil) {
		p = findPopupIn(t.prevTop, t.popupSet(), t.root, t.boxes, t.parents)
	}
	switch {
	case p != nil:
		// One more on top. The ones under it are NOT let go of, though
		// they may be gone from the tree: Chromium prunes everything
		// behind a modal dialog, the dialog it was opened from included,
		// and that one comes back when this one is answered.
		if len(t.popups) == 0 {
			// The page as it was the moment before: the backdrop, for
			// as long as the page under the popup is not in the tree —
			// and where the reader was on it, to come back to.
			t.back, t.backParts, t.backSecs = t.lay, t.parts, t.secs
			t.backRead, t.backSec, t.backTop = t.read, t.sec, t.top
		}
		t.popups = append(t.popups, p.ID)
	default:
		// Nothing new: whatever the page has taken down is let go of.
		kept := t.popups[:0]
		for _, id := range t.popups {
			if nodeByID(t.root, id) != nil {
				kept = append(kept, id)
			}
		}
		t.popups = kept
	}
	t.prevTop = allIDs(t.root)
	now := cdp.BackendNodeID(0)
	if len(t.popups) > 0 {
		now = t.popups[len(t.popups)-1]
	}
	return now != was
}

// sansPopup is the page without the popup over it: what the parts are
// cut from while one is up, so that closing it puts the page back the
// way it was.
func sansPopup(root *ir.Node, popups []cdp.BackendNodeID) *ir.Node {
	if len(popups) == 0 || root == nil {
		return root
	}
	skip := map[cdp.BackendNodeID]bool{}
	for _, id := range popups {
		skip[id] = true
	}
	// Only the top level is copied: the parts are cut from the top-level
	// blocks, and a popup deeper than that is inside one of them, whose
	// box it does not change enough to matter.
	out := &ir.Node{Kind: ir.Document, Name: root.Name, URL: root.URL, ID: root.ID}
	for _, c := range root.Children {
		if !skip[c.ID] {
			out.Children = append(out.Children, c)
		}
	}
	return out
}

// backdrop is the tab as the panel draws it under a popup: the page's
// own layout, dimmed the way a page being left is, and no cursor — the
// cursor is in the float. A copy, so nothing about the tab moves.
func (t *tab) backdrop() *tab {
	bt := *t
	bt.lay = t.back
	bt.parts = t.backParts
	bt.secs, bt.read, bt.sec, bt.top = t.backSecs, t.backRead, t.backSec, t.backTop
	bt.gutter = lineNumW(len(t.back.rows))
	bt.cursor = -1
	bt.loading = true
	bt.popups = nil
	bt.drill = nil
	if bt.sec >= len(bt.secs) {
		bt.read = false
	}
	return &bt
}

// popupTitleOf is what a popup's border says: its name, or its first
// line when the page gave it none.
func (t *tab) popupTitleOf(p *ir.Node) string {
	// p is nil for one Chromium has pruned out of the tree behind the
	// popup over it: its layout, kept from when it was on top, still
	// has its first line.
	if p != nil {
		if s := oneLine(p.Name); s != "" {
			return s
		}
	}
	for _, r := range t.lay.rows {
		if s := strings.TrimSpace(r.plain()); s != "" {
			return s
		}
	}
	return "popup"
}
