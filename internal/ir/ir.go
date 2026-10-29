// Package ir is webu's content tree: what a page presents, reduced to the
// twenty-odd kinds a terminal can draw. The reader builds it from a
// page's HTML (internal/reader).
//
// There is no layout in here — no widths, colours or coordinates. Rendering
// is the UI's job; this package answers "what is on the page and what can be
// done to it".
package ir

import (
	"fmt"
	"strings"
)

// Kind is what a node IS for the purpose of drawing and acting on it.
// Several elements may share a Kind, and the Role field keeps the one the
// page wrote, for the Inspect popup.
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
	ListItem                // one bullet; Marker is its bullet or number
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
	Gauge                   // progressbar / meter — a value on a range that reads only; Min, Max, Value
)

var kindNames = [...]string{
	"document", "landmark", "heading", "paragraph", "text", "link", "button",
	"textbox", "check", "combobox", "option", "list", "listitem", "table", "row",
	"cell", "media", "separator", "code", "quote", "span", "group", "unsupported",
	"gauge",
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
	// Role is what the page wrote it as — "searchbox", not "textbox" — for
	// the Inspect popup.
	Role string
	// Name is the accessible name: link text, button label, the label of an
	// input, alt of an image, the heading's text.
	Name string
	// Value is an input's current text, a combobox's chosen option.
	Value string
	// URL is a link's href or an image's src, made absolute.
	URL string
	// Level is a heading's 1–6.
	Level int

	Checked   Tri
	Multiline bool // Textbox: a textarea or a contenteditable
	Protected bool // Textbox: a password field — the value is shown masked
	// InputType is what an <input> declared it takes — "email", "date",
	// "number" — lower-cased, empty when the page said nothing or the
	// field is not an <input>.
	InputType string
	Disabled  bool
	Selected  bool // an Option
	Header    bool // a Cell that is a column or row header
	// Block marks an inline kind that the page laid out as a block — a link
	// styled display:block, a button on a line of its own. Set from the
	// captured layout, never from the role.
	Block bool
	// Marker is a ListItem's bullet or number: "• ", "1. ".
	Marker string
	// Lang is a Code block's language when it is known — a whole document
	// that was JSON, YAML, TOML, Markdown… — for syntax colour. Empty for
	// code whose page does not name its language.
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
// interactive node, every heading.
func (n *Node) IsItem() bool {
	switch n.Kind {
	case Link, Button, Textbox, Check, Combobox, Media, Heading:
		return true
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
