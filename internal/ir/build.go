package ir

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
)

// Capture is everything the translator reads from one page: the AX tree,
// and the computed display of every DOM node that has a layout box, keyed
// by the backendDOMNodeId both tables share. Display may be nil — the tree
// still builds, with every generic container treated as inline.
type Capture struct {
	Nodes   []*accessibility.Node        `json:"nodes"`
	Display map[cdp.BackendNodeID]string `json:"display,omitempty"`
	// Protected marks the inputs that are type=password (page.Capture
	// reads it off the same snapshot). The AX tree does not say, and an
	// empty password box is not told apart any other way (2026-09-21).
	Protected map[cdp.BackendNodeID]bool `json:"protected,omitempty"`
	// Current marks the elements with aria-current (a link, or its list
	// item): where the user is in a navigation or a breadcrumb.
	Current map[cdp.BackendNodeID]bool `json:"current,omitempty"`
	// Breadcrumb marks the elements the page calls a breadcrumb — by
	// class, aria-label or schema.org itemtype — nav, list or div.
	Breadcrumb map[cdp.BackendNodeID]bool `json:"breadcrumb,omitempty"`
	// Skip marks what the page calls a skip link by class, id or label —
	// the "Skip to main content" at the top, or a block of them ("Skip
	// to: Top Bar · Sidebar · Main Content"); a link's text says it too
	// (ui). A block is wrapped in a navigation of its own at build time.
	Skip map[cdp.BackendNodeID]bool `json:"skip,omitempty"`
	// Types is what an <input> declared it takes — "email", "date",
	// "number" — by backend id. The AX tree folds all of them into
	// textbox, and a terminal has to say which, because a box on screen
	// shows nothing of what it expects.
	Types map[cdp.BackendNodeID]string `json:"types,omitempty"`
	// Viewport is the window the page was laid out in — not the page's
	// own size, which Boxes gives. A page shorter than its viewport is
	// whole: nothing is off screen for the chrome to be in the way of
	// (ui.splitParts, 2026-09-23). Zero when nothing measured it.
	Viewport Box `json:"viewport,omitempty"`
	// Boxes is where each element was laid out: x, y, width, height, in
	// page coordinates. It is the only thing that says a block sits
	// BESIDE another rather than under it, and where a page's top and
	// bottom bands are — which is how webu tells a page's parts apart
	// without reading a single tag attribute (ui.splitParts).
	Boxes map[cdp.BackendNodeID]Box `json:"boxes,omitempty"`
	// Hidden marks the elements laid out at a point — a box of 1×1 or
	// less. That is how a page writes text for a screen reader and no
	// one else: position:absolute, width:1px, height:1px, clipped. A
	// reader meets it one line at a time and needs the repetition for
	// context; webu puts the whole card on screen at once, where the
	// same text three times is noise. Chromium has already folded it
	// into the accessible names, so dropping the nodes loses nothing
	// (page.Capture reads the bounds off the snapshot we already take).
	Hidden map[cdp.BackendNodeID]bool `json:"hidden,omitempty"`
	// Anchors is every element id on the page and Parents every node's
	// parent, off the same snapshot: what a link into the page lands on
	// (ui tab.jumpToAnchor). Not part of the tree.
	Anchors map[string]cdp.BackendNodeID            `json:"anchors,omitempty"`
	Parents map[cdp.BackendNodeID]cdp.BackendNodeID `json:"parents,omitempty"`
	// ContentType is document.contentType: what the response was. Empty
	// in the role fixtures, which are all HTML.
	ContentType string `json:"contentType,omitempty"`
}

// Box is a laid-out rectangle in page coordinates.
type Box struct {
	X, Y, W, H float64
}

// Area is how much of the page it covers.
func (b Box) Area() float64 { return b.W * b.H }

// Union is the smallest box holding both; a zero box contributes nothing,
// so a part's box can be built up from the descendants that have one.
func (b Box) Union(o Box) Box {
	if b.W <= 0 || b.H <= 0 {
		return o
	}
	if o.W <= 0 || o.H <= 0 {
		return b
	}
	x, y := min(b.X, o.X), min(b.Y, o.Y)
	return Box{X: x, Y: y,
		W: max(b.X+b.W, o.X+o.W) - x,
		H: max(b.Y+b.H, o.Y+o.H) - y}
}

// textDocument says whether a content type is a document Chromium shows
// as text in a <pre> (with, for JSON, a viewer of its own around it) —
// which webu draws as one code block instead (revision 2026-09-20).
func textDocument(ct string) bool {
	ct = strings.ToLower(ct)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch {
	case ct == "application/json", strings.HasSuffix(ct, "+json"),
		ct == "text/plain", ct == "text/csv", ct == "text/markdown",
		strings.Contains(ct, "yaml"), strings.Contains(ct, "toml"),
		ct == "application/xml", ct == "text/xml", strings.HasSuffix(ct, "+xml"),
		ct == "application/javascript", ct == "text/javascript", ct == "text/css":
		return true
	}
	return false
}

// langOf is the name a lexer knows the content type by; "" for plain.
func langOf(ct string) string {
	ct = strings.ToLower(ct)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch {
	case ct == "application/json", strings.HasSuffix(ct, "+json"):
		return "json"
	case strings.Contains(ct, "yaml"):
		return "yaml"
	case strings.Contains(ct, "toml"):
		return "toml"
	case ct == "text/markdown":
		return "markdown"
	case ct == "application/xml", ct == "text/xml", strings.HasSuffix(ct, "+xml"):
		return "xml"
	case strings.Contains(ct, "javascript"):
		return "javascript"
	case ct == "text/css":
		return "css"
	}
	return ""
}

// asText reduces a text document to Document{Code}: the longest text run
// in the tree is the body (Chrome's JSON viewer adds a form and a few
// labels around it), pretty-printed when it is JSON.
func asText(root *Node, ct string) *Node {
	var body string
	root.Walk(func(n *Node) bool {
		if n.Kind == Text && len(n.Name) > len(body) {
			body = n.Name
		}
		return true
	})
	if strings.TrimSpace(body) == "" {
		return root
	}
	if strings.Contains(strings.ToLower(ct), "json") {
		var buf bytes.Buffer
		if json.Indent(&buf, []byte(strings.TrimSpace(body)), "", "  ") == nil {
			body = buf.String()
		}
	}
	return &Node{Kind: Document, Role: "RootWebArea", Name: root.Name, URL: root.URL,
		Children: []*Node{{Kind: Code, Role: "code", Lang: langOf(ct),
			Children: []*Node{{Kind: Text, Role: "StaticText", Name: body}}}}}
}

// Build turns a capture into an IR tree.
//
// The AX list is flat, parent-to-child by ID, root first. Nodes Chromium
// marks ignored are containers it would not read out either; they vanish and
// their children take their place, the same as a generic. Nothing is hidden
// on purpose (function.md §3): a role the table does not know still comes
// through, as Unsupported, with its name and its children.
func Build(c Capture) *Node {
	if len(c.Nodes) == 0 {
		return &Node{Kind: Document, Role: "RootWebArea"}
	}
	b := builder{
		byID:       make(map[accessibility.NodeID]*accessibility.Node, len(c.Nodes)),
		display:    c.Display,
		protected:  c.Protected,
		types:      c.Types,
		current:    c.Current,
		hidden:     c.Hidden,
		skip:       maps.Clone(c.Skip),
		breadcrumb: maps.Clone(c.Breadcrumb),
	}
	for _, n := range c.Nodes {
		b.byID[n.NodeID] = n
	}
	out := b.convert(c.Nodes[0])
	root := &Node{Kind: Document, Role: "RootWebArea", Children: out}
	if len(out) == 1 && out[0].Kind == Document {
		root = out[0]
	}
	if textDocument(c.ContentType) {
		return asText(root, c.ContentType)
	}
	return root
}

type builder struct {
	byID       map[accessibility.NodeID]*accessibility.Node
	display    map[cdp.BackendNodeID]string
	protected  map[cdp.BackendNodeID]bool
	types      map[cdp.BackendNodeID]string
	current    map[cdp.BackendNodeID]bool
	hidden     map[cdp.BackendNodeID]bool
	skip       map[cdp.BackendNodeID]bool
	breadcrumb map[cdp.BackendNodeID]bool // consumed as trails are wrapped
	navDepth   int                        // navigation landmarks being entered
}

func (b *builder) blockBox(id cdp.BackendNodeID) bool {
	return IsBlockDisplay(b.display[id])
}

func (b *builder) children(ax *accessibility.Node) []*Node {
	var out []*Node
	for _, id := range ax.ChildIDs {
		c := b.byID[id]
		if c == nil {
			continue
		}
		out = append(out, b.convert(c)...)
	}
	return out
}

// container is a transparent role's conversion: normally its children in
// its place, but a container the page laid out as a BLOCK keeps a Group
// around them when anything inline is inside — that box's line break is
// the only thing separating this text from the neighbour's. A block of
// nothing but blocks needs no wrapper: its children break lines themselves.
func (b *builder) container(ax *accessibility.Node, role string) []*Node {
	kids := b.children(ax)
	if len(kids) == 0 || !b.blockBox(ax.BackendDOMNodeID) {
		return kids
	}
	inline := false
	for _, k := range kids {
		if !k.IsBlock() && !(k.Kind == Text && strings.TrimSpace(k.Name) == "") {
			inline = true
			break
		}
	}
	if !inline {
		return kids
	}
	return []*Node{{Kind: Group, Role: role, ID: ax.BackendDOMNodeID, Children: kids}}
}

// convert yields the IR for one AX node: one node, several (a transparent
// container's children), or none.
func (b *builder) convert(ax *accessibility.Node) []*Node {
	if b.skip[ax.BackendDOMNodeID] && str(ax.Role) != "link" {
		// A block of skip links — "Skip to:" and a list of anchors —
		// becomes a navigation of its own, so it is one row like the
		// rest of the chrome (2026-09-21). Looked at before the ignored
		// check: the block is a plain div, which Chromium ignores. The
		// mark is spent, or this would recurse; a block with no anchor
		// in it was not one.
		delete(b.skip, ax.BackendDOMNodeID)
		kids := b.convert(ax)
		if !anchorLinks(kids) {
			return kids
		}
		return []*Node{{Kind: Landmark, Role: "navigation", Name: skipName(kids), Skip: true,
			ID: ax.BackendDOMNodeID, Children: kids}}
	}
	if b.hidden[ax.BackendDOMNodeID] {
		// Laid out at a point: written for a screen reader, not for a
		// reader. Its subtree goes with it — the whole run is the
		// hidden text.
		return nil
	}
	if ax.Ignored {
		return b.children(ax)
	}
	role := str(ax.Role)
	spec := Lookup(role)
	// A contenteditable element has no role of its own — Chromium leaves it
	// generic, focusable, with editable="richtext" — and it is the whole
	// input surface of a Slack or a Gmail. It is a textarea for webu's
	// purposes: a multi-line box the user types into.
	editableDiv := spec.Transparent && isEditableGeneric(ax)
	if editableDiv {
		spec = Spec{Kind: Textbox}
		role = "textbox"
	}
	switch {
	case spec.Skip:
		return nil
	case b.breadcrumb[ax.BackendDOMNodeID] && b.navDepth == 0 && role != "navigation":
		// A breadcrumb the page marked on a list or a div outside any
		// navigation becomes one, so the UI has one shape for a trail
		// (2026-09-21). The mark is spent, or this would recurse.
		delete(b.breadcrumb, ax.BackendDOMNodeID)
		b.navDepth++
		kids := b.convert(ax)
		b.navDepth--
		return []*Node{{Kind: Landmark, Role: "navigation", Name: "breadcrumb", Breadcrumb: true,
			ID: ax.BackendDOMNodeID, Children: kids}}
	case spec.Transparent:
		return b.container(ax, role)
	}

	n := &Node{
		Kind:    spec.Kind,
		Role:    role,
		Name:    str(ax.Name),
		Value:   str(ax.Value),
		ID:      ax.BackendDOMNodeID,
		Current: b.current[ax.BackendDOMNodeID],
		Skip:    b.skip[ax.BackendDOMNodeID],
	}
	for _, p := range ax.Properties {
		switch p.Name {
		case accessibility.PropertyNameURL:
			n.URL = str(p.Value)
		case accessibility.PropertyNameLevel:
			n.Level = num(p.Value)
		case accessibility.PropertyNameChecked:
			switch str(p.Value) {
			case "true":
				n.Checked = On
			case "mixed":
				n.Checked = Mixed
			}
		case accessibility.PropertyNameMultiline:
			n.Multiline = boolean(p.Value)
		case accessibility.PropertyNameDisabled:
			n.Disabled = boolean(p.Value)
		case accessibility.PropertyNameRequired:
			n.Required = boolean(p.Value)
		case accessibility.PropertyNameFocusable:
			n.Focusable = boolean(p.Value)
		case accessibility.PropertyNameExpanded:
			n.Expanded = boolean(p.Value)
		case accessibility.PropertyNameSelected:
			n.Selected = boolean(p.Value)
		case accessibility.PropertyNameValuetext:
			// A spinbutton reports its number here rather than as a value.
			if n.Value == "" {
				n.Value = str(p.Value)
			}
		}
	}
	// An inline kind the page put on a line of its own keeps that line.
	if !n.IsBlock() && n.Kind != Text && b.blockBox(n.ID) {
		n.Block = true
	}

	switch n.Kind {
	case Text:
		if role == "LineBreak" {
			n.Name = "\n"
		}
		return []*Node{n}
	case Link, Button, Media, Check, Textbox:
		// Leaves: the accessible name is the reading, and whatever is inside
		// (an image in a link, the text of a button, the editable div under
		// a textbox) has already been folded into it by Chromium.
		n.Name = strings.TrimSpace(n.Name)
		if n.Kind == Textbox {
			if editableDiv {
				// contenteditable: the text is the children, not a value.
				n.Multiline = true
				n.Value = strings.TrimSpace(textOf(b.children(ax)))
			}
			n.Value = strings.ReplaceAll(n.Value, "\r\n", "\n")
			// A password field: the DOM says so (Capture.Protected). The
			// dots Chromium masks the value with are the fallback, for a
			// capture that did not look at the DOM — the AX node itself
			// does not say.
			n.Protected = b.protected[n.ID] || (n.Value != "" && strings.Trim(n.Value, "•") == "")
			// What the page said the box takes. The AX tree folds email,
			// tel, url, date and the rest into one textbox, and a box on
			// screen shows nothing of what it expects, so the popup over
			// it has to say (ui editFieldAs).
			n.InputType = b.types[n.ID]
		}
		return []*Node{n}
	case Combobox:
		n.Name = strings.TrimSpace(n.Name)
		n.Children = b.options(ax)
		return []*Node{n}
	case Option:
		return []*Node{n}
	case Cell:
		n.Header = role == "columnheader" || role == "rowheader"
	case ListItem:
		n.Level = 0 // Chromium's nesting level; the tree already says it
	case Landmark, Group, Quote, Unsupported:
		n.Value = ""
	}

	nav := n.Kind == Landmark && role == "navigation"
	if nav {
		// A trail, by its name or by the DOM's word for it.
		n.Breadcrumb = b.breadcrumb[n.ID] || strings.Contains(strings.ToLower(n.Name), "breadcrumb")
		b.navDepth++
	}
	n.Children = b.children(ax)
	if nav {
		b.navDepth--
	}
	if n.Kind == ListItem {
		n.Marker = marker(ax, b)
	}
	return []*Node{n}
}

// anchorLinks says whether nodes hold at least one link and every link
// in them points at an anchor: the shape of a block of skip links.
func anchorLinks(nodes []*Node) bool {
	links, anchors := 0, 0
	for _, n := range nodes {
		n.Walk(func(x *Node) bool {
			if x.Kind == Link {
				links++
				if strings.Contains(x.URL, "#") {
					anchors++
				}
			}
			return true
		})
	}
	return links > 0 && links == anchors
}

// skipName is a skip block's word: its own text where it starts with
// "skip" ("Skip to:"), else "Skip to".
func skipName(nodes []*Node) string {
	for _, n := range nodes {
		first := ""
		n.Walk(func(x *Node) bool {
			if first == "" && x.Kind == Text && strings.TrimSpace(x.Name) != "" {
				first = strings.TrimSpace(x.Name)
			}
			return first == ""
		})
		if first != "" {
			if strings.HasPrefix(strings.ToLower(first), "skip") {
				return strings.TrimRight(first, ": ")
			}
			break
		}
	}
	return "Skip to"
}

// options collects every option under a combobox, wherever Chromium nested
// it (a MenuListPopup, an ignored wrapper), so the Choose action has the
// list without walking the tree again.
func (b *builder) options(ax *accessibility.Node) []*Node {
	var out []*Node
	var walk func(*accessibility.Node)
	walk = func(a *accessibility.Node) {
		for _, id := range a.ChildIDs {
			c := b.byID[id]
			if c == nil {
				continue
			}
			if str(c.Role) == "option" {
				o := &Node{Kind: Option, Role: "option", Name: str(c.Name), ID: c.BackendDOMNodeID}
				for _, p := range c.Properties {
					if p.Name == accessibility.PropertyNameSelected {
						o.Selected = boolean(p.Value)
					}
				}
				out = append(out, o)
				continue
			}
			walk(c)
		}
	}
	walk(ax)
	return out
}

// marker is a list item's ListMarker child, read straight from the AX
// children because the table skipped it before convert could see it.
func marker(ax *accessibility.Node, b *builder) string {
	for _, id := range ax.ChildIDs {
		c := b.byID[id]
		if c != nil && str(c.Role) == "ListMarker" {
			return str(c.Name)
		}
	}
	return ""
}

func isEditableGeneric(ax *accessibility.Node) bool {
	focusable, editable := false, false
	for _, p := range ax.Properties {
		switch p.Name {
		case accessibility.PropertyNameFocusable:
			focusable = boolean(p.Value)
		case accessibility.PropertyNameEditable:
			editable = str(p.Value) != ""
		}
	}
	return focusable && editable
}

func textOf(nodes []*Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.Text())
	}
	return b.String()
}

// str reads an AX value as a string: strings come back unquoted, anything
// else as its JSON.
func str(v *accessibility.Value) string {
	if v == nil || v.Value == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err == nil {
		return s
	}
	return string(v.Value)
}

func num(v *accessibility.Value) int {
	if v == nil || v.Value == nil {
		return 0
	}
	var f float64
	if err := json.Unmarshal(v.Value, &f); err != nil {
		return 0
	}
	return int(f)
}

func boolean(v *accessibility.Value) bool {
	if v == nil || v.Value == nil {
		return false
	}
	var b bool
	if err := json.Unmarshal(v.Value, &b); err == nil {
		return b
	}
	return string(v.Value) == `"true"`
}
