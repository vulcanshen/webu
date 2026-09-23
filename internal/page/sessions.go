package page

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/vulcanshen/webu/internal/ir"
)

// A frame from another site is another process, and another target: the
// tab's session sees its <iframe> element and nothing inside — the frame
// tree does not even list it. To read it webu attaches a session of its
// own to that target (chromedp.WithTargetID, which the browser allows for
// an iframe target) and asks there: the AX tree, the snapshot, a click at
// the frame's own coordinates (measured, 2026-09-23).
//
// Its node ids are that process's, and start at 1 again — the same
// numbers the page has. So every id from such a frame is carried with an
// offset, one slot per frame (ir.Capture.Base), and an action addressed
// to one is resolved back to the frame's session and the id it knows (on).

// Sessions is a tab's sessions on its cross-site frames. Attached on
// first use; forgotten when one answers nothing, since a frame that moved
// to another site is a new target under the same frame id.
type Sessions struct {
	mu      sync.Mutex
	tab     context.Context
	ctxs    map[cdp.FrameID]context.Context
	cancels map[cdp.FrameID]context.CancelFunc
	slots   map[cdp.FrameID]int64
	next    int64
}

func NewSessions(tab context.Context) *Sessions {
	s := &Sessions{ctxs: map[cdp.FrameID]context.Context{},
		cancels: map[cdp.FrameID]context.CancelFunc{}, slots: map[cdp.FrameID]int64{}}
	// A frame's session derives from a context that carries these
	// sessions, so a frame inside the frame is reached the same way.
	s.tab = WithSessions(tab, s)
	return s
}

type sessionsKey struct{}

// WithSessions hands a tab's Sessions to every call made with the
// context: Capture reads other sites' frames through them, and the
// actions find the session an id belongs to.
func WithSessions(ctx context.Context, s *Sessions) context.Context {
	return context.WithValue(ctx, sessionsKey{}, s)
}

func sessionsFrom(ctx context.Context) *Sessions {
	s, _ := ctx.Value(sessionsKey{}).(*Sessions)
	return s
}

// frame is the session on frame id — attached now, when there is none
// yet — and the offset its ids are carried at. Attaching is bounded: a
// target that never answers is not a frame webu can enter.
func (s *Sessions) frame(id cdp.FrameID) (context.Context, cdp.BackendNodeID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx, ok := s.ctxs[id]; ok {
		return ctx, cdp.BackendNodeID(s.slots[id]) * ir.CarriedFrom, nil
	}
	ctx, cancel := chromedp.NewContext(s.tab, chromedp.WithTargetID(target.ID(id)))
	done := make(chan error, 1)
	go func() { done <- chromedp.Run(ctx) }()
	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		err = context.DeadlineExceeded
	}
	if err != nil {
		cancel()
		return nil, 0, err
	}
	s.next++
	s.ctxs[id], s.cancels[id], s.slots[id] = ctx, cancel, s.next
	return ctx, cdp.BackendNodeID(s.next) * ir.CarriedFrom, nil
}

// forget drops a frame's session.
func (s *Sessions) forget(id cdp.FrameID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.cancels[id]; ok {
		cancel()
	}
	delete(s.ctxs, id)
	delete(s.cancels, id)
	delete(s.slots, id)
}

// resolve turns an id as the IR carries it into the session it belongs
// to and the id that session knows: the tab's own for the page's ids.
func (s *Sessions) resolve(ctx context.Context, id cdp.BackendNodeID) (context.Context, cdp.BackendNodeID) {
	slot := int64(id / ir.CarriedFrom)
	if slot == 0 {
		return ctx, id
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for fid, sl := range s.slots {
		if sl == slot {
			return s.ctxs[fid], id % ir.CarriedFrom
		}
	}
	return ctx, id
}

// on runs fn against the session the node belongs to, with the id that
// session knows. Every action addressed to a node goes through here.
func on(ctx context.Context, id cdp.BackendNodeID, fn func(context.Context, cdp.BackendNodeID) error) error {
	if s := sessionsFrom(ctx); s != nil {
		ctx, id = s.resolve(ctx, id)
	}
	return run(ctx, func(ctx context.Context) error { return fn(ctx, id) })
}
