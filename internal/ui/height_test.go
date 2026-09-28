package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/webu/internal/page"
)

// rowsOf is how many rows a popup's box takes on screen.
func rowsOf(view string) int { return len(strings.Split(ansi.Strip(view), "\n")) }

// A popup's height is set as it opens and stays (tdp F7): content that
// grows scrolls inside the box, content that shrinks leaves blank rows.

func TestEditorKeepsItsHeight(t *testing.T) {
	e := newEditorPopup()
	e.setSize(100, 40)
	e.ask("text, several lines", "Notes", "one", 7, 1)
	e.anim.phase = animOpen
	h := rowsOf(e.view())
	for i := 0; i < 30; i++ {
		e.update(tea.KeyMsg{Type: tea.KeyEnter})
		e.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line")})
	}
	if got := rowsOf(e.view()); got != h {
		t.Errorf("thirty more lines should scroll, not grow the box: %d rows, then %d", h, got)
	}
}

func TestFilePickerKeepsItsHeight(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.html", "b.html", "c.html"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	sub := filepath.Join(dir, "big")
	os.Mkdir(sub, 0o755)
	for i := 0; i < 60; i++ {
		os.WriteFile(filepath.Join(sub, "f"+itoa(i)+".html"), []byte("x"), 0o644)
	}
	m := newFilePicker()
	m.setSize(100, 40)
	m.open(glyphFolder, "Import", dir, 1)
	m.anim.phase = animOpen
	h := rowsOf(m.view())
	m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzz")}) // no match
	if got := rowsOf(m.view()); got != h {
		t.Errorf("filtering to nothing should not shrink the box: %d rows, then %d", h, got)
	}
	m.enter(sub) // a folder of sixty
	if got := rowsOf(m.view()); got != h {
		t.Errorf("a bigger folder should scroll, not grow the box: %d rows, then %d", h, got)
	}
	// Set as it opens, from what the folder holds: opened on the big one,
	// the box is taller than on the folder of three.
	big := newFilePicker()
	big.setSize(100, 40)
	big.open(glyphFolder, "Import", sub, 1)
	big.anim.phase = animOpen
	if got := rowsOf(big.view()); got <= h {
		t.Errorf("a folder of sixty should open taller than one of three: %d rows, then %d", h, got)
	}
}

func TestNetworkDetailKeepsItsHeight(t *testing.T) {
	m := newDevDetailPopup()
	m.setSize(100, 40)
	m.show(page.NetEntry{URL: "https://example.com/", Mime: "text/plain", Status: 200}, 2)
	m.anim.phase = animOpen
	h := rowsOf(m.view())
	// The body is on its way: the box opens at its full height, room for it.
	if want := m.maxRows() + 2; h != want {
		t.Errorf("the detail should open at its full height: %d rows, want %d", h, want)
	}
	m.setBody(strings.Repeat("a line of the body\n", 200), nil)
	if got := rowsOf(m.view()); got != h {
		t.Errorf("the body arriving should fill the box, not grow it: %d rows, then %d", h, got)
	}
}

// A box whose submit can fail opens with its error row; a refusal writes
// there, the box keeps its height and what was typed (tdp F7, K3). A box
// that cannot fail has no such row.
func TestInputErrorRow(t *testing.T) {
	d := keysDriver(t)

	d.key("L")
	d.until("the location box", func() bool { return d.m.input.anim.isInteractive() })
	plain := rowsOf(d.m.input.view())
	d.key("esc")
	d.until("closed", func() bool { return !d.m.input.anim.owns() })

	// A folder goes where the cursor is: under news, x is taken.
	d.m.folders = []string{"news", "news/x"}
	d.key("B")
	d.m.lists.setEntries(d.m.bookmarkEntries())
	d.m.lists.cursorToFolder("news")
	d.key("A")
	d.until("the folder box", func() bool { return d.m.input.anim.isInteractive() })
	h := rowsOf(d.m.input.view())
	if h != plain+2 {
		t.Errorf("a box that can refuse should open with a blank row and its error row: %d rows, a plain one %d", h, plain)
	}
	d.key("news")
	d.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("x")
	d.key("enter")
	if !d.m.input.anim.owns() || d.m.input.value != "x" || !strings.Contains(d.m.input.refused, "already") {
		t.Fatalf("a taken name should keep the box and what was typed, saying why: open %v value %q refused %q",
			d.m.input.anim.owns(), d.m.input.value, d.m.input.refused)
	}
	if got := rowsOf(d.m.input.view()); got != h || !strings.Contains(ansi.Strip(d.m.input.view()), "already a folder") {
		t.Errorf("the refusal should be written in the error row, the height as it was: %d rows, then %d", h, got)
	}
	d.key("s") // a new name clears the error
	if d.m.input.refused != "" {
		t.Error("typing should clear the refusal")
	}
	d.key("enter")
	d.until("the folder made", func() bool { return !d.m.input.anim.owns() && len(d.m.folders) == 3 })
}
