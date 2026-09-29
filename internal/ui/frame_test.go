package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// A mode names itself at the right end of its panel's top border, one word
// between two junctions of the border's line, Yellow and bold, and is gone
// when the mode is (tdp K11, D3 v0.1.20).
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
		if !strings.HasSuffix(ansi.Strip(l), "╡Visual╞═╗") {
			t.Errorf("%d: the top border should end with the mode between junctions: %q", w, ansi.Strip(l))
		}
		if !strings.Contains(l, "1;38;2;249;226;175mVisual") { // #f9e2af, bold
			t.Errorf("%d: the mode should be Yellow and bold: %q", w, l)
		}
		for i, row := range strings.Split(ansi.Strip(d.m.View()), "\n") {
			if ansi.StringWidth(row) != w {
				t.Errorf("%d: row %d is %d cells", w, i, ansi.StringWidth(row))
			}
		}
		d.m.sel.on = false
		if strings.Contains(ansi.Strip(top()), "Visual") {
			t.Errorf("%d: out of the mode, the name goes", w)
		}
	}
}

// The junctions are the border's own line and colour; the name is the
// mode's: on a round, unfocused border they differ.
func TestModeTagJunctionsAreTheBorder(t *testing.T) {
	withColour(t)
	top := strings.Split(panelChromeMode(40, nil, "[2] Page", "Visual", toneIdle), "\n")[0]
	if !strings.Contains(ansi.Strip(top), "┤Visual├") {
		t.Errorf("a round border takes ┤ ├: %q", ansi.Strip(top))
	}
	if !strings.Contains(top, "38;2;88;91;112m┤") || !strings.Contains(top, "1;38;2;249;226;175mVisual") { // #585b70; #f9e2af bold
		t.Errorf("junctions in the border's Surface2, the name Yellow and bold: %q", top)
	}
}

// Where the capsule and the name do not both fit, the title is cut and the
// name stays; the border keeps its width (tdp D3 v0.1.20).
func TestModeNameGivesWay(t *testing.T) {
	for _, w := range []int{12, 18, 19, 40} {
		top := strings.Split(ansi.Strip(panelChromeMode(w, nil, "[2] Page", "Visual", toneSelect)), "\n")[0]
		if ansi.StringWidth(top) != w+2 {
			t.Errorf("inner %d: the top border is %d cells: %q", w, ansi.StringWidth(top), top)
		}
		if !strings.HasSuffix(top, "╡Visual╞═╗") {
			t.Errorf("inner %d: the name should always show: %q", w, top)
		}
		if whole := w >= 10+9; strings.Contains(top, "[2] Page") != whole {
			t.Errorf("inner %d: the title should be cut only where both do not fit: %q", w, top)
		}
		if w >= 18 && !strings.Contains(top, "[2]") {
			t.Errorf("inner %d: cut, not dropped — its start stays: %q", w, top)
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

// Only the side of the search with the keys is bright (tdp F1, D3
// v0.1.18): typing, the query is Text and the row under the hand a pale
// Subtext1; on the list, the query row is all Overlay0 and the row under
// the hand the popup's layer colour, bold.
func TestFinderShowsWhichSideHasTheKeys(t *testing.T) {
	withColour(t)
	f := newFinder()
	f.kind, f.layer, f.query = finderSearch, 1, "al"
	f.hits = []hit{{part: partMain, text: "alpha"}, {part: partMain, text: "also"}}
	const text, overlay0 = "38;2;205;214;243mal", "38;2;108;112;134m" // #cdd6f4, #6c7086 as lipgloss rounds them
	subtext1 := "48;2;186;194;222m"                                   // #bac2de
	layer := lipgloss.NewStyle().Background(popupLayerColor(1)).Render("x")
	layerBg := layer[strings.Index(layer, "48;2;") : strings.Index(layer, "m")+1]

	f.mode = finderInput
	rows := f.listColumn(40, 6)
	if !strings.Contains(rows[0], text) || !strings.Contains(rows[2], subtext1) {
		t.Errorf("typing: the query should be Text and the hand Subtext1:\n%q\n%q", rows[0], rows[2])
	}
	f.mode = finderNav
	rows = f.listColumn(40, 6)
	if strings.Contains(rows[0], text) || !strings.Contains(rows[0], overlay0+" ") || strings.Count(rows[0], "\x1b[0m") != 1 {
		t.Errorf("on the list: the query row should be one run of Overlay0: %q", rows[0])
	}
	if !strings.Contains(rows[2], layerBg) || !strings.Contains(rows[2], "\x1b[1;") {
		t.Errorf("on the list: the hand should be the layer colour, bold: %q", rows[2])
	}

	// [go] has one phase — digits and j/k at once, no Tab — so it is not
	// that finder: its number stays Text and its hand Blue.
	f.kind, f.query = finderGo, "1"
	f.hits = []hit{{line: 1, text: "alpha"}}
	rows = f.listColumn(40, 6)
	if !strings.Contains(rows[0], "38;2;205;214;243m1") || !strings.Contains(rows[2], "48;2;137;179;250m") { // #cdd6f4, #89b4fa
		t.Errorf("[go] keeps its colours:\n%q\n%q", rows[0], rows[2])
	}
}

// A hint that does not fit loses whole items from its end, never half of
// one (tdp D3 v0.1.18).
func TestHintDropsWholeItems(t *testing.T) {
	pairs := [][2]string{{"j/k", "move"}, {"Enter", "run"}, {"Esc", "close"}}
	whole := dispW(hintLegend(pairs))
	for _, tc := range []struct {
		w    int
		want string
	}{
		{whole, "j/k:move Enter:run Esc:close"},
		{whole - 1, "j/k:move Enter:run"},
		{dispW(hintLegend(pairs[:1])) - 1, ""},
	} {
		if got := strings.TrimSpace(ansi.Strip(fitLegend(pairs, tc.w))); got != tc.want {
			t.Errorf("%d cells: %q, want %q", tc.w, got, tc.want)
		}
	}

	// [2]'s border: the keys give way before the status.
	keys := [][2]string{{"Enter", "stay"}}
	full := dispW(statusLegend("12 items", true, keys...))
	for _, tc := range []struct {
		w    int
		want string
	}{
		{full, "12 items Enter:stay"},
		{full - 1, "12 items"},
		{dispW(statusLegend("12 items", true)) - 1, ""},
	} {
		if got := strings.TrimSpace(ansi.Strip(fitStatus("12 items", true, keys, tc.w))); got != tc.want {
			t.Errorf("status in %d cells: %q, want %q", tc.w, got, tc.want)
		}
	}

	// The search's list box beside its preview is narrow: at 100 columns
	// its hint ends on a whole item.
	f := newFinder()
	f.setSize(100, 30)
	f.kind, f.mode = finderSearch, finderNav
	f.hits = []hit{{node: para(text("alpha")), part: partMain, text: "alpha"}}
	lines := strings.Split(ansi.Strip(f.view()), "\n")
	var bottom string
	for _, l := range lines {
		if strings.Contains(l, "╰") {
			bottom = l
			break
		}
	}
	m := boxHint.FindStringSubmatch(bottom)
	if m == nil {
		t.Fatalf("no list box bottom: %q", bottom)
	}
	_, all := f.titleAndHint()
	ok := false
	for n := 1; n <= len(all); n++ {
		ok = ok || strings.TrimSpace(m[1]) == strings.TrimSpace(ansi.Strip(hintLegend(all[:n])))
	}
	if !ok {
		t.Errorf("the hint should end on a whole item: %q", m[1])
	}
}

// [2] narrow, the hand on the pagetab: Enter:stay gives way, the count
// stays.
func TestPageBorderKeepsTheStatusLast(t *testing.T) {
	d := keysDriver(t)
	tb := &tab{id: 1, url: "https://example.com/", parts: []part{{kind: partMain}, {kind: partFooter}}, pagetab: 1}
	d.m.tabs, d.m.shown, d.m.focus = []*tab{tb}, 0, panelPage
	for _, tc := range []struct {
		outer int
		want  string
	}{{40, "0 items Enter:stay"}, {18, "0 items"}} {
		lines := strings.Split(ansi.Strip(d.m.pagePanel(tc.outer, 12)), "\n")
		m := boxHint.FindStringSubmatch(lines[len(lines)-1])
		if m == nil || strings.TrimSpace(m[1]) != tc.want {
			t.Errorf("%d wide: %q, want %q", tc.outer, lines[len(lines)-1], tc.want)
		}
	}
}

// DevTools draws its own frame; its hint gives way the same way.
func TestDevtoolsHintDropsWholeItems(t *testing.T) {
	hintAt := func(w int, tab devTab) string {
		dt := newDevtoolsPopup()
		dt.setSize(w, 30)
		dt.tab = tab
		lines := strings.Split(ansi.Strip(dt.view()), "\n")
		return strings.TrimSpace(boxHint.FindStringSubmatch(lines[len(lines)-1])[1])
	}
	for _, tab := range []devTab{devNetwork, devStorage, devConsole, devSource} {
		full, narrow := hintAt(300, tab), hintAt(40, tab)
		if narrow == full || !strings.HasPrefix(full, narrow+" ") {
			t.Errorf("%s at 40: %q should be whole items of %q", devTabLabels[tab], narrow, full)
		}
	}
}
