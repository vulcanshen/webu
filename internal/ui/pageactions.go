package ui

import (
	"context"
	"errors"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
)

// Panel [2]'s actions (ux.md §A.1), registered with the lib: Enter's,
// the item's — what a right click would list, by the item's role, no
// letters — and the page's own. They are worked out again whenever the
// page or the cursor moves (syncPage), so the Space menu, the key
// reference and the hint always speak of what is under the cursor now.

// syncPage puts the shown tab into panel [2]'s frame: its tabs (the whole
// page, then its parts), the loading icon, the actions.
func (m *AppModel) syncPage() {
	p := m.pagePanel
	if p == nil {
		return
	}
	t := m.shownTab()
	p.Loading = t != nil && t.working()
	p.Tabs, p.Tab = nil, 0
	if t != nil && t.root != nil && len(t.parts) >= 2 && t.popupNode() == nil {
		p.Tabs = partLabels(t.parts)
		if t.at != partWhole {
			p.Tab = t.partIndex(t.at) + 1
		}
	}
	p.Actions = m.pageActions(t)
}

// partTab is h or l on panel [2]: the whole page, or one of its parts.
func (m *AppModel) partTab(i int) tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	kind := partWhole
	if i > 0 && i-1 < len(t.parts) {
		kind = t.parts[i-1].kind
	}
	t.showPart(kind, m.pageW(), m.pageVisible())
	m.syncPage()
	return nil
}

// pageEsc is Esc on panel [2], one level up (Rules K4): out of a thing,
// back to the section list; at the top it does nothing. A popup the page
// put up is not closed by it: it wants an answer (dev-remarks, 偏離 tdp).
func (m *AppModel) pageEsc() tea.Cmd {
	t := m.shownTab()
	switch {
	case t == nil:
		return nil
	case t.drilled():
		t.leaveDrill(m.pageW(), m.pageVisible())
	case t.popupNode() != nil:
		return m.toast("This popup wants an answer: press a button in it")
	case t.read:
		t.closeSection()
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
	if t != nil && !t.listing() {
		if n := t.current(); n != nil {
			acts = append(acts, m.itemActions(t, n)...)
		}
	}
	return append(acts, m.panelActions(t)...)
}

// item is an item operation with no hotkey: the page's items are
// menu-only, the letters being the panel's (ux.md §A.0).
func (m *AppModel) item(label, desc string, do func() tea.Cmd) terminu.Action {
	return terminu.Action{Label: label, Desc: oneLine(desc), Scope: terminu.ItemScope, Do: m.do(do)}
}

// enterAction is what Enter does to the item under the cursor (ux.md
// §A.0.K.1): its name and hint follow the item.
func (m *AppModel) enterAction(t *tab) *terminu.Action {
	if t == nil || t.root == nil {
		return nil
	}
	label, desc, hint := "", "", ""
	switch n := t.current(); {
	case t.listing():
		if t.sec >= len(t.secs) {
			return nil
		}
		s := t.secs[t.sec]
		label, desc, hint = "Open section", truncate(s.title, 40)+" · "+plural(s.lines(), "line"), "open"
	case n == nil:
		return nil
	default:
		label, desc, hint = enterWords(t, n)
	}
	return &terminu.Action{Key: "Enter", Label: label, Desc: desc, Scope: terminu.ItemScope, Hint: hint,
		Disabled: m.busy, Do: m.do(m.enterItem)}
}

// enterWords names Enter on a node: the label, the line under it, the
// hint's word.
func enterWords(t *tab, n *ir.Node) (string, string, string) {
	switch n.Kind {
	case ir.ListItem:
		if isThing(n) {
			return "Go in", "this item, to the whole panel", "go in"
		}
	case ir.Cell:
		return "Read", "the cell, or what it holds", "read"
	case ir.Media:
		if n.Frame != "" {
			return "Go in", "the frame's page", "go in"
		}
		return "Click", "the page decides", "click"
	case ir.Code:
		return "Read", "the block on its own, lines unfolded", "read"
	case ir.Landmark, ir.Heading:
		if t.curFolded() {
			return "Expand", "show what is inside", "expand"
		}
		return "Collapse", "one line, out of the way", "collapse"
	case ir.Link:
		return "Open", oneLine(n.URL), "open"
	case ir.Textbox:
		return "Edit", "type a value", "edit"
	case ir.Combobox:
		return "Choose", "pick an option", "choose"
	case ir.Button, ir.Check, ir.Option, ir.Unsupported:
		return "Click", "press it", "click"
	case ir.Group:
		if n.Role == "treeitem" {
			return "Click", "open the branch, or pick the leaf", "click"
		}
	}
	return "Click", "the page decides", "click"
}

// itemActions are the item operations for a node, by its role (ux.md
// §A.1), without the one Enter already is.
func (m *AppModel) itemActions(t *tab, n *ir.Node) []terminu.Action {
	var out []terminu.Action
	busy := func(a terminu.Action) terminu.Action { a.Disabled = m.busy; return a }
	switch n.Kind {
	case ir.Cell:
		out = append(out, m.item("Content", "the cell in full", func() tea.Cmd { return m.showCell(t, n) }))
		for i, x := range entryTargets(n) {
			x := x
			out = append(out, busy(m.item(strings.Repeat("  ", x.depth)+truncate(targetLabel(x.node), 50), targetDesc(x.node, i),
				func() tea.Cmd { return m.actOn(t, x.node, false) })))
		}
	case ir.Link:
		out = append(out,
			m.item("Open in new tab", "and switch to it", func() tea.Cmd {
				if n.URL == "" {
					return nil
				}
				return m.openTab(n.URL, true)
			}),
			// "link url", not "url": with the cursor on a link, a bare "Yank
			// url" reads as the page's, which is [Y].
			m.item("Yank link url", n.URL, func() tea.Cmd { return copyToClipboard(n.URL) }))
	case ir.Media:
		if n.Frame != "" {
			out = append(out, busy(m.item("Click", "the page decides", func() tea.Cmd { return m.click(t, n) })))
		}
		yank := m.item("Yank media url", nameOr(n.URL, "no address"), func() tea.Cmd { return copyToClipboard(n.URL) })
		yank.Disabled = func() bool { return n.URL == "" }
		out = append(out, yank)
	case ir.Textbox:
		if n.Value != "" {
			if !n.Multiline {
				out = append(out, busy(m.item("Submit", "press [Enter] in the field", func() tea.Cmd { return m.submit(t, n) })))
			}
			out = append(out,
				busy(m.item("Clear", "empty the field", func() tea.Cmd {
					id := n.ID
					return t.press(func(ctx context.Context) error { return page.Type(ctx, id, "") })
				})),
				m.item("Yank value", "copy what is in the field", func() tea.Cmd { return copyToClipboard(n.Value) }))
		}
	case ir.Unsupported:
		// What webu cannot do with it is said where the one thing it can do
		// is, not in a disabled row of its own (tdp M6).
		out = append(out, busy(m.item("Click", "the page decides; "+n.Role+" is not supported yet", func() tea.Cmd { return m.click(t, n) })))
	}
	return append(out,
		m.item("Yank text", "what it says", func() tea.Cmd { return copyToClipboard(n.Text()) }),
		m.item("Inspect", "role, name, state, node id", func() tea.Cmd {
			return m.a.Viewer(&terminu.Viewer{Title: "Inspect", Subject: oneLine(nameOr(n.Name, n.Role)), Glyph: glyphInfo, Lines: inspectLines(n)})
		}))
}

// targetLabel is a target's name in a cell's list.
func targetLabel(n *ir.Node) string {
	label := oneLine(n.Name)
	if label == "" {
		label = oneLine(nameOr(n.Text(), n.URL))
	}
	return label
}

// targetDesc says what a target in a cell is — and "here" first when it
// is where the user is (aria-current).
func targetDesc(n *ir.Node, _ int) string {
	desc := oneLine(n.URL)
	switch n.Kind {
	case ir.Button:
		desc = "button"
	case ir.Textbox:
		desc = "a field: type into it"
	case ir.Check:
		desc = "check box"
	case ir.Combobox:
		desc = "select"
	}
	if desc == "" {
		desc = "press it"
	}
	if n.Current {
		desc = "here · " + desc
	}
	return desc
}

// panelActions are the page's own (ux.md §A.1): what it does to the whole
// page, not to the item.
func (m *AppModel) panelActions(t *tab) []terminu.Action {
	noTab := func() bool { return m.shownTab() == nil }
	noPage := func() bool { t := m.shownTab(); return t == nil || t.root == nil }
	panel := func(key, label, desc, hint string, off func() bool, do func() tea.Cmd) terminu.Action {
		return terminu.Action{Key: key, Label: label, Desc: desc, Scope: terminu.PanelScope, Hint: hint, Disabled: off, Do: m.do(do)}
	}
	reading := func() bool { t := m.shownTab(); return t == nil || !t.read }
	acts := []terminu.Action{
		panel("R", "Reload", "this page", "", func() bool { return noTab() || m.busy() }, func() tea.Cmd {
			t := m.shownTab()
			return t.load(t.url)
		}),
		// The same T as [1]'s: a new tab is wanted from the page as often as
		// from the list.
		panel("T", "Tab", "a new one, at a URL", "", nil, m.newTabBox),
		panel("P", "Previous", "back in this tab", "", func() bool { t := m.shownTab(); return t == nil || !t.canBack || m.busy() }, func() tea.Cmd {
			return m.shownTab().navigate("back", page.Back)
		}),
		panel("N", "Next", "forward in this tab", "", func() bool { t := m.shownTab(); return t == nil || !t.canForward || m.busy() }, func() tea.Cmd {
			return m.shownTab().navigate("forward", page.Forward)
		}),
		panel("/", "Search", "every part of the page; [Enter] goes there", "search",
			func() bool { t := m.shownTab(); return t == nil || t.root == nil || t.popupNode() != nil }, m.openFinder),
		panel("v", "visual mode", "walk the text by character, copy some", "", noPage, m.enterSelect),
		panel("L", "Location", "a URL or a search; this page's own is offered", "", nil, m.locationBox),
		panel("A", "Add bookmark", "this page", "", noTab, func() tea.Cmd {
			t := m.shownTab()
			return m.addBookmark(t.title, t.url)
		}),
		panel("go", "Go to line", "by its number in the gutter", "", func() bool {
			t := m.shownTab()
			return t == nil || t.popupNode() != nil || t.lineCount() == 0
		}, m.goLineBox),
		panel("n", "next section", "the one after this, at the same depth", "", reading, func() tea.Cmd { return m.stepSection(1) }),
		panel("p", "previous section", "the one before this, at the same depth", "", reading, func() tea.Cmd { return m.stepSection(-1) }),
		m.sectionsAction(t),
		panel("Z", "Zoom", "the page alone, or the grid back", "", nil, func() tea.Cmd {
			m.zoom = !m.zoom
			m.relayoutTabs()
			return nil
		}),
		panel("Y", "Yank page url", "to the clipboard", "", noTab, func() tea.Cmd { return copyToClipboard(m.shownTab().url) }),
		panel("", "Yank markdown", "the page itself, as markdown", "", noPage, func() tea.Cmd {
			return copyToClipboard(ir.Markdown(m.shownTab().root))
		}),
		panel("C", "Close", "this tab", "", noTab, func() tea.Cmd { return m.closeTab(m.shown) }),
	}
	return acts
}

// sectionsAction turns the cut into sections off for a page it does not
// suit, or on for one it was not offered for (section.go).
func (m *AppModel) sectionsAction(t *tab) terminu.Action {
	label, desc := "Sections", "read this page one section at a time"
	if t != nil && len(t.secs) > 0 && !t.flat {
		label, desc = "One sheet", "read the whole page in one run"
	}
	return terminu.Action{Label: label, Desc: desc, Scope: terminu.PanelScope,
		Disabled: func() bool { t := m.shownTab(); return t == nil || len(t.secs) == 0 },
		Do: m.do(func() tea.Cmd {
			t := m.shownTab()
			t.flat, t.read = !t.flat, false
			if t.flat {
				t.scrollToCursor(m.pageVisible())
				return nil
			}
			t.sec = sectionAt(t.secs, t.top)
			t.clampSecTop(m.pageVisible())
			return nil
		})}
}

// stepSection is n or p: the next or previous section at the same depth,
// without going back through the list.
func (m *AppModel) stepSection(step int) tea.Cmd {
	t := m.shownTab()
	if t == nil || !t.read {
		return nil
	}
	if !t.stepSection(step, m.pageVisible()) {
		return m.toast("No section that way")
	}
	return nil
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
			t := m.shownTab()
			if t == nil {
				return m.openTab(m.resolveURL(v), true)
			}
			return t.load(m.resolveURL(v))
		})})
}

// goLineBox is [go]: a line of what is on screen, by the number in the
// gutter — on the section list the lines are the sections.
func (m *AppModel) goLineBox() tea.Cmd {
	t := m.shownTab()
	n := t.lineCount()
	return m.a.Number(terminu.NumberInput{Title: "Go to line", Glyph: glyphSearch, Name: "The line",
		Prompt: "A line from 1 to " + itoa(n), Verb: "go", Min: 1, Max: float64(n),
		Done: m.cmdDone(func(v string) tea.Cmd {
			i, err := strconv.Atoi(v)
			if err == nil {
				t.goToLine(i, m.pageVisible())
			}
			return nil
		})})
}

// enterItem is Enter on panel [2] (ux.md §A.0.K.1): what a left click on
// the item would do, in terminal terms; for what a click cannot mean —
// a section, a list item, a frame — it goes in.
func (m *AppModel) enterItem() tea.Cmd {
	t := m.shownTab()
	if t == nil {
		return nil
	}
	if t.listing() {
		t.openSection(m.pageVisible())
		return nil
	}
	n := t.current()
	if n == nil {
		return nil
	}
	switch n.Kind {
	case ir.ListItem:
		// A list item is one thing; Enter is how you go into it. One that
		// is nothing but its first line has no inside to go to — Enter is
		// Enter on what that line holds.
		w, vis := m.pageW(), m.pageVisible()
		if t.drillInto(n, w, vis) && t.drillBody >= len(t.lay.rows) {
			t.leaveDrill(w, vis)
			if x := firstItemIn(n); x != nil {
				return m.enterOn(t, x)
			}
		}
		return nil
	case ir.Cell:
		return m.enterCell(t, n)
	case ir.Media:
		if n.Frame != "" {
			// An iframe is one thing until you go in: its document, when it
			// has come; asked for, when not.
			if len(n.Children) > 0 {
				t.drillInto(n, m.pageW(), m.pageVisible())
				return nil
			}
			return t.openFrame(n)
		}
	case ir.Code:
		return m.showCode(n)
	case ir.Landmark:
		if n.Role == "search" {
			// A search with one box has one obvious operation: the box.
			var boxes []*ir.Node
			for _, x := range entryTargets(n) {
				if x.node.Kind == ir.Textbox {
					boxes = append(boxes, x.node)
				}
			}
			if len(boxes) == 1 {
				return m.editFieldAs(t, boxes[0], true)
			}
		}
		m.fold(t)
		return nil
	case ir.Heading:
		m.fold(t)
		return nil
	}
	return m.enterOn(t, n)
}

// fold opens or shuts the landmark or heading under the cursor.
func (m *AppModel) fold(t *tab) {
	t.toggleFold(m.pageW())
	t.scrollToCursor(m.pageVisible())
}

// firstItemIn is the first thing the cursor could stop on inside n, or
// nil: what Enter on a one-line list item is Enter on.
func firstItemIn(n *ir.Node) *ir.Node {
	var found *ir.Node
	n.Walk(func(x *ir.Node) bool {
		if found == nil && x != n && x.ID != 0 && x.IsItem() {
			found = x
		}
		return found == nil
	})
	return found
}

// click presses a node.
func (m *AppModel) click(t *tab, n *ir.Node) tea.Cmd {
	if m.busy() {
		return nil
	}
	id := n.ID
	return t.press(func(ctx context.Context) error { return page.Click(ctx, id) })
}

// submit presses Enter in a field: the page's implicit submission.
func (m *AppModel) submit(t *tab, n *ir.Node) tea.Cmd {
	id := n.ID
	return t.press(func(ctx context.Context) error { return page.Submit(ctx, id) })
}

// enterOn is Enter on an interactive node: the item under the cursor, or
// the one thing a table cell holds. A field opens to type, a select drops
// its list, a link asks first, the rest is a click.
func (m *AppModel) enterOn(t *tab, n *ir.Node) tea.Cmd {
	if m.busy() {
		return nil
	}
	if n.Disabled {
		// A click on a disabled control does nothing in the browser either,
		// but a terminal's grey is easy to miss: say why nothing happened.
		return m.toast(oneLine(nameOr(n.Name, "This")) + " is disabled on the page")
	}
	switch n.Kind {
	case ir.Textbox:
		return m.editField(t, n)
	case ir.Combobox:
		return m.chooseOptionsFor(t, n)
	case ir.Link:
		// A link into the page lands the cursor, no confirm: nothing is
		// left.
		if frag := sameFragment(t.url, n.URL); frag != "" && t.jumpToAnchor(frag, m.pageVisible()) {
			return nil
		}
		return m.askOpenLink(t, n)
	}
	return m.click(t, n)
}

// enterCell is Enter on a data table's cell: the content in full — unless
// the cell is one link, one button, one field and nothing else, in which
// case the cell IS that thing; a cell holding text and links both lists
// them, Content first.
func (m *AppModel) enterCell(t *tab, n *ir.Node) tea.Cmd {
	ts := entryTargets(n)
	switch {
	case len(ts) == 0:
		return m.showCell(t, n)
	case len(ts) == 1 && oneLine(n.Text()) == oneLine(ts[0].node.Text()):
		return m.enterOn(t, ts[0].node)
	}
	acts := []terminu.Action{m.item("Content", "the cell in full", func() tea.Cmd { return m.showCell(t, n) })}
	for i, x := range ts {
		x := x
		acts = append(acts, m.item(strings.Repeat("  ", x.depth)+truncate(targetLabel(x.node), 50), targetDesc(x.node, i),
			func() tea.Cmd { return m.actOn(t, x.node, false) }))
	}
	return m.a.Menu(truncate(oneLine(nameOr(columnHeader(t.root, n), "cell")), 40), terminu.MenuSection{Actions: acts})
}

// showCell is a cell's content in full: the column's header as the
// title, the text wrapped.
func (m *AppModel) showCell(t *tab, n *ir.Node) tea.Cmd {
	text := strings.TrimSpace(n.Text())
	if text == "" {
		text = "(empty)"
	}
	return m.a.Viewer(&terminu.Viewer{Title: "Cell", Subject: oneLine(nameOr(columnHeader(t.root, n), "")),
		Glyph: glyphTable, Lines: strings.Split(text, "\n")})
}

// showCode is a code block in full: every line as the page wrote it,
// coloured by its language when the page named one.
func (m *AppModel) showCode(n *ir.Node) tea.Cmd {
	text := strings.TrimRight(n.Text(), "\n")
	return m.a.Viewer(&terminu.Viewer{Title: "Code", Subject: n.Lang, Glyph: glyphCode,
		Lines: strings.Split(text, "\n"), Lang: n.Lang})
}

// actOn does to a node inside a cell what Enter does to it as an item: a
// field opens to type, a select drops its list, the rest is a click — no
// confirm for a link, the list having been the look.
func (m *AppModel) actOn(t *tab, x *ir.Node, search bool) tea.Cmd {
	if m.busy() {
		return nil
	}
	switch x.Kind {
	case ir.Textbox:
		return m.editFieldAs(t, x, search || x.Role == "searchbox")
	case ir.Combobox:
		return m.chooseOptionsFor(t, x)
	case ir.Link:
		if frag := sameFragment(t.url, x.URL); frag != "" && t.jumpToAnchor(frag, m.pageVisible()) {
			return nil
		}
	}
	return m.click(t, x)
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

// askOpenLink puts a link's text and URL up before following it: a click
// would leave the page, and nothing on the screen says where to.
func (m *AppModel) askOpenLink(t *tab, n *ir.Node) tea.Cmd {
	details := []string{oneLine(n.URL)}
	if text := oneLine(n.Text()); text != "" {
		details = append([]string{text}, details...)
	}
	return m.a.Confirm(terminu.Ask{Title: "Open link", Details: details,
		Question: "Open it in this tab?", Verb: "open"}, m.do(func() tea.Cmd { return m.click(t, n) }))
}

// chooseOptionsFor lists a select's options, the current one chosen;
// picking one sets it (ux.md §2.1).
func (m *AppModel) chooseOptionsFor(t *tab, n *ir.Node) tea.Cmd {
	if n == nil || n.Kind != ir.Combobox {
		return nil
	}
	if len(n.Children) == 0 {
		return m.toast("No options to choose from")
	}
	opts := make([]string, len(n.Children))
	sel := -1
	for i, o := range n.Children {
		opts[i] = oneLine(nameOr(o.Name, "(empty)"))
		if o.Selected {
			sel = i
		}
	}
	return m.a.Select(terminu.Select{Title: "Choose", Subject: oneLine(n.Name), Glyph: glyphSelect,
		Options: opts, Selected: sel,
		Done: func(i int) tea.Cmd {
			if i < 0 || i >= len(n.Children) {
				return nil
			}
			opt := n.Children[i].ID
			cmd := t.press(func(ctx context.Context) error { return page.Choose(ctx, opt) })
			m.sync()
			return cmd
		}})
}

// editField opens the input for a textbox; a box inside a search
// landmark, or one that says it is a search, is a search box.
func (m *AppModel) editField(t *tab, n *ir.Node) tea.Cmd {
	return m.editFieldAs(t, n, n.Role == "searchbox" || inSearch(t, n))
}

// inSearch reports whether a node sits inside a search landmark: a box
// there is a search box whatever it calls itself.
func inSearch(t *tab, n *ir.Node) bool {
	for _, a := range chainTo(t.root, n) {
		if a.Kind == ir.Landmark && a.Role == "search" {
			return true
		}
	}
	return false
}

// editFieldAs opens the input that suits the field (ux.md §2.1): a
// password's masked box, a textarea, a slider's numbers, a date or a
// colour checked against the browser's shape, or one line of text. What
// is written into a search box is then offered to the page's Enter.
func (m *AppModel) editFieldAs(t *tab, n *ir.Node, search bool) tea.Cmd {
	id := n.ID
	name := oneLine(nameOr(n.Name, "the field"))
	title := fieldTakes(n)
	typeIn := func(v string) tea.Cmd {
		return t.press(func(ctx context.Context) error { return page.Type(ctx, id, v) })
	}
	switch {
	case n.Protected:
		// Never the old value: Chromium hands over dots, not the secret.
		return m.a.Password(terminu.PasswordInput{Title: "Password", Glyph: glyphPencil, Prompt: name, Verb: "set",
			Done: func(v string, _ bool) tea.Cmd { cmd := typeIn(v); m.sync(); return cmd }})
	case n.Multiline:
		return m.a.Textarea(terminu.Textarea{Title: title, Subject: name, Glyph: glyphPencil, Name: "The text", Old: n.Value,
			Done: m.cmdDone(typeIn)})
	case n.Role == "slider":
		return m.slideMenu(t, n)
	}
	in := terminu.TextInput{Title: title, Glyph: glyphPencil, Prompt: name, Verb: "set", Name: "The value", Old: cleanOld(n.Value)}
	if n.Invalid {
		// The page marked what is there wrong: the one thing the box should
		// say about the value it is about to replace.
		in.Prompt += " — the page says the value is not right"
	}
	if shape := fillFormat(n.InputType); shape != "" {
		// A date, a time, a colour: one string in the browser's shape, set
		// whole — the browser would drop one in another shape without a
		// word, so the box says so instead.
		in.Prompt += " (" + shape + ")"
		in.Check = func(v string) error {
			if v != "" && !fillOK(shape, v) {
				return errors.New("The value wants " + shape)
			}
			return nil
		}
		in.Done = m.cmdDone(func(v string) tea.Cmd {
			return t.press(func(ctx context.Context) error { return page.Fill(ctx, id, v) })
		})
		return m.a.Text(in)
	}
	in.Done = m.cmdDone(func(v string) tea.Cmd {
		write := typeIn(v)
		if !search || v == "" {
			return write
		}
		// A search box: what was typed is what the user came to submit, so
		// the box's Enter offers that at once — the page's own Enter in the
		// field. Esc keeps the value, unsent.
		return tea.Batch(write, m.a.Confirm(terminu.Ask{Title: "Search", Details: []string{v},
			Question: "Press [Enter] in the field, so the page searches?", Verb: "search"},
			m.do(func() tea.Cmd { return m.submit(t, n) })))
	})
	return m.a.Text(in)
}

// cleanOld is a field's value as a one-line box can offer it: its line
// breaks and tabs as spaces.
func cleanOld(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, v)
}

// slideMenu lists a slider's numbers, the cursor on where it stands;
// picking one moves the slider there. A bar is chosen along, not typed
// into (user, 2026-09-23).
func (m *AppModel) slideMenu(t *tab, n *ir.Node) tea.Cmd {
	values, at := sliderValues(n)
	id := n.ID
	return m.a.Select(terminu.Select{Title: "Slide", Subject: oneLine(nameOr(n.Name, "slider")) + " · " + sliderRange(n),
		Glyph: glyphPencil, Options: values, Selected: at,
		Done: func(i int) tea.Cmd {
			if i < 0 || i >= len(values) {
				return nil
			}
			v := values[i]
			cmd := t.press(func(ctx context.Context) error { return page.Slide(ctx, id, v) })
			m.sync()
			return cmd
		}})
}

// fieldTakes is what the box over a field says it wants: the page's own
// word for it, where the page gave one — a terminal shows nothing of a
// field's type, where a browser shows a date picker or a number stepper.
func fieldTakes(n *ir.Node) string {
	if n.Protected {
		return "password"
	}
	switch t := n.InputType; t {
	case "":
		switch {
		case n.Multiline:
			return "text, several lines"
		case n.Role == "searchbox":
			return "search"
		case n.Role == "spinbutton":
			return "number"
		}
		return "text"
	case "tel":
		return "phone number"
	case "datetime-local":
		return "date and time"
	default:
		return t
	}
}
