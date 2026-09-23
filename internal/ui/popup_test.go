package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A popup is told by behaviour: new at the top of the tree, over
// something, the keyboard in it or declared modal, and something to
// press (popup.go).
func TestAPopupIsToldByBehaviour(t *testing.T) {
	box := func(x, y, w, h float64) ir.Box { return ir.Box{X: x, Y: y, W: w, H: h} }
	body := &ir.Node{Kind: ir.Landmark, Role: "main", ID: 1, Children: []*ir.Node{para(text("the page"))}}
	button := func(id int, name string, focused bool) *ir.Node {
		return &ir.Node{Kind: ir.Button, Name: name, ID: cdp.BackendNodeID(id), Focusable: true, Focused: focused}
	}
	prev := map[cdp.BackendNodeID]bool{1: true}
	boxes := map[cdp.BackendNodeID]ir.Box{1: box(0, 0, 1000, 2000)}

	// A sheet over the body with the keyboard in it.
	sheet := &ir.Node{Kind: ir.Group, ID: 10, Children: []*ir.Node{para(text("Session expiring")), button(11, "Stay", true)}}
	boxes[10] = box(200, 100, 600, 300)
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{body, sheet}}
	if p := findPopup(prev, root, boxes); p != sheet {
		t.Errorf("a new block over the body with the keyboard in it is a popup: %v", p)
	}
	// The same block in the last capture already: not new, not a popup.
	if p := findPopup(map[cdp.BackendNodeID]bool{1: true, 10: true}, root, boxes); p != nil {
		t.Errorf("a block that was there already is not a popup: %v", p)
	}
	// Nothing to press: a toast, content.
	toast := &ir.Node{Kind: ir.Group, ID: 20, Children: []*ir.Node{para(text("Saved."))}}
	boxes[20] = box(800, 10, 150, 40)
	if p := findPopup(prev, &ir.Node{Kind: ir.Document, Children: []*ir.Node{body, toast}}, boxes); p != nil {
		t.Errorf("a block with nothing to press is not a popup: %v", p)
	}
	// Something to press but no keyboard and no declaration: a chat
	// bubble, content.
	bubble := &ir.Node{Kind: ir.Group, ID: 30, Children: []*ir.Node{button(31, "Chat", false)}}
	boxes[30] = box(900, 1800, 80, 80)
	if p := findPopup(prev, &ir.Node{Kind: ir.Document, Children: []*ir.Node{body, bubble}}, boxes); p != nil {
		t.Errorf("a block the page did not put the keyboard into is not a popup: %v", p)
	}
	// The keyboard in it, but laid out AFTER the body rather than over
	// it: content that arrived, not a popup.
	after := &ir.Node{Kind: ir.Group, ID: 40, Children: []*ir.Node{button(41, "Load more", true)}}
	boxes[40] = box(0, 2000, 1000, 60)
	if p := findPopup(prev, &ir.Node{Kind: ir.Document, Children: []*ir.Node{body, after}}, boxes); p != nil {
		t.Errorf("a block after the body is in the flow, not over it: %v", p)
	}
	// Declared modal: the declaration is the whole of the evidence, with
	// no geometry at all.
	dlg := &ir.Node{Kind: ir.Landmark, Role: "dialog", ID: 50, Modal: true, Children: []*ir.Node{para(text("Cookies?")), button(51, "Accept", false)}}
	if p := findPopup(prev, &ir.Node{Kind: ir.Document, Children: []*ir.Node{dlg}}, map[cdp.BackendNodeID]ir.Box{}); p != dlg {
		t.Errorf("a modal dialog is a popup on its word alone: %v", p)
	}
	// And wherever the page hung it: inside main, after the example it
	// belongs to, the way the ARIA practices' own examples are built.
	inner := &ir.Node{Kind: ir.Landmark, Role: "main", ID: 1, Children: []*ir.Node{para(text("the page")), dlg}}
	if p := findPopup(prev, &ir.Node{Kind: ir.Document, Children: []*ir.Node{inner}}, map[cdp.BackendNodeID]ir.Box{}); p != dlg {
		t.Errorf("a dialog inside main is found there: %v", p)
	}
	// A new subtree's root is the candidate, not every new node under
	// it: the dialog, not its Accept button.
	if p := findPopup(map[cdp.BackendNodeID]bool{1: true}, &ir.Node{Kind: ir.Document, Children: []*ir.Node{inner}}, nil); p != dlg {
		t.Errorf("the root of the new subtree: %v", p)
	}
}

// Against the pinned Chromium: a native modal dialog, a custom sheet
// that takes the keyboard, and a toast. The first two become the panel,
// Esc does not leave them, and answering them puts the page back; the
// toast is content.
func TestPopupsArePanelsUntilAnswered(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/popup.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the popups page", d.loaded("Popups"))
	p := d.page()
	if p.at != partMain || p.popup != 0 {
		t.Fatalf("opens on the body with no popup: at=%s popup=%d", p.at.word(), p.popup)
	}

	// The native modal.
	d.cursorOn(ir.Button, "Open dialog")
	d.key("enter")
	d.until("the dialog is the panel", func() bool { return p.popupNode() != nil })
	v := dumpLayout(p.lay)
	if !strings.Contains(v, "We use cookies") || strings.Contains(v, "Some body text") {
		t.Errorf("the cursor walks the dialog and nothing else:\n%s", v)
	}
	// Drawn as a float over the page: the dialog in a box, the page
	// still there under it.
	if f := d.m.pagePopupView(p); !strings.Contains(f, "We use cookies") || !strings.Contains(f, "Accept") {
		t.Errorf("the float is the dialog:\n%s", f)
	}
	if view := d.m.View(); !strings.Contains(view, "We use cookies") || !strings.Contains(view, "Some body text") {
		t.Errorf("the page shows through under the float:\n%s", view)
	}
	d.key("esc")
	if p.popupNode() == nil || !d.m.toast.isActive() {
		t.Error("Esc does not leave a popup; it says so")
	}
	d.key("esc") // the toast
	// No line numbers in a dialog, and nowhere for [go] to go.
	if p.gutter != 0 {
		t.Errorf("a popup has no gutter: %d", p.gutter)
	}
	d.key("g")
	d.key("o")
	if d.m.finder.isActive() || !d.m.toast.isActive() {
		t.Error("[go] in a popup says there is nowhere to go")
	}
	d.key("esc")
	d.cursorOn(ir.Button, "Accept")
	d.key("enter")
	d.until("answered", func() bool { return p.popupNode() == nil && strings.Contains(dumpLayout(p.lay), "accepted") })
	if p.at != partMain {
		t.Errorf("answering it puts the body back: at=%s", p.at.word())
	}

	// The custom sheet: no role, no modal — the page put the keyboard
	// into it, over the body.
	d.cursorOn(ir.Button, "Open sheet")
	d.key("enter")
	d.until("the sheet is the panel", func() bool { return p.popupNode() != nil })
	if v := dumpLayout(p.lay); !strings.Contains(v, "about to expire") || strings.Contains(v, "Some body text") {
		t.Errorf("the cursor walks the sheet:\n%s", v)
	}
	d.cursorOn(ir.Button, "Stay signed in")
	d.key("enter")
	d.until("extended", func() bool { return p.popupNode() == nil && strings.Contains(dumpLayout(p.lay), "extended") })

	// A disabled control: Enter says why nothing happened, rather than
	// clicking on nothing.
	d.cursorOn(ir.Button, "Nope")
	d.key("enter")
	if !d.m.toast.isActive() || !strings.Contains(d.m.toast.msg, "disabled") {
		t.Errorf("Enter on a disabled button says so: %q", d.m.toast.msg)
	}
	d.key("esc")

	// The toast: nothing to press, so it is content, and the page stays.
	d.cursorOn(ir.Button, "Notify")
	d.key("enter")
	deadline := time.Now().Add(2 * time.Second)
	d.until("the toast has landed", func() bool { return strings.Contains(dumpLayout(p.lay), "Saved.") || time.Now().After(deadline) })
	if p.popupNode() != nil {
		t.Error("a toast is not a popup")
	}
	if v := dumpLayout(p.lay); !strings.Contains(v, "Some body text") {
		t.Errorf("the page is still the page:\n%s", v)
	}
}

// A press right after a walk of the cursor lands where the cursor is.
// Every cursor move reveals its node, and a walk leaves reveals in
// flight when Enter comes; a click reads its target's box and presses
// at that point, and a stale reveal scrolling the page between the two
// put one press in six on whatever had moved under it (2026-09-23).
// The page actions are serialised per tab now (tab.act).
func TestAPressLandsWhereTheCursorIs(t *testing.T) {
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
	abs, _ := filepath.Abs("testdata/popup.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the popups page", d.loaded("Popups"))
	p := d.page()
	for round := 1; round <= 5; round++ {
		// To the bottom of the page and back up to the button, then Enter
		// at once: the reveals of the walk are still in flight.
		d.key("G")
		d.cursorOn(ir.Button, "Open dialog")
		d.key("enter")
		deadline := time.Now().Add(4 * time.Second)
		d.until("the dialog", func() bool { return p.popupNode() != nil || time.Now().After(deadline) })
		if p.popupNode() == nil {
			t.Fatalf("round %d: the press missed its button", round)
		}
		d.cursorOn(ir.Button, "Accept")
		d.key("enter")
		deadline = time.Now().Add(4 * time.Second)
		d.until("answered", func() bool { return p.popupNode() == nil || time.Now().After(deadline) })
		if p.popupNode() != nil {
			t.Fatalf("round %d: Accept did not close the dialog", round)
		}
	}
}
