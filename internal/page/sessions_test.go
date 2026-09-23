package page

import (
	"context"
	"testing"

	"github.com/vulcanshen/webu/internal/ir"
)

// An id carried at a frame's offset resolves to that frame's session and
// the id it knows; the page's own ids resolve to the tab.
func TestASessionResolvesItsIDs(t *testing.T) {
	tab := context.Background()
	fctx := context.WithValue(tab, "frame", true)
	s := NewSessions(tab)
	s.ctxs["F1"], s.slots["F1"] = fctx, 2
	if ctx, id := s.resolve(tab, 7); ctx != tab || id != 7 {
		t.Errorf("the page's own: %v %d", ctx, id)
	}
	if ctx, id := s.resolve(tab, 2*ir.CarriedFrom+7); ctx != fctx || id != 7 {
		t.Errorf("the frame's: %v %d", ctx, id)
	}
	if ctx, id := s.resolve(tab, 3*ir.CarriedFrom+7); ctx != tab {
		t.Errorf("a slot nobody holds falls back to the tab: %v %d", ctx, id)
	}
}
