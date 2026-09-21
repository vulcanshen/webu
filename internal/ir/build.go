package ir

import (
	"bytes"
	"encoding/json"
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
	// ContentType is document.contentType: what the response was. Empty
	// in the role fixtures, which are all HTML.
	ContentType string `json:"contentType,omitempty"`
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
		byID:      make(map[accessibility.NodeID]*accessibility.Node, len(c.Nodes)),
		display:   c.Display,
		protected: c.Protected,
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
	byID      map[accessibility.NodeID]*accessibility.Node
	display   map[cdp.BackendNodeID]string
	protected map[cdp.BackendNodeID]bool
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
	case spec.Transparent:
		return b.container(ax, role)
	}

	n := &Node{
		Kind:  spec.Kind,
		Role:  role,
		Name:  str(ax.Name),
		Value: str(ax.Value),
		ID:    ax.BackendDOMNodeID,
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

	n.Children = b.children(ax)
	if n.Kind == ListItem {
		n.Marker = marker(ax, b)
	}
	return []*Node{n}
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
