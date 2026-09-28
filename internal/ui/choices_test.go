package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/ir"
)

// A select's options and a slider's numbers are a step of their own: a
// popup over the item menu that asked for them, not the item menu's box
// with its content swapped (tdp F1 v0.1.9, F4, F7). Esc goes back to the
// item menu, as it was.
func TestChoicesStackOnTheItemMenu(t *testing.T) {
	sel := &ir.Node{Kind: ir.Combobox, Name: "Size", Children: []*ir.Node{
		{Kind: ir.Option, Name: "Small"}, {Kind: ir.Option, Name: "Large", Selected: true}}}
	slider := &ir.Node{Kind: ir.Textbox, Role: "slider", Name: "Volume", Min: 0, Max: 100, Value: "40"}

	for _, tc := range []struct {
		what string
		open func(m *AppModel) tea.Cmd
		n    int // rows the list has
	}{
		{"a select's options", func(m *AppModel) tea.Cmd { mm, cmd := m.chooseOptionsFor(sel); *m = mm.(AppModel); return cmd }, 2},
		{"a slider's numbers", func(m *AppModel) tea.Cmd { return m.slideMenu(slider) }, 101},
	} {
		d := keysDriver(t)
		menu := []menuItem{{label: "Size", key: "entry:0"}, {label: "Volume", key: "entry:1"}}
		d.m.options.setItems(menu, "Cell", 2)
		d.m.optionsKind = optItemMenu
		d.exec(d.m.options.open())
		d.until("the item menu", func() bool { return d.m.options.anim.isInteractive() })
		h := rowsOf(d.m.options.view())

		d.exec(tc.open(&d.m))
		d.until(tc.what, func() bool { return d.m.choices.anim.isInteractive() })
		if !d.m.options.anim.owns() || len(d.m.options.items) != len(menu) || d.m.options.items[0].label != "Size" {
			t.Fatalf("%s: the item menu should stay under, as it was: %+v", tc.what, d.m.options.items)
		}
		if len(d.m.choices.items) != tc.n || rowsOf(d.m.options.view()) != h {
			t.Errorf("%s: its own popup with its own rows, the item menu's box untouched: %d items", tc.what, len(d.m.choices.items))
		}
		d.key("esc")
		if d.m.choices.anim.owns() || !d.m.options.anim.owns() {
			t.Errorf("%s: Esc should go back to the item menu", tc.what)
		}
	}
}
