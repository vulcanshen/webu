package ui

import (
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
// popupGrace is how long after a press an appearing block can still be
// the answer to it. A page that keeps building itself long after would
// otherwise turn its every late arrival into a demand.
const popupGrace = settleGrace

// findPopup is the block a press just brought up, or nil. prev is the
// top of the tree as the last capture had it, by Chromium's id.
func findPopup(prev map[cdp.BackendNodeID]bool, root *ir.Node, boxes map[cdp.BackendNodeID]ir.Box) *ir.Node {
	if root == nil || len(prev) == 0 {
		return nil
	}
	for _, c := range root.Children {
		if c.ID == 0 || prev[c.ID] || countItems(c) == 0 {
			continue
		}
		if !c.Modal && !hasFocus(c) {
			continue
		}
		if c.Modal {
			// Chromium prunes the tree behind a modal dialog, so there
			// may be nothing left for it to overlap: the declaration is
			// the whole of the evidence, and it is enough.
			return c
		}
		b := boxOf(c, boxes)
		if b.Area() <= 0 {
			continue
		}
		for _, o := range root.Children {
			if o == c {
				continue
			}
			if ob := boxOf(o, boxes); ob.Area() > 0 && overlap(b, ob) >= b.Area()/2 {
				// Over something rather than after it: a block the
				// page laid out in its flow overlaps nothing.
				return c
			}
		}
	}
	return nil
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

// topIDs is the top of the tree by Chromium's id: what the next capture
// is diffed against.
func topIDs(root *ir.Node) map[cdp.BackendNodeID]bool {
	out := map[cdp.BackendNodeID]bool{}
	if root == nil {
		return out
	}
	for _, c := range root.Children {
		if c.ID != 0 {
			out[c.ID] = true
		}
	}
	return out
}

// popupNode is the popup the tab is inside, as the current tree has it,
// or nil — gone from the tree is gone from the screen.
func (t *tab) popupNode() *ir.Node {
	if t.popup == 0 || t.root == nil {
		return nil
	}
	for _, c := range t.root.Children {
		if c.ID == t.popup {
			return c
		}
	}
	return nil
}

// noticePopup runs after a capture is built: the popup the page just
// took down is let go of, and one it just put up is taken. True when
// which popup the tab is in changed, so the caller can start the cursor
// over.
func (t *tab) noticePopup(fresh bool) bool {
	was := t.popup
	if t.popupNode() == nil {
		t.popup = 0
	}
	if t.popup == 0 && !fresh && time.Now().Before(t.popupUntil) {
		if p := findPopup(t.prevTop, t.root, t.boxes); p != nil {
			t.popup = p.ID
		}
	}
	t.prevTop = topIDs(t.root)
	return t.popup != was
}

// sansPopup is the page without the popup over it: what the parts are
// cut from while one is up, so that closing it puts the page back the
// way it was.
func sansPopup(root *ir.Node, popup cdp.BackendNodeID) *ir.Node {
	if popup == 0 || root == nil {
		return root
	}
	out := &ir.Node{Kind: ir.Document, Name: root.Name, URL: root.URL, ID: root.ID}
	for _, c := range root.Children {
		if c.ID != popup {
			out.Children = append(out.Children, c)
		}
	}
	return out
}
