package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
	d.until("the key reference", func() bool { return d.m.help.anim.isInteractive() })
	d.send(keySpace)
	if !d.m.help.anim.owns() || d.m.spaceMenu.anim.owns() {
		t.Error("Space on the key reference should do nothing")
	}
	d.key("esc")

	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyQ)
	d.until("the quit confirm", func() bool { return d.m.quitAsk.anim.isInteractive() })
	d.send(keySpace)
	if !d.m.quitAsk.anim.owns() {
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
	d.until("the key reference", func() bool { return d.m.help.anim.isInteractive() })
	if !d.quits(keyQ) {
		t.Error("q over the key reference should quit")
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
	d.until("the quit confirm", func() bool { return d.m.quitAsk.anim.isInteractive() })
	if d.m.quitAsk.action != confirmQuit {
		t.Fatalf("Ctrl-C with a download in flight should ask first: %v", d.m.quitAsk.action)
	}
	if d.quits(keyQ) {
		t.Error("q on the quit confirm should not leave")
	}
	d.send(keyQ)
	// As it is: not asked again, which would replay its opening.
	if !d.m.quitAsk.anim.isInteractive() || d.m.quitAsk.action != confirmQuit {
		t.Error("q on the quit confirm should leave it as it is")
	}
	if !d.quits(keyCtrlC) {
		t.Error("Ctrl-C on the quit confirm should leave at once")
	}
}

// ? on a panel reads (tdp K6, M4): the panel's keys, taken from its Space
// menu, then the core keys — nothing in it runs. A second ? closes it.
func TestQuestionMarkOnAPanelReads(t *testing.T) {
	d := keysDriver(t)

	d.key("B")
	d.key("?")
	d.until("the key reference", func() bool { return d.m.help.anim.isInteractive() })
	if d.m.globalMenu.anim.owns() {
		t.Fatal("? on a panel should not open anything that runs")
	}
	e := d.m.help.entries
	if !helpHas(e, "A") || !helpHas(e, "/") || !helpHas(e, "q · Ctrl+C") {
		t.Errorf("the key reference should list the panel's keys and the core keys: %+v", e)
	}
	for _, x := range e {
		if x.key == "/" && strings.HasPrefix(x.desc, "[") {
			t.Errorf("a key written into a label is listed once, as the key: %q", x.desc)
		}
	}

	d.key("A") // read, not run: the Add folder box does not open
	d.key("enter")
	if d.m.input.anim.owns() || !d.m.help.anim.owns() {
		t.Error("keys on the key reference should run nothing")
	}
	if v := d.m.View(); !strings.Contains(v, "core keys") || !strings.Contains(v, "Bookmarks · keys") {
		t.Errorf("the key reference should be drawn with its headings:\n%s", v)
	}
	d.key("?")
	if d.m.help.anim.owns() {
		t.Error("a second ? should close the key reference")
	}
}

// The panel's key reference follows its Space menu: [1] with a tab lists
// what Enter does there, and the general Enter line is not repeated.
func TestKeyReferenceFromTheMenu(t *testing.T) {
	items := []menuItem{
		{header: true, label: "item operation"},
		{label: "[Enter] Switch to", key: "enter", hint: "show this tab in [2]"},
		{label: "Close", key: "c", hint: "this tab"},
		{label: "Yank markdown", key: "yankmd", hint: "menu-only"},
		{separator: true},
		{header: true, label: "panel operation"},
		{label: "[go] Go to line", key: "go", hint: "by its number"},
		{separator: true},
		{header: true, label: "menu-only region"},
		{label: "Yank markdown", key: "yankmd"},
	}
	e := keyReference(items)
	for _, x := range e {
		if x.desc == "menu-only region" {
			t.Errorf("a heading with no key under it should go: %+v", e)
		}
	}
	enters := 0
	for _, x := range e {
		if x.key == "Enter" {
			enters++
		}
	}
	if enters != 1 || !helpHas(e, "c") || !helpHas(e, "go") || helpHas(e, "yankmd") {
		t.Errorf("the reference should hold Enter once, c, go and no menu-only row: %+v", e)
	}
}

// Every Space menu ends in the global region, always one row (tdp M2). A
// menu that was a single flat region is labelled, since it is one of two
// now. The row opens the global operation popup over the menu (tdp M4,
// F4): Esc goes back to the menu, a row run closes both (tdp T1).
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
		if ti := d.m.spaceMenu.title; screen == "W" && ti != "[1] Tabs" && ti != "[2] Page" {
			t.Errorf("the Space menu is titled with its panel's label (tdp D4): %q", d.m.spaceMenu.title)
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
	d.until("the global operation popup", func() bool { return d.m.globalMenu.anim.isInteractive() })
	if !d.m.spaceMenu.anim.owns() {
		t.Fatal("the global operation popup should stack on the Space menu")
	}
	if d.m.globalMenu.items[0].header || !menuHas(d.m.globalMenu.items, "Quit") {
		t.Errorf("the popup should list the global operations, runnable from the first row: %+v", d.m.globalMenu.items[0])
	}
	for _, it := range d.m.globalMenu.items {
		if _, screen := screenKeys[it.key]; !screen && it.key != "q" {
			t.Errorf("only the screens and quitting act on the app; %q is a panel operation", it.label)
		}
	}

	d.key("esc")
	if d.m.globalMenu.anim.owns() || !d.m.spaceMenu.anim.owns() {
		t.Fatal("Esc should go back to the Space menu")
	}

	d.key("G")
	d.key("enter")
	d.until("the global operation popup again", func() bool { return d.m.globalMenu.anim.isInteractive() })
	d.key("B")
	if d.m.screen != screenBookmarks || d.m.globalMenu.anim.owns() || d.m.spaceMenu.anim.owns() {
		t.Errorf("a row run should close the whole stack: screen %v", d.m.screen)
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
	d.until("the quit confirm", func() bool { return d.m.quitAsk.anim.isInteractive() })
	d.key("?")
	d.until("the confirm's help", func() bool { return d.m.quitHelp.anim.isInteractive() })
	if len(d.m.quitHelp.entries) != len(helpConfirm) {
		t.Errorf("? on a confirm should show its two keys: %+v", d.m.quitHelp.entries)
	}
	d.key("enter") // pressed on the help, not on the confirm under it
	if !d.m.quitAsk.anim.owns() {
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
// (tdp D3). The global operation popup's rows do the same.
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

	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.key("G")
	d.key("enter")
	d.until("the global operation popup", func() bool { return d.m.globalMenu.anim.isInteractive() })
	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyQ) // the quit row; with a download in flight it asks first
	d.until("the quit question", func() bool { return d.m.quitAsk.anim.isInteractive() })
	if !d.m.globalMenu.anim.owns() || !d.m.spaceMenu.anim.owns() {
		t.Fatal("the global operation popup should stay under the box it opened")
	}
	d.key("esc")
	if !d.m.globalMenu.anim.owns() {
		t.Error("Esc on the box should come back to the global operation popup")
	}
}

// The quit flow asks in a popup of its own, over the whole stack (tdp D3,
// K4, F4): a question being answered stays under it, and Esc on the quit
// question comes back to that one, not past it.
func TestQuitAskKeepsTheQuestionUnder(t *testing.T) {
	d := keysDriver(t)

	d.key("H")
	d.key("C")
	d.until("the clear-history question", func() bool { return d.m.confirm.anim.isInteractive() })
	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyQ)
	d.until("the quit question", func() bool { return d.m.quitAsk.anim.isInteractive() })
	if !d.m.confirm.anim.owns() || d.m.confirm.action != confirmClearHistory {
		t.Fatalf("the quit question should not replace the one under it: %v", d.m.confirm.action)
	}
	if v := d.m.View(); !strings.Contains(v, "Quitting stops it") {
		t.Errorf("the quit question should be drawn on top:\n%s", v)
	}
	d.key("esc")
	if d.m.quitAsk.anim.owns() || !d.m.confirm.anim.owns() || d.m.confirm.action != confirmClearHistory {
		t.Error("Esc on the quit question should come back to the question under it")
	}
}

// The quit question has a help of its own (tdp D3): help can be under it
// — q pressed on help — and its own ? opens over it. Esc unwinds them one
// at a time, back to the help that was there first.
func TestQuitHelpOverTheQuitAsk(t *testing.T) {
	d := keysDriver(t)

	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.key("?")
	d.until("the menu's help", func() bool { return d.m.help.anim.isInteractive() })
	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.send(keyQ)
	d.until("the quit question over help", func() bool { return d.m.quitAsk.anim.isInteractive() })
	d.key("?")
	d.until("the quit question's help", func() bool { return d.m.quitHelp.anim.isInteractive() })
	if len(d.m.quitHelp.entries) != len(helpConfirm) || !d.m.help.anim.owns() {
		t.Errorf("? on the quit question should open its own help, over the other: %+v", d.m.quitHelp.entries)
	}
	d.key("esc")
	if d.m.quitHelp.anim.owns() || !d.m.quitAsk.anim.owns() {
		t.Fatal("the first Esc should close the quit question's help")
	}
	d.key("esc")
	if d.m.quitAsk.anim.owns() || !d.m.help.anim.owns() {
		t.Error("the second Esc should close the quit question and leave the help under it")
	}
}

// In the global operation popup the screen already up is dimmed, not left
// out, and does nothing (tdp M4, M6).
func TestGlobalPopupDimsTheScreenYouAreOn(t *testing.T) {
	d := keysDriver(t)

	d.key("H")
	d.send(keySpace)
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	d.key("G")
	d.key("enter")
	d.until("the global operation popup", func() bool { return d.m.globalMenu.anim.isInteractive() })
	for _, it := range d.m.globalMenu.items {
		if it.disabled != (it.key == "H") {
			t.Errorf("only History should be dimmed on History: %+v", it)
		}
	}
	d.key("H")
	if !d.m.globalMenu.anim.owns() || d.m.screen != screenHistory {
		t.Error("the dimmed row should do nothing")
	}
}

// In visual mode ? is the mode's help (tdp K11): its keys, read-only, from
// the same list the Space cheatsheet shows.
func TestQuestionMarkInVisualMode(t *testing.T) {
	d := keysDriver(t)
	d.m.sel.on = true

	d.key("?")
	d.until("the mode's help", func() bool { return d.m.help.anim.isInteractive() })
	if d.m.help.title != "Visual mode" || len(d.m.help.entries) != len(selectKeys) {
		t.Errorf("? in visual mode should show the mode's keys: %q %+v", d.m.help.title, d.m.help.entries)
	}
	d.key("y") // read, not run
	if !d.m.help.anim.owns() || !d.m.sel.on {
		t.Error("keys on the mode's help should run nothing")
	}
	for i, e := range selectKeys {
		if !strings.Contains(selectCheatsheet[i], e.key) || !strings.Contains(selectCheatsheet[i], e.desc) {
			t.Errorf("the cheatsheet and the help should read the same list: %q", selectCheatsheet[i])
		}
	}
}

// The bookmark box is as wide as its legend at its widest, so the bottom
// border is never cut short, and Tab from a field with an offer to one
// without does not change its width (tdp L2).
func TestInputGroupLegendFits(t *testing.T) {
	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/examples/dialog/", title: "Modal Dialog Example | APG | WAI | W3C"}}
	d.m.shown = 0

	d.key("B")
	d.key("a")
	d.until("the bookmark box", func() bool { return d.m.input.anim.isInteractive() })
	box := func() (string, int) {
		lines := strings.Split(ansi.Strip(d.m.input.view()), "\n")
		return lines[len(lines)-1], ansi.StringWidth(lines[0])
	}
	bottom, w := box()
	if !strings.Contains(bottom, "Esc cancel") || !strings.Contains(bottom, "→ accept") || !strings.Contains(bottom, "Bksp decline") {
		t.Errorf("the whole legend should fit on the bottom border, → accepting in a group: %q", bottom)
	}
	d.send(tea.KeyMsg{Type: tea.KeyTab})
	// In a group Tab only moves: the URL on offer stays an offer (tdp K2).
	if d.m.input.at != 1 || d.m.input.value != "" || d.m.input.placeholder == "" {
		t.Fatalf("Tab in a group should move to the next field and accept nothing: at %d value %q", d.m.input.at, d.m.input.value)
	}
	d.key("x") // the title field typed: its offer is gone
	if _, w2 := box(); w2 != w {
		t.Errorf("the box should keep its width as the legend changes: %d then %d", w, w2)
	}
}
