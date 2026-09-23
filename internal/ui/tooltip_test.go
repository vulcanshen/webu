package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A timer's text flows where it is; a tooltip is drawn, as an aside,
// while the page shows it — which it does on hover, and Enter hovers
// before it presses (user, 2026-09-23). Neither is a stop, and a
// tooltip is never a popup: it has nothing to press.
func TestATimerFlowsAndATooltipIsAnAside(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	abs, _ := filepath.Abs("testdata/tooltip.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the tooltip page", d.loaded("Tooltip"))
	p := d.page()
	v := dumpLayout(p.lay)

	rowWith := func(parts ...string) bool {
		for _, r := range p.lay.rows {
			s := r.plain()
			all := true
			for _, part := range parts {
				all = all && strings.Contains(s, part)
			}
			if all {
				return true
			}
		}
		return false
	}
	if !rowWith("Time left", "0", ":", "in this session.") {
		t.Errorf("a timer's text flows in its sentence:\n%s", v)
	}
	if !rowWith(glyphInfo, "Always shown") {
		t.Errorf("a tooltip the page shows is drawn as an aside:\n%s", v)
	}
	if strings.Contains(v, "Saves your work") {
		t.Errorf("a tooltip the page hides is not on the page:\n%s", v)
	}
	if strings.Contains(v, "timer") || strings.Contains(v, "tooltip") {
		t.Errorf("neither is a stop, nor named by its role:\n%s", v)
	}

	// Enter on the button hovers it: the tooltip appears, as an aside,
	// and is not taken for a popup.
	d.cursorOn(ir.Button, "Save")
	d.key("enter")
	d.until("the tooltip", func() bool { return rowWith(glyphInfo, "Saves your work") })
	if p.popupNode() != nil {
		t.Errorf("a tooltip is not a popup: %+v", p.popupNode())
	}
	if n := p.current(); n == nil || n.Name != "Save" {
		t.Errorf("the cursor stays on the button: %+v", n)
	}
}
