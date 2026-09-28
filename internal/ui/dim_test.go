package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// withColour renders in truecolor for the test: go test's stdout is not a
// terminal, and lipgloss would otherwise draw no colour at all.
func withColour(t *testing.T) {
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(was) })
}

var (
	fgRe = regexp.MustCompile(`38;2;(\d+;\d+;\d+)`)
	bgRe = regexp.MustCompile(`48;2;(\d+;\d+;\d+)`)
)

// colours is every truecolor foreground in s.
type colours map[string]bool

func fgs(s string) colours { return found(fgRe, s) }

// bgs is every truecolor background in s.
func bgs(s string) colours { return found(bgRe, s) }

func found(re *regexp.Regexp, s string) colours {
	out := colours{}
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out[m[1]] = true
	}
	return out
}

// has reports whether c is among them, give or take the renderer's
// rounding of a hex colour.
func (cs colours) has(c lipgloss.Color) bool {
	r, g, b := hexRGB(string(c))
	for k := range cs {
		var x, y, z int
		fmt.Sscanf(k, "%d;%d;%d", &x, &y, &z)
		if abs(x-r) <= 2 && abs(y-g) <= 2 && abs(z-b) <= 2 {
			return true
		}
	}
	return false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// The dimmed colours the tests expect, worked out by hand from tdp D2 —
// c × 0.45 + base × 0.55, base #1e1e2e (30,30,46), rounded — not with the
// code under test.
const (
	dimLayer1 = lipgloss.Color("#5a678a") // #A4C0FA (164,192,250) → 90,103,138
	dimLayer2 = lipgloss.Color("#536888") // #94C3F5 (148,195,245) → 83,104,136
	dimWarn   = lipgloss.Color("#7e4f65") // #f38ba8 (243,139,168) → 126,79,101
	dimLive   = lipgloss.Color("#5b7762") // #a6e3a1 (166,227,161) → 91,119,98
	dimFocus  = lipgloss.Color("#4e628a") // #89b4fa (137,180,250) → 78,98,138
	dimHand   = lipgloss.Color("#64687d") // #bac2de (186,194,222) → 100,104,125
	dimTextC  = lipgloss.Color("#6d7187") // #cdd6f4 (205,214,244) → 109,113,135
)

func TestDimANSI(t *testing.T) {
	// A layer-coloured border; warning text, bold and reversed, on the
	// hand colour; plain text with no colour; a 256-colour red; black.
	in := "\x1b[38;2;148;195;245m╭──╮\x1b[0m plain " +
		"\x1b[1;7;38;2;243;139;168;48;2;186;194;222merror\x1b[0m " +
		"\x1b[38;5;196mred\x1b[0m \x1b[38;2;0;0;0mblack\x1b[0m"
	out := dimANSI(in)

	if ansi.Strip(out) != ansi.Strip(in) {
		t.Fatalf("dimming should change colours only: %q", ansi.Strip(out))
	}
	fg, bg := fgs(out), bgs(out)
	for what, c := range map[string]lipgloss.Color{
		"a layer border fades, still its layer's colour":  dimLayer2,
		"a warning fades, not turned into one grey":       dimWarn,
		"text with no colour gets the dimmed text colour": dimTextC,
	} {
		if !fg.has(c) {
			t.Errorf("%s: want %s among %v", what, c, fg)
		}
	}
	if !bg.has(dimHand) {
		t.Errorf("a background fades, it does not go: want %s among %v", dimHand, bg)
	}
	if !strings.Contains(out, "1;7;") {
		t.Errorf("bold and reverse stay: %q", out)
	}
	// 256-colour 196 is rgb(255,0,0): 131,0,0 — its zero channels stay zero.
	if !fg.has("#830000") {
		t.Errorf("a 256-colour red should fade as its RGB: %v", fg)
	}
	// Never lighter (D2): black fades toward the base but keeps its 0,0,0.
	if !strings.Contains(out, "38;2;0;0;0m") {
		t.Errorf("black should stay black, not lighten toward the base: %q", out)
	}
	// The screen is cut and rejoined line by line (overlay): each line
	// opens on the dimmed text colour, not on what the last one left.
	two := dimANSI("\x1b[38;2;148;195;245mtop\nplain")
	if line := strings.Split(two, "\n")[1]; !fgs(line).has(dimTextC) {
		t.Errorf("a line with no colour of its own should open dimmed: %q", line)
	}
}

// Backgrounds fade with the rest: the header's current-screen capsule is
// still there under a popup, darker (tdp F8, D2).
func TestDimKeepsCapsules(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	header := func() colours { return bgs(strings.Split(d.m.View(), "\n")[0]) }
	if !header().has(focusColor) {
		t.Fatalf("the header's current screen is a capsule on the focus colour: %v", header())
	}
	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	if h := header(); !h.has(dimFocus) || h.has(focusColor) {
		t.Errorf("under a popup the capsule should keep its ground, faded: %v", h)
	}
}

// What is drawn on top is what Esc closes and what is bright (tdp D3,
// F8): a message opened from options is above them in all three.
func TestOneOrderForDrawingEscAndBright(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	d.m.options.setItems([]menuItem{{label: "Inspect", key: "inspect"}}, "Link", 2)
	d.exec(d.m.options.open())
	d.until("the options", func() bool { return d.m.options.anim.isInteractive() })
	d.exec(d.m.message.show(glyphInfo, "Inspect", []string{"role: link"}, 3))
	d.until("the message", func() bool { return d.m.message.anim.isInteractive() })

	if v := fgs(d.m.View()); v.has(popupLayerColor(2)) || !v.has(popupLayerColor(3)) {
		t.Errorf("the message is drawn on top and bright, the options under it dim: %v", v)
	}
	d.key("esc")
	if d.m.message.anim.owns() || !d.m.options.anim.owns() {
		t.Error("Esc should close the message, the one on top, and leave the options")
	}
}

// With a popup open only the topmost is bright (tdp F8): the base screen
// below it is dimmed, the header's live download progress too (T2's
// exception); a second popup dims the first, whose border keeps a dimmed
// version of its layer colour; a toast changes nothing; Esc brings the
// layer below back.
func TestOnlyTheTopPopupIsBright(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	d.m.dls = []download{{guid: "g1", name: "big.iso", total: 100, received: 40}}
	header := func() colours { return fgs(strings.Split(d.m.View(), "\n")[0]) }

	if !header().has(liveColor) {
		t.Fatalf("with nothing open the header's progress is live: %v", header())
	}

	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	if h := header(); h.has(liveColor) || !h.has(dimLive) {
		t.Errorf("under a popup the header's progress should fade: %v", h)
	}
	menu1 := popupLayerColor(1)
	if !fgs(d.m.View()).has(menu1) {
		t.Error("the Space menu is the top: its border should be bright")
	}

	d.key("G")
	d.key("enter")
	d.until("the global operation popup", func() bool { return d.m.globalMenu.anim.isInteractive() })
	v := fgs(d.m.View())
	if v.has(menu1) || !v.has(dimLayer1) {
		t.Errorf("the Space menu under it should keep a dimmed layer colour: %v", v)
	}
	if !v.has(popupLayerColor(2)) {
		t.Error("the global operation popup is the top: its border should be bright")
	}

	before := d.m.View()
	d.exec(d.m.toast.show("copied", toastInfo))
	d.until("the toast", func() bool { return d.m.toast.anim.isInteractive() })
	after := d.m.View()
	if strings.Split(after, "\n")[0] != strings.Split(before, "\n")[0] || !fgs(after).has(popupLayerColor(2)) {
		t.Error("a toast is not a layer: it should neither dim nor be dimmed")
	}
	d.key("esc") // the toast
	d.until("the toast gone", func() bool { return !d.m.toast.isActive() })

	d.key("esc") // the global operation popup
	// Still closing, it no longer holds the keys (tdp F3): the Space menu
	// is the top at once, and the closing box above it is dim.
	if !d.m.globalMenu.isActive() || d.m.globalMenu.anim.owns() {
		t.Fatal("the popup should be mid-close")
	}
	if v := fgs(d.m.View()); !v.has(menu1) || v.has(popupLayerColor(2)) {
		t.Errorf("mid-close, the Space menu should be bright and the closing popup dim: %v", v)
	}
	d.until("back to the Space menu", func() bool { return !d.m.globalMenu.isActive() })
	if !fgs(d.m.View()).has(menu1) {
		t.Error("with the popup above it gone, the Space menu should be bright again")
	}
	d.key("esc")
	d.until("nothing open", func() bool { return !d.m.spaceMenu.isActive() })
	if !header().has(liveColor) {
		t.Error("with nothing open, the header's progress should be live again")
	}
}
