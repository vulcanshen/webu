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
}

type row struct {
	segs []seg
	// code: a row of a code block, drawn on the code background across
	// the row, padding included.
	code bool
}

// plain is the row's text with no styling — what tests and the search read.
func (r row) plain() string {
	var b strings.Builder
	for _, s := range r.segs {
		b.WriteString(s.text)
	}
	return b.String()
}

type item struct {
	node        *ir.Node
	first, last int  // row span, inclusive
	col         int  // cell column where it starts on its first row: h/l walk a row by it
	folded      bool // a landmark drawn as one line, its content hidden
}

type layout struct {
	rows  []row
	items []item
	// marks is the first row of every landmark and heading, for the
	// outline to jump to (ui.md §3.1).
	marks map[*ir.Node]int
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
	space bool // a breaking space: dropped at a line end
}

// renderOpts is what a layout depends on besides the tree.
type renderOpts struct {
	width int
	// measure caps how wide text flows (ui.md revision 2026-09-20): a
	// paragraph across 200 cells cannot be read back to its start. Tables,
	// code and rules still take the width. 0 is no cap.
	measure int
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
	// inNav counts navigations being entered: their lists flow inline.
	inNav int
	// inNavList: a navigation list is flowing on one line, so an entry the
	// page styled as a block flows too.
	inNavList bool
	indent    string // prefix every wrapped line of the current block gets
	lead      string // prefix the FIRST line gets instead of indent (a list marker); consumed by the next flush
	gap       bool   // a blank row is owed before the next content row
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
	r := &renderer{width: max(1, o.width), textW: max(1, o.width), fold: o.fold, marks: map[*ir.Node]int{}}
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
	label := " " + glyph + " " + n.Role
	if n.Name != "" {
		label += " " + oneLine(n.Name)
	}
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

// navList says whether a list is a row of short entries — the kind a
// navigation is made of — that reads better as one line than as a column.
// A link the page styled display:block is still a short entry: the
// column was the page's CSS, not its structure.
func navList(n *ir.Node) bool {
	for _, li := range n.Children {
		if li.Kind != ir.ListItem {
			return false
		}
		for _, c := range li.Children {
			if c.IsBlock() && !inlineKind(c) {
				return false
			}
		}
		if dispW(li.Text()) > 40 {
			return false
		}
	}
	return len(n.Children) > 0
}

// inlineKind is a node that flows by nature, whatever the page's CSS said.
func inlineKind(n *ir.Node) bool {
	switch n.Kind {
	case ir.Text, ir.Link, ir.Button, ir.Textbox, ir.Check, ir.Combobox, ir.Media, ir.Code:
		return true
	}
	return false
}

// ------------------------------------------------------------------ blocks

func (r *renderer) block(n *ir.Node, depth int) {
	switch n.Kind {
	case ir.Document:
		r.children(n, depth)
	case ir.Landmark:
		r.flush()
		r.markNext = append(r.markNext, n)
		folded := r.folded(n)
		id := r.newItem(n)
		r.items[id].folded = folded
		r.landmarkRule(n, id, folded)
		if folded {
			r.gap = true
			return
		}
		if n.Role == "navigation" {
			r.inNav++
		}
		r.children(n, depth)
		r.flush()
		if n.Role == "navigation" {
			r.inNav--
		}
		r.gap = true
	case ir.Heading:
		r.flush()
		r.markNext = append(r.markNext, n)
		id := -1
		if !hasItem(n) {
			id = r.newItem(n)
		}
		r.add(atom{text: strings.Repeat("#", clamp(n.Level, 1, 6)), item: id, kind: segHeading})
		r.add(atom{text: " ", item: id, kind: segHeading, space: true})
		r.inlineChildren(n, id, segHeading)
		r.flush()
		r.gap = true
	case ir.Paragraph:
		r.flush()
		r.inlineChildren(n, -1, segPlain)
		r.flush()
		r.gap = true
	case ir.List:
		r.flush()
		if r.inNav > 0 && navList(n) {
			// A navigation's list of short entries is one line of flow —
			// Platform · Solutions · Resources — not a column: the column
			// was costing a screen before the page began.
			first := true
			r.inNavList = true
			for _, li := range n.Children {
				if !first {
					r.space()
					r.add(atom{text: "·", item: -1, kind: segDim})
					r.space()
				}
				first = false
				r.inlineChildren(li, -1, segPlain)
			}
			r.inNavList = false
			r.flush()
			if depth <= 1 {
				r.gap = true
			}
			return
		}
		r.children(n, depth)
		r.flush()
		if depth <= 1 {
			r.gap = true
		}
	case ir.ListItem:
		r.flush()
		marker := n.Marker
		if marker == "" {
			marker = "  "
		}
		saved := r.indent
		r.lead = r.indent + marker
		r.indent += strings.Repeat(" ", dispW(marker))
		r.inlineChildren(n, -1, segPlain)
		r.flush()
		r.indent = saved
	case ir.Table:
		r.flush()
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
		r.inlineChildren(n, -1, segPlain)
		r.flush()
	case ir.Separator:
		r.flush()
		r.emit(row{segs: []seg{{text: r.indent + strings.Repeat("─", clamp(r.width-dispW(r.indent), 1, 40)), item: -1, kind: segDim}}})
		r.gap = true
	case ir.Quote:
		r.flush()
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
		r.inlineChildren(n, -1, segPlain)
		r.flush()
	case ir.Code:
		r.flush()
		r.codeBlock(n)
		r.gap = true
	case ir.Unsupported:
		r.flush()
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

func (r *renderer) children(n *ir.Node, depth int) {
	for _, c := range n.Children {
		if c.IsBlock() {
			r.block(c, depth+1)
		} else {
			r.inline(c, -1, segPlain)
		}
	}
}

// inlineChildren flows n's children; a block child inside a flow breaks it
// — except inside a table cell, where it flattens (see inCell).
func (r *renderer) inlineChildren(n *ir.Node, item int, kind segKind) {
	for _, c := range n.Children {
		if c.IsBlock() && !(r.inNavList && inlineKind(c)) {
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

// inline adds one inline node to the flow. item/kind are inherited from an
// enclosing item (a heading's text) and overridden by the node's own.
func (r *renderer) inline(n *ir.Node, item int, kind segKind) {
	switch n.Kind {
	case ir.Text:
		r.words(n.Name, item, kind)
	case ir.Link:
		id := r.newItem(n)
		name := n.Name
		if name == "" {
			name = linkFallback(n.URL)
		}
		r.add(atom{text: glyphLink + " ", item: id, kind: segLink})
		r.words(name, id, segLink)
	case ir.Button:
		id := r.newItem(n)
		k := segButton
		if n.Disabled {
			k = segDim
		}
		r.add(atom{text: "[ " + oneLine(nameOr(n.Name, n.Value)) + " ]", item: id, kind: k})
	case ir.Textbox:
		r.dropLabel(n.Name)
		id := r.newItem(n)
		r.add(atom{text: glyphInput + " ", item: id, kind: segInput})
		if n.Name != "" {
			r.words(n.Name, id, segInput)
			r.add(atom{text: " ", item: id, kind: segInput, space: true})
		}
		r.add(atom{text: fieldText(n), item: id, kind: segInput})
	case ir.Check:
		r.dropLabel(n.Name)
		id := r.newItem(n)
		r.add(atom{text: checkText(n) + " ", item: id, kind: segCheck})
		r.words(n.Name, id, segCheck)
	case ir.Combobox:
		r.dropLabel(n.Name)
		id := r.newItem(n)
		if n.Name != "" {
			r.words(n.Name, id, segInput)
			r.add(atom{text: " ", item: id, kind: segInput, space: true})
		}
		r.add(atom{text: "[" + oneLine(n.Value) + " ▾]", item: id, kind: segInput})
	case ir.Media:
		id := r.newItem(n)
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
			id = r.newItem(n)
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
	if strings.TrimSpace(trailing) != name {
		return
	}
	r.flow = r.flow[:start]
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

// fieldText is a textbox's box: the value, or nothing, on a bed of
// underscores wide enough to be seen as a field (ux.md §2.2).
func fieldText(n *ir.Node) string {
	v := oneLine(n.Value)
	if n.Protected && v != "" {
		v = "••••"
	}
	const bed = 12
	w := max(bed, dispW(v)+4)
	pad := w - dispW(v)
	return strings.Repeat("_", pad/2) + v + strings.Repeat("_", pad-pad/2)
}

func checkText(n *ir.Node) string {
	if n.Role == "radio" {
		if n.Checked == ir.On {
			return "(•)"
		}
		return "( )"
	}
	switch n.Checked {
	case ir.On:
		return "[x]"
	case ir.Mixed:
		return "[-]"
	}
	return "[ ]"
}

func mediaText(n *ir.Node) string {
	g := glyphImage
	switch n.Role {
	case "Video":
		g = glyphVideo
	case "Audio":
		g = glyphAudio
	case "Iframe":
		g = glyphFrame
	case "Canvas":
		g = glyphCanvas
	}
	name := oneLine(n.Name)
	if name == "" {
		name = "no alt"
	}
	return "[" + g + " " + name + "]"
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
func (r *renderer) table(n *ir.Node) {
	if n.Name != "" {
		r.emit(row{segs: []seg{{text: r.indent + n.Name, item: -1, kind: segDim}}})
	}
	type cell struct {
		segs []seg
		w    int
	}
	var grid [][]cell
	var widths []int
	for _, tr := range n.Children {
		if tr.Kind != ir.Row {
			continue
		}
		var cells []cell
		for _, td := range tr.Children {
			if td.Kind != ir.Cell {
				continue
			}
			segs := r.cellSegs(td)
			w := 0
			for _, s := range segs {
				w += dispW(s.text)
			}
			cells = append(cells, cell{segs, w})
			if len(widths) < len(cells) {
				widths = append(widths, 0)
			}
			widths[len(cells)-1] = max(widths[len(cells)-1], w)
		}
		grid = append(grid, cells)
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
	for _, cells := range grid {
		out := []seg{{text: r.indent, item: -1, kind: segPlain}}
		for i, c := range cells {
			if i > 0 {
				out = append(out, seg{text: "  ", item: -1, kind: segPlain})
			}
			out = append(out, fitSegs(c.segs, widths[i])...)
		}
		r.emit(row{segs: out})
	}
}

// cellSegs is a cell's inline content as segments (no wrapping inside a
// cell: a table row is one screen row).
func (r *renderer) cellSegs(td *ir.Node) []seg {
	saved, wasCell := r.flow, r.inCell
	r.flow, r.inCell = nil, true
	defer func() { r.inCell = wasCell }()
	kind := segPlain
	if td.Header {
		kind = segTableHeader
	}
	r.inlineChildren(td, -1, kind)
	var out []seg
	for i, a := range r.flow {
		if a.space && (i == 0 || i == len(r.flow)-1) {
			continue
		}
		out = append(out, seg{text: a.text, item: a.item, kind: a.kind})
	}
	r.flow = saved
	return out
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
	for i := 0; i < len(flow); i++ {
		a := flow[i]
		if a.space {
			if used > dispW(prefix) {
				pending, pendingItem, pendingKind = true, a.item, a.kind
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
			line = append(line, seg{text: " ", item: pendingItem, kind: pendingKind})
			used++
			pending = false
		}
		text := a.text
		for dispW(text) > avail() && avail() > 0 {
			// A word wider than the line: hard-split it.
			head := truncateNoEllipsis(text, avail())
			line = append(line, seg{text: head, item: a.item, kind: a.kind})
			text = strings.TrimPrefix(text, head)
			emitLine()
		}
		line = append(line, seg{text: text, item: a.item, kind: a.kind})
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
