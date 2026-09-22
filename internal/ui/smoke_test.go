package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// The site smoke tests (function.md §11): three kinds of page, each has to
// load, draw, and keep every unsupported role visible. They reach the
// network, so they run only when asked — WEBU_SMOKE=1 — and never in CI by
// accident. They print the top of each page so a change in the renderer
// can be judged by eye in the -v log.
func smokeBrowser(t *testing.T) *browser.Browser {
	t.Helper()
	if os.Getenv("WEBU_SMOKE") == "" {
		t.Skip("set WEBU_SMOKE=1 to run the site smoke tests (they use the network)")
	}
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

func smokeLoad(t *testing.T, b *browser.Browser, url string) *driver {
	t.Helper()
	d := newDriver(t, New(b, url))
	t.Cleanup(d.m.Close)
	d.send(tea.WindowSizeMsg{Width: 120, Height: 40})
	d.until(url, func() bool {
		p := d.page()
		return p != nil && !p.loading && (p.root != nil || p.errText != "")
	})
	if p := d.page(); p.errText != "" {
		t.Fatalf("%s: %s", url, p.errText)
	}
	// A page keeps arriving after its load event; let the settle captures
	// land before looking.
	d.pumpFor(2 * time.Second)
	p := d.page()
	t.Logf("%s — %q", p.url, p.title)
	lines := strings.Split(ir.Dump(p.root), "\n")
	t.Logf("IR head:\n%s", strings.Join(lines[:min(40, len(lines))], "\n"))
	return d
}

// pumpFor keeps the model updated for a while, for pages that arrive in
// pieces.
func (d *driver) pumpFor(dur time.Duration) {
	deadline := time.After(dur)
	for {
		select {
		case msg := <-d.msgs:
			d.send(msg)
		case <-deadline:
			return
		}
	}
}

func countKind(l layout, k ir.Kind) int {
	n := 0
	for _, it := range l.items {
		if it.node.Kind == k {
			n++
		}
	}
	return n
}

func head(l layout, n int) string {
	var b strings.Builder
	for i, r := range l.rows {
		if i >= n {
			break
		}
		b.WriteString(r.plain())
		b.WriteString("\n")
	}
	return b.String()
}

func TestSmokeHackerNews(t *testing.T) {
	b := smokeBrowser(t)
	d := smokeLoad(t, b, "https://news.ycombinator.com/")
	l := d.page().lay
	t.Logf("%d rows, %d items, %d links\n%s", len(l.rows), len(l.items), countKind(l, ir.Link), head(l, 40))
	if countKind(l, ir.Link) < 60 {
		t.Errorf("a front page has 30 stories with several links each; got %d links", countKind(l, ir.Link))
	}
	if countKind(l, ir.Textbox) == 0 {
		t.Errorf("the search box at the bottom is missing")
	}

	// Enter on the first story opens its menu; Enter again follows the
	// link — the click works on a real page, not only on the fixtures.
	story := -1
	for i, it := range l.items {
		if it.node.Kind == ir.Link && strings.HasPrefix(it.node.URL, "http") &&
			!strings.Contains(it.node.URL, "ycombinator.com") {
			story = i
			break
		}
	}
	if story < 0 {
		t.Fatal("no external story link on the front page")
	}
	d.page().cursor = story
	target := l.items[story].node.URL
	d.act()
	d.until("the story loads", func() bool {
		p := d.page()
		return !p.loading && p.root != nil && !strings.Contains(p.url, "ycombinator.com")
	})
	t.Logf("followed %s → %s — %q", target, d.page().url, d.page().title)
}

func TestSmokeGitHub(t *testing.T) {
	b := smokeBrowser(t)
	d := smokeLoad(t, b, "https://github.com/chromedp/chromedp")
	l := d.page().lay
	t.Logf("%d rows, %d items, %d links\n%s", len(l.rows), len(l.items), countKind(l, ir.Link), head(l, 60))
	if countKind(l, ir.Link) < 20 || countKind(l, ir.Button) == 0 {
		t.Errorf("a repository page has dozens of links and buttons; got %d links, %d buttons",
			countKind(l, ir.Link), countKind(l, ir.Button))
	}
	found := false
	d.page().root.Walk(func(n *ir.Node) bool {
		if n.Kind == ir.Link && n.Name == "chromedp" && strings.HasSuffix(n.URL, "/chromedp/chromedp") {
			found = true
		}
		return !found
	})
	if !found {
		t.Errorf("no link naming the repository")
	}
}

// TestSmokeSections is the section cut against real pages (section.go): a
// document has to come out as a list of its headings, and one of them has to
// open to the panel on its own. It prints both so the shape of the screen can
// be judged by eye.
func TestSmokeSections(t *testing.T) {
	b := smokeBrowser(t)
	for _, url := range []string{
		"https://www.w3schools.com/html/html_tables.asp",
		"https://go.dev/doc/tutorial/getting-started",
		"https://developer.mozilla.org/en-US/docs/Web/HTML/Element/table",
		"https://en.wikipedia.org/wiki/Terminal_emulator",
		"https://pkg.go.dev/strings",
		"https://news.ycombinator.com/",
	} {
		t.Run(url, func(t *testing.T) {
			d := smokeLoad(t, b, url)
			p := d.page()
			t.Logf("shape=%d sections=%d listing=%v", p.shape, len(p.secs), p.listing())
			// Every heading in scope has to reach the list, minus the
			// navigation columns pruneNav takes out: a heading that is
			// drawn but never cut on is a bug between the two.
			scope := mainOf(p.root)
			drawn := 0
			p.root.Walk(func(n *ir.Node) bool {
				if n.Kind == ir.Heading {
					if _, ok := p.lay.marks[n]; ok && (scope == nil || holds(scope, n)) {
						drawn++
					}
				}
				return true
			})
			t.Logf("headings: %d drawn in scope (main=%v), %d sections", drawn, scope != nil, len(p.secs))
			for i, s := range p.secs {
				t.Logf("  %2d  h%d  %-46s %3d lines  %2d prose %2d link  %dt %dc %dm",
					i+1, s.level, truncate(s.title, 46), s.lines(), s.prose, s.links, s.tables, s.codes, s.media)
			}
			t.Logf("SCREEN\n%s", d.m.View())
			if !p.listing() {
				return
			}
			d.key("j")
			d.key("enter")
			t.Logf("OPENED %q\n%s", p.secs[p.sec].title, d.m.View())
		})
	}
}
