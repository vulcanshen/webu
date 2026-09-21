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
	t.Setenv("WEBU_DATA", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}

func startAt(t *testing.T, b *browser.Browser, url string, cfg store.Config) *driver {
	t.Helper()
	d := newDriver(t, New(b, url).WithStore(nil, nil, cfg, nil))
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
	d.act()
	d.until("a second tab showing page B", func() bool {
		return len(d.m.tabs) == 2 && d.m.shown == 1 && d.m.tabs[1].title == "Page B" && !d.m.tabs[1].loading
	})
	if d.m.cur2 != 1 || d.m.focus != panelPage {
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
	d.act()
	d.until("alert shown", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmDialog })
	if !strings.Contains(strings.Join(d.m.confirm.lines, " "), "hello from the page") {
		t.Errorf("alert text: %v", d.m.confirm.lines)
	}
	d.key("esc") // an alert has one answer; Esc gives it too
	d.until("alert gone", func() bool { return !d.m.confirm.isActive() && d.m.dialog == nil })

	d.cursorOn(ir.Button, "Prompt")
	d.act()
	d.until("prompt shown", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputPrompt })
	if d.m.input.value != "anon" {
		t.Errorf("prompt default %q", d.m.input.value)
	}
	d.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("bob")
	d.key("enter")
	d.until("prompt answered", func() bool { return strings.Contains(dumpLayout(d.page().lay), "answer:bob") })

	d.cursorOn(ir.Button, "Confirm")
	d.act()
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
	d.act()
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
	d.act()
	d.until("saved toast", func() bool {
		return d.m.toast.isActive() && strings.HasPrefix(d.m.toast.msg, "saved ")
	})
	if _, err := os.Stat(filepath.Join(dir, "hello.txt")); err != nil {
		t.Errorf("the file is not in the download dir: %v", err)
	}
}

func TestHTTPAuthIsAsked(t *testing.T) {
	b := hookBrowser(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "ann" || p != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="the vault"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("<title>Denied</title>denied"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<title>Inside</title><h1>Welcome ann</h1>"))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})
	d.until("name asked", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputAuthUser })
	if !strings.Contains(d.m.input.prompt, "the vault") {
		t.Errorf("prompt names the realm: %q", d.m.input.prompt)
	}
	d.key("ann")
	d.key("enter")
	d.until("password asked", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputAuthPass })
	if !d.m.input.masked {
		t.Error("the password box is not masked")
	}
	d.key("secret")
	d.key("enter")
	d.until("inside", d.loaded("Inside"))
	if !strings.Contains(d.m.View(), "Welcome ann") {
		t.Errorf("page not drawn:\n%s", d.m.View())
	}
}

func TestFileUploadIsAsked(t *testing.T) {
	b := hookBrowser(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.txt")
	os.WriteFile(file, []byte("hi"), 0o644)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<title>Upload</title><label>Attach <input type="file" onchange="document.querySelector('#out').textContent = 'chose:' + this.files[0].name"></label><p id="out"></p>`))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})
	d.until("upload page", d.loaded("Upload"))
	d.cursorOn(ir.Button, "Attach")
	d.act()
	d.until("path asked", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputFile })
	d.key(file)
	d.key("enter")
	d.until("file chosen", func() bool { return strings.Contains(dumpLayout(d.page().lay), "chose:notes.txt") })
}

func TestUndoCloseAndQuitConfirm(t *testing.T) {
	b := hookBrowser(t)
	abs, _ := filepath.Abs("testdata/nav2.html")
	d := startAt(t, b, "file://"+abs, store.Config{})
	d.until("page B", d.loaded("Page B"))
	d.key("1")
	d.key("c")
	if len(d.m.tabs) != 0 || len(d.m.closed) != 1 {
		t.Fatalf("after c: %d tabs, %d closed", len(d.m.tabs), len(d.m.closed))
	}
	d.key("U")
	d.until("page B is back", d.loaded("Page B"))
	if len(d.m.closed) != 0 {
		t.Errorf("undo should pop the stack: %d left", len(d.m.closed))
	}

	// C in [2] closes the page being shown, whatever [1]'s cursor says.
	d.key("2")
	d.key("C")
	if len(d.m.tabs) != 0 || len(d.m.closed) != 1 || d.m.shownTab() != nil {
		t.Fatalf("after C: %d tabs, %d closed", len(d.m.tabs), len(d.m.closed))
	}
	if !strings.Contains(d.m.View(), "no page") {
		t.Errorf("[2] should be empty:\n%s", d.m.View())
	}
	d.key("1")
	d.key("U")
	d.until("page B is back again", d.loaded("Page B"))

	d.m.dls = []download{{guid: "g1", name: "big.iso"}}
	d.key("q")
	if !d.m.confirm.isActive() || d.m.confirm.action != confirmQuit {
		t.Error("q with a download in flight should ask first")
	}
}

// A JSON document is drawn as the JSON, indented, on the code ground —
// not as Chrome's viewer with its Pretty-print form — and the window
// starts at the top.
func TestJSONDocumentIsCode(t *testing.T) {
	b := hookBrowser(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"title":"first"},{"id":2,"title":"second"}]`))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})
	d.until("json page", func() bool { p := d.page(); return p != nil && !p.loading && p.root != nil })
	rows := d.page().lay.rows
	if len(rows) < 8 || !rows[0].code || !strings.HasPrefix(rows[0].plain(), "[") || !strings.Contains(rows[2].plain(), `"id": 1`) {
		t.Fatalf("JSON not drawn as an indented code block:\n%s", dumpLayout(d.page().lay))
	}
	if d.page().top != 0 {
		t.Errorf("a fresh page starts at the top, not at %d", d.page().top)
	}
	keyed := false
	for _, s := range rows[2].segs {
		if strings.TrimSpace(s.text) == `"id"` && s.kind == segCodeKey {
			keyed = true
		}
	}
	if !keyed {
		t.Errorf("the JSON key is not coloured as a key: %+v", rows[2].segs)
	}
	for _, it := range d.page().lay.items {
		if it.node.Kind == ir.Check {
			t.Error("Chrome's Pretty-print checkbox leaked into the page")
		}
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
	// Enter on the match leaves the mode and asks to open the link; Enter
	// again opens it.
	d.key("enter")
	d.until("open link?", func() bool { return d.m.confirm.isInteractive() && d.m.confirm.action == confirmOpenLink })
	if d.m.sel.on {
		t.Error("Enter out of selection mode should leave it")
	}
	d.key("enter")
	d.until("page B", d.loaded("Page B"))
}

// The command line is the Location box: every argument is a tab, the first
// one shown, and a bare host gets its scheme (function.md §12).
func TestCommandLineOpensEachArgument(t *testing.T) {
	b := hookBrowser(t)
	abs, _ := filepath.Abs("testdata/nav.html")
	abs2, _ := filepath.Abs("testdata/nav2.html")
	d := newDriver(t, New(b, "file://"+abs, "file://"+abs2).WithStore(nil, nil, store.Config{}, nil))
	t.Cleanup(d.m.Close)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("page A", d.loaded("Page A"))
	if len(d.m.tabs) != 2 || d.m.shown != 0 {
		t.Fatalf("%d tabs, shown %d", len(d.m.tabs), d.m.shown)
	}
	d.until("page B in the second tab", func() bool { return d.m.tabs[1].title == "Page B" })

	m := New(nil, "go.dev", " ", "two words")
	if len(m.startURLs) != 2 || m.resolveURL(m.startURLs[0]) != "https://go.dev" || !strings.Contains(m.resolveURL(m.startURLs[1]), "q=two") {
		t.Errorf("start arguments: %v -> %q, %q", m.startURLs, m.resolveURL(m.startURLs[0]), m.resolveURL(m.startURLs[1]))
	}
}

// A bookmark opens in a new tab, never over the one the web was showing.
func TestBookmarkOpensANewTab(t *testing.T) {
	b := hookBrowser(t)
	abs, _ := filepath.Abs("testdata/nav.html")
	abs2, _ := filepath.Abs("testdata/nav2.html")
	d := startAt(t, b, "file://"+abs, store.Config{})
	d.until("page A", d.loaded("Page A"))
	d.m.bookmarks = []store.Bookmark{{Title: "B", URL: "file://" + abs2}}

	d.key("B")
	if d.m.screen != screenBookmarks {
		t.Fatalf("screen %v", d.m.screen)
	}
	d.key("enter")
	if d.m.screen != screenWeb || len(d.m.tabs) != 2 || d.m.shown != 1 {
		t.Fatalf("after Enter: screen %v, %d tabs, shown %d", d.m.screen, len(d.m.tabs), d.m.shown)
	}
	d.until("page B in the new tab", d.loaded("Page B"))
	if d.m.tabs[0].title != "Page A" {
		t.Errorf("the first tab should be untouched: %q", d.m.tabs[0].title)
	}
}
