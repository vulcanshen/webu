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
// fetch behind it; the DevTools popup has to show all four.
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
		if r.URL.Path == "/api/data" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":"yes"}`))
			return
		}
		if r.URL.Path == "/pic.png" {
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(devPage))
	}))
	defer srv.Close()
	d := startAt(t, b, srv.URL, store.Config{})
	d.until("dev page", d.loaded("Dev"))
	d.until("the fetch has landed", func() bool { return strings.Contains(dumpLayout(d.page().lay), "yes") })

	d.key("I")
	d.until("devtools open", func() bool { return d.m.devtools.isInteractive() })
	d.until("storage fetched", func() bool { return len(d.m.devtools.storage.data.Cookies) > 0 })
	st := d.m.devtools.storage.data
	if st.Cookies[0].Name != "tasty" || st.Cookies[0].Value != "yes" {
		t.Errorf("cookie: %+v", st.Cookies[0])
	}
	if len(st.Local) != 1 || st.Local[0] != [2]string{"theme", "dark"} {
		t.Errorf("local storage: %v", st.Local)
	}
	if len(st.Session) != 1 || st.Session[0][0] != "once" {
		t.Errorf("session storage: %v", st.Session)
	}
	if v := d.m.View(); !strings.Contains(v, "tasty") || !strings.Contains(v, "theme") {
		t.Errorf("storage tab not drawn:\n%s", v)
	}

	// x on the cookie deletes it and the tab refetches.
	d.key("x")
	d.until("cookie gone", func() bool { return len(d.m.devtools.storage.data.Cookies) == 0 })

	d.key("l")
	d.until("network tab", func() bool { return d.m.devtools.tab == devNetwork })
	d.until("requests listed", func() bool {
		for _, e := range d.m.devtools.network.entries {
			if strings.HasSuffix(e.URL, "/api/data") && e.Done && e.Status == 200 {
				return true
			}
		}
		return false
	})
	if v := d.m.View(); !strings.Contains(v, "/api/data") {
		t.Errorf("network tab not drawn:\n%s", v)
	}
	// Filter to the fetch, open its detail, and the body is there.
	d.key("/")
	d.key("api")
	d.key("enter")
	if n := len(d.m.devtools.network.visible("api")); n != 1 {
		t.Fatalf("filter: %d rows", n)
	}
	d.key("enter")
	d.until("detail with body", func() bool {
		return d.m.devtools.detail.isActive() && strings.Contains(strings.Join(d.m.devtools.detail.lines, "\n"), `{"ok":"yes"}`)
	})
	d.key("esc")
	d.until("detail closed", func() bool { return !d.m.devtools.detail.isActive() })
	d.key("esc") // the filter
	if d.m.devtools.filter[devNetwork] != "" {
		t.Error("Esc should clear the filter first")
	}

	// An image's body is not shown as text.
	d.until("the image request", func() bool {
		for _, e := range d.m.devtools.network.entries {
			if strings.HasSuffix(e.URL, "/pic.png") && e.Done {
				return true
			}
		}
		return false
	})
	d.key("/")
	d.key("pic")
	d.key("enter")
	d.key("enter")
	d.until("binary body", func() bool {
		return d.m.devtools.detail.isActive() && strings.Contains(strings.Join(d.m.devtools.detail.lines, "\n"), "(binary:")
	})
	for i, l := range d.m.devtools.detail.lines {
		if strings.Contains(l, "PNG") || strings.ContainsRune(l, 0) {
			t.Errorf("line %d leaks the bytes: %q", i, l)
		}
	}
	d.key("esc")
	d.key("esc")

	d.key("l")
	d.until("console tab", func() bool { return d.m.devtools.tab == devConsole })
	d.until("console lines", func() bool { return len(d.m.devtools.console.entries) >= 2 })
	joined := ""
	for _, e := range d.m.devtools.console.entries {
		joined += e.Level + ":" + e.Text + "|"
	}
	if !strings.Contains(joined, "log:hello console 42|") || !strings.Contains(joined, "error:bad thing|") {
		t.Errorf("console: %s", joined)
	}

	// Enter is the entry, whole: the list cuts long messages at the width.
	for i, e := range d.m.devtools.console.entries {
		if e.Text == "bad thing" {
			d.m.devtools.console.cursor = i
		}
	}
	d.key("enter")
	d.until("console detail", func() bool {
		return d.m.devtools.detail.isActive() && strings.Contains(strings.Join(d.m.devtools.detail.lines, "\n"), "bad thing")
	})
	if !strings.Contains(strings.Join(d.m.devtools.detail.lines, "\n"), "level    error") {
		t.Errorf("detail head: %v", d.m.devtools.detail.lines)
	}
	d.key("esc")
	d.until("detail closed", func() bool { return !d.m.devtools.detail.isActive() })

	// i is the prompt: an expression runs in the page, the input and its
	// result join the list, and the prompt stays for the next one.
	d.key("i")
	d.until("prompt", func() bool { return d.m.input.isInteractive() && d.m.input.action == inputEval })
	d.key("1 + 2")
	d.key("enter")
	d.until("3", func() bool { return strings.Contains(consoleText(d), "input:1 + 2|result:3|") })
	if !d.m.input.isInteractive() || d.m.input.value != "" {
		t.Errorf("the prompt should stay open and empty: active=%v value=%q", d.m.input.isActive(), d.m.input.value)
	}
	d.key("document.title")
	d.key("enter")
	d.until("the title", func() bool { return strings.Contains(consoleText(d), "result:'Dev'|") })
	d.key("await Promise.resolve(40 + 2)")
	d.key("enter")
	d.until("awaited", func() bool { return strings.Contains(consoleText(d), "result:42|") })
	d.key("nope()")
	d.key("enter")
	d.until("the error", func() bool { return strings.Contains(consoleText(d), "error:ReferenceError: nope is not defined") })
	d.key("esc")
	d.until("prompt gone", func() bool { return !d.m.input.isActive() && d.m.devtools.isInteractive() })

	// The Source tab is the page's HTML, and / greps it.
	d.key("l")
	d.until("source tab", func() bool { return d.m.devtools.tab == devSource })
	d.until("html fetched", func() bool {
		return strings.Contains(strings.Join(d.m.devtools.source.lines, "\n"), "<title>Dev</title>")
	})
	d.key("/")
	d.key("h1")
	d.key("enter")
	if v := d.m.devtools.source.visible("h1"); len(v) != 1 || !strings.Contains(v[0], "Dev page") {
		t.Errorf("grep h1: %q", v)
	}
	d.key("esc") // the filter
	d.key("esc")
	d.until("devtools closed", func() bool { return !d.m.devtools.isActive() })
}

func consoleText(d *driver) string {
	out := ""
	for _, e := range d.m.devtools.console.entries {
		out += e.Level + ":" + e.Text + "|"
	}
	return out
}

func TestDevtoolsWithoutAPage(t *testing.T) {
	m := New(nil, "")
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mm := model.(AppModel)
	model, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	mm = model.(AppModel)
	if mm.devtools.isActive() {
		t.Error("DevTools opened with no tab")
	}
}
