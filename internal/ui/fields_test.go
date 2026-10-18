package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A field the page marked wrong wears the colour of "is wrong" on its
// value, and the box over it says so; a fieldset's legend is a group
// heading drawn once; a code block is a stop, read in full on Enter
// (user, 2026-09-23).
func TestInvalidLegendAndCode(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/form2.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	d.until("the form", d.loaded("Form two"))
	p := d.page()

	// The invalid value, in its own kind; the valid one not.
	kinds := map[string]segKind{}
	for _, r := range p.lay.rows {
		for _, s := range r.segs {
			if w := strings.TrimSpace(s.text); w == "not-an-email" || w == "City" {
				kinds[w] = s.kind
			}
		}
	}
	if kinds["not-an-email"] != segInvalid {
		t.Errorf("an invalid value is drawn as one: %v", kinds)
	}
	d.cursorOn(ir.Textbox, "Email")
	d.press("enter")
	d.until("the box", d.shows("EDIT: Email"))
	if !strings.Contains(d.view(), "the page says the value is not right") {
		t.Errorf("the box says the value is wrong:\n%s", d.view())
	}
	d.press("esc")

	// Each legend once, bold, and not again as a label row.
	v := dumpLayout(p.lay)
	for _, legend := range []string{"Account", "Address"} {
		if n := strings.Count(v, legend); n != 1 {
			t.Errorf("%q should be drawn once, is %d times:\n%s", legend, n, v)
		}
	}
	bold := 0
	for _, r := range p.lay.rows {
		for _, s := range r.segs {
			if (s.text == "Account" || s.text == "Address") && s.attr&attrBold != 0 {
				bold++
			}
		}
	}
	if bold != 2 {
		t.Errorf("legends are group headings, in bold: %d", bold)
	}

	// The code block: a stop, Enter reads it whole with its long line
	// unfolded.
	d.cursorOn(ir.Code, "package main")
	d.press("enter")
	d.until("the code", d.shows("CODE"))
	if v := d.view(); !strings.Contains(v, `fmt.Println("a line of code`) || !strings.Contains(v, "and then some") {
		t.Errorf("the block in full, its long line folded under itself:\n%s", v)
	}
}
