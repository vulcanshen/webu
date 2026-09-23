package ui

import (
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

// render lays an IR tree out for one width: rows of segments, and the list
// of items the cursor can stop on, in reading order. It is the whole of
// "draw the page" — the panel only colours what comes out and picks a
// window of rows to show.
//
// Blocks break lines; inline nodes flow together and wrap at spaces. An item
// (a link, a button, a field) is a run of segments that may span several
// rows, and every segment remembers which item it belongs to so the cursor
// can light the whole run.

type segKind uint8

const (
	segPlain segKind = iota
	segDim
	segHeading
	segLink
	segButton
	segInput
	// segCaret is the one cell that says a value can be typed here
	// (theme.pageInputBg).
	segCaret
	segCheck
	segMedia
	segCode
	segUnsupported
	segLandmark    // a landmark's rule row: its role and name
	segTableHeader // a table's header cells
	// Inside a code block with a language (highlight.go).
	segCodeKey
	segCodeString
	segCodeNumber
	segCodeConst
	segCodeKeyword
	segCodeComment
	segCodePunct
	segCodeHeading
	segCodeStrong
	segCodeEmph
)

type seg struct {
	text string
	item int // index into layout.items, or -1
	kind segKind
	// attr is what the page marked this run up as — bold, italic, struck
	// through — on top of whatever its kind draws (textAttr).
	attr textAttr
}

// textAttr is the terminal's own text attributes, which is how webu draws
// the inline markup a page carries: <strong>, <em>, <del>, <ins>, <mark>
// (2026-09-22). A bitfield rather than more segKinds, because they
// compose — bold inside a link is both — and because a kind already says
// what a run IS while an attribute says how it is marked up.
type textAttr uint8

const (
	attrBold textAttr = 1 << iota
	attrItalic
	attrStrike
	attrUnderline
	attrReverse
)

// attrOf is the attribute an inline run's role draws with.
func attrOf(role string) textAttr {
	switch role {
	case "strong":
		return attrBold
	case "emphasis":
		return attrItalic
	case "deletion":
		return attrStrike
	case "insertion":
		return attrUnderline
	case "mark":
		// Reverse video: a highlight that borrows no colour band, so it
		// cannot be read as a link, as code, or as the cursor (theme).
		return attrReverse
	}
	return 0
}

type row struct {
	segs []seg
	// code: a row of a code block, drawn on the code background across
	// the row, padding included.
	code bool
	// table: a row of a data table, on the table's ground; header marks
	// the header row, on the deeper one (2026-09-21).
	table, header bool
	// heading: how deep the heading this row belongs to sits, counting
	// from 1, which is the ink it is drawn in (theme.levelColor). Zero
	// for every other row.
	heading int
	// box: the row is part of a framed block — a form — and which part;
	// boxW is how wide that block is. The frame is drawn by the panel,
	// the way a code block's ground is (pagepanel.pageRows).
	box  boxPart
	boxW int
}

// plain is the row's text with no styling — what tests and the search read.
func (r row) plain() string {
	var b strings.Builder
	for _, s := range r.segs {
		b.WriteString(s.text)
	}
	return b.String()
}

// boxPart is which part of a framed block a row is.
type boxPart uint8

const (
	boxNone boxPart = iota
	boxTop
	boxSide
	boxBottom
)

type item struct {
	node        *ir.Node
	first, last int  // row span, inclusive
	col         int  // cell column where it starts on its first row: h/l walk a row by it
	folded      bool // a landmark drawn as one line, its content hidden
}

type layout struct {
	rows  []row
	items []item
	// marks is the first row of every landmark, heading and block — a
	// paragraph, a list item, a table — so a jump to one (the section
	// list, a search hit) has a row to land on.
	marks map[*ir.Node]int
}

// mainOf is the page's main landmark, or nil.
func mainOf(root *ir.Node) *ir.Node {
	if root == nil {
		return nil
	}
	var main *ir.Node
	root.Walk(func(x *ir.Node) bool {
		if main == nil && x.Kind == ir.Landmark && x.Role == "main" {
			main = x
		}
		return main == nil
	})
	return main
}

// itemAt is the first item whose span includes row, or -1.
func (l layout) itemAt(row int) int {
	for i, it := range l.items {
		if row >= it.first && row <= it.last {
			return i
		}
	}
	return -1
}

// itemAtCol is the item under the character at row/col (a rune index), or
// -1: what Enter clicks in selection mode.
func (l layout) itemAtCol(row, col int) int {
	if row < 0 || row >= len(l.rows) {
		return -1
	}
	at := 0
	for _, s := range l.rows[row].segs {
		n := len([]rune(s.text))
		if col >= at && col < at+n {
			return s.item
		}
		at += n
	}
	return -1
}

// nearestItem is the item whose rows are closest to row — where the item
// cursor lands when selection mode ends (ux.md §1).
func (l layout) nearestItem(row int) int {
	best, bestD := -1, 1<<30
	for i, it := range l.items {
		d := 0
		switch {
		case row < it.first:
			d = it.first - row
		case row > it.last:
			d = row - it.last
		}
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// atom is one unbreakable unit of inline flow: a word, a glyph, a space.
type atom struct {
	text  string
	item  int
	kind  segKind
	attr  textAttr // stamped by add from the renderer's current markup
	space bool     // a breaking space: dropped at a line end
}

// renderOpts is what a layout depends on besides the tree.
type renderOpts struct {
	width int
	// measure caps how wide text flows (ui.md revision 2026-09-20): a
	// paragraph across 200 cells cannot be read back to its start. Tables,
	// code and rules still take the width. 0 is no cap.
	measure int
	// drill is the list item opened to the whole panel: it alone draws
	// its contents, every other one draws its first line (ui tab.drill).
	drill cdp.BackendNodeID
	// fold overrides the default folding of landmarks: true folds, false
	// opens. Absent means the default (renderer.folded).
	fold map[cdp.BackendNodeID]bool
}

type renderer struct {
	width int
	textW int // where flow wraps: width, or the measure when narrower
	rows  []row
	items []item
	flow  []atom
	fold  map[cdp.BackendNodeID]bool
	// drill is the list item opened to the whole panel (renderOpts).
	drill cdp.BackendNodeID
	// formLabel is the width of the label column while a form is being
	// drawn, 0 outside one (render.formLabelW).
	formLabel int
	// boxed counts the framed blocks being drawn around the current row,
	// boxW how wide the innermost one is. formStack is set when the
	// terminal is too narrow to put a value beside its label, and inForm
	// says a form is being drawn at all — which a label column of zero
	// cannot say for itself.
	boxed     int
	boxW      int
	formStack bool
	inForm    bool
	root      *ir.Node
	// A collapsed heading (2026-09-21) hides its section: everything after
	// it up to the next heading of its level or higher, or the end of the
	// landmark it is in. suppress is on while that is being skipped;
	// foldLevel is the heading's level, foldLm the landmark depth it sat
	// at, lmDepth the landmark depth now.
	suppress  bool
	foldLevel int
	foldLm    int
	lmDepth   int
	// cellItem is the item every atom inside a data table's cell belongs
	// to while the cell is drawn: the cell is the one stop, the links in
	// it are behind it (table, 2026-09-21). -1 outside a cell.
	cellItem int
	// heads is the stack of heading levels open above the one being
	// gathered: its size is the depth, which is what the outline is drawn
	// by (theme.levelColor). A page that goes h1 → h3 → h4 nests three
	// deep — the shape of a hierarchy is the shape, not the tag names.
	heads []int
	// head is the DEPTH of the heading being gathered, which emit stamps
	// onto its rows; 0 outside one.
	head int
	// attr is the markup the run being gathered sits inside — <strong>,
	// <em>, <del> — which add stamps onto every atom (inline, ir.Span).
	attr   textAttr
	indent string // prefix every wrapped line of the current block gets
	lead   string // prefix the FIRST line gets instead of indent (a list marker); consumed by the next flush
	gap    bool   // a blank row is owed before the next content row
	// inCell: a table cell is being gathered. A cell is one line, so a block
	// inside it (a <center>, a <div>) flattens into the flow instead of
	// emitting rows of its own — which would land ABOVE the table, since the
	// table's rows are emitted after every cell has been measured.
	inCell bool
	// markNext are nodes waiting for their first row: assigned by emit, so
	// a gap row owed before them is not counted as theirs.
	markNext []*ir.Node
	marks    map[*ir.Node]int
}

func render(root *ir.Node, width int) layout {
	return renderWith(root, renderOpts{width: width})
}

func renderWith(root *ir.Node, o renderOpts) layout {
	r := &renderer{width: max(1, o.width), textW: max(1, o.width), fold: o.fold,
		drill: o.drill, root: root, marks: map[*ir.Node]int{}, cellItem: -1}
	if o.measure > 0 && o.measure < r.width {
		r.textW = o.measure
	}
	r.block(root, 0)
	r.flush()
	return layout{rows: r.rows, items: r.items, marks: r.marks}
}

// folded says whether a landmark is drawn shut: only when the user shut it.
// Everything opens by default (revised 2026-09-21 — the landmarks outside
// main used to start folded when the page had one, and a page that hides
// its own header on arrival reads as broken, not tidy).
func (r *renderer) folded(n *ir.Node) bool {
	return r.fold[n.ID]
}

// landmarkRule is the row that names a landmark: a triangle for its
// state, its role and name, and the count of what a folded one hides,
// then a rule across the panel. It is an item, so Enter can open or shut
// it and the Outline can land on it.
func (r *renderer) landmarkRule(n *ir.Node, id int, folded bool) {
	glyph := "▾"
	if folded {
		glyph = "▸"
	}
	// Its NAME, not its role. "form" and "article" are words out of the
	// spec, and a reader does not need to be told the spelling of what
	// they are looking at — the shape says it: a form is a label column
	// against a value column, an article is one row you go into (user,
	// 2026-09-22). A region with a name has something to say, and says
	// only that.
	label := " " + glyph + " " + oneLine(n.Name)
	if c := countItems(n); folded && c > 0 {
		label += " · " + plural(c, "item")
	}
	label = truncate(label, max(1, r.width-2))
	rest := max(0, r.width-dispW(label)-1)
	r.emit(row{segs: []seg{
		{text: label, item: id, kind: segLandmark},
		{text: " " + strings.Repeat("─", rest), item: -1, kind: segDim},
	}})
}

// A piece of page chrome — a banner, a navigation or breadcrumb, a
// search, a sidebar, a footer, a dialog — is an entry, not a region
// (ux.md §A.0.K, 2026-09-21): what it holds is its item operations, not
// items, since a row of links invites walking sideways and a terminal
// has no width to spare for one. Since 2026-09-22 it is a capsule on
// the pagetab under the URL rather than a row of the page (capsule).

// depthOf opens a heading of this level and says how deep it sits: every
// heading at or below it is closed first, the way an outline nests.
func (r *renderer) depthOf(level int) int {
	for len(r.heads) > 0 && r.heads[len(r.heads)-1] >= level {
		r.heads = r.heads[:len(r.heads)-1]
	}
	r.heads = append(r.heads, level)
	return len(r.heads)
}

// thing draws one of the page's own things — a list item, an article —
// shut: one item, one line (user, 2026-09-22).
//
// A card, a result, an entry is a THING, and a thing that spreads over a
// dozen unlabelled lines is a thing you cannot see. The rule is the same
// whatever is in it: one that already fits on a line simply looks the way
// it always did.
func (r *renderer) thing(n *ir.Node) {
	id := r.newItem(n)
	for _, sg := range r.firstLine(n) {
		r.add(atom{text: sg.text, item: id, kind: sg.kind, attr: sg.attr})
	}
	r.flush()
	if n.Kind == ir.Landmark {
		// An article stands apart from what follows it; a bullet does
		// not — a list of them is a list, not a run of paragraphs.
		r.gap = true
	}
}

// firstLine is what a list item shows when it is shut: the first line it
// would draw, styled as it would be drawn.
//
// Taken by rendering the item on its own and keeping the first row that
// has anything on it, rather than by flattening the text: a card's
// flattened text is every field it has run together, and its first LINE
// is what a reader would call its title.
func (r *renderer) firstLine(n *ir.Node) []seg {
	sub := &renderer{width: r.width, textW: r.textW, root: r.root,
		marks: map[*ir.Node]int{}, cellItem: -1, fold: r.fold}
	sub.inlineChildren(n, -1, segPlain)
	sub.flush()
	for _, row := range sub.rows {
		if strings.TrimSpace(row.plain()) != "" {
			return row.segs
		}
	}
	return nil
}

// formChildren draws a landmark's children, and a form's in columns.
//
// A form is drawn as a form: every label in one column, every value
// against the same left edge (user, 2026-09-22, after sshu's). The
// alignment IS what says "this is a form" — a terminal has no box to draw
// one with, and labels trailing their own field at whatever column it
// happens to start read as a paragraph with underscores in it.
func (r *renderer) formChildren(n *ir.Node, depth int) {
	if n.Role != "form" {
		r.children(n, depth)
		return
	}
	// A form is a box, frame and all — sshu's form popup, drawn into the
	// page instead of over it (user, 2026-09-22). The frame is what says
	// where the form starts and stops: a run of aligned rows says they
	// line up, and a box says they belong to each other.
	r.flush()
	savedW, savedIndent, savedLabel := r.textW, r.indent, r.formLabel
	savedBox, savedStack, savedIn := r.boxW, r.formStack, r.inForm
	// As wide as the form needs and no wider. A box across the whole
	// terminal reads as a banner; sshu's is the width of its own rows.
	want, value := formLabelW(n), formValueW(n)
	r.boxW = min(savedW, boxEdge*2+boxPad*2+want+2+value)
	// The wrap width counts the indent, which is the LEFT pad already —
	// subtracting both pads took two cells off every row and wrapped the
	// bed the box had just been sized to hold.
	r.textW = max(12, r.boxW-boxEdge*2-boxPad)
	r.indent += strings.Repeat(" ", boxPad)
	r.formLabel, r.formStack = fitForm(want, value, r.textW-boxPad)
	r.inForm = true
	r.emit(row{box: boxTop, boxW: r.boxW,
		segs: []seg{{text: oneLine(n.Name), item: -1, kind: segDim}}})
	r.boxed++
	r.children(n, depth)
	r.flush()
	r.boxed--
	r.emit(row{box: boxBottom, boxW: r.boxW})
	r.textW, r.indent, r.formLabel = savedW, savedIndent, savedLabel
	r.boxW, r.formStack, r.inForm = savedBox, savedStack, savedIn
	r.gap = true
}

const (
	// boxPad is how far a framed block's content sits from its frame,
	// boxEdge the frame itself, formSlot the bed a value sits on.
	boxPad   = 2
	boxEdge  = 1
	formSlot = 40
	// formValueMin is the narrowest a value column may be and still hold
	// something.
	formValueMin = 8
)

// fitForm settles the label column against the room there actually is,
// and says when there is not enough for one at all.
//
// A terminal's width is not ours to choose (user, 2026-09-22): the same
// form is read at 200 columns and at 60, and a layout that assumes the
// first truncates the labels and wraps the beds at the second. So the
// column gives way in order — its full width, then as much as is left,
// then none at all, at which point the value goes on its own line under
// its label rather than beside it.
func fitForm(want, value, avail int) (int, bool) {
	if want == 0 {
		// No control in this form has a name: there is no column, and
		// nothing to stack a value under either.
		return 0, false
	}
	if want+2+value <= avail {
		return want, false
	}
	return 0, true
}

// formValueW is the value column this form needs: the field glyph, a
// space, the widest value it actually holds, and the caret after it. The
// box is sized by it, so an empty form is not a sliver and a form full
// of long values is not cut to pieces.
func formValueW(form *ir.Node) int {
	w := 0
	form.Walk(func(n *ir.Node) bool {
		switch n.Kind {
		case ir.Textbox:
			w = max(w, dispW(fieldValue(n)))
		case ir.Combobox:
			w = max(w, dispW(oneLine(n.Value)))
		}
		return true
	})
	return dispW(glyphInput) + 1 + clamp(w, formValueMin, formSlot) + 1
}

// formLabelW is how wide a form's label column is: the widest name any
// of its controls has, whole.
//
// It is never cut. A field in a form shows in full or it does not show
// (user, 2026-09-22) — a label with its end missing is a field you
// cannot be sure you have identified, and being unsure which box the
// password goes in is worse than reading two lines. When the whole label
// will not sit beside its value, the row stacks instead (fitForm).
func formLabelW(form *ir.Node) int {
	w := 0
	form.Walk(func(n *ir.Node) bool {
		switch n.Kind {
		case ir.Textbox, ir.Check, ir.Combobox:
			w = max(w, dispW(oneLine(n.Name)))
		}
		return true
	})
	if w == 0 {
		// Not one control has a name — Hacker News writes its "username:"
		// as text in a table cell beside the box, not as a label the AX
		// tree can tie to it. There is nothing to put in a column, and an
		// empty column is an indent for nothing: the form is drawn the
		// way a loose field is.
		return 0
	}
	return w + 2 // room for the required mark
}

// formField draws one control inside a form: its name in the label
// column, dim and NOT an item, then the value against the column's edge,
// which is what the cursor stops on and what Enter opens. The label is
// chrome for the value, and a cursor that stopped on it would have
// nothing to do there.
func (r *renderer) formField(n *ir.Node, id int, value func()) {
	r.flush()
	// A required field says so on its label, the way every form on paper
	// and screen has said it. The page says it in ink webu does not read,
	// so without this the asterisk simply went missing.
	// Never cut: the column was made wide enough for the longest of them,
	// and when it could not be, the row stacks and the label wraps like
	// any other text (fitForm).
	label := oneLine(n.Name)
	if n.Required {
		label += " *"
	}
	// The label is the page's own text, not a hint about it: dim is for a
	// control that cannot be used. And the gap is part of the column
	// rather than a space between words — the flow collapses a space
	// atom, which left the values a cell off from the button's.
	k := segPlain
	if n.Disabled {
		k = segDim
	}
	if r.formStack {
		// No room for a column: the label takes the row and the value
		// the next one, indented under it. Still a form, still aligned —
		// just down the page instead of across it.
		r.words(label, -1, k)
		r.flush()
		r.add(atom{text: "  ", item: -1, kind: segDim})
		value()
		r.flush()
		return
	}
	if r.formLabel > 0 {
		r.add(atom{text: padRight(label, r.formLabel) + "  ", item: -1, kind: k})
	}
	value()
	r.flush()
}

// isSkipLink says whether a link is a skip link — the "Skip to main
// content" an accessible page puts first, for a keyboard to pass the
// header by: a same-page anchor whose text starts with skip or jump to
// (Wikipedia's "Jump to content"), or which the page's class calls one.
func isSkipLink(n *ir.Node) bool {
	if n.Kind != ir.Link || !strings.Contains(n.URL, "#") {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(n.Text()))
	return n.Skip || strings.HasPrefix(text, "skip") || strings.HasPrefix(text, "jump to")
}

// entryTarget is one thing inside a container, with how deeply its lists
// nest — what a table cell's popup lists (app.targetItems).
type entryTarget struct {
	node  *ir.Node
	depth int
}

// entryTargets is what a container holds, in reading order.
func entryTargets(n *ir.Node) []entryTarget {
	var out []entryTarget
	keepSkip := n.Skip // a skip block's links ARE its list
	var walk func(x *ir.Node, lists int)
	walk = func(x *ir.Node, lists int) {
		for _, c := range x.Children {
			switch c.Kind {
			case ir.Link, ir.Button, ir.Textbox, ir.Check, ir.Combobox:
				if !keepSkip && isSkipLink(c) {
					continue // says nothing an entry's list needs
				}
				out = append(out, entryTarget{c, max(0, lists-1)})
			case ir.List:
				walk(c, lists+1)
			default:
				walk(c, lists)
			}
		}
	}
	walk(n, 0)
	return out
}

func countItems(n *ir.Node) int {
	count := 0
	for _, c := range n.Children {
		c.Walk(func(x *ir.Node) bool {
			if x.IsItem() {
				count++
			}
			return true
		})
	}
	return count
}

// ------------------------------------------------------------------ blocks

func (r *renderer) block(n *ir.Node, depth int) {
	if r.suppress {
		// Inside a collapsed heading's section. The heading that ends it
		// is drawn; a wrapper with that heading somewhere inside is walked
		// for it and nothing of its own is drawn; anything else is skipped.
		switch {
		case n.Kind == ir.Heading && n.Level <= r.foldLevel:
			r.suppress = false
		case hasHeadingUpTo(n, r.foldLevel):
			if n.Kind == ir.Landmark {
				r.lmDepth++
			}
			r.children(n, depth)
			if n.Kind == ir.Landmark {
				r.leaveLandmark()
			}
			return
		default:
			return
		}
	}
	switch n.Kind {
	case ir.Document:
		r.children(n, depth)
	case ir.Landmark:
		if n.Skip {
			// A block of skip links — "Skip to:" over a list of anchors
			// — which ir.Build wraps in a navigation of its own. Dropped
			// with the bare skip links (user, 2026-09-22): without a kind
			// of its own it would fold into "nav" and put the page's
			// anchors among its real navigation.
			return
		}
		r.flush()
		if n.Role == "article" && n.ID != r.drill {
			// An article is one thing, the way a list item is: one row
			// until you go into it (user, 2026-09-22).
			r.thing(n)
			return
		}
		if (n.Name == "" || namedByItsHeading(n)) && n.Role != "main" {
			// Nothing to name it with: a rule that says only "▾" is a
			// line across the page for no reason. It is drawn the way
			// main is — its content, in place — and it claims no row,
			// because it has none. It is still marked, so a search hit
			// among its bare children has a row to land on (finder.go).
			//
			// A region named by the heading inside it is the same case:
			// <section aria-labelledby=…> is how a document marks up
			// every section, and a rule saying "Background" over a
			// heading saying "Background" printed the page's outline
			// twice, one row apart (measured on Wikipedia, 2026-09-23).
			r.lmDepth++
			r.markNext = append(r.markNext, n)
			r.formChildren(n, depth)
			r.flush()
			r.leaveLandmark()
			r.gap = true
			return
		}
		r.markNext = append(r.markNext, n)
		if n.Role == "main" {
			// The page itself: no rule, no item. With the chrome on the
			// pagetab, main is where the page starts (2026-09-22).
			r.lmDepth++
			r.children(n, depth)
			r.flush()
			r.leaveLandmark()
			r.gap = true
			return
		}
		folded := r.folded(n)
		id := r.newItem(n)
		r.items[id].folded = folded
		r.landmarkRule(n, id, folded)
		if folded {
			r.gap = true
			return
		}
		r.lmDepth++
		r.formChildren(n, depth)
		r.flush()
		r.leaveLandmark()
		r.gap = true
	case ir.Heading:
		r.flush()
		r.markNext = append(r.markNext, n)
		id := -1
		if !hasItem(n) {
			id = r.newItem(n)
		}
		// A heading that is an item collapses on Enter, like a landmark's
		// row: the row keeps the heading and says what it hides.
		r.head = r.depthOf(clamp(n.Level, 1, 6))
		folded := id >= 0 && r.fold[n.ID]
		if folded {
			r.items[id].folded = true
			r.add(atom{text: "▸", item: id, kind: segHeading})
			r.add(atom{text: " ", item: id, kind: segHeading, space: true})
		}
		r.add(atom{text: strings.Repeat("#", clamp(n.Level, 1, 6)), item: id, kind: segHeading})
		r.add(atom{text: " ", item: id, kind: segHeading, space: true})
		r.inlineChildren(n, id, segHeading)
		if folded {
			if c := sectionItems(r.root, n); c > 0 {
				r.add(atom{text: " ", item: id, kind: segDim, space: true})
				r.add(atom{text: "· " + plural(c, "item"), item: id, kind: segDim})
			}
		}
		r.flush()
		r.head = 0
		r.gap = true
		if folded {
			r.suppress, r.foldLevel, r.foldLm = true, n.Level, r.lmDepth
		}
	case ir.Paragraph:
		r.flush()
		r.markNext = append(r.markNext, n)
		r.inlineChildren(n, -1, segPlain)
		r.flush()
		r.gap = true
	case ir.List:
		r.flush()
		r.children(n, depth)
		r.flush()
		if depth <= 1 {
			r.gap = true
		}
	case ir.ListItem:
		r.flush()
		r.markNext = append(r.markNext, n)
		marker := n.Marker
		if marker == "" {
			marker = "  "
		}
		saved := r.indent
		r.lead = r.indent + marker
		r.indent += strings.Repeat(" ", dispW(marker))
		if n.ID != r.drill {
			r.thing(n)
			r.indent = saved
			return
		}
		r.inlineChildren(n, -1, segPlain)
		r.flush()
		r.indent = saved
	case ir.Table:
		r.flush()
		r.markNext = append(r.markNext, n)
		if hasHeader(n) {
			r.table(n)
			r.gap = true
		} else {
			// No header cell anywhere: a table used for layout that Chromium
			// still called a table (Hacker News). Columns would squeeze the
			// one column that matters; each row is one line of flow instead
			// (function.md §3 heuristics).
			for _, tr := range n.Children {
				r.flush()
				r.markNext = append(r.markNext, tr)
				// One line per row, wrapped: the cells and whatever blocks
				// they hold flatten into it, the same as inside a data cell.
				r.inCell = true
				r.inlineChildren(tr, -1, segPlain)
				r.inCell = false
				r.flush()
			}
		}
	case ir.Row:
		// A row outside a table (a grid we did not recognise as one): one
		// line, cells run together.
		r.flush()
		r.markNext = append(r.markNext, n)
		r.inlineChildren(n, -1, segPlain)
		r.flush()
	case ir.Separator:
		r.flush()
		r.emit(row{segs: []seg{{text: r.indent + strings.Repeat("─", clamp(r.width-dispW(r.indent), 1, 40)), item: -1, kind: segDim}}})
		r.gap = true
	case ir.Quote:
		r.flush()
		r.markNext = append(r.markNext, n)
		saved := r.indent
		r.indent += "│ "
		r.inlineChildren(n, -1, segPlain)
		r.flush()
		r.indent = saved
		if depth <= 1 {
			r.gap = true
		}
	case ir.Group:
		r.flush()
		r.markNext = append(r.markNext, n)
		r.inlineChildren(n, -1, segPlain)
		r.flush()
	case ir.Code:
		r.flush()
		r.markNext = append(r.markNext, n)
		r.codeBlock(n)
		r.gap = true
	case ir.Unsupported:
		r.flush()
		r.markNext = append(r.markNext, n)
		id := -1
		if n.Focusable {
			id = r.newItem(n)
		}
		r.add(atom{text: glyphUnsupported + " " + n.Role, item: id, kind: segUnsupported})
		if n.Name != "" {
			r.add(atom{text: " ", item: id, kind: segUnsupported, space: true})
			r.words(n.Name, id, segUnsupported)
		}
		r.inlineChildren(n, -1, segPlain)
		r.flush()
	default:
		// An inline kind asked to be a block (Block flag): its own line.
		r.flush()
		r.inline(n, -1, segPlain)
		r.flush()
	}
}

// namedByItsHeading reports whether a region takes its name from the
// first heading inside it — <section aria-labelledby=…> — so that the
// name is drawn once, on the heading, and not again on a rule above it.
func namedByItsHeading(n *ir.Node) bool {
	if n.Kind != ir.Landmark || n.Name == "" {
		return false
	}
	name := oneLine(n.Name)
	var first *ir.Node
	n.Walk(func(x *ir.Node) bool {
		if x != n && x.Kind == ir.Heading && first == nil {
			first = x
		}
		return first == nil
	})
	return first != nil && oneLine(first.Name) == name
}

func (r *renderer) children(n *ir.Node, depth int) {
	inRun := linkRun(n.Children)
	for i, c := range n.Children {
		switch {
		case inRun[i]:
			// A run of bare links the page laid out one per line — a
			// sidebar of them, a menu — flows like words instead
			// (2026-09-22). webu does not do CSS layout, and a column
			// of 120 one-word rows is the page's styling, not its
			// content; flowed, nothing is hidden and nothing is guessed.
			if !r.suppress {
				r.inline(c, -1, segPlain)
			}
		case c.IsBlock():
			r.block(c, depth+1)
		case !r.suppress:
			r.inline(c, -1, segPlain)
		}
	}
}

// bareBlockLink is a link the page put on a line of its own with nothing
// blocky inside it: a link whose row is CSS, not structure.
func bareBlockLink(n *ir.Node) bool {
	if n.Kind != ir.Link || !n.Block {
		return false
	}
	for _, c := range n.Children {
		if c.IsBlock() {
			return false
		}
	}
	return true
}

// linkRun marks the children that belong to a run of two or more bare
// block links, blank text between them not counting as a break. One such
// link on its own keeps its line: it is a button-like thing, not a list.
func linkRun(kids []*ir.Node) []bool {
	out := make([]bool, len(kids))
	run := []int{}
	flush := func() {
		if len(run) > 1 {
			for _, i := range run {
				out[i] = true
			}
		}
		run = run[:0]
	}
	for i, c := range kids {
		switch {
		case bareBlockLink(c):
			run = append(run, i)
		case c.Kind == ir.Text && strings.TrimSpace(c.Name) == "" && len(run) > 0:
			// A blank between two of them is the page's own spacing.
		default:
			flush()
		}
	}
	flush()
	return out
}

// leaveLandmark is the end of a landmark's children: a collapsed
// heading's section cannot outlive the landmark it began in.
func (r *renderer) leaveLandmark() {
	r.lmDepth--
	if r.suppress && r.lmDepth < r.foldLm {
		r.suppress = false
	}
}

// hasHeadingUpTo reports whether a heading of level or higher is inside n.
func hasHeadingUpTo(n *ir.Node, level int) bool {
	found := false
	for _, c := range n.Children {
		c.Walk(func(x *ir.Node) bool {
			if x.Kind == ir.Heading && x.Level <= level {
				found = true
			}
			return !found
		})
	}
	return found
}

// walkSection visits, in reading order, what a heading's section holds:
// everything after h up to the next heading of its level or higher, or the
// end of the landmark h is in. fn returning false stops the walk.
func walkSection(root, h *ir.Node, fn func(*ir.Node) bool) {
	started, done, hLm := false, false, 0
	var walk func(n *ir.Node, lm int)
	walk = func(n *ir.Node, lm int) {
		if done {
			return
		}
		if n == h {
			started, hLm = true, lm
			return
		}
		if started {
			if n.Kind == ir.Heading && n.Level <= h.Level {
				done = true
				return
			}
			if !fn(n) {
				done = true
				return
			}
		}
		inner := lm
		if n.Kind == ir.Landmark {
			inner++
		}
		for _, c := range n.Children {
			walk(c, inner)
			if done {
				return
			}
		}
		if started && n.Kind == ir.Landmark && lm < hLm {
			done = true
		}
	}
	walk(root, 0)
}

// sectionItems counts the items a collapsed heading hides.
func sectionItems(root, h *ir.Node) int {
	count := 0
	walkSection(root, h, func(n *ir.Node) bool {
		if n.IsItem() {
			count++
		}
		return true
	})
	return count
}

// sectionHolds reports whether n is inside heading h's section.
func sectionHolds(root, h, n *ir.Node) bool {
	held := false
	walkSection(root, h, func(x *ir.Node) bool {
		held = x == n
		return !held
	})
	return held
}

// inlineChildren flows n's children; a block child inside a flow breaks it
// — except inside a table cell, where it flattens (see inCell).
func (r *renderer) inlineChildren(n *ir.Node, item int, kind segKind) {
	for _, c := range n.Children {
		if c.IsBlock() {
			if r.inCell {
				r.space()
				r.inlineChildren(c, item, kind)
				r.space()
				continue
			}
			r.block(c, 2)
			continue
		}
		r.inline(c, item, kind)
	}
}

// hasHeader reports whether any cell of the table is a header cell.
func hasHeader(t *ir.Node) bool {
	found := false
	t.Walk(func(n *ir.Node) bool {
		if n.Kind == ir.Cell && n.Header {
			found = true
		}
		return !found
	})
	return found
}

func hasItem(n *ir.Node) bool {
	found := false
	for _, c := range n.Children {
		c.Walk(func(x *ir.Node) bool {
			if x.IsItem() {
				found = true
			}
			return !found
		})
	}
	return found
}

// ------------------------------------------------------------------ inline

// itemOf is the item an interactive node draws as: its own, or the cell
// it sits in — inside a data table's cell everything is one stop, the
// cell (table); what the cell holds is behind it (app enterCell).
func (r *renderer) itemOf(n *ir.Node) int {
	if r.cellItem >= 0 {
		return r.cellItem
	}
	return r.newItem(n)
}

// inline adds one inline node to the flow. item/kind are inherited from an
// enclosing item (a heading's text) and overridden by the node's own.
func (r *renderer) inline(n *ir.Node, item int, kind segKind) {
	switch n.Kind {
	case ir.Text:
		r.words(n.Name, item, kind)
	case ir.Span:
		// The page's own markup: everything inside is drawn with one more
		// attribute on it, whatever else it is (2026-09-22).
		saved := r.attr
		r.attr |= attrOf(n.Role)
		r.inlineChildren(n, item, kind)
		r.attr = saved
	case ir.Link:
		if isSkipLink(n) && !r.inCell && r.cellItem < 0 {
			// Dropped, not drawn and not filed anywhere (user,
			// 2026-09-22). A skip link exists to jump a screen reader
			// past the navigation to the content — and webu has already
			// taken the navigation off the page and started the cursor
			// at main. It is furniture for a problem this browser does
			// not have.
			return
		}
		id := r.itemOf(n)
		name := n.Name
		if name == "" {
			name = linkFallback(n.URL)
		}
		r.add(atom{text: glyphLink + " ", item: id, kind: segLink})
		r.words(name, id, segLink)
	case ir.Button:
		id := r.itemOf(n)
		k := segButton
		if n.Disabled {
			k = segDim
		}
		if r.inForm {
			// A form's own button lines up under the values it acts on,
			// not under their labels: it answers the column, it does not
			// name a row of it.
			r.flush()
			// Not a space atom: a run of spaces marked as space is
			// trimmed off the head of a line, which is right for flow
			// and wrong for a column.
			r.add(atom{text: strings.Repeat(" ", r.formLabel+2), item: -1, kind: segDim})
		}
		r.add(atom{text: glyphButton + " ", item: id, kind: k})
		r.words(oneLine(nameOr(n.Name, n.Value)), id, k)
	case ir.Textbox:
		r.dropLabel(n.Name)
		id := r.itemOf(n)
		if r.inForm {
			r.formField(n, id, func() {
				r.add(atom{text: glyphInput + " ", item: id, kind: segInput})
				// The slot recedes and the value stands in it. sshu's form
				// draws no bed at all — the alignment is what says a value
				// goes here — but it has placeholders to put in an empty
				// one and the AX tree gives webu none, so an empty field
				// would be an empty row. The bed is the least that can be
				// there: dim, so it is the floor and not the furniture
				// (user, 2026-09-22 — mauve underscores were the loudest
				// thing on the page).
				if v := fieldValue(n); v != "" {
					r.words(v, id, segInput)
					r.add(atom{text: " ", item: id, kind: segInput, space: true})
				}
				r.add(atom{text: " ", item: id, kind: segCaret})
			})
			break
		}
		r.add(atom{text: glyphInput + " ", item: id, kind: segInput})
		if n.Name != "" {
			r.words(n.Name, id, segInput)
			r.add(atom{text: " ", item: id, kind: segInput, space: true})
		}
		if v := fieldValue(n); v != "" {
			r.words(v, id, segInput)
			r.add(atom{text: " ", item: id, kind: segInput, space: true})
		}
		r.add(atom{text: " ", item: id, kind: segCaret})
	case ir.Check:
		r.dropLabel(n.Name)
		id := r.itemOf(n)
		if r.inForm {
			// The box alone in the value column: the name is already in
			// the label column, and saying it twice on one row is the
			// thing the column was for.
			r.formField(n, id, func() {
				r.add(atom{text: checkText(n), item: id, kind: segCheck})
			})
			break
		}
		r.add(atom{text: checkText(n) + " ", item: id, kind: segCheck})
		r.words(n.Name, id, segCheck)
	case ir.Combobox:
		r.dropLabel(n.Name)
		id := r.itemOf(n)
		if r.inForm {
			r.formField(n, id, func() {
				r.add(atom{text: glyphSelect + " ", item: id, kind: segInput})
				r.words(oneLine(n.Value), id, segInput)
			})
			break
		}
		if n.Name != "" {
			r.words(n.Name, id, segInput)
			r.add(atom{text: " ", item: id, kind: segInput, space: true})
		}
		r.add(atom{text: glyphSelect + " ", item: id, kind: segInput})
		r.words(oneLine(n.Value), id, segInput)
	case ir.Media:
		id := r.itemOf(n)
		g := mediaGlyph(n)
		if label, value, ok := labelled(oneLine(n.Name)); ok {
			// The name is all this node has, and it is a field: an icon
			// whose alt reads "Priority: Highest" is carrying a value,
			// not describing a picture.
			r.add(atom{text: g + " ", item: id, kind: segMedia})
			r.add(atom{text: label, item: id, kind: segDim})
			r.add(atom{text: "  ", item: id, kind: segDim, space: true})
			r.words(value, id, segPlain)
			break
		}
		r.add(atom{text: mediaText(n), item: id, kind: segMedia})
	case ir.Code:
		if strings.Contains(n.Text(), "\n") && !r.inCell {
			r.flush()
			r.codeBlock(n)
			r.flush()
			return
		}
		r.words(n.Text(), item, segCode)
	case ir.Cell:
		r.space()
		r.inlineChildren(n, item, kind)
	case ir.Unsupported:
		id := item
		if n.Focusable {
			id = r.itemOf(n)
		}
		r.add(atom{text: glyphUnsupported + " " + n.Role, item: id, kind: segUnsupported})
		if n.Name != "" {
			r.add(atom{text: " ", item: id, kind: segUnsupported, space: true})
			r.words(n.Name, id, segUnsupported)
		}
		r.inlineChildren(n, id, kind)
	case ir.Option:
		// Only ever drawn by the options popup.
	default:
		if n.IsBlock() && !r.inCell {
			r.block(n, 2)
			return
		}
		r.inlineChildren(n, item, kind)
	}
}

// words splits text into breakable atoms: words, and one space between
// them. Newlines inside text (a <br>, a pre) force a line break.
func (r *renderer) words(text string, item int, kind segKind) {
	lines := strings.Split(text, "\n")
	for li, line := range lines {
		if li > 0 {
			if r.inCell {
				r.space()
			} else {
				r.flushKeepIndent()
			}
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			if line != "" && len(r.flow) > 0 {
				r.add(atom{text: " ", item: item, kind: kind, space: true})
			}
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			r.add(atom{text: " ", item: item, kind: kind, space: true})
		}
		for i, f := range fields {
			if i > 0 {
				r.add(atom{text: " ", item: item, kind: kind, space: true})
			}
			r.add(atom{text: f, item: item, kind: kind})
		}
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			r.add(atom{text: " ", item: item, kind: kind, space: true})
		}
	}
}

func (r *renderer) add(a atom) {
	a.attr |= r.attr
	// Two spaces in a row collapse, the way HTML collapses them.
	if a.space && len(r.flow) > 0 && r.flow[len(r.flow)-1].space {
		return
	}
	// Two items back to back get a space between them. The AX tree drops
	// the whitespace HTML had between two <label>s or two <a>s, and without
	// it the fields read as one word.
	//
	// The same for a word that follows an item: "Filled" after a field's
	// underscores was one word too. Only a word — punctuation such as the ")"
	// closing a link stays where the author put it.
	if !a.space && len(r.flow) > 0 {
		prev := r.flow[len(r.flow)-1]
		switch {
		case prev.space || prev.item < 0:
		case a.item >= 0 && prev.item != a.item,
			a.item < 0 && startsWord(a.text):
			r.flow = append(r.flow, atom{text: " ", item: -1, kind: segPlain, space: true})
		}
	}
	r.flow = append(r.flow, a)
}

// dropLabel removes the text just before a field when it IS the field's
// name. A <label> wrapping an input leaves its text in the flow AND gives
// the input that text as its accessible name, so without this every field
// reads "Name 󰛿 Name ____". Only plain text is eaten — never another item.
func (r *renderer) dropLabel(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	start := len(r.flow)
	for start > 0 && r.flow[start-1].item < 0 {
		start--
	}
	var parts []string
	for _, a := range r.flow[start:] {
		if !a.space {
			parts = append(parts, a.text)
		}
	}
	trailing := strings.Join(parts, " ")
	if strings.TrimSpace(trailing) == name {
		r.flow = r.flow[:start]
		return
	}
	if len(r.flow) == 0 {
		r.dropLabelRow(name)
	}
}

// dropLabelRow is the same for a label the page laid out as a block, so
// it was flushed to a row of its own before the field arrived. GitHub's
// sign-in draws its labels that way, and webu printed each of them twice
// — once as the page's text, once as the field's accessible name, which
// IS that label.
//
// Only the row just emitted, only when it is entirely text no item is on,
// and only when nothing points at it: a row index is held by the items
// that span it and by the marks that open on it, and shifting those to
// save a duplicate is not a trade worth making.
func (r *renderer) dropLabelRow(name string) {
	at := len(r.rows) - 1
	if at < 0 || strings.TrimSpace(r.rows[at].plain()) != name {
		return
	}
	for _, sg := range r.rows[at].segs {
		if sg.item >= 0 {
			return
		}
	}
	for _, it := range r.items {
		if it.first <= at && at <= it.last {
			return
		}
	}
	// A landmark's or a heading's row is what the outline opens on and
	// stays. A block's own mark — the label's paragraph — is the thing
	// being dropped, and goes with it.
	for n, mark := range r.marks {
		if mark != at {
			continue
		}
		if n.Kind == ir.Landmark || n.Kind == ir.Heading {
			return
		}
	}
	for n, mark := range r.marks {
		if mark == at {
			delete(r.marks, n)
		}
	}
	r.rows = r.rows[:at]
}

// linkFallback names a link that has no name (an image with no alt, an
// empty div with a click handler): the host for a root URL, else the last
// path segment. The whole URL is what the Inspect popup is for.
func linkFallback(u string) string {
	if u == "" {
		return "link"
	}
	rest := u
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimRight(rest, "/")
	if i := strings.LastIndexByte(rest, '/'); i >= 0 {
		rest = rest[i+1:]
	}
	if rest == "" {
		return "link"
	}
	return rest
}

func startsWord(s string) bool {
	for _, c := range s {
		return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c > 0x7f
	}
	return false
}

// space adds a breaking space, unless the flow is empty or already ends in one.
func (r *renderer) space() {
	if len(r.flow) > 0 {
		r.add(atom{text: " ", item: -1, kind: segPlain, space: true})
	}
}

func (r *renderer) newItem(n *ir.Node) int {
	r.items = append(r.items, item{node: n, first: -1, last: -1})
	return len(r.items) - 1
}

// fieldValue is what a textbox shows: its value, masked when it is a
// password. The glyph in front has already said it is a field, so there
// is no bed and no dashes — just the value, and one lit cell after it
// for where the next character would go (render, user 2026-09-22).
func fieldValue(n *ir.Node) string {
	v := oneLine(n.Value)
	if n.Protected && v != "" {
		v = strings.Repeat("•", min(12, len([]rune(n.Value))))
	}
	return v
}

// checkText is the box or the dot, as a glyph: the state IS the glyph,
// so a check box leads its name the way every other control does.
func checkText(n *ir.Node) string {
	if n.Role == "radio" {
		if n.Checked == ir.On {
			return glyphRadioOn
		}
		return glyphRadioOff
	}
	switch n.Checked {
	case ir.On:
		return glyphCheckOn
	case ir.Mixed:
		return glyphCheckMixed
	}
	return glyphCheckOff
}

func mediaText(n *ir.Node) string {
	name := oneLine(n.Name)
	if name == "" {
		name = "no alt"
	}
	return mediaGlyph(n) + " " + name
}

func mediaGlyph(n *ir.Node) string {
	switch n.Role {
	case "Video":
		return glyphVideo
	case "Audio":
		return glyphAudio
	case "Iframe":
		return glyphFrame
	case "Canvas":
		return glyphCanvas
	}
	return glyphImage
}

// labelled splits a name shaped "Label: value" — the label dim, the value
// in the ink of whatever it is (2026-09-22).
//
// This is only ever asked of a node that has no text of its own, so its
// NAME is the whole of what it carries: an icon, an empty group. Authors
// write those as "Priority: Highest", "Assignee: vulcanshen", "Due date:
// Sep 8, 2026" because a screen reader has nothing else to go on — the
// label is deliberate, not a coincidence of punctuation. Prose is never
// put through this: a paragraph that begins "Note: " is a sentence.
//
// A label is short and is not itself a sentence. Anything else is a name
// that happens to contain a colon, and is left alone.
func labelled(name string) (string, string, bool) {
	i := strings.Index(name, ": ")
	if i <= 0 || i > 24 {
		return "", "", false
	}
	label, value := name[:i], strings.TrimSpace(name[i+2:])
	if value == "" || strings.ContainsAny(label, ".,;!?") {
		return "", "", false
	}
	return label, value, true
}

// oneLine folds a value into one line for a field or a chip: the first line,
// with an ellipsis when there is more.
func oneLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + "…"
	}
	return strings.TrimSpace(s)
}

// ----------------------------------------------------------------- blocks

// codeBlock draws a code node's lines verbatim: no wrapping, clipped at the
// width, because a wrapped line of code reads as two lines of code.
func (r *renderer) codeBlock(n *ir.Node) {
	// A line wider than the block folds onto the next row at the block's
	// width — the text measure, which is also where its ground stops —
	// rather than being cut: a cut line of code or JSON is a line lost.
	width := max(1, r.textW-dispW(r.indent))
	text := strings.ReplaceAll(n.Text(), "\t", "    ")
	lines := highlightLines(n.Lang, text)
	if lines == nil {
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, []seg{{text: line, item: -1, kind: segCode}})
		}
	}
	for _, segs := range lines {
		for _, fold := range foldSegs(segs, width) {
			r.emit(row{segs: append([]seg{{text: r.indent, item: -1, kind: segCode}}, fold...), code: true})
		}
	}
}

// foldSegs breaks one line of segments into rows of at most width cells,
// cutting a segment where it must. An empty line is one empty row.
func foldSegs(segs []seg, width int) [][]seg {
	var out [][]seg
	var line []seg
	used := 0
	for _, s := range segs {
		text := s.text
		for text != "" {
			room := width - used
			if room <= 0 {
				out = append(out, line)
				line, used = nil, 0
				room = width
			}
			head := truncateNoEllipsis(text, room)
			if head == "" {
				head = text // one cell wider than the row: let it be
			}
			line = append(line, seg{text: head, item: -1, kind: s.kind})
			used += dispW(head)
			text = strings.TrimPrefix(text, head)
		}
	}
	return append(out, line)
}

// table draws one screen row per table row, cells padded to their column's
// width. Widths are the widest cell, shrunk together when the table is wider
// than the panel: the widest column gives first, the way sshu's tables do.
// A cell cut to its column is the normal case, so every cell is a stop —
// the one place h/l and j/k all move — and Enter shows it in full
// (2026-09-21). The rows sit on the table's ground, the header row on
// the deeper one.
func (r *renderer) table(n *ir.Node) {
	if n.Name != "" {
		r.emit(row{segs: []seg{{text: r.indent + n.Name, item: -1, kind: segDim}}})
	}
	type cell struct {
		segs []seg
		w    int
		id   int // the cell's item, on its padding too: an empty cell is a stop
	}
	var grid [][]cell
	var header []bool
	var widths []int
	for _, tr := range n.Children {
		if tr.Kind != ir.Row {
			continue
		}
		var cells []cell
		allHeader := true
		for _, td := range tr.Children {
			if td.Kind != ir.Cell {
				continue
			}
			id := r.newItem(td)
			segs := r.cellSegs(td, id)
			w := 0
			for _, s := range segs {
				w += dispW(s.text)
			}
			cells = append(cells, cell{segs, w, id})
			allHeader = allHeader && td.Header
			if len(widths) < len(cells) {
				widths = append(widths, 0)
			}
			widths[len(cells)-1] = max(widths[len(cells)-1], w)
		}
		grid = append(grid, cells)
		header = append(header, allHeader && len(cells) > 0)
	}
	if len(widths) == 0 {
		return
	}
	avail := r.width - dispW(r.indent) - 2*(len(widths)-1)
	for total := sum(widths); total > avail && total > 0; total = sum(widths) {
		widest := 0
		for i, w := range widths {
			if w > widths[widest] {
				widest = i
			}
		}
		if widths[widest] <= 4 {
			break
		}
		widths[widest]--
	}
	for ri, cells := range grid {
		out := []seg{{text: r.indent, item: -1, kind: segPlain}}
		for i, c := range cells {
			if i > 0 {
				out = append(out, seg{text: "  ", item: -1, kind: segPlain})
			}
			for _, s := range fitSegs(c.segs, widths[i]) {
				s.item = c.id
				out = append(out, s)
			}
		}
		r.emit(row{segs: out, table: true, header: header[ri]})
	}
}

// cellSegs is a cell's inline content as segments (no wrapping inside a
// cell: a table row is one screen row), every one of them the cell's
// item id: the cell is the stop, whatever it holds.
func (r *renderer) cellSegs(td *ir.Node, id int) []seg {
	saved, wasCell, wasItem := r.flow, r.inCell, r.cellItem
	r.flow, r.inCell, r.cellItem = nil, true, id
	defer func() { r.inCell, r.cellItem = wasCell, wasItem }()
	kind := segPlain
	if td.Header {
		kind = segTableHeader
	}
	r.inlineChildren(td, id, kind)
	var out []seg
	for i, a := range r.flow {
		if a.space && (i == 0 || i == len(r.flow)-1) {
			continue
		}
		out = append(out, seg{text: a.text, item: id, kind: a.kind})
	}
	r.flow = saved
	return out
}

// columnHeader is the header cell above a cell, by column, for the
// title of the cell's popup; "" when the column has none.
func columnHeader(root, cell *ir.Node) string {
	var table *ir.Node
	col := -1
	root.Walk(func(x *ir.Node) bool {
		if table != nil || x.Kind != ir.Table {
			return table == nil
		}
		for _, tr := range x.Children {
			if tr.Kind != ir.Row {
				continue
			}
			i := 0
			for _, td := range tr.Children {
				if td.Kind != ir.Cell {
					continue
				}
				if td == cell {
					table, col = x, i
					return false
				}
				i++
			}
		}
		return true
	})
	if table == nil {
		return ""
	}
	for _, tr := range table.Children {
		if tr.Kind != ir.Row {
			continue
		}
		i := 0
		for _, td := range tr.Children {
			if td.Kind != ir.Cell {
				continue
			}
			if i == col && td.Header {
				return oneLine(td.Text())
			}
			i++
		}
	}
	return ""
}

// fitSegs pads or clips segments to exactly w cells.
func fitSegs(segs []seg, w int) []seg {
	out := make([]seg, 0, len(segs)+1)
	used := 0
	for _, s := range segs {
		sw := dispW(s.text)
		if used+sw > w {
			s.text = truncate(s.text, w-used)
			out = append(out, s)
			used = w
			break
		}
		out = append(out, s)
		used += sw
	}
	if used < w {
		out = append(out, seg{text: strings.Repeat(" ", w-used), item: -1, kind: segPlain})
	}
	return out
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

// ------------------------------------------------------------------ flow

// flush wraps the pending flow into rows and clears it.
func (r *renderer) flush() {
	r.wrap()
	r.lead = ""
}

// flushKeepIndent ends the current line inside a block (a <br>) without
// ending the block.
func (r *renderer) flushKeepIndent() {
	if len(r.flow) == 0 {
		r.emit(row{segs: []seg{{text: r.prefix(), item: -1, kind: segPlain}}})
		return
	}
	r.wrap()
}

func (r *renderer) prefix() string {
	if r.lead != "" {
		p := r.lead
		r.lead = ""
		return p
	}
	return r.indent
}

func (r *renderer) wrap() {
	flow := r.flow
	r.flow = nil
	// Trim breaking spaces at both ends.
	for len(flow) > 0 && flow[0].space {
		flow = flow[1:]
	}
	for len(flow) > 0 && flow[len(flow)-1].space {
		flow = flow[:len(flow)-1]
	}
	if len(flow) == 0 {
		return
	}
	line := []seg{}
	prefix := r.prefix()
	used := dispW(prefix)
	line = append(line, seg{text: prefix, item: -1, kind: segPlain})
	avail := func() int { return r.textW - used }
	emitLine := func() {
		r.emit(row{segs: line})
		prefix = r.prefix()
		used = dispW(prefix)
		line = []seg{{text: prefix, item: -1, kind: segPlain}}
	}
	// A space is not placed until the word after it is: if the two do not
	// fit together the line breaks BEFORE the word, and the space is what
	// the break consumes. Placing it eagerly and dropping it when it did not
	// fit let the next word land flush against the previous one.
	pending := false
	var pendingItem int
	var pendingKind segKind
	var pendingAttr textAttr
	for i := 0; i < len(flow); i++ {
		a := flow[i]
		if a.space {
			if used > dispW(prefix) {
				pending, pendingItem, pendingKind, pendingAttr = true, a.item, a.kind, a.attr
			}
			continue
		}
		w := dispW(a.text)
		need := w
		if pending {
			need++
		}
		if used+need > r.textW && used > dispW(prefix) {
			emitLine()
			pending = false
		}
		if pending {
			line = append(line, seg{text: " ", item: pendingItem, kind: pendingKind, attr: pendingAttr})
			used++
			pending = false
		}
		text := a.text
		for dispW(text) > avail() && avail() > 0 {
			// A word wider than the line: hard-split it.
			head := truncateNoEllipsis(text, avail())
			line = append(line, seg{text: head, item: a.item, kind: a.kind, attr: a.attr})
			text = strings.TrimPrefix(text, head)
			emitLine()
		}
		line = append(line, seg{text: text, item: a.item, kind: a.kind, attr: a.attr})
		used += dispW(text)
	}
	if len(line) > 1 {
		r.emit(row{segs: line})
	}
}

// truncateNoEllipsis cuts s to at most w cells with no marker.
func truncateNoEllipsis(s string, w int) string {
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := dispW(string(r))
		if used+rw > w {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

// emit appends a row, paying any gap owed first. A gap is never the first
// row of the page and never doubles.
func (r *renderer) emit(rw row) {
	if r.gap {
		r.gap = false
		if len(r.rows) > 0 && r.rows[len(r.rows)-1].plain() != "" {
			r.rows = append(r.rows, row{})
		}
	}
	if r.head > 0 {
		rw.heading = r.head
	}
	if r.boxed > 0 && rw.box == boxNone {
		rw.box, rw.boxW = boxSide, r.boxW
	}
	at := len(r.rows)
	r.rows = append(r.rows, rw)
	col := 0
	for _, s := range rw.segs {
		r.touch(s.item, at, col)
		col += dispW(s.text)
	}
	for _, n := range r.markNext {
		r.marks[n] = at
	}
	r.markNext = r.markNext[:0]
}

// touch records that item is on row at, starting at col the first time.
// Only emit calls it: a row index guessed before the row exists is wrong
// whenever a gap is owed.
func (r *renderer) touch(item, at, col int) {
	if item < 0 || item >= len(r.items) {
		return
	}
	it := &r.items[item]
	if it.first < 0 {
		it.first, it.col = at, col
	}
	it.last = at
}
