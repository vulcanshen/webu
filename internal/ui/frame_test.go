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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<title>Inner</title><h2>Inside another site</h2><p>remote text</p>` +
			`<button onclick="this.textContent='Pressed'">Press me</button>`))
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
	// Pressed where it lives: the button is that process's node.
	d.cursorOn(ir.Button, "Press me")
	d.key("enter")
	d.until("pressed", func() bool { return strings.Contains(dumpLayout(p.lay), "Pressed") })
	d.key("esc")
	if p.drilled() || !strings.Contains(dumpLayout(p.lay), "after the frame") {
		t.Errorf("Esc is back out to the page:\n%s", dumpLayout(p.lay))
	}
}
