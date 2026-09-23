package ir

import (
	"strings"
)

// Markdown writes the page out as markdown (2026-09-22).
//
// webu already draws the page as markdown does — headings with their
// level, prose wrapped, lists with their markers, code in its block,
// quotes indented behind a bar, the inline markup as text attributes —
// because that is what a terminal can honestly show of a document whose
// CSS it does not read. This is the same tree said in the other
// direction: what the user yanks when they want the page itself rather
// than its URL, and what a golden can be read by eye.
//
// What markdown has no word for — a button, a field, a check box — is
// written as a bracketed note rather than dropped, so a form still reads
// as a form. Nothing is escaped: a page's own asterisks are rarer than
// the noise escaping them would add, and the output is for a reader.
func Markdown(root *Node) string {
	if root == nil {
		return ""
	}
	w := &mdWriter{}
	w.block(root, "")
	return strings.Join(w.out, "\n\n") + "\n"
}

type mdWriter struct{ out []string }

func (w *mdWriter) push(s string) {
	if strings.TrimSpace(s) != "" {
		w.out = append(w.out, strings.TrimRight(s, " \n"))
	}
}

// children writes a container's children, gathering the runs of inline
// ones into a block of their own the way the page's own layout does: a
// label and the field it names are one line there, and two blocks here
// would read as two separate things.
func (w *mdWriter) children(n *Node, indent string) {
	var flow strings.Builder
	flush := func() {
		if line := para(flow.String()); line != "" {
			w.push(prefixLines(line, indent))
		}
		flow.Reset()
	}
	for _, c := range n.Children {
		if c.IsBlock() || (c.Kind == Code && strings.Contains(c.Text(), "\n")) {
			flush()
			w.block(c, indent)
			continue
		}
		writeInline(&flow, c)
	}
	flush()
}

// block writes one node that stands on its own; indent is what a list or
// a quote has put in front of every line of it.
func (w *mdWriter) block(n *Node, indent string) {
	switch n.Kind {
	case Document, Landmark, Group, Row:
		w.children(n, indent)
	case Heading:
		level := n.Level
		if level < 1 || level > 6 {
			level = 1
		}
		w.push(indent + strings.Repeat("#", level) + " " + oneline(inlineOf(n)))
	case Paragraph:
		w.push(prefixLines(para(inlineOf(n)), indent))
	case Cell:
		w.push(indent + oneline(inlineOf(n)))
	case List:
		w.list(n, indent)
	case Table:
		w.table(n, indent)
	case Code:
		fence := "```" + n.Lang
		body := strings.TrimRight(n.Text(), "\n")
		w.push(indent + fence + "\n" + prefixLines(body, indent) + "\n" + indent + "```")
	case Quote:
		inner := &mdWriter{}
		inner.children(n, "")
		w.push(prefixLines(strings.Join(inner.out, "\n\n"), indent+"> "))
	case Separator:
		w.push(indent + "---")
	case Media:
		w.push(indent + inlineOf(&Node{Children: []*Node{n}}))
	default:
		// An interactive node, or a run of text, standing on its own.
		w.push(indent + inlineOf(&Node{Children: []*Node{n}}))
	}
}

// list writes a list and the lists nested inside its items, each level
// one step further in. The marker is markdown's, not the page's: a page
// draws its own bullets and markdown names them. The whole list is one
// block, its items consecutive lines — a blank line between them would
// make every list a loose one.
func (w *mdWriter) list(n *Node, indent string) {
	w.push(strings.Join(listLines(n, indent), "\n"))
}

func listLines(n *Node, indent string) []string {
	var out []string
	num := 0
	for _, it := range n.Children {
		if it.Kind != ListItem {
			if line := oneline(inlineOf(&Node{Children: []*Node{it}})); line != "" {
				out = append(out, indent+line)
			}
			continue
		}
		num++
		marker := "- "
		if isOrdered(it.Marker) {
			marker = itoa(num) + ". "
		}
		var head strings.Builder
		var nested []*Node
		for _, c := range it.Children {
			if c.Kind == List {
				nested = append(nested, c)
				continue
			}
			writeInline(&head, c)
		}
		out = append(out, indent+marker+oneline(head.String()))
		for _, c := range nested {
			out = append(out, listLines(c, indent+strings.Repeat(" ", len(marker)))...)
		}
	}
	return out
}

// table writes a pipe table. Markdown has one header row and no more, so
// the first row is it, whether or not the page said so.
func (w *mdWriter) table(n *Node, indent string) {
	var rows [][]string
	for _, r := range n.Children {
		if r.Kind != Row {
			continue
		}
		var cells []string
		for _, c := range r.Children {
			if c.Kind == Cell {
				cells = append(cells, oneline(inlineOf(c)))
			}
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	if len(rows) == 0 {
		return
	}
	wide := 0
	for _, r := range rows {
		if len(r) > wide {
			wide = len(r)
		}
	}
	line := func(cells []string) string {
		for len(cells) < wide {
			cells = append(cells, "")
		}
		return indent + "| " + strings.Join(cells, " | ") + " |"
	}
	out := []string{line(rows[0])}
	rule := make([]string, wide)
	for i := range rule {
		rule[i] = "---"
	}
	out = append(out, line(rule))
	for _, r := range rows[1:] {
		out = append(out, line(r))
	}
	w.push(strings.Join(out, "\n"))
}

// para is a run of inline text as a markdown paragraph: the spaces that
// joining nodes left squeezed, and a <br> — a text node of one newline —
// kept as markdown's own hard break rather than swallowed.
func para(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(collapse(line)); line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "  \n")
}

// inlineOf is a node's children as one run of markdown text.
func inlineOf(n *Node) string {
	var b strings.Builder
	for _, c := range n.Children {
		writeInline(&b, c)
	}
	return strings.TrimSpace(collapse(b.String()))
}

func writeInline(b *strings.Builder, n *Node) {
	switch n.Kind {
	case Text:
		mdWrite(b, n.Name)
	case Span:
		inner := inlineOf(n)
		if inner == "" {
			return
		}
		switch n.Role {
		case "strong":
			mdWrite(b, "**"+inner+"**")
		case "emphasis":
			mdWrite(b, "*"+inner+"*")
		case "deletion":
			mdWrite(b, "~~"+inner+"~~")
		case "insertion":
			mdWrite(b, "<ins>"+inner+"</ins>")
		case "mark":
			mdWrite(b, "=="+inner+"==")
		default:
			mdWrite(b, inner)
		}
	case Link:
		text := nameOrText(n)
		if n.URL == "" {
			mdWrite(b, text)
			return
		}
		mdWrite(b, "["+text+"]("+n.URL+")")
	case Media:
		alt := n.Name
		if alt == "" {
			alt = n.Role
		}
		mdWrite(b, "!["+alt+"]("+n.URL+")")
	case Code:
		mdWrite(b, "`"+oneline(n.Text())+"`")
	case Button:
		mdWrite(b, "["+nameOrText(n)+"]")
	case Textbox:
		mdWrite(b, "["+fieldNote(n)+"]")
	case Check:
		box := "[ ] "
		if n.Checked == On {
			box = "[x] "
		}
		mdWrite(b, box+nameOrText(n))
	case Combobox:
		mdWrite(b, "["+nameOrText(n)+": "+n.Value+"]")
	case Gauge:
		label := n.Name
		if label == "" {
			label = n.Role
		}
		if n.Value == "" {
			mdWrite(b, "["+label+"]") // a progress with no value yet
		} else {
			mdWrite(b, "["+label+": "+n.Value+"]")
		}
	case Separator:
		mdWrite(b, "---")
	default:
		for _, c := range n.Children {
			writeInline(b, c)
		}
		if len(n.Children) == 0 {
			mdWrite(b, n.Name)
		}
	}
}

// mdWrite appends one chunk, putting a space between it and what is
// already there only where one is wanted: the AX tree drops the space
// HTML had between two elements, so two words would otherwise run
// together — while punctuation the author put after a link stays where
// they put it (the rule renderer.add keeps for the same reason).
func mdWrite(b *strings.Builder, s string) {
	if s == "" {
		return
	}
	if b.Len() > 0 {
		last := rune(b.String()[b.Len()-1])
		first := rune(s[0])
		if !isSpace(last) && !isSpace(first) && !closingPunct(first) {
			b.WriteByte(' ')
		}
	}
	b.WriteString(s)
}

func isSpace(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }

// closingPunct is what an author writes tight against the word before it.
func closingPunct(r rune) bool {
	return strings.ContainsRune(",.;:!?)]}%'\"", r)
}

func fieldNote(n *Node) string {
	name := nameOrText(n)
	switch {
	case n.Protected:
		return name + ": ••••"
	case n.Value != "":
		return name + ": " + n.Value
	}
	return name
}

func nameOrText(n *Node) string {
	if s := strings.TrimSpace(n.Name); s != "" {
		return oneline(s)
	}
	return oneline(n.Text())
}

// isOrdered says whether the marker Chromium drew is a number.
func isOrdered(marker string) bool {
	m := strings.TrimSpace(marker)
	return m != "" && m[0] >= '0' && m[0] <= '9'
}

func prefixLines(s, prefix string) string {
	if prefix == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(prefix+l, " ")
	}
	return strings.Join(lines, "\n")
}

// collapse squeezes the runs of space that joining nodes leaves behind.
func collapse(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

func oneline(s string) string {
	return strings.TrimSpace(collapse(strings.ReplaceAll(s, "\n", " ")))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
