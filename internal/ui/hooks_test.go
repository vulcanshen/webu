package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/store"
)

// The hooks (function.md §5) against the pinned Chromium: a window the
// page opens, the three dialogs, a certificate Chromium refuses, a download.

func hookBrowser(t *testing.T) *browser.Browser {
	t.Helper()
	t.Setenv("WEBU_CONFIG", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}

func startAt(t *testing.T, b *browser.Browser, url string, cfg store.Config) *driver {
	t.Helper()
	d := newDriver(t, New(b, url).WithStore(nil, cfg, nil))
	t.Cleanup(d.m.Close)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	return d
}

func TestNewWindowBecomesATab(t *testing.T) {
	b := hookBrowser(t)
	abs, _ := filepath.Abs("testdata/nav.html")
	d := startAt(t, b, "file://"+abs, store.Config{})
	d.until("page A", d.loaded("Page A"))
	d.cursorOn(ir.Link, "B in a new tab")
	d.key("enter")
	d.until("a second tab showing page B", func() bool {
		return len(d.m.tabs) == 2 && d.m.shown == 1 && d.m.tabs[1].title == "Page B" && !d.m.tabs[1].loading
	})
	if d.m.cur2 != 1 || d.m.focus != panel3 {
		t.Errorf("the new tab is where the cursor and keyboard went: cur2 %d focus %d", d.m.cur2, d.m.focus)
	}
	if !strings.Contains(d.m.View(), "You made it.") {
		t.Errorf("page B not drawn:\n%s", d.m.View())
	}
}

func TestDialogsAreAnswered(t *testing.T) {
	b := hookBrowser(t)
	abs, _ := filepath.Abs("testdata/nav.html")
	d := startAt(t, b, "file://"+abs, store.Config{})
	d.until("page A", d.loaded("Page A"))

	d.cursorOn(ir.Button, "Alert")
	d.key("enter")
	d.until("alert shown", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmDialog })
	if !strings.Contains(strings.Join(d.m.confirm.lines, " "), "hello from the page") {
		t.Errorf("alert text: %v", d.m.confirm.lines)
	}
	d.key("esc") // an alert has one answer; Esc gives it too
	d.until("alert gone", func() bool { return !d.m.confirm.isActive() && d.m.dialog == nil })

	d.cursorOn(ir.Button, "Prompt")
	d.key("enter")
	d.until("prompt shown", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputPrompt })
	if d.m.input.value != "anon" {
		t.Errorf("prompt default %q", d.m.input.value)
	}
	d.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("bob")
	d.key("enter")
	d.until("prompt answered", func() bool { return strings.Contains(dumpLayout(d.page().lay), "answer:bob") })

	d.cursorOn(ir.Button, "Confirm")
	d.key("enter")
	d.until("confirm shown", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmDialog })
	d.key("esc")
	d.until("confirm declined", func() bool { return strings.Contains(dumpLayout(d.page().lay), "confirmed:false") })
}

func TestCertificateErrorAsksOnce(t *testing.T) {
	b := hookBrowser(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<title>Secure</title><h1>Behind a self-signed certificate</h1><a href='/two'>two</a>"))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})
	d.until("the certificate question", func() bool {
		return d.m.confirm.isInteractive() && d.m.confirm.action == confirmCert
	})
	if p := d.page(); !strings.Contains(p.errText, "ERR_CERT") {
		t.Errorf("error text: %q", p.errText)
	}
	d.key("enter")
	d.until("page loads once allowed", d.loaded("Secure"))
	// A second navigation in the same tab is not asked again.
	d.cursorOn(ir.Link, "two")
	d.key("enter")
	d.until("second page", func() bool { return strings.HasSuffix(d.page().url, "/two") && !d.page().loading })
	if d.m.confirm.isActive() {
		t.Error("asked again for the same tab")
	}
}

func TestDownloadToasts(t *testing.T) {
	b := hookBrowser(t)
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hello.txt" {
			w.Header().Set("Content-Disposition", "attachment; filename=hello.txt")
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<title>Files</title><a href='/hello.txt'>get the file</a>"))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{DownloadDir: dir})
	d.until("files page", d.loaded("Files"))
	d.cursorOn(ir.Link, "get the file")
	d.key("enter")
	d.until("saved toast", func() bool {
		return d.m.toast.isActive() && strings.HasPrefix(d.m.toast.msg, "saved ")
	})
	if _, err := os.Stat(filepath.Join(dir, "hello.txt")); err != nil {
		t.Errorf("the file is not in the download dir: %v", err)
	}
}

func TestSearchThenEnterClicks(t *testing.T) {
	b := hookBrowser(t)
	abs, _ := filepath.Abs("testdata/nav.html")
	d := startAt(t, b, "file://"+abs, store.Config{})
	d.until("page A", d.loaded("Page A"))
	d.key("/")
	if !d.m.sel.on || !d.m.sel.typing {
		t.Fatalf("/ enters selection mode typing: %+v", d.m.sel)
	}
	d.key("link to b")
	d.key("enter")
	if d.m.sel.cur != 0 {
		t.Fatalf("no match found: %+v", d.m.sel)
	}
	if v := d.m.View(); !strings.Contains(v, "/link to b") {
		t.Errorf("the query is not on the URL row:\n%s", v)
	}
	d.key("enter")
	d.until("page B", d.loaded("Page B"))
	if d.m.sel.on {
		t.Error("clicking out of selection mode should leave it")
	}
}
