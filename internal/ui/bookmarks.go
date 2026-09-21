package ui

import (
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/store"
)

// The Bookmarks screen's folders (ui.md §2, 2026-09-21): a path on each
// bookmark — "dev/go" is go inside dev — plus the paths declared on their
// own in bookmarks.yaml so an empty folder exists. The screen and the Move
// picker show them as a tree, indented by depth; folding the tree is v2.

// folderNames is every folder there is — declared, in use by a bookmark,
// or an ancestor of either — in tree order: a parent right before what is
// in it.
func (m AppModel) folderNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		parts := strings.Split(f, "/")
		for i := range parts {
			p := strings.Join(parts[:i+1], "/")
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	for _, f := range m.folders {
		add(f)
	}
	for _, b := range m.bookmarks {
		add(b.Folder)
	}
	sort.Slice(out, func(i, j int) bool { return folderLess(out[i], out[j]) })
	return out
}

// folderLess orders paths component by component, so "a" comes before
// "a/b" and "a/b" before "a-x" — the tree, not the bytes.
func folderLess(a, b string) bool {
	pa, pb := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

// folderDepth is how deep a path is: 0 for the top level, 1 for "dev".
func folderDepth(f string) int {
	if f == "" {
		return 0
	}
	return strings.Count(f, "/") + 1
}

// folderBase is a path's own name.
func folderBase(f string) string {
	if i := strings.LastIndex(f, "/"); i >= 0 {
		return f[i+1:]
	}
	return f
}

// inFolder reports whether path is f or inside it.
func inFolder(path, f string) bool {
	return path == f || strings.HasPrefix(path, f+"/")
}

// bookmarkEntries is the Bookmarks screen's rows: the top level first,
// then each folder as a row of its own with what is in it under it, the
// folders' rows indented by depth.
func (m AppModel) bookmarkEntries() []listEntry {
	var out []listEntry
	for i, b := range m.bookmarks {
		if b.Folder == "" {
			out = append(out, listEntry{title: b.Title, url: b.URL, ref: i})
		}
	}
	for _, f := range m.folderNames() {
		out = append(out, listEntry{title: folderBase(f), folder: f, isFolder: true, ref: -1, depth: folderDepth(f) - 1})
		for i, b := range m.bookmarks {
			if b.Folder == f {
				out = append(out, listEntry{title: b.Title, url: b.URL, folder: f, ref: i, depth: folderDepth(f)})
			}
		}
	}
	return out
}

// movePicker is m on a bookmark: the folders to move it to as a tree,
// keyed by number like a select's options, the top level first.
func (m *AppModel) movePicker(ref int) tea.Cmd {
	if ref < 0 || ref >= len(m.bookmarks) {
		return nil
	}
	m.moveRef = ref
	m.optionsFor, m.optionsKind = nil, optMoveTo
	now := m.bookmarks[ref].Folder
	items := []menuItem{{label: "(no folder)", key: "0", hint: "the top level"}}
	if now == "" {
		items[0].hint = "here now"
	}
	for i, f := range m.folderNames() {
		hint := ""
		if f == now {
			hint = "here now"
		}
		label := strings.Repeat("  ", folderDepth(f)-1) + folderBase(f)
		items = append(items, menuItem{label: label, key: strconv.Itoa(i + 1), hint: hint})
	}
	m.options.setItems(items, "Move to…", m.layer())
	return m.options.open()
}

// moveBookmark puts bookmark ref in the folder the picker chose (0 is the
// top level), writes the file, and keeps the cursor on it.
func (m *AppModel) moveBookmark(ref, idx int) tea.Cmd {
	if ref < 0 || ref >= len(m.bookmarks) {
		return nil
	}
	folder := ""
	if names := m.folderNames(); idx > 0 && idx <= len(names) {
		folder = names[idx-1]
	}
	m.bookmarks[ref].Folder = folder
	if err := store.SaveBookmarks(m.bookmarks, m.folders); err != nil {
		return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorTo(ref)
	return nil
}

// addFolder is the answer to f or F: a new, empty folder, inside parent
// when there is one. A name is one component — a slash would be a path,
// and a path is made one folder at a time.
func (m *AppModel) addFolder(parent, name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if strings.Contains(name, "/") {
		return m.toast.show("a name, not a path: make the folders one at a time", toastInfo)
	}
	full := name
	if parent != "" {
		full = parent + "/" + name
	}
	for _, f := range m.folderNames() {
		if f == full {
			return m.toast.show("already a folder", toastInfo)
		}
	}
	m.folders = append(m.folders, full)
	if err := store.SaveBookmarks(m.bookmarks, m.folders); err != nil {
		return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorToFolder(full)
	return m.toast.show("folder "+full, toastInfo)
}

// deleteFolder is x on a folder row: only an empty one goes — the
// bookmarks and folders in it are the user's, not the folder's.
func (m *AppModel) deleteFolder(path string) tea.Cmd {
	for _, b := range m.bookmarks {
		if inFolder(b.Folder, path) {
			return m.toast.show("not empty: move its bookmarks out first", toastInfo)
		}
	}
	for _, f := range m.folderNames() {
		if f != path && inFolder(f, path) {
			return m.toast.show("not empty: delete the folders inside it first", toastInfo)
		}
	}
	kept := m.folders[:0]
	for _, f := range m.folders {
		if f != path {
			kept = append(kept, f)
		}
	}
	m.folders = kept
	if err := store.SaveBookmarks(m.bookmarks, m.folders); err != nil {
		return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.bookmarkEntries())
	return nil
}
