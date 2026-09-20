package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/ir"
)

// The Outline popup (ui.md §3.1): the page's landmarks and headings, indented
// by nesting, Enter jumps panel [3] to the one chosen. It is a menu, so it
// is a spaceMenu instance with its own animator — the same reuse sshu makes
// for its pickers — and the rows are built here.

func newOutlineMenu() spaceMenu { return spaceMenu{anim: newPopupAnimator("outline")} }

// outlineEntry is one row: the node it stands for and its label.
type outlineEntry struct {
	node  *ir.Node
	label string
}

// outlineEntries walks the page for landmarks and headings. Depth grows
// inside a landmark only; a heading's own level is in its label.
func outlineEntries(root *ir.Node) []outlineEntry {
	var out []outlineEntry
	var walk func(n *ir.Node, depth int)
	walk = func(n *ir.Node, depth int) {
		switch n.Kind {
		case ir.Landmark:
			label := n.Role
			if n.Name != "" {
				label += " " + oneLine(n.Name)
			}
			out = append(out, outlineEntry{node: n, label: strings.Repeat("  ", depth) + label})
			depth++
		case ir.Heading:
			text := oneLine(n.Text())
			if text == "" {
				text = oneLine(n.Name)
			}
			out = append(out, outlineEntry{node: n,
				label: strings.Repeat("  ", depth) + strings.Repeat("#", clamp(n.Level, 1, 6)) + " " + text})
		}
		for _, c := range n.Children {
			walk(c, depth)
		}
	}
	walk(root, 0)
	return out
}

// openOutline builds the popup for the shown page.
func (m *AppModel) openOutline() tea.Cmd {
	t := m.shownTab()
	if t == nil || t.root == nil {
		return m.toast.show("no page to outline", toastInfo)
	}
	m.outlineFor = outlineEntries(t.root)
	items := make([]menuItem, 0, len(m.outlineFor))
	for i, e := range m.outlineFor {
		items = append(items, menuItem{label: e.label, key: strconv.Itoa(i)})
	}
	if len(items) == 0 {
		items = append(items, menuItem{header: true, label: "no landmarks or headings on this page"})
	}
	m.outline.setItems(items, "Outline", m.layer())
	// Open on the entry nearest the cursor, so the popup says where you
	// are before it says where you could go.
	if n := t.current(); n != nil {
		if row := t.lay.items[t.cursor].first; row >= 0 {
			for i, e := range m.outlineFor {
				if r, ok := t.lay.marks[e.node]; ok && r <= row {
					m.outline.cursor = i
				}
			}
		}
	}
	return m.outline.open()
}

func (m AppModel) outlineKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var key string
	m.outline, key, _ = m.outline.update(msg)
	if key == "" {
		return m, nil
	}
	closeCmd := m.outline.close()
	i, err := strconv.Atoi(key)
	t := m.shownTab()
	if err != nil || i < 0 || i >= len(m.outlineFor) || t == nil {
		return m, closeCmd
	}
	t.jumpTo(m.outlineFor[i].node, m.pageVisible())
	m.focus = panel3
	return m, closeCmd
}

// jumpTo puts the cursor on node — the heading itself when it is an item,
// else the first item at or after the node's row — and scrolls so the
// node's row is at the top.
func (t *tab) jumpTo(n *ir.Node, visible int) {
	row, ok := t.lay.marks[n]
	if !ok {
		return
	}
	t.cursor = -1
	for i, it := range t.lay.items {
		if it.node == n || it.first >= row {
			t.cursor = i
			break
		}
	}
	if t.cursor < 0 && len(t.lay.items) > 0 {
		t.cursor = len(t.lay.items) - 1
	}
	t.top = clamp(row, 0, max(0, len(t.lay.rows)-1))
	if t.cursor >= 0 {
		it := t.lay.items[t.cursor]
		if it.last >= t.top+visible {
			t.top = max(0, it.last-visible+1)
		}
	}
}
