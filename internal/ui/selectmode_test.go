package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/ir"
)

// a page of three paragraphs and one link, laid out wide enough that
// nothing wraps, so row numbers are predictable.
func selectFixture() *tab {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Text, Name: "alpha beta gamma"}}},
		{Kind: ir.Paragraph, Children: []*ir.Node{
			{Kind: ir.Text, Name: "see "},
			{Kind: ir.Link, Name: "the Link", URL: "u"},
			{Kind: ir.Text, Name: " now"},
		}},
		{Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Text, Name: "Delta delta"}}},
	}}
	t := &tab{cursor: -1}
	t.root = root
	t.relayout(60)
	return t
}

func press(s *selectMode, keys ...string) (selResult, string) {
	var res selResult
	var text string
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		}
		res, text = s.key(msg, 10)
	}
	return res, text
}

func TestSelectMotionsAndYank(t *testing.T) {
	tb := selectFixture()
	if got := strings.Join([]string{tb.lay.rows[0].plain(), tb.lay.rows[2].plain()}, "|"); got != "alpha beta gamma|see 󰌷 the Link now" {
		t.Fatalf("fixture rows: %q", got)
	}
	var s selectMode
	s.enter(tb, false)
	if !s.on || s.row != 0 || s.col != 0 {
		t.Fatalf("enter: %+v", s)
	}
	press(&s, "w")
	if s.col != 6 {
		t.Errorf("w → col %d, want 6 (beta)", s.col)
	}
	press(&s, "e")
	if s.col != 9 {
		t.Errorf("e → col %d, want 9 (end of beta)", s.col)
	}
	press(&s, "w", "w") // gamma, then off the end onto the next non-empty row
	if s.row != 2 || s.col != 0 {
		t.Errorf("w past the end → %d/%d, want 2/0", s.row, s.col)
	}
	press(&s, "b")
	if s.row != 0 || s.col != 11 {
		t.Errorf("b back up → %d/%d, want 0/11 (gamma)", s.row, s.col)
	}
	press(&s, "$")
	if s.col != 15 {
		t.Errorf("$ → %d", s.col)
	}
	press(&s, "0", "v", "e")
	res, text := press(&s, "y")
	if res != selYank || text != "alpha" {
		t.Errorf("v e y → %v %q", res, text)
	}
	if s.selecting {
		t.Error("selection should end after a yank")
	}
	press(&s, "V", "j", "j")
	_, text = press(&s, "y")
	if text != "alpha beta gamma\n\nsee 󰌷 the Link now" {
		t.Errorf("V j j y → %q", text)
	}
	press(&s, "gg")
	if s.row != 0 {
		t.Errorf("gg → row %d", s.row)
	}
	press(&s, "G")
	if s.row != len(s.text)-1 {
		t.Errorf("G → row %d of %d", s.row, len(s.text))
	}
	_, text = press(&s, "y")
	if text != "Delta delta" {
		t.Errorf("y with no selection copies the row: %q", text)
	}
}

func TestSelectSearchSmartCaseAndClick(t *testing.T) {
	tb := selectFixture()
	var s selectMode
	s.enter(tb, true)
	if !s.typing {
		t.Fatal("/ should start typing")
	}
	press(&s, "d", "e", "l", "t", "a")
	if len(s.matches) != 2 {
		t.Fatalf("lowercase query matches both cases: got %d", len(s.matches))
	}
	press(&s, "enter")
	if s.typing || s.cur != 0 || s.row != 4 || s.col != 0 {
		t.Errorf("Enter jumps to the first match: cur %d at %d/%d typing %v", s.cur, s.row, s.col, s.typing)
	}
	press(&s, "n")
	if s.cur != 1 || s.col != 6 {
		t.Errorf("n → cur %d col %d", s.cur, s.col)
	}
	press(&s, "n")
	if s.cur != 0 {
		t.Errorf("n wraps: cur %d", s.cur)
	}
	if st := s.status(); st != "/delta  1/2" {
		t.Errorf("status %q", st)
	}

	press(&s, "/", "D", "e", "l", "t", "a", "enter")
	if len(s.matches) != 1 {
		t.Errorf("a capital makes it case-sensitive: %d matches", len(s.matches))
	}

	press(&s, "/", "L", "i", "n", "k", "enter")
	if s.row != 2 {
		t.Fatalf("Link is on row 2, cursor on %d", s.row)
	}
	if i := tb.lay.itemAtCol(s.row, s.col); i != 0 || tb.lay.items[i].node.Kind != ir.Link {
		t.Errorf("the match sits on the link item: got %d", i)
	}
	res, _ := press(&s, "enter")
	if res != selClick {
		t.Errorf("Enter on a match asks for a click: %v", res)
	}

	// Esc while typing cancels the search and stays; Esc again leaves.
	press(&s, "/", "x")
	res, _ = press(&s, "esc")
	if res != selNone || !s.on || s.query != "" || s.typing {
		t.Errorf("Esc while typing: %v on=%v query=%q", res, s.on, s.query)
	}
	res, _ = press(&s, "esc")
	if res != selLeave || s.on {
		t.Errorf("Esc leaves: %v on=%v", res, s.on)
	}
}

func TestSelectRowsDrawTheCursor(t *testing.T) {
	tb := selectFixture()
	m := New(nil, "")
	m.tabs = append(m.tabs, tb)
	m.shown = 0
	m.sel.enter(tb, false)
	m.sel.col = 6
	rows := m.selectRows(tb, 40, 3)
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	for i, r := range rows {
		if w := dispW(r); w != 40 {
			t.Errorf("row %d is %d wide", i, w)
		}
	}
	if !strings.Contains(rows[0], "b") {
		t.Errorf("row 0 lost its text: %q", rows[0])
	}
}

func TestVEntersSelectionFromAnyPanel(t *testing.T) {
	m := New(nil, "")
	tb := selectFixture()
	m.tabs = append(m.tabs, tb)
	m.shown = 0
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	mm := model.(AppModel)
	if !mm.sel.on || mm.sel.typing || mm.focus != panel3 {
		t.Errorf("V: on=%v typing=%v focus=%d", mm.sel.on, mm.sel.typing, mm.focus)
	}
	mm.focus = panel2
	mm.sel.on = false
	model, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	mm = model.(AppModel)
	if !mm.sel.on || mm.focus != panel3 {
		t.Errorf("V from [2] moves to [3] first: on=%v focus=%d", mm.sel.on, mm.focus)
	}
}

func TestWrapWords(t *testing.T) {
	got := wrapWords("An iframe which has both allow-scripts and allow-same-origin", 20)
	for i, l := range got {
		if dispW(l) > 20 {
			t.Errorf("line %d too wide: %q", i, l)
		}
	}
	if strings.Join(got, " ") != "An iframe which has both allow-scripts and allow-same-origin" {
		t.Errorf("words lost or reordered: %q", got)
	}
	if long := wrapWords(strings.Repeat("x", 25), 10); len(long) != 3 {
		t.Errorf("a long word is cut: %q", long)
	}
}

func TestHighlightJSONKeysAndValues(t *testing.T) {
	lines := highlightLines("json", "{\n  \"a\": 1,\n  \"b\": \"x\",\n  \"c\": true\n}")
	if len(lines) != 5 {
		t.Fatalf("lines: %d", len(lines))
	}
	kinds := map[string]segKind{}
	for _, l := range lines {
		for _, s := range l {
			kinds[strings.TrimSpace(s.text)] = s.kind
		}
	}
	if kinds[`"a"`] != segCodeKey || kinds["1"] != segCodeNumber || kinds[`"x"`] != segCodeString || kinds["true"] != segCodeConst {
		t.Errorf("kinds: %v", kinds)
	}
	if highlightLines("", "x") != nil || highlightLines("no-such-language", "x") != nil {
		t.Error("no lexer means no highlight, not a crash")
	}
	// YAML and TOML keys are keys too.
	y := highlightLines("yaml", "name: webu\nport: 8080\n")
	found := false
	for _, s := range y[0] {
		if strings.TrimSpace(s.text) == "name" && s.kind == segCodeKey {
			found = true
		}
	}
	if !found {
		t.Errorf("yaml key not marked: %+v", y[0])
	}
	// A folded highlighted line keeps its segments' kinds.
	folded := foldSegs([]seg{{text: `"key"`, kind: segCodeKey}, {text: `: "` + strings.Repeat("v", 30) + `"`, kind: segCodeString}}, 12)
	if len(folded) < 3 || folded[0][0].kind != segCodeKey {
		t.Errorf("fold: %+v", folded)
	}
	for i, l := range folded {
		w := 0
		for _, s := range l {
			w += dispW(s.text)
		}
		if w > 12 {
			t.Errorf("folded row %d is %d wide", i, w)
		}
	}
}

func TestCodeRowsAndTableHeaders(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Code, Children: []*ir.Node{{Kind: ir.Text, Name: "one\ntwo"}}},
		{Kind: ir.Table, Children: []*ir.Node{{Kind: ir.Row, Children: []*ir.Node{
			{Kind: ir.Cell, Header: true, Children: []*ir.Node{{Kind: ir.Text, Name: "H"}}},
			{Kind: ir.Cell, Children: []*ir.Node{{Kind: ir.Text, Name: "c"}}},
		}}}},
	}}
	l := render(root, 40)
	if !l.rows[0].code || !l.rows[1].code {
		t.Errorf("code block rows should be flagged: %+v", l.rows[:2])
	}
	var header, cell segKind = segPlain, segPlain
	for _, r := range l.rows {
		for _, s := range r.segs {
			if s.text == "H" {
				header = s.kind
			}
			if s.text == "c" {
				cell = s.kind
			}
		}
	}
	if header != segTableHeader || cell != segPlain {
		t.Errorf("header kind %d, cell kind %d", header, cell)
	}
}

func TestOutlineEntriesAndJump(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{
		{Kind: ir.Landmark, Role: "banner", Children: []*ir.Node{{Kind: ir.Text, Name: "top"}}},
		{Kind: ir.Landmark, Role: "main", Children: []*ir.Node{
			{Kind: ir.Heading, Level: 1, Children: []*ir.Node{{Kind: ir.Text, Name: "Title"}}},
			{Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Text, Name: "body"}}},
			{Kind: ir.Landmark, Role: "region", Name: "Sec", Children: []*ir.Node{
				{Kind: ir.Heading, Level: 2, Children: []*ir.Node{{Kind: ir.Text, Name: "Sub"}}},
				{Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Link, Name: "go", URL: "u"}}},
			}},
		}},
	}}
	tb := &tab{cursor: -1, root: root}
	tb.relayout(60)
	entries := outlineEntries(root)
	labels := make([]string, len(entries))
	for i, e := range entries {
		labels[i] = e.label
	}
	want := []string{"banner", "main", "  # Title", "  region Sec", "    ## Sub"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Fatalf("outline:\n%s\nwant:\n%s", strings.Join(labels, "\n"), strings.Join(want, "\n"))
	}
	for _, e := range entries {
		if _, ok := tb.lay.marks[e.node]; !ok {
			t.Errorf("no row mark for %q", e.label)
		}
	}
	tb.jumpTo(entries[4].node, 10) // ## Sub
	if n := tb.current(); n == nil || n.Kind != ir.Heading || n.Text() != "Sub" {
		t.Errorf("jump to a heading lands on it: %+v", n)
	}
	tb.jumpTo(entries[3].node, 10) // region Sec: its own rule row is the item
	if n := tb.current(); n == nil || n.Kind != ir.Landmark || n.Name != "Sec" {
		t.Errorf("jump to a landmark lands on its rule: %+v", n)
	}
	if tb.top != tb.lay.marks[entries[3].node] {
		t.Errorf("top %d, want the landmark's row %d", tb.top, tb.lay.marks[entries[3].node])
	}
}
