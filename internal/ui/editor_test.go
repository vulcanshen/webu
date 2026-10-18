package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/ir"
)

// Against the page: Enter on a textarea opens the lib's textarea with the
// field's value, writing at its end; Esc steps out to move, Enter there
// sets the value, and what is set lands in the field, newlines and all.
func TestATextareaOpensTheEditor(t *testing.T) {
	b := launch(t)
	d := newDriver(t, New(b, fixture("nav.html")))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	d.key("2")
	p := d.page()
	d.cursorOn(ir.Textbox, "Notes")
	if n := p.current(); n == nil || !n.Multiline {
		t.Fatalf("the textarea is multiline: %+v", n)
	}
	d.press("enter")
	d.until("the editor", d.shows("TEXT, SEVERAL LINES: Notes"))
	if !strings.Contains(d.view(), "line one") {
		t.Fatalf("the box holds the value:\n%s", d.view())
	}
	d.key("enter")
	d.key("line two")
	d.key("esc")
	d.key("enter")
	d.until("set", func() bool {
		n := p.current()
		return n != nil && strings.Contains(n.Value, "line two")
	})
	if v := p.current().Value; v != "line one\nline two" {
		t.Errorf("the field holds both lines: %q", v)
	}
}
