package reader

import (
	"strings"

	"github.com/vulcanshen/webu/internal/ir"
)

// Line is one line of a page written as markdown: its text; Hang, what a
// line it wraps into starts with (a list item's indent, a quote's bar, a
// form's side); the things on it that can be pressed or filled; and the
// nodes that start on it, which is where a jump to one lands.
//
// Markdown is what the reader shows for now (docs/reader.md); webu's own
// elements come later, off the same tree.
type Line struct {
	Text   string
	Hang   string
	Items  []Item
	Starts []*ir.Node
}

// Item is a stretch of a line that is one thing: a heading, a link, an
// image, a field, a button. From and To are byte offsets into the line.
type Item struct {
	From, To int
	Node     *ir.Node
}

// Markdown is root written as markdown lines.
func (d *Doc) Markdown(root *ir.Node) []Line {
	w := &mdw{d: d}
	w.blocks(root.Children, &lead{})
	for len(w.lines) > 0 && strings.TrimSpace(w.lines[len(w.lines)-1].Text) == "" {
		w.lines = w.lines[:len(w.lines)-1]
	}
	return w.lines
}

// Text is lines as one markdown text, what Yank markdown copies.
func Text(lines []Line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

type mdw struct {
	d       *Doc
	lines   []Line
	pending []*ir.Node
	tight   bool // inside a list item: no blank lines between its blocks
}

// lead is what a block's lines start with: head on its first, hang on the
// rest. Blocks nested in a list item share it, so the item's marker is on
// its first line whatever block that is.
type lead struct {
	head, hang string
	used       bool
}

func (l *lead) next() string {
	if l.used {
		return l.hang
	}
	l.used = true
	return l.head
}

// emit adds a line. A flow's text may hold line breaks (<br>): each is a
// line of its own, the items going with their text.
func (w *mdw) emit(ld *lead, f *flow) {
	text := f.b.String()
	start := 0
	for start <= len(text) {
		end := strings.IndexByte(text[start:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += start
		}
		part := text[start:end]
		trimmed := strings.TrimRight(part, " ")
		lead := strings.TrimLeft(trimmed, " ")
		cut := len(trimmed) - len(lead) // spaces dropped at the start
		if lead != "" {
			p := ld.next()
			line := Line{Text: p + lead, Hang: ld.hang}
			for _, it := range f.items {
				if it.From >= start && it.To <= end {
					line.Items = append(line.Items, Item{From: len(p) + it.From - start - cut, To: len(p) + min(it.To-start, len(trimmed)) - cut, Node: it.Node})
				}
			}
			line.Starts = w.pending
			for _, it := range line.Items {
				line.Starts = append(line.Starts, it.Node)
			}
			w.pending = nil
			if strings.TrimSpace(line.Text) != "" {
				w.lines = append(w.lines, line)
			}
		}
		start = end + 1
	}
}

// blank adds an empty line between blocks, once, the lead's bar kept.
func (w *mdw) blank(ld *lead) {
	if w.tight || len(w.lines) == 0 {
		return
	}
	last := w.lines[len(w.lines)-1].Text
	bar := strings.TrimRight(ld.hang, " ")
	if strings.TrimSpace(last) == strings.TrimSpace(bar) {
		return
	}
	w.lines = append(w.lines, Line{Text: bar, Hang: ld.hang})
}

// blocks writes a run of children: inline ones gathered into a line, the
// way the page flows them, block ones each on lines of their own.
func (w *mdw) blocks(nodes []*ir.Node, ld *lead) {
	f := &flow{w: w}
	flush := func() {
		if strings.TrimSpace(f.b.String()) != "" {
			w.blank(ld)
			w.emit(ld, f)
		}
		f = &flow{w: w}
	}
	for _, n := range nodes {
		if isBlock(n) {
			flush()
			w.block(n, ld)
			continue
		}
		f.inline(n)
	}
	flush()
}

func isBlock(n *ir.Node) bool {
	return n.IsBlock() || n.Kind == ir.Code && strings.Contains(n.Text(), "\n")
}

func (w *mdw) block(n *ir.Node, ld *lead) {
	w.pending = append(w.pending, n)
	switch n.Kind {
	case ir.Heading:
		w.blank(ld)
		f := &flow{w: w}
		f.b.WriteString(strings.Repeat("#", max(1, n.Level)) + " ")
		f.inlines(n.Children)
		// The heading is where the cursor stops, unless there is a link in
		// it: then the link is, and one stretch is never two things.
		if len(f.items) == 0 {
			f.items = []Item{{From: 0, To: f.b.Len(), Node: n}}
		}
		w.emit(ld, f)
	case ir.Paragraph:
		w.blank(ld)
		f := &flow{w: w}
		f.inlines(n.Children)
		w.emit(ld, f)
	case ir.List:
		w.blank(ld)
		prev := w.tight
		w.tight = true
		for _, c := range n.Children {
			if c.Kind != ir.ListItem {
				w.blocks([]*ir.Node{c}, ld)
				continue
			}
			marker := nameOr(c.Marker, "- ")
			pad := strings.Repeat(" ", len(marker))
			item := &lead{head: ld.next() + marker, hang: ld.hang + pad}
			w.pending = append(w.pending, c)
			w.blocks(c.Children, item)
			if !item.used {
				w.lines = append(w.lines, Line{Text: strings.TrimRight(item.head, " "), Hang: item.hang, Starts: w.pending})
				w.pending = nil
			}
		}
		w.tight = prev
	case ir.Table:
		w.blank(ld)
		for i, row := range n.Children {
			f := &flow{w: w}
			f.b.WriteString("|")
			for _, cell := range row.Children {
				f.b.WriteString(" ")
				f.cell = true
				f.inlines(cell.Children)
				f.cell = false
				f.b.WriteString(" |")
			}
			w.emit(&lead{head: ld.next(), hang: ld.hang}, f)
			if i == 0 {
				sep := "|"
				for range row.Children {
					sep += " --- |"
				}
				w.lines = append(w.lines, Line{Text: ld.hang + sep, Hang: ld.hang})
			}
		}
	case ir.Code:
		w.blank(ld)
		w.lines = append(w.lines, Line{Text: ld.next() + "```" + n.Lang, Hang: ld.hang, Starts: w.pending})
		w.pending = nil
		for _, l := range strings.Split(n.Text(), "\n") {
			w.lines = append(w.lines, Line{Text: ld.hang + l, Hang: ld.hang})
		}
		w.lines = append(w.lines, Line{Text: ld.hang + "```", Hang: ld.hang})
	case ir.Quote:
		w.blank(ld)
		w.blocks(n.Children, &lead{head: ld.next() + "> ", hang: ld.hang + "> "})
	case ir.Separator:
		w.blank(ld)
		w.lines = append(w.lines, Line{Text: ld.next() + "---", Hang: ld.hang, Starts: w.pending})
		w.pending = nil
	case ir.Group:
		if n.Role == "form" {
			w.blank(ld)
			title := "┌ Form"
			if n.Name != "" {
				title += ": " + n.Name
			}
			w.lines = append(w.lines, Line{Text: ld.next() + title, Hang: ld.hang, Starts: w.pending})
			w.pending = nil
			prev := w.tight
			w.tight = true
			w.blocks(n.Children, &lead{head: ld.hang + "│ ", hang: ld.hang + "│ "})
			w.tight = prev
			w.lines = append(w.lines, Line{Text: ld.hang + "└", Hang: ld.hang})
			return
		}
		w.blocks(n.Children, ld)
	default:
		w.blocks(n.Children, ld)
	}
}

// flow is a line being written: its text, and the items on it by byte
// offset.
type flow struct {
	w     *mdw
	b     strings.Builder
	items []Item
	cell  bool // in a table's cell: a line break is a space
	plain bool // inside a link: its text only, no items of its own
}

// write adds s, one space where its white space meets the flow's.
func (f *flow) write(s string) {
	if f.cell {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	cur := f.b.String()
	if strings.HasPrefix(s, " ") && (cur == "" || strings.HasSuffix(cur, " ") || strings.HasSuffix(cur, "\n")) {
		s = strings.TrimLeft(s, " ")
	}
	f.b.WriteString(s)
}

func (f *flow) item(n *ir.Node, s string) {
	if f.plain {
		f.write(s)
		return
	}
	cur := f.b.String()
	if cur != "" && !strings.HasSuffix(cur, " ") && !strings.HasSuffix(cur, "\n") && f.needsSpace(n) {
		f.b.WriteString(" ")
	}
	from := f.b.Len()
	f.b.WriteString(s)
	f.items = append(f.items, Item{From: from, To: f.b.Len(), Node: n})
}

// needsSpace says a field or button is set off from the word before it,
// which a page does with margins markdown has not got.
func (f *flow) needsSpace(n *ir.Node) bool {
	switch n.Kind {
	case ir.Textbox, ir.Check, ir.Combobox, ir.Button:
		return true
	}
	return false
}

func (f *flow) inlines(nodes []*ir.Node) {
	for _, n := range nodes {
		f.inline(n)
	}
}

func (f *flow) inline(n *ir.Node) {
	switch n.Kind {
	case ir.Text:
		f.write(n.Name)
	case ir.Link:
		sub := &flow{w: f.w, plain: true, cell: true}
		sub.inlines(n.Children)
		label := strings.TrimSpace(sub.b.String())
		if label == "" {
			label = n.Name
		}
		f.item(n, "["+label+"]("+n.URL+")")
	case ir.Media:
		f.item(n, "!["+n.Name+"]("+n.URL+")")
	case ir.Span:
		mark := map[string]string{"strong": "**", "emphasis": "*", "deletion": "~~", "mark": "=="}[n.Role]
		sub := &flow{w: f.w, plain: f.plain, cell: f.cell}
		sub.inlines(n.Children)
		inner := sub.b.String()
		if strings.TrimSpace(inner) == "" {
			return
		}
		lead, trail := inner[:len(inner)-len(strings.TrimLeft(inner, " "))], inner[len(strings.TrimRight(inner, " ")):]
		f.write(lead)
		base := f.b.Len() + len(mark)
		f.write(mark + strings.TrimSpace(inner) + mark)
		for _, it := range sub.items {
			off := base - len(lead)
			f.items = append(f.items, Item{From: it.From + off, To: it.To + off, Node: it.Node})
		}
		f.write(trail)
	case ir.Code:
		if t := strings.TrimSpace(n.Text()); t != "" {
			f.write("`" + t + "`")
		}
	case ir.Textbox:
		f.item(n, f.w.box(n))
	case ir.Check:
		mark := "[ ]"
		if n.Role == "radio" {
			mark = "( )"
		}
		if n.Checked == ir.On {
			mark = map[bool]string{true: "(•)", false: "[x]"}[n.Role == "radio"]
		}
		f.item(n, mark)
	case ir.Combobox:
		f.item(n, "["+nameOr(n.Value, " ")+" ▾]")
	case ir.Button:
		f.item(n, "( "+n.Name+" )")
	default:
		f.inlines(n.Children)
	}
}

// box is a text field as the line shows it: what is in it, a password as
// dots, an empty one as its label's room.
func (w *mdw) box(n *ir.Node) string {
	v := strings.ReplaceAll(n.Value, "\n", " ")
	switch {
	case n.Disabled && n.InputType == "file":
		return "[file: not supported]"
	case n.Protected && v != "":
		v = strings.Repeat("•", min(len([]rune(v)), 12))
	case v == "":
		hint := ""
		if f := w.d.fields[n]; f != nil {
			hint = f.Hint
		}
		return "[" + hint + strings.Repeat("_", max(4, 12-len([]rune(hint)))) + "]"
	}
	if r := []rune(v); len(r) > 40 {
		v = string(r[:40]) + "…"
	}
	return "[" + v + "]"
}
