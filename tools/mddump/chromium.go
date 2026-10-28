package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/domsnapshot"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// rendered loads the page in Chromium, lets its JavaScript run for settle,
// and rebuilds it as an HTML tree from the DOM snapshot, the page itself
// untouched. Each element carries what Chromium knows and HTML does not:
//
//	data-webu-node    the backend node id, what a click or a field goes to
//	data-webu-role    the role in Chromium's accessibility tree (LayoutTable…)
//	data-webu-box     x,y,w,h where it was laid out
//	data-webu-hidden  laid out nowhere, nor anything inside it
//
// Text that was not laid out is left out: it is on no screen.
func rendered(page, exec string, settle time.Duration) (*html.Node, *url.URL, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserAgent(userAgent))
	if exec != "" {
		opts = append(opts, chromedp.ExecPath(exec))
	}
	actx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(actx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var (
		docs  []*domsnapshot.DocumentSnapshot
		strs  []string
		nodes []*accessibility.Node
		loc   string
	)
	err := chromedp.Run(ctx,
		chromedp.Navigate(page),
		chromedp.WaitReady("body"),
		chromedp.Sleep(settle),
		chromedp.Location(&loc),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			docs, strs, err = domsnapshot.CaptureSnapshot([]string{"display", "visibility"}).Do(ctx)
			if err != nil {
				return err
			}
			nodes, err = accessibility.GetFullAXTree().Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, nil, err
	}
	if len(docs) == 0 || docs[0].Nodes == nil {
		return nil, nil, fmt.Errorf("snapshot: no document")
	}
	final, err := url.Parse(loc)
	if err != nil {
		return nil, nil, err
	}
	return rebuild(docs[0], strs, roles(nodes)), final, nil
}

// roles is each node's role in the accessibility tree, by backend id.
func roles(nodes []*accessibility.Node) map[cdp.BackendNodeID]string {
	out := map[cdp.BackendNodeID]string{}
	for _, n := range nodes {
		if n.Ignored || n.BackendDOMNodeID == 0 || n.Role == nil {
			continue
		}
		var r string
		if json.Unmarshal(n.Role.Value, &r) == nil && r != "" && r != "generic" && r != "none" {
			out[n.BackendDOMNodeID] = r
		}
	}
	return out
}

// rebuild turns the snapshot's node table back into an HTML tree. The
// table is in document order, a parent before its children.
func rebuild(doc *domsnapshot.DocumentSnapshot, strs []string, role map[cdp.BackendNodeID]string) *html.Node {
	str := func(i int64) string {
		if i < 0 || int(i) >= len(strs) {
			return ""
		}
		return strs[i]
	}
	t := doc.Nodes
	n := len(t.NodeType)
	laid := make([]int, n) // the node's row in the layout table, or -1
	for i := range laid {
		laid[i] = -1
	}
	if doc.Layout != nil {
		for j, ni := range doc.Layout.NodeIndex {
			if ni >= 0 && int(ni) < n {
				laid[ni] = j
			}
		}
	}
	// shown: the node, or something inside it, was laid out. Children come
	// after their parent, so a pass from the end carries it upwards.
	shown := make([]bool, n)
	for i := n - 1; i >= 0; i-- {
		if laid[i] >= 0 {
			shown[i] = true
		}
		if p := t.ParentIndex[i]; shown[i] && p >= 0 && int(p) < n {
			shown[p] = true
		}
	}

	// The browser's own insides — an <input>'s editor, a <video>'s
	// controls — are in user-agent shadow roots: not the page's.
	ua := map[int]bool{}
	if s := t.ShadowRootType; s != nil {
		for j, ni := range s.Index {
			if j < len(s.Value) && str(int64(s.Value[j])) == "user-agent" {
				ua[int(ni)] = true
			}
		}
	}

	out := make([]*html.Node, n)
	var root *html.Node
	inHead := make([]bool, n)
	for i := 0; i < n; i++ {
		p := int(t.ParentIndex[i])
		if p >= 0 && p < n {
			inHead[i] = inHead[p]
		}
		// Left out, and with it all it holds (its children find no parent).
		if ua[i] || p >= 0 && p < n && out[p] != nil && out[p].Type == html.ElementNode && void[out[p].Data] {
			continue
		}
		var node *html.Node
		switch t.NodeType[i] {
		case 9: // document
			node = &html.Node{Type: html.DocumentNode}
		case 1: // element
			name := strings.ToLower(str(int64(t.NodeName[i])))
			if name == "head" {
				inHead[i] = true
			}
			node = &html.Node{Type: html.ElementNode, Data: name, DataAtom: atom.Lookup([]byte(name))}
			if i < len(t.Attributes) {
				a := t.Attributes[i]
				for k := 0; k+1 < len(a); k += 2 {
					node.Attr = append(node.Attr, html.Attribute{Key: strings.ToLower(str(int64(a[k]))), Val: str(int64(a[k+1]))})
				}
			}
			id := t.BackendNodeID[i]
			mark := func(k, v string) { node.Attr = append(node.Attr, html.Attribute{Key: k, Val: v}) }
			mark("data-webu-node", strconv.FormatInt(int64(id), 10))
			if r := role[id]; r != "" {
				mark("data-webu-role", r)
			}
			if j := laid[i]; j >= 0 && doc.Layout != nil && j < len(doc.Layout.Bounds) && len(doc.Layout.Bounds[j]) == 4 {
				b := doc.Layout.Bounds[j]
				mark("data-webu-box", fmt.Sprintf("%.0f,%.0f,%.0f,%.0f", b[0], b[1], b[2], b[3]))
				if j < len(doc.Layout.Styles) && len(doc.Layout.Styles[j]) > 1 && str(int64(doc.Layout.Styles[j][1])) == "hidden" {
					mark("data-webu-hidden", "visibility")
				}
			}
			if !shown[i] && !inHead[i] && name != "html" {
				mark("data-webu-hidden", "unlaid")
			}
		case 3: // text
			if !inHead[i] && laid[i] < 0 {
				continue // on no screen
			}
			node = &html.Node{Type: html.TextNode, Data: str(int64(t.NodeValue[i]))}
		default: // doctype, comments, …
			continue
		}
		out[i] = node
		if p >= 0 && p < n && out[p] != nil {
			out[p].AppendChild(node)
		} else if root == nil {
			root = node
		}
	}
	if root == nil || root.Type != html.DocumentNode {
		d := &html.Node{Type: html.DocumentNode}
		if root != nil {
			d.AppendChild(root)
		}
		root = d
	}
	return root
}

// apply does what the marks say before the content is picked: what was
// laid out nowhere goes, a table Chromium calls a layout table becomes
// plain blocks, and the roles Readability already leaves out are written
// where only Chromium knew them.
func apply(doc *html.Node) {
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, layout bool) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type != html.ElementNode {
				c = next
				continue
			}
			if attr(c, "data-webu-hidden") != "" {
				n.RemoveChild(c)
				c = next
				continue
			}
			role := attr(c, "data-webu-role")
			inLayout := layout
			switch c.Data {
			case "table":
				inLayout = role == "LayoutTable"
			}
			if inLayout {
				switch c.Data {
				case "table", "thead", "tbody", "tfoot", "tr", "td", "th", "caption":
					c.Data, c.DataAtom = "div", atom.Div
				}
			}
			switch role {
			case "navigation", "complementary", "dialog", "alertdialog", "menu", "menubar":
				if attr(c, "role") == "" {
					c.Attr = append(c.Attr, html.Attribute{Key: "role", Val: role})
				}
			}
			walk(c, inLayout)
			c = next
		}
	}
	walk(doc, false)
}

// void are the elements HTML gives no children.
var void = map[string]bool{"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
