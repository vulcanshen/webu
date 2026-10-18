package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A progress or a meter is a bar filled to its value, and no stop; a
// <details> summary is a button with a triangle that opens what follows;
// a date or colour box takes its value whole, in the browser's shape,
// and refuses one that is not (user, 2026-09-23).
func TestGaugesDisclosuresAndShapedBoxes(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/gauges.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	d.until("the gauges page", d.loaded("Gauges"))
	p := d.page()
	v := dumpLayout(p.lay)

	rowWith := func(parts ...string) bool {
		for _, r := range p.lay.rows {
			s := r.plain()
			all := true
			for _, part := range parts {
				all = all && strings.Contains(s, part)
			}
			if all {
				return true
			}
		}
		return false
	}
	// Half of one is a percent; a meter reads against its top; a
	// progress with no value yet is an empty bar and an ellipsis.
	if !rowWith("Upload", "━━━━━━──────", "50%", "done.") {
		t.Errorf("a progress is a bar filled to its value:\n%s", v)
	}
	if !rowWith("Waiting", "────────────", "…") {
		t.Errorf("an indeterminate progress is empty:\n%s", v)
	}
	if !rowWith("Disk", "━━━━━━━━────", "65/100", "used.") {
		t.Errorf("a meter reads against its top:\n%s", v)
	}
	if !rowWith("Build", "━━━━━───────", "42/100") {
		t.Errorf("an ARIA progressbar too:\n%s", v)
	}
	if strings.Contains(v, "gauge ") {
		t.Errorf("a gauge reads only, so it is no stop:\n%s", v)
	}

	// <details>: shut, its content is not on the page; Enter opens it.
	if strings.Contains(v, "The hidden part.") || !rowWith("▸ Show more") || !rowWith("▾ Already open") {
		t.Errorf("a summary wears its triangle, shut or open:\n%s", v)
	}
	d.cursorOn(ir.Button, "Show more")
	d.key("enter")
	d.until("opened", func() bool {
		return rowWith("▾ Show more") && strings.Contains(dumpLayout(p.lay), "The hidden part.")
	})

	// A date box: the shape on the border, the value in the box.
	d.cursorOn(ir.Textbox, "Day")
	d.press("enter")
	d.until("the date box", d.shows("EDIT: Day"))
	if v := d.view(); !strings.Contains(v, "YYYY-MM-DD") || !strings.Contains(v, "2026-09-23") {
		t.Errorf("the box says the shape and offers the value:\n%s", v)
	}
	for i := 0; i < 10; i++ {
		d.key("backspace")
	}
	for _, c := range "2026-10-01" {
		d.key(string(c))
	}
	d.key("enter")
	d.until("the day set", func() bool { return rowWith("Day", "2026-10-01") })

	// A colour box refuses a value not in its shape, and keeps the box.
	d.settle()
	d.cursorOn(ir.Textbox, "Tint")
	d.press("enter")
	d.until("the colour box", d.shows("EDIT: Tint"))
	if v := d.view(); !strings.Contains(v, "#rrggbb") || !strings.Contains(v, "#336699") {
		t.Errorf("the colour box:\n%s", v)
	}
	for i := 0; i < 7; i++ {
		d.key("backspace")
	}
	for _, c := range "red" {
		d.key(string(c))
	}
	d.key("enter")
	if !strings.Contains(d.view(), "wants #rrggbb") {
		t.Errorf("a value not in the shape is refused and the box stays:\n%s", d.view())
	}
	for i := 0; i < 3; i++ {
		d.key("backspace")
	}
	for _, c := range "#ff0000" {
		d.key(string(c))
	}
	d.key("enter")
	d.until("the tint set", func() bool { return rowWith("Tint", "#ff0000") })
}
