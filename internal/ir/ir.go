// Package ir is webu's intermediate representation: the accessibility tree
// Chromium keeps for screen readers, reduced to the twenty-odd kinds a
// terminal can draw, with every interactive node still carrying the
// backendDOMNodeId that lets an action be sent back (function.md §3).
//
// There is no layout in here — no widths, colours or coordinates. Rendering
// is the UI's job; this package answers "what is on the page and what can be
// done to it". The source is the AX tree today; anything else that produces
// the same Node tree would draw the same.
package ir

import (
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/cdp"
)

// Kind is what a node IS for the purpose of drawing and acting on it. One AX
// role maps to one Kind (roles.go); several roles may share a Kind, and the
// Role field keeps the original spelling for the Inspect popup and the
// unsupported glyph.
type Kind uint8

const (
	Document    Kind = iota // RootWebArea — the page
	Landmark                // banner / navigation / main / … — a named region, enters the outline
	Heading                 // heading, with Level
	Paragraph               // a block of inline flow
	Text                    // one run of text inside a flow
	Link                    // Enter opens it; URL is where
	Button                  // Enter presses it
	Textbox                 // textbox / searchbox / textarea (Multiline)
	Check                   // checkbox / radio / switch; Checked is the state
	Combobox                // <select>; Children are its Options
	Option                  // one <option>, only ever under a Combobox
	List                    // list; Children are ListItems
	ListItem                // one bullet; Marker is what Chromium drew for it
	Table                   // table; Children are Rows
	Row                     // one row; Children are Cells
	Cell                    // cell / columnheader / rowheader; Header says which
	Media                   // image / video / audio / canvas / iframe — a placeholder box
	Separator               // <hr>
	Code                    // code, inline or block — the renderer looks at the text
	Quote                   // blockquote
	Span                    // an inline run drawn with a text attribute; Role says which
	Group                   // a container kept only for its line break (a layout-table row, a block div)
	Unsupported             // a role the whitelist does not cover: name as text, children kept, glyph on
)

var kindNames = [...]string{
	"document", "landmark", "heading", "paragraph", "text", "link", "button",
	"textbox", "check", "combobox", "option", "list", "listitem", "table", "row",
	"cell", "media", "separator", "code", "quote", "span", "group", "unsupported",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("kind(%d)", k)
}

// Tri is a checkbox's state. Mixed is aria-checked="mixed".
type Tri uint8

const (
	Off Tri = iota
	On
	Mixed
)

func (t Tri) String() string {
	switch t {
	case On:
		return "on"
	case Mixed:
		return "mixed"
	}
	return "off"
}

// Node is one thing on the page.
type Node struct {
	Kind Kind
	// Role is the AX role exactly as Chromium spelled it — "searchbox", not
	// "textbox" — for the Inspect popup and the unsupported marker.
	Role string
	// Name is the accessible name: link text, button label, the label of an
	// input, alt of an image, the heading's text. Already computed by
	// Chromium, so a label wired with aria-labelledby arrives resolved.
	Name string
	// Value is an input's current text, a combobox's chosen option.
	Value string
	// URL is a link's href or an image's src, as the AX node carries it.
	URL string
	// Level is a heading's 1–6.
	Level int
	// ID is the backendDOMNodeId: the one link back to Chromium, and what every
	// action is addressed to. Zero for nodes with no DOM node behind them.
	ID cdp.BackendNodeID

	Checked   Tri
	Multiline bool // Textbox: a textarea or a contenteditable
	Protected bool // Textbox: a password field — the value is shown masked
	// Invalid: the page marked the field's value wrong (aria-invalid, or
	// the browser's own validation). Drawn in the colour of "is wrong".
	Invalid bool
	// Focused: the page's keyboard is here (the AX tree's focused). A
	// block that appears after a press and takes the keyboard is asking
	// for an answer — how a popup is told from a chat bubble (ui popup).
	Focused bool
	// Modal: the page declared this a modal dialog (aria-modal, or
	// <dialog>.showModal()). Chromium prunes the rest of the tree behind
	// one, so it is the one popup that needs no geometry to be seen.
	Modal bool
	// Expandable: the page can open and shut this (aria-expanded is
	// there at all); Expanded is whether it is open. A tree's branch
	// against its leaf: the leaf has no aria-expanded to be false.
	Expandable bool
	// Multi: many of these can be chosen at once — a listbox declared
	// multiselectable, on it and on each of its options (Build). An
	// option of one draws as a check box; of a single-choice list, as a
	// radio button.
	Multi bool
	// Frame is the frame an <iframe> holds, by Chromium's frame id
	// (Capture.FrameOf): the one Media that is a thing to go into. Its
	// Children are that frame's document once it has been opened.
	Frame string
	// InputType is what an <input> declared it takes — "email", "date",
	// "number" — lower-cased, empty when the page said nothing or the
	// field is not an <input> (Capture.Types).
	InputType string
	// Current is aria-current: where the user is, in a navigation or a
	// breadcrumb (on the link, or on its list item). From the DOM, not
	// the AX tree (Capture.Current).
	Current bool
	// Breadcrumb marks a navigation landmark that is a breadcrumb trail:
	// named so, or so marked in the DOM (Capture.Breadcrumb). A trail
	// outside any navigation is wrapped in one at build time.
	Breadcrumb bool
	// Skip marks a link the page calls a skip link by class; a link whose
	// text starts with "skip" and points at an anchor is one regardless
	// (ui isSkipLink).
	Skip     bool
	Disabled bool
	// Required: the form will not go without this one. The AX tree says
	// so and a page usually says so in ink webu does not read, so the
	// label carries it (ui render.formField).
	Required  bool
	Focusable bool
	Expanded  bool // a combobox or disclosure that is open
	Selected  bool // an Option
	Header    bool // a Cell that is a column or row header
	// Block marks an inline kind that the page laid out as a block — a link
	// styled display:block, a button on a line of its own. Set from the
	// captured layout, never from the role.
	Block bool
	// Marker is a ListItem's bullet as Chromium rendered it: "• ", "1. ".
	Marker string
	// Lang is a Code block's language when it is known — a whole document
	// that was JSON, YAML, TOML, Markdown… — for syntax colour. Empty for
	// code found inside a page, which the AX tree does not name.
	Lang string

	Children []*Node
}

// IsBlock reports whether the node starts on a line of its own. Inline kinds
// flow together and wrap; block kinds break the flow before and after.
//
// An Unsupported node is a block when anything inside it is — a region the
// whitelist does not know is still a region — and inline otherwise.
func (n *Node) IsBlock() bool {
	if n.Block {
		return true
	}
	switch n.Kind {
	case Document, Landmark, Heading, Paragraph, List, ListItem, Table, Row,
		Separator, Quote, Group:
		return true
	case Unsupported:
		for _, c := range n.Children {
			if c.IsBlock() {
				return true
			}
		}
		return false
	}
	return false
}

// IsItem reports whether the cursor can stop here (ux.md §1): every
// interactive node, every heading, and any unsupported node the page made
// focusable — because Enter is a click and a click works on anything.
func (n *Node) IsItem() bool {
	switch n.Kind {
	case Link, Button, Textbox, Check, Combobox, Media, Heading:
		return true
	case Unsupported:
		return n.Focusable
	}
	return false
}

// Text is the node's reading: its own name for a leaf that has one, else
// its children's text run together. A link or button reads as its name — the
// accessible name is already the right reading of whatever is inside.
func (n *Node) Text() string {
	switch n.Kind {
	case Text, Link, Button, Media:
		return n.Name
	}
	if len(n.Children) == 0 {
		return n.Name
	}
	var b strings.Builder
	for _, c := range n.Children {
		if c.Kind == Option {
			continue
		}
		// A block child starts a line of its own, so a landmark's text
		// reads as its paragraphs rather than as one run-together string.
		if c.IsBlock() && b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString(c.Text())
	}
	return b.String()
}

// Walk visits n and every descendant, depth first, stopping early when fn
// returns false for a node (its children are then skipped).
func (n *Node) Walk(fn func(*Node) bool) {
	if !fn(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

// Dump is the IR as an indented outline, one node per line. It is the golden
// form every fixture is checked against, so it says everything that matters
// and nothing that does not.
func Dump(n *Node) string {
	var b strings.Builder
	dump(&b, n, 0)
	return b.String()
}

func dump(b *strings.Builder, n *Node, depth int) {
	b.WriteString(strings.Repeat("  ", depth))
	b.WriteString(n.Kind.String())
	if n.Kind == Landmark || n.Kind == Unsupported || n.Kind == Media ||
		(n.Kind == Check && n.Role != "checkbox") ||
		(n.Kind == Textbox && n.Role != "textbox") ||
		(n.Kind == Cell && n.Role != "cell") ||
		(n.Kind == Group && n.Role != "group") {
		b.WriteString(":" + n.Role)
	}
	if n.Name != "" {
		fmt.Fprintf(b, " %q", n.Name)
	}
	if n.Value != "" {
		fmt.Fprintf(b, " value=%q", n.Value)
	}
	if n.URL != "" {
		fmt.Fprintf(b, " url=%s", n.URL)
	}
	if n.Level > 0 {
		fmt.Fprintf(b, " level=%d", n.Level)
	}
	if n.Kind == Check {
		fmt.Fprintf(b, " %s", n.Checked)
	}
	if n.Marker != "" {
		fmt.Fprintf(b, " marker=%q", n.Marker)
	}
	for _, f := range []struct {
		on   bool
		name string
	}{
		{n.Multiline, "multiline"}, {n.Protected, "protected"}, {n.Current, "current"}, {n.Breadcrumb, "breadcrumb"}, {n.Skip, "skip"}, {n.Disabled, "disabled"}, {n.Required, "required"}, {n.Invalid, "invalid"},
		{n.Focusable, "focusable"}, {n.Expanded, "expanded"}, {n.Selected, "selected"},
		{n.Header, "header"}, {n.Block, "block"},
	} {
		if f.on {
			b.WriteString(" " + f.name)
		}
	}
	if n.ID != 0 {
		fmt.Fprintf(b, " #%d", n.ID)
	}
	b.WriteString("\n")
	for _, c := range n.Children {
		dump(b, c, depth+1)
	}
}
