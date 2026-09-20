package page

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Every action is addressed by backendDOMNodeId — the one link the IR keeps
// to Chromium (function.md §2). None of them waits for what the page does
// next; the caller re-captures when it is ready to look.

// Reveal scrolls the node into the viewport. The TUI's scrolling is not
// Chromium's, and a page lazy-loads on "did this enter the viewport" — so
// the cursor landing on a node is what has to move the real viewport
// (function.md §4). A node with no box is not an error.
func Reveal(ctx context.Context, id cdp.BackendNodeID) error {
	return run(ctx, func(ctx context.Context) error { return reveal(ctx, id) })
}

func reveal(ctx context.Context, id cdp.BackendNodeID) error {
	err := dom.ScrollIntoViewIfNeeded().WithBackendNodeID(id).Do(ctx)
	if err != nil && strings.Contains(err.Error(), "layout object") {
		return nil
	}
	return err
}

// Click is a mouse click on the node: a synthetic left press and release
// at the centre of its box, so the page sees mousedown, mouseup and a
// TRUSTED click — which is what lets a target=_blank link open its window;
// el.click() is untrusted and Chrome refuses it that. A node with no box
// gets el.click(), the only click there is for it.
//
// There is deliberately NO mouseMoved before the press. Measured against
// the pinned Chromium: a lone synthetic move is acknowledged by the
// headless renderer only after a five-second timeout, so a click that
// opened a dialog showed it five seconds late, and every other click
// paid the same. Press and release alone are answered in a millisecond.
// The cost is hover: a menu that opens on pointer-over does not open for
// webu yet (function.md §4 asked for it; webu-implementation.md §4 says
// why not).
func Click(ctx context.Context, id cdp.BackendNodeID) error {
	return run(ctx, func(ctx context.Context) error {
		if err := reveal(ctx, id); err != nil {
			return err
		}
		box, err := dom.GetBoxModel().WithBackendNodeID(id).Do(ctx)
		if err != nil || box == nil || len(box.Content) < 8 {
			return jsClick(ctx, id)
		}
		x, y := quadCentre(box.Content)
		if x < 0 || y < 0 {
			return jsClick(ctx, id)
		}
		if err := input.DispatchMouseEvent(input.MousePressed, x, y).
			WithButton(input.Left).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseReleased, x, y).
			WithButton(input.Left).WithClickCount(1).Do(ctx)
	})
}

func quadCentre(q dom.Quad) (float64, float64) {
	var sx, sy float64
	for i := 0; i+1 < len(q); i += 2 {
		sx += q[i]
		sy += q[i+1]
	}
	n := float64(len(q) / 2)
	return sx / n, sy / n
}

func jsClick(ctx context.Context, id cdp.BackendNodeID) error {
	return call(ctx, id, `function() { this.click(); }`)
}

// call runs a function with the node as `this`. Needs an executor context:
// only reached from inside run.
func call(ctx context.Context, id cdp.BackendNodeID, fn string) error {
	obj, err := dom.ResolveNode().WithBackendNodeID(id).Do(ctx)
	if err != nil {
		return err
	}
	if obj == nil || obj.ObjectID == "" {
		return errors.New("node is gone")
	}
	_, exc, err := runtime.CallFunctionOn(fn).WithObjectID(obj.ObjectID).Do(ctx)
	if err != nil {
		return err
	}
	if exc != nil {
		return fmt.Errorf("page script: %s", exc.Text)
	}
	return nil
}

// Type replaces a field's text: focus it, select everything in it, and
// insert the new text the way an IME would — Input.insertText fires the
// input events a framework listens for, where setting .value would not.
// An empty text clears the field.
func Type(ctx context.Context, id cdp.BackendNodeID, text string) error {
	return run(ctx, func(ctx context.Context) error {
		if err := reveal(ctx, id); err != nil {
			return err
		}
		if err := dom.Focus().WithBackendNodeID(id).Do(ctx); err != nil {
			return err
		}
		if err := call(ctx, id, `function() {
			if (typeof this.select === "function") { this.select(); return; }
			const sel = window.getSelection(); const r = document.createRange();
			r.selectNodeContents(this); sel.removeAllRanges(); sel.addRange(r);
		}`); err != nil {
			return err
		}
		if text == "" {
			return input.DispatchKeyEvent(input.KeyDown).WithKey("Backspace").WithCode("Backspace").
				WithWindowsVirtualKeyCode(8).Do(ctx)
		}
		return input.InsertText(text).Do(ctx)
	})
}

// Choose selects one option of a <select> and tells the page (function.md
// §4): the option element is marked selected and the select dispatches the
// input and change events a listener expects.
func Choose(ctx context.Context, option cdp.BackendNodeID) error {
	return run(ctx, func(ctx context.Context) error {
		return call(ctx, option, `function() {
			this.selected = true;
			const s = this.closest("select");
			if (s) {
				s.dispatchEvent(new Event("input", {bubbles: true}));
				s.dispatchEvent(new Event("change", {bubbles: true}));
			}
		}`)
	})
}

// Submit presses Enter in the field — implicit submission, the way a search
// box with no button is sent (ux.md §2.2).
func Submit(ctx context.Context, id cdp.BackendNodeID) error {
	return run(ctx, func(ctx context.Context) error {
		if err := dom.Focus().WithBackendNodeID(id).Do(ctx); err != nil {
			return err
		}
		return key(ctx, "Enter")
	})
}

// Key sends one key press to the page. Only the keys webu needs are known:
// Enter and Escape.
func Key(ctx context.Context, name string) error {
	return run(ctx, func(ctx context.Context) error { return key(ctx, name) })
}

func key(ctx context.Context, name string) error {
	var vk int64
	switch name {
	case "Enter":
		vk = 13
	case "Escape":
		vk = 27
	default:
		return fmt.Errorf("key %q is not one webu sends", name)
	}
	down := input.DispatchKeyEvent(input.KeyDown).WithKey(name).WithCode(name).WithWindowsVirtualKeyCode(vk)
	if name == "Enter" {
		down = down.WithText("\r")
	}
	if err := down.Do(ctx); err != nil {
		return err
	}
	return input.DispatchKeyEvent(input.KeyUp).WithKey(name).WithCode(name).WithWindowsVirtualKeyCode(vk).Do(ctx)
}

// Navigate points the tab at url and waits for the load event. A navigation
// Chromium refuses (DNS, connection) comes back as an error carrying its
// errorText — chromedp folds it into the error — which is what the error
// page shows (function.md §8).
//
// chromedp.Run is also what brings a fresh tab context to life: the target
// is only created on the first Run.
func Navigate(ctx context.Context, url string) error {
	return chromedp.Run(ctx, chromedp.Navigate(url), chromedp.WaitReady("body"))
}

// Location is the tab's current URL and title.
func Location(ctx context.Context) (url, title string, err error) {
	err = chromedp.Run(ctx, chromedp.Location(&url), chromedp.Title(&title))
	return
}

// Back, Forward and Reload move through the tab's own navigation stack
// (function.md §8).
func Back(ctx context.Context) error    { return chromedp.Run(ctx, chromedp.NavigateBack()) }
func Forward(ctx context.Context) error { return chromedp.Run(ctx, chromedp.NavigateForward()) }
func Reload(ctx context.Context) error  { return chromedp.Run(ctx, chromedp.Reload()) }
