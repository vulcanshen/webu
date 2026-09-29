package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// A key as tdp M5 names it: the name on the key cap, a letter as it is
// typed, a range with an en dash, keys doing one thing split by "/".
var (
	legendKey = regexp.MustCompile(`(?:^| )(\S+):`)
	keyCap    = map[string]bool{"Esc": true, "Tab": true, "Enter": true, "Space": true, "Backspace": true,
		"Delete": true, "Home": true, "End": true, "PgUp": true, "PgDn": true, "gg": true}
	keyRange = regexp.MustCompile(`^[0-9]–[0-9]$`)
	keyMod   = regexp.MustCompile(`^(Ctrl|Alt|Shift)-\S+$`)
	boxHint  = regexp.MustCompile(`╰─*([^─╯╰]*?)─*╯`)
)

// checkKeyName fails unless k is written the way tdp M5 writes a key.
func checkKeyName(t *testing.T, where, k string) {
	t.Helper()
	if k == "/" {
		return
	}
	for _, p := range strings.Split(k, "/") {
		if len([]rune(p)) == 1 || keyCap[p] || keyRange.MatchString(p) || keyMod.MatchString(p) {
			continue
		}
		t.Errorf("%s: %q is not a key as tdp M5 writes it (in %q)", where, p, k)
	}
}

// checkLegend fails unless a hint or footer is written key:description,
// one space between items (tdp M5, D1).
func checkLegend(t *testing.T, where, s string) {
	t.Helper()
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if strings.Contains(s, " · ") || strings.Contains(s, "  ") || strings.Contains(s, "+") {
		t.Errorf("%s: items are key:description, one space apart: %q", where, s)
	}
	keys := legendKey.FindAllStringSubmatch(s, -1)
	if len(keys) == 0 {
		t.Errorf("%s: no key:description in %q", where, s)
	}
	for _, k := range keys {
		checkKeyName(t, where, k[1])
	}
}

// checkScreen checks the footer and every hint on a bottom border.
func checkScreen(t *testing.T, where string, d *driver) {
	t.Helper()
	lines := strings.Split(ansi.Strip(d.m.View()), "\n")
	checkLegend(t, where+" footer", lines[len(lines)-1])
	for _, l := range lines[:len(lines)-1] {
		for _, h := range boxHint.FindAllStringSubmatch(l, -1) {
			if strings.Contains(h[1], ":") {
				checkLegend(t, where, h[1])
			}
		}
	}
}

// Every footer and every popup's hint is written key:description (tdp
// M5, v0.1.15): key-cap names, no Bksp, no lower-case space or esc, one
// space between items.
func TestLegendsWriteKeyColonDescription(t *testing.T) {
	d := keysDriver(t)
	checkScreen(t, "web", d)
	d.key(" ")
	d.until("the Space menu", func() bool { return d.m.spaceMenu.anim.isInteractive() })
	checkScreen(t, "Space menu", d)
	d.key("esc")
	d.until("the Space menu to close", func() bool { return !d.m.popupOpen() })
	for _, s := range []string{"B", "H", "D", "S"} {
		d.key(s)
		checkScreen(t, "screen "+s, d)
		checkLegend(t, "screen "+s+" hint", hintLegend(d.m.lists.hintPairs()))
	}
	d.key("W")

	d.m.sel.on = true
	if f := strings.TrimSpace(d.m.footer()); !strings.HasPrefix(f, "?:help") {
		t.Fatalf("the footer should be visual mode's: %q", f)
	}
	checkLegend(t, "visual mode footer", d.m.footer())
	d.m.sel.typing = true
	checkLegend(t, "visual mode typing footer", d.m.footer())
	d.m.sel.on, d.m.sel.typing = false, false

	for _, tc := range []struct {
		what string
		open func() tea.Cmd
		up   func() bool
	}{
		{"toast", func() tea.Cmd { return d.m.toast.show("copied", toastInfo) }, func() bool { return d.m.toast.anim.isInteractive() }},
		{"message", func() tea.Cmd { return d.m.message.show("", "Note", []string{"one line"}, 1) }, func() bool { return d.m.message.anim.isInteractive() }},
		{"confirm", func() tea.Cmd {
			return d.m.confirm.ask(confirmPopup{title: "Delete", lines: []string{"gone"}, accept: "delete"}, 1)
		}, func() bool { return d.m.confirm.anim.isInteractive() }},
		{"editor", func() tea.Cmd { return d.m.editor.ask("Notes", "", "a\nb", 0, 1) }, func() bool { return d.m.editor.anim.isInteractive() }},
		{"picker", func() tea.Cmd { return d.m.picker.open("", "Import", t.TempDir(), 1) }, func() bool { return d.m.picker.anim.isInteractive() }},
	} {
		d.exec(tc.open())
		d.until(tc.what, tc.up)
		checkScreen(t, tc.what, d)
		if tc.what == "editor" {
			d.key("esc") // out to the box: the moving hint
			if d.m.editor.mode != editorMoving {
				t.Fatal("Esc should take the editor out to the box")
			}
			checkScreen(t, "editor moving", d)
		}
		d.exec(d.m.closeStack())
		d.exec(d.m.toast.close())
		d.until(tc.what+" to close", func() bool { return !d.m.popupOpen() && !d.m.toast.anim.owns() })
	}

	d.key("L")
	d.until("the location box", func() bool { return d.m.input.anim.isInteractive() })
	checkScreen(t, "input", d)
	d.key("esc")

	f := newFinder()
	for _, st := range []struct {
		what       string
		kind, mode int
	}{{"go", int(finderGo), 0}, {"search list", int(finderSearch), int(finderNav)}, {"search typing", int(finderSearch), int(finderInput)}} {
		f.kind, f.mode = finderKind(st.kind), finderMode(st.mode)
		_, h := f.titleAndHint()
		checkLegend(t, "finder "+st.what, ansi.Strip(h))
	}

	dt := newDevtoolsPopup()
	dt.setSize(100, 30)
	for _, tab := range []devTab{devNetwork, devStorage, devConsole, devSource} {
		dt.tab = tab
		lines := strings.Split(ansi.Strip(dt.view()), "\n")
		checkLegend(t, "devtools "+devTabLabels[tab], boxHint.FindStringSubmatch(lines[len(lines)-1])[1])
	}
}

// The key is Blue, the colon and the description Overlay0 (tdp D2).
func TestLegendColonIsOverlay0(t *testing.T) {
	withColour(t)
	const blue, overlay0 = "38;2;137;179;250m", "38;2;108;112;134m" // #89b4fa (lipgloss rounds b4 to 179), #6c7086
	for where, s := range map[string]string{
		"hint":   hintLegend([][2]string{{"j/k", "move"}}),
		"footer": keyLegend([][2]string{{"Space", "menu"}}, 40),
	} {
		if !strings.Contains(s, blue+"Space") && !strings.Contains(s, blue+"j/k") {
			t.Errorf("%s: the key should be Blue: %q", where, s)
		}
		if !strings.Contains(s, overlay0+":") {
			t.Errorf("%s: the colon should go with the description, Overlay0: %q", where, s)
		}
	}
}

// [2]\x27s bottom border saying a state is not a key: no colon, the
// description colour, not Blue (tdp D2; webu 2026-09-29).
func TestPageStatusIsNotAKey(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/", loading: true}}
	d.m.shown = 0
	out := d.m.pagePanel(80, 20)
	lines := strings.Split(out, "\n")
	bottom := lines[len(lines)-1]
	if strings.Contains(ansi.Strip(bottom), "loading:") {
		t.Errorf("a state should not be written key:description: %q", ansi.Strip(bottom))
	}
	if !strings.Contains(bottom, "38;2;108;112;134mloading") { // #6c7086
		t.Errorf("the state should be Overlay0: %q", bottom)
	}
}

// checkRefKeys fails unless every key in a key reference is written as
// tdp M5 writes a key: no " · ", no spaces, no "+", a range with "–".
func checkRefKeys(t *testing.T, where string, entries []helpEntry) {
	t.Helper()
	for _, e := range entries {
		switch e.key {
		case "", "a row\x27s key", "go": // a heading; a row, not a key; [go] a sequence like gg
			continue
		}
		if strings.ContainsAny(e.key, " +") {
			t.Errorf("%s: %q: keys are split by / (tdp M5)", where, e.key)
			continue
		}
		checkKeyName(t, where, e.key)
	}
}

// Every key reference writes its keys the way a hint does, only without
// the colon: j/k, q/Ctrl-C, 0–9 (tdp M5 v0.1.15).
func TestKeyReferenceWritesKeysByM5(t *testing.T) {
	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/", title: "Example"}}
	d.m.shown = 0
	for _, focus := range []panelID{panelTabs, panelPage} {
		d.m.focus = focus
		items, title := d.m.panelMenu()
		checkRefKeys(t, title, keyReference(items))
	}
	for _, s := range []string{"B", "H", "D", "S"} {
		d.key(s)
		items, title := d.m.panelMenu()
		checkRefKeys(t, title, keyReference(items))
	}
	for name, e := range map[string][]helpEntry{"menu": helpMenu, "options": helpOptions, "confirm": helpConfirm,
		"finder": helpFinder, "go": helpGo, "editor": helpEditor, "message": helpMessage, "devtools": helpDevtools,
		"visual mode": selectKeys} {
		checkRefKeys(t, name, e)
	}
	_, e := d.m.floatHelp()
	checkRefKeys(t, "Space menu", e)
}

// The key reference: keys Blue, descriptions Text (tdp D2 v0.1.15).
func TestKeyReferenceColours(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	d.exec(d.m.help.open("x", []helpEntry{{"j/k", "move"}}, 1))
	d.until("the key reference", func() bool { return d.m.help.anim.isInteractive() })
	v := d.m.help.view()
	if !strings.Contains(v, "38;2;137;179;250m  j/k") { // #89b4fa, rounded as lipgloss does
		t.Errorf("the key should be Blue: %q", v)
	}
	if !strings.Contains(v, "38;2;205;214;243mmove") { // #cdd6f4, f4 rounded to 243
		t.Errorf("the description should be Text: %q", v)
	}
}
