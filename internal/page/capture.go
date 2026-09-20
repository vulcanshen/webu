// Package page is the CDP side of the translation layer: what webu asks a
// tab for (its accessibility tree, its layout) and what it tells the tab to
// do (click this node, type into that one). internal/ir turns the answers
// into a tree; internal/ui draws it. Nothing here knows about terminals.
package page

import (
	"context"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/domsnapshot"
	"github.com/vulcanshen/webu/internal/ir"
)

// Capture reads everything the translator needs from a tab: the full AX
// tree, and the computed `display` of every DOM node that has a layout box.
//
// The second is what tells a div from a span. The AX tree deliberately
// carries no layout, and a generic container looks the same whether it is a
// block or an inline; without this, two adjacent <div>s of text would run
// together on one line (ir.Build). DOMSnapshot is one call for the whole
// page, which is what makes it affordable on every redraw.
func Capture(ctx context.Context) (ir.Capture, error) {
	nodes, err := accessibility.GetFullAXTree().Do(ctx)
	if err != nil {
		return ir.Capture{}, err
	}
	docs, strs, err := domsnapshot.CaptureSnapshot([]string{"display"}).Do(ctx)
	if err != nil {
		return ir.Capture{}, err
	}
	return ir.Capture{Nodes: nodes, Display: displayMap(docs, strs)}, nil
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
