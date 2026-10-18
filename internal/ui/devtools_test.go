package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/store"
)

// A page that leaves a cookie, a storage item, two console lines and one
// fetch behind it; the DevTools screen has to show all four.
const devPage = `<!doctype html><title>Dev</title>
<script>
document.cookie = "tasty=yes; path=/";
localStorage.setItem("theme", "dark");
sessionStorage.setItem("once", "1");
console.log("hello", "console", 42);
console.error("bad thing");
fetch("/api/data").then(r => r.json()).then(d => { document.querySelector("#out").textContent = d.ok; });
</script>
<h1>Dev page</h1><p id="out"></p><img src="/pic.png" alt="a picture">`

// pngBytes is the start of a PNG: not text, not valid UTF-8, with a NUL.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func TestDevtoolsShowsStorageNetworkConsole(t *testing.T) {
	b := hookBrowser(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":"yes"}`))
		case "/pic.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes)
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(devPage))
		}
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})
	d.until("dev page", d.loaded("Dev"))
	d.until("the fetch has landed", func() bool { return strings.Contains(dumpLayout(d.page().lay), "yes") })

	// I is the DevTools screen: the network first, the requests listed.
	d.key("I")
	if !d.m.devOn() {
		t.Fatal("I is the DevTools screen")
	}
	d.until("requests listed", func() bool {
		for _, e := range d.m.dev.net {
			if strings.HasSuffix(e.URL, "/api/data") {
				return true
			}
		}
		return false
	})
	// / filters; [2] follows the cursor with the request in full, its body
	// once it has come.
	d.key("/")
	d.key("api")
	d.key("enter")
	d.until("detail with body", d.shows(`{"ok":"yes"}`))
	d.key("esc") // the filter
	d.key("/")
	d.key("pic")
	d.key("enter")
	d.until("binary body", d.shows("of image/png"))
	d.key("esc")

	// Storage is the second tab: read on asking, a row a cookie or item.
	d.key("l")
	d.until("storage fetched", d.shows("tasty"))
	if v := d.view(); !strings.Contains(v, "theme") || !strings.Contains(v, "once") {
		t.Errorf("local and session storage too:\n%s", v)
	}
	d.m.devList().Select("c0")
	d.key("x")
	d.until("cookie gone", func() bool { return len(d.m.dev.storage.Cookies) == 0 })

	// The console: the page's lines, and i runs a line in the page.
	d.key("l")
	d.until("console lines", d.shows("bad thing"))
	d.press("i")
	d.until("prompt", d.shows("RUN JAVASCRIPT"))
	d.key("1 + 2")
	d.press("enter")
	d.until("3", func() bool { return strings.Contains(consoleText(d), "input:1 + 2|result:3|") })
	d.press("i")
	d.key("nope()")
	d.press("enter")
	d.until("the error", func() bool { return strings.Contains(consoleText(d), "error:ReferenceError: nope is not defined") })

	// The Source tab is the page's HTML, and / filters it.
	d.key("l")
	d.until("html fetched", d.shows("<title>Dev</title>"))
	d.key("/")
	d.key("h1")
	d.key("enter")
	if rows := d.m.devList().Rows(); len(rows) == 0 {
		t.Fatal("the source has rows")
	}
	if !strings.Contains(d.view(), "Dev page") {
		t.Errorf("the filter keeps the h1:\n%s", d.view())
	}
}

func consoleText(d *driver) string {
	out := ""
	for _, e := range d.m.dev.console {
		out += e.Level + ":" + e.Text + "|"
	}
	return out
}

func TestDevtoolsWithoutAPage(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	d := newDriver(t, New(nil, ""))
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.key("I")
	d.settle()
	if !d.m.devOn() || len(d.m.devList().Rows()) != 0 {
		t.Errorf("DevTools with no tab is empty:\n%s", d.view())
	}
}
