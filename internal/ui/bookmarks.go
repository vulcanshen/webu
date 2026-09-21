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
// picker show them as a tree, indented by depth; Enter on a folder's row
// folds it.

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

// underFold reports whether some folder above f (not f itself) is folded,
// which hides f and everything in it.
func (m AppModel) underFold(f string) bool {
	parts := strings.Split(f, "/")
	for i := 1; i < len(parts); i++ {
		if m.foldedFolders[strings.Join(parts[:i], "/")] {
			return true
		}
	}
	return false
}

// bookmarkEntries is the Bookmarks screen's rows: the top level first,
// then each folder as a row of its own with what is in it under it, the
// folders' rows indented by depth. A folded folder is one row, with a
// count of what it hides.
func (m AppModel) bookmarkEntries() []listEntry {
	var out []listEntry
	for i, b := range m.bookmarks {
		if b.Folder == "" {
			out = append(out, listEntry{title: b.Title, url: b.URL, ref: i})
		}
	}
	for _, f := range m.folderNames() {
		if m.underFold(f) {
			continue
		}
		e := listEntry{title: folderBase(f), folder: f, isFolder: true, ref: -1, depth: folderDepth(f) - 1}
		if m.foldedFolders[f] {
			e.folded = true
			for _, b := range m.bookmarks {
				if inFolder(b.Folder, f) {
					e.count++
				}
			}
			out = append(out, e)
			continue
		}
		out = append(out, e)
		for i, b := range m.bookmarks {
			if b.Folder == f {
				out = append(out, listEntry{title: b.Title, url: b.URL, folder: f, ref: i, depth: folderDepth(f)})
			}
		}
	}
	return out
}

// toggleFolder is Enter on a folder row: shut, or open again.
func (m *AppModel) toggleFolder(path string) {
	if m.foldedFolders == nil {
		m.foldedFolders = map[string]bool{}
	}
	m.foldedFolders[path] = !m.foldedFolders[path]
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorToFolder(path)
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

// addFolder is the answer to A: a folder under parent — where the cursor
// was — given as a name or a path: "dir1/dir2/dir3" makes all three at
// once (revised 2026-09-21).
func (m *AppModel) addFolder(parent, path string) tea.Cmd {
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	full := strings.Join(parts, "/")
	if parent != "" {
		full = parent + "/" + full
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
	return m.toast.show("folder "+path+" removed", toastInfo)
}

// ---------------------------------------------------------- adding one

// startAddBookmark is a on the Bookmarks screen: a bookmark typed in, in
// two boxes — the URL, then the title — into the folder the cursor is in.
// The page [W]eb is showing is on offer in both, so adding the current
// page is a, Enter, Enter: here Enter on the untouched offer TAKES it,
// because the offer is the whole point of the box (unlike Location, where
// the offer is the page you are already on).
func (m *AppModel) startAddBookmark(folder string) tea.Cmd {
	m.newBookmark = store.Bookmark{Folder: folder}
	p := inputPopup{title: "Add bookmark", glyph: glyphBookmark, prompt: "URL",
		accept: "next", action: inputBookmarkURL}
	if t := m.shownTab(); t != nil && t.url != "" && t.url != "about:blank" {
		p.placeholder = t.url
	}
	return m.input.ask(p, m.layer())
}

// bookmarkURLGiven is the first box answered: the title box follows.
func (m *AppModel) bookmarkURLGiven(value string) tea.Cmd {
	url := strings.TrimSpace(value)
	if url == "" {
		url = m.input.placeholder
	}
	if url == "" {
		return m.input.close()
	}
	url = m.resolveURL(url)
	m.newBookmark.URL = url
	title := url
	if t := m.shownTab(); t != nil && t.url == url && t.title != "" {
		title = t.title
	}
	return tea.Batch(m.input.close(), m.input.ask(inputPopup{title: "Add bookmark", glyph: glyphBookmark,
		prompt: "title", placeholder: title, accept: "add", action: inputBookmarkTitle}, m.layer()))
}

// bookmarkTitleGiven is the second box answered: the bookmark is written.
func (m *AppModel) bookmarkTitleGiven(value string) tea.Cmd {
	title := strings.TrimSpace(value)
	if title == "" {
		title = m.input.placeholder
	}
	b := m.newBookmark
	b.Title = title
	for _, x := range m.bookmarks {
		if x.URL == b.URL {
			return tea.Batch(m.input.close(), m.toast.show("already bookmarked", toastInfo))
		}
	}
	m.bookmarks = append(m.bookmarks, b)
	if err := store.SaveBookmarks(m.bookmarks, m.folders); err != nil {
		return tea.Batch(m.input.close(), m.toast.show("bookmarks.yaml: "+err.Error(), toastError))
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorTo(len(m.bookmarks) - 1)
	return tea.Batch(m.input.close(), m.toast.show("added "+oneLine(nameOr(b.Title, b.URL)), toastInfo))
}
