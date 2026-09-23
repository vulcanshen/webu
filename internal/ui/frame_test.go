package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/store"
)

// A frame from another site is another process and another target: Enter
// on its row enters it all the same, read through a session of webu's
// own on that target, and what is pressed inside it is pressed there
// (2026-09-23). A file:// page holding an http:// frame is such a pair.
func TestACrossSiteFrameIsEntered(t *testing.T) {
	b := hookBrowser(t)
	// One server, two sites: 127.0.0.1 and localhost are different sites
	// to the browser, so the frame inside the frame is another process
	// again.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/deep" {
			w.Write([]byte(`<title>Deep</title><p>deepest text</p>`))
			return
		}
		w.Write([]byte(`<title>Inner</title><h2>Inside another site</h2><p>remote text</p>` +
			`<button onclick="this.textContent='Pressed'">Press me</button>` +
			`<iframe src="` + strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + `/deep" title="Deeper" width="300" height="100"></iframe>`))
	}))
	defer srv.Close()
	outer := filepath.Join(t.TempDir(), "outer.html")
	if err := os.WriteFile(outer, []byte(`<title>Outer</title><h1>Outer page</h1>`+
		`<iframe src="`+srv.URL+`/frame" title="The embed" width="400" height="200"></iframe><p>after the frame</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := startAt(t, b, "file://"+outer, store.Config{})
	d.until("outer", d.loaded("Outer"))
	p := d.page()
	if v := dumpLayout(p.lay); strings.Contains(v, "Inside another site") || !strings.Contains(v, "The embed") {
		t.Fatalf("the frame is one row, shut:\n%s", v)
	}
	d.cursorOn(ir.Media, "The embed")
	if n := p.current(); n.Frame == "" {
		t.Fatalf("the row knows its frame, another process or not: %+v", n)
	}
	d.key("enter")
	d.until("inside", func() bool { return p.drilled() && strings.Contains(dumpLayout(p.lay), "Inside another site") })
	if v := dumpLayout(p.lay); strings.Contains(v, "after the frame") {
		t.Errorf("inside the frame the page outside is not shown:\n%s", v)
	}
	// Pressed where it lives: the button is that process's node — and
	// placed where it is on the page: inside the frame's own box.
	d.cursorOn(ir.Button, "Press me")
	if btn, own := p.current(), nodeByName(p.root, ir.Media, "The embed"); own == nil {
		t.Error("the frame's row is still on the page")
	} else if b, ok := p.boxes[btn.ID]; !ok || b.X < p.boxes[own.ID].X || b.Y < p.boxes[own.ID].Y {
		t.Errorf("the button's box is inside its frame's: %+v in %+v", b, p.boxes[own.ID])
	}
	d.key("enter")
	d.until("pressed", func() bool { return strings.Contains(dumpLayout(p.lay), "Pressed") })
	// A frame inside the frame, from a third site: entered the same way.
	d.cursorOn(ir.Media, "Deeper")
	d.key("enter")
	d.until("deeper", func() bool { return strings.Contains(dumpLayout(p.lay), "deepest text") })
	d.key("esc")
	d.key("esc")
	if p.drilled() || !strings.Contains(dumpLayout(p.lay), "after the frame") {
		t.Errorf("Esc twice is back out to the page:\n%s", dumpLayout(p.lay))
	}
}

// nodeByName finds the first node of a kind with a name, or nil.
func nodeByName(root *ir.Node, kind ir.Kind, name string) *ir.Node {
	var found *ir.Node
	root.Walk(func(n *ir.Node) bool {
		if found == nil && n.Kind == kind && n.Name == name {
			found = n
		}
		return found == nil
	})
	return found
}
