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
