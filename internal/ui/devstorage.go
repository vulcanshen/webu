package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/chromedp/cdproto/network"
	"github.com/vulcanshen/webu/internal/page"
)

// The Storage tab (ui.md §3.2): three sections — cookies, local storage,
// session storage — each a small table. The cursor walks the entries and
// skips the section headers.

// storageMsg is a fetch landing.
type storageMsg struct {
	tabID int
	data  page.StorageData
	err   error
}

type devStorageTab struct {
	data   page.StorageData
	cursor int // index into rows(filter)'s entries
	top    int
	err    string
}

// storageRow is one drawn row: a section header, or an entry.
type storageRow struct {
	header string
	cookie *network.Cookie
	key    string
	value  string
	local  bool // for a web-storage entry: which store
	entry  bool
}

func (t devStorageTab) rows(filter string) []storageRow {
	var out []storageRow
	var cookies []storageRow
	for _, c := range t.data.Cookies {
		if devMatches(filter, c.Name+" "+c.Value+" "+c.Domain) {
			cookies = append(cookies, storageRow{cookie: c, key: c.Name, value: c.Value, entry: true})
		}
	}
	out = append(out, storageRow{header: "cookies · " + itoa(len(cookies))})
	out = append(out, cookies...)
	for _, store := range []struct {
		name  string
		items [][2]string
		local bool
	}{{"local storage", t.data.Local, true}, {"session storage", t.data.Session, false}} {
		var entries []storageRow
		for _, kv := range store.items {
			if devMatches(filter, kv[0]+" "+kv[1]) {
				entries = append(entries, storageRow{key: kv[0], value: kv[1], local: store.local, entry: true})
			}
		}
		out = append(out, storageRow{header: store.name + " · " + itoa(len(entries))})
		out = append(out, entries...)
	}
	return out
}

// entries is rows without the headers, which is what the cursor counts.
func (t devStorageTab) entries(filter string) []storageRow {
	var out []storageRow
	for _, r := range t.rows(filter) {
		if r.entry {
			out = append(out, r)
		}
	}
	return out
}

func (t devStorageTab) current(filter string) (storageRow, bool) {
	e := t.entries(filter)
	if t.cursor < 0 || t.cursor >= len(e) {
		return storageRow{}, false
	}
	return e[t.cursor], true
}

func (t *devStorageTab) move(k string, page int, filter string) {
	t.cursor = moveCursor(t.cursor, len(t.entries(filter)), k, page)
}

func (t *devStorageTab) set(d page.StorageData, err error) {
	t.data = d
	t.err = ""
	if err != nil {
		t.err = err.Error()
	}
	t.cursor = clamp(t.cursor, 0, max(0, len(t.entries(""))-1))
}

func (t devStorageTab) view(innerW, n int, filter string) []string {
	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)
	red := lipgloss.NewStyle().Foreground(warnColor)

	out := []string{dim.Render(padRight(" "+nameOr(t.data.Origin, "no origin (a file:// page has none)"), innerW))}
	if t.err != "" {
		out = append(out, red.Render(padRight(" "+t.err, innerW)))
	}
	rows := t.rows(filter)
	// Which drawn row holds the cursor, to scroll it into view.
	curRow, ei := -1, 0
	for i, r := range rows {
		if r.entry {
			if ei == t.cursor {
				curRow = i
			}
			ei++
		}
	}
	room := n - len(out)
	top := t.top
	if curRow >= 0 {
		top = scrollTo(top, curRow, room)
	}
	top = clamp(top, 0, max(0, len(rows)-1))
	keyW := 12
	for _, r := range rows {
		if r.entry {
			keyW = max(keyW, min(dispW(r.key), innerW/3))
		}
	}
	for i := top; i < len(rows) && len(out) < n; i++ {
		r := rows[i]
		if !r.entry {
			out = append(out, dim.Render(padRight(" "+r.header, innerW)))
			continue
		}
		line := " " + padRight(r.key, keyW) + "  "
		if r.cookie != nil {
			c := r.cookie
			exp := "session"
			if !c.Session && c.Expires > 0 {
				exp = time.Unix(int64(c.Expires), 0).Local().Format("2006-01-02")
			}
			var flags []string
			if c.HTTPOnly {
				flags = append(flags, "HttpOnly")
			}
			if c.Secure {
				flags = append(flags, "Secure")
			}
			if c.SameSite != "" {
				flags = append(flags, string(c.SameSite))
			}
			meta := "  " + c.Domain + "  " + c.Path + "  " + exp + "  " + strings.Join(flags, " ")
			valW := max(8, innerW-dispW(line)-dispW(meta))
			line += padRight(oneLine(c.Value), valW) + meta
		} else {
			line += oneLine(r.value)
		}
		if i == curRow {
			out = append(out, cur.Render(padRight(line, innerW)))
		} else {
			out = append(out, txt.Render(padRight(line, innerW)))
		}
	}
	return out
}
