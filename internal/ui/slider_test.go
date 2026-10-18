package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A slider is its bar and where it stands, and Enter lists its numbers
// ten to a window, the cursor on the current one; j/k/u/d/gg/G walk them and
// Enter moves the slider there (user, 2026-09-23): an <input type=range>
// takes it as its value, an ARIA slider is stepped there with the arrows.
func TestASliderIsABarAndListsItsNumbers(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	abs, _ := filepath.Abs("testdata/slider.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the slider page", d.loaded("Slider"))
	p := d.page()

	rowOf := func(name string) string {
		for _, r := range p.lay.rows {
			if s := r.plain(); strings.Contains(s, name) && strings.Contains(s, "●") {
				return s
			}
		}
		return ""
	}
	red := rowOf("Red")
	if red == "" || !strings.Contains(red, "128") {
		t.Fatalf("the range input is a bar with its value:\n%s", dumpLayout(p.lay))
	}
	if vol := rowOf("Volume"); vol == "" || !strings.Contains(vol, "40") {
		t.Fatalf("the ARIA slider too:\n%s", dumpLayout(p.lay))
	}

	// Enter lists every number of the bar, the range in the title, the one
	// it stands on chosen; G is the last, gg the first, Enter the one under
	// the cursor.
	if vals, at := sliderValues(nodeNamed(p, "Red")); len(vals) != 256 || vals[at] != "128" {
		t.Errorf("every number of the bar, the cursor where it stands: %d, %q", len(vals), vals[at])
	}
	d.cursorOn(ir.Textbox, "Red")
	d.press("enter")
	d.until("the number list", d.shows("SLIDE: Red · 0–255"))
	d.key("G")
	d.key("enter")
	d.until("red at 255", func() bool { return strings.HasSuffix(strings.TrimSpace(rowOf("Red")), " 255") })
	d.settle()

	// An ARIA slider has no value to set: it is walked there.
	d.cursorOn(ir.Textbox, "Volume")
	d.press("enter")
	d.until("the number list", d.shows("SLIDE: Volume · 0–100"))
	d.key("g")
	d.key("g")
	d.key("enter")
	d.until("volume at 0", func() bool { return strings.Contains(rowOf("Volume"), " 0") })
}
