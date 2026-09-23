package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A listbox's options are rows of their own — a radio glyph for a list
// that takes one, a check glyph for one that takes many — and Enter
// picks (user, 2026-09-23). They used to vanish: an option was only
// ever drawn behind a combobox.
func TestAListboxIsRowsOfOptions(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/listbox.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the listbox page", d.loaded("Listbox"))
	p := d.page()
	v := dumpLayout(p.lay)
	for _, want := range []string{glyphRadioOn + " Neptunium", glyphRadioOff + " Plutonium", glyphCheckOff + " Cheese", glyphCheckOn + " Olives"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "listbox") {
		t.Errorf("the list is its options, not its role:\n%s", v)
	}
	d.cursorOn(ir.Option, "Plutonium")
	d.key("enter")
	d.until("picked", func() bool {
		n := p.current()
		return n != nil && n.Kind == ir.Option && n.Selected && strings.Contains(dumpLayout(p.lay), "Chosen: Plutonium")
	})
	if v := dumpLayout(p.lay); !strings.Contains(v, glyphRadioOff+" Neptunium") {
		t.Errorf("one choice: the old one is let go of:\n%s", v)
	}
	d.cursorOn(ir.Option, "Cheese")
	d.key("enter")
	d.until("toggled", func() bool { return strings.Contains(dumpLayout(p.lay), glyphCheckOn+" Cheese") })
	if v := dumpLayout(p.lay); !strings.Contains(v, glyphCheckOn+" Olives") {
		t.Errorf("many choices: the other stays chosen:\n%s", v)
	}
}
