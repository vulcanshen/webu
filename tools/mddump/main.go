// Command mddump is the experiment for webu's next page view (user,
// 2026-10-28): a page fetched with net/http, parsed as HTML, its content
// picked out from the site around it (navigation, sidebars, footers),
// written as markdown. No Chromium.
//
//	go run . <url>                    the content, as markdown
//	go run . -whole <url>             the whole <body> as markdown, nothing left out
//	go run . -save page.html <url>    keeps what was fetched
//	go run . -file page.html <url>    a saved page instead of fetching it
//
// What came back — status, content type, final URL, size — and whether
// the page looks like a shell its JavaScript fills in go to stderr; the
// markdown alone goes to stdout. The content is picked by Readability
// (Firefox's reader view, in Go), as a baseline to measure webu's own
// rules against.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// userAgent is a browser's, with webu's name at the end: some sites
// answer anything else with less.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 webu/dev"

func main() {
	whole := flag.Bool("whole", false, "the whole <body> as markdown, nothing left out")
	save := flag.String("save", "", "write the fetched HTML to this file")
	file := flag.String("file", "", "read the HTML from this file instead of fetching the URL")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: mddump [-whole] [-save page.html | -file page.html] <url>")
		os.Exit(2)
	}
	page, err := url.Parse(flag.Arg(0))
	if err != nil {
		fail(err)
	}

	raw, ctype, final, err := fetch(page, *file)
	if err != nil {
		fail(err)
	}
	// Where the page really is, after its redirects: what its relative
	// links are relative to.
	page = final
	if *save != "" {
		if err := os.WriteFile(*save, raw, 0o644); err != nil {
			fail(err)
		}
	}
	if ctype != "" && !strings.Contains(ctype, "html") {
		fail(fmt.Errorf("not HTML: %s", ctype))
	}
	// Whatever the page was written in, UTF-8 from here (Big5, Shift_JIS…).
	r, err := charset.NewReader(bytes.NewReader(raw), ctype)
	if err != nil {
		fail(err)
	}
	doc, err := html.Parse(r)
	if err != nil {
		fail(err)
	}
	absolutize(doc, page)
	text, scripts := bodyWeight(doc)
	fmt.Fprintf(os.Stderr, "body: %d characters of text, %d scripts", text, scripts)
	if text < 200 && scripts > 0 {
		fmt.Fprint(os.Stderr, " — looks like a shell its JavaScript fills in")
	}
	fmt.Fprintln(os.Stderr)

	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(), commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(), strikethrough.NewStrikethroughPlugin()))
	domain := page.Scheme + "://" + page.Host

	if *whole {
		md, err := conv.ConvertNode(body(doc), converter.WithDomain(domain))
		if err != nil {
			fail(err)
		}
		fmt.Println(string(md))
		return
	}

	fmt.Fprintf(os.Stderr, "readerable: %v\n", readability.CheckDocument(doc))
	art, err := readability.FromDocument(doc, page)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "title: %q  byline: %q  site: %q\n", art.Title(), art.Byline(), art.SiteName())
	if art.Node == nil {
		fail(fmt.Errorf("no content found"))
	}
	md, err := conv.ConvertNode(art.Node, converter.WithDomain(domain))
	if err != nil {
		fail(err)
	}
	if t := strings.TrimSpace(art.Title()); t != "" {
		fmt.Printf("# %s\n\n", t)
	}
	fmt.Println(string(md))
}

// fetch gets the page: from file when one is named, else over HTTP,
// following redirects and keeping cookies across them. It says where the
// page ended up.
func fetch(page *url.URL, file string) ([]byte, string, *url.URL, error) {
	if file != "" {
		raw, err := os.ReadFile(file)
		return raw, "text/html", page, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", nil, err
	}
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", page.String(), nil)
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	// No Accept-Encoding: Go asks for gzip itself and undoes it; brotli it
	// cannot read, and a server is not offered it.
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", nil, err
	}
	ctype := resp.Header.Get("Content-Type")
	fmt.Fprintf(os.Stderr, "%s  %s  %s  %d bytes\n", resp.Status, ctype, resp.Request.URL, len(raw))
	if resp.StatusCode >= 400 {
		return raw, ctype, resp.Request.URL, fmt.Errorf("%s", resp.Status)
	}
	return raw, ctype, resp.Request.URL, nil
}

// absolutize makes every href and src absolute, against the page's
// <base href> when it has one, else the page's own address: a link
// written "#try_it" or "../a" means nothing once it is out of its page.
func absolutize(doc *html.Node, page *url.URL) {
	base := page
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i, a := range n.Attr {
				switch {
				case n.Data == "base" && a.Key == "href":
					if u, err := page.Parse(strings.TrimSpace(a.Val)); err == nil {
						base = u
					}
				case a.Key == "href" || a.Key == "src":
					if u, err := base.Parse(strings.TrimSpace(a.Val)); err == nil {
						n.Attr[i].Val = u.String()
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
}

// body is the document's <body>, or the document when it has none.
func body(doc *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == nil {
		return doc
	}
	return found
}

// bodyWeight is how much text the <body> holds outside scripts, styles and
// templates, and how many scripts the page has: a page with almost no
// text and some script is one its JavaScript builds.
func bodyWeight(doc *html.Node) (text, scripts int) {
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inBody bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script":
				scripts++
				return
			case "style", "noscript", "template", "svg":
				return
			case "body":
				inBody = true
			}
		}
		if n.Type == html.TextNode && inBody {
			text += len([]rune(strings.TrimSpace(n.Data)))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inBody)
		}
	}
	walk(doc, false)
	return text, scripts
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mddump:", err)
	os.Exit(1)
}
