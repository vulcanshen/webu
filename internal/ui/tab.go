package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
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
	// boxes is where the page laid each element out. It is what tells a
	// page's parts apart — beside, above, below (parts.splitParts).
	boxes map[cdp.BackendNodeID]ir.Box
	// viewport is the window it was laid out in: a page no taller than
	// that one has no parts at all (parts.splitParts).
	viewport ir.Box
	// gutter is how many columns the line-number column takes off the
	// left of the panel; the layout was made at what it left (relayout).
	gutter int
	// popup is the block the page put up after a press and that wants an
	// answer, by Chromium's id; the panel is that block until the page
	// takes it down (pagepopup.go). prevTop is every node of the tree at
	// the last capture, which is what an appearing block is told
	// against, and popupUntil how long after a press one can still be
	// its answer.
	popup      cdp.BackendNodeID
	prevTop    map[cdp.BackendNodeID]bool
	popupUntil time.Time
	// drillHead is the row of the drilled thing's first line, which the
	// panel draws as its header row rather than in the page — the way a
	// section's heading is the header while it is read; drillBody is
	// the first row after it (relayout). -1 and 0 when not drilled.
	drillHead, drillBody int
	// pending: restored from the last session but not loaded yet — it loads
	// when it is switched to (ux.md §6).
	pending bool

	root   *ir.Node
	lay    layout
	layW   int
	cursor int // index into lay.items; -1 when the page has none
	top    int // first page row on screen
	// parts is the page cut into its four (parts.splitParts) and at is
	// the one being shown — the panel below the pagetab IS that part,
	// with the whole of webu's content display applied to it (user,
	// 2026-09-22). A page opens on its body.
	parts []part
	at    partKind
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
	// drill is the path of things opened to the whole panel — Enter goes
	// in, Esc comes back out one level. A list item or an article is one
	// thing, so it is one row until you go into it (render.firstLine,
	// user 2026-09-22).
	//
	// It is a STACK because the nesting has no bound: a card holds a
	// list, whose items hold lists of their own, and a page may nest as
	// deep as it likes. A single "the thing I am inside" was a guess that
	// three levels was the most there could be (user).
	drill  []drillStep
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
	// print is treePrint of the capture on screen, and settlingUntil how
	// long the next one that differs from it still counts as the page
	// arriving rather than the page living.
	print         uint64
	settlingUntil time.Time
	// changed says the last capture differed from the one before it.
	changed bool
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
	// ua and uaMeta are what the tab says it is (browser.Browser.identify),
	// applied when it is prepared.
	ua     string
	uaMeta *emulation.UserAgentMetadata
	// fold is the user's word on which landmarks are open or shut, by node
	// id, which survives a recapture; measure is the text width cap.
	fold    map[cdp.BackendNodeID]bool
	measure int
}

// blankGrace is how long a page that keeps arriving empty is still shown
// as on its way. Long enough for an application to render, short enough
// that a page which really is blank says so rather than spinning.
const blankGrace = 8 * time.Second

// settleGrace is how long after the user asks for something the page
// answering differently still means "still coming" rather than "alive".
// The same eight seconds: they are the same judgement about how long a
// page is allowed to take before a terminal stops waiting on it.
const settleGrace = 8 * time.Second

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

// settling reports whether the page is still being built: the capture
// just applied is not the one before it, and the grace has not run out.
//
// A page does not announce that it has finished. Chromium's load event
// fires on the shell, and everything an application draws arrives after
// it; page.Navigate waiting for <body> says only that there is a
// document. What says a page is still coming is that it keeps answering
// differently — so webu looks again, and goes on saying it is loading,
// until two looks agree (user, 2026-09-23: "切換頁面，都不知道是切換了
// 還是卡住了").
//
// The grace is what stops a page that never settles — a clock, a ticker,
// an animation a terminal cannot show anyway — from spinning for as long
// as it is open.
func (t *tab) settling() bool {
	return t.errText == "" && t.changed && time.Now().Before(t.settlingUntil)
}

// working is the page not being finished, by either measure: a fetch is
// in flight, or the page is still being built under one that landed.
//
// Wider than loading on purpose. loading DIMS the page, because the page
// on screen is the one being left; a page still filling in is the page
// you are on and its content is real — dimming it would say the opposite.
// What both deserve is the spinner: something is happening, and the
// terminal is not stuck.
func (t *tab) working() bool { return t.loading || t.settling() }

// drillStep is one level of that path: the node, by the id Chromium gave
// it, and its first line as it read when it was opened — what the header
// row says while you are inside it.
type drillStep struct {
	id    cdp.BackendNodeID
	title string
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
		fold: map[cdp.BackendNodeID]bool{}, measure: m.cfg.TextWidth(),
		ua: m.browser.UserAgent, uaMeta: m.browser.UAMeta}
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
	ua, meta := t.ua, t.uaMeta
	return func() tea.Msg {
		if err := chromedp.Run(ctx); err != nil {
			return pageMsg{tabID: id, gen: gen, err: fmt.Errorf("attach: %w", err)}
		}
		if err := page.Prepare(ctx, ua, meta); err != nil {
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
	t.settlingUntil = time.Now().Add(settleGrace)
	t.url = url
	gen, id, ctx := t.gen, t.id, t.ctx
	ua, meta := t.ua, t.uaMeta
	prepare := !t.prepared
	t.prepared = true
	return func() tea.Msg {
		if prepare {
			if err := page.Prepare(ctx, ua, meta); err != nil {
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
	t.blankUntil = time.Now().Add(blankGrace)
	t.settlingUntil = time.Now().Add(settleGrace)
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
	// And for as long as the page keeps changing afterwards: opening a
	// list item on an application takes as long as a small navigation
	// does, and said nothing while it did (user, 2026-09-22).
	t.settlingUntil = time.Now().Add(settleGrace)
	// And whatever the page puts up in answer is a popup (popup.go).
	t.popupUntil = time.Now().Add(popupGrace)
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
	// The hand on the pagetab is on a KIND of part, and the kinds are a
	// closed set: they survive a recapture even when every node behind
	// them is new (2026-09-22).
	wasBar, wasBarKind := t.onPagetab(), partMain
	if i := t.pagetabIndex(); wasBar && i >= 0 && i < len(t.parts) {
		wasBarKind = t.parts[i].kind
	}
	t.root = ir.Build(msg.cap)
	was := t.print
	t.print = treePrint(t.root)
	t.changed = t.print != was
	t.anchors, t.parents, t.boxes = msg.cap.Anchors, msg.cap.Parents, msg.cap.Boxes
	t.viewport = msg.cap.Viewport
	popped := t.noticePopup(fresh)
	t.relayout(width)
	if popped {
		// Into the popup, or back out of it: the page on screen is
		// another one, and the cursor starts where it starts.
		t.drill = nil
		t.relayout(width)
		t.cursor, t.top = t.firstItem(), 0
		t.leavePagetab()
		t.scrollToCursor(0)
		return
	}
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
		t.focusPagetab(t.partIndex(wasBarKind))
	}
	t.scrollToCursor(0)
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
	if t.drilled() {
		// Inside a thing: its body, past the header row — unless the
		// header holds the only item there is.
		if at := t.itemAtOrAfter(t.drillBody, false); at >= 0 {
			return at
		}
		return 0
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
	// The panel shows one PART of the page, and inside a list item it
	// shows that item: either way its contents are the page, so the
	// cursor, the scrolling and the row window all work against them
	// without a second set of rules (user, 2026-09-22).
	t.parts = splitParts(sansPopup(t.root, t.popup), t.boxes, t.viewport)
	base := t.root
	if p := t.popupNode(); p != nil {
		// A popup is the panel until it is answered (popup.go).
		base = &ir.Node{Kind: ir.Document, Children: []*ir.Node{p}}
	} else if p := t.activePart(); p != nil {
		base = &ir.Node{Kind: ir.Document, Children: p.nodes}
	}
	root := base
	inside := t.drillNode(base)
	if inside != base {
		root = &ir.Node{Kind: ir.Document, Children: inside.Children}
	}
	// The page is drawn behind a column of line numbers, so the text is
	// laid out into what that column leaves. How wide it is depends on
	// how many lines there are, which depends on how wide the text is —
	// so it is measured: laid out once at the width the last page
	// needed, and again when that turns out to be the wrong number of
	// digits. Twice at most, and only ever at a power of ten.
	draw := func(g int) layout {
		return renderWith(root, renderOpts{width: max(1, width-g), measure: t.measure,
			fold: t.fold, drill: inside.ID})
	}
	g := t.gutter
	lay := draw(g)
	if g2 := lineNumW(len(lay.rows)); g2 != g {
		g, lay = g2, draw(g2)
	}
	t.lay, t.gutter = lay, g
	t.layW = width
	t.drillHead, t.drillBody = -1, 0
	if inside != base {
		// Inside a thing its first line is the header row (pagepanel
		// insideRow): the first row with anything on it, then the blank
		// under it, are not the page. Printing the line under a header
		// that said the same thing was every drill saying its name
		// twice (measured, 2026-09-23).
		for i, r := range lay.rows {
			if strings.TrimSpace(r.plain()) != "" {
				t.drillHead = i
				break
			}
		}
		if t.drillHead >= 0 {
			at := t.drillHead + 1
			for at < len(lay.rows) && strings.TrimSpace(lay.rows[at].plain()) == "" {
				at++
			}
			t.drillBody = at
		}
	}
	// The page may have lost the part the hand was on.
	if i := t.pagetabIndex(); i >= len(t.parts) {
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
	if t.drilled() {
		return t.drillBody, len(t.lay.rows) - 1
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

// current is the item under the cursor, or nil — including while the
// hand is on the pagetab, where what it is on is a part, not an item.
func (t *tab) current() *ir.Node {
	if t == nil {
		return nil
	}
	if t.onPagetab() || t.cursor < 0 || t.cursor >= len(t.lay.items) {
		return nil
	}
	return t.lay.items[t.cursor].node
}

// currentTargets is what Enter's list holds for the item under the
// cursor, and whether it is a search's, whose every box is one.
func (t *tab) currentTargets() ([]entryTarget, bool) {
	if n := t.current(); n != nil {
		return entryTargets(n), n.Role == "search"
	}
	return nil, false
}

// curFolded is whether the item under the cursor is a landmark drawn
// shut — never on the pagetab, whose tabs do not fold.
func (t *tab) curFolded() bool {
	return !t.onPagetab() && t.cursor >= 0 && t.cursor < len(t.lay.items) && t.lay.items[t.cursor].folded
}

// The hand can be on the pagetab under the URL instead of in the page
// (pagepanel.pagetabRow). tab.pagetab encodes where: 0 in the page, i+1
// with the hand on part i.
func (t *tab) onPagetab() bool    { return t.pagetab != 0 }
func (t *tab) leavePagetab()      { t.pagetab = 0 }
func (t *tab) focusPagetab(i int) { t.pagetab = i + 1 }

// pagetabIndex is the part the hand is on, or -1.
func (t *tab) pagetabIndex() int {
	if t.pagetab > 0 {
		return t.pagetab - 1
	}
	return -1
}

// enterPagetab puts the hand on the part being shown, which is where the
// user's eye already is. False when the page has only the one part: there
// is nothing to move between (app pagetabItem says so instead).
func (t *tab) enterPagetab() bool {
	if len(t.parts) < 2 || t.popupNode() != nil {
		return false
	}
	t.focusPagetab(t.partIndex(t.at))
	return true
}

// stepPagetab walks the hand along the parts, wrapping at either end the
// way every menu of the family does — and the panel walks with it: the
// part the hand reaches is the part on screen, at once (user,
// 2026-09-23). Moving the hand and then pressing Enter to send it was
// two steps for one decision; Enter is now the confirmation, which is to
// take the hand back down (app.enterItem).
func (t *tab) stepPagetab(k string, visible int) {
	n := len(t.parts)
	if n == 0 {
		t.leavePagetab()
		return
	}
	at := t.pagetabIndex()
	if k == "h" || k == "left" {
		at--
	} else {
		at++
	}
	at = (at + n) % n
	t.showPart(t.parts[at].kind, t.layW, visible)
	t.focusPagetab(at)
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
			t.stepPagetab(k, visible)
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

// treePrint is a cheap summary of a captured page, so the next capture
// can be told from this one without keeping either.
//
// Structure, roles and text go in; node ids do not. A page that rebuilds
// a block hands it back under fresh ids without anything the reader can
// see having changed (render TestTheHandStaysOnThePagetab), and a page
// that never prints the same twice would spin out its whole grace.
func treePrint(n *ir.Node) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	eat := func(b byte) { h = (h ^ uint64(b)) * prime }
	feed := func(s string) {
		for i := 0; i < len(s); i++ {
			eat(s[i])
		}
		eat(0)
	}
	var walk func(*ir.Node)
	walk = func(x *ir.Node) {
		if x == nil {
			return
		}
		eat(byte(x.Kind))
		feed(x.Role)
		feed(x.Name)
		feed(x.Value)
		for _, c := range x.Children {
			walk(c)
		}
		eat('}') // where a node ends, so nesting counts
	}
	walk(n)
	return h
}

// lineCount is how many lines the panel is showing: the section list,
// or the page, or the one section open in it, or the one list item
// drilled into. The number in the gutter counts from the top of that,
// so this is what it counts to (pagepanel.pageRows, sectionRows).
func (t *tab) lineCount() int {
	if t.listing() {
		return len(t.secs)
	}
	lo, hi := t.rowRange()
	return max(0, hi-lo+1)
}

// lineText is what line n of what is showing says, for a list of them.
func (t *tab) lineText(n int) string {
	if t.listing() {
		if n < 1 || n > len(t.secs) {
			return ""
		}
		s := t.secs[n-1]
		return strings.Repeat("  ", max(0, s.depth-1)) + s.title
	}
	lo, hi := t.rowRange()
	row := lo + n - 1
	if n < 1 || row > hi {
		return ""
	}
	return strings.TrimSpace(t.lay.rows[row].plain())
}

// goToLine puts the reader on a line of what is on screen, one-based.
// On the section list that is the list's cursor; on a page it is the
// window, with the item cursor on the nearest item at or after it — a
// line with nothing to stop on is still somewhere to look, so the window
// moves whether or not the cursor can follow.
func (t *tab) goToLine(n, visible int) bool {
	if t.listing() {
		if n < 1 || n > len(t.secs) {
			return false
		}
		t.sec = n - 1
		t.clampSecTop(visible)
		return true
	}
	lo, hi := t.rowRange()
	row := lo + n - 1
	if n < 1 || row > hi {
		return false
	}
	t.landRow(row, visible)
	return true
}

// landRow is where a jump into the page arrives: the window scrolled so
// the row is the first on screen, the hand off the pagetab, and the
// cursor on the first item at or after the row when there is one.
func (t *tab) landRow(row, visible int) {
	lo, hi := t.rowRange()
	row = clamp(row, lo, hi)
	t.leavePagetab()
	t.top = clamp(row, lo, max(lo, hi-visible+1))
	if at := t.itemAtOrAfter(row, true); at >= 0 && t.lay.items[at].first <= hi {
		t.cursor = at
	}
}

// rowOf is the row a node starts on in the current layout, or -1: an
// item's first row when the node is one or holds one; the row the
// renderer marked for it (a heading, a paragraph, a table); else the
// item the node is INSIDE — a table's cell is the item, and a link in
// the cell lands on the cell.
func (t *tab) rowOf(n *ir.Node) int {
	if n == nil {
		return -1
	}
	for _, it := range t.lay.items {
		if it.node == n || holds(n, it.node) {
			return it.first
		}
	}
	if at, ok := t.lay.marks[n]; ok {
		return at
	}
	for _, it := range t.lay.items {
		if holds(it.node, n) {
			return it.first
		}
	}
	return -1
}

// chainTo is every node from the root down to n, n last; nil when the
// tree does not hold it.
func chainTo(root, n *ir.Node) []*ir.Node {
	var chain []*ir.Node
	var find func(x *ir.Node) bool
	find = func(x *ir.Node) bool {
		chain = append(chain, x)
		if x == n {
			return true
		}
		for _, c := range x.Children {
			if find(c) {
				return true
			}
		}
		chain = chain[:len(chain)-1]
		return false
	}
	if root == nil || !find(root) {
		return nil
	}
	return chain
}

// chainOf is a hit's node as the CURRENT tree has it, with its
// ancestors: by Chromium's id when it has one, since a recapture hands
// back the same page under new pointers; by its trail of child indexes
// when it has none, or the id is gone — with the kind checked, so a page
// that changed shape under the finder fails rather than lands somewhere
// else. Nil when it cannot be found.
func (t *tab) chainOf(h hit) []*ir.Node {
	if t.root == nil || h.node == nil {
		return nil
	}
	if h.node.ID != 0 {
		if n := nodeByID(t.root, h.node.ID); n != nil {
			return chainTo(t.root, n)
		}
	}
	chain := []*ir.Node{t.root}
	n := t.root
	for _, i := range h.trail {
		if i < 0 || i >= len(n.Children) {
			return nil
		}
		n = n.Children[i]
		chain = append(chain, n)
	}
	if n.Kind != h.node.Kind {
		return nil
	}
	return chain
}
