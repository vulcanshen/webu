package ui

import (
	"strings"

	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/reader"
)

// mdLayout lays a reader page's markdown lines out at width as panel [2]
// draws any page: rows of segments, the things on them that can be
// pressed or filled as items, the first row of every node as its mark. A
// line too long for the panel wraps at a space, or between two wide
// characters, its next rows starting with the line's hang; a Tab is tabW
// spaces.
func mdLayout(lines []reader.Line, width, tabW int) layout {
	lay := layout{marks: map[*ir.Node]int{}}
	tab := strings.Repeat(" ", max(1, tabW))
	for _, l := range lines {
		first := len(lay.rows)
		for _, n := range l.Starts {
			if _, ok := lay.marks[n]; !ok {
				lay.marks[n] = first
			}
		}
		base := len(lay.items)
		for _, it := range l.Items {
			lay.items = append(lay.items, item{node: it.Node, first: -1})
		}
		for _, r := range wrapLine(l, width, tab) {
			var out row
			col := 0
			for _, p := range r {
				s := seg{text: p.text, item: -1}
				if p.item >= 0 {
					s.item = base + p.item
					s.kind = itemKind(l.Items[p.item].Node)
					it := &lay.items[s.item]
					if it.first < 0 {
						it.first, it.col = len(lay.rows), col
					}
					it.last = len(lay.rows)
				}
				out.segs = append(out.segs, s)
				col += dispW(p.text)
			}
			lay.rows = append(lay.rows, out)
		}
	}
	// An item that came to no row — nothing of it to draw — is not one.
	kept := lay.items[:0]
	remap := make([]int, len(lay.items))
	for i, it := range lay.items {
		remap[i] = -1
		if it.first >= 0 {
			remap[i] = len(kept)
			kept = append(kept, it)
		}
	}
	for _, r := range lay.rows {
		for j := range r.segs {
			if r.segs[j].item >= 0 {
				r.segs[j].item = remap[r.segs[j].item]
			}
		}
	}
	lay.items = kept
	return lay
}

// itemKind is how an item of the markdown is drawn: in the colour of what
// it is, so what can be pressed shows.
func itemKind(n *ir.Node) segKind {
	switch n.Kind {
	case ir.Link:
		return segLink
	case ir.Button:
		return segButton
	case ir.Textbox, ir.Combobox:
		return segInput
	case ir.Check:
		return segCheck
	case ir.Media:
		return segMedia
	case ir.Heading:
		return segHeading
	}
	return segPlain
}

// piece is a run of a row that is one thing: plain text, or (item >= 0)
// part of the line's item'th item.
type piece struct {
	text string
	item int
}

type cell struct {
	r    rune
	item int
	w    int
}

// wrapLine cuts a line into rows no wider than width.
func wrapLine(l reader.Line, width int, tab string) [][]piece {
	var cells []cell
	for off, r := range l.Text {
		it := -1
		for i, x := range l.Items {
			if off >= x.From && off < x.To {
				it = i
				break
			}
		}
		if r == '\t' {
			for _, s := range tab {
				cells = append(cells, cell{r: s, item: it, w: 1})
			}
			continue
		}
		cells = append(cells, cell{r: r, item: it, w: dispW(string(r))})
	}
	var hang []cell
	if dispW(l.Hang) < width/2 {
		for _, r := range l.Hang {
			hang = append(hang, cell{r: r, item: -1, w: dispW(string(r))})
		}
	}
	var rows [][]cell
	cur := []cell{}
	w, brk := 0, -1 // brk: where cur may be cut, after a space or a wide character
	for _, c := range cells {
		if w+c.w > width && len(cur) > len(hang) {
			cut := len(cur)
			if brk > len(hang) && brk < len(cur) {
				cut = brk
			}
			rows = append(rows, trimCells(cur[:cut]))
			rest := cur[cut:]
			for len(rest) > 0 && rest[0].r == ' ' {
				rest = rest[1:]
			}
			cur = append(append([]cell{}, hang...), rest...)
			w, brk = 0, -1
			for _, x := range cur {
				w += x.w
			}
		}
		cur = append(cur, c)
		w += c.w
		if c.r == ' ' || c.w > 1 {
			brk = len(cur)
		}
	}
	rows = append(rows, trimCells(cur))
	out := make([][]piece, len(rows))
	for i, r := range rows {
		for _, c := range r {
			if n := len(out[i]); n > 0 && out[i][n-1].item == c.item {
				out[i][n-1].text += string(c.r)
				continue
			}
			out[i] = append(out[i], piece{text: string(c.r), item: c.item})
		}
	}
	return out
}

func trimCells(c []cell) []cell {
	for len(c) > 0 && c[len(c)-1].r == ' ' && c[len(c)-1].item < 0 {
		c = c[:len(c)-1]
	}
	return c
}
