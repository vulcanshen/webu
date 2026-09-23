package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/ir"
)

// The finder is the popup behind [/] and [go]: a query, a list under it,
// and — for a search — a preview beside the list. filu's own finder,
// with one unit instead of two: filu picks a file by name or by content,
// webu has only content, and the thing it picks is a node of the page
// (user, 2026-09-23).
//
// [/] used to open selection mode with a search box, which froze the
// page, took every global key, and could see only the layout on screen —
// nothing in another part, nothing in a shut list item, nothing in a
// section not open. The finder searches the whole page, every part of
// it, and Enter GOES there: the part switches, the section opens, the
// list item is drilled into, and the cursor lands. It never presses
// anything — a hit on a link's text must not leave the page — so what
// Enter would do on the page is one more Enter away, where you can see
// what it is.
//
// [go] is the same popup over the lines of what is showing, filtered by
// number: what the gutter's line numbers are for.
type finderKind uint8

const (
	finderSearch finderKind = iota
	finderGo
)

// finderMode is where the keyboard is. Typing edits the query; Enter
// hands the keyboard to the list, where j/k/u/d move, Enter goes and Esc
// comes back up to the query. [go] has no query to speak of — digits and
// Backspace are its whole input — so its keyboard is always on the list.
type finderMode uint8

const (
	finderInput finderMode = iota
	finderNav
)

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

type finder struct {
	anim    popupAnimator
	kind    finderKind
	mode    finderMode
	query   string
	all     []hit
	hits    []hit
	cursor  int
	top     int
	preview []string // hits[cursor] drawn, for the preview box
	prevFor *ir.Node
	prevW   int
	pageURL string
	layer   int
	screenW int
	screenH int
}

func newFinder() finder { return finder{anim: newPopupAnimator("finder")} }

func (f finder) isActive() bool      { return f.anim.isActive() }
func (f finder) isInteractive() bool { return f.anim.isInteractive() }
func (f *finder) close() tea.Cmd     { return f.anim.close() }
func (f *finder) setSize(w, h int)   { f.screenW, f.screenH = w, h }

// typing reports whether every printable key is a character right now:
// a search with the keyboard on its query. [go] takes digits alone, and
// Space on its list closes it the way Space closes any popup.
func (f finder) typing() bool {
	return f.anim.owns() && f.kind == finderSearch && f.mode == finderInput
}

// openSearch indexes the page and opens the finder over it.
func (f *finder) openSearch(t *tab, layer int) tea.Cmd {
	f.kind, f.mode, f.query, f.layer = finderSearch, finderInput, "", layer
	f.all = indexPage(t)
	f.pageURL = t.url
	f.cursor, f.top, f.prevFor = 0, 0, nil
	f.refilter()
	return f.anim.open()
}

// openGo opens the finder over the lines of what the panel is showing.
func (f *finder) openGo(t *tab, layer int) tea.Cmd {
	f.kind, f.mode, f.query, f.layer = finderGo, finderNav, "", layer
	f.all = f.all[:0]
	for n := 1; n <= t.lineCount(); n++ {
		f.all = append(f.all, hit{line: n, text: t.lineText(n)})
	}
	f.cursor, f.top, f.prevFor = 0, 0, nil
	f.refilter()
	return f.anim.open()
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
		if n.Kind == ir.Option {
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
				if text := oneLine(c.Text()); text != "" {
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
	only := t.popupNode()
	for i, c := range t.root.Children {
		if only != nil && c != only {
			// A popup is the panel until it is answered: the page under
			// it is out of reach, so it is out of the index too.
			continue
		}
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
	case ir.Document, ir.Text, ir.Span, ir.List, ir.Table, ir.Row, ir.Separator, ir.Option:
		return ""
	case ir.Heading:
		if s := oneLine(n.Name); s != "" {
			return s
		}
	case ir.Landmark:
		if n.Role == "main" || n.Skip {
			return ""
		}
		return oneLine(n.Name)
	case ir.Link, ir.Button, ir.Media, ir.Textbox, ir.Check, ir.Combobox:
		// Inline things: their block carries them. On their own — a bare
		// link the page laid out as a block — they are their own hit.
		if !n.Block {
			return ""
		}
		return oneLine(n.Text())
	case ir.Code:
		return oneLine(n.Text())
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
	return oneLine(b.String())
}

// refilter rebuilds the list from the query. A search is fuzzy on the
// short things — a link, a heading, a label — and literal on prose: a
// subsequence of a paragraph's four hundred characters is nearly any
// word, and a list of everything is a list of nothing. [go] is a prefix
// on the line number, digits only.
func (f *finder) refilter() {
	f.hits = f.hits[:0]
	if f.kind == finderGo {
		for _, h := range f.all {
			if f.query == "" || strings.HasPrefix(itoa(h.line), f.query) {
				f.hits = append(f.hits, h)
			}
		}
	} else {
		type scored struct {
			hit
			score, idx int
		}
		var found []scored
		for i, h := range f.all {
			if s, ok := hitScore(h.text, f.query); ok {
				found = append(found, scored{h, s, i})
			}
		}
		sort.SliceStable(found, func(a, b int) bool {
			if found[a].score != found[b].score {
				return found[a].score > found[b].score
			}
			return found[a].idx < found[b].idx
		})
		for _, s := range found {
			f.hits = append(f.hits, s.hit)
		}
	}
	f.cursor = clamp(f.cursor, 0, max(0, len(f.hits)-1))
	f.scroll()
}

// fuzzyNameMax is how long a text can be and still be matched as a
// subsequence: the length of a name, not of a paragraph.
const fuzzyNameMax = 64

// hitScore ranks a text against the query: a literal hit first — earlier
// in the text and at a word's start counting for more — then, on a short
// text, filepicker's fuzzy order.
func hitScore(text, q string) (int, bool) {
	if q == "" {
		return 0, true
	}
	lt, lq := strings.ToLower(text), strings.ToLower(q)
	if at := strings.Index(lt, lq); at >= 0 {
		score := 100 - min(40, at/4)
		if at == 0 || strings.ContainsRune(" /_-.([\"'", rune(lt[at-1])) {
			score += 20
		}
		return score, true
	}
	if len([]rune(text)) > fuzzyNameMax {
		return 0, false
	}
	return fuzzyScore(text, q)
}

// visible is how many list rows the box shows: the box's rows less the
// query bar and the rule under it.
func (f finder) visible() int {
	_, _, rows, _, _ := f.geometry()
	return max(1, rows-2)
}

func (f *finder) scroll() {
	vis := f.visible()
	if f.cursor < f.top {
		f.top = f.cursor
	}
	if f.cursor >= f.top+vis {
		f.top = f.cursor - vis + 1
	}
	f.top = max(0, min(f.top, max(0, len(f.hits)-vis)))
}

// escape is Esc: back from the list to the query, or closed. True when
// the finder is now closing.
func (f *finder) escape() (tea.Cmd, bool) {
	if f.kind == finderSearch && f.mode == finderNav {
		f.mode = finderInput
		return nil, false
	}
	return f.close(), true
}

// update takes one keystroke. The returned hit is the one Enter chose,
// with ok true; the finder is not closed here — the caller lands on the
// hit and then closes it, so a landing that fails can say why.
func (f *finder) update(msg tea.KeyMsg) (hit, bool) {
	if !f.anim.isInteractive() {
		return hit{}, false
	}
	k := msg.String()
	if f.kind == finderGo {
		switch {
		case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9':
			f.query += string(msg.Runes)
			f.refilter()
			return hit{}, false
		case msg.Type == tea.KeyBackspace:
			if r := []rune(f.query); len(r) > 0 {
				f.query = string(r[:len(r)-1])
				f.refilter()
			}
			return hit{}, false
		}
		return f.navKey(k)
	}
	if f.mode == finderNav {
		return f.navKey(k)
	}
	switch msg.Type {
	case tea.KeyEnter:
		if len(f.hits) > 0 {
			f.mode = finderNav
		}
	case tea.KeyBackspace:
		if r := []rune(f.query); len(r) > 0 {
			f.query = string(r[:len(r)-1])
			f.refilter()
		}
	case tea.KeyCtrlU:
		f.query = ""
		f.refilter()
	case tea.KeySpace:
		f.query += " "
		f.refilter()
	case tea.KeyRunes:
		f.query += string(msg.Runes)
		f.refilter()
	case tea.KeyUp, tea.KeyDown:
		// The arrows move the preselection without leaving the query.
		f.cursor = moveCursor(f.cursor, len(f.hits), k, f.visible())
		f.scroll()
	}
	return hit{}, false
}

// navKey is the list's own keys: the page's vocabulary.
func (f *finder) navKey(k string) (hit, bool) {
	switch k {
	case "j", "down", "k", "up", "d", "ctrl+d", "u", "ctrl+u", "G":
		f.cursor = moveCursor(f.cursor, len(f.hits), k, f.visible())
	case "g":
		f.cursor = 0
	case "enter":
		if f.cursor < len(f.hits) {
			return f.hits[f.cursor], true
		}
	}
	f.scroll()
	return hit{}, false
}

// geometry is the finder's shape: a list box and, for a search, a
// preview box — side by side when the terminal is wide enough for both
// to be useful, stacked when it is not (filu's rule).
func (f finder) geometry() (side bool, listW, listRows, prevW, prevRows int) {
	if f.kind == finderGo {
		w := popupInnerW(f.screenW, 56)
		return false, w, max(4, min(f.screenH-6, f.screenH*3/5)), 0, 0
	}
	if f.screenW >= 96 {
		totalW := min(f.screenW-2, f.screenW*19/20)
		h := min(f.screenH-2, f.screenH*9/10)
		listOuter := max(totalW*2/5, 32)
		return true, max(listOuter-2, 20), max(h-2, 4), max(totalW-listOuter-3, 20), max(h-2, 4)
	}
	w := min(f.screenW-2, f.screenW*9/10) - 2
	h := min(f.screenH-2, f.screenH*9/10)
	lh := max(h*11/20, 8)
	return false, max(w, 20), max(lh-2, 4), max(w, 20), max(h-lh-2, 3)
}

func (f finder) view() string {
	bc := popupLayerColor(f.layer)
	side, listW, listRows, prevW, prevRows := f.geometry()
	title, hint := f.titleAndHint()
	list := drawPopupBoxPad(bc, title, hint, animRows(f.anim, f.listColumn(listW, listRows)), listW, false)
	if f.kind == finderGo {
		return list
	}
	pt := " preview "
	if f.cursor < len(f.hits) {
		h := f.hits[f.cursor]
		pt = " " + h.part.glyph() + " " + h.part.word()
		if h.under != "" {
			pt += " › " + h.under
		}
		pt = truncate(pt, prevW-2) + " "
	}
	prev := drawPopupBoxPad(bc, pt, "", animRows(f.anim, f.previewColumn(prevW, prevRows)), prevW, false)
	if side {
		gap := strings.TrimSuffix(strings.Repeat(" \n", strings.Count(list, "\n")+1), "\n")
		return joinHorizontal(list, gap, prev)
	}
	return joinVertical(list, prev)
}

func (f finder) titleAndHint() (string, string) {
	if f.kind == finderGo {
		return " " + glyphList + " Go to line ", hintLegend([][2]string{
			{"0-9", "filter"}, {"j/k", "move"}, {"Enter", "go"}, {"Esc", "close"}})
	}
	title := " " + glyphSearch + " Search "
	if f.mode == finderNav {
		return title, hintLegend([][2]string{
			{"j/k/u/d", "move"}, {"Enter", "go"}, {"Esc", "query"}, {"Space", "close"}})
	}
	return title, hintLegend([][2]string{{"Enter", "list"}, {"Esc", "close"}})
}

// listColumn is the query bar, a rule, and the hits, exactly rows tall.
func (f finder) listColumn(w, rows int) []string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	hand := lipgloss.NewStyle().Foreground(handColor)
	caret := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(editColor)

	// The query, and on the right how many: all of them while typing,
	// which of them once the keyboard is on the list.
	count := itoa(len(f.hits))
	if f.mode == finderNav && len(f.hits) > 0 {
		count = itoa(f.cursor+1) + "/" + itoa(len(f.hits))
	}
	lead := " " + glyphSearch + " "
	q := f.query
	switch {
	case f.kind == finderGo && q == "":
		q = dim.Render("line number")
	case f.kind == finderGo:
		q = txt.Render(q) + caret.Render(" ")
	case f.mode == finderInput:
		q = txt.Render(truncateHead(q, w-dispW(lead)-dispW(count)-3)) + caret.Render(" ")
	default:
		q = txt.Render(truncateHead(q, w-dispW(lead)-dispW(count)-3))
	}
	gap := max(1, w-dispW(lead)-dispW(q)-dispW(count)-1)
	out := []string{
		hand.Render(lead) + q + strings.Repeat(" ", gap) + dim.Render(count) + " ",
		dim.Render(strings.Repeat("─", w)),
	}
	listRows := rows - 2
	// The row under the hand: the neutral hand colour while it is a
	// preselection, the structural blue once j/k are moving it.
	bg := handColor
	if f.mode == finderNav {
		bg = focusColor
	}
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(bg)
	if len(f.hits) == 0 {
		note := "  no match"
		if f.query == "" {
			note = "  nothing here"
		}
		out = append(out, dim.Render(padRight(note, w)))
	}
	end := min(len(f.hits), f.top+listRows)
	for i := f.top; i < end; i++ {
		out = append(out, f.hitRow(f.hits[i], w, i == f.cursor, cur))
	}
	for len(out) < rows {
		out = append(out, strings.Repeat(" ", w))
	}
	return out[:rows]
}

// hitRow is one line of the list: the part's glyph and the text, cut to
// the width — never wrapped, the preview has the rest. [go]'s rows lead
// with their number instead.
func (f finder) hitRow(h hit, w int, lit bool, cur lipgloss.Style) string {
	txt := lipgloss.NewStyle().Foreground(textColor)
	num := lipgloss.NewStyle().Foreground(focusColor)
	if f.kind == finderGo {
		nw := len(itoa(len(f.all)))
		n := padLeft(itoa(h.line), nw)
		body := truncate(h.text, max(1, w-nw-3))
		if lit {
			return cur.Render(padRight(" "+n+"  "+body, w))
		}
		return " " + num.Render(n) + "  " + txt.Render(padRight(body, max(0, w-nw-3)))
	}
	glyph := h.part.glyph()
	body := truncate(h.text, max(1, w-dispW(glyph)-3))
	if lit {
		return cur.Render(padRight(" "+glyph+" "+body, w))
	}
	return " " + lipgloss.NewStyle().Foreground(headerColor).Render(glyph) + " " +
		txt.Render(padRight(body, max(0, w-dispW(glyph)-3)))
}

// previewColumn is the hit under the hand, drawn whole: the node itself
// rendered at the box's width, a list item open, every line of it.
func (f *finder) previewColumn(w, rows int) []string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	if f.cursor >= len(f.hits) {
		return padLines([]string{dim.Render(" nothing to show")}, w, rows)
	}
	h := f.hits[f.cursor]
	if f.prevFor != h.node || f.prevW != w {
		f.prevFor, f.prevW = h.node, w
		f.preview = previewOf(h.node, w-2)
	}
	out := make([]string, 0, rows)
	for _, l := range f.preview {
		out = append(out, " "+l)
	}
	return padLines(out, w, rows)
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

// padLines is exactly rows lines of width w, clipped and padded.
func padLines(lines []string, w, rows int) []string {
	out := make([]string, 0, rows)
	for _, l := range lines {
		if len(out) == rows {
			break
		}
		l = clipANSI(l, w)
		out = append(out, l+strings.Repeat(" ", max(0, w-dispW(l))))
	}
	for len(out) < rows {
		out = append(out, strings.Repeat(" ", w))
	}
	return out
}
