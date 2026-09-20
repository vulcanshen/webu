package ui

import (
	"strings"

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
)

type seg struct {
	text string
	item int // index into layout.items, or -1
	kind segKind
}

type row struct {
	segs []seg
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
	first, last int // row span, inclusive
}

type layout struct {
	rows  []row
	items []item
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

// atom is one unbreakable unit of inline flow: a word, a glyph, a space.
type atom struct {
	text  string
	item  int
	kind  segKind
	space bool // a breaking space: dropped at a line end
}

type renderer struct {
	width  int
	rows   []row
	items  []item
	flow   []atom
	indent string // prefix every wrapped line of the current block gets
	lead   string // prefix the FIRST line gets instead of indent (a list marker); consumed by the next flush
	gap    bool   // a blank row is owed before the next content row
}

func render(root *ir.Node, width int) layout {
	r := &renderer{width: max(1, width)}
	r.block(root, 0)
	r.flush()
	return layout{rows: r.rows, items: r.items}
}

// ------------------------------------------------------------------ blocks

func (r *renderer) block(n *ir.Node, depth int) {
	switch n.Kind {
	case ir.Document:
		r.children(n, depth)
	case ir.Landmark:
		r.flush()
		r.children(n, depth)
		r.flush()
		r.gap = true
	case ir.Heading:
		r.flush()
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
		r.table(n)
		r.gap = true
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

// inlineChildren flows n's children; a block child inside a flow breaks it.
func (r *renderer) inlineChildren(n *ir.Node, item int, kind segKind) {
	for _, c := range n.Children {
		if c.IsBlock() {
			r.block(c, 2)
			continue
		}
		r.inline(c, item, kind)
	}
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
			name = nameOr(n.URL, "link")
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
		if strings.Contains(n.Text(), "\n") {
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
		if n.IsBlock() {
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
			r.flushKeepIndent()
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
	for _, line := range strings.Split(n.Text(), "\n") {
		line = strings.ReplaceAll(line, "\t", "    ")
		r.emit(row{segs: []seg{{text: r.indent + truncate(line, r.width-dispW(r.indent)), item: -1, kind: segCode}}})
	}
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
	saved := r.flow
	r.flow = nil
	kind := segPlain
	if td.Header {
		kind = segHeading
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
	avail := func() int { return r.width - used }
	emitLine := func() {
		r.emit(row{segs: line})
		prefix = r.prefix()
		used = dispW(prefix)
		line = []seg{{text: prefix, item: -1, kind: segPlain}}
	}
	for i := 0; i < len(flow); i++ {
		a := flow[i]
		if a.space {
			if used+1 < r.width {
				line = append(line, seg{text: " ", item: a.item, kind: a.kind})
				used++
			}
			continue
		}
		w := dispW(a.text)
		if used+w > r.width && used > dispW(prefix) {
			// Drop a trailing space before breaking.
			if last := line[len(line)-1]; last.text == " " {
				line = line[:len(line)-1]
			}
			emitLine()
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
	for _, s := range rw.segs {
		r.touch(s.item, at)
	}
}

// touch records that item is on row at. Only emit calls it: a row index
// guessed before the row exists is wrong whenever a gap is owed.
func (r *renderer) touch(item, at int) {
	if item < 0 || item >= len(r.items) {
		return
	}
	it := &r.items[item]
	if it.first < 0 {
		it.first = at
	}
	it.last = at
}
