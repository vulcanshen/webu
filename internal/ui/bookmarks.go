package ui

import (
	"sort"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/store"
)

// The Bookmarks screen's folders (ui.md §2, 2026-09-21): a flat name on
// each bookmark, plus the names declared on their own in bookmarks.yaml so
// an empty folder exists. The screen groups by them; the tree with folds
// is still v2.

// folderNames is every folder there is, declared or in use, sorted.
func (m AppModel) folderNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	for _, f := range m.folders {
		add(f)
	}
	for _, b := range m.bookmarks {
		add(b.Folder)
	}
	sort.Strings(out)
	return out
}

// bookmarkEntries is the Bookmarks screen's rows: the top level first,
// then each folder as a row of its own with what is in it under it.
func (m AppModel) bookmarkEntries() []listEntry {
	var out []listEntry
	for i, b := range m.bookmarks {
		if b.Folder == "" {
			out = append(out, listEntry{title: b.Title, url: b.URL, ref: i})
		}
	}
	for _, f := range m.folderNames() {
		out = append(out, listEntry{title: f, folder: f, isFolder: true, ref: -1})
		for i, b := range m.bookmarks {
			if b.Folder == f {
				out = append(out, listEntry{title: b.Title, url: b.URL, folder: f, ref: i})
			}
		}
	}
	return out
}

// movePicker is m on a bookmark: the folders to move it to, keyed by
// number like a select's options, the top level first.
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
		items = append(items, menuItem{label: f, key: strconv.Itoa(i + 1), hint: hint})
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

// addFolder is the answer to F: a new, empty folder.
func (m *AppModel) addFolder(name string) tea.Cmd {
	if name == "" {
		return nil
	}
	for _, f := range m.folderNames() {
		if f == name {
			return m.toast.show("already a folder", toastInfo)
		}
	}
	m.folders = append(m.folders, name)
	if err := store.SaveBookmarks(m.bookmarks, m.folders); err != nil {
		return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.bookmarkEntries())
	return m.toast.show("folder "+name, toastInfo)
}

// deleteFolder is x on a folder row: only an empty one goes — the
// bookmarks in a folder are the user's, not the folder's.
func (m *AppModel) deleteFolder(name string) tea.Cmd {
	for _, b := range m.bookmarks {
		if b.Folder == name {
			return m.toast.show("not empty: move its bookmarks out first", toastInfo)
		}
	}
	kept := m.folders[:0]
	for _, f := range m.folders {
		if f != name {
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
