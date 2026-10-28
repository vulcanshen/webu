package ui

import (
	"path"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/reader"
)

// renderReader draws a reader page's content tree as webu's elements
// (docs/reader.md, the elements): no markdown symbols on screen — a
// heading is a row on its level's colour, a list item its icon or its
// number, code and quotes on Mantle, a link and an image an icon before
// their words — laid out at width, the things that can be pressed as
// items, the first row of every block as its mark.
func renderReader(root *ir.Node, d *reader.Doc, width, tabW int) layout {
	r := &rrender{width: max(1, width), tabW: max(1, tabW), d: d, lay: layout{marks: map[*ir.Node]int{}}, at: map[*ir.Node]int{}}
	if root != nil {
		r.blocks(root.Children, &rctx{})
	}
	for len(r.lay.rows) > 0 && r.lay.rows[len(r.lay.rows)-1].plain() == "" {
		r.lay.rows = r.lay.rows[:len(r.lay.rows)-1]
	}
	// An item that came to no row is not one.
	kept := r.lay.items[:0]
	remap := make([]int, len(r.lay.items))
	for i, it := range r.lay.items {
		remap[i] = -1
		if it.first >= 0 {
			remap[i] = len(kept)
			kept = append(kept, it)
		}
	}
	for _, row := range r.lay.rows {
		for j := range row.segs {
			if row.segs[j].item >= 0 {
				row.segs[j].item = remap[row.segs[j].item]
			}
		}
	}
	r.lay.items = kept
	return r.lay
}

type rrender struct {
	width, tabW int
	d           *reader.Doc
	lay         layout
	at          map[*ir.Node]int // a node's item
}

// rcell is one cell of a row being made: its rune, its width, how it is
// drawn, and the item it belongs to (-1 for none).
type rcell struct {
	r    rune
	w    int
	kind segKind
	attr textAttr
	item int
}

// rctx is what a block's rows start with — head on the first, hang on the
// rest — and the ground they are on; tight lists have no blank rows
// between their blocks.
type rctx struct {
	head, hang     []rcell
	used           *bool
	ground         lipgloss.Color
	tight          bool
	udepth, odepth int // how deep in bulleted, in numbered lists
}

func (c *rctx) next() []rcell {
	if c.used == nil || *c.used {
		return c.hang
	}
	*c.used = true
	return c.head
}

func (c *rctx) inner() rctx {
	return rctx{hang: c.hang, ground: c.ground, tight: c.tight, udepth: c.udepth, odepth: c.odepth}
}

func cells(s string, kind segKind, attr textAttr, item int) []rcell {
	out := make([]rcell, 0, len(s))
	for _, r := range s {
		out = append(out, rcell{r: r, w: dispW(string(r)), kind: kind, attr: attr, item: item})
	}
	return out
}

func cellsW(cs []rcell) int {
	w := 0
	for _, c := range cs {
		w += c.w
	}
	return w
}

// row adds cs as a row on ground, the items in it told where they are.
func (r *rrender) row(cs []rcell, ground, ink lipgloss.Color, code bool) {
	at := len(r.lay.rows)
	out := row{ground: ground, ink: ink, code: code}
	col := 0
	for _, c := range cs {
		if c.item >= 0 {
			it := &r.lay.items[c.item]
			if it.first < 0 {
				it.first, it.col = at, col
			}
			it.last = at
		}
		col += c.w
		if n := len(out.segs); n > 0 {
			last := &out.segs[n-1]
			if last.kind == c.kind && last.attr == c.attr && last.item == c.item {
				last.text += string(c.r)
				continue
			}
		}
		out.segs = append(out.segs, seg{text: string(c.r), item: c.item, kind: c.kind, attr: c.attr})
	}
	r.lay.rows = append(r.lay.rows, out)
}

// blank is an empty row between blocks, once, with the bars of the quote
// it is in.
func (r *rrender) blank(c *rctx) {
	if c.tight || len(r.lay.rows) == 0 {
		return
	}
	last := strings.TrimSpace(r.lay.rows[len(r.lay.rows)-1].plain())
	if last == "" || last == strings.TrimSpace(plainCells(c.hang)) {
		return
	}
	r.row(trimCells2(c.hang), c.ground, "", false)
}

func plainCells(cs []rcell) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteRune(c.r)
	}
	return b.String()
}

func trimCells2(cs []rcell) []rcell {
	for len(cs) > 0 && cs[len(cs)-1].r == ' ' {
		cs = cs[:len(cs)-1]
	}
	return cs
}

// flow wraps a run of inline cells into rows: at a space or between wide
// characters, the first row after the block's head, the rest after its
// hang; a line break in the page is one here too.
func (r *rrender) flow(cs []rcell, c *rctx, ground, ink lipgloss.Color) {
	for len(cs) > 0 && cs[0].r == ' ' {
		cs = cs[1:]
	}
	cs = trimCells2(cs)
	if len(cs) == 0 {
		return
	}
	cur := append([]rcell{}, c.next()...)
	start := len(cur)
	w, brk := cellsW(cur), -1
	emit := func() {
		r.row(trimCells2(cur), ground, ink, false)
		cur = append([]rcell{}, c.hang...)
		start = len(cur)
		w, brk = cellsW(cur), -1
	}
	for _, x := range cs {
		if x.r == '\n' {
			emit()
			continue
		}
		if w+x.w > r.width && len(cur) > start {
			cut := len(cur)
			if brk > start && brk < len(cur) {
				cut = brk
			}
			rest := append([]rcell{}, cur[cut:]...)
			cur = cur[:cut]
			emit()
			for len(rest) > 0 && rest[0].r == ' ' {
				rest = rest[1:]
			}
			cur = append(cur, rest...)
			w = cellsW(cur)
		}
		if x.r == ' ' && len(cur) == start {
			continue
		}
		cur = append(cur, x)
		w += x.w
		if x.r == ' ' || x.w > 1 {
			brk = len(cur)
		}
	}
	if len(cur) > start {
		r.row(trimCells2(cur), ground, ink, false)
	}
}

// blocks draws a run of children: the inline ones gathered into one flow,
// the way the page runs them on, each block on rows of its own.
func (r *rrender) blocks(nodes []*ir.Node, c *rctx) {
	var run []rcell
	flush := func() {
		if strings.TrimSpace(plainCells(run)) != "" {
			r.blank(c)
			r.flow(run, c, c.ground, "")
		}
		run = nil
	}
	for _, n := range nodes {
		if n.IsBlock() || n.Kind == ir.Code && (n.Block || strings.Contains(n.Text(), "\n")) {
			flush()
			r.block(n, c)
			continue
		}
		run = r.inline(run, n, 0, -1)
	}
	flush()
}

func (r *rrender) mark(n *ir.Node) {
	if _, ok := r.lay.marks[n]; !ok {
		r.lay.marks[n] = len(r.lay.rows)
	}
}

func (r *rrender) block(n *ir.Node, c *rctx) {
	r.mark(n)
	switch n.Kind {
	case ir.Heading:
		r.blank(c)
		var run []rcell
		for _, k := range n.Children {
			run = r.inline(run, k, attrBold, -1)
		}
		r.flow(run, c, levelColor(n.Level), lipgloss.Color(baseHex))
	case ir.Paragraph:
		r.blank(c)
		var run []rcell
		for _, k := range n.Children {
			run = r.inline(run, k, 0, -1)
		}
		r.flow(run, c, c.ground, "")
	case ir.List:
		r.list(n, c)
	case ir.Code:
		r.code(n, c)
	case ir.Quote:
		r.blank(c)
		bar := append(cells("▎", segQuoteBar, 0, -1), cells(" ", segPlain, 0, -1)...)
		used := false
		in := rctx{head: append(append([]rcell{}, c.next()...), bar...), hang: append(append([]rcell{}, c.hang...), bar...),
			used: &used, ground: lipgloss.Color(mantleHex), udepth: c.udepth, odepth: c.odepth}
		r.blocks(n.Children, &in)
	case ir.Separator:
		r.blank(c)
		rule := append(append([]rcell{}, c.next()...), cells(strings.Repeat("─", max(1, r.width-cellsW(c.hang))), segDim, 0, -1)...)
		r.row(rule, c.ground, "", false)
	case ir.Table:
		r.table(n, c)
	case ir.Group:
		if n.Role == "form" {
			r.form(n, c)
			return
		}
		r.blocks(n.Children, c)
	default:
		r.blocks(n.Children, c)
	}
}

// list draws a list's items, each after its bullet — a circle, a square,
// a diamond by depth — or its number — 1., a., i. by depth.
func (r *rrender) list(n *ir.Node, c *rctx) {
	r.blank(c)
	ordered := n.Role == "ol"
	inner := c.inner()
	inner.tight = true
	if ordered {
		inner.odepth++
	} else {
		inner.udepth++
	}
	for i, it := range n.Children {
		if it.Kind != ir.ListItem {
			r.blocks([]*ir.Node{it}, &inner)
			continue
		}
		var marker []rcell
		if ordered {
			num := i + 1
			if v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(it.Marker), ".")); err == nil {
				num = v
			}
			marker = cells(olLabel(num, inner.odepth)+". ", segPlain, 0, -1)
		} else {
			marker = append(cells(bullets[(inner.udepth-1)%len(bullets)], segBullet, 0, -1), cells(" ", segPlain, 0, -1)...)
		}
		used := false
		item := inner
		item.head = append(append([]rcell{}, c.next()...), marker...)
		item.hang = append(append([]rcell{}, c.hang...), cells(strings.Repeat(" ", cellsW(marker)), segPlain, 0, -1)...)
		item.used = &used
		r.mark(it)
		r.blocks(it.Children, &item)
		if !used {
			r.row(trimCells2(item.head), c.ground, "", false)
		}
	}
}

// bullets are a bulleted list's icons by depth: nf-fa-circle, nf-fa-square,
// nf-fa-diamond.
var bullets = []string{string(rune(0xf111)), string(rune(0xf0c8)), string(rune(0xf219))}

// olLabel is a numbered list's number at its depth: 1, a, i, then 1 again.
func olLabel(n, depth int) string {
	switch (max(1, depth) - 1) % 3 {
	case 1:
		return alpha(n)
	case 2:
		return roman(n)
	}
	return strconv.Itoa(n)
}

// alpha is n as letters: a…z, then aa, ab…
func alpha(n int) string {
	if n < 1 {
		return strconv.Itoa(n)
	}
	var s []byte
	for n > 0 {
		n--
		s = append([]byte{byte('a' + n%26)}, s...)
		n /= 26
	}
	return string(s)
}

// roman is n in lower-case Roman numerals.
func roman(n int) string {
	if n < 1 || n > 3999 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}

// code draws a code block on Mantle, coloured by its language — the
// page's word for it, else chroma's guess — named at its top right.
func (r *rrender) code(n *ir.Node, c *rctx) {
	r.blank(c)
	text := strings.TrimRight(strings.ReplaceAll(n.Text(), "\t", strings.Repeat(" ", r.tabW)), "\n")
	lang := n.Lang
	if lang == "" {
		lang = guessLang(text)
	}
	ground := lipgloss.Color(mantleHex)
	pre := c.next()
	if lang != "" {
		pad := max(0, r.width-cellsW(pre)-dispW(lang))
		r.row(append(append([]rcell{}, pre...), append(cells(strings.Repeat(" ", pad), segPlain, 0, -1), cells(lang, segDim, 0, -1)...)...), ground, "", false)
		pre = c.hang
	}
	lines := highlightLines(lang, text)
	if lines == nil {
		for _, l := range strings.Split(text, "\n") {
			lines = append(lines, []seg{{text: l, kind: segCode, item: -1}})
		}
	}
	for i, line := range lines {
		if i > 0 || lang != "" {
			pre = c.hang
		}
		rc := append([]rcell{}, pre...)
		for _, s := range line {
			rc = append(rc, cells(s.text, s.kind, 0, -1)...)
		}
		r.row(rc, ground, "", true)
	}
}

// table draws a table of data as a grid, its header row on its own
// ground, cut to the panel with … where it is too wide.
func (r *rrender) table(n *ir.Node, c *rctx) {
	r.blank(c)
	var grid [][][]rcell
	var head []bool
	cols := 0
	for _, row := range n.Children {
		var rowCells [][]rcell
		header := len(row.Children) > 0
		for _, cell := range row.Children {
			var run []rcell
			for _, k := range cell.Children {
				run = r.inline(run, k, 0, -1)
			}
			for i := range run {
				if run[i].r == '\n' {
					run[i].r, run[i].w = ' ', 1
				}
			}
			for len(run) > 0 && run[0].r == ' ' {
				run = run[1:]
			}
			rowCells = append(rowCells, trimCells2(run))
			header = header && cell.Header
		}
		grid = append(grid, rowCells)
		head = append(head, header)
		cols = max(cols, len(rowCells))
	}
	if cols == 0 {
		return
	}
	widths := make([]int, cols)
	for _, row := range grid {
		for j, cl := range row {
			widths[j] = max(widths[j], cellsW(cl))
		}
	}
	avail := r.width - cellsW(c.hang) - (3*cols + 1)
	for sumInts(widths) > avail {
		widest := 0
		for j := range widths {
			if widths[j] > widths[widest] {
				widest = j
			}
		}
		if widths[widest] <= 3 {
			break
		}
		widths[widest]--
	}
	rule := func(l, mid, rr string) []rcell {
		var b strings.Builder
		b.WriteString(l)
		for j, w := range widths {
			if j > 0 {
				b.WriteString(mid)
			}
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteString(rr)
		return append(append([]rcell{}, c.next()...), cells(b.String(), segDim, 0, -1)...)
	}
	r.row(rule("┌", "┬", "┐"), c.ground, "", false)
	for i, row := range grid {
		rc := append([]rcell{}, c.next()...)
		rc = append(rc, cells("│", segDim, 0, -1)...)
		for j := 0; j < cols; j++ {
			var cl []rcell
			if j < len(row) {
				cl = row[j]
			}
			if cellsW(cl) > widths[j] {
				cl = append(cutCells(cl, widths[j]-1), cells("…", segDim, 0, -1)...)
			}
			rc = append(rc, cells(" ", segPlain, 0, -1)...)
			rc = append(rc, cl...)
			rc = append(rc, cells(strings.Repeat(" ", widths[j]-cellsW(cl)+1), segPlain, 0, -1)...)
			rc = append(rc, cells("│", segDim, 0, -1)...)
		}
		var ground lipgloss.Color = c.ground
		if head[i] {
			ground = lipgloss.Color(surface0Hex)
		}
		r.row(rc, ground, "", false)
		if head[i] && i+1 < len(grid) && !head[i+1] {
			r.row(rule("├", "┼", "┤"), c.ground, "", false)
		}
	}
	r.row(rule("└", "┴", "┘"), c.ground, "", false)
}

func sumInts(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

// cutCells is cs up to w cells wide.
func cutCells(cs []rcell, w int) []rcell {
	used := 0
	for i, c := range cs {
		if used+c.w > w {
			return cs[:i]
		}
		used += c.w
	}
	return cs
}

// form draws a form in its box: its name on top, its fields down the
// side.
func (r *rrender) form(n *ir.Node, c *rctx) {
	r.blank(c)
	title := "┌ Form"
	if n.Name != "" {
		title += ": " + n.Name
	}
	r.row(append(append([]rcell{}, c.next()...), cells(title, segDim, 0, -1)...), c.ground, "", false)
	side := cells("│ ", segDim, 0, -1)
	used := false
	in := c.inner()
	in.head = append(append([]rcell{}, c.hang...), side...)
	in.hang = in.head
	in.used, in.tight = &used, true
	r.blocks(n.Children, &in)
	r.row(append(append([]rcell{}, c.hang...), cells("└", segDim, 0, -1)...), c.ground, "", false)
}

// item is n's item, made the first time it is met.
func (r *rrender) item(n *ir.Node) int {
	if i, ok := r.at[n]; ok {
		return i
	}
	r.at[n] = len(r.lay.items)
	r.lay.items = append(r.lay.items, item{node: n, first: -1})
	return r.at[n]
}

// inline adds n to a run of cells: text with its marks, a link or an
// image as its icon and its words, a field as its box. inItem is the item
// the cells belong to when they are inside one (a link's words).
func (r *rrender) inline(run []rcell, n *ir.Node, attr textAttr, inItem int) []rcell {
	space := func() {
		if len(run) > 0 && run[len(run)-1].r != ' ' && run[len(run)-1].r != '\n' {
			run = append(run, cells(" ", segPlain, 0, -1)...)
		}
	}
	switch n.Kind {
	case ir.Text:
		kind := segPlain
		if inItem >= 0 {
			kind = r.lay.items[inItem].kindOf()
		}
		s := n.Name
		if len(run) > 0 && run[len(run)-1].r == ' ' {
			s = strings.TrimLeft(s, " ")
		}
		return append(run, cells(s, kind, attr, inItem)...)
	case ir.Span:
		for _, k := range n.Children {
			run = r.inline(run, k, attr|attrOf(n.Role), inItem)
		}
		return run
	case ir.Code:
		if t := strings.TrimSpace(n.Text()); t != "" {
			return append(run, cells(t, segInlineCode, attr, inItem)...)
		}
		return run
	case ir.Link:
		if inItem >= 0 {
			for _, k := range n.Children {
				run = r.inline(run, k, attr, inItem)
			}
			return run
		}
		i := r.item(n)
		run = append(run, cells(glyphLink+" ", segLink, attr, i)...)
		before := len(run)
		for _, k := range n.Children {
			run = r.inline(run, k, attr, i)
		}
		if strings.TrimSpace(plainCells(run[before:])) == "" {
			run = append(run[:before], cells(nameOr(n.Name, n.URL), segLink, attr, i)...)
		}
		return run
	case ir.Media:
		words := strings.TrimSpace(n.Name)
		if words == "" {
			words = path.Base(n.URL)
		}
		if inItem >= 0 {
			return append(run, cells(glyphImage+" "+words, segLink, attr, inItem)...)
		}
		return append(run, cells(glyphImage+" "+words, segMedia, attr, r.item(n))...)
	case ir.Textbox:
		space()
		return append(run, cells(fieldBox(r.d, n), segInput, attr, r.item(n))...)
	case ir.Check:
		space()
		mark := "[ ]"
		if n.Role == "radio" {
			mark = "( )"
		}
		if n.Checked == ir.On {
			mark = map[bool]string{true: "(•)", false: "[x]"}[n.Role == "radio"]
		}
		return append(run, cells(mark, segCheck, attr, r.item(n))...)
	case ir.Combobox:
		space()
		return append(run, cells("["+nameOr(n.Value, " ")+" ▾]", segInput, attr, r.item(n))...)
	case ir.Button:
		space()
		return append(run, cells("( "+n.Name+" )", segButton, attr, r.item(n))...)
	}
	for _, k := range n.Children {
		run = r.inline(run, k, attr, inItem)
	}
	return run
}

// kindOf is how the words of an item are drawn: a link's as a link.
func (it item) kindOf() segKind {
	if it.node != nil && it.node.Kind == ir.Link {
		return segLink
	}
	return segPlain
}

// fieldBox is a text field as its box shows it: its value, a password as
// dots, an empty one its hint's room.
func fieldBox(d *reader.Doc, n *ir.Node) string {
	v := strings.ReplaceAll(n.Value, "\n", " ")
	switch {
	case n.Disabled && n.InputType == "file":
		return "[file: not supported]"
	case n.Protected && v != "":
		return "[" + strings.Repeat("•", min(len([]rune(v)), 12)) + "]"
	case v == "":
		hint := ""
		if d != nil {
			if f := d.Field(n); f != nil {
				hint = f.Hint
			}
		}
		return "[" + hint + strings.Repeat("_", max(4, 12-len([]rune(hint)))) + "]"
	}
	if r := []rune(v); len(r) > 40 {
		v = string(r[:40]) + "…"
	}
	return "[" + v + "]"
}
