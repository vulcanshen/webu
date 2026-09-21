package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A bare v is the family's easter egg (its V is webu's visual mode): the
// logo reveals in stages, the name and version follow, and any key puts
// the screen back.
func TestSplashEasterEgg(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	d := newDriver(t, New(nil, ""))
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})

	d.key("V")
	if d.m.splash.isActive() {
		t.Fatal("V is visual mode, never the splash")
	}
	d.key("v")
	if !d.m.splash.isActive() {
		t.Fatal("v should open the splash")
	}
	d.until("the mark revealed and named", func() bool { return d.m.splash.identityVisible })
	v := d.m.View()
	if !strings.Contains(v, "webu") || !strings.Contains(v, "A terminal browser") || strings.Contains(v, "[1] Tabs") {
		t.Errorf("the splash should replace the frame and carry the name:\n%s", v)
	}
	if lines := strings.Split(v, "\n"); len(lines) != 30 {
		t.Errorf("the splash should fill the terminal: %d lines", len(lines))
	}
	d.key("j") // any key
	if d.m.splash.isActive() || !strings.Contains(d.m.View(), "[1] Tabs") {
		t.Error("any key should dismiss the splash and put the frame back")
	}

	// On a list screen v is the egg too; the screen is still there after.
	d.key("B")
	d.key("v")
	if !d.m.splash.isActive() {
		t.Fatal("v on the Bookmarks screen should open the splash")
	}
	d.key("esc")
	if d.m.splash.isActive() || d.m.screen != screenBookmarks {
		t.Errorf("Esc should close the splash and leave the screen as it was: %v", d.m.screen)
	}

	// Every logo row is as wide as the first, and only the four codes appear.
	for i, row := range logoPixels {
		if len(row) != len(logoPixels[0]) || strings.Trim(row, "DUWEB") != "" {
			t.Errorf("logo row %d: %q", i, row)
		}
	}
}
