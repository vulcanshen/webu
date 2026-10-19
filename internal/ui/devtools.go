package ui

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chromedp/cdproto/network"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/page"
)

// DevTools (ui.md §3) is a screen of its own: what the page the web screen
// shows has fetched, keeps, logged and is made of. [1] is four lists on
// one panel's tabs — Network, Storage, Console, Source — each with its
// filter; [2] is the row under [1]'s cursor in full (layout/panel,
// details).
//
// It was a popup with a tab bar of its own, a cursor list and a detail
// over it; the lib has no popup like that, and a screen gives every list a
// filter, a details panel and the whole terminal.

const (
	devNetwork = iota
	devStorage
	devConsole
	devSource
)

// devTickMsg redraws the lists while DevTools is up: the network and the
// console keep arriving.
type devTickMsg struct{}

func devTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return devTickMsg{} })
}

// storageMsg, bodyMsg, devSourceMsg and evalMsg are Chromium's answers to
// what DevTools asked for.
type storageMsg struct {
	tabID int
	data  page.StorageData
	err   error
}

type bodyMsg struct {
	tabID int
	id    string
	body  string
	err   error
}

type devSourceMsg struct {
	tabID int
	html  string
	err   error
}

type evalMsg struct {
	tabID int
	entry page.ConsoleEntry
}

// devtools is the screen's state.
type devtools struct {
	screen  *terminu.Screen
	panel   *terminu.Panel
	detail  *terminu.Panel
	pager   *terminu.Pager
	lists   [4]*terminu.List
	tabID   int // the tab the lists are of
	storage page.StorageData
	source  []string
	net     []page.NetEntry
	console []page.ConsoleEntry
	// bodyFor is the request whose body the detail is waiting on.
	bodyFor string
}

func (m *AppModel) buildDevtools() {
	d := &devtools{pager: &terminu.Pager{EmptyFact: "Nothing under the cursor", EmptyHint: "Press [1], then [j] or [k], to show a row here"}}
	m.dev = d
	facts := [4][2]string{
		{"No requests yet", "Press [W] to browse, or [Space] to see what you can do here"},
		{"No storage", "Press [R] to read it again"},
		{"No console lines yet", "Press [i] to run a line of JavaScript, or [Space] for more"},
		{"No source", "Press [R] to read it again"},
	}
	cols := [4][]terminu.Column{
		{{Title: "Method"}, {Title: "Status", Number: true}, {Title: "Type"}, {Title: "Size", Number: true}, {Title: "URL", Flex: true}},
		{{Title: "Kind"}, {Title: "Name"}, {Title: "Value", Flex: true}},
		{{Title: "Level"}, {Title: "Text", Flex: true}},
		{{Title: "Line", Number: true}, {Title: "Source", Flex: true}},
	}
	for i := range d.lists {
		i := i
		d.lists[i] = &terminu.List{Columns: cols[i], Filterable: true, EmptyFact: facts[i][0], EmptyHint: facts[i][1],
			OnMove: func(r terminu.Row) tea.Cmd { return m.devMoved(i, r) }}
	}
	d.detail = &terminu.Panel{Label: "Detail", Body: d.pager}
	// A text with no items: Enter is the panel's own, the whole of it in a
	// viewer to scroll, select and copy (tdp K3).
	d.detail.Actions = []terminu.Action{{Key: "Enter", Label: "Read", Desc: "the detail in full, to scroll, select and copy",
		Scope: terminu.PanelScope, Hint: "read", Disabled: func() bool { return d.pager.Empty() },
		Do: func() tea.Cmd {
			return m.a.Viewer(&terminu.Viewer{Title: "Detail", Glyph: glyphDevTools, Lines: d.pager.Lines(),
				Select: selection()})
		}}}
	d.panel = &terminu.Panel{Label: "Network", Tabs: []string{"Storage", "Console", "Source"}, Body: d.lists[devNetwork],
		Details: d.detail, OnTab: m.devTab, Stream: true}
	d.panel.Actions = m.devActions()
	d.screen = m.a.AddScreen(&terminu.Screen{Name: "DevTools", Key: "I", Desc: "the shown page's network, storage, console and source",
		Panels: []*terminu.Panel{d.panel, d.detail},
		Layout: func(w, h int) []terminu.Rect {
			left := w * 3 / 5
			return []terminu.Rect{{X: 0, Y: 0, W: left, H: h}, {X: left, Y: 0, W: w - left, H: h}}
		},
		NarrowBelow: narrowW})
}

// devOn reports whether DevTools is the screen up.
func (m *AppModel) devOn() bool { return m.a.Screen() == m.dev.screen }

// devList is the list on [1]'s current tab.
func (m *AppModel) devList() *terminu.List { return m.dev.lists[m.dev.panel.Tab] }

// devTab is h or l on [1]: another list, read again when it is one that
// is only read on asking.
func (m *AppModel) devTab(i int) tea.Cmd {
	d := m.dev
	d.panel.Body = d.lists[i]
	d.panel.Stream = i == devNetwork || i == devConsole
	d.pager.SetLines(nil)
	d.panel.Fail = nil
	var fetch, detail tea.Cmd
	switch i {
	case devStorage:
		fetch = m.fetchStorage()
	case devSource:
		fetch = m.fetchSource()
	}
	// [2] shows the row under the cursor of the list now shown.
	if r, ok := d.lists[i].Current(); ok {
		detail = m.devMoved(i, r)
	}
	return tea.Batch(fetch, detail)
}

// devMsg is one of DevTools' messages.
func (m *AppModel) devMsg(msg tea.Msg) tea.Cmd {
	d := m.dev
	t := m.shownTab()
	switch msg := msg.(type) {
	case devTickMsg:
		if m.devOn() {
			m.refreshDev()
		}
		return devTick()
	case storageMsg:
		if t != nil && msg.tabID == t.id {
			d.panel.Loading = false
			if msg.err != nil {
				d.panel.Fail = &terminu.Failure{Fact: "Can't read the storage: " + msg.err.Error(), Hint: "Press [R] to try again"}
				return m.toastErr("Storage: " + msg.err.Error())
			}
			d.panel.Fail = nil
			d.storage = msg.data
			m.refreshDev()
		}
	case devSourceMsg:
		if t != nil && msg.tabID == t.id {
			d.panel.Loading = false
			if msg.err != nil {
				d.panel.Fail = &terminu.Failure{Fact: "Can't read the source: " + msg.err.Error(), Hint: "Press [R] to try again"}
				return m.toastErr("Source: " + msg.err.Error())
			}
			d.panel.Fail = nil
			d.source = strings.Split(strings.ReplaceAll(msg.html, "\r\n", "\n"), "\n")
			m.refreshDev()
		}
	case bodyMsg:
		if t != nil && msg.tabID == t.id && msg.id == d.bodyFor {
			d.detail.Loading = false
			lines := d.pager.Lines()
			switch {
			case msg.err != nil:
				lines = append(lines, terminu.Fg(terminu.Red).Render("The body could not be had: "+msg.err.Error()))
			default:
				e, _ := m.netEntry(msg.id)
				if text, ok := textBody(msg.body, e.Mime); ok {
					lines = append(lines, strings.Split(strings.ReplaceAll(text, "\t", "    "), "\n")...)
				} else {
					lines = append(lines, terminu.Fg(terminu.Overlay0).Render(humanBytes(float64(len(msg.body)))+" of "+nameOr(e.Mime, "binary")))
				}
			}
			d.pager.SetLines(lines)
		}
	case evalMsg:
		_, et := m.tabByID(msg.tabID)
		if et == nil {
			return nil
		}
		et.dev.Add(msg.entry)
		m.refreshDev()
	}
	return nil
}

// refreshDev fills the lists from the shown tab.
func (m *AppModel) refreshDev() {
	d := m.dev
	t := m.shownTab()
	if t == nil {
		for _, l := range d.lists {
			l.SetRows(nil)
		}
		return
	}
	if d.tabID != t.id {
		d.tabID = t.id
		d.storage, d.source = page.StorageData{}, nil
	}
	d.net = t.dev.Net()
	d.console = t.dev.Console()
	var rows []terminu.Row
	for _, e := range d.net {
		status := ""
		switch {
		case e.Error != "":
			status = terminu.Fg(terminu.Red).Render("failed")
		case e.Status > 0:
			status = strconv.FormatInt(e.Status, 10)
		default:
			status = terminu.Fg(terminu.Overlay0).Render("…")
		}
		rows = append(rows, terminu.Row{ID: string(e.ID), Cells: []string{e.Method, status, e.Type, humanBytes(e.Size), oneLine(e.URL)},
			Sort: []string{e.Method, strconv.FormatInt(e.Status, 10), e.Type, strconv.FormatFloat(e.Size, 'f', 0, 64), e.URL}})
	}
	d.lists[devNetwork].SetRows(rows)

	rows = nil
	for i, c := range d.storage.Cookies {
		rows = append(rows, terminu.Row{ID: "c" + strconv.Itoa(i), Cells: []string{"cookie", oneLine(c.Name), oneLine(c.Value)}})
	}
	for i, kv := range d.storage.Local {
		rows = append(rows, terminu.Row{ID: "l" + strconv.Itoa(i), Cells: []string{"local", oneLine(kv[0]), oneLine(kv[1])}})
	}
	for i, kv := range d.storage.Session {
		rows = append(rows, terminu.Row{ID: "s" + strconv.Itoa(i), Cells: []string{"session", oneLine(kv[0]), oneLine(kv[1])}})
	}
	d.lists[devStorage].SetRows(rows)

	rows = nil
	for i, c := range d.console {
		level := c.Level
		switch c.Level {
		case "error":
			level = terminu.Fg(terminu.Red).Render(level)
		case "warning":
			level = terminu.Fg(terminu.Peach).Render(level)
		}
		rows = append(rows, terminu.Row{ID: strconv.Itoa(i), Cells: []string{level, oneLine(c.Text)}})
	}
	d.lists[devConsole].SetRows(rows)

	rows = nil
	for i, l := range d.source {
		rows = append(rows, terminu.Row{ID: strconv.Itoa(i), Cells: []string{strconv.Itoa(i + 1), strings.ReplaceAll(l, "\t", "    ")}})
	}
	d.lists[devSource].SetRows(rows)
}

// netEntry is the request with this id.
func (m *AppModel) netEntry(id string) (page.NetEntry, bool) {
	for _, e := range m.dev.net {
		if string(e.ID) == id {
			return e, true
		}
	}
	return page.NetEntry{}, false
}

// storageRow is the storage row with this id: a cookie, or a local or a
// session item.
func (m *AppModel) storageRow(id string) (cookie *network.Cookie, local bool, key, value string, ok bool) {
	if id == "" {
		return nil, false, "", "", false
	}
	i, err := strconv.Atoi(id[1:])
	if err != nil {
		return nil, false, "", "", false
	}
	s := m.dev.storage
	switch id[0] {
	case 'c':
		if i < len(s.Cookies) {
			return s.Cookies[i], false, s.Cookies[i].Name, s.Cookies[i].Value, true
		}
	case 'l':
		if i < len(s.Local) {
			return nil, true, s.Local[i][0], s.Local[i][1], true
		}
	case 's':
		if i < len(s.Session) {
			return nil, false, s.Session[i][0], s.Session[i][1], true
		}
	}
	return nil, false, "", "", false
}

// devMoved is [1]'s cursor reaching a row: [2] shows it in full.
func (m *AppModel) devMoved(list int, r terminu.Row) tea.Cmd {
	d := m.dev
	if list != d.panel.Tab {
		return nil
	}
	d.detail.Loading = false
	switch list {
	case devNetwork:
		e, ok := m.netEntry(r.ID)
		if !ok {
			d.pager.SetLines(nil)
			return nil
		}
		lines := []string{e.Method + " " + e.URL, "status   " + strconv.FormatInt(e.Status, 10), "type     " + e.Type + "  " + e.Mime,
			"size     " + humanBytes(e.Size), "time     " + e.Duration.Round(time.Millisecond).String()}
		if e.Error != "" {
			lines = append(lines, terminu.Fg(terminu.Red).Render("error    "+e.Error))
		}
		lines = append(lines, "", "REQUEST HEADERS")
		lines = append(lines, headerLines(e.ReqHdr)...)
		lines = append(lines, "", "RESPONSE HEADERS")
		lines = append(lines, headerLines(e.RespHdr)...)
		lines = append(lines, "", "BODY")
		d.pager.SetLines(lines)
		t := m.shownTab()
		if t == nil || !e.Done || e.Error != "" {
			return nil
		}
		d.bodyFor, d.detail.Loading = r.ID, true
		ctx, id, reqID := t.ctx, t.id, e.ID
		return func() tea.Msg {
			body, err := page.ResponseBody(ctx, reqID)
			return bodyMsg{tabID: id, id: string(reqID), body: body, err: err}
		}
	case devStorage:
		_, _, key, value, ok := m.storageRow(r.ID)
		if !ok {
			d.pager.SetLines(nil)
			return nil
		}
		d.pager.SetLines(append([]string{key, ""}, strings.Split(value, "\n")...))
	case devConsole:
		i, err := strconv.Atoi(r.ID)
		if err != nil || i >= len(d.console) {
			d.pager.SetLines(nil)
			return nil
		}
		e := d.console[i]
		text := e.Text
		if e.Detail != "" {
			text = e.Detail // the whole object, not its one-line preview
		}
		head := []string{"level    " + e.Level, "at       " + e.At.Local().Format("15:04:05")}
		if e.Where != "" {
			head = append(head, "from     "+e.Where)
		}
		d.pager.SetLines(append(append(head, ""), strings.Split(text, "\n")...))
	case devSource:
		d.pager.SetLines(strings.Split(r.Cells[len(r.Cells)-1], "\n"))
	}
	return nil
}

// devActions are DevTools [1]'s: what each tab's list can do.
func (m *AppModel) devActions() []terminu.Action {
	d := m.dev
	on := func(tabs ...int) func() bool {
		return func() bool {
			for _, t := range tabs {
				if d.panel.Tab == t {
					return false
				}
			}
			return true
		}
	}
	noPage := func() bool { return m.shownTab() == nil }
	return []terminu.Action{
		// No hint: [1] streams, and the lib keeps a streaming panel's hint
		// lit off focus too, beside [2]'s own Enter (tdp M9; TODO(lib
		// WB13)). The Space menu and ? still list it.
		{Key: "Enter", Label: "Detail", Desc: "the row in full, in [2]", Scope: terminu.ItemScope,
			ToDetails: func() bool { return true }},
		{Key: "y", Label: "yank", Desc: "the URL, the value, the line", Scope: terminu.ItemScope,
			Do: func() tea.Cmd {
				r, ok := m.devList().Current()
				if !ok {
					return nil
				}
				switch d.panel.Tab {
				case devNetwork:
					e, _ := m.netEntry(r.ID)
					return copyToClipboard(e.URL)
				case devStorage:
					_, _, _, v, _ := m.storageRow(r.ID)
					return copyToClipboard(v)
				case devConsole:
					i, _ := strconv.Atoi(r.ID)
					if i < len(d.console) {
						return copyToClipboard(d.console[i].Text)
					}
					return nil
				}
				return copyToClipboard(r.Cells[len(r.Cells)-1])
			}},
		{Key: "x", Label: "Delete", Desc: "this cookie or storage item", Scope: terminu.ItemScope, Hidden: on(devStorage),
			Do: m.do(m.deleteStorageRow)},
		{Key: "i", Label: "input JavaScript", Desc: "a line, in the page; its answer comes into the console", Scope: terminu.PanelScope,
			Hint: "run", Hidden: on(devConsole), Disabled: noPage, Do: m.evalBox},
		{Key: "R", Label: "Read again", Desc: "the storage, or the source, as it is now", Scope: terminu.PanelScope,
			Hidden: on(devStorage, devSource), Disabled: noPage,
			Do: m.do(func() tea.Cmd {
				if d.panel.Tab == devStorage {
					return m.fetchStorage()
				}
				return m.fetchSource()
			})},
		{Key: "C", Label: "Clear", Desc: "the list: the requests, the console lines", Scope: terminu.PanelScope,
			Hidden: on(devNetwork, devConsole), Disabled: noPage, Do: m.do(m.devClear)},
		{Key: "C", Label: "Clear site data", Desc: "cookies, storage, caches: everything the site keeps", Scope: terminu.PanelScope,
			Hidden: on(devStorage), Disabled: func() bool { return noPage() || d.storage.Origin == "" },
			Ask: &terminu.Ask{Title: "Clear site data", Details: []string{"Cookies, storage, caches — everything Chromium keeps for the site.", "Logins there will be gone."},
				Question: "Clear everything this site keeps?", Verb: "clear", Warn: []int{1}},
			Do: m.do(func() tea.Cmd {
				t, origin := m.shownTab(), d.storage.Origin
				if t == nil || origin == "" {
					return nil
				}
				return m.storageThen(t, "Clear site data", func(ctx context.Context) error { return page.ClearSiteData(ctx, origin) })
			})},
	}
}

// fetchStorage reads the shown page's cookies and storage.
func (m *AppModel) fetchStorage() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	return m.storageThen(t, "", nil)
}

// storageThen runs fn — what names it — on the tab and then re-reads the
// storage, in ONE command: a fetch sequenced after it would race the
// change it is meant to show. When fn fails the storage is still read
// again, so that the list stops saying it is reading.
func (m *AppModel) storageThen(t *tab, what string, fn func(context.Context) error) tea.Cmd {
	m.dev.panel.Loading, m.dev.panel.LoadingText = true, "reading the storage…"
	ctx, url, id := t.ctx, t.url, t.id
	return func() tea.Msg {
		var failed error
		if fn != nil {
			failed = fn(ctx)
		}
		d, err := page.StorageFor(ctx, url)
		read := storageMsg{tabID: id, data: d, err: err}
		if failed != nil {
			return tea.BatchMsg{
				func() tea.Msg { return read },
				func() tea.Msg { return actionErrMsg{what: what, err: failed} },
			}
		}
		return read
	}
}

// fetchSource reads the shown page's HTML.
func (m *AppModel) fetchSource() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	m.dev.panel.Loading, m.dev.panel.LoadingText = true, "reading the source…"
	ctx, id := t.ctx, t.id
	return func() tea.Msg {
		html, err := page.Source(ctx)
		return devSourceMsg{tabID: id, html: html, err: err}
	}
}

// deleteStorageRow is x on a storage row.
func (m *AppModel) deleteStorageRow() tea.Cmd {
	t := m.shownTab()
	r, ok := m.devList().Current()
	if t == nil || !ok {
		return nil
	}
	cookie, local, key, _, ok := m.storageRow(r.ID)
	if !ok {
		return nil
	}
	if cookie != nil {
		return m.storageThen(t, "Delete cookie", func(ctx context.Context) error { return page.DeleteCookie(ctx, cookie) })
	}
	origin := m.dev.storage.Origin
	return m.storageThen(t, "Delete item", func(ctx context.Context) error { return page.RemoveStorageItem(ctx, origin, local, key) })
}

// devClear is C: the network or console log, or the site's data, which
// asks first.
func (m *AppModel) devClear() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	switch m.dev.panel.Tab {
	case devNetwork:
		t.dev.ClearNet()
	case devConsole:
		t.dev.ClearConsole()
	}
	m.refreshDev()
	return nil
}

// evalBox is the console's prompt: a line of JavaScript, run in the page;
// the line and its answer go to the console list.
func (m *AppModel) evalBox() tea.Cmd {
	return m.a.Text(terminu.TextInput{Title: "Run JavaScript", Glyph: glyphDevTools, Name: "The line",
		Prompt: "One line, run in the page", Verb: "run",
		Done: func(v string) tea.Cmd {
			t := m.shownTab()
			expr := strings.TrimSpace(v)
			if t == nil || expr == "" {
				return nil
			}
			t.dev.Add(page.ConsoleEntry{Level: "input", Text: expr})
			m.refreshDev()
			ctx, id := t.ctx, t.id
			return func() tea.Msg { return evalMsg{tabID: id, entry: page.Eval(ctx, expr)} }
		}})
}

// textBody is a response body as text to read, when it is text.
func textBody(body, mime string) (string, bool) {
	if !textMime(mime) || strings.ContainsRune(body, 0) || !utf8.ValidString(body) {
		return "", false
	}
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return -1
	}, body), true
}

// textMime reports a type whose body reads as text.
func textMime(mime string) bool {
	m := strings.ToLower(mime)
	if m == "" || strings.HasPrefix(m, "text/") {
		return true
	}
	for _, s := range []string{"json", "javascript", "ecmascript", "xml", "svg", "x-www-form-urlencoded", "graphql", "yaml", "csv"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// headerLines are a request's or a response's headers, sorted, one a line.
func headerLines(h map[string]string) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.Map(func(r rune) rune {
			if r == '\t' || (r >= 0x20 && r != 0x7f) {
				return r
			}
			return ' '
		}, strings.ToValidUTF8(h[k], "�"))
		out = append(out, "  "+k+": "+v)
	}
	if len(out) == 0 {
		out = append(out, "  (none)")
	}
	return out
}
