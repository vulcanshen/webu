// Package page is the CDP side of the translation layer: what webu asks a
// tab for (its accessibility tree, its layout) and what it tells the tab to
// do (click this node, type into that one). internal/ir turns the answers
// into a tree; internal/ui draws it. Nothing here knows about terminals.
package page

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/domsnapshot"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/vulcanshen/webu/internal/ir"
)

// run executes fn against the tab: chromedp.Run is what puts the target's
// executor in the context, and a raw cdproto call without it answers
// "invalid context". Every entry point in this package goes through here,
// so a caller holding a plain tab context never has to know.
func run(ctx context.Context, fn func(context.Context) error) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(fn))
}

// Capture reads everything the translator needs from a tab: the full AX
// tree, and the computed `display` of every DOM node that has a layout box.
//
// The second is what tells a div from a span. The AX tree deliberately
// carries no layout, and a generic container looks the same whether it is a
// block or an inline; without this, two adjacent <div>s of text would run
// together on one line (ir.Build). DOMSnapshot is one call for the whole
// page, which is what makes it affordable on every redraw.
func Capture(ctx context.Context, frames []cdp.FrameID) (ir.Capture, error) {
	var c ir.Capture
	err := run(ctx, func(ctx context.Context) error {
		nodes, err := accessibility.GetFullAXTree().Do(ctx)
		if err != nil {
			return err
		}
		docs, strs, err := domsnapshot.CaptureSnapshot([]string{"display"}).Do(ctx)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return errors.New("snapshot: no document")
		}
		c = captureDoc(nodes, docs[0], strs)
		// The frames the page holds, by the element that holds each, and
		// the opened ones captured whole under their owner (ir.Build
		// splices them in).
		attachFrames(ctx, &c, docs, strs, 0, frames)
		// The window the page was laid out in. A page that fits inside it
		// has no parts: the four exist so the reader does not wade through
		// chrome to reach content, and there is no wading on a page that
		// is already all on screen (ui.splitParts, 2026-09-23).
		if _, _, _, _, css, _, err := cdppage.GetLayoutMetrics().Do(ctx); err == nil && css != nil {
			c.Viewport = ir.Box{W: css.ClientWidth, H: css.ClientHeight}
		}
		// What the response was: a JSON or plain-text document is drawn as
		// its text, not as Chrome's viewer for it (ir.Build).
		if obj, _, err := runtime.Evaluate("document.contentType").WithReturnByValue(true).Do(ctx); err == nil && obj != nil {
			var ct string
			if json.Unmarshal(obj.Value, &ct) == nil {
				c.ContentType = ct
			}
		}
		return nil
	})
	return c, err
}

// attachFrames fills in the frames a document holds, by the element that
// holds each. The snapshot has every same-process document and says
// which element owns which; a frame from another site is in another
// process, so not in the snapshot — its <iframe> is, and DOM.describeNode
// says which frame it holds: the id of a target of its own (sessions.go).
// The frames the reader has opened are captured whole, under their owner
// — a same-process one from here, another site's through its session —
// and each of those has its own frames looked at the same way, so a
// frame inside a frame is reached too.
func attachFrames(ctx context.Context, c *ir.Capture, docs []*domsnapshot.DocumentSnapshot, strs []string, cur int, frames []cdp.FrameID) {
	owners := frameOwners(docs, strs, cur)
	c.FrameOf = map[cdp.BackendNodeID]string{}
	for owner, fr := range owners {
		c.FrameOf[owner] = string(fr.id)
	}
	remote := map[cdp.BackendNodeID]cdp.FrameID{}
	for owner := range c.Srcs {
		if _, ok := owners[owner]; ok {
			continue
		}
		if n, err := dom.DescribeNode().WithBackendNodeID(owner).Do(ctx); err == nil && n != nil && n.FrameID != "" {
			remote[owner] = n.FrameID
			c.FrameOf[owner] = string(n.FrameID)
		}
	}
	keep := func(owner cdp.BackendNodeID, fc *ir.Capture) {
		if c.Frames == nil {
			c.Frames = map[cdp.BackendNodeID]*ir.Capture{}
		}
		c.Frames[owner] = fc
		// Its boxes join the page's, placed where its owner is and keyed
		// as the tree will carry them: the parts and the popups are cut
		// by geometry, and a frame's content is on the page too
		// (ui.splitParts, pagepopup). A frame's own boxes are in its
		// own coordinates, from its top-left corner; a frame inside it
		// has already been placed in those.
		at := c.Boxes[owner]
		if c.Boxes == nil {
			c.Boxes = map[cdp.BackendNodeID]ir.Box{}
		}
		for id, b := range fc.Boxes {
			if id < ir.CarriedFrom {
				id += fc.Base
			}
			b.X, b.Y = b.X+at.X, b.Y+at.Y
			c.Boxes[id] = b
		}
	}
	for _, want := range frames {
		for owner, fr := range owners {
			if fr.id != want {
				continue
			}
			fnodes, err := accessibility.GetFullAXTree().WithFrameID(fr.id).Do(ctx)
			if err != nil {
				continue
			}
			fc := captureDoc(fnodes, docs[fr.doc], strs)
			attachFrames(ctx, &fc, docs, strs, int(fr.doc), frames)
			keep(owner, &fc)
		}
		for owner, fid := range remote {
			if fid != want {
				continue
			}
			fc, err := captureRemote(ctx, fid, frames)
			if err != nil {
				// It answered nothing: the row stays shut, and says so.
				continue
			}
			keep(owner, fc)
		}
	}
}

// captureRemote reads a frame from another site through the tab's session
// on it (Sessions): its AX tree and snapshot, the way the page's own are
// read, with its ids carried at the session's offset. A session that
// answers nothing is dropped, for the next capture to attach afresh.
func captureRemote(ctx context.Context, id cdp.FrameID, frames []cdp.FrameID) (*ir.Capture, error) {
	s := sessionsFrom(ctx)
	if s == nil {
		return nil, errors.New("no sessions for another site's frames")
	}
	fctx, base, err := s.frame(id)
	if err != nil {
		return nil, err
	}
	fctx, cancel := context.WithTimeout(fctx, 10*time.Second)
	defer cancel()
	var fc ir.Capture
	err = run(fctx, func(ctx context.Context) error {
		nodes, err := accessibility.GetFullAXTree().Do(ctx)
		if err != nil {
			return err
		}
		docs, strs, err := domsnapshot.CaptureSnapshot([]string{"display"}).Do(ctx)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return errors.New("snapshot: no document")
		}
		fc = captureDoc(nodes, docs[0], strs)
		fc.Base = base
		attachFrames(ctx, &fc, docs, strs, 0, frames)
		return nil
	})
	if err != nil {
		s.forget(id)
		return nil, err
	}
	return &fc, nil
}

// captureDoc is everything the translator needs about one document: its
// AX tree, and what the DOM says that the tree does not (ir.Build).
func captureDoc(nodes []*accessibility.Node, doc *domsnapshot.DocumentSnapshot, strs []string) ir.Capture {
	boxes, hidden := layoutMaps(doc)
	c := ir.Capture{Nodes: nodes, Display: displayMap(doc, strs), Boxes: boxes, Hidden: hidden,
		// What the AX tree does not say, the DOM does (ir.Build).
		Protected: attrMarks(doc, strs, func(tag, name, value string) bool {
			return tag == "input" && name == "type" && value == "password"
		}),
		Current: attrMarks(doc, strs, func(_, name, value string) bool {
			return name == "aria-current" && value != "" && value != "false"
		}),
		Breadcrumb: attrMarks(doc, strs, func(_, name, value string) bool {
			switch name {
			case "class", "aria-label":
				return strings.Contains(value, "breadcrumb")
			case "itemtype":
				return strings.Contains(value, "breadcrumblist")
			}
			return false
		}),
		Skip: attrMarks(doc, strs, func(_, name, value string) bool {
			return (name == "class" || name == "id" || name == "aria-label") && strings.Contains(value, "skip")
		}),
		// What a field takes, as the page declared it: the input popup
		// says "email" over an email box rather than "value", which is
		// the one thing the box itself cannot show (user, 2026-09-23).
		Types: attrValues(doc, strs, func(tag, name string) bool {
			return tag == "input" && name == "type"
		}),
		// A frame's address, for the row that stands for one that cannot
		// be entered: Yank media url has it.
		Srcs: attrValues(doc, strs, func(tag, name string) bool {
			return tag == "iframe" && name == "src"
		}),
	}
	c.Anchors, c.Parents = anchors(doc, strs)
	return c
}

// frame is a child document of the snapshot: its id and its index.
type frame struct {
	id  cdp.FrameID
	doc int64
}

// frameOwners is every frame in the snapshot by the element that holds
// it, whatever document that element is in: an iframe inside an iframe
// is owned inside the inner document.
func frameOwners(docs []*domsnapshot.DocumentSnapshot, strs []string, cur int) map[cdp.BackendNodeID]frame {
	out := map[cdp.BackendNodeID]frame{}
	str := func(i int64) string {
		if i < 0 || int(i) >= len(strs) {
			return ""
		}
		return strs[i]
	}
	// Only the owners in document cur: the snapshot holds every document
	// of the process, and a frame's own frames are the ones its document
	// holds — the whole table read from inside a frame found the frame's
	// own owner, and captured the frame inside itself without end.
	if cur < 0 || cur >= len(docs) {
		return out
	}
	doc := docs[cur]
	if doc.Nodes == nil || doc.Nodes.ContentDocumentIndex == nil {
		return out
	}
	cdi := doc.Nodes.ContentDocumentIndex
	for j, ni := range cdi.Index {
		if j >= len(cdi.Value) || int(ni) >= len(doc.Nodes.BackendNodeID) {
			continue
		}
		di := cdi.Value[j]
		if di < 0 || int(di) >= len(docs) {
			continue
		}
		out[doc.Nodes.BackendNodeID[ni]] = frame{id: cdp.FrameID(str(int64(docs[di].FrameID))), doc: di}
	}
	return out
}

// hiddenMap is every element the page laid out at a point: a box of 1×1
// or less. That is the sr-only pattern — position:absolute, width:1px,
// height:1px, clip — which a page uses to write for a screen reader
// alone: "Show subtasks for AIS-2442 …", the issue key a second time,
// the summary a third. Chromium folds that text into the accessible
// names it computes, so the names keep it; only the duplicate rows go.
//
// Both sides have to be small. A rule is one pixel tall and the width of
// the page, and it is not hidden — it is a rule.
//
// A node with no box at all is not hidden, it is unmeasured: the layout
// tree only lists what was laid out, and absent data is not evidence.
func layoutMaps(doc *domsnapshot.DocumentSnapshot) (map[cdp.BackendNodeID]ir.Box, map[cdp.BackendNodeID]bool) {
	boxes, hidden := map[cdp.BackendNodeID]ir.Box{}, map[cdp.BackendNodeID]bool{}
	if doc == nil || doc.Nodes == nil || doc.Layout == nil {
		return boxes, hidden
	}
	ids := doc.Nodes.BackendNodeID
	lay := doc.Layout
	for j, ni := range lay.NodeIndex {
		if ni < 0 || int(ni) >= len(ids) || j >= len(lay.Bounds) {
			continue
		}
		// Rectangle is [x, y, width, height].
		b := lay.Bounds[j]
		if len(b) != 4 {
			continue
		}
		if b[2] <= 1 && b[3] <= 1 {
			hidden[ids[ni]] = true
			continue
		}
		boxes[ids[ni]] = ir.Box{X: b[0], Y: b[1], W: b[2], H: b[3]}
	}
	return boxes, hidden
}

// attrMarks reads the snapshot's node table for the elements pick says
// yes to — by tag and one attribute, all lower-cased — as a set of
// backend node ids. The AX tree carries no attributes: which input is a
// password, which link is aria-current, which list is a breadcrumb, all
// come from here (ir.Build).
func attrMarks(doc *domsnapshot.DocumentSnapshot, strs []string, pick func(tag, name, value string) bool) map[cdp.BackendNodeID]bool {
	out := map[cdp.BackendNodeID]bool{}
	if doc == nil || doc.Nodes == nil {
		return out
	}
	nodes := doc.Nodes
	str := func(i int64) string {
		if i < 0 || int(i) >= len(strs) {
			return ""
		}
		return strings.ToLower(strs[i])
	}
	for i, name := range nodes.NodeName {
		if i >= len(nodes.Attributes) || i >= len(nodes.BackendNodeID) {
			continue
		}
		tag := str(int64(name))
		attrs := nodes.Attributes[i]
		for j := 0; j+1 < len(attrs); j += 2 {
			if pick(tag, str(attrs[j]), str(attrs[j+1])) {
				out[nodes.BackendNodeID[i]] = true
				break
			}
		}
	}
	return out
}

// attrValues is attrMarks for an attribute's VALUE rather than its
// presence: what a field's type attribute says it takes.
func attrValues(doc *domsnapshot.DocumentSnapshot, strs []string, pick func(tag, name string) bool) map[cdp.BackendNodeID]string {
	out := map[cdp.BackendNodeID]string{}
	if doc == nil || doc.Nodes == nil {
		return out
	}
	nodes := doc.Nodes
	str := func(i int64) string {
		if i < 0 || int(i) >= len(strs) {
			return ""
		}
		return strings.ToLower(strs[i])
	}
	for i, name := range nodes.NodeName {
		if i >= len(nodes.Attributes) || i >= len(nodes.BackendNodeID) {
			continue
		}
		tag := str(int64(name))
		attrs := nodes.Attributes[i]
		for j := 0; j+1 < len(attrs); j += 2 {
			if pick(tag, str(attrs[j])) {
				out[nodes.BackendNodeID[i]] = str(attrs[j+1])
				break
			}
		}
	}
	return out
}

// anchors reads two things off the snapshot's node table that a link
// into the page needs (ui tab.jumpToAnchor): every element's id, as the
// page wrote it, and every node's parent — so the item nearest under
// the element a fragment names can be found from the outside.
func anchors(doc *domsnapshot.DocumentSnapshot, strs []string) (map[string]cdp.BackendNodeID, map[cdp.BackendNodeID]cdp.BackendNodeID) {
	ids := map[string]cdp.BackendNodeID{}
	parents := map[cdp.BackendNodeID]cdp.BackendNodeID{}
	if doc == nil || doc.Nodes == nil {
		return ids, parents
	}
	nodes := doc.Nodes
	str := func(i int64) string {
		if i < 0 || int(i) >= len(strs) {
			return ""
		}
		return strs[i]
	}
	for i, back := range nodes.BackendNodeID {
		if i < len(nodes.ParentIndex) {
			if p := nodes.ParentIndex[i]; p >= 0 && int(p) < len(nodes.BackendNodeID) {
				parents[back] = nodes.BackendNodeID[p]
			}
		}
		if i >= len(nodes.Attributes) {
			continue
		}
		attrs := nodes.Attributes[i]
		for j := 0; j+1 < len(attrs); j += 2 {
			if str(attrs[j]) == "id" {
				if id := str(attrs[j+1]); id != "" {
					ids[id] = back
				}
			}
		}
	}
	return ids, parents
}

// displayMap joins the snapshot's node table to its layout table: layout row
// j belongs to node NodeIndex[j], whose backendNodeId is the key webu's IR
// already uses. Only the main document — an iframe's nodes are not in the
// main AX tree either (v1).
func displayMap(doc *domsnapshot.DocumentSnapshot, strs []string) map[cdp.BackendNodeID]string {
	out := map[cdp.BackendNodeID]string{}
	if doc == nil || doc.Nodes == nil || doc.Layout == nil {
		return out
	}
	ids := doc.Nodes.BackendNodeID
	lay := doc.Layout
	for j, ni := range lay.NodeIndex {
		if ni < 0 || int(ni) >= len(ids) || j >= len(lay.Styles) || len(lay.Styles[j]) == 0 {
			continue
		}
		si := lay.Styles[j][0]
		if si < 0 || int(si) >= len(strs) {
			continue
		}
		out[ids[ni]] = strs[si]
	}
	return out
}
