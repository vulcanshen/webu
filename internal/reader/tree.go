package reader

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/vulcanshen/webu/internal/ir"
	"golang.org/x/net/html"
)

// build is root as webu's content tree, a Document node, leaving out what
// skip says to (nil: nothing).
func (d *Doc) build(t *trees, root *html.Node, skip func(*html.Node) bool) *ir.Node {
	doc := &ir.Node{Kind: ir.Document}
	b := &builder{d: d, t: t, skip: skip, labels: labelsFor(root), anchors: map[string]*ir.Node{}, block: doc}
	doc.Children = b.children(root)
	d.anchors[doc] = b.anchors
	return doc
}

type builder struct {
	d    *Doc
	t    *trees
	skip func(*html.Node) bool
	// labels is the text of each <label for=id>, by the id.
	labels  map[string]string
	anchors map[string]*ir.Node
	// form is the form being built; label the text of the <label> the
	// walk is inside, which names the field in it.
	form  *Form
	label string
	// block is the block being built: where an id inside it lands.
	block *ir.Node
}

// blockTags are the elements that start on a line of their own, with
// nothing of their own to say: a Group, which the markdown writer breaks
// lines around.
var blockTags = map[string]bool{"div": true, "section": true, "article": true, "main": true, "header": true,
	"footer": true, "nav": true, "aside": true, "figure": true, "address": true, "center": true, "details": true,
	"dl": true, "dd": true, "hgroup": true, "search": true, "fieldset": true, "body": true, "html": true}

func (b *builder) children(n *html.Node) []*ir.Node {
	var out []*ir.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, b.node(c)...)
	}
	return out
}

func (b *builder) node(n *html.Node) []*ir.Node {
	switch n.Type {
	case html.TextNode:
		if s := collapse(n.Data); s != "" {
			return []*ir.Node{text(s)}
		}
		return nil
	case html.DocumentNode:
		return b.children(n)
	case html.ElementNode:
	default:
		return nil
	}
	if b.skip != nil && b.skip(n) {
		return nil
	}
	out := b.element(n)
	// What a link to #id lands on: the block it made, else the block it
	// is in.
	id := attr(n, "id")
	if id == "" && n.Data == "a" {
		id = attr(n, "name")
	}
	if id != "" && b.anchors[id] == nil {
		b.anchors[id] = b.block
		if len(out) > 0 && out[0].IsBlock() {
			b.anchors[id] = out[0]
		}
	}
	return out
}

func (b *builder) element(n *html.Node) []*ir.Node {
	tag := n.Data
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		h := b.within(&ir.Node{Kind: ir.Heading, Level: int(tag[1] - '0')}, n)
		h.Name = strings.TrimSpace(inlineText(h.Children))
		return one(h)
	case "p":
		return one(b.within(&ir.Node{Kind: ir.Paragraph}, n))
	case "a":
		href := attr(n, "href")
		if href == "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(href)), "javascript:") {
			return b.children(n)
		}
		l := &ir.Node{Kind: ir.Link, URL: href, Children: b.children(n)}
		l.Name = strings.TrimSpace(inlineText(l.Children))
		if l.Name == "" {
			l.Name = strings.TrimSpace(nameOr(attr(n, "aria-label"), attr(n, "title")))
		}
		if l.Name == "" && !hasItem(&ir.Node{Children: l.Children}) {
			// A link with nothing to read on it — an arrow drawn by CSS,
			// an icon the page called decoration — says nothing here.
			return nil
		}
		return one(l)
	case "img":
		alt := strings.TrimSpace(attr(n, "alt"))
		if has(n, "alt") && alt == "" {
			return nil // decorative, the page says
		}
		return one(&ir.Node{Kind: ir.Media, Role: "img", Name: alt, URL: attr(n, "src")})
	case "br":
		return one(text("\n"))
	case "hr":
		return one(&ir.Node{Kind: ir.Separator})
	case "ul", "ol", "menu":
		return b.list(n)
	case "li":
		return one(b.within(&ir.Node{Kind: ir.ListItem, Marker: "- "}, n))
	case "table":
		return b.table(n)
	case "pre":
		t := strings.TrimPrefix(textOf(n), "\n")
		return one(&ir.Node{Kind: ir.Code, Block: true, Lang: language(n),
			Children: []*ir.Node{text(strings.TrimRight(t, "\n"))}})
	case "code", "kbd", "samp", "tt":
		if s := collapse(textOf(n)); strings.TrimSpace(s) != "" {
			return one(&ir.Node{Kind: ir.Code, Children: []*ir.Node{text(s)}})
		}
		return nil
	case "blockquote":
		return one(b.within(&ir.Node{Kind: ir.Quote}, n))
	case "strong", "b":
		return b.span(n, "strong")
	case "em", "i", "cite", "dfn":
		return b.span(n, "emphasis")
	case "del", "s", "strike":
		return b.span(n, "deletion")
	case "ins", "u":
		return b.span(n, "insertion")
	case "mark":
		return b.span(n, "mark")
	case "label":
		prev := b.label
		b.label = strings.TrimSpace(collapse(textOf(n)))
		out := b.children(n)
		b.label = prev
		return out
	case "legend", "figcaption", "summary", "caption", "dt":
		return one(&ir.Node{Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Span, Role: "strong", Children: b.children(n)}}})
	case "form":
		return b.formOf(n)
	case "input":
		return b.input(n)
	case "textarea":
		f := b.field(n, "textarea")
		if f.Value == "" && len(f.Nodes) == 0 {
			f.Value = textOf(n)
		}
		return one(b.show(f, &ir.Node{Kind: ir.Textbox, Role: "textarea", Multiline: true, Name: f.Label, Value: f.Value}))
	case "select":
		return b.selectOf(n)
	case "button":
		typ := strings.ToLower(nameOr(attr(n, "type"), "submit"))
		f := b.field(n, typ)
		name := strings.TrimSpace(collapse(textOf(n)))
		return one(b.show(f, &ir.Node{Kind: ir.Button, Role: "button", Name: nameOr(name, nameOr(f.Label, "Button"))}))
	case "head", "title", "option", "optgroup", "datalist", "source", "track", "area", "map", "audio", "video":
		return nil
	}
	if blockTags[tag] {
		return one(b.within(&ir.Node{Kind: ir.Group, Role: tag}, n))
	}
	// Anything else — span, small, abbr, time, a custom element — is its
	// content, inline.
	return b.children(n)
}

// within builds n's children into x, x being the block an id inside lands
// on when it is one.
func (b *builder) within(x *ir.Node, n *html.Node) *ir.Node {
	prev := b.block
	if x.IsBlock() {
		b.block = x
	}
	x.Children = b.children(n)
	b.block = prev
	return x
}

func (b *builder) span(n *html.Node, role string) []*ir.Node {
	kids := b.children(n)
	if strings.TrimSpace(inlineText(kids)) == "" {
		return kids
	}
	return one(&ir.Node{Kind: ir.Span, Role: role, Children: kids})
}

// list is a list with its items numbered, or bulleted.
func (b *builder) list(n *html.Node) []*ir.Node {
	l := &ir.Node{Kind: ir.List, Role: n.Data}
	num := 1
	if s, err := strconv.Atoi(attr(n, "start")); err == nil {
		num = s
	}
	prev := b.block
	b.block = l
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" && (b.skip == nil || !b.skip(c)) {
			marker := "- "
			if n.Data == "ol" {
				marker = strconv.Itoa(num) + ". "
				num++
			}
			l.Children = append(l.Children, b.within(&ir.Node{Kind: ir.ListItem, Marker: marker}, c))
			continue
		}
		for _, x := range b.node(c) {
			if x.Kind == ir.Text && strings.TrimSpace(x.Name) == "" {
				continue
			}
			l.Children = append(l.Children, x)
		}
	}
	b.block = prev
	if len(l.Children) == 0 {
		return nil
	}
	return one(l)
}

// table is a table of data — one with headers or a caption, or that says
// it is a table — or, for a table the page laid itself out with, its rows
// one after another, each a line of its cells (HN's list of stories: even
// Chromium calls it a table of data).
func (b *builder) table(n *html.Node) []*ir.Node {
	rows := tableRows(n)
	data := attr(n, "role") == "table" || attr(n, "role") == "grid"
	var caption *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "caption" {
			caption, data = c, true
		}
	}
	for _, r := range rows {
		for c := r.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "th" {
				data = true
			}
		}
	}
	switch attr(n, "role") {
	case "presentation", "none":
		data = false
	}
	var out []*ir.Node
	if !data {
		for _, r := range rows {
			g := &ir.Node{Kind: ir.Group, Role: "tr"}
			prev := b.block
			b.block = g
			for c := r.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.ElementNode || c.Data != "td" && c.Data != "th" {
					continue
				}
				// A row is a line: what the page boxed inside a cell
				// (a <center>, a <div>) runs on with the rest.
				kids := flat(b.children(c))
				if len(kids) == 0 {
					continue
				}
				if len(g.Children) > 0 {
					g.Children = append(g.Children, text(" "))
				}
				g.Children = append(g.Children, kids...)
			}
			b.block = prev
			if strings.TrimSpace(g.Text()) != "" || hasItem(g) {
				out = append(out, g)
			}
		}
		return out
	}
	if caption != nil {
		out = append(out, b.element(caption)...)
	}
	t := &ir.Node{Kind: ir.Table}
	prev := b.block
	b.block = t
	for _, r := range rows {
		row := &ir.Node{Kind: ir.Row}
		for c := r.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
				row.Children = append(row.Children, b.within(&ir.Node{Kind: ir.Cell, Header: c.Data == "th"}, c))
			}
		}
		if len(row.Children) > 0 {
			t.Children = append(t.Children, row)
		}
	}
	b.block = prev
	if len(t.Children) == 0 {
		return out
	}
	return append(out, t)
}

// tableRows are a table's own rows, through its thead, tbody and tfoot,
// none of a table inside it.
func tableRows(n *html.Node) []*html.Node {
	var rows []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "tr":
			rows = append(rows, c)
		case "thead", "tbody", "tfoot":
			rows = append(rows, tableRows(c)...)
		}
	}
	return rows
}

// formOf is a form: its box in the tree, the form itself once for both
// trees.
func (b *builder) formOf(n *html.Node) []*ir.Node {
	fm := b.t.forms[n]
	if fm == nil {
		fm = &Form{Name: formName(n), Method: "GET", Enctype: strings.ToLower(attr(n, "enctype")), Action: b.d.URL}
		if strings.EqualFold(attr(n, "method"), "post") {
			fm.Method = "POST"
		}
		if a := strings.TrimSpace(attr(n, "action")); a != "" {
			if u, err := url.Parse(a); err == nil {
				fm.Action = u
			}
		}
		b.t.forms[n] = fm
		b.d.Forms = append(b.d.Forms, fm)
	}
	g := &ir.Node{Kind: ir.Group, Role: "form", Name: fm.Name}
	prev := b.form
	b.form = fm
	b.within(g, n)
	b.form = prev
	return one(g)
}

// formName is what a form is called, if the page says.
func formName(n *html.Node) string {
	for _, k := range []string{"aria-label", "title", "name", "id"} {
		if v := strings.TrimSpace(attr(n, k)); v != "" {
			return v
		}
	}
	return ""
}

func (b *builder) input(n *html.Node) []*ir.Node {
	typ := strings.ToLower(nameOr(attr(n, "type"), "text"))
	f := b.field(n, typ)
	switch typ {
	case "hidden":
		return nil // sent with the form, never shown
	case "checkbox", "radio":
		c := &ir.Node{Kind: ir.Check, Role: typ, Name: f.Label}
		if f.Checked {
			c.Checked = ir.On
		}
		return one(b.show(f, c))
	case "submit", "image", "reset", "button":
		name := map[string]string{"submit": "Submit", "image": "Submit", "reset": "Reset", "button": "Button"}[typ]
		if typ == "image" {
			name = nameOr(attr(n, "alt"), name)
		}
		return one(b.show(f, &ir.Node{Kind: ir.Button, Role: typ, Name: nameOr(f.Value, name)}))
	}
	return one(b.show(f, &ir.Node{Kind: ir.Textbox, Role: typ, InputType: typ, Name: f.Label,
		Value: f.Value, Protected: typ == "password", Disabled: f.Disabled || typ == "file"}))
}

func (b *builder) selectOf(n *html.Node) []*ir.Node {
	f := b.field(n, "select")
	c := &ir.Node{Kind: ir.Combobox, Role: "select", Name: f.Label}
	for i, o := range f.Options {
		c.Children = append(c.Children, &ir.Node{Kind: ir.Option, Name: o.Label, Selected: o.Selected})
		if o.Selected || i == 0 && c.Value == "" {
			c.Value = o.Label
		}
	}
	return one(b.show(f, c))
}

// show ties a field to the node it is drawn as.
func (b *builder) show(f *Field, x *ir.Node) *ir.Node {
	x.Disabled = x.Disabled || f.Disabled
	f.Nodes = append(f.Nodes, x)
	b.d.fields[x] = f
	return x
}

// field is the form field n is, made the first time either tree meets it:
// its name, type, the page's value, and what it is called.
func (b *builder) field(n *html.Node, typ string) *Field {
	if f := b.t.fields[n]; f != nil {
		return f
	}
	f := &Field{Form: b.form, Name: attr(n, "name"), Type: typ, Value: attr(n, "value"),
		Checked: has(n, "checked"), Disabled: has(n, "disabled")}
	if typ == "select" {
		f.Options = options(n)
		if o, ok := f.chosen(); ok {
			f.Value = o.Value
		}
	}
	if typ == "checkbox" || typ == "radio" {
		f.Value = nameOr(f.Value, "on")
	}
	label := b.labels[attr(n, "id")]
	if label == "" && b.label != "" {
		label = b.label
	}
	// The hint an empty box shows: its placeholder, else a name the page
	// shows nowhere else — a <label> is on the page already.
	f.Hint = strings.TrimSpace(attr(n, "placeholder"))
	for _, k := range []string{"aria-label", "placeholder", "title", "name"} {
		if label != "" {
			break
		}
		label = strings.TrimSpace(attr(n, k))
		if f.Hint == "" {
			f.Hint = label
		}
	}
	f.Label = label
	b.t.fields[n] = f
	if b.form != nil {
		b.form.Fields = append(b.form.Fields, f)
	}
	return f
}

// options are a select's options, through its optgroups.
func options(n *html.Node) []Option {
	var out []Option
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "option":
			label := strings.TrimSpace(collapse(textOf(c)))
			value := label
			if has(c, "value") {
				value = attr(c, "value")
			}
			out = append(out, Option{Label: nameOr(label, value), Value: value, Selected: has(c, "selected")})
		case "optgroup":
			out = append(out, options(c)...)
		}
	}
	return out
}

// labelsFor is every <label for=id>'s text, by the id.
func labelsFor(root *html.Node) map[string]string {
	out := map[string]string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "label" {
			if id := attr(n, "for"); id != "" {
				out[id] = strings.TrimSpace(collapse(textOf(n)))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// language is a code block's language, as its class names it
// ("language-go", "lang-go").
func language(n *html.Node) string {
	for x := n; x != nil; x = x.FirstChild {
		if x.Type != html.ElementNode {
			continue
		}
		for _, c := range strings.Fields(attr(x, "class")) {
			for _, p := range []string{"language-", "lang-"} {
				if strings.HasPrefix(c, p) {
					return strings.TrimPrefix(c, p)
				}
			}
		}
	}
	return ""
}

// inlineText is nodes' text, run together.
func inlineText(nodes []*ir.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Kind {
		case ir.Text, ir.Link:
			b.WriteString(n.Name)
		case ir.Media:
			b.WriteString(n.Name)
		default:
			b.WriteString(inlineText(n.Children))
		}
	}
	return b.String()
}

// flat is nodes with their groups and paragraphs opened up, so they run
// on as one line; lists, tables and the like stay what they are.
func flat(nodes []*ir.Node) []*ir.Node {
	var out []*ir.Node
	for _, n := range nodes {
		if (n.Kind == ir.Group && n.Role != "form") || n.Kind == ir.Paragraph {
			if len(out) > 0 {
				out = append(out, text(" "))
			}
			out = append(out, flat(n.Children)...)
			continue
		}
		out = append(out, n)
	}
	return out
}

func hasItem(n *ir.Node) bool {
	found := false
	n.Walk(func(x *ir.Node) bool {
		switch x.Kind {
		case ir.Link, ir.Media, ir.Textbox, ir.Check, ir.Combobox, ir.Button:
			found = true
		}
		return !found
	})
	return found
}

// collapse is s with every run of white space one space, the way HTML
// draws text outside a <pre>.
func collapse(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '\f' {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

func text(s string) *ir.Node { return &ir.Node{Kind: ir.Text, Name: s} }

func one(n *ir.Node) []*ir.Node { return []*ir.Node{n} }
