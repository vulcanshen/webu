package ui

import (
	"errors"
	"testing"
)

// A look again at the page on screen that fails — a settle, a refresh, or
// R — keeps the page and leaves the app a toast to say (tdp
// panel.15); a new address that fails is the error in its place.
func TestFailedReloadKeepsThePage(t *testing.T) {
	root := doc(hd(1, "Kept"), para(text("Still here.")))
	tb := &tab{id: 1, url: "https://example.com/kept", root: root, cursor: -1}
	tb.relayout(80)

	tb.reload()
	tb.apply(pageMsg{tabID: 1, gen: tb.gen, url: tb.url, err: errors.New("navigate: net::ERR_INTERNET_DISCONNECTED")}, 80)
	if tb.root == nil || tb.errText != "" || tb.refreshErr == "" {
		t.Errorf("R that failed: root kept %v, errText %q, refreshErr %q", tb.root != nil, tb.errText, tb.refreshErr)
	}
	if tb.loading || tb.navigating {
		t.Errorf("the tab still says it is loading: %v %v", tb.loading, tb.navigating)
	}

	tb.refreshErr = ""
	tb.load("https://example.com/elsewhere")
	tb.apply(pageMsg{tabID: 1, gen: tb.gen, url: tb.url, err: errors.New("navigate: net::ERR_NAME_NOT_RESOLVED")}, 80)
	if tb.root != nil || tb.errText == "" {
		t.Errorf("a new address that failed: root %v, errText %q", tb.root != nil, tb.errText)
	}
}
