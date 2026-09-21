package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
)

// tab is one Chromium target and everything webu knows about it: the page
// as last captured, laid out for the panel's width, and where the cursor is
// in it. Panel [1] lists them; panel [1] shows one.
type tab struct {
	id     int
	ctx    context.Context
	cancel context.CancelFunc

	url, title string
	loading    bool
	errText    string // a navigation Chromium refused: DNS, connection, certificate
	// anchors and parents come off the capture's DOM snapshot: every
	// element id, and every node's parent, so a link into the page can
	// land the cursor (jumpToAnchor).
	anchors map[string]cdp.BackendNodeID
	parents map[cdp.BackendNodeID]cdp.BackendNodeID
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
	// dev is the tab's network and console record for DevTools (ui.md §3.2).
	dev *page.DevLog
	// fold is the user's word on which landmarks are open or shut, by node
	// id, which survives a recapture; measure is the text width cap.
	fold    map[cdp.BackendNodeID]bool
	measure int
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

// downloadMsg is a download starting, moving, or finishing (function.md
// §8); guid is how the events of one download are told from another's.
type downloadMsg struct {
	guid, name, url, path string
	received, total       int64
	begin, done, failed   bool
}

// authMsg is an HTTP basic / digest challenge waiting on credentials
// (function.md §5). The request is paused until it is answered.
type authMsg struct {
	tabID  int
	id     fetch.RequestID
	origin string
	realm  string
	scheme string
}

// fileMsg is a file chooser the page opened; webu's own picker answers it.
type fileMsg struct {
	tabID    int
	node     cdp.BackendNodeID
	multiple bool
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
	t := &tab{id: m.nextTabID, ctx: ctx, cancel: cancel, cursor: -1, dev: &page.DevLog{},
		fold: map[cdp.BackendNodeID]bool{}, measure: m.cfg.TextWidth()}
	m.nextTabID++
	page.Observe(ctx, t.dev)
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
		case *fetch.EventRequestPaused:
			// Every request pauses once (auth handling is on); this is the
			// answer for the ones that asked nothing. Off the listener's
			// goroutine, or the answer would wait on the event loop that
			// carries it.
			reqID := e.RequestID
			go func() { _ = page.ContinueRequest(ctx, reqID) }()
			return
		case *fetch.EventAuthRequired:
			msg := authMsg{tabID: id, id: e.RequestID}
			if e.AuthChallenge != nil {
				msg.origin, msg.realm, msg.scheme = e.AuthChallenge.Origin, e.AuthChallenge.Realm, e.AuthChallenge.Scheme
			}
			go func() { ch <- msg }()
			return
		case *cdppage.EventFileChooserOpened:
			msg := fileMsg{tabID: id, node: e.BackendNodeID, multiple: e.Mode == cdppage.FileChooserOpenedModeSelectMultiple}
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
			msg := downloadMsg{guid: e.GUID, name: e.SuggestedFilename, url: e.URL, begin: true}
			go func() { ch <- msg }()
		case *cdpbrowser.EventDownloadProgress:
			msg := downloadMsg{guid: e.GUID, path: e.FilePath,
				received: int64(e.ReceivedBytes), total: int64(e.TotalBytes),
				done:   e.State == cdpbrowser.DownloadProgressStateCompleted,
				failed: e.State == cdpbrowser.DownloadProgressStateCanceled}
			go func() { ch <- msg }()
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

// navFailMsg is a history move that had nowhere to go, or failed: the tab
// stops showing as loading and the page on screen stays.
type navFailMsg struct {
	tabID int
	gen   int
	what  string
	err   error
}

// navigate runs a history move (back, forward) and captures what it lands
// on. The tab shows as loading from the keystroke until the capture lands
// — the same as load — which is what lets the app swallow the next P
// pressed before the first has answered (ux.md §6).
func (t *tab) navigate(what string, fn func(context.Context) error) tea.Cmd {
	t.gen++
	t.loading, t.errText = true, ""
	gen, id, ctx := t.gen, t.id, t.ctx
	return func() tea.Msg {
		if err := fn(ctx); err != nil {
			return navFailMsg{tabID: id, gen: gen, what: what, err: err}
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
	// fresh: another page than the one drawn, so the cursor starts over
	// (the first item, inside main when there is one — ux.md §6) and the
	// window is at the top, wherever that item sits. A redraw of the same
	// page keeps both.
	fresh := msg.url != "" && msg.url != t.lastVisit
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
	t.anchors, t.parents = msg.cap.Anchors, msg.cap.Parents
	t.relayout(width)
	if fresh {
		t.cursor, t.top = t.firstItem(), 0
		return
	}
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

// firstItem is where the cursor starts on a new page: the first item
// inside main, else the first item at all, else -1.
func (t *tab) firstItem() int {
	if len(t.lay.items) == 0 {
		return -1
	}
	at := 0
	for i, it := range t.lay.items {
		if it.node.Kind == ir.Landmark && it.node.Role == "main" {
			for j := i + 1; j < len(t.lay.items); j++ {
				if t.lay.items[j].node.Kind != ir.Landmark {
					at = j
					break
				}
			}
			break
		}
	}
	// Never on a skip link, or a block of them: what they offer is what
	// this function does.
	for at < len(t.lay.items)-1 && isSkipItem(t.lay.items[at].node) {
		at++
	}
	return at
}

// jumpToAnchor puts the cursor on the element a fragment names when it
// is an item, else on the first item inside it, else on the item it is
// inside of, and scrolls to it: what a link into the page does, in
// webu's terms. False when the page has no such id, or nothing to stop
// on either way — the click is then the page's to answer.
func (t *tab) jumpToAnchor(frag string, visible int) bool {
	target, ok := t.anchors[frag]
	if !ok || target == 0 {
		return false
	}
	// The element itself, or the first item inside it.
	for i, it := range t.lay.items {
		id := it.node.ID
		for hop := 0; id != 0 && hop < 256; hop++ {
			if id == target {
				t.cursor = i
				t.scrollToCursor(visible)
				return true
			}
			id = t.parents[id]
		}
	}
	// Else the item it is inside of: a span an item's text wraps.
	byID := map[cdp.BackendNodeID]int{}
	for i, it := range t.lay.items {
		if it.node.ID != 0 {
			byID[it.node.ID] = i
		}
	}
	for id, hop := t.parents[target], 0; id != 0 && hop < 256; id, hop = t.parents[id], hop+1 {
		if i, ok := byID[id]; ok {
			t.cursor = i
			t.scrollToCursor(visible)
			return true
		}
	}
	return false
}

func (t *tab) relayout(width int) {
	if t.root == nil {
		return
	}
	t.lay = renderWith(t.root, renderOpts{width: max(1, width), measure: t.measure, fold: t.fold})
	t.layW = width
}

// textWidth is how wide text flows in this tab's layout: the measure, or
// the panel when narrower.
func (t *tab) textWidth() int {
	if t.measure > 0 && t.measure < t.layW {
		return t.measure
	}
	return t.layW
}

// toggleFold opens or shuts the landmark under the cursor and keeps the
// cursor on it through the re-layout.
func (t *tab) toggleFold(width int) {
	n := t.current()
	if n == nil || (n.Kind != ir.Landmark && n.Kind != ir.Heading) {
		return
	}
	if t.fold == nil {
		t.fold = map[cdp.BackendNodeID]bool{}
	}
	t.fold[n.ID] = !t.lay.items[t.cursor].folded
	t.relayout(width)
	for i, it := range t.lay.items {
		if it.node == n {
			t.cursor = i
			break
		}
	}
}

// reveal opens every landmark shut around n, and every collapsed heading
// whose section holds it, so a jump to it (the Outline) has somewhere to
// land. Nothing happens when it is already drawn.
func (t *tab) reveal(n *ir.Node, width int) {
	if _, ok := t.lay.marks[n]; ok || t.root == nil {
		return
	}
	var path []*ir.Node
	var find func(x *ir.Node) bool
	find = func(x *ir.Node) bool {
		path = append(path, x)
		if x == n {
			return true
		}
		for _, c := range x.Children {
			if find(c) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	if !find(t.root) {
		return
	}
	if t.fold == nil {
		t.fold = map[cdp.BackendNodeID]bool{}
	}
	changed := false
	for _, a := range path[:len(path)-1] {
		if a.Kind == ir.Landmark {
			t.fold[a.ID] = false
			changed = true
		}
	}
	t.root.Walk(func(h *ir.Node) bool {
		if h.Kind == ir.Heading && t.fold[h.ID] && sectionHolds(t.root, h, n) {
			t.fold[h.ID] = false
			changed = true
		}
		return true
	})
	if changed {
		t.relayout(width)
	}
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

// moveItem walks the cursor by navigation key. The page is a grid of rows
// with items on them: j/k step to the nearest row that has one, landing on
// the item closest to the column the cursor was in; h/l walk the items of
// the row. Nothing wraps — the page has a top and a bottom (ux.md §3).
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
		t.cursor = t.rowStep(1)
	case "k", "up":
		t.cursor = t.rowStep(-1)
	case "l", "right":
		t.cursor = t.alongRow(1)
	case "h", "left":
		t.cursor = t.alongRow(-1)
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

// rowStep is the item on the nearest row of items in direction dir whose
// column is closest to the cursor's; the cursor itself when there is none.
func (t *tab) rowStep(dir int) int {
	cur := t.lay.items[t.cursor]
	target := -1
	for _, it := range t.lay.items {
		switch {
		case dir > 0 && it.first > cur.first && (target < 0 || it.first < target):
			target = it.first
		case dir < 0 && it.first < cur.first && (target < 0 || it.first > target):
			target = it.first
		}
	}
	if target < 0 {
		return t.cursor
	}
	best, bestD := -1, 0
	for i, it := range t.lay.items {
		if it.first != target {
			continue
		}
		d := it.col - cur.col
		if d < 0 {
			d = -d
		}
		if best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// alongRow is the next (dir 1) or previous item on the cursor's row, or
// the cursor itself at the row's end.
func (t *tab) alongRow(dir int) int {
	cur := t.lay.items[t.cursor]
	best := -1
	for i, it := range t.lay.items {
		if i == t.cursor || it.first != cur.first {
			continue
		}
		switch {
		case dir > 0 && it.col > cur.col && (best < 0 || it.col < t.lay.items[best].col):
			best = i
		case dir < 0 && it.col < cur.col && (best < 0 || it.col > t.lay.items[best].col):
			best = i
		}
	}
	if best < 0 {
		return t.cursor
	}
	return best
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
