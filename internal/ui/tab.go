package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
)

// tab is one Chromium target and everything webu knows about it: the page
// as last captured, laid out for the panel's width, and where the cursor is
// in it. Panel [2] lists them; panel [3] shows one.
type tab struct {
	id     int
	ctx    context.Context
	cancel context.CancelFunc

	url, title string
	loading    bool
	errText    string // a navigation Chromium refused: DNS, connection, certificate
	// pending: restored from the last session but not loaded yet — it loads
	// when it is switched to (ux.md §6).
	pending bool

	root   *ir.Node
	lay    layout
	layW   int
	cursor int // index into lay.items; -1 when the page has none
	top    int // first page row on screen

	// gen guards captures: a result from before the latest navigation is
	// thrown away rather than drawn over the newer page.
	gen int
}

// pageMsg is a capture landing: the page as Chromium has it now.
type pageMsg struct {
	tabID int
	gen   int
	url   string
	title string
	cap   ir.Capture
	err   error
}

// pageEventMsg says Chromium reported a navigation or load in this tab;
// the app re-captures after a short settle.
type pageEventMsg struct{ tabID int }

// settleMsg fires the re-capture a pageEventMsg or an action asked for.
type settleMsg struct {
	tabID int
	gen   int
}

// newTab opens a target on the browser and starts listening to it.
func (m *AppModel) newTab() *tab {
	ctx, cancel := chromedp.NewContext(m.browser.Ctx)
	t := &tab{id: m.nextTabID, ctx: ctx, cancel: cancel, cursor: -1}
	m.nextTabID++
	ch := m.events
	id := t.id
	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev.(type) {
		case *cdppage.EventLoadEventFired, *cdppage.EventNavigatedWithinDocument, *cdppage.EventFrameNavigated:
			select {
			case ch <- pageEventMsg{tabID: id}:
			default: // a burst of events is one redraw; dropping the rest is the debounce
			}
		}
	})
	return t
}

// waitEvent hands the next page event to Update; the app re-issues it after
// every one, so the channel is always being read.
func waitEvent(ch <-chan pageEventMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (t *tab) close() {
	if t.cancel != nil {
		t.cancel()
	}
}

// load navigates and captures. The tab shows as loading until the capture
// lands.
func (t *tab) load(url string) tea.Cmd {
	t.gen++
	t.loading, t.pending, t.errText = true, false, ""
	t.url = url
	gen, id, ctx := t.gen, t.id, t.ctx
	return func() tea.Msg {
		if err := page.Navigate(ctx, url); err != nil {
			return pageMsg{tabID: id, gen: gen, url: url, err: fmt.Errorf("navigate: %w", err)}
		}
		return capture(ctx, id, gen)
	}
}

// refresh captures the page as it is now, without navigating.
func (t *tab) refresh() tea.Cmd {
	t.gen++
	gen, id, ctx := t.gen, t.id, t.ctx
	return func() tea.Msg { return capture(ctx, id, gen) }
}

// settle asks for a capture a little later, so a click has had time to
// change the page; the gen keeps only the latest request alive.
func (t *tab) settle(after time.Duration) tea.Cmd {
	t.gen++
	gen, id := t.gen, t.id
	return tea.Tick(after, func(time.Time) tea.Msg { return settleMsg{tabID: id, gen: gen} })
}

// act runs one CDP action on the tab and then settles.
func (t *tab) act(fn func(context.Context) error) tea.Cmd {
	ctx := t.ctx
	do := func() tea.Msg {
		if err := fn(ctx); err != nil {
			return actionErrMsg{err: err}
		}
		return nil
	}
	return tea.Sequence(do, t.settle(300*time.Millisecond))
}

type actionErrMsg struct{ err error }

func capture(ctx context.Context, id, gen int) tea.Msg {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	url, title, err := page.Location(ctx)
	if err != nil {
		return pageMsg{tabID: id, gen: gen, err: fmt.Errorf("location: %w", err)}
	}
	c, err := page.Capture(ctx)
	if err != nil {
		err = fmt.Errorf("capture: %w", err)
	}
	return pageMsg{tabID: id, gen: gen, url: url, title: title, cap: c, err: err}
}

// apply takes a capture in: the page, its layout at the current width, and
// the cursor kept on the node it was on (function.md §4) — by ID first, and
// when the page rebuilt its DOM and the IDs are new, by the nearest item.
func (t *tab) apply(msg pageMsg, width int) {
	t.loading = false
	if msg.err != nil {
		t.errText = msg.err.Error()
		t.root, t.lay = nil, layout{}
		t.cursor = -1
		return
	}
	t.errText = ""
	if msg.url != "" {
		t.url = msg.url
	}
	t.title = msg.title
	if t.title == "" {
		t.title = t.url
	}
	var wasID = int64(-1)
	wasIdx := t.cursor
	if t.cursor >= 0 && t.cursor < len(t.lay.items) {
		wasID = int64(t.lay.items[t.cursor].node.ID)
	}
	t.root = ir.Build(msg.cap)
	t.relayout(width)
	t.cursor = -1
	for i, it := range t.lay.items {
		if int64(it.node.ID) == wasID {
			t.cursor = i
			break
		}
	}
	if t.cursor < 0 && len(t.lay.items) > 0 {
		t.cursor = clamp(wasIdx, 0, len(t.lay.items)-1)
	}
	t.scrollToCursor(0)
}

func (t *tab) relayout(width int) {
	if t.root == nil {
		return
	}
	t.lay = render(t.root, max(1, width))
	t.layW = width
}

// scrollToCursor keeps the cursor's rows on screen; visible is how many page
// rows the panel shows (0: unknown yet, keep top).
func (t *tab) scrollToCursor(visible int) {
	if visible <= 0 || t.cursor < 0 || t.cursor >= len(t.lay.items) {
		t.top = clamp(t.top, 0, max(0, len(t.lay.rows)-1))
		return
	}
	it := t.lay.items[t.cursor]
	if it.first < t.top {
		t.top = it.first
	}
	if it.last >= t.top+visible {
		t.top = it.last - visible + 1
	}
	t.top = clamp(t.top, 0, max(0, len(t.lay.rows)-1))
}

// current is the item under the cursor, or nil.
func (t *tab) current() *ir.Node {
	if t == nil || t.cursor < 0 || t.cursor >= len(t.lay.items) {
		return nil
	}
	return t.lay.items[t.cursor].node
}

// moveItem walks the cursor by navigation key. The page has a top and a
// bottom, so j/k do not wrap here (ux.md §3).
func (t *tab) moveItem(k string, visible int) {
	n := len(t.lay.items)
	if n == 0 {
		// No items: the keys scroll the text instead.
		t.top = moveScroll(t.top, max(0, len(t.lay.rows)-visible), k, visible)
		return
	}
	half := max(1, visible/2)
	switch k {
	case "j", "down":
		t.cursor = min(n-1, t.cursor+1)
	case "k", "up":
		t.cursor = max(0, t.cursor-1)
	case "d", "ctrl+d":
		t.cursor = t.itemFromRow(t.lay.items[t.cursor].first+half, 1)
	case "u", "ctrl+u":
		t.cursor = t.itemFromRow(t.lay.items[t.cursor].first-half, -1)
	case "gg":
		t.cursor = 0
	case "G":
		t.cursor = n - 1
	}
	t.scrollToCursor(visible)
}

// itemFromRow is the first item at or after row (dir 1) or at or before it
// (dir -1), clamped to the ends.
func (t *tab) itemFromRow(row, dir int) int {
	n := len(t.lay.items)
	if dir > 0 {
		for i := t.cursor + 1; i < n; i++ {
			if t.lay.items[i].first >= row {
				return i
			}
		}
		return n - 1
	}
	for i := t.cursor - 1; i >= 0; i-- {
		if t.lay.items[i].first <= row {
			return i
		}
	}
	return 0
}
