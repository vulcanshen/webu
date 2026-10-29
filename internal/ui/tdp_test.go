package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/terminu-go/tdptest"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/store"
)

// TestTDP is webu's first launch walked as tdptest does for every terminu
// app (tdp L1, L4, E5): on many terminal sizes and icon widths, the Space
// menu, the global operation popup, the App Log and ? on every screen and
// panel, then under the least size and back. webu runs without Chromium
// here, as New(nil) does: what is checked is the screen, not a page.
func TestTDP(t *testing.T) {
	tdptest.Check(t, tdptest.App{Name: "webu", New: func() *terminu.App { return New(nil).Program() }})
}

// What webu shows from outside — a page's title, its text, a bookmark —
// never reaches the terminal as it is (tdp components.5):
// tdptest's tricky values in each, every frame checked on the way.
func TestTrickyTextFromOutside(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	var bms []store.Bookmark
	var paras []*ir.Node
	for i, v := range tdptest.Tricky {
		bms = append(bms, store.Bookmark{Title: v, URL: "https://example.com/" + itoa(i) + "/" + v})
		paras = append(paras, para(text(v)))
	}
	m := New(nil).WithStore(bms, nil, store.Config{}, nil)
	d := tdptest.Start(t, m.Program(), 100, 30)
	// The \n drawn by name is the tricky data's own, from outside.
	d.AllowLineBreaks = true
	d.Note("tdptest.Tricky as tab titles, the page's text, bookmarks")
	d.Do(func() tea.Cmd {
		for i, v := range tdptest.Tricky {
			tb := &tab{id: i + 1, url: "https://example.com/" + itoa(i), title: v, root: doc(append([]*ir.Node{hd(1, v)}, paras...)...), cursor: -1}
			m.tabs = append(m.tabs, tb)
		}
		m.shown = 0
		m.tabs[0].relayout(m.pageW())
		m.sync()
		return nil
	})
	d.Press("2", "G", "v", "Esc", "B", "?", "Esc", "W")
}

// The App Log has what the user did (tdp note.7): an operation run by its
// key or by Enter, in webu's words, a switch of screens in the lib's.
func TestTheAppLogHasWhatTheUserDid(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	bms := []store.Bookmark{{Title: "Go", URL: "https://go.dev", Folder: "dev"}}
	m := New(nil).WithStore(bms, nil, store.Config{}, nil)
	d := tdptest.Start(t, m.Program(), 100, 30)
	d.Do(func() tea.Cmd {
		tb := &tab{id: 1, url: "https://example.com/", title: "Example", root: doc(hd(1, "Example"), para(text("Words"))), cursor: -1}
		m.tabs = append(m.tabs, tb)
		m.shown = 0
		tb.relayout(m.pageW())
		m.sync()
		return nil
	})
	d.Press("2", "j", "Z", "Z", "B", "Enter", "!")
	for _, want := range []string{"Zoomed in on the page", "Zoomed out to the tabs and the page",
		"Switched to Bookmarks", "Shut the folder dev"} {
		if !d.Has(want) {
			t.Errorf("the App Log should say %q:\n%s", want, strings.Join(d.Screen(), "\n"))
		}
	}
}

// Visual mode has a layer (tdp K11): selecting, it is Visual>select and
// Esc unselects, back to Visual; Esc again leaves the mode. Once
// selecting, v does nothing: it does not go back (tdp note.4, K7).
func TestVisualModeGoesBackALayer(t *testing.T) {
	m := New(nil)
	d := tdptest.Start(t, m.Program(), 100, 30)
	d.Do(func() tea.Cmd {
		tb := &tab{id: 1, url: "https://example.com/", title: "Example", root: doc(para(text("Some words to select"))), cursor: -1}
		m.tabs = append(m.tabs, tb)
		m.shown = 0
		tb.relayout(m.pageW())
		m.sync()
		return nil
	})
	screen := func() string { return strings.Join(d.Screen(), "\n") }
	d.Press("2", "v", "v")
	if !d.Has("Visual>select") || !d.Has("Esc:unselect") {
		t.Fatalf("selecting, the mode is Visual>select and Esc unselects:\n%s", screen())
	}
	d.Press("Esc")
	if d.Has("Visual>select") || !d.Has("Visual") || !m.sel.on || m.sel.selecting || !d.Has("v:select") {
		t.Fatalf("Esc goes back to Visual, still in the mode, v on the hint:\n%s", screen())
	}
	d.Press("Esc")
	if m.sel.on || m.pagePanel.InMode() != nil {
		t.Fatalf("Esc again leaves the mode:\n%s", screen())
	}
	d.Press("v", "v", "v")
	if !d.Has("Visual>select") || !m.sel.selecting || d.Has("v:select") {
		t.Fatalf("v again does nothing, and the hint has no v:\n%s", screen())
	}
	d.Press("Esc", "Esc")
	if m.sel.on || m.pagePanel.InMode() != nil {
		t.Fatalf("Esc twice leaves the mode:\n%s", screen())
	}
}
