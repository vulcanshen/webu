package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// The textarea's box: writing, then Esc out to the box, then Enter to
// set — Esc layered so a paragraph is not lost to a reflex (user,
// 2026-09-23).
func TestTheEditorHasTwoModes(t *testing.T) {
	e := newEditorPopup()
	e.setSize(100, 30)
	e.ask("text, several lines", "Notes", "one\ntwo", 7, 1)
	e.anim.phase = animOpen
	if e.mode != editorWriting || e.row != 1 || e.col != 3 {
		t.Fatalf("opens writing at the end: mode %d row %d col %d", e.mode, e.row, e.col)
	}
	key := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	e.update(tea.KeyMsg{Type: tea.KeyEnter})
	e.update(key("three"))
	if e.value() != "one\ntwo\nthree" {
		t.Errorf("Enter is a new line: %q", e.value())
	}
	e.update(tea.KeyMsg{Type: tea.KeyBackspace})
	e.update(tea.KeyMsg{Type: tea.KeyBackspace})
	if e.value() != "one\ntwo\nthr" {
		t.Errorf("Backspace: %q", e.value())
	}
	for i := 0; i < 3; i++ {
		e.update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	e.update(tea.KeyMsg{Type: tea.KeyBackspace})
	if e.value() != "one\ntwo" || e.row != 1 || e.col != 3 {
		t.Errorf("Backspace at a line's start joins it to the one above: %q row %d col %d", e.value(), e.row, e.col)
	}
	// Space while writing is a space, not the popup closing.
	if !e.typing() {
		t.Error("writing is typing")
	}
	e.update(tea.KeyMsg{Type: tea.KeySpace})
	e.update(key("x"))
	if e.value() != "one\ntwo x" {
		t.Errorf("Space: %q", e.value())
	}

	// Esc: out to the box. h/j/k/l walk, i writes again, Enter sets.
	if _, closed := e.escape(); closed || e.mode != editorMoving {
		t.Fatal("the first Esc is out of writing, not out of the box")
	}
	e.update(key("k"))
	e.update(key("0"))
	if e.row != 0 || e.col != 0 {
		t.Errorf("k and 0: row %d col %d", e.row, e.col)
	}
	if v, done := e.update(key("j")); done || v != "" {
		t.Error("moving commits nothing")
	}
	e.update(key("A"))
	e.update(key("!"))
	if e.mode != editorWriting || e.value() != "one\ntwo x!" {
		t.Errorf("A appends: mode %d %q", e.mode, e.value())
	}
	e.escape()
	if v, done := e.update(tea.KeyMsg{Type: tea.KeyEnter}); !done || v != "one\ntwo x!" {
		t.Errorf("Enter while moving sets the value: %v %q", done, v)
	}
	if _, closed := e.escape(); !closed {
		t.Error("the second Esc closes")
	}
}

// Against the page: Enter on a textarea opens the box, and what is set
// lands in the field, newlines and all.
func TestATextareaOpensTheEditor(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/nav.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	p := d.page()
	d.cursorOn(ir.Textbox, "Notes")
	if n := p.current(); n == nil || !n.Multiline {
		t.Fatalf("the textarea is multiline: %+v", n)
	}
	d.key("enter")
	d.until("the editor", func() bool { return d.m.editor.isInteractive() })
	if d.m.input.isActive() || d.m.editor.prompt != "Notes" || d.m.editor.value() != "line one" {
		t.Fatalf("the box holds the value: %q / %q", d.m.editor.prompt, d.m.editor.value())
	}
	d.key("enter")
	d.key("line two")
	d.key("esc")
	if !d.m.editor.isActive() || d.m.editor.mode != editorMoving {
		t.Fatal("Esc steps out to the box")
	}
	d.key("enter")
	d.until("set", func() bool {
		n := p.current()
		return !d.m.editor.isActive() && n != nil && strings.Contains(n.Value, "line two")
	})
	if v := p.current().Value; v != "line one\nline two" {
		t.Errorf("the field holds both lines: %q", v)
	}
}
