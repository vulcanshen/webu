package reader

import (
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strings"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/vulcanshen/webu/internal/ir"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Doc is a page as the reader read it: the content the page itself
// presents, and the whole of it for the key that shows everything.
type Doc struct {
	URL   *url.URL
	Title string
	// Status is the HTTP status: an error page is read like any other,
	// and the app says what the status was.
	Status     int
	StatusText string
	// Content is what the page presents, the site around it left out
	// (docs/reader.md); Whole is everything in its <body>. Each is a
	// Document node.
	Content, Whole *ir.Node
	// Picked says how Content was found: "article" (Readability), "page"
	// (the whole page less its navigation, header, footer and sidebars),
	// or "text" (a document that is text, not HTML).
	Picked string
	Forms  []*Form
	// Icon is where the page says its icon is (<link rel=icon>), "" when
	// it does not.
	Icon string

	fields  map[*ir.Node]*Field
	anchors map[*ir.Node]map[string]*ir.Node
}

// Field is the form field drawn as n, or nil.
func (d *Doc) Field(n *ir.Node) *Field { return d.fields[n] }

// Anchor is the node of root a link to #id lands on, or nil.
func (d *Doc) Anchor(root *ir.Node, id string) *ir.Node { return d.anchors[root][id] }

// Read reads a page off what came back.
func Read(r *Response) (*Doc, error) {
	d := &Doc{URL: r.URL, Status: r.Status, StatusText: r.StatusText,
		fields: map[*ir.Node]*Field{}, anchors: map[*ir.Node]map[string]*ir.Node{}}
	body, err := charset.NewReader(bytes.NewReader(r.Body), r.ContentType)
	if err != nil {
		body = bytes.NewReader(r.Body)
	}
	if r.Type != "" && r.Type != "text/html" && r.Type != "application/xhtml+xml" {
		// Text reads as itself: one block, as it came.
		raw, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		lang := ""
		if strings.Contains(r.Type, "json") {
			lang = "json"
		}
		code := &ir.Node{Kind: ir.Code, Lang: lang, Children: []*ir.Node{{Kind: ir.Text, Name: strings.TrimRight(string(raw), "\n")}}}
		d.Content = &ir.Node{Kind: ir.Document, Children: []*ir.Node{code}}
		d.Whole, d.Picked, d.Title = d.Content, "text", lastPart(r.URL)
		return d, nil
	}
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}
	absolutize(doc, r.URL)
	d.Title = strings.TrimSpace(textOf(find(doc, "title")))
	d.Icon = iconOf(doc)
	clean(doc)
	shared := &trees{fields: map[*html.Node]*Field{}, forms: map[*html.Node]*Form{}}
	bodyNode := find(doc, "body")
	if bodyNode == nil {
		bodyNode = doc
	}
	d.Whole = d.build(shared, bodyNode, nil)

	if readability.CheckDocument(doc) {
		if art, err := readability.FromDocument(doc, r.URL); err == nil && art.Node != nil {
			d.Content, d.Picked = d.build(shared, art.Node, nil), "article"
			if t := strings.TrimSpace(art.Title()); t != "" {
				d.Title = t
			}
		}
	}
	if d.Content == nil {
		d.Content, d.Picked = d.build(shared, bodyNode, chrome), "page"
		if len(d.Content.Children) == 0 {
			d.Content = d.Whole
		}
	}
	// The title on top, when the content does not start with it.
	if d.Title != "" && d.Picked != "text" && !startsWith(d.Content, d.Title) {
		h := &ir.Node{Kind: ir.Heading, Level: 1, Name: d.Title, Children: []*ir.Node{{Kind: ir.Text, Name: d.Title}}}
		d.Content.Children = append([]*ir.Node{h}, d.Content.Children...)
	}
	return d, nil
}

// iconOf is the address of the page's icon, by its <link rel=icon>,
// read before clean takes the <link>s out.
func iconOf(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "link" {
		for _, w := range strings.Fields(strings.ToLower(attr(n, "rel"))) {
			if w == "icon" && attr(n, "href") != "" {
				return attr(n, "href")
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if s := iconOf(c); s != "" {
			return s
		}
	}
	return ""
}

// trees is what both trees of a page share: a field is one field however
// many times it is drawn.
type trees struct {
	fields map[*html.Node]*Field
	forms  map[*html.Node]*Form
}

// clean takes out of the page what is on no screen, or over it: scripts
// and styles, what the page hid, dialogs, cookie and consent notices.
func clean(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.CommentNode || c.Type == html.ElementNode && unseen(c) {
			n.RemoveChild(c)
		} else {
			clean(c)
		}
		c = next
	}
}

var hiddenStyle = regexp.MustCompile(`(?i)(display\s*:\s*none|visibility\s*:\s*hidden)`)

// unseen is an element no one sees as part of the page.
func unseen(n *html.Node) bool {
	switch n.Data {
	case "script", "style", "noscript", "template", "svg", "canvas", "iframe", "object", "embed", "link", "meta":
		return true
	case "dialog":
		return !has(n, "open")
	}
	if has(n, "hidden") || attr(n, "aria-hidden") == "true" || hiddenStyle.MatchString(attr(n, "style")) {
		return true
	}
	switch attr(n, "role") {
	case "dialog", "alertdialog":
		return true
	}
	for _, w := range words(attr(n, "class") + " " + attr(n, "id")) {
		switch w {
		case "cookie", "cookies", "consent", "gdpr":
			return true
		}
	}
	return false
}

// chrome is the site around a page's content: its navigation, header,
// footer and sidebars, by tag, role or name.
func chrome(n *html.Node) bool {
	switch n.Data {
	case "nav", "header", "footer", "aside":
		return true
	}
	switch attr(n, "role") {
	case "navigation", "banner", "contentinfo", "complementary", "menu", "menubar":
		return true
	}
	for _, w := range words(attr(n, "class") + " " + attr(n, "id")) {
		switch w {
		case "nav", "navbar", "navigation", "menu", "menubar", "sidebar", "footer", "breadcrumb", "breadcrumbs":
			return true
		}
	}
	return false
}

// words splits class names and ids into their words: "main-menu" holds
// "menu".
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

// absolutize makes every address absolute against the page's <base href>
// or its own: a link written "#top" or "../a" means nothing once it is out
// of its page.
func absolutize(doc *html.Node, page *url.URL) {
	base := page
	if b := find(doc, "base"); b != nil {
		if u, err := page.Parse(strings.TrimSpace(attr(b, "href"))); err == nil && has(b, "href") {
			base = u
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i, a := range n.Attr {
				switch a.Key {
				case "href", "src", "action":
					if u, err := base.Parse(strings.TrimSpace(a.Val)); err == nil {
						n.Attr[i].Val = u.String()
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
}

// startsWith reports whether root's first block is a heading saying title.
func startsWith(root *ir.Node, title string) bool {
	for _, c := range root.Children {
		if c.Kind == ir.Heading {
			return strings.EqualFold(strings.TrimSpace(c.Text()), title)
		}
		if strings.TrimSpace(c.Text()) != "" {
			return false
		}
	}
	return false
}

func lastPart(u *url.URL) string {
	p := strings.TrimSuffix(u.Path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	return nameOr(p, u.Host)
}

// find is the first element named tag in n, n included.
func find(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := find(c, tag); f != nil {
			return f
		}
	}
	return nil
}

// textOf is n's text, as it is in the source.
func textOf(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func has(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
