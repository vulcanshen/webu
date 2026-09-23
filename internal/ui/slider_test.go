package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A slider is its bar and where it stands, and Enter asks for a number
// (user, 2026-09-23): an <input type=range> takes it as its value, an
// ARIA slider is stepped there with the arrows.
func TestASliderIsABarAndTakesANumber(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
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
	thumb := strings.Index(red, "●")

	// Enter asks for a number, the ends of the bar on the border.
	d.cursorOn(ir.Textbox, "Red")
	d.key("enter")
	d.until("the number box", func() bool { return d.m.input.isInteractive() })
	if d.m.input.title != "number 0–255" || d.m.input.value != "128" {
		t.Errorf("the box says the range and holds the value: %q %q", d.m.input.title, d.m.input.value)
	}
	for _, k := range []string{"backspace", "backspace", "backspace", "2", "0", "0"} {
		d.key(k)
	}
	d.key("enter")
	d.until("red at 200", func() bool { r := rowOf("Red"); return strings.Contains(r, "200") && strings.Index(r, "●") > thumb })

	// An ARIA slider has no value to set: it is walked there.
	d.cursorOn(ir.Textbox, "Volume")
	d.key("enter")
	d.until("the number box", func() bool { return d.m.input.isInteractive() })
	if d.m.input.title != "number 0–100" {
		t.Errorf("its range too: %q", d.m.input.title)
	}
	for _, k := range []string{"backspace", "backspace", "3", "5"} {
		d.key(k)
	}
	d.key("enter")
	d.until("volume at 35", func() bool { return strings.Contains(rowOf("Volume"), "35") })
}
