package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
		if k == "enter" {
			k = "Enter"
		}
		res, text = s.key(k, 10)
	}
	return res, text
}

func TestSelectMotionsAndYank(t *testing.T) {
	tb := selectFixture()
	if got := strings.Join([]string{tb.lay.rows[0].plain(), tb.lay.rows[2].plain()}, "|"); got != "alpha beta gamma|see \U000f0337 the Link now" {
		t.Fatalf("fixture rows: %q", got)
	}
	var s selectMode
	s.enter(tb)
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
	s.enter(tb)
	find := func(q string) {
		s.query = q
		s.search()
		s.jumpMatch(s.firstMatchFrom(s.row, s.col))
	}
	find("delta")
	if len(s.matches) != 2 {
		t.Fatalf("lowercase query matches both cases: got %d", len(s.matches))
	}
	if s.cur != 0 || s.row != 4 || s.col != 0 {
		t.Errorf("a search jumps to the first match: cur %d at %d/%d", s.cur, s.row, s.col)
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

	find("Delta")
	if len(s.matches) != 1 {
		t.Errorf("a capital makes it case-sensitive: %d matches", len(s.matches))
	}

	find("Link")
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

}

func TestSelectRowsDrawTheCursor(t *testing.T) {
	tb := selectFixture()
	m := New("")
	m.tabs = append(m.tabs, tb)
	m.shown = 0
	m.sel.enter(tb)
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

// Code is in the family's syntax colours (terminu.Syntax): a JSON key, a
// number, a string and a constant each its own ink, every character kept;
// an escape in the page's text stays itself, for the row to name (tdp
// components.5), and a language no lexer knows is plain.
func TestCodeInTheFamilysSyntaxColours(t *testing.T) {
	lines := syntaxLines("json", "{\n  \"a\": 1,\n  \"b\": \"x\",\n  \"c\": true\n}")
	if len(lines) != 5 {
		t.Fatalf("lines: %d", len(lines))
	}
	inks := map[string]lipgloss.Color{}
	var all strings.Builder
	for i, l := range lines {
		if i > 0 {
			all.WriteString("\n")
		}
		for _, sg := range l {
			inks[strings.TrimSpace(sg.text)] = sg.ink
			all.WriteString(sg.text)
		}
	}
	if all.String() != "{\n  \"a\": 1,\n  \"b\": \"x\",\n  \"c\": true\n}" {
		t.Errorf("the text changed: %q", all.String())
	}
	key, num, str, con := inks[`"a"`], inks["1"], inks[`"x"`], inks["true"]
	if key == "" || num == "" || str == "" || con == "" || key == num || num == str || str == con {
		t.Errorf("inks: %v", inks)
	}
	esc := syntaxLines("go", "x := \"\x1b[31m\"")
	var got strings.Builder
	for _, sg := range esc[0] {
		got.WriteString(sg.text)
	}
	if got.String() != "x := \"\x1b[31m\"" {
		t.Errorf("an escape in the text should stay itself: %q", got.String())
	}
	if l := syntaxLines("no-such-language", "x"); len(l) != 1 || len(l[0]) != 1 || l[0][0].text != "x" {
		t.Errorf("a language no lexer knows should be plain: %+v", l)
	}
}
