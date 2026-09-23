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
// ten to a window, the cursor on the current one; j/k/u/d walk them and
// Enter moves the slider there (user, 2026-09-23): an <input type=range>
// takes it as its value, an ARIA slider is stepped there with the arrows.
func TestASliderIsABarAndListsItsNumbers(t *testing.T) {
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

	// Enter lists the numbers: ten a window, the current one under the
	// cursor and mid-window, the range in the title.
	d.cursorOn(ir.Textbox, "Red")
	d.key("enter")
	d.until("the number list", func() bool { return d.m.options.isInteractive() })
	o := d.m.options
	if o.title != "Red · 0–255" || len(o.items) != 256 || o.visible() != 10 {
		t.Errorf("every number of the bar, ten at a time, under the range: %q %d %d", o.title, len(o.items), o.visible())
	}
	if it := o.items[o.cursor]; it.label != "128" || it.hint != "current" {
		t.Errorf("the cursor is on where it stands: %+v", it)
	}
	if o.top != o.cursor-5 {
		t.Errorf("mid-window: top %d for cursor %d", o.top, o.cursor)
	}
	// d is half a window, j one row; Enter is the number under the cursor.
	d.key("d")
	d.key("j")
	if it := d.m.options.items[d.m.options.cursor]; it.label != "134" {
		t.Errorf("d then j is six rows down: %q", it.label)
	}
	d.key("enter")
	d.until("red at 134", func() bool { return strings.Contains(rowOf("Red"), "134") })

	// An ARIA slider has no value to set: it is walked there.
	d.cursorOn(ir.Textbox, "Volume")
	d.key("enter")
	d.until("the number list", func() bool { return d.m.options.isInteractive() })
	if d.m.options.title != "Volume · 0–100" {
		t.Errorf("its range too: %q", d.m.options.title)
	}
	d.key("u")
	d.key("enter")
	d.until("volume at 35", func() bool { return strings.Contains(rowOf("Volume"), "35") })
}
