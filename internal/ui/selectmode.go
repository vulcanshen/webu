package ui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Selection mode (ux.md §1): a character cursor over the rendered page, so
// text can be walked, searched, swept and copied. The page is FROZEN while
// it is on — a capture that lands is kept aside (tab.frozen) and applied on
// the way out — because a cursor on text that moves under it is a cursor
// on nothing.
//
// The mode owns the keyboard: no global letter fires, Space is a cheatsheet
// rather than a menu, and Esc is layered — it drops a search being typed
// before it drops the mode.
type selectMode struct {
	on   bool
	text [][]rune // the page's rows, as the mode sees them; fixed while on
	row  int
	col  int // rune index into text[row]; may equal len (past the end)

	selecting bool
	lineWise  bool
	anchorRow int
	anchorCol int

	query   string
	typing  bool
	matches []match
	cur     int // index into matches, or -1

	pendingG bool
}

type match struct{ row, start, end int } // rune indexes, end exclusive

// selResult is what a key did that the app has to act on.
type selResult int

const (
	selNone  selResult = iota
	selLeave           // the mode ended
	selClick           // Enter: click the item under the cursor
	selYank            // y: text to copy is in the second return
)

// enter starts the mode over a laid-out page, with the cursor on the item
// cursor's row (or the top of the window).
func (s *selectMode) enter(t *tab, typing bool) {
	*s = selectMode{on: true, cur: -1, typing: typing}
	s.text = rowsRunes(t.lay)
	s.row = clamp(t.top, 0, max(0, len(s.text)-1))
	if n := t.current(); n != nil {
		s.row = clamp(t.lay.items[t.cursor].first, 0, max(0, len(s.text)-1))
	}
}

func rowsRunes(l layout) [][]rune {
	out := make([][]rune, len(l.rows))
	for i, r := range l.rows {
		out[i] = []rune(r.plain())
	}
	return out
}

// resize takes a re-laid-out page (the terminal changed width) and keeps
// the cursor somewhere sensible.
func (s *selectMode) resize(t *tab) {
	s.text = rowsRunes(t.lay)
	s.row = clamp(s.row, 0, max(0, len(s.text)-1))
	s.clampCol()
	s.search()
}

func (s *selectMode) line() []rune {
	if s.row < 0 || s.row >= len(s.text) {
		return nil
	}
	return s.text[s.row]
}

func (s *selectMode) clampCol() {
	s.col = clamp(s.col, 0, max(0, len(s.line())-1))
}

// key handles one keystroke. visible is the page window's height, for the
// half-page motions.
func (s *selectMode) key(msg tea.KeyMsg, visible int) (selResult, string) {
	if s.typing {
		switch msg.Type {
		case tea.KeyEscape:
			s.typing, s.query, s.matches, s.cur = false, "", nil, -1
		case tea.KeyEnter:
			s.typing = false
			s.search()
			s.jumpMatch(s.firstMatchFrom(s.row, s.col))
		case tea.KeyBackspace:
			if r := []rune(s.query); len(r) > 0 {
				s.query = string(r[:len(r)-1])
			}
			s.search()
		case tea.KeySpace:
			s.query += " "
			s.search()
		case tea.KeyRunes:
			s.query += string(msg.Runes)
			s.search()
		}
		return selNone, ""
	}

	k := msg.String()
	if s.pendingG {
		s.pendingG = false
		if k == "g" {
			k = "gg"
		}
	} else if k == "g" {
		s.pendingG = true
		return selNone, ""
	}

	n := len(s.text)
	half := max(1, visible/2)
	switch k {
	case "esc":
		s.on = false
		return selLeave, ""
	case "h", "left":
		s.col = max(0, s.col-1)
	case "l", "right":
		s.col = min(max(0, len(s.line())-1), s.col+1)
	case "j", "down":
		s.row = min(n-1, s.row+1)
		s.clampCol()
	case "k", "up":
		s.row = max(0, s.row-1)
		s.clampCol()
	case "u", "ctrl+u":
		s.row = max(0, s.row-half)
		s.clampCol()
	case "d", "ctrl+d":
		s.row = min(n-1, s.row+half)
		s.clampCol()
	case "gg":
		s.row, s.col = 0, 0
	case "G":
		s.row = max(0, n-1)
		s.clampCol()
	case "0", "home":
		s.col = 0
	case "$", "end":
		s.col = max(0, len(s.line())-1)
	case "w":
		s.wordForward()
	case "e":
		s.wordEnd()
	case "b":
		s.wordBack()
	case "v":
		s.toggleSelect(false)
	case "V":
		s.toggleSelect(true)
	case "y":
		text := s.selectedText()
		s.selecting = false
		return selYank, text
	case "/":
		s.typing = true
		s.query = ""
		s.matches, s.cur = nil, -1
	case "n":
		if len(s.matches) > 0 {
			s.jumpMatch((s.cur + 1) % len(s.matches))
		}
	case "N":
		if len(s.matches) > 0 {
			s.jumpMatch((s.cur - 1 + len(s.matches)) % len(s.matches))
		}
	case "enter":
		return selClick, ""
	}
	return selNone, ""
}

func (s *selectMode) toggleSelect(lineWise bool) {
	if s.selecting && s.lineWise == lineWise {
		s.selecting = false
		return
	}
	if !s.selecting {
		s.anchorRow, s.anchorCol = s.row, s.col
	}
	s.selecting, s.lineWise = true, lineWise
}

// ------------------------------------------------------------- motions

func isBlank(r rune) bool { return unicode.IsSpace(r) }

// wordForward is vim's W: to the start of the next run of non-blanks,
// across rows.
func (s *selectMode) wordForward() {
	line := s.line()
	i := s.col
	for i < len(line) && !isBlank(line[i]) {
		i++
	}
	for i < len(line) && isBlank(line[i]) {
		i++
	}
	if i < len(line) {
		s.col = i
		return
	}
	// Off the end: the first word of the next non-empty row.
	for r := s.row + 1; r < len(s.text); r++ {
		for j, c := range s.text[r] {
			if !isBlank(c) {
				s.row, s.col = r, j
				return
			}
		}
	}
	s.col = max(0, len(line)-1)
}

// wordEnd is vim's E: to the end of the current or next word.
func (s *selectMode) wordEnd() {
	line := s.line()
	i := s.col + 1
	for i < len(line) && isBlank(line[i]) {
		i++
	}
	if i >= len(line) {
		for r := s.row + 1; r < len(s.text); r++ {
			for j, c := range s.text[r] {
				if !isBlank(c) {
					s.row, s.col = r, j
					line, i = s.text[r], j
					goto run
				}
			}
		}
		s.col = max(0, len(line)-1)
		return
	}
run:
	for i+1 < len(line) && !isBlank(line[i+1]) {
		i++
	}
	s.col = i
}

// wordBack is vim's B: to the start of the previous word.
func (s *selectMode) wordBack() {
	line := s.line()
	i := s.col - 1
	for i >= 0 && isBlank(line[i]) {
		i--
	}
	if i < 0 {
		for r := s.row - 1; r >= 0; r-- {
			for j := len(s.text[r]) - 1; j >= 0; j-- {
				if !isBlank(s.text[r][j]) {
					s.row = r
					line, i = s.text[r], j
					goto run
				}
			}
		}
		s.col = 0
		return
	}
run:
	for i > 0 && !isBlank(line[i-1]) {
		i--
	}
	s.col = i
}

// ----------------------------------------------------------- selection

// span is the selection's ordered corners.
func (s *selectMode) span() (r1, c1, r2, c2 int) {
	r1, c1, r2, c2 = s.anchorRow, s.anchorCol, s.row, s.col
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	return
}

// selected reports whether the cell at row/col is inside the selection.
func (s *selectMode) selected(row, col int) bool {
	if !s.selecting {
		return false
	}
	r1, c1, r2, c2 := s.span()
	if row < r1 || row > r2 {
		return false
	}
	if s.lineWise {
		return true
	}
	if r1 == r2 {
		return col >= c1 && col <= c2
	}
	switch row {
	case r1:
		return col >= c1
	case r2:
		return col <= c2
	}
	return true
}

// selectedText is what y copies: the selection, or the cursor's row when
// nothing is selected. Rows are joined with newlines and trailing blanks
// dropped, so a column of padding never travels.
func (s *selectMode) selectedText() string {
	if !s.selecting {
		return strings.TrimRight(string(s.line()), " ")
	}
	r1, c1, r2, c2 := s.span()
	var out []string
	for r := r1; r <= r2; r++ {
		line := s.text[r]
		from, to := 0, len(line)
		if !s.lineWise {
			if r == r1 {
				from = min(c1, len(line))
			}
			if r == r2 {
				to = min(c2+1, len(line))
			}
		}
		if from > to {
			from = to
		}
		out = append(out, strings.TrimRight(string(line[from:to]), " "))
	}
	return strings.Join(out, "\n")
}

// -------------------------------------------------------------- search

// search recomputes the matches of query. Smart case (ux.md §1.1): an
// all-lowercase query ignores case, one with a capital does not.
func (s *selectMode) search() {
	s.matches, s.cur = nil, -1
	q := []rune(s.query)
	if len(q) == 0 {
		return
	}
	fold := strings.ToLower(s.query) == s.query
	if fold {
		q = []rune(strings.ToLower(s.query))
	}
	for r, line := range s.text {
		hay := line
		if fold {
			hay = []rune(strings.ToLower(string(line)))
		}
		for i := 0; i+len(q) <= len(hay); i++ {
			if runesEqual(hay[i:i+len(q)], q) {
				s.matches = append(s.matches, match{row: r, start: i, end: i + len(q)})
				i += len(q) - 1
			}
		}
	}
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// firstMatchFrom is the first match at or after row/col, wrapping to the
// first match of all; -1 when there are none.
func (s *selectMode) firstMatchFrom(row, col int) int {
	for i, m := range s.matches {
		if m.row > row || (m.row == row && m.start >= col) {
			return i
		}
	}
	if len(s.matches) > 0 {
		return 0
	}
	return -1
}

func (s *selectMode) jumpMatch(i int) {
	if i < 0 || i >= len(s.matches) {
		return
	}
	s.cur = i
	s.row, s.col = s.matches[i].row, s.matches[i].start
}

// matchAt says whether row/col is inside a match, and whether that match
// is the current one.
func (s *selectMode) matchAt(row, col int) (in, current bool) {
	for i, m := range s.matches {
		if m.row == row && col >= m.start && col < m.end {
			return true, i == s.cur
		}
	}
	return false, false
}

// status is the URL row's text while searching: /query and the count.
func (s *selectMode) status() string {
	if !s.typing && s.query == "" {
		return ""
	}
	out := "/" + s.query
	if len(s.matches) > 0 {
		out += "  " + itoa(s.cur+1) + "/" + itoa(len(s.matches))
	} else if s.query != "" {
		out += "  0/0"
	}
	return out
}

// ------------------------------------------------------------ drawing

// selectRows draws the page window with the character cursor, the
// selection and the matches over the ordinary colours. Cell by cell: the
// overlays cut across segment boundaries, so segments are not the unit.
func (m AppModel) selectRows(t *tab, innerW, innerH int) []string {
	s := &m.sel
	styles := segStyles()
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	sel := lipgloss.NewStyle().Foreground(textColor).Background(lipgloss.Color("#45475a"))
	hit := lipgloss.NewStyle().Foreground(textColor).Background(borderDim)
	hitCur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(selectColor)

	out := make([]string, 0, innerH)
	end := min(len(t.lay.rows), t.top+innerH)
	for r := t.top; r < end; r++ {
		var b strings.Builder
		used, col := 0, 0
		for _, sg := range t.lay.rows[r].segs {
			for _, ch := range sg.text {
				w := dispW(string(ch))
				if used+w > innerW {
					break
				}
				style := styles[sg.kind]
				if in, isCur := s.matchAt(r, col); in {
					style = hit
					if isCur {
						style = hitCur
					}
				}
				if s.selected(r, col) {
					style = sel
				}
				if r == s.row && col == s.col {
					style = cur
				}
				b.WriteString(style.Render(string(ch)))
				used += w
				col++
			}
		}
		// A cursor past the end of a short row still has to be seen.
		if r == s.row && s.col >= col && used < innerW {
			b.WriteString(cur.Render(" "))
			used++
		}
		b.WriteString(strings.Repeat(" ", max(0, innerW-used)))
		out = append(out, b.String())
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return out
}

// selectLegendPairs is the footer while the mode is on (ux.md §B): only
// the keys that mean something here.
func selectLegendPairs(typing bool) [][2]string {
	if typing {
		return [][2]string{{"Enter", "find"}, {"Esc", "cancel"}}
	}
	return [][2]string{
		{"y", "copy"},
		{"v/V", "select"},
		{"Esc", "leave"},
		{"/", "search"},
		{"n/N", "next/prev"},
		{"Enter", "click"},
		{"hjkl", "move"},
		{"w/e/b", "word"},
		{"0/$", "line ends"},
		{"u/d", "half page"},
	}
}

// selectCheatsheet is what Space shows in the mode (ux.md §A.1): every
// key, and pressing one runs it.
var selectCheatsheet = []string{
	"  h j k l        move by character and row",
	"  w e b          next word, word end, previous word",
	"  0 $            start / end of the row",
	"  u d            half a page up / down",
	"  gg G           top / bottom of the page",
	"  v V            select by character / by row",
	"  y              copy the selection (or this row)",
	"  /              search; Enter finds, n N step",
	"  Enter          click what the cursor is on",
	"  Esc            cancel the search, then leave",
}
