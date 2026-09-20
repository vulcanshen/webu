package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
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
	// prepared: the mutation observer is armed (page.Prepare), which has to
	// happen before the first navigation and only once.
	prepared bool
	// lastVisit is the URL last written to the history log for this tab,
	// so a settle capture of the same page does not log it again.
	lastVisit string
	// certErr: the last navigation failed on the certificate; certAsked:
	// the user has been asked about it once already for this page.
	certErr, certAsked bool
	// frozen is a capture that arrived while selection mode held the page
	// still (ux.md §1); applied when the mode ends.
	frozen *pageMsg
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

// dialogMsg is a page asking something through alert / confirm / prompt /
// beforeunload (function.md §5). The page is stalled until it is answered.
type dialogMsg struct {
	tabID         int
	kind          cdppage.DialogType
	message       string
	defaultPrompt string
}

// newTargetMsg is a page opening a window of its own — target=_blank,
// window.open — which becomes a tab (function.md §5).
type newTargetMsg struct {
	id  target.ID
	url string
}

// downloadMsg is a download starting or finishing (function.md §8).
type downloadMsg struct {
	name   string
	path   string
	done   bool
	failed bool
}

// settleMsg fires the re-capture a pageEventMsg or an action asked for.
type settleMsg struct {
	tabID int
	gen   int
}

// newTab opens a target on the browser and starts listening to it.
func (m *AppModel) newTab() *tab {
	return m.newTabWith(chromedp.NewContext(m.browser.Ctx))
}

// newTabFor adopts a target the page opened itself.
func (m *AppModel) newTabFor(id target.ID) *tab {
	return m.newTabWith(chromedp.NewContext(m.browser.Ctx, chromedp.WithTargetID(id)))
}

func (m *AppModel) newTabWith(ctx context.Context, cancel context.CancelFunc) *tab {
	t := &tab{id: m.nextTabID, ctx: ctx, cancel: cancel, cursor: -1}
	m.nextTabID++
	ch := m.events
	id := t.id
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *cdppage.EventLoadEventFired, *cdppage.EventNavigatedWithinDocument, *cdppage.EventFrameNavigated:
		case *runtime.EventBindingCalled:
			// The injected observer saying the DOM changed (function.md §6).
			if e.Name != page.MutationBinding {
				return
			}
		case *cdppage.EventJavascriptDialogOpening:
			// Never dropped: the page is stalled until this is answered.
			msg := dialogMsg{tabID: id, kind: e.Type, message: e.Message, defaultPrompt: e.DefaultPrompt}
			go func() { ch <- msg }()
			return
		default:
			return
		}
		select {
		case ch <- pageEventMsg{tabID: id}:
		default: // a burst of events is one redraw; dropping the rest is the debounce
		}
	})
	return t
}

// listenBrowser wires the browser-level events: a page opening a window of
// its own, and downloads. Both are handed over in a goroutine rather than
// dropped — a tab or a file the user never hears about is worse than a
// late message.
func (m *AppModel) listenBrowser() {
	if m.browser == nil {
		return
	}
	ch := m.events
	chromedp.ListenBrowser(m.browser.Ctx, func(ev any) {
		switch e := ev.(type) {
		case *target.EventTargetCreated:
			info := e.TargetInfo
			if info.Type != "page" || info.OpenerID == "" || info.Attached {
				return
			}
			msg := newTargetMsg{id: info.TargetID, url: info.URL}
			go func() { ch <- msg }()
		case *cdpbrowser.EventDownloadWillBegin:
			msg := downloadMsg{name: e.SuggestedFilename}
			go func() { ch <- msg }()
		case *cdpbrowser.EventDownloadProgress:
			switch e.State {
			case cdpbrowser.DownloadProgressStateCompleted:
				msg := downloadMsg{done: true, path: e.FilePath}
				go func() { ch <- msg }()
			case cdpbrowser.DownloadProgressStateCanceled:
				msg := downloadMsg{failed: true}
				go func() { ch <- msg }()
			}
		}
	})
}

// waitEvent hands the next browser or page event to Update; the app
// re-issues it after every one, so the channel is always being read.
func waitEvent(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// adopt attaches to a target the page opened and captures it. The target
// already has a document, so Prepare runs the observer in it as well.
func (t *tab) adopt() tea.Cmd {
	t.gen++
	t.loading, t.prepared = true, true
	gen, id, ctx := t.gen, t.id, t.ctx
	return func() tea.Msg {
		if err := chromedp.Run(ctx); err != nil {
			return pageMsg{tabID: id, gen: gen, err: fmt.Errorf("attach: %w", err)}
		}
		if err := page.Prepare(ctx); err != nil {
			return pageMsg{tabID: id, gen: gen, err: fmt.Errorf("prepare: %w", err)}
		}
		if err := chromedp.Run(ctx, chromedp.WaitReady("body")); err != nil {
			return pageMsg{tabID: id, gen: gen, err: fmt.Errorf("wait: %w", err)}
		}
		return capture(ctx, id, gen)
	}
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
	prepare := !t.prepared
	t.prepared = true
	return func() tea.Msg {
		if prepare {
			if err := page.Prepare(ctx); err != nil {
				return pageMsg{tabID: id, gen: gen, url: url, err: fmt.Errorf("prepare: %w", err)}
			}
		}
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
		// net::ERR_CERT_* is Chromium refusing the site's certificate; the
		// user gets to overrule it (function.md §8).
		if was := t.certErr; !was {
			t.certAsked = false
		}
		t.certErr = strings.Contains(t.errText, "ERR_CERT")
		return
	}
	t.errText = ""
	t.certErr = false
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
