package page

import (
	"testing"

	cdppage "github.com/chromedp/cdproto/page"
)

// Back and Forward have somewhere to go only where step would go: the
// about:blank every tab starts on is not a page (tdp M6: what dims them).
func TestHistoryHasAnEntryToGoTo(t *testing.T) {
	entries := []*cdppage.NavigationEntry{{URL: "about:blank"}, {URL: "https://a/"}, {URL: "https://b/"}}
	for _, tc := range []struct {
		i    int
		want bool
	}{{-1, false}, {0, false}, {1, true}, {2, true}, {3, false}} {
		if got := hasEntry(entries, tc.i); got != tc.want {
			t.Errorf("entry %d: %v, want %v", tc.i, got, tc.want)
		}
	}
}
