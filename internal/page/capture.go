// Package page is the CDP side of the translation layer: what webu asks a
// tab for (its accessibility tree, its layout) and what it tells the tab to
// do (click this node, type into that one). internal/ir turns the answers
// into a tree; internal/ui draws it. Nothing here knows about terminals.
package page

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
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
func Capture(ctx context.Context) (ir.Capture, error) {
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
		boxes, hidden := layoutMaps(docs)
		c = ir.Capture{Nodes: nodes, Display: displayMap(docs, strs), Boxes: boxes, Hidden: hidden,
			// What the AX tree does not say, the DOM does (ir.Build).
			Protected: attrMarks(docs, strs, func(tag, name, value string) bool {
				return tag == "input" && name == "type" && value == "password"
			}),
			Current: attrMarks(docs, strs, func(_, name, value string) bool {
				return name == "aria-current" && value != "" && value != "false"
			}),
			Breadcrumb: attrMarks(docs, strs, func(_, name, value string) bool {
				switch name {
				case "class", "aria-label":
					return strings.Contains(value, "breadcrumb")
				case "itemtype":
					return strings.Contains(value, "breadcrumblist")
				}
				return false
			}),
			Skip: attrMarks(docs, strs, func(_, name, value string) bool {
				return (name == "class" || name == "id" || name == "aria-label") && strings.Contains(value, "skip")
			}),
		}
		// What a field takes, as the page declared it: the input popup
		// says "email" over an email box rather than "value", which is
		// the one thing the box itself cannot show (user, 2026-09-23).
		c.Types = attrValues(docs, strs, func(tag, name string) bool {
			return tag == "input" && name == "type"
		})
		c.Anchors, c.Parents = anchors(docs, strs)
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
func layoutMaps(docs []*domsnapshot.DocumentSnapshot) (map[cdp.BackendNodeID]ir.Box, map[cdp.BackendNodeID]bool) {
	boxes, hidden := map[cdp.BackendNodeID]ir.Box{}, map[cdp.BackendNodeID]bool{}
	if len(docs) == 0 || docs[0].Nodes == nil || docs[0].Layout == nil {
		return boxes, hidden
	}
	ids := docs[0].Nodes.BackendNodeID
	lay := docs[0].Layout
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
func attrMarks(docs []*domsnapshot.DocumentSnapshot, strs []string, pick func(tag, name, value string) bool) map[cdp.BackendNodeID]bool {
	out := map[cdp.BackendNodeID]bool{}
	if len(docs) == 0 || docs[0].Nodes == nil {
		return out
	}
	nodes := docs[0].Nodes
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
func attrValues(docs []*domsnapshot.DocumentSnapshot, strs []string, pick func(tag, name string) bool) map[cdp.BackendNodeID]string {
	out := map[cdp.BackendNodeID]string{}
	if len(docs) == 0 || docs[0].Nodes == nil {
		return out
	}
	nodes := docs[0].Nodes
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
func anchors(docs []*domsnapshot.DocumentSnapshot, strs []string) (map[string]cdp.BackendNodeID, map[cdp.BackendNodeID]cdp.BackendNodeID) {
	ids := map[string]cdp.BackendNodeID{}
	parents := map[cdp.BackendNodeID]cdp.BackendNodeID{}
	if len(docs) == 0 || docs[0].Nodes == nil {
		return ids, parents
	}
	nodes := docs[0].Nodes
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
func displayMap(docs []*domsnapshot.DocumentSnapshot, strs []string) map[cdp.BackendNodeID]string {
	out := map[cdp.BackendNodeID]string{}
	if len(docs) == 0 || docs[0].Nodes == nil || docs[0].Layout == nil {
		return out
	}
	ids := docs[0].Nodes.BackendNodeID
	lay := docs[0].Layout
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
