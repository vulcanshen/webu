package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/store"
)

// A page does not announce that it has finished, so webu says it is
// still coming for as long as it keeps answering differently — and stops
// the moment two looks agree (user, 2026-09-23: switching pages gave no
// sign whether it had switched or hung).
func TestAPageIsComingWhileItKeepsChanging(t *testing.T) {
	page := func(lines ...string) pageMsg {
		nodes := []*accessibility.Node{axNode(1, "RootWebArea", "Page", 1)}
		for i, s := range lines {
			nodes[0].ChildIDs = append(nodes[0].ChildIDs, accessibility.NodeID(itoa(10+i)))
			nodes = append(nodes, axNode(10+i, "paragraph", "", 10+i, 100+i),
				axNode(100+i, "StaticText", s, 100+i))
		}
		return pageMsg{url: "https://x.test/", title: "Page", cap: ir.Capture{Nodes: nodes}}
	}
	tb := &tab{cursor: -1, settlingUntil: time.Now().Add(time.Minute)}

	tb.apply(page("one"), 60)
	if !tb.settling() {
		t.Error("the first look has nothing to agree with: still coming")
	}
	tb.apply(page("one"), 60)
	if tb.settling() {
		t.Error("two looks agree: the page has arrived")
	}
	tb.apply(page("one", "two"), 60)
	if !tb.settling() {
		t.Error("the page grew under webu: still coming")
	}

	// The grace is what stops a page that never settles — a clock, a
	// ticker — from spinning for as long as it is open.
	tb.settlingUntil = time.Now().Add(-time.Second)
	tb.apply(page("one", "three"), 60)
	if tb.settling() {
		t.Error("past the grace a page that keeps moving is alive, not loading")
	}

	// A page rebuilt under fresh ids has not changed: what the reader
	// sees is the print, not Chromium's numbering.
	tb.settlingUntil = time.Now().Add(time.Minute)
	tb.apply(page("a", "b"), 60)
	same := page("a", "b")
	for _, n := range same.cap.Nodes {
		n.BackendDOMNodeID += 500
	}
	tb.apply(same, 60)
	if tb.settling() {
		t.Error("new ids for the same page are not a change")
	}
}

// And the spinner turns for all of it, while the page below stays
// readable: loading dims because the page on screen is the one being
// left; a page still filling in is the page you are on.
func TestTheSpinnerTurnsWhileThePageFillsIn(t *testing.T) {
	b := hookBrowser(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><title>Late</title><h1>Shell</h1><div id="x"></div>
<script>
setTimeout(() => { document.querySelector("#x").innerHTML = "<p>first batch</p>"; }, 400);
setTimeout(() => { document.querySelector("#x").innerHTML += "<p>second batch</p>"; }, 1200);
</script>`))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})

	// The shell arrives first and webu must not call that arrival.
	d.until("the shell", func() bool { return strings.Contains(dumpLayout(d.page().lay), "Shell") })
	if v := d.m.View(); !strings.Contains(v, glyphLive) {
		t.Errorf("the spinner should turn while the page is still filling in:\n%s", v)
	}

	d.until("everything", func() bool { return strings.Contains(dumpLayout(d.page().lay), "second batch") })
	d.until("at rest", func() bool { return !d.page().working() })
	if v := d.m.View(); !strings.Contains(v, glyphWeb) {
		t.Errorf("once it has settled the glyph is the web one:\n%s", v)
	}
	if !strings.Contains(d.m.View(), "second batch") {
		t.Error("and everything the page drew is on screen")
	}
}
