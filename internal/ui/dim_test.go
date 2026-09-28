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

var fgRe = regexp.MustCompile(`38;2;(\d+;\d+;\d+)`)

// colours is every truecolor foreground in s.
type colours map[string]bool

func fgs(s string) colours {
	out := colours{}
	for _, m := range fgRe.FindAllStringSubmatch(s, -1) {
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

func dimOf(c lipgloss.Color) lipgloss.Color { return lerpHex(string(c), baseHex, 0.55) }

func TestDimANSI(t *testing.T) {
	withColour(t)
	border := lipgloss.NewStyle().Foreground(popupLayerColor(2)).Render("╭──╮")
	warn := lipgloss.NewStyle().Foreground(warnColor).Background(lipgloss.Color(baseHex)).Render("error")
	in := border + " plain " + warn
	out := dimANSI(in)

	if ansi.Strip(out) != ansi.Strip(in) {
		t.Fatalf("dimming should change colours only: %q", ansi.Strip(out))
	}
	got := fgs(out)
	if !got.has(dimOf(popupLayerColor(2))) || got.has(popupLayerColor(2)) {
		t.Errorf("a layer border should turn to its own dimmed colour: %v", got)
	}
	if got.has(warnColor) || !got.has(dimColor) {
		t.Errorf("a warning should turn to the dim colour: %v", got)
	}
	if strings.Contains(out, "48;") {
		t.Errorf("backgrounds should go: %q", out)
	}

	// A background's own numbers are stepped over, not read as a colour:
	// rgb(38,2,1) behind a layer-coloured border.
	r, g, b := hexRGB(string(popupLayerColor(1)))
	raw := fmt.Sprintf("\x1b[48;2;38;2;1;38;2;%d;%d;%dm╭╮\x1b[0m", r, g, b)
	if got := fgs(dimANSI(raw)); !got.has(dimOf(popupLayerColor(1))) {
		t.Errorf("the border after a background should still be read as a layer: %v", got)
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
	if h := header(); h.has(liveColor) || !h.has(dimColor) {
		t.Errorf("under a popup the header should be dim, its progress too: %v", h)
	}
	menu1 := popupLayerColor(1)
	if !fgs(d.m.View()).has(menu1) {
		t.Error("the Space menu is the top: its border should be bright")
	}

	d.key("G")
	d.key("enter")
	d.until("the global operation popup", func() bool { return d.m.globalMenu.anim.isInteractive() })
	v := fgs(d.m.View())
	if v.has(menu1) || !v.has(dimOf(menu1)) {
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
