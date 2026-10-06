package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/chromedp/cdproto/cdp"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
)

// wideIcon is a Nerd Font icon: one cell where the terminal moves the
// cursor one for it, two where it moves two.
var wideIcon = string(rune(0xf015))

// withIcons sets how many cells an icon takes for one test.
func withIcons(t *testing.T, n int) {
	t.Helper()
	was := iconCells
	iconCells = n
	t.Cleanup(func() { iconCells = was })
}

// d6Driver is the app, 100 × 30, on a page whose headings, text, links and
// title carry icons, the focus on [2].
func d6Driver(t *testing.T) *driver {
	t.Helper()
	d := keysDriver(t)
	tb := &tab{id: 1, url: "https://example.com/" + strings.Repeat("path/", 30) + wideIcon, title: "Home " + wideIcon}
	tb.root = doc(hd(1, "Title "+wideIcon), para(text("lede "+wideIcon+" and more")),
		hd(2, "Part "+wideIcon), para(link("go "+wideIcon, "https://example.com/x", 5)),
		hd(2, "Other"), para(text(strings.Repeat(wideIcon+" word ", 30))))
	d.m.tabs, d.m.shown = []*tab{tb}, 0
	tb.relayout(d.m.pageW())
	d.key("2")
	return d
}

// exactRows fails unless every line of s is w cells, and there are h lines
// when h > 0.
func exactRows(t *testing.T, where string, s string, w, h int) {
	t.Helper()
	lines := strings.Split(s, "\n")
	if h > 0 && len(lines) != h {
		t.Errorf("%s: %d lines, want %d", where, len(lines), h)
	}
	for i, l := range lines {
		if got := dispW(l); got != w {
			t.Errorf("%s: line %d is %d cells, want %d: %q", where, i, got, w, ansi.Strip(l))
			return
		}
	}
}

// tdp D6, L4: with icons one cell wide and two, the screen and every popup —
// the box alone, and laid over the screen — keep every line exactly as wide
// as it should be.
func TestD6EveryPopupEveryLineExact(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"+wideIcon+".html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, cells := range []int{1, 2} {
		withIcons(t, cells)
		for _, p := range []struct {
			name string
			open func(d *driver) (func() bool, func() string)
		}{
			{"none", func(d *driver) (func() bool, func() string) { return func() bool { return true }, nil }},
			{"space menu", func(d *driver) (func() bool, func() string) {
				d.key(" ")
				return func() bool { return d.m.spaceMenu.anim.isInteractive() }, func() string { return d.m.spaceMenu.view() }
			}},
			{"help", func(d *driver) (func() bool, func() string) {
				d.key("?")
				return func() bool { return d.m.help.anim.isInteractive() }, func() string { return d.m.help.view() }
			}},
			{"toast", func(d *driver) (func() bool, func() string) {
				d.exec(d.m.toast.show("copied "+wideIcon, toastInfo))
				return func() bool { return d.m.toast.anim.isInteractive() }, func() string { return d.m.toast.view() }
			}},
			{"message", func(d *driver) (func() bool, func() string) {
				d.exec(d.m.message.show(glyphInfo, "Note "+wideIcon, []string{"a line " + wideIcon}, 1))
				return func() bool { return d.m.message.anim.isInteractive() }, func() string { return d.m.message.view() }
			}},
			{"confirm", func(d *driver) (func() bool, func() string) {
				d.exec(d.m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Delete " + wideIcon, lines: []string{"gone " + wideIcon}, accept: "delete"}, 1))
				return func() bool { return d.m.confirm.anim.isInteractive() }, func() string { return d.m.confirm.view() }
			}},
			{"input", func(d *driver) (func() bool, func() string) {
				d.key("L")
				return func() bool { return d.m.input.anim.isInteractive() }, func() string { return d.m.input.view() }
			}},
			{"editor", func(d *driver) (func() bool, func() string) {
				d.exec(d.m.editor.ask("Notes "+wideIcon, "", "a "+wideIcon+"\nb", 0, 1))
				return func() bool { return d.m.editor.anim.isInteractive() }, func() string { return d.m.editor.view() }
			}},
			{"picker", func(d *driver) (func() bool, func() string) {
				d.exec(d.m.picker.open(glyphFolder, "Import "+wideIcon, dir, 1))
				return func() bool { return d.m.picker.anim.isInteractive() }, func() string { return d.m.picker.view() }
			}},
			{"finder", func(d *driver) (func() bool, func() string) {
				d.key("/")
				return func() bool { return d.m.finder.anim.isInteractive() }, func() string { return d.m.finder.view() }
			}},
			{"page popup", func(d *driver) (func() bool, func() string) {
				tb := d.page()
				tb.root.Children = append(tb.root.Children, &ir.Node{Kind: ir.Landmark, Role: "dialog", Modal: true, ID: 42,
					Name: "Hi " + wideIcon, Children: []*ir.Node{para(text("inside " + wideIcon)), para(link("ok "+wideIcon, "https://example.com/ok", 43))}})
				tb.popups = []cdp.BackendNodeID{42}
				tb.relayout(d.m.pageW())
				return func() bool { return tb.popupNode() != nil }, nil
			}},
			{"network detail", func(d *driver) (func() bool, func() string) {
				d.m.devtools.tab = devNetwork
				d.exec(d.m.devtools.anim.open())
				d.exec(d.m.devtools.detail.show(page.NetEntry{URL: "https://example.com/" + strings.Repeat("a/", 60) + wideIcon, Method: "GET"}, 2))
				// The box: DevTools with the detail laid over it.
				return func() bool { return d.m.devtools.detail.anim.isInteractive() }, func() string { return d.m.devtools.view() }
			}},
			{"devtools", func(d *driver) (func() bool, func() string) {
				d.m.devtools.tab = devNetwork
				d.exec(d.m.devtools.anim.open())
				return func() bool { return d.m.devtools.anim.isInteractive() }, func() string { return d.m.devtools.view() }
			}},
		} {
			d := d6Driver(t)
			up, box := p.open(d)
			d.until(p.name, up)
			where := p.name + " at " + itoa(cells)
			if box != nil {
				v := box()
				exactRows(t, where+" alone", v, dispW(strings.Split(v, "\n")[0]), 0)
			}
			exactRows(t, where+" on the screen", d.m.View(), 100, 30)
			if p.name == "network detail" {
				// Its title keeps the loading icon after the URL: the URL is
				// cut by what the two icons take (tdp F7, D6).
				top := strings.Split(ansi.Strip(d.m.devtools.detail.view()), "\n")[0]
				turning := false
				for _, f := range spinnerFrames {
					turning = turning || strings.Contains(top, f)
				}
				if !turning {
					t.Errorf("%s: the title should keep its loading icon: %q", where, top)
				}
			}
		}
	}
}

// The list screens and the narrow layout with icons two cells wide.
func TestD6ScreensEveryLineExact(t *testing.T) {
	withIcons(t, 2)
	for _, size := range [][2]int{{100, 30}, {60, 15}} {
		d := d6Driver(t)
		d.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		exactRows(t, "web "+itoa(size[0]), d.m.View(), size[0], size[1])
		d.key("/") // the search stacks its list over its preview below 96 columns
		d.until("the finder", func() bool { return d.m.finder.anim.isInteractive() })
		exactRows(t, "finder "+itoa(size[0]), d.m.View(), size[0], size[1])
		v := d.m.finder.view()
		exactRows(t, "finder box "+itoa(size[0]), v, dispW(strings.Split(v, "\n")[1]), 0)
		d.key("esc")
		for _, s := range []string{"B", "H", "D", "S"} {
			d.key(s)
			exactRows(t, "screen "+s+" "+itoa(size[0]), d.m.View(), size[0], size[1])
		}
	}
}

// The loading icon and the space held for it are the same width, turning
// or still, however wide the terminal draws an icon (tdp F7, D6).
func TestD6LoadingIconHoldsItsWidth(t *testing.T) {
	for _, cells := range []int{1, 2} {
		withIcons(t, cells)
		if on, off := dispW(loadingIcon(true)), dispW(loadingIcon(false)); on != off || on != cells {
			t.Errorf("%d cells: turning %d, still %d", cells, on, off)
		}
	}
}

// composite lays fg over bg by the terminal's width. Each case is written
// out, icons two cells: a popup row with an icon, an icon under the popup,
// and an icon cut by the popup's left or right edge (the half outside turns
// into a space).
func TestD6Composite(t *testing.T) {
	withIcons(t, 2)
	I := wideIcon
	for _, tc := range []struct {
		name, fg, bg string
		x            int
		want         string
	}{
		{"an icon in the popup", "[" + I + "]", "abcdefghij", 2, "ab[" + I + "]ghij"},
		{"an icon under, left of it", "[]", I + "cdefghij", 4, I + "cd[]ghij"},
		{"an icon cut by the left edge", "[]", "a" + I + "defghij", 2, "a []efghij"},
		{"an icon cut by the right edge", "[]", "abc" + I + "fghij", 2, "ab[]" + " " + "fghij"},
	} {
		got := composite(tc.fg, tc.bg, overlay.Left, overlay.Top, tc.x, 0)
		if got != tc.want || dispW(got) != 10 {
			t.Errorf("%s: %q (%d cells), want %q", tc.name, got, dispW(got), tc.want)
		}
	}
}

// cutLeft drops cells from the left; center centres like lipgloss.Place;
// joinH pads each column to its own width.
func TestD6CutCenterJoin(t *testing.T) {
	withIcons(t, 2)
	I := wideIcon
	if got := cutLeft(I+"abc", 1); got != " abc" {
		t.Errorf("cutting into an icon leaves a space: %q", got)
	}
	if got := cutLeft(I+"abc", 2); got != "abc" {
		t.Errorf("cutting past an icon: %q", got)
	}
	if got := center(7, 0, I+"a"); got != "  "+I+"a  " {
		t.Errorf("centred by width: %q", got)
	}
	if got := joinH(I+"\nab", "|"); got != I+"|\nab " {
		t.Errorf("each column its own width: %q", got)
	}
	if got := dispW("a" + I + "\n" + I + I); got != 4 {
		t.Errorf("a block is its widest line: %d", got)
	}
}

// A box wider or taller than the screen — drawn at the old size in the
// frame a resize lands in — starts at 0 and is cut at the screen's edge;
// it never panics, and the screen keeps its size (tdp D6 v0.1.21).
func TestD6OversizedOverlayIsCut(t *testing.T) {
	screen := strings.TrimSuffix(strings.Repeat("abcdefghij\n", 5), "\n") // 10 × 5
	row := func(w int) string { return "[" + wideIcon + strings.Repeat("x", w-3) + "]" }
	block := func(w, h int) string {
		lines := make([]string, h)
		for i := range lines {
			lines[i] = row(w)
		}
		return strings.Join(lines, "\n")
	}
	for _, cells := range []int{1, 2} {
		withIcons(t, cells)
		for name, fg := range map[string]string{
			"wider":  block(14, 3),
			"taller": block(6, 8),
			"both":   block(14, 8),
		} {
			for _, pos := range []struct {
				x, y   overlay.Position
				dx, dy int
			}{{overlay.Center, overlay.Center, 0, 0}, {overlay.Center, overlay.Bottom, 0, -2}, {overlay.Left, overlay.Top, 3, 3}} {
				var got string
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("%s at %d cells: panicked: %v", name, cells, r)
						}
					}()
					got = composite(fg, screen, pos.x, pos.y, pos.dx, pos.dy)
				}()
				if got == "" {
					continue
				}
				exactRows(t, name+" at "+itoa(cells), got, 10, 5)
			}
		}
	}
}
