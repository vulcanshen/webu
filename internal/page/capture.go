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
		c = ir.Capture{Nodes: nodes, Display: displayMap(docs, strs), Protected: passwordFields(docs, strs)}
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

// passwordFields reads which inputs are type=password off the snapshot's
// node table. The AX tree does not say: Chromium masks the value itself,
// so a filled password box shows as dots, and an empty one looks like any
// other textbox (ir.Build).
func passwordFields(docs []*domsnapshot.DocumentSnapshot, strs []string) map[cdp.BackendNodeID]bool {
	out := map[cdp.BackendNodeID]bool{}
	if len(docs) == 0 || docs[0].Nodes == nil {
		return out
	}
	nodes := docs[0].Nodes
	str := func(i int64) string {
		if i < 0 || int(i) >= len(strs) {
			return ""
		}
		return strs[i]
	}
	for i, name := range nodes.NodeName {
		if !strings.EqualFold(str(int64(name)), "input") || i >= len(nodes.Attributes) || i >= len(nodes.BackendNodeID) {
			continue
		}
		attrs := nodes.Attributes[i]
		for j := 0; j+1 < len(attrs); j += 2 {
			if strings.EqualFold(str(attrs[j]), "type") && strings.EqualFold(str(attrs[j+1]), "password") {
				out[nodes.BackendNodeID[i]] = true
			}
		}
	}
	return out
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
