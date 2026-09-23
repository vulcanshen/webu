package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A tablist is one strip, the way the pagetab is: the tabs as segments,
// the chosen one lit, and under it the panel it opens. Enter on a tab
// chooses it (user, 2026-09-23).
func TestATablistIsAStrip(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/tabs.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the tabs page", d.loaded("Tabs"))
	p := d.page()
	v := dumpLayout(p.lay)
	if strings.Contains(v, "tablist") || strings.Contains(v, "tab ") {
		t.Errorf("the strip is its tabs, not its role:\n%s", v)
	}
	// One row holds all three, as segments of a chain.
	strip := -1
	for i, r := range p.lay.rows {
		s := r.plain()
		if strings.Contains(s, "Maria") && strings.Contains(s, "Carl") && strings.Contains(s, "Ida") {
			strip = i
		}
	}
	if strip < 0 {
		t.Fatalf("the three tabs are one strip:\n%s", v)
	}
	on, off := 0, 0
	for _, s := range p.lay.rows[strip].segs {
		switch s.kind {
		case segTabOn:
			on++
		case segTabOff:
			off++
		}
	}
	if on != 1 || off != 2 {
		t.Errorf("one tab lit, two not: %d lit, %d not", on, off)
	}
	if !strings.Contains(v, "Maria was a composer") || strings.Contains(v, "Carl was") {
		t.Errorf("only the chosen tab's panel shows:\n%s", v)
	}
	d.cursorOn(ir.Button, "Carl")
	d.key("enter")
	d.until("switched", func() bool {
		v := dumpLayout(p.lay)
		return strings.Contains(v, "Carl was a composer") && !strings.Contains(v, "Maria was")
	})
	n := p.current()
	if n == nil || n.Name != "Carl" || !n.Selected {
		t.Errorf("the cursor stays on the tab, now chosen: %+v", n)
	}
}
