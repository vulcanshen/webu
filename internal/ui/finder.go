package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/ir"
)

// The finder is [/] (finder: components/input/finder): every node of
// the page that holds text, across all its parts, and Enter GOES there —
// the part switches, the section opens, the list item is drilled into, and
// the cursor lands. It never presses anything: a hit on a link's text must
// not leave the page, so what Enter would do on the page is one more Enter
// away, where you can see what it is (user, 2026-09-23). The popup is the
// lib's; the index and the landing are webu's.

// hit is one row of the list: a node of the page and where it is.
//
// It keeps no pointers it would have to trust later. The page is
// recaptured while the finder is up — every cursor move settles, and a
// settle rebuilds the tree — so a hit is found again at landing time by
// Chromium's id, or failing that by its trail of child indexes from the
// root, and everything else (what to drill into, where to land) is read
// off the tree as it is then (app.goToHit).
type hit struct {
	node  *ir.Node // as indexed: for the preview and the text, never for landing
	part  partKind
	trail []int // child indexes from the root, outermost first
	// under is the heading the node sits under, for the preview's title.
	under string
	// text is what the node says, one line — what is matched and shown.
	text string
	// line is [go]'s line number.
	line int
}

// indexPage is every content node of the page, in reading order, across
// all its parts. A hit is the smallest block that holds text — a
// paragraph with its links, a heading, a list item's own line, a cell —
// so nothing is listed twice: the paragraph carries the link in it, and
// a list item that is only a link is one row, not two. Containers with
// no text of their own (a list, a table, an unnamed region) are not
// hits; a named region is, on its name.
func indexPage(t *tab) []hit {
	if t == nil || t.root == nil {
		return nil
	}
	partOf := map[*ir.Node]partKind{}
	for _, p := range t.parts {
		for _, n := range p.nodes {
			partOf[n] = p.kind
		}
	}
	var out []hit
	under := ""
	var walk func(n *ir.Node, part partKind, trail []int)
	walk = func(n *ir.Node, part partKind, trail []int) {
		if n.Kind == ir.Combobox {
			// Its options are listed behind it, not on the page.
			return
		}
		if n.Kind == ir.Heading {
			under = headingTitle(n, t.url)
		}
		add := func(node *ir.Node, text string, tr []int) {
			out = append(out, hit{node: node, part: part, trail: append([]int(nil), tr...),
				under: under, text: text})
		}
		if text := hitText(n); text != "" && !namedByItsHeading(n) {
			add(n, text, trail)
		}
		if n.Kind == ir.Document || n.Kind == ir.Landmark {
			// A region's own inline children — a footer that is three
			// bare links, a nav that is a run of them — are drawn one
			// per line (render.children), and each is a hit of its own.
			for i, c := range n.Children {
				if c.IsBlock() || c.Kind == ir.Option {
					continue
				}
				if text := strings.TrimSpace(c.Text()); text != "" {
					add(c, text, append(trail, i))
				}
			}
		}
		for i, c := range n.Children {
			walk(c, part, append(trail, i))
		}
	}
	// A heading holds until the next one, or the next PART: a heading
	// in the body says nothing about the footer, and a flat page — every
	// paragraph a top-level node — is still one run of reading.
	last := partKind(255)
	for i, c := range t.root.Children {
		part, ok := partOf[c]
		if !ok {
			part = partMain
		}
		if part != last {
			under, last = "", part
		}
		walk(c, part, []int{i})
	}
	return out
}

// hitText is what a node contributes to the index: the text of its own
// inline run, with blocks under it left to be hits of their own. Empty
// for a node that is only a container.
func hitText(n *ir.Node) string {
	switch n.Kind {
	case ir.Document, ir.Text, ir.Span, ir.List, ir.Table, ir.Row, ir.Separator:
		return ""
	case ir.Option:
		// A listbox's option is a row of its own (a combobox's never
		// reach here).
		return strings.TrimSpace(n.Name)
	case ir.Heading:
		if s := strings.TrimSpace(n.Name); s != "" {
			return s
		}
	case ir.Landmark:
		if n.Role == "main" || n.Skip {
			return ""
		}
		return strings.TrimSpace(n.Name)
	case ir.Link, ir.Button, ir.Media, ir.Textbox, ir.Check, ir.Combobox:
		// Inline things: their block carries them. On their own — a bare
		// link the page laid out as a block — they are their own hit.
		if !n.Block {
			return ""
		}
		return strings.TrimSpace(n.Text())
	case ir.Code:
		return strings.TrimSpace(n.Text())
	}
	// The inline run, as the page spaced it: a text node carries its own
	// spaces, and a space put between a link and the comma after it was
	// webu's, not the page's.
	var b strings.Builder
	if n.Kind == ir.Unsupported && n.Name != "" {
		b.WriteString(n.Name + " ")
	}
	for _, c := range n.Children {
		if !c.IsBlock() {
			b.WriteString(c.Text())
		}
	}
	return strings.TrimSpace(b.String())
}

// previewOf draws one node the way the page draws it, at width, as
// styled lines.
func previewOf(n *ir.Node, width int) []string {
	doc := &ir.Node{Kind: ir.Document, Children: []*ir.Node{n}}
	lay := renderWith(doc, renderOpts{width: max(1, width), drill: n.ID})
	styles := segStyles()
	out := make([]string, 0, len(lay.rows))
	for _, r := range lay.rows {
		var b strings.Builder
		for _, s := range r.segs {
			b.WriteString(withAttr(styles[s.kind], s.attr).Render(s.text))
		}
		out = append(out, b.String())
	}
	// Trailing blank rows are the page's gaps, not the node's.
	for len(out) > 0 && strings.TrimSpace(lay.rows[len(out)-1].plain()) == "" {
		out = out[:len(out)-1]
	}
	return out
}

// openFinder is [/] over the shown tab.
func (m *AppModel) openFinder() tea.Cmd {
	t := m.shownTab()
	if t == nil || t.root == nil {
		return m.toast("No page to search")
	}
	if t.popupNode() != nil {
		// A dialog is a few lines to answer: nothing in it is reached by
		// search (user, 2026-09-23).
		return m.toast("A popup is answered, not searched")
	}
	hits := indexPage(t)
	items := make([]terminu.FinderItem, len(hits))
	for i, h := range hits {
		items[i] = terminu.FinderItem{ID: strconv.Itoa(i), Text: h.text, Icon: h.part.glyph(), IconColor: terminu.Overlay0}
	}
	return m.a.Find(&terminu.Finder{Title: "Search", Subject: strings.TrimSpace(t.title), Glyph: glyphSearch, Verb: "go",
		Items: items,
		Preview: func(it terminu.FinderItem) (string, []string) {
			i, err := strconv.Atoi(it.ID)
			if err != nil || i >= len(hits) {
				return "", nil
			}
			h := hits[i]
			return nameOr(h.under, h.part.word()), previewOf(h.node, max(20, m.pageW()/2))
		},
		Done: func(it terminu.FinderItem) tea.Cmd {
			defer m.sync()
			i, err := strconv.Atoi(it.ID)
			if err != nil || i >= len(hits) {
				return nil
			}
			if !m.goToHit(t, hits[i]) {
				return m.toast("That is not on the page any more")
			}
			return m.focusPage()
		}})
}

// goToHit takes the page to a search hit: the part it is in, the things it
// is inside drilled into, the section that holds it opened, and the window
// and cursor on its row. Nothing is pressed.
//
// Everything about WHERE is read off the tree as it is now — the hit says
// which node, the tree says what it is inside (tab.chainOf): the page is
// recaptured while the finder is up, and a path remembered at index time
// would be a path through a tree that is gone.
func (m *AppModel) goToHit(t *tab, h hit) bool {
	w, vis := m.pageW(), m.pageVisible()
	chain := t.chainOf(h)
	if chain == nil {
		return false
	}
	node := chain[len(chain)-1]
	if t.at != partWhole {
		t.showPart(h.part, w, vis)
	}
	// Each list item or article on the way down is one thing until you go
	// in: go in.
	for _, a := range chain[:len(chain)-1] {
		if isThing(a) && !t.drillInto(a, w, vis) {
			return false
		}
	}
	t.reveal(node, w)
	// The node's own row, or the nearest ancestor's that has one — a
	// table's cell is the item, and what is in the cell lands on it.
	row := -1
	for i := len(chain) - 1; i >= 0 && row < 0; i-- {
		row = t.rowOf(chain[i])
	}
	if row < 0 {
		return false
	}
	if t.listing() || t.read {
		// A document: the section holding the row is the one to read.
		t.sec = sectionAt(t.secs, row)
		t.read = true
	}
	t.landRow(row, vis)
	return true
}
