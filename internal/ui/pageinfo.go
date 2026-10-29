package ui

import (
	"net/http"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/reader"
)

// What is left of DevTools with no Chromium (user, 2026-10-29): the
// page's source, its site's cookies and the requests that loaded it, each
// a viewer opened from the top of [2]'s Space menu. The page's own console
// and storage went with its JavaScript, and the requests for its styles,
// scripts and images with the browser that made them.

const (
	glyphCookie   = "\U000f0198" // nf-md-cookie
	glyphRequests = "\uf0ec"     // nf-fa-exchange
)

// showSource is the page as it came.
func (m *AppModel) showSource() tea.Cmd {
	t := m.shownTab()
	m.did("Read the source of " + tabName(t))
	return m.a.Viewer(&terminu.Viewer{Title: "Source", Subject: t.doc.URL.String(), Glyph: glyphCode,
		Lines: strings.Split(t.doc.Source, "\n"), Lang: t.doc.Type, Select: selection()})
}

// showCookies is the cookies of the page's site, C clearing them.
func (m *AppModel) showCookies() tea.Cmd {
	host := m.shownTab().doc.URL.Hostname()
	site := reader.Site(host)
	m.did("Read the cookies of " + site)
	cookies := m.rc.Jar.SiteCookies(host)
	v := &terminu.Viewer{Title: "Cookies", Subject: site, Glyph: glyphCookie, Lines: cookieLines(site, cookies), Select: selection()}
	if len(cookies) > 0 {
		v.Keys = []terminu.ViewerKey{{Key: "C", Desc: "clear", Do: func() tea.Cmd {
			return m.a.Confirm(terminu.Ask{Title: "Clear cookies", Subject: site,
				Details:  []string{"Signed out of " + site + ", every page of it."},
				Question: "Clear " + plural(len(cookies), "cookie") + "?", Verb: "clear", Warn: []int{0}}, func() tea.Cmd {
				n, err := m.rc.Jar.ClearSite(host)
				if err != nil {
					return m.a.Error("Clear failed", err.Error())
				}
				v.Lines, v.Keys = cookieLines(site, nil), nil
				return m.toastOK("Cleared " + plural(n, "cookie") + " of " + site)
			})
		}}}
	}
	return m.a.Viewer(v)
}

// cookieLines are the cookies, a name=value line each, what it is for
// under it.
func cookieLines(site string, cookies []*http.Cookie) []string {
	if len(cookies) == 0 {
		return []string{"No cookies for " + site}
	}
	var out []string
	for i, c := range cookies {
		if i > 0 {
			out = append(out, "")
		}
		expires := "session"
		if !c.Expires.IsZero() {
			expires = "until " + c.Expires.Local().Format("2006-01-02 15:04")
		}
		about := []string{c.Domain, "path " + c.Path, expires}
		if c.Secure {
			about = append(about, "Secure")
		}
		if c.HttpOnly {
			about = append(about, "HttpOnly")
		}
		out = append(out, c.Name+"="+c.Value, "  "+strings.Join(about, " · "))
	}
	return out
}

// showRequests is what loading the page asked for, as curl -v writes it:
// > the headers sent, < the ones that came back.
func (m *AppModel) showRequests() tea.Cmd {
	t := m.shownTab()
	m.did("Read the requests of " + tabName(t))
	var lines []string
	for i, e := range t.doc.Requests {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, e.Method+" "+e.URL)
		if e.Err != "" {
			lines = append(lines, "  failed: "+e.Err)
		} else {
			lines = append(lines, "  "+e.Status+" · "+e.Took.Round(time.Millisecond).String()+" · "+humanBytes(float64(e.Size)))
		}
		lines = append(lines, headers("> ", e.Request)...)
		lines = append(lines, headers("< ", e.Response)...)
	}
	return m.a.Viewer(&terminu.Viewer{Title: "Requests", Subject: t.doc.URL.String(), Glyph: glyphRequests,
		Lines: lines, Select: selection()})
}

// headers are h's, a line a value, by name.
func headers(prefix string, h http.Header) []string {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	sort.Strings(names)
	var out []string
	for _, k := range names {
		for _, v := range h[k] {
			out = append(out, "  "+prefix+k+": "+v)
		}
	}
	return out
}
