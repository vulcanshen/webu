package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/reader"
	"github.com/vulcanshen/webu/internal/store"
)

// site is a small site for the reader to read: an article with its
// navigation, a second page, a sign-in, a search, a file, a missing page.
func site(t *testing.T) *httptest.Server {
	t.Helper()
	prose := strings.Repeat("Words of the first page, enough of them to be read as an article. ", 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<html><head><title>First</title></head><body><nav><a href="/elsewhere">Site menu</a></nav>
<article><h1>First</h1><p><a href="/two">the second page</a> `+prose+`</p><p>`+prose+`</p>
<p><a href="/f.zip">a file</a></p></article></body></html>`)
		case "/two":
			io.WriteString(w, `<html><head><title>Second</title></head><body><h1>Second</h1><p>The second page.</p>
<ul><li>One<ul><li>Two</li></ul></li></ul><ol><li>A</li><li>B<ol><li>C<ol><li>D</li></ol></li></ol></li></ol>
<pre class="language-go">x := 1</pre><blockquote><p>Quoted words</p></blockquote></body></html>`)
		case "/login":
			if r.Method == "POST" {
				r.ParseForm()
				if r.Form.Get("token") != "t0k" || r.Form.Get("user") != "me" || r.Form.Get("pass") != "secret" {
					http.Error(w, "no", http.StatusForbidden)
					return
				}
				http.SetCookie(w, &http.Cookie{Name: "session", Value: "me", Path: "/"})
				http.Redirect(w, r, "/home", http.StatusSeeOther)
				return
			}
			io.WriteString(w, `<html><head><title>Sign in</title></head><body><form method="post" action="/login">
<input type="hidden" name="token" value="t0k"><label>Name <input name="user"></label>
<label>Password <input type="password" name="pass"></label><button>Sign in</button></form></body></html>`)
		case "/home":
			c, err := r.Cookie("session")
			if err != nil {
				io.WriteString(w, `<html><body><p>Signed out</p></body></html>`)
				return
			}
			io.WriteString(w, `<html><head><title>Home</title></head><body><p>Welcome, `+c.Value+`</p></body></html>`)
		case "/search":
			io.WriteString(w, `<html><head><title>Search</title></head><body><form action="/find"><input name="q" placeholder="Find words"></form></body></html>`)
		case "/find":
			io.WriteString(w, `<html><head><title>Found</title></head><body><p>Results for `+r.URL.Query().Get("q")+`</p></body></html>`)
		case "/f.zip":
			w.Header().Set("Content-Type", "application/zip")
			io.WriteString(w, "PK zip bytes")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// readerAt is webu with a reader tab at url, its files in scratch
// directories, its downloads in dl.
func readerAt(t *testing.T, url, dl string) (*driver, *AppModel) {
	t.Helper()
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	t.Setenv("WEBU__STATE", "")
	m := New(nil, url).WithStore(nil, nil, store.Config{DownloadDir: dl}, nil)
	m.chromiumPages = false
	jar, err := reader.OpenJar(store.Path("cookies.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.WithReader(reader.NewClient(jar, ""))
	d := newDriver(t, m)
	t.Cleanup(m.Close)
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	return d, m
}

// readerOn moves Focus mode's cursor onto the item whose node says what,
// j by j, going into the mode first.
func readerOn(d *driver, what string) {
	d.t.Helper()
	if md := d.m.pagePanel.InMode(); md == nil || md.Name != "Focus" {
		d.press("f")
	}
	for range 40 {
		if t := d.m.shownTab(); t != nil {
			if n := t.current(); n != nil && (strings.Contains(n.Name, what) || strings.Contains(n.URL, what) ||
				n.Kind == ir.Textbox && strings.Contains(n.Name, what)) {
				return
			}
		}
		d.key("j")
	}
	d.t.Fatalf("no item %q on the page:\n%s", what, d.view())
}

// A reader page is its content as markdown: the article without the
// site's menu, f for the full page; a link followed, P and N back and
// forth along the tab's own history; the App Log has what was done.
func TestReaderReadsAndFollows(t *testing.T) {
	srv := site(t)
	d, m := readerAt(t, srv.URL+"/", t.TempDir())
	d.until("the first page", d.shows("Words of the first page"))
	if v := d.view(); strings.Contains(v, "Site menu") || strings.Contains(v, "# First") {
		t.Errorf("the content holds the site's menu, or a markdown symbol:\n%s", v)
	}
	d.press("2")
	d.press("w")
	d.until("the full page", d.shows("Site menu"))
	d.press("w")
	d.until("the content again", func() bool { return !strings.Contains(d.view(), "Site menu") })

	// / finds, n steps, Esc puts the search away.
	d.press("/")
	d.key("enough")
	d.press("enter")
	tb := m.shownTab()
	if tb.find == nil || len(tb.find.hits) < 2 {
		t.Fatalf("the search found %v", tb.find)
	}
	d.press("n")
	if tb.find.at != 1 {
		t.Errorf("n should go to the second hit, is on %d", tb.find.at)
	}
	d.press("esc")
	if tb.find != nil {
		t.Error("Esc should put the search away")
	}

	readerOn(d, "/two")
	d.press("enter")
	d.until("the second page", d.shows("The second page."))
	v := d.view()
	for _, want := range []string{"\uf111 One", "\uf0c8 Two", "1. A", "2. B", "a. C", "i. D", "go", "x := 1", "▎ Quoted words"} {
		if !strings.Contains(v, want) {
			t.Errorf("the second page should show %q:\n%s", want, v)
		}
	}
	if md := m.pagePanel.InMode(); md != nil {
		t.Errorf("following a link should leave Focus mode, is in %s", md.Name)
	}
	d.press("H")
	d.until("back on the first", d.shows("Words of the first page"))
	d.press("L")
	d.until("forward on the second", d.shows("The second page."))
	if tb := m.shownTab(); !tb.canBack || tb.canForward {
		t.Errorf("history: back %v forward %v", tb.canBack, tb.canForward)
	}
	d.press("!")
	for _, want := range []string{"Showed the full page of First", "Followed the link to " + srv.URL + "/two", "Went back in Second"} {
		if !strings.Contains(d.view(), want) {
			t.Errorf("the App Log should say %q:\n%s", want, d.view())
		}
	}
}

// A sign-in form is filled in and sent with its hidden field; the cookie
// it gets is kept in cookies.json, the user's alone, and sent after. A
// form with one field to type in offers to send itself.
func TestReaderSignsInAndSearches(t *testing.T) {
	srv := site(t)
	d, _ := readerAt(t, srv.URL+"/login", t.TempDir())
	d.until("the form", d.shows("┌ Form"))
	d.press("2")
	readerOn(d, "Name")
	d.press("enter")
	d.key("me")
	d.press("enter")
	readerOn(d, "Password")
	d.press("enter")
	d.key("secret")
	d.press("enter")
	if strings.Contains(d.view(), "secret") {
		t.Errorf("the password shows:\n%s", d.view())
	}
	readerOn(d, "Sign in")
	d.press("enter")
	d.until("signed in", d.shows("Welcome, me"))
	info, err := os.Stat(store.Path("cookies.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("cookies.json: %v %v", info, err)
	}
	if raw, _ := os.ReadFile(store.Path("cookies.json")); !strings.Contains(string(raw), "session") {
		t.Errorf("the cookie is not kept: %s", raw)
	}

	d.press("o")
	d.key("backspace")
	d.key(srv.URL + "/search")
	d.press("enter")
	d.until("the search", d.shows("[Find words"))
	readerOn(d, "Find words")
	d.press("enter")
	d.key("terminal")
	d.press("enter")
	d.until("the question", d.shows("Send the form"))
	d.press("enter")
	d.until("the results", d.shows("Results for terminal"))
}

// A link to a file downloads it into the download folder, the page
// staying; a page that is not there says so.
func TestReaderDownloadsAndMissingPages(t *testing.T) {
	srv := site(t)
	dl := t.TempDir()
	d, m := readerAt(t, srv.URL+"/", dl)
	d.until("the first page", d.shows("Words of the first page"))
	d.press("2")
	readerOn(d, "f.zip")
	d.press("enter")
	d.until("the file saved", func() bool {
		_, err := os.Stat(filepath.Join(dl, "f.zip"))
		return err == nil && len(m.dls) == 1 && m.dls[0].state == dlDone
	})
	if tb := m.shownTab(); tb.title != "First" || tb.doc == nil {
		t.Errorf("the page should stay, not %q:\n%s", tb.title, d.view())
	}
	d.press("esc") // out of Focus mode, where o opens a new tab
	d.press("o")
	d.key("backspace")
	d.key(srv.URL + "/nowhere")
	d.press("enter")
	d.until("the toast", d.shows("404 Not Found"))
}
