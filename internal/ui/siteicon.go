package ui

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/reader"
)

// A tab's row in [1] starts with its site's mark (user, 2026-10-28): the
// site's own Nerd Font icon when it has one, a globe otherwise, in the
// colour of the site's favicon — a terminal cannot draw the favicon
// itself — and a Blue globe when neither is known.

// glyphGlobe is nf-fa-globe, the mark of a site with no icon of its own.
var glyphGlobe = string(rune(0xf0ac))

// siteGlyphs are the sites Nerd Fonts has an icon for, by the end of
// their host name.
var siteGlyphs = []struct {
	host  string
	glyph rune
}{
	{"github.com", 0xf09b},        // nf-fa-github
	{"gitlab.com", 0xf296},        // nf-fa-gitlab
	{"wikipedia.org", 0xf266},     // nf-fa-wikipedia_w
	{"wikimedia.org", 0xf266},     // nf-fa-wikipedia_w
	{"youtube.com", 0xf167},       // nf-fa-youtube
	{"youtu.be", 0xf167},          // nf-fa-youtube
	{"reddit.com", 0xf1a1},        // nf-fa-reddit
	{"stackoverflow.com", 0xf16c}, // nf-fa-stack_overflow
	{"stackexchange.com", 0xf18d}, // nf-fa-stack_exchange
	{"twitter.com", 0xf099},       // nf-fa-twitter
	{"x.com", 0xf099},             // nf-fa-twitter
	{"google.com", 0xf1a0},        // nf-fa-google
	{"ycombinator.com", 0xf1d4},   // nf-fa-hacker_news
	{"medium.com", 0xf23a},        // nf-fa-medium
	{"linkedin.com", 0xf0e1},      // nf-fa-linkedin
	{"facebook.com", 0xf09a},      // nf-fa-facebook
	{"amazon.com", 0xf270},        // nf-fa-amazon
	{"apple.com", 0xf179},         // nf-fa-apple
	{"mozilla.org", 0xf269},       // nf-fa-firefox
	{"twitch.tv", 0xf1e8},         // nf-fa-twitch
	{"go.dev", 0xe627},            // nf-seti-go
	{"golang.org", 0xe627},        // nf-seti-go
	{"python.org", 0xe73c},        // nf-dev-python
	{"rust-lang.org", 0xe7a8},     // nf-dev-rust
	{"npmjs.com", 0xe71e},         // nf-dev-npm
	{"docker.com", 0xe7b0},        // nf-dev-docker
}

// siteGlyph is the icon for host: its own, else the globe.
func siteGlyph(host string) string {
	host = strings.ToLower(host)
	for _, s := range siteGlyphs {
		if host == s.host || strings.HasSuffix(host, "."+s.host) {
			return string(s.glyph)
		}
	}
	return glyphGlobe
}

// siteMark is what a tab's row starts with, and its colour.
func (m *AppModel) siteMark(addr string) (string, lipgloss.Color) {
	host := ""
	if u, err := url.Parse(addr); err == nil {
		host = u.Hostname()
	}
	color := lipgloss.Color(blueHex)
	if c, ok := m.favColor[host]; ok {
		color = c
	}
	return siteGlyph(host), color
}

// faviconMsg is a site's icon colour, come.
type faviconMsg struct {
	host  string
	color lipgloss.Color
}

// fetchFavicon asks once, for a page's site, what colour its icon is: the
// icon the page names, else /favicon.ico.
func (m *AppModel) fetchFavicon(d *reader.Doc) tea.Cmd {
	host := d.URL.Hostname()
	if host == "" || m.favTried[host] {
		return nil
	}
	m.favTried[host] = true
	icon := d.Icon
	if icon == "" {
		icon = d.URL.Scheme + "://" + d.URL.Host + "/favicon.ico"
	}
	rc := m.rc
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r, g, b, ok := rc.IconColor(ctx, icon)
		if !ok {
			return nil
		}
		return faviconMsg{host: host, color: lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))}
	}
}
