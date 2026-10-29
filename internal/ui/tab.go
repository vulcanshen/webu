package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/reader"
)

// tab is one tab and everything webu knows about it: the page, laid out
// for the panel's width, and where the cursor is in it. Panel [1] lists
// them; panel [2] shows one.
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
	errText    string // a load that failed: DNS, connection, certificate
	// gutter is how many columns the line-number column takes off the
	// left of the panel; the layout was made at what it left (relayout).
	gutter int
	// canBack and canForward: whether the tab's history has a page before
	// and after this one — what dims Back and Forward (tdp M6).
	canBack, canForward bool
	// pending: restored from the last session but not loaded yet — it loads
	// when it is switched to (ux.md §6).
	pending bool

	root   *ir.Node
	lay    layout
	layW   int
	cursor int // index into lay.items; -1 when the page has none
	top    int // first page row on screen
	// gen guards loads: a result from before the latest one is thrown away
	// rather than drawn over the newer page.
	gen int
	// lastVisit is the URL last written to the history log for this tab,
	// so the same page is not logged again.
	lastVisit string
	// reloading is R in flight with a page on screen to keep.
	reloading bool
	// tabW is how many spaces a Tab in a code block is: input_tab_width
	// (tdp E3, reading content).
	tabW int
	// tick is the app's clock (App.Tick), for the settle timer a test can
	// move; tea.Tick when unset.
	tick func(time.Duration, func(time.Time) tea.Msg) tea.Cmd

	// The tab's pages come over net/http, through rc (reader.go,
	// docs/reader.md). doc is the page, lines it written out — its
	// content, or the whole of it while whole —, hist and histAt the tab's
	// own history, loadCancel the request on its way.
	rc         *reader.Client
	doc        *reader.Doc
	lines      []reader.Line
	whole      bool
	hist       []readerPlace
	histAt     int
	loadCancel context.CancelFunc
	// find is what / found on the page, nil when nothing is being found.
	find *readerFind
}

// working is a fetch in flight.
func (t *tab) working() bool { return t.loading }

// downloadMsg is a download starting, moving, or finishing (function.md
// §8); guid is how the events of one download are told from another's.
type downloadMsg struct {
	guid, name, url, path string
	received, total       int64
	begin, done, failed   bool
}

// newTab is a new tab: a reader's, its pages over net/http (reader.go).
func (m *AppModel) newTab() *tab { return m.newReaderTab() }

// waitEvent hands the next browser or page event to Update; the app
// re-issues it after every one, so the channel is always being read.
func waitEvent(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (t *tab) close() {
	if t.loadCancel != nil {
		t.loadCancel()
	}
	if t.cancel != nil {
		t.cancel()
	}
}

// load fetches url into the tab, as a new history entry (reader.go).
func (t *tab) load(url string) tea.Cmd { return t.readerLoad(url, -1) }

// reload is R: the page on screen loaded again. If that fails the page
// stays and a toast says so (tdp panel.15); a page
// that is not there to keep is the error in its place, as for any load.
func (t *tab) reload() tea.Cmd {
	cmd := t.load(t.url)
	t.reloading = t.root != nil
	return cmd
}

// actionErrMsg is something done that failed; what names it in the error
// popup's title: "<what> failed" (tdp note.6).
type actionErrMsg struct {
	what string
	err  error
}

// relayout lays the page out again at width (reader.go).
func (t *tab) relayout(width int) { t.relayoutReader(width) }

// rowRange is the stretch of the layout the panel may show: the page.
func (t *tab) rowRange() (int, int) { return 0, len(t.lay.rows) - 1 }

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

// current is the item under the cursor, or nil.
func (t *tab) current() *ir.Node {
	if t == nil {
		return nil
	}
	if t.cursor < 0 || t.cursor >= len(t.lay.items) {
		return nil
	}
	return t.lay.items[t.cursor].node
}

// lineText is what line n of what is showing says, for a list of them.
func (t *tab) lineText(n int) string {
	lo, hi := t.rowRange()
	row := lo + n - 1
	if n < 1 || row > hi {
		return ""
	}
	return strings.TrimSpace(t.lay.rows[row].plain())
}
