package reader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// A load keeps each request it made, a redirect one of them, with the
// headers both ways and the bytes read; the page keeps its source.
func TestRequestsAndSource(t *testing.T) {
	const page = "<html><head><title>New</title></head><body><p>Here.</p></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, page)
	}))
	defer srv.Close()
	c := NewClient(EmptyJar(""), "test-agent")
	log := &Requests{}
	resp, err := c.Get(Record(context.Background(), log), srv.URL+"/old")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Read(resp)
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != page || d.Type != "text/html" {
		t.Errorf("source %q, type %q", d.Source, d.Type)
	}
	got := log.List()
	if len(got) != 2 {
		t.Fatalf("%d requests, want the redirect and the page: %+v", len(got), got)
	}
	if got[0].Method != "GET" || !strings.HasSuffix(got[0].URL, "/old") || !strings.HasPrefix(got[0].Status, "301") ||
		got[0].Response.Get("Location") != "/new" {
		t.Errorf("the redirect: %+v", got[0])
	}
	if !strings.HasSuffix(got[1].URL, "/new") || !strings.HasPrefix(got[1].Status, "200") ||
		got[1].Size != int64(len(page)) || got[1].Request.Get("User-Agent") != "test-agent" {
		t.Errorf("the page: %+v", got[1])
	}
	if _, err := c.Get(context.Background(), srv.URL+"/new"); err != nil || len(log.List()) != 2 {
		t.Errorf("a request with no Requests in its context is not kept: %v %d", err, len(log.List()))
	}
}

// A site's cookies are its subdomains' too, and clearing them leaves
// another site's, in the jar and in the file.
func TestSiteCookies(t *testing.T) {
	path := t.TempDir() + "/cookies.json"
	j, err := OpenJar(path)
	if err != nil {
		t.Fatal(err)
	}
	at := func(host string) *url.URL { return &url.URL{Scheme: "https", Host: host, Path: "/"} }
	j.SetCookies(at("en.example.org"), []*http.Cookie{{Name: "a", Value: "1"}})
	j.SetCookies(at("www.example.org"), []*http.Cookie{{Name: "b", Value: "2", Domain: ".example.org"}})
	j.SetCookies(at("other.net"), []*http.Cookie{{Name: "c", Value: "3"}})
	if Site("en.example.org") != "example.org" || Site("localhost") != "localhost" || Site("127.0.0.1") != "127.0.0.1" {
		t.Errorf("sites: %s %s", Site("en.example.org"), Site("localhost"))
	}
	got := j.SiteCookies("example.org")
	if len(got) != 2 || got[0].Name != "a" || got[0].Domain != "en.example.org" || got[1].Name != "b" || got[1].Domain != "example.org" {
		t.Fatalf("example.org's cookies: %+v", got)
	}
	if n, err := j.ClearSite("en.example.org"); n != 2 || err != nil {
		t.Fatalf("cleared %d: %v", n, err)
	}
	if len(j.SiteCookies("example.org")) != 0 || len(j.Cookies(at("en.example.org"))) != 0 || len(j.Cookies(at("other.net"))) != 1 {
		t.Error("only example.org's cookies should be gone from the jar")
	}
	again, err := OpenJar(path)
	if err != nil || len(again.SiteCookies("example.org")) != 0 || len(again.SiteCookies("other.net")) != 1 {
		t.Errorf("the file should keep only other.net's: %v", err)
	}
}
