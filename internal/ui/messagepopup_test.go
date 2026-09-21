package ui

import (
	"strings"
	"testing"
)

// A message longer than the box scrolls with the page's own keys, and
// says so in its hint.
func TestMessageScrolls(t *testing.T) {
	m := newMessagePopup()
	m.setSize(80, 20)
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "line "+itoa(i))
	}
	m.show(glyphTable, "Name", lines, false, 1)
	m.anim.phase = animOpen
	if v := m.view(); !strings.Contains(v, "line 0 ") || strings.Contains(v, "line 30") || !strings.Contains(v, "scroll") {
		t.Fatalf("the window should start at the top and offer to scroll:\n%s", v)
	}
	for i := 0; i < 30; i++ {
		m.scroll("j")
	}
	if v := m.view(); !strings.Contains(v, "line 30") || strings.Contains(v, "line 0 ") {
		t.Errorf("j should scroll down:\n%s", v)
	}
	m.scroll("G")
	if v := m.view(); !strings.Contains(v, "line 49") {
		t.Errorf("G should reach the end:\n%s", v)
	}
	m.scroll("g")
	if v := m.view(); !strings.Contains(v, "line 0 ") {
		t.Errorf("g should return to the top:\n%s", v)
	}
}
