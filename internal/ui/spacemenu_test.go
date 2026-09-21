package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A menu taller than the terminal scrolls to keep the cursor's row in
// view, and j/k wrap at the ends — the family's menus all do.
func TestMenuScrollsAndWraps(t *testing.T) {
	m := newSpaceMenu()
	m.setSize(80, 20)
	items := []menuItem{{header: true, label: "item operation"}}
	for i := 0; i < 40; i++ {
		items = append(items, menuItem{label: "row " + itoa(i), key: "entry:" + itoa(i)})
	}
	m.setItems(items, "Main", 1)
	m.anim.phase = animOpen
	key := func(k string) {
		m, _, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
	if v := m.view(); !strings.Contains(v, "row 0 ") || strings.Contains(v, "row 30") {
		t.Fatalf("the window should start at the top:\n%s", v)
	}
	key("k") // off the top: the last row
	if v := m.view(); m.cursor != len(items)-1 || !strings.Contains(v, "row 39") || strings.Contains(v, "row 0 ") {
		t.Errorf("k on the first row should wrap to the last and show it: cursor %d\n%s", m.cursor, v)
	}
	key("j") // off the bottom: the first row
	if v := m.view(); m.cursor != 1 || !strings.Contains(v, "row 0 ") {
		t.Errorf("j on the last row should wrap to the first: cursor %d\n%s", m.cursor, v)
	}
	for i := 0; i < 25; i++ {
		key("j")
	}
	if v := m.view(); !strings.Contains(v, "row 25") || strings.Contains(v, "row 0 ") {
		t.Errorf("the window should follow the cursor down:\n%s", v)
	}
}
