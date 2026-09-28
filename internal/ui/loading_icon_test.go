package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/webu/internal/page"
)

// hasSpinner reports whether s carries any frame of the loading icon.
func hasSpinner(s string) bool {
	for _, f := range spinnerFrames {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}

func titleRow(view string) string { return strings.Split(ansi.Strip(view), "\n")[0] }

// A popup whose content is still on its way turns the loading icon after
// its title, and the title does not move when it comes and goes (tdp F7,
// D3, L2): the network detail waiting for its body.
func TestDetailTurnsTheLoadingIcon(t *testing.T) {
	m := newDevDetailPopup()
	m.setSize(100, 40)
	m.show(page.NetEntry{URL: "https://example.com/a", Mime: "text/plain", Status: 200}, 2)
	m.anim.phase = animOpen
	waiting := titleRow(m.view())
	if !hasSpinner(waiting) {
		t.Fatalf("waiting for its body, the detail's title should turn the icon: %q", waiting)
	}
	m.setBody("hello", nil)
	done := titleRow(m.view())
	if hasSpinner(done) {
		t.Errorf("with the body in, the icon should be gone: %q", done)
	}
	if ansi.StringWidth(done) != ansi.StringWidth(waiting) {
		t.Errorf("the title should not move as the icon goes:\n%q\n%q", waiting, done)
	}
}

// DevTools waiting on its Storage or Source tab turns the icon after its
// name; the tab chain stays where it is.
func TestDevtoolsTurnsTheLoadingIcon(t *testing.T) {
	m := newDevtoolsPopup()
	m.setSize(100, 40)
	m.anim.phase = animOpen
	m.loading = true
	waiting := titleRow(m.body())
	m.loading = false
	done := titleRow(m.body())
	if !hasSpinner(waiting) || hasSpinner(done) {
		t.Errorf("the icon should turn while loading and only then:\n%q\n%q", waiting, done)
	}
	// Columns, not bytes: a frame of the icon is four bytes and a blank one.
	col := func(s string) int { return ansi.StringWidth(s[:strings.Index(s, "Network")]) }
	if col(done) != col(waiting) {
		t.Errorf("the tab chain should not move as the icon comes and goes:\n%q\n%q", waiting, done)
	}
}

// The icon turns on the spin tick, which runs while DevTools waits and
// stops once it has its answer.
func TestSpinTickRunsWhileDevtoolsWaits(t *testing.T) {
	d := keysDriver(t)
	d.m.devtools.anim.phase = animOpen
	d.m.devtools.loading = true
	if _, cmd := d.m.Update(spinTickMsg{}); cmd == nil {
		t.Error("the tick should keep running while DevTools waits")
	}
	d.m.devtools.loading = false
	if _, cmd := d.m.Update(spinTickMsg{}); cmd != nil {
		t.Error("the tick should stop once nothing is loading")
	}
}
