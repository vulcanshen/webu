package reader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vulcanshen/webu/internal/ir"
	"golang.org/x/text/encoding/traditionalchinese"
)

// page reads body as an HTML page at addr.
func page(t *testing.T, addr, body string) *Doc {
	t.Helper()
	u, _ := url.Parse(addr)
	d, err := Read(&Response{URL: u, Status: 200, Type: "text/html", ContentType: "text/html; charset=utf-8", Body: []byte(body)})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func md(d *Doc, root *ir.Node) string { return Text(d.Markdown(root)) }

var prose = strings.Repeat("A sentence of the article, long enough that Readability takes it for one. ", 12)

// An article keeps its words and leaves the site around it out; the whole
// page still has everything.
func TestAnArticleLeavesTheSiteOut(t *testing.T) {
	d := page(t, "https://example.com/post", `<html><head><title>The post</title></head><body>
<header><a href="/">Home</a></header>
<nav><a href="/a">Section A</a> <a href="/b">Section B</a></nav>
<article><h1>The post</h1><p>`+prose+`</p><p>`+prose+` <a href="/more">read more</a></p></article>
<aside>Related: <a href="/x">something else</a></aside>
<footer>Copyright</footer></body></html>`)
	content, whole := md(d, d.Content), md(d, d.Whole)
	if d.Picked != "article" {
		t.Errorf("picked %q", d.Picked)
	}
	for _, want := range []string{"# The post", "A sentence of the article", "[read more](https://example.com/more)"} {
		if !strings.Contains(content, want) {
			t.Errorf("the content should hold %q:\n%s", want, content)
		}
	}
	for _, gone := range []string{"Section A", "something else", "Copyright"} {
		if strings.Contains(content, gone) {
			t.Errorf("the content should not hold %q:\n%s", gone, content)
		}
		if !strings.Contains(whole, gone) {
			t.Errorf("the whole page should hold %q:\n%s", gone, whole)
		}
	}
}

// A page that is no article — a list of links — is the page less its
// navigation, header, footer and sidebars; a table it laid itself out
// with is its rows, a line each, and a table of data stays one.
func TestAListPageAndItsTables(t *testing.T) {
	d := page(t, "https://news.example/", `<html><body>
<nav class="main-menu"><a href="/login">login</a></nav>
<table><tr><td>1.</td><td><center><a href="/vote"><div class="votearrow"></div></a></center></td><td><a href="https://a.example/">A story</a> (a.example)</td></tr>
<tr><td></td><td>182 points by <a href="/user?id=x">x</a></td></tr></table>
<table><caption>Scores</caption><tr><th>Team</th><th>Points</th></tr><tr><td>Red</td><td>3</td></tr></table>
<div id="footer-links">About</div></body></html>`)
	content := md(d, d.Content)
	if d.Picked != "page" {
		t.Errorf("picked %q", d.Picked)
	}
	for _, want := range []string{"1. [A story](https://a.example/) (a.example)", "182 points by [x](https://news.example/user?id=x)",
		"| Team | Points |", "| --- | --- |", "| Red | 3 |"} {
		if !strings.Contains(content, want) {
			t.Errorf("the content should hold %q:\n%s", want, content)
		}
	}
	for _, gone := range []string{"login", "About", "/vote"} {
		if strings.Contains(content, gone) {
			t.Errorf("the content should not hold %q:\n%s", gone, content)
		}
	}
}

// What no one sees is not read: scripts, what the page hid, dialogs,
// cookie notices.
func TestWhatIsHiddenIsLeftOut(t *testing.T) {
	d := page(t, "https://example.com/", `<html><body><p>Seen</p>
<p hidden>Hidden attr</p><p aria-hidden="true">Aria hidden</p><p style="display: none">Display none</p>
<dialog>A dialog</dialog><div role="dialog">Role dialog</div><div class="cookie-banner">We use cookies</div>
<script>var x = "script text"</script><style>.a{}</style></body></html>`)
	whole := md(d, d.Whole)
	if !strings.Contains(whole, "Seen") {
		t.Errorf("seen text lost:\n%s", whole)
	}
	for _, gone := range []string{"Hidden attr", "Aria hidden", "Display none", "A dialog", "Role dialog", "We use cookies", "script text"} {
		if strings.Contains(whole, gone) {
			t.Errorf("%q should be left out:\n%s", gone, whole)
		}
	}
}

// An item is exactly its stretch of the line, and a link to #id lands on
// the heading that has it.
func TestItemsAndAnchors(t *testing.T) {
	d := page(t, "https://example.com/doc", `<html><body><h2 id="use">Use</h2>
<p>See <a href="#use">the use</a> and <strong>bold <a href="/b">link</a></strong>.</p>
<ul><li>One</li><li>Two <a href="/two">two</a></li></ul><pre class="language-go">x := 1
y := 2</pre></body></html>`)
	lines := d.Markdown(d.Whole)
	seen := map[string]bool{}
	for _, l := range lines {
		for _, it := range l.Items {
			seen[l.Text[it.From:it.To]] = true
		}
	}
	for _, want := range []string{"## Use", "[the use](https://example.com/doc#use)", "[link](https://example.com/b)", "[two](https://example.com/two)"} {
		if !seen[want] {
			t.Errorf("no item %q among %v:\n%s", want, seen, Text(lines))
		}
	}
	text := Text(lines)
	for _, want := range []string{"**bold [link](https://example.com/b)**", "- One\n- Two", "```go\nx := 1\ny := 2\n```"} {
		if !strings.Contains(text, want) {
			t.Errorf("should hold %q:\n%s", want, text)
		}
	}
	if h := d.Anchor(d.Whole, "use"); h == nil || h.Kind != ir.Heading {
		t.Errorf("#use lands on %v", h)
	}
}

// A form is filled and sent: GET puts the fields in the address, POST in
// the body with the hidden ones; a box only when ticked, a button only the
// one pressed, a select its choice.
func TestFormsAreSent(t *testing.T) {
	d := page(t, "https://example.com/start", `<html><body>
<form action="/search"><label>Search <input name="q"></label><button name="go" value="1">Go</button></form>
<form method="post" action="/login"><input type="hidden" name="token" value="t0k">
<label for="u">Name</label><input id="u" name="user"><input type="password" name="pass" placeholder="Password">
<input type="checkbox" name="keep" value="yes"> Keep me in
<select name="lang"><option value="en">English</option><option value="zh" selected>中文</option></select>
<input type="submit" name="act" value="Sign in"><input type="submit" name="act" value="Register"></form></body></html>`)
	if len(d.Forms) != 2 {
		t.Fatalf("%d forms", len(d.Forms))
	}
	search, login := d.Forms[0], d.Forms[1]
	search.Fields[0].Set("terminal browser")
	req, err := search.Request(context.Background(), search.Fields[1])
	if err != nil || req.Method != "GET" || req.URL.String() != "https://example.com/search?go=1&q=terminal+browser" {
		t.Errorf("search: %v %v", req.URL, err)
	}
	field := func(name string) *Field {
		for _, f := range login.Fields {
			if f.Name == name {
				return f
			}
		}
		t.Fatalf("no field %s", name)
		return nil
	}
	field("user").Set("me")
	field("pass").Set("secret")
	field("keep").Toggle()
	if field("user").Label != "Name" || field("pass").Label != "Password" {
		t.Errorf("labels: %q %q", field("user").Label, field("pass").Label)
	}
	var signIn *Field
	for _, f := range login.Fields {
		if f.Type == "submit" && f.Value == "Sign in" {
			signIn = f
		}
	}
	req, err = login.Request(context.Background(), signIn)
	if err != nil || req.Method != "POST" || req.URL.String() != "https://example.com/login" {
		t.Fatalf("login: %v %v", req, err)
	}
	body, _ := io.ReadAll(req.Body)
	got, _ := url.ParseQuery(string(body))
	want := url.Values{"token": {"t0k"}, "user": {"me"}, "pass": {"secret"}, "keep": {"yes"}, "lang": {"zh"}, "act": {"Sign in"}}
	if got.Encode() != want.Encode() {
		t.Errorf("login body %v, want %v", got, want)
	}
	text := md(d, d.Whole)
	for _, want := range []string{"┌ Form", "│ Search [terminal browser]", "( Go )", "[••••••]", "[x]", "[中文 ▾]", "└"} {
		if !strings.Contains(text, want) {
			t.Errorf("the form should read %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "t0k") || strings.Contains(text, "secret") {
		t.Errorf("a hidden field or a password shows:\n%s", text)
	}
}

// Cookies outlive the jar: written to a file only the user can read, read
// back by the next one, gone when cleared.
func TestCookiesAreKept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "old", Value: "x", Path: "/", MaxAge: -1})
		}
		if c, err := r.Cookie("session"); err == nil {
			io.WriteString(w, "<p>signed in as "+c.Value+"</p>")
			return
		}
		io.WriteString(w, "<p>signed out</p>")
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "cookies.json")
	jar, err := OpenJar(path)
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(jar, "test")
	if _, err := c.Get(context.Background(), srv.URL+"/set"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the file: %v %v", info, err)
	}
	jar2, err := OpenJar(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewClient(jar2, "test").Get(context.Background(), srv.URL+"/")
	if err != nil || !strings.Contains(string(r.Body), "signed in as abc") {
		t.Fatalf("the next jar does not send it: %s %v", r.Body, err)
	}
	if err := jar2.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the file outlived Clear: %v", err)
	}
	r, _ = NewClient(jar2, "test").Get(context.Background(), srv.URL+"/")
	if !strings.Contains(string(r.Body), "signed out") {
		t.Errorf("still signed in after Clear: %s", r.Body)
	}
	os.WriteFile(path, []byte("not json"), 0o600)
	if _, err := OpenJar(path); err == nil {
		t.Error("a file that will not read should be an error, for MOVE ASIDE")
	}
}

// Text reads as itself; a page in Big5 reads in UTF-8; anything else is a
// download, saved under its name, never over another file.
func TestTextBig5AndDownloads(t *testing.T) {
	big5, _ := traditionalchinese.Big5.NewEncoder().String("<p>中文網頁</p>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a.txt":
			w.Header().Set("Content-Type", "text/plain")
			io.WriteString(w, "line one\nline two\n")
		case "/big5":
			w.Header().Set("Content-Type", "text/html; charset=big5")
			io.WriteString(w, big5)
		case "/file.zip":
			w.Header().Set("Content-Type", "application/zip")
			io.WriteString(w, "PK")
		}
	}))
	defer srv.Close()
	jar, _ := OpenJar("")
	c := NewClient(jar, "test")
	r, err := c.Get(context.Background(), srv.URL+"/a.txt")
	if err != nil || !r.IsPage() {
		t.Fatalf("text: %v", err)
	}
	d, _ := Read(r)
	if got := md(d, d.Content); got != "```\nline one\nline two\n```\n" {
		t.Errorf("text reads as %q", got)
	}
	r, _ = c.Get(context.Background(), srv.URL+"/big5")
	d, _ = Read(r)
	if got := md(d, d.Content); !strings.Contains(got, "中文網頁") {
		t.Errorf("big5 reads as %q", got)
	}
	dir := t.TempDir()
	for i, want := range []string{"file.zip", "file (2).zip"} {
		r, _ = c.Get(context.Background(), srv.URL+"/file.zip")
		if r.IsPage() {
			t.Fatal("a zip is not a page")
		}
		p, err := r.Save(dir, nil)
		if err != nil || filepath.Base(p) != want {
			t.Errorf("save %d: %s %v", i, p, err)
		}
	}
}
