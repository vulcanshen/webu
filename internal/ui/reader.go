package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/reader"
)

// The reader (docs/reader.md): every tab's pages come over net/http, the
// content picked out of the site around it and drawn as webu's own
// elements (readerrender.go).

// readerUA is what the reader calls itself: a browser, and webu.
const readerUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 webu"

// place is where the reader was on a page of a tab's history, for going
// back to it.
type readerPlace struct {
	url         string
	top, cursor int
}

// readerMsg is a page the reader fetched, or the download it turned out
// to be, or why neither.
type readerMsg struct {
	tabID, gen int
	url        string
	// to is the history entry the page goes to: -1 a new one.
	to     int
	doc    *reader.Doc
	resp   *reader.Response // a download, its body open
	cancel context.CancelFunc
	err    error
}

// newReaderTab is a new tab, a context of its own that ends with it.
func (m *AppModel) newReaderTab() *tab {
	ctx, cancel := context.WithCancel(context.Background())
	t := &tab{id: m.nextTabID, ctx: ctx, cancel: cancel, cursor: -1,
		measure: m.cfg.TextWidth(), tabW: m.cfg.TabWidth(), tick: m.a.Tick,
		rc: m.rc, histAt: -1}
	m.nextTabID++
	return t
}

// readerLoad fetches url into the tab, to the history entry to (-1: a new
// one). The page on screen stays until the new one lands.
func (t *tab) readerLoad(url string, to int) tea.Cmd {
	ctx, reqs := t.startLoad(url)
	t.reloading = t.doc != nil && url == t.url && to < 0
	t.url = url
	gen, id, rc, cancel := t.gen, t.id, t.rc, t.loadCancel
	return func() tea.Msg {
		resp, err := rc.Get(ctx, url)
		return readerResult(id, gen, url, to, resp, err, cancel, reqs)
	}
}

// readerSend sends a form of the page, by the button that sent it.
func (t *tab) readerSend(fm *reader.Form, by *reader.Field) tea.Cmd {
	ctx, reqs := t.startLoad(t.url)
	gen, id, rc, cancel := t.gen, t.id, t.rc, t.loadCancel
	return func() tea.Msg {
		req, err := fm.Request(ctx, by)
		if err != nil {
			cancel()
			return readerMsg{tabID: id, gen: gen, url: fm.Action.String(), to: -1, err: err}
		}
		resp, err := rc.Do(req)
		return readerResult(id, gen, req.URL.String(), -1, resp, err, cancel, reqs)
	}
}

// startLoad says the tab is on its way somewhere and gives the request a
// context of its own, the one before it cancelled, that keeps what the
// load asks for (the Requests popup).
func (t *tab) startLoad(url string) (context.Context, *reader.Requests) {
	t.gen++
	t.loading, t.navigating, t.pending, t.errText = true, true, false, ""
	if t.loadCancel != nil {
		t.loadCancel()
	}
	// Not the tab's: a download outlives the tab it started in.
	ctx, cancel := context.WithCancel(context.Background())
	t.loadCancel = cancel
	reqs := &reader.Requests{}
	return reader.Record(ctx, reqs), reqs
}

func readerResult(id, gen int, url string, to int, resp *reader.Response, err error, cancel context.CancelFunc, reqs *reader.Requests) tea.Msg {
	msg := readerMsg{tabID: id, gen: gen, url: url, to: to, err: err}
	switch {
	case err != nil:
		cancel()
	case !resp.IsPage():
		// The request's context goes with the download: removing the
		// row cancels it.
		msg.resp, msg.cancel = resp, cancel
	default:
		cancel()
		msg.doc, msg.err = reader.Read(resp)
		if msg.doc != nil {
			msg.doc.Requests = reqs.List()
		}
	}
	return msg
}

// readerGo is P and N on a reader tab: delta entries along its history.
func (t *tab) readerGo(delta int) tea.Cmd {
	j := t.histAt + delta
	if j < 0 || j >= len(t.hist) {
		return nil
	}
	return t.readerLoad(t.hist[j].url, j)
}

// applyReader is what a reader tab's fetch came back with.
func (m *AppModel) applyReader(msg readerMsg) tea.Cmd {
	_, t := m.tabByID(msg.tabID)
	if t == nil || msg.gen != t.gen {
		if msg.resp != nil {
			msg.resp.Close()
			msg.cancel()
		}
		return nil
	}
	again := t.reloading
	t.loading, t.navigating, t.reloading = false, false, false
	var cmds []tea.Cmd
	if err := m.rc.Jar.Err(); err != nil {
		cmds = append(cmds, m.saveFailed("cookies.json", err))
	}
	switch {
	case msg.err != nil:
		m.a.Log(terminu.Error, "Loading "+msg.url+": "+msg.err.Error())
		if again && t.doc != nil {
			// The page on screen stays; a toast says the look again
			// failed (tdp panel.15).
			t.url = t.doc.URL.String()
			return tea.Batch(append(cmds, m.toastErr("Could not load the page again: "+msg.err.Error()))...)
		}
		t.errText, t.doc, t.root, t.lines = msg.err.Error(), nil, nil, nil
		t.lay, t.cursor, t.top = layout{}, -1, 0
		return tea.Batch(cmds...)
	case msg.resp != nil:
		// Not a page: a download, and the page on screen stays.
		if t.doc != nil {
			t.url = t.doc.URL.String()
		} else {
			t.title = msg.resp.Filename
		}
		return tea.Batch(append(cmds, m.readerDownload(msg.resp, msg.cancel))...)
	}
	d := msg.doc
	// Another page: Focus mode and a search were on the one before.
	if md := m.pagePanel.InMode(); md != nil && md.Name == "Focus" {
		m.pagePanel.LeaveMode()
	}
	t.find = nil
	if t.histAt >= 0 && t.histAt < len(t.hist) {
		t.hist[t.histAt].top, t.hist[t.histAt].cursor = t.top, t.cursor
	}
	url := d.URL.String()
	back := msg.to >= 0 && msg.to < len(t.hist)
	switch {
	case back:
		t.histAt = msg.to
		t.hist[t.histAt].url = url
	case t.histAt >= 0 && t.hist[t.histAt].url == url:
		// The same page again (R): its entry, not a new one.
	default:
		t.hist = append(t.hist[:t.histAt+1], readerPlace{url: url, cursor: -1})
		t.histAt = len(t.hist) - 1
	}
	t.canBack, t.canForward = t.histAt > 0, t.histAt < len(t.hist)-1
	t.doc, t.url, t.title, t.errText = d, url, d.Title, ""
	t.top, t.cursor = 0, -1
	m.remark(t)
	if back || again {
		p := t.hist[t.histAt]
		t.top, t.cursor = clamp(p.top, 0, max(0, len(t.lay.rows)-1)), clamp(p.cursor, -1, len(t.lay.items)-1)
	} else if d.URL.Fragment != "" {
		m.readerJump(t, d.URL.Fragment)
	}
	if d.Status >= 400 {
		cmds = append(cmds, m.toastErr(d.StatusText+": "+url))
	}
	cmds = append(cmds, m.fetchFavicon(d))
	return tea.Batch(append(cmds, m.recordVisit(t))...)
}

// remark writes the tab's page out again — the content, or the whole of
// it — after it came, after a field changed, after f.
func (m *AppModel) remark(t *tab) {
	if t.doc == nil {
		return
	}
	root := t.doc.Content
	if t.whole {
		root = t.doc.Whole
	}
	t.root, t.lines = root, t.doc.Markdown(root)
	t.relayout(m.pageW())
	t.cursor = clamp(t.cursor, -1, len(t.lay.items)-1)
	t.scrollToCursor(m.pageVisible())
}

// relayoutReader lays the tab's page out at width, behind the column of
// line numbers.
func (t *tab) relayoutReader(width int) {
	draw := func(g int) layout { return renderReader(t.root, t.doc, max(1, width-g), t.tabW) }
	g := t.gutter
	lay := draw(g)
	if g2 := lineNumW(len(lay.rows)); g2 != g {
		g, lay = g2, draw(g2)
	}
	t.lay, t.gutter, t.layW = lay, g, width
	if t.find != nil {
		// The rows moved: the hits are found again, the one the search was
		// on kept as near as there is.
		at := t.find.at
		t.find.hits = findHits(t, t.find.query)
		t.find.at = clamp(at, 0, max(0, len(t.find.hits)-1))
		if len(t.find.hits) == 0 {
			t.find = nil
		}
	}
}

// readerJump puts the cursor on what a link to #frag lands on: the row it
// starts on at the top, the first thing to press from there.
func (m *AppModel) readerJump(t *tab, frag string) bool {
	n := t.doc.Anchor(t.root, frag)
	if n == nil {
		return false
	}
	row, ok := t.lay.marks[n]
	if !ok {
		return false
	}
	// Reading, nothing is chosen: the row comes to the top.
	t.top = clamp(row, 0, max(0, len(t.lay.rows)-1))
	return true
}

// readerEnter is Enter on a reader tab's item.
func (m *AppModel) readerEnter(t *tab, n *ir.Node) tea.Cmd {
	if n == nil || m.busy() {
		return nil
	}
	if n.Disabled {
		return m.toast("On the page, " + strings.TrimSpace(nameOr(n.Name, "this")) + " is disabled")
	}
	switch n.Kind {
	case ir.Link:
		return m.readerFollow(t, n.URL)
	case ir.Textbox:
		return m.readerEdit(t, n)
	case ir.Check:
		return m.readerTick(t, n)
	case ir.Combobox:
		return m.readerChoose(t, n)
	case ir.Button:
		return m.readerPress(t, n)
	case ir.Media:
		return m.toast("An image: [Space] › Yank media url copies its address")
	}
	return nil
}

// readerFollow goes where a link points: into the page when it points
// there, else the page it points at, in this tab.
func (m *AppModel) readerFollow(t *tab, url string) tea.Cmd {
	if frag := sameFragment(t.url, url); frag != "" && m.readerJump(t, frag) {
		m.did("Went to #" + frag)
		return nil
	}
	m.did("Followed the link to " + url)
	return t.readerLoad(url, -1)
}

// fieldName is how a field is named in a popup's title and the App Log.
func fieldName(f *reader.Field) string {
	return strings.TrimSpace(nameOr(f.Label, nameOr(f.Name, "the field")))
}

// readerEdit is Enter on a text field: a box to type in, the field's value
// in it; a password's never shown. A form with this one field to type in
// asks whether to send it, the way Enter in it sends it on the page.
func (m *AppModel) readerEdit(t *tab, n *ir.Node) tea.Cmd {
	f := t.doc.Field(n)
	if f == nil {
		return nil
	}
	name := fieldName(f)
	done := func(v string) tea.Cmd {
		f.Set(v)
		m.remark(t)
		// What was typed stays out of the App Log: it may be a secret.
		m.did("Typed into " + name)
		if f.Form == nil || v == "" || textFields(f.Form) != 1 {
			return nil
		}
		return m.a.Confirm(terminu.Ask{Title: "Send", Details: []string{v},
			Question: "Send the form, as Enter in the field does?", Verb: "send"},
			m.do(func() tea.Cmd { return m.readerSubmit(t, f.Form, nil) }))
	}
	switch {
	case n.Protected:
		return m.a.Password(terminu.PasswordInput{Title: "Edit", Subject: name, Glyph: glyphPencil, Name: "The password",
			Prompt: "A password for " + name, Verb: "set", Set: f.Value != "",
			Done: func(v string, _ bool) tea.Cmd { cmd := done(v); m.sync(); return cmd }})
	case n.Multiline:
		return m.a.Textarea(terminu.Textarea{Title: "Edit", Subject: name, Glyph: glyphPencil, Name: "The text", Old: f.Value,
			Done: m.cmdDone(done)})
	}
	return m.a.Text(terminu.TextInput{Title: "Edit", Subject: name, Glyph: glyphPencil, Name: "The value",
		Prompt: "A value for " + name, Verb: "set", Old: f.Value, Done: m.cmdDone(done)})
}

// textFields is how many fields of fm take typing.
func textFields(fm *reader.Form) int {
	n := 0
	for _, f := range fm.Fields {
		if f.Text() && !f.Disabled {
			n++
		}
	}
	return n
}

// readerTick is Enter on a check box (ticked or not) or a radio button
// (chosen).
func (m *AppModel) readerTick(t *tab, n *ir.Node) tea.Cmd {
	f := t.doc.Field(n)
	if f == nil {
		return nil
	}
	f.Toggle()
	m.remark(t)
	switch {
	case f.Type == "radio":
		m.did("Chose " + fieldName(f))
	case f.Checked:
		m.did("Ticked " + fieldName(f))
	default:
		m.did("Unticked " + fieldName(f))
	}
	return nil
}

// readerChoose is Enter on a select: its options, the chosen one marked.
func (m *AppModel) readerChoose(t *tab, n *ir.Node) tea.Cmd {
	f := t.doc.Field(n)
	if f == nil || len(f.Options) == 0 {
		return m.toast("No options to choose from")
	}
	opts := make([]string, len(f.Options))
	sel := -1
	for i, o := range f.Options {
		opts[i] = nameOr(o.Label, "(empty)")
		if o.Selected {
			sel = i
		}
	}
	name := fieldName(f)
	return m.a.Select(terminu.Select{Title: "Choose", Subject: name, Glyph: glyphSelect, Options: opts, Selected: sel,
		Done: func(i int) tea.Cmd {
			if i < 0 || i >= len(f.Options) {
				return nil
			}
			f.Choose(i)
			m.remark(t)
			m.did("Chose " + opts[i] + " in " + name)
			m.sync()
			return nil
		}})
}

// readerPress is Enter on a button: one that sends its form sends it;
// any other wants the page's JavaScript, which the reader does not run.
func (m *AppModel) readerPress(t *tab, n *ir.Node) tea.Cmd {
	f := t.doc.Field(n)
	if f == nil || !f.Submits() {
		return m.toast("This button needs the page's JavaScript, which webu does not run")
	}
	return m.readerSubmit(t, f.Form, f)
}

// readerSubmit sends fm, by the button by (nil: none, as Enter in a field).
func (m *AppModel) readerSubmit(t *tab, fm *reader.Form, by *reader.Field) tea.Cmd {
	what := "the form"
	if fm.Name != "" {
		what += " " + fm.Name
	}
	m.did("Sent " + what + " to " + fm.Action.String())
	return t.readerSend(fm, by)
}

// readerDownload saves what a link turned out to be into the download
// folder, its row in Downloads moving as it comes.
func (m *AppModel) readerDownload(resp *reader.Response, cancel context.CancelFunc) tea.Cmd {
	m.rdSeq++
	guid := fmt.Sprintf("reader-%d", m.rdSeq)
	m.rdCancel[guid] = cancel
	total := max(resp.Size, 0)
	m.dls = append(m.dls, download{guid: guid, name: resp.Filename, url: resp.URL.String(), total: total, at: time.Now()})
	m.refreshDownloads()
	ch, dir := m.events, m.downloadDir()
	go func() {
		var got int64
		p, err := resp.Save(dir, func(n int64) {
			got = n
			ch <- downloadMsg{guid: guid, received: n, total: total}
		})
		cancel()
		if err != nil {
			ch <- downloadMsg{guid: guid, failed: true}
			return
		}
		ch <- downloadMsg{guid: guid, done: true, path: p, received: got, total: max(total, got)}
	}()
	return m.toast("Downloading " + resp.Filename)
}

// readerItemActions are the item operations of a reader tab's field, for
// its Space menu.
func (m *AppModel) readerItemActions(t *tab, n *ir.Node) []terminu.Action {
	f := t.doc.Field(n)
	if f == nil || !f.Text() || f.Value == "" {
		return nil
	}
	out := []terminu.Action{
		m.item("Clear", "empty the field", func() tea.Cmd {
			f.Set("")
			m.remark(t)
			m.did("Cleared " + fieldName(f))
			return nil
		}),
	}
	if !n.Protected {
		out = append(out, m.item("Yank value", "copy what is in the field", func() tea.Cmd { return copyToClipboard(f.Value) }))
	}
	if f.Form != nil {
		out = append([]terminu.Action{m.item("Send", "the form, as Enter in the field does", func() tea.Cmd {
			return m.readerSubmit(t, f.Form, nil)
		})}, out...)
	}
	return out
}
