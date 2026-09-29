package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// corners are the top-left glyphs of [1] and [2] on the panels' top row.
func corners(t *testing.T, d *driver) (string, string) {
	t.Helper()
	for _, l := range strings.Split(ansi.Strip(d.m.View()), "\n") {
		if strings.Contains(l, "[1] Tabs") {
			r := []rune(l)
			return string(r[0]), string(r[sideW])
		}
	}
	t.Fatal("no panel row")
	return "", ""
}

// rowWidths is every row's width on screen.
func rowWidths(d *driver) []int {
	var w []int
	for _, l := range strings.Split(ansi.Strip(d.m.View()), "\n") {
		w = append(w, ansi.StringWidth(l))
	}
	return w
}

// The focus is told by the line as well as the colour: a double line on
// the focused panel, a round one on the other, the same width, so moving
// the focus moves nothing (tdp L5 v0.1.19).
func TestFocusIsADoubleLine(t *testing.T) {
	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/", title: "Example"}}
	d.m.shown = 0

	d.key("1")
	if a, b := corners(t, d); a != "╔" || b != "╭" {
		t.Errorf("focus on [1]: corners %q %q, want ╔ ╭", a, b)
	}
	before := rowWidths(d)
	d.key("2")
	if a, b := corners(t, d); a != "╭" || b != "╔" {
		t.Errorf("focus on [2]: corners %q %q, want ╭ ╔", a, b)
	}
	after := rowWidths(d)
	if len(before) != len(after) {
		t.Fatalf("rows: %d, then %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("row %d: %d cells, then %d when the focus moved", i, before[i], after[i])
		}
	}
	// In visual mode the border turns Yellow and stays a double line.
	d.m.sel.on = true
	if _, b := corners(t, d); b != "╔" {
		t.Errorf("visual mode: [2] should keep the double line, got %q", b)
	}
}

// The bottom border that carries [2]'s status is the same line as the
// rest of the frame.
func TestStatusBorderKeepsTheLine(t *testing.T) {
	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/", loading: true}}
	d.m.shown = 0
	for _, tc := range []struct {
		focus       panelID
		left, right string
	}{{panelPage, "╚", "╝"}, {panelTabs, "╰", "╯"}} {
		d.m.focus = tc.focus
		lines := strings.Split(ansi.Strip(d.m.pagePanel(80, 20)), "\n")
		bottom := []rune(lines[len(lines)-1])
		if !strings.Contains(string(bottom), "loading") || string(bottom[0]) != tc.left || string(bottom[len(bottom)-1]) != tc.right {
			t.Errorf("focus %v: %q, want %s…%s", tc.focus, string(bottom), tc.left, tc.right)
		}
	}
}
