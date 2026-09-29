package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// A mode names itself at the right end of its panel's top border, in the
// border's Yellow, and is gone when the mode is (tdp K11 v0.1.18).
func TestVisualModeNamesItself(t *testing.T) {
	withColour(t)
	for _, w := range []int{100, 40} {
		d := keysDriver(t)
		d.send(tea.WindowSizeMsg{Width: w, Height: 20})
		d.m.tabs = []*tab{{id: 1, url: "https://example.com/", title: "Example"}}
		d.m.shown = 0
		d.key("2")
		top := func() string {
			for _, l := range strings.Split(d.m.View(), "\n") {
				if strings.Contains(ansi.Strip(l), "[2] Page") {
					return l
				}
			}
			t.Fatalf("%d: no [2] top border", w)
			return ""
		}
		d.m.sel.on = true
		l := top()
		if !strings.HasSuffix(ansi.Strip(l), " Visual mode ═╗") {
			t.Errorf("%d: the top border should end with the mode: %q", w, ansi.Strip(l))
		}
		if !regexp.MustCompile("38;2;249;226;175m[^\x1b]*Visual mode").MatchString(l) { // #f9e2af
			t.Errorf("%d: the mode should be Yellow: %q", w, l)
		}
		for i, row := range strings.Split(ansi.Strip(d.m.View()), "\n") {
			if ansi.StringWidth(row) != w {
				t.Errorf("%d: row %d is %d cells", w, i, ansi.StringWidth(row))
			}
		}
		d.m.sel.on = false
		if strings.Contains(ansi.Strip(top()), "Visual mode") {
			t.Errorf("%d: out of the mode, the name goes", w)
		}
	}
}

// Where the capsule and the mode do not both fit, the mode is left out
// and the border keeps its width.
func TestModeNameGivesWay(t *testing.T) {
	for _, w := range []int{20, 24, 25, 40} {
		top := strings.Split(ansi.Strip(panelChromeMode(w, nil, "[2] Page", "Visual mode", toneSelect)), "\n")[0]
		if ansi.StringWidth(top) != w+2 {
			t.Errorf("inner %d: the top border is %d cells: %q", w, ansi.StringWidth(top), top)
		}
		if fits := w >= 10+14; strings.Contains(top, "Visual mode") != fits {
			t.Errorf("inner %d: the mode should show only where it fits: %q", w, top)
		}
	}
}

// On a panel without the focus the border hint is grey: keys Overlay0,
// the colon, the words and the status Surface2. Blue is the focus's
// colour, only where the keys go (tdp D2 v0.1.18).
func TestUnfocusedHintIsGrey(t *testing.T) {
	withColour(t)
	const blue, overlay0, surface2 = "38;2;137;179;250m", "38;2;108;112;134m", "38;2;88;91;112m" // #89b4fa, #6c7086, #585b70 as lipgloss rounds them
	for _, tc := range []struct {
		focused   bool
		key, rest string
	}{{true, blue, overlay0}, {false, overlay0, surface2}} {
		s := statusLegend("12 items", tc.focused, [2]string{"Enter", "stay"})
		if !strings.Contains(s, tc.key+"Enter") || !strings.Contains(s, tc.rest+":stay") || !strings.Contains(s, tc.rest+"12 items") {
			t.Errorf("focused %v: %q", tc.focused, s)
		}
	}

	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/", loading: true}}
	d.m.shown = 0
	for _, tc := range []struct {
		focus panelID
		want  string
	}{{panelPage, overlay0}, {panelTabs, surface2}} {
		d.m.focus = tc.focus
		lines := strings.Split(d.m.pagePanel(80, 20), "\n")
		if bottom := lines[len(lines)-1]; !strings.Contains(bottom, tc.want+"loading") {
			t.Errorf("focus %v: %q", tc.focus, bottom)
		}
	}
}
