package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	d.exec(d.m.help.open("x", []helpEntry{{key: "j/k", desc: "move"}}, 1))
	d.until("the key reference", func() bool { return d.m.help.anim.isInteractive() })
	v := d.m.help.view()
	if !strings.Contains(v, "38;2;137;179;250m  j/k") { // #89b4fa, rounded as lipgloss does
		t.Errorf("the key should be Blue: %q", v)
	}
	if !strings.Contains(v, "38;2;205;214;243mmove") { // #cdd6f4, f4 rounded to 243
		t.Errorf("the description should be Text: %q", v)
	}
}

// sentence is a wrapped, centred body read back as one line of words.
func sentence(lines []string) string {
	return strings.Join(strings.Fields(ansi.Strip(strings.Join(lines, " "))), " ")
}

// A key named in a sentence — an empty state, a toast, a message, a
// prompt, a menu description — is in square brackets (tdp M5 v0.1.15),
// and an empty state still lights it as a key.
func TestKeysInSentencesAreBracketed(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	lit := lipgloss.NewStyle().Foreground(handColor)
	for where, tc := range map[string]struct {
		lines []string
		want  string
		keys  []string
	}{
		"[1] empty": {d.m.tabsBody(22, 10), "Press [T] to open one", []string{"[T]"}},
		"[2] empty": {d.m.pageBody(70, 10), "Press [L] to enter a location, or [T] for a new tab", []string{"[L]", "[T]"}},
	} {
		if got := sentence(tc.lines); !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q, want %q", where, got, tc.want)
		}
		for _, k := range tc.keys {
			if !strings.Contains(strings.Join(tc.lines, ""), lit.Render(k)) {
				t.Errorf("%s: %s should be lit as a key", where, k)
			}
		}
	}
	for kind, want := range map[listKind]string{
		listHistory:   "Pages you visit are listed here; [W] is the web",
		listDownloads: "Files the page saves are listed here; [W] is the web",
		listBookmarks: "Press [a] to add one, [A] for a folder, or [W] for the web",
	} {
		_, words := listPanel{kind: kind}.emptyState()
		var got []string
		for _, w := range words {
			got = append(got, w.text)
			if strings.HasPrefix(w.text, "[") != w.key {
				t.Errorf("%q: a bracketed word is a key, and only that", w.text)
			}
		}
		if strings.Join(got, " ") != want {
			t.Errorf("empty list: %q, want %q", strings.Join(got, " "), want)
		}
	}

	// Visual mode answers Tab with a toast naming the way out.
	d.m.sel.on = true
	d.send(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.Contains(d.m.toast.msg, "[Esc]") {
		t.Errorf("the toast should bracket the key: %q", d.m.toast.msg)
	}
	d.m.sel.on = false

	for where, s := range map[string]string{
		"Quit":          globalMenuItems(screenWeb)[5].hint,
		"visual mode /": selectKeys[7].desc,
	} {
		if !strings.Contains(s, "[") {
			t.Errorf("%s: a key in a description is a sentence\x27s, bracketed: %q", where, s)
		}
	}
}

// ? lists a key that is there but cannot be pressed now, dimmed as its
// Space menu row is; its words stay as they are (tdp M6 v0.1.14).
func TestKeyReferenceDimsWhatCannotRun(t *testing.T) {
	withColour(t)
	d := keysDriver(t)
	d.m.tabs = []*tab{{id: 1, url: "https://example.com/", title: "Example"}}
	d.m.shown = 0
	d.m.focus = panelTabs
	items, _ := d.m.panelMenu()
	e := keyReference(items)
	off := map[string]bool{}
	for _, x := range e {
		off[x.key] = x.disabled
	}
	if !off["X"] || !off["U"] || off["T"] {
		t.Errorf("[1] with one tab: X and U cannot run, T can: %+v", e)
	}
	for _, x := range e {
		if x.key == "X" && strings.HasPrefix(x.desc, "X ") {
			t.Errorf("the key should not be printed twice: %q", x.desc)
		}
	}
	d.exec(d.m.help.open("[1] Tabs", e, 1))
	d.until("the key reference", func() bool { return d.m.help.anim.isInteractive() })
	const dim, blue = "38;2;108;112;134m  X ", "38;2;137;179;250m  T " // #6c7086; #89b4fa as lipgloss rounds it
	if v := d.m.help.view(); !strings.Contains(v, dim) || !strings.Contains(v, blue) {
		t.Errorf("X should be drawn dim and T lit:\n%q", v)
	}

	d.m.tabs, d.m.focus = nil, panelPage
	items, _ = d.m.panelMenu()
	var lit []string
	for _, x := range keyReference(items) {
		if x.key != "" && !x.disabled && x.desc != "" && !isCore(x.key) {
			lit = append(lit, x.key)
		}
	}
	if strings.Join(lit, " ") != "T L Z" {
		t.Errorf("[2] with no tab: only T, L and Z can run, got %v", lit)
	}
}

func isCore(k string) bool {
	for _, c := range coreKeys {
		if c.key == k {
			return true
		}
	}
	return false
}

// rowOff is whether the row a key reaches is dimmed, in the Space menu and
// in ?; ok false when there is no such row.
func rowOff(t *testing.T, d *driver, key string) (off, ok bool) {
	t.Helper()
	items, _ := d.m.panelMenu()
	for _, it := range items {
		if it.key == key {
			off, ok = it.disabled, true
		}
	}
	for _, e := range keyReference(items) {
		k := e.key
		if k == "Enter" {
			k = "enter"
		}
		if k == key && e.disabled != off {
			t.Errorf("%s: ? says disabled %v, the Space menu %v", key, e.disabled, off)
		}
	}
	return off, ok
}

// A row lit in the Space menu runs when pressed: what it needs is what
// dims it, not a toast after the press (tdp M6).
func TestRowsDimWhenTheyCannotRun(t *testing.T) {
	d := keysDriver(t)
	tb := &tab{id: 1, url: "https://example.com/", title: "Example"}
	d.m.tabs, d.m.shown, d.m.focus = []*tab{tb}, 0, panelPage

	// Back and forward: only with a page to go to.
	tb.apply(pageMsg{tabID: 1, url: tb.url, back: true}, 80)
	if p, _ := rowOff(t, d, "P"); p {
		t.Error("Previous should be lit with a page to go back to")
	}
	if n, _ := rowOff(t, d, "N"); !n {
		t.Error("Next should be dimmed with nothing ahead")
	}
	tb.apply(pageMsg{tabID: 1, url: tb.url, forward: true}, 80)
	if p, _ := rowOff(t, d, "P"); !p {
		t.Error("Previous should be dimmed with nothing behind")
	}
	if n, _ := rowOff(t, d, "N"); n {
		t.Error("Next should be lit with a page ahead")
	}

	// Visual mode and search: only with a page captured.
	tb.root = nil
	for _, k := range []string{"v", "/"} {
		if off, _ := rowOff(t, d, k); !off {
			t.Errorf("%s should be dimmed before the page is captured", k)
		}
	}

	// History: Clear with nothing to clear.
	d.key("H")
	if off, ok := rowOff(t, d, "C"); !ok || !off {
		t.Errorf("History Clear should be listed and dimmed with no visit: listed %v", ok)
	}

	// Downloads: Open file only on a finished one; Clear done only with
	// something finished or cancelled.
	d.m.dls = []download{{name: "a.zip", state: dlRunning}}
	d.key("D")
	if off, _ := rowOff(t, d, "enter"); !off {
		t.Error("Open file should be dimmed while downloading")
	}
	if off, _ := rowOff(t, d, "C"); !off {
		t.Error("Clear done should be dimmed with only a running download")
	}
	for _, st := range []downloadState{dlCancelled, dlDone} {
		d.m.dls = []download{{name: "a.zip", state: st}}
		d.m.lists.setEntries(d.m.listEntries(listDownloads))
		if off, _ := rowOff(t, d, "enter"); off != (st != dlDone) {
			t.Errorf("state %d: Open file disabled %v", st, off)
		}
		if off, _ := rowOff(t, d, "C"); off {
			t.Errorf("state %d: Clear done should be lit", st)
		}
	}
}
