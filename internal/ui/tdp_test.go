package ui

import (
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
// never reaches the terminal as it is (components, control characters):
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
