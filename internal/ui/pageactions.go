package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/ir"
)

// Panel [2]'s actions (ux.md §A.1), registered with the lib: Enter's,
// the item's — what a right click would list, by the item's role, no
// letters — and the page's own. They are worked out again whenever the
// page or the cursor moves (syncPage), so the Space menu, the key
// reference and the hint always speak of what is under the cursor now.

// syncPage puts the shown tab into panel [2]'s frame: the loading icon,
// the actions.
func (m *AppModel) syncPage() {
	p := m.pagePanel
	if p == nil {
		return
	}
	t := m.shownTab()
	p.Loading = t != nil && t.working()
	p.LoadingText = ""
	if t != nil {
		p.LoadingText = "loading " + nameOr(hostOf(t.url), "the page") + "…"
	}
	p.Actions = m.pageActions(t)
	// Esc is listed, and does something, only where there is a level up.
	p.OnEsc = nil
	if t != nil && t.find != nil {
		p.OnEsc = m.do(m.pageEsc)
	}
}

// pageEsc is Esc on panel [2], one level up (tdp K4): reading, that is
// the search put away.
func (m *AppModel) pageEsc() tea.Cmd {
	if t := m.shownTab(); t != nil {
		t.find = nil
	}
	return nil
}

// busy reports whether the shown page is on its way somewhere: a
// navigation or history move has been asked for and has not landed. The
// keys that would stack another one on it are swallowed (ux.md §6).
func (m *AppModel) busy() bool {
	t := m.shownTab()
	return t != nil && t.navigating
}

// pageActions are panel [2]'s actions for the page as it is now.
func (m *AppModel) pageActions(t *tab) []terminu.Action {
	var acts []terminu.Action
	if enter := m.enterAction(t); enter != nil {
		acts = append(acts, *enter)
	}
	if n := t.current(); n != nil {
		acts = append(acts, m.itemActions(t, n)...)
	}
	return append(acts, m.panelActions(t)...)
}

// item is an item operation with no hotkey: the page's items are
// menu-only, the letters being the panel's (ux.md §A.0).
func (m *AppModel) item(label, desc string, do func() tea.Cmd) terminu.Action {
	return terminu.Action{Label: label, Desc: strings.TrimSpace(desc), Scope: terminu.ItemScope, Do: m.do(do)}
}

// enterAction is what Enter does to the item under the cursor (ux.md
// §A.0.K.1): its name and hint follow the item.
func (m *AppModel) enterAction(t *tab) *terminu.Action {
	if t == nil || t.root == nil {
		return nil
	}
	label, desc, hint := "", "", ""
	n := t.current()
	if n == nil {
		return nil
	}
	label, desc, hint = enterWords(n)
	act := &terminu.Action{Key: "Enter", Label: label, Desc: desc, Scope: terminu.ItemScope, Hint: hint,
		Disabled: m.busy, Do: m.do(m.enterItem)}
	return act
}

// enterWords names Enter on a node: the label, the line under it, the
// hint's word.
func enterWords(n *ir.Node) (string, string, string) {
	switch n.Kind {
	case ir.Link:
		return "Open", strings.TrimSpace(n.URL), "open"
	case ir.Textbox:
		return "Edit", "type a value", "edit"
	case ir.Combobox:
		return "Choose", "pick an option", "choose"
	case ir.Button, ir.Check, ir.Option:
		return "Click", "press it", "click"
	}
	return "Click", "the page decides", "click"
}

// itemActions are the item operations for a node, by its role (ux.md
// §A.1), without the one Enter already is.
func (m *AppModel) itemActions(t *tab, n *ir.Node) []terminu.Action {
	var out []terminu.Action
	switch n.Kind {
	case ir.Link:
		out = append(out,
			m.item("Open in new tab", "and switch to it", func() tea.Cmd {
				if n.URL == "" {
					return nil
				}
				m.did("Opened " + n.URL + " in a new tab")
				return m.openTab(n.URL, true)
			}),
			// "link url", not "url": with the cursor on a link, a bare "Yank
			// url" reads as the page's, which is [Y].
			m.item("Yank link url", n.URL, func() tea.Cmd { return copyToClipboard(n.URL) }))
	case ir.Media:
		yank := m.item("Yank media url", nameOr(n.URL, "no address"), func() tea.Cmd { return copyToClipboard(n.URL) })
		yank.Disabled = func() bool { return n.URL == "" }
		out = append(out, yank)
	case ir.Textbox:
		out = append(out, m.readerItemActions(t, n)...)
	}
	return append(out,
		m.item("Yank text", "what it says", func() tea.Cmd { return copyToClipboard(n.Text()) }),
		m.item("Inspect", "role, name, state, node id", func() tea.Cmd {
			m.did("Inspected " + nodeName(n))
			return m.a.Viewer(&terminu.Viewer{Title: "Inspect", Subject: strings.TrimSpace(nameOr(n.Name, n.Role)), Glyph: glyphInfo, Lines: inspectLines(n),
				Select: selection()})
		}))
}

// panelActions are the page's own (ux.md §A.1): what it does to the whole
// page, not to the item (readerkeys.go).
func (m *AppModel) panelActions(t *tab) []terminu.Action { return m.readerPanelActions(t) }

// wholeAction is w: the whole page, its navigation and all, or back to
// its content (docs/reader.md).
func (m *AppModel) wholeAction(t *tab) terminu.Action {
	label, desc := "Full page", "everything on it, the site around the content too"
	if t != nil && t.whole {
		label, desc = "Content only", "what the page itself presents"
	}
	return terminu.Action{Key: "w", Label: label, Desc: desc, Scope: terminu.PanelScope,
		Disabled: func() bool { t := m.shownTab(); return t == nil || t.doc == nil },
		Do: m.do(func() tea.Cmd {
			t := m.shownTab()
			t.whole = !t.whole
			t.top, t.cursor = 0, -1
			m.remark(t)
			if t.whole {
				m.did("Showed the full page of " + tabName(t))
			} else {
				m.did("Showed the content of " + tabName(t))
			}
			return nil
		})}
}

// locationBox is [L]: the page's own URL is offered — Tab takes it into
// the line to edit, Backspace drops it, typing over it starts fresh
// (ux.md §2.2).
func (m *AppModel) locationBox() tea.Cmd {
	old := ""
	if t := m.shownTab(); t != nil && t.url != "about:blank" {
		old = t.url
	}
	return m.a.Text(terminu.TextInput{Title: "Location", Glyph: glyphSearch, Name: "The address",
		Prompt: "URL, or words to search for", Verb: "open", Old: old,
		Done: m.cmdDone(func(v string) tea.Cmd {
			if v == "" {
				return nil
			}
			t, url := m.shownTab(), m.resolveURL(v)
			if t == nil {
				m.did("Opened " + url + " in a new tab")
				return m.openTab(url, true)
			}
			m.did("Went to " + url)
			return t.load(url)
		})})
}

// enterItem is Enter on panel [2] (ux.md §A.0.K.1): what the item under
// Focus mode's cursor does (readerEnter).
func (m *AppModel) enterItem() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	return m.readerEnter(t, t.current())
}

// selection is a viewer's selection mode: v brings the cursor out, v
// again starts selecting, y copies (tdp note.4).
func selection() *terminu.Selection {
	return &terminu.Selection{Key: "v", Name: "Visual", Copy: copyToClipboard}
}

// sameFragment is the fragment of a link that points into the page it is
// on — "#main", or the page's own URL with a fragment — else "".
func sameFragment(pageURL, url string) string {
	i := strings.Index(url, "#")
	if i < 0 {
		return ""
	}
	base, frag := url[:i], url[i+1:]
	if j := strings.Index(pageURL, "#"); j >= 0 {
		pageURL = pageURL[:j]
	}
	if frag == "" || (base != "" && base != pageURL) {
		return ""
	}
	return frag
}

// hostOf is a URL's host, for saying which page is on its way.
func hostOf(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}
	return u
}
