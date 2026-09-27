package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// keysDriver is an app with no browser behind it: the core keys route the
// same whether or not a page is up.
func keysDriver(t *testing.T) *driver {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	d := newDriver(t, New(nil, ""))
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	return d
}

// quits reports whether a key, pressed now, ends the program.
func (d *driver) quits(msg tea.KeyMsg) bool {
	_, cmd := d.m.Update(msg)
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

var (
	keySpace = tea.KeyMsg{Type: tea.KeySpace}
	keyCtrlC = tea.KeyMsg{Type: tea.KeyCtrlC}
	keyQ     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
)

// Space opens and closes the Space menu, and only that (tdp K5, F6): on
// help or on a confirm it does nothing, and they stay up.
func TestSpaceClosesOnlyTheSpaceMenu(t *testing.T) {
	d := keysDriver(t)

	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.send(keySpace)
	if d.m.spaceMenu.anim.owns() {
		t.Error("Space on the Space menu should close it")
	}

	d.key("?")
	d.until("help", func() bool { return d.m.help.anim.isInteractive() })
	d.send(keySpace)
	if !d.m.help.anim.owns() || d.m.spaceMenu.anim.owns() {
		t.Error("Space on help should do nothing")
	}
	d.key("esc")

	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyQ)
	d.until("the quit confirm", func() bool { return d.m.confirm.anim.isInteractive() })
	d.send(keySpace)
	if !d.m.confirm.anim.owns() {
		t.Error("Space on a confirm should not cancel it")
	}
}

// q and Ctrl-C are one way out (tdp K1, K9): the quit flow from every
// surface but a text box, and a second Ctrl-C on its confirm leaves.
func TestQuitFromEverywhere(t *testing.T) {
	d := keysDriver(t)

	// Nothing in flight: out at once, from the panel and from over a float.
	if !d.quits(keyQ) || !d.quits(keyCtrlC) {
		t.Error("q and Ctrl-C on the panel should quit")
	}
	d.key("?")
	d.until("help", func() bool { return d.m.help.anim.isInteractive() })
	if !d.quits(keyQ) {
		t.Error("q over help should quit")
	}
	d.key("esc")
	d.m.sel.on = true
	if !d.quits(keyQ) {
		t.Error("q in visual mode should quit")
	}
	d.m.sel.on = false

	// In a text box q is a letter; Ctrl-C still leaves.
	d.key("L")
	d.until("the location box", func() bool { return d.m.input.anim.isInteractive() })
	if d.quits(keyQ) {
		t.Error("q in a text box is a character, not quit")
	}
	if !d.quits(keyCtrlC) {
		t.Error("Ctrl-C in a text box should quit")
	}
	d.key("esc")
	d.until("the box closed", func() bool { return !d.m.input.anim.owns() })

	// A download in flight: both ask first; q on the ask stays one ask,
	// Ctrl-C on it leaves.
	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyCtrlC)
	d.until("the quit confirm", func() bool { return d.m.confirm.anim.isInteractive() })
	if d.m.confirm.action != confirmQuit {
		t.Fatalf("Ctrl-C with a download in flight should ask first: %v", d.m.confirm.action)
	}
	if d.quits(keyQ) {
		t.Error("q on the quit confirm should not leave")
	}
	d.send(keyQ)
	if !d.m.confirm.anim.owns() || d.m.confirm.action != confirmQuit {
		t.Error("q on the quit confirm should leave it as it is")
	}
	if !d.quits(keyCtrlC) {
		t.Error("Ctrl-C on the quit confirm should leave at once")
	}
}
