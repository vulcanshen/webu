package ui

import (
	"strings"
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
	d.until("the ? menu", func() bool { return d.m.globalMenu.anim.isInteractive() })
	d.send(keySpace)
	if !d.m.globalMenu.anim.owns() || d.m.spaceMenu.anim.owns() {
		t.Error("Space on the ? menu should do nothing")
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
	d.until("the ? menu", func() bool { return d.m.globalMenu.anim.isInteractive() })
	if !d.quits(keyQ) {
		t.Error("q over the ? menu should quit")
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

// ? on a panel is the ? menu (tdp M4): the global operations, run from
// it like a Space menu's rows, and under them the core keys to read. A
// second ? closes it.
func TestQuestionMarkMenu(t *testing.T) {
	d := keysDriver(t)

	d.key("?")
	d.until("the ? menu", func() bool { return d.m.globalMenu.anim.isInteractive() })
	items := d.m.globalMenu.items
	if !items[0].header || items[0].label != "global operation" {
		t.Errorf("the ? menu should open on its global operations: %+v", items[0])
	}
	ref := -1
	for i, it := range items {
		if it.header && it.label == "key reference" {
			ref = i
		}
	}
	if ref < 0 || !items[len(items)-1].note {
		t.Fatal("the ? menu should end in a key reference")
	}
	d.key("G")
	if c := d.m.globalMenu.cursor; c >= ref {
		t.Errorf("the key reference is read, not run: G landed on row %d (%q)", c, items[c].label)
	}
	if v := d.m.View(); !strings.Contains(v, "key reference") || !strings.Contains(v, "[B]ookmarks") {
		t.Errorf("the ? menu should show both regions:\n%s", v)
	}

	d.key("B") // a row's key runs it
	if d.m.globalMenu.anim.owns() || d.m.screen != screenBookmarks {
		t.Errorf("B in the ? menu should close it and open Bookmarks: screen %v", d.m.screen)
	}
	d.key("?")
	d.until("the ? menu on a screen", func() bool { return d.m.globalMenu.anim.isInteractive() })
	d.key("enter") // the first row: Web
	if d.m.screen != screenWeb {
		t.Errorf("Enter on Web should go back to the web: screen %v", d.m.screen)
	}

	d.key("?")
	d.until("the ? menu", func() bool { return d.m.globalMenu.anim.isInteractive() })
	d.key("?")
	if d.m.globalMenu.anim.owns() || d.m.help.anim.owns() {
		t.Error("a second ? should close the ? menu")
	}
}

// Every Space menu ends in the global region: one row into the ? menu
// (tdp M2; the one row is webu's deviation, dev-remarks.md). A menu that
// was a single flat region is labelled, since it is one of two now.
func TestSpaceMenuEndsInGlobal(t *testing.T) {
	d := keysDriver(t)

	for _, screen := range []string{"W", "S", "H"} {
		d.key(screen)
		d.send(keySpace)
		d.until("the Space menu on "+screen, func() bool { return d.m.spaceMenu.anim.isInteractive() })
		items := d.m.spaceMenu.items
		n := len(items)
		if !items[0].header {
			t.Errorf("%s: the first region should be labelled: %+v", screen, items[0])
		}
		if n < 2 || items[n-2].label != "global operation" || items[n-1].key != "globalmenu" {
			t.Fatalf("%s: the menu should end in the global region: %+v", screen, items[max(0, n-2):])
		}
		d.key("esc")
	}

	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.key("G")
	d.key("enter")
	d.until("the ? menu", func() bool { return d.m.globalMenu.anim.isInteractive() })
	if d.m.spaceMenu.anim.owns() {
		t.Error("the global row should open the ? menu in the Space menu's place")
	}
}

// ? on a popup is that popup's keys, and nothing else (tdp K6); Esc takes
// the help away and leaves the popup as it was.
func TestQuestionMarkOnAPopup(t *testing.T) {
	d := keysDriver(t)

	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.key("?")
	d.until("the menu's help", func() bool { return d.m.help.anim.isInteractive() })
	if d.m.globalMenu.anim.owns() || !helpHas(d.m.help.entries, "Space · Esc") || helpHas(d.m.help.entries, "W · B · H · D · S") {
		t.Errorf("? on the Space menu should show the menu's keys: %+v", d.m.help.entries)
	}
	if v := d.m.View(); !strings.Contains(v, "a row's key") {
		t.Errorf("the help should be drawn over the menu:\n%s", v)
	}
	d.key("esc")
	if d.m.help.anim.owns() || !d.m.spaceMenu.anim.owns() {
		t.Error("Esc should close the help and leave the menu")
	}
	d.key("esc")

	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyQ)
	d.until("the quit confirm", func() bool { return d.m.confirm.anim.isInteractive() })
	d.key("?")
	d.until("the confirm's help", func() bool { return d.m.help.anim.isInteractive() })
	if len(d.m.help.entries) != len(helpConfirm) {
		t.Errorf("? on a confirm should show its two keys: %+v", d.m.help.entries)
	}
	d.key("enter") // pressed on the help, not on the confirm under it
	if !d.m.confirm.anim.owns() {
		t.Error("keys on the help should not reach the confirm")
	}
}

func helpHas(entries []helpEntry, key string) bool {
	for _, e := range entries {
		if e.key == key {
			return true
		}
	}
	return false
}

func menuHas(items []menuItem, label string) bool {
	for _, it := range items {
		if it.label == label {
			return true
		}
	}
	return false
}

// An empty list has no item, so its Space menu has no item region — not
// even the heading (tdp M2). Add on the empty Bookmarks is the list's.
func TestEmptyListHasNoItemRegion(t *testing.T) {
	d := keysDriver(t)

	for _, screen := range []string{"H", "D", "B"} {
		d.key(screen)
		d.send(keySpace)
		d.until("the Space menu on "+screen, func() bool { return d.m.spaceMenu.anim.isInteractive() })
		items := d.m.spaceMenu.items
		if menuHas(items, "item operation") || items[0].label != "panel operation" {
			t.Errorf("%s: an empty list should have no item region: %+v", screen, items)
		}
		if screen == "B" && !menuHas(items, "Add") {
			t.Error("B: Add belongs to the empty list's panel region")
		}
		d.key("esc")
	}
}

// A row that cannot run stays, dimmed, in its own words, and neither Enter
// nor its key does anything (tdp M6).
func TestDisabledRowDoesNothing(t *testing.T) {
	d := keysDriver(t)

	d.key("2") // the page panel, no page on it
	d.send(keySpace)
	d.until("the page's Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	for _, it := range d.m.spaceMenu.items {
		if it.key == "pagetab" && it.hint != "header, body, others, footer" {
			t.Errorf("a disabled row keeps its own hint: %q", it.hint)
		}
	}
	cur := d.m.spaceMenu.items[d.m.spaceMenu.cursor]
	if cur.label != "Reload" || !cur.disabled {
		t.Fatalf("the first row should be a disabled Reload: %+v", cur)
	}
	d.key("enter")
	d.key("R")
	if !d.m.spaceMenu.anim.owns() || d.m.toast.anim.owns() {
		t.Error("Enter or the key on a disabled row should do nothing: no close, no toast")
	}
}

// Visual mode's footer leads with Space and ? like every other surface
// that is not being typed into (tdp M1).
func TestVisualModeFooterShowsSpaceAndHelp(t *testing.T) {
	d := keysDriver(t)
	d.m.sel.on = true
	if f := d.m.footer(); !strings.Contains(f, "space") || !strings.Contains(f, "? help") {
		t.Errorf("visual mode's footer should show space and ?: %q", f)
	}
}

// Tab, 1 and 2 do not switch panels while visual mode is on (tdp K2, K4):
// a note says to leave the mode first.
func TestVisualModeHoldsThePanel(t *testing.T) {
	d := keysDriver(t)
	d.key("2")
	d.m.sel.on = true
	for _, k := range []tea.KeyMsg{{Type: tea.KeyTab}, {Type: tea.KeyRunes, Runes: []rune("1")}} {
		d.send(k)
		if d.m.focus != panelPage || !d.m.toast.anim.owns() {
			t.Errorf("%q in visual mode should stay on the panel and say why: focus %v", k.String(), d.m.focus)
		}
	}
}

// A float opened from a menu leaves the menu under it (tdp F4, T1): Esc
// comes back to the menu, and finishing the errand closes the whole stack
// (tdp D3). The ? menu's rows do the same.
func TestMenuStaysUnderWhatItOpened(t *testing.T) {
	d := keysDriver(t)

	d.key("B")
	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.key("A")
	d.until("the folder box", func() bool { return d.m.input.anim.isInteractive() })
	if !d.m.spaceMenu.anim.owns() {
		t.Fatal("the menu should stay under the box it opened")
	}
	d.key("esc")
	if d.m.input.anim.owns() || !d.m.spaceMenu.anim.owns() {
		t.Fatal("Esc on the box should come back to the menu")
	}

	d.key("A")
	d.until("the folder box again", func() bool { return d.m.input.anim.isInteractive() })
	for _, r := range "news" {
		d.key(string(r))
	}
	d.key("enter")
	if d.m.input.anim.owns() || d.m.spaceMenu.anim.owns() {
		t.Error("finishing the errand should close the box and the menu under it")
	}

	d.key("?")
	d.until("the ? menu", func() bool { return d.m.globalMenu.anim.isInteractive() })
	d.key("L")
	d.until("the location box", func() bool { return d.m.input.anim.isInteractive() })
	if !d.m.globalMenu.anim.owns() {
		t.Fatal("the ? menu should stay under the box it opened")
	}
	d.key("esc")
	if !d.m.globalMenu.anim.owns() {
		t.Error("Esc on the box should come back to the ? menu")
	}
}
