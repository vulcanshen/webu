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
	// loading says a fetch is in flight, which is what the spinner and
	// the dimmed page are about. navigating says that fetch is a
	// NAVIGATION — a load, a back, a forward — which is the only thing
	// that swallows the keys behind it (AppModel.busy).
	//
	// They were one field until pressing something on the page began to
	// say it was busy: the swallow is right for "P pressed three times
	// while the first back is still answering", and wrong for a click,
	// which is local and fast and whose settle should not leave the
	// keyboard dead for 300ms (2026-09-22).
	loading    bool
	navigating bool
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
	// pagetab is where the hand is when it is on the pagetab under the URL
	// rather than in the page (pagepanel.pagetabRow): 0 in the page, i+1 on
	// capsule i of lay.pagetab, pagetabMore on the "+N" that stands for the
	// capsules the width left out. Zero in the page, so a tab starts
	// there. The cursor keeps its place meanwhile: j comes back to it.
	pagetab int

	// secs is the page cut into sections (section.go) and shape says
	// whether that cut is used at all: a document gets the section list,
	// anything else stays the one sheet it always was. flat is the user
	// overruling that cut for this page — one sheet, the way every page
	// was drawn before; read is whether a section is open to the whole
	// panel, sec is which one, and secTop scrolls the list when it is
	// longer than the panel.
	// drill is the list item opened to the whole panel — Enter on one,
	// Esc back out. A list item is one thing, so it is one row until you
	// go into it (render.firstLine, user 2026-09-22).
	drill  cdp.BackendNodeID
	// drillTitle is that item's first line as it read when it was opened:
	// what the panel's header row says while you are inside it.
	drillTitle string
	secs   []section
	shape  pageShape
	flat   bool
	read   bool
	sec    int
	secTop int

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
	// blankUntil is how long a page that arrived empty is still counted
	// as on its way. page.Navigate waits for <body> to exist, which on an
	// application is the shell and nothing else — the content comes later,
	// through the mutation observer. Calling that "loaded" stopped the
	// spinner over a blank panel and left the user waiting with no sign
	// that anything was still coming (user, 2026-09-22). The deadline is
	// what keeps a page that is genuinely empty from spinning forever.
	blankUntil time.Time
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

// blankGrace is how long a page that keeps arriving empty is still shown
// as on its way. Long enough for an application to render, short enough
// that a page which really is blank says so rather than spinning.
const blankGrace = 8 * time.Second

// stillComing reports whether the capture just applied left the panel
// with nothing on it and the grace has not run out — in which case the
// page is not loaded, whatever <body> said.
func (t *tab) stillComing() bool {
	if t.errText != "" || t.root == nil {
		return false
	}
	if time.Now().After(t.blankUntil) {
		return false
	}
	for _, r := range t.lay.rows {
		if strings.TrimSpace(r.plain()) != "" {
			return false
		}
	}
	return true
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
	t.loading, t.navigating, t.pending, t.errText = true, true, false, ""
	t.blankUntil = time.Now().Add(blankGrace)
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
	t.loading, t.navigating, t.errText = true, true, ""
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
// press is act for something the USER asked for, which also says the page
// is busy until the recapture lands.
//
// The page answers a click in its own time and webu has to look again
// before it knows what changed. Until this said so, Enter on a disclosure
// sat silent for a settle — so the user pressed it again, and the second
// press shut what the first had opened (user, 2026-09-22). Saying it also
// swallows that second press (AppModel.busy).
//
// Not every action is one of these. webu reveals the cursor's node after
// every capture, on its own; if that said the page was busy it would
// schedule a settle, whose capture would reveal again, forever. Work webu
// does for itself is quiet — act.
func (t *tab) press(fn func(context.Context) error) tea.Cmd {
	t.loading = true
	return t.act(fn)
}

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
	t.loading, t.navigating = false, false
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
	wasBar, wasMore := t.onPagetab(), t.onMore()
	var wasBarID cdp.BackendNodeID
	wasBarKind, wasBarIdx := pagetabHeader, t.pagetabIndex()
	if wasBar && !wasMore && wasBarIdx >= 0 && wasBarIdx < len(t.lay.pagetab) {
		c := t.lay.pagetab[wasBarIdx]
		wasBarID, wasBarKind = c.nodes[0].ID, c.kind
	}
	t.root = ir.Build(msg.cap)
	t.anchors, t.parents = msg.cap.Anchors, msg.cap.Parents
	t.relayout(width)
	if fresh {
		// A different page: it opens where it declares it starts. A
		// document opens on its section list, everything else on the
		// page, its window at the first item rather than at row 0 — on a
		// page with no main those are not the same row, and the top of
		// such a page is its furniture (2026-09-22).
		t.flat, t.read, t.sec, t.secTop = false, false, 0, 0
		t.cursor, t.top = t.firstItem(), 0
		if t.cursor >= 0 {
			t.top = clamp(t.lay.items[t.cursor].first, 0, max(0, len(t.lay.rows)-1))
		}
		t.leavePagetab()
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
	if wasBar {
		// The hand stays on the pagetab through a redraw. By the node
		// behind the capsule first; failing that by its KIND, which is a
		// closed vocabulary and the same on every capture; failing that
		// by the slot it was in.
		//
		// The node alone was not enough: "other" holds whatever lies
		// outside main in no landmark, whose ids Chromium reassigns when
		// the page rebuilds that part of its DOM — so a hand parked on
		// the last capsule of a page that keeps settling fell back into
		// the page, which reads as Esc undoing itself (user, 2026-09-22).
		t.leavePagetab()
		switch {
		case wasMore && t.lay.fit < len(t.lay.pagetab):
			t.focusMore()
		case wasMore:
			// The "+N" is gone — everything fits now. The hand takes the
			// last capsule rather than the page.
			if n := len(t.lay.pagetab); n > 0 {
				t.focusPagetab(n - 1)
			}
		default:
			t.focusCapsuleLike(wasBarID, wasBarKind, wasBarIdx)
		}
	}
	t.scrollToCursor(0)
}

// focusCapsuleLike puts the hand back on the capsule it was on, by the
// node behind it, then by its kind, then by its place — and leaves it in
// the page only when the pagetab is now empty.
func (t *tab) focusCapsuleLike(id cdp.BackendNodeID, kind pagetabKind, at int) {
	if len(t.lay.pagetab) == 0 {
		return
	}
	if id != 0 {
		for i, c := range t.lay.pagetab {
			if c.nodes[0].ID == id {
				t.focusPagetab(i)
				return
			}
		}
	}
	for i, c := range t.lay.pagetab {
		if c.kind == kind {
			t.focusPagetab(i)
			return
		}
	}
	t.focusPagetab(clamp(at, 0, len(t.lay.pagetab)-1))
}

// firstItem is where the page starts: the first item inside main — main
// has no rule of its own, so by its row (marks) — else, on a page that
// declares no main, the first heading, which is where its reading
// begins (2026-09-22); else the first item at all. The chrome is on the
// pagetab, not in the page, so nothing of it is ever the start.
func (t *tab) firstItem() int {
	if len(t.lay.items) == 0 {
		return -1
	}
	if main := mainOf(t.root); main != nil {
		if row, ok := t.lay.marks[main]; ok {
			if at := t.itemAtOrAfter(row, true); at >= 0 {
				return at
			}
			if at := t.itemAtOrAfter(row, false); at >= 0 {
				return at
			}
		}
	}
	// No main: the page's own title row. The first heading of the
	// shallowest level there is — a page's h1 is what it is about, and
	// what precedes it is its furniture.
	best, level := -1, 0
	for i, it := range t.lay.items {
		if it.node.Kind != ir.Heading {
			continue
		}
		if l := it.node.Level; best < 0 || (l > 0 && l < level) {
			best, level = i, l
		}
	}
	if best >= 0 {
		return best
	}
	return 0
}

// itemAtOrAfter is the first item starting at or after row; content
// only skips the landmark rules on the way.
func (t *tab) itemAtOrAfter(row int, content bool) int {
	for i, it := range t.lay.items {
		if it.first >= row && (!content || it.node.Kind != ir.Landmark) {
			return i
		}
	}
	return -1
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
				t.landOn(i, visible)
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
			t.landOn(i, visible)
			return true
		}
	}
	return false
}

// landOn is where a link into the page arrives: the hand on the item,
// off the pagetab it may have been on, and the window scrolled so the
// item is the first row on screen — what following an anchor does in a
// browser. Keeping it merely visible (scrollToCursor) left the page
// looking unmoved when the target was already on screen (2026-09-22).
func (t *tab) landOn(i, visible int) {
	t.cursor = i
	t.leavePagetab()
	t.top = clamp(t.lay.items[i].first, 0, max(0, len(t.lay.rows)-1))
	t.scrollToCursor(visible)
}

func (t *tab) relayout(width int) {
	if t.root == nil {
		return
	}
	// Inside a list item the panel IS that item: its contents are the
	// page, so the cursor, the scrolling and the row window all work
	// against them without a second set of rules (user, 2026-09-22).
	root := t.root
	if t.drill != 0 {
		n := nodeByID(t.root, t.drill)
		if n == nil {
			t.drill, t.drillTitle = 0, ""
		} else {
			root = &ir.Node{Kind: ir.Document, Children: n.Children}
		}
	}
	t.lay = renderWith(root, renderOpts{width: max(1, width), measure: t.measure,
		fold: t.fold, drill: t.drill})
	t.layW = width
	// The pagetab may have lost the capsule the hand was on, or its "+N".
	if i := t.pagetabIndex(); i >= len(t.lay.pagetab) || (t.onMore() && t.lay.fit >= len(t.lay.pagetab)) {
		t.leavePagetab()
	}
	t.recut()
}

// recut cuts the fresh layout into sections and keeps the reader where it
// was: rows move when the width changes, the heading does not.
func (t *tab) recut() {
	was := t.sec
	var wasNode *ir.Node
	if t.sec < len(t.secs) {
		wasNode = t.secs[t.sec].node
	}
	t.secs = sectionsOf(t.root, t.lay, t.url)
	t.shape = shapeOf(t.secs)
	if t.shape != shapeDoc {
		t.read = false
	}
	// Keep the reader where it was. A re-capture builds a whole new tree,
	// so the heading is a new pointer and matching on that always fails:
	// it is found by the id Chromium gave it, the way the item cursor is
	// (apply), and failing that the list holds its place rather than
	// going back to the top. A page that keeps mutating settles over and
	// over, and every one of those was resetting the list.
	t.sec = clamp(was, 0, max(0, len(t.secs)-1))
	if wasNode != nil {
		for i, s := range t.secs {
			if s.node == wasNode || (wasNode.ID != 0 && s.node != nil && s.node.ID == wasNode.ID) {
				t.sec = i
				break
			}
		}
	}
	t.clampSecTop(0)
}

// clampSecTop keeps the list's cursor on screen; visible is how many rows
// the panel shows (0: unknown yet).
func (t *tab) clampSecTop(visible int) {
	if visible <= 0 {
		t.secTop = clamp(t.secTop, 0, max(0, len(t.secs)-1))
		return
	}
	if t.sec < t.secTop {
		t.secTop = t.sec
	}
	if t.sec >= t.secTop+visible {
		t.secTop = t.sec - visible + 1
	}
	t.secTop = clamp(t.secTop, 0, max(0, len(t.secs)-1))
}

// listing reports whether the panel is showing the section list rather than
// the page: a document that is not currently open at one of its sections.
func (t *tab) listing() bool {
	return t != nil && !t.drilled() && t.shape == shapeDoc && !t.flat && !t.read && len(t.secs) > 0
}

// rowRange is the stretch of the layout the panel may show: the open
// section while reading one, the whole page otherwise.
func (t *tab) rowRange() (int, int) {
	if t.read && t.sec < len(t.secs) {
		s := t.secs[t.sec]
		return s.body, min(s.last, len(t.lay.rows)-1)
	}
	return 0, len(t.lay.rows) - 1
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
	if t.onPagetab() || n == nil || (n.Kind != ir.Landmark && n.Kind != ir.Heading) {
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
	lo, hi := t.rowRange()
	if visible <= 0 || t.cursor < 0 || t.cursor >= len(t.lay.items) {
		t.top = clamp(t.top, lo, max(lo, hi))
		return
	}
	it := t.lay.items[t.cursor]
	if it.first < t.top {
		t.top = it.first
	}
	if it.last >= t.top+visible {
		t.top = it.last - visible + 1
	}
	t.top = clamp(t.top, lo, max(lo, hi))
}

// current is what the hand is on: the capsule's first landmark, on the
// pagetab; else the item under the cursor; nil on the pagetab's "+N", or on
// nothing.
func (t *tab) current() *ir.Node {
	if t == nil {
		return nil
	}
	if c := t.currentCapsule(); c != nil {
		return c.nodes[0]
	}
	if t.onPagetab() || t.cursor < 0 || t.cursor >= len(t.lay.items) {
		return nil
	}
	return t.lay.items[t.cursor].node
}

// currentCapsule is the capsule under the hand, or nil.
func (t *tab) currentCapsule() *capsule {
	if i := t.pagetabIndex(); i >= 0 && i < len(t.lay.pagetab) {
		return &t.lay.pagetab[i]
	}
	return nil
}

// currentTargets is what Enter's list holds for what the hand is on: a
// capsule's targets across its landmarks, else an item's inside; and
// whether they are a search's, whose every box is one.
func (t *tab) currentTargets() ([]entryTarget, bool) {
	if c := t.currentCapsule(); c != nil {
		return capsuleTargets(*c), c.kind == pagetabSearch
	}
	if n := t.current(); n != nil {
		return entryTargets(n), n.Role == "search"
	}
	return nil, false
}

// curFolded is whether the item under the cursor is a landmark drawn
// shut — never on the pagetab, whose capsules do not fold.
func (t *tab) curFolded() bool {
	return !t.onPagetab() && t.cursor >= 0 && t.cursor < len(t.lay.items) && t.lay.items[t.cursor].folded
}

// The hand can be on the pagetab under the URL instead of in the page
// (pagepanel.pagetabRow). tab.pagetab encodes where: 0 in the page, i+1 on
// capsule i, pagetabMore on the "+N" at the pagetab's end.
const pagetabMore = -2

func (t *tab) onPagetab() bool    { return t.pagetab != 0 }
func (t *tab) onMore() bool       { return t.pagetab == pagetabMore }
func (t *tab) leavePagetab()      { t.pagetab = 0 }
func (t *tab) focusPagetab(i int) { t.pagetab = i + 1 }
func (t *tab) focusMore()         { t.pagetab = pagetabMore }

// pagetabIndex is the capsule the hand is on, or -1.
func (t *tab) pagetabIndex() int {
	if t.pagetab > 0 {
		return t.pagetab - 1
	}
	return -1
}

// pagetabSlot is the slot the hand is on: a capsule index, pagetabMore, or -1
// when the hand is in the page.
func (t *tab) pagetabSlot() int {
	if t.pagetab == pagetabMore {
		return pagetabMore
	}
	return t.pagetabIndex()
}

// focusSlot puts the hand on a slot as pagetabSlots lists them.
func (t *tab) focusSlot(s int) {
	if s == pagetabMore {
		t.focusMore()
	} else {
		t.focusPagetab(s)
	}
}

// pagetabSlots is the pagetab left to right: the capsules the width holds, and
// pagetabMore last when it left some out. A capsule chosen from behind the
// "+N" (app openPagetabMore) takes the last slot while the hand is on it.
func (t *tab) pagetabSlots() []int {
	fit, n := t.lay.fit, len(t.lay.pagetab)
	slots := make([]int, 0, fit+1)
	for i := 0; i < fit; i++ {
		slots = append(slots, i)
	}
	if fit < n {
		if i := t.pagetabIndex(); i >= fit {
			if fit > 0 {
				slots[fit-1] = i
			} else {
				slots = append(slots, i)
			}
		}
		slots = append(slots, pagetabMore)
	}
	return slots
}

// enterPagetab puts the hand on the pagetab's first slot; false when the
// page has no chrome, which is what the menu row says instead of moving
// nothing (app pagetabItem).
func (t *tab) enterPagetab() bool {
	slots := t.pagetabSlots()
	if len(slots) == 0 {
		return false
	}
	t.focusSlot(slots[0])
	return true
}

// stepPagetab walks the hand along the pagetab's slots, wrapping at either end
// the way every menu of the family does.
func (t *tab) stepPagetab(k string) {
	slots := t.pagetabSlots()
	if len(slots) == 0 {
		t.leavePagetab()
		return
	}
	at := 0
	for i, s := range slots {
		if s == t.pagetabSlot() {
			at = i
		}
	}
	if k == "h" || k == "left" {
		at--
	} else {
		at++
	}
	t.focusSlot(slots[(at+len(slots))%len(slots)])
}

// capsuleOf is the index of the capsule n is in, or -1 when it is not
// chrome.
func (t *tab) capsuleOf(n *ir.Node) int {
	for i, c := range t.lay.pagetab {
		for _, x := range c.nodes {
			if x == n {
				return i
			}
		}
	}
	return -1
}

// moveItem walks the cursor by navigation key. The page is a grid of rows
// with items on them: j/k step to the nearest row that has one, landing on
// the item closest to the column the cursor was in; h/l walk the items of
// the row. Nothing wraps — the page has a top and a bottom (ux.md §3) —
// except that above the top is the pagetab under the URL, the page's chrome
// (pagepanel.pagetabRow): k from the top puts the hand on its first capsule,
// h/l walk the capsules and wrap the way a menu does, j comes back down
// to the item the hand left, and any other key comes down first and then
// does what it does.
func (t *tab) moveItem(k string, visible int) {
	if t.onPagetab() {
		switch k {
		case "h", "left", "l", "right":
			t.stepPagetab(k)
			return
		case "k", "up":
			return
		case "j", "down":
			t.leavePagetab()
			if t.cursor < 0 && len(t.lay.items) > 0 {
				t.cursor = t.firstItem()
			}
			t.scrollToCursor(visible)
			return
		}
		t.leavePagetab()
	}
	n := len(t.lay.items)
	lo, hi := t.rowRange()
	first, last := 0, n-1
	if t.read {
		first, last = t.sectionItems()
	}
	if n == 0 || first < 0 {
		// Nothing to stop on — an empty page, or a section that is all
		// prose: the keys scroll the text instead, and k at the top goes
		// up onto the pagetab.
		if (k == "k" || k == "up") && t.top <= lo && !t.read && t.enterPagetab() {
			return
		}
		t.top = clamp(moveScroll(t.top-lo, max(0, hi-lo+1-visible), k, visible)+lo, lo, max(lo, hi))
		return
	}
	half := max(1, visible/2)
	switch k {
	case "j", "down":
		t.cursor = t.rowStep(1)
	case "k", "up":
		// Off the top: up onto the pagetab, which is the row above the
		// page. Inside an open section it stops — a movement key moves
		// the cursor and does not change which screen you are on; Esc is
		// the one move up (user, 2026-09-22, the same objection as l
		// opening a section).
		if at := t.rowStep(-1); at != t.cursor && at >= first {
			t.cursor = at
		} else if !t.read && t.enterPagetab() {
			return
		}
	case "l", "right":
		t.cursor = t.alongRow(1)
	case "h", "left":
		t.cursor = t.alongRow(-1)
	case "d", "ctrl+d":
		t.cursor = t.itemFromRow(t.lay.items[t.cursor].first+half, 1)
	case "u", "ctrl+u":
		t.cursor = t.itemFromRow(t.lay.items[t.cursor].first-half, -1)
	case "gg":
		t.cursor = first
	case "G":
		t.cursor = last
	}
	t.cursor = clamp(t.cursor, first, last)
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
