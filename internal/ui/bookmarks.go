package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/page"
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

// saveBookmarks writes bookmarks.yaml with every folder there is declared
// — the ones made by hand, the ones a path implied, the ones a bookmark
// sits in — so a folder, once it exists, exists until x on its row. It
// used to write only the declared ones, and deleting "x/y" then took the
// "x" that only y had implied along with it (2026-09-21).
func (m *AppModel) saveBookmarks() error {
	m.folders = m.folderNames()
	return store.SaveBookmarks(m.bookmarks, m.folders)
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
	if err := m.saveBookmarks(); err != nil {
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
	if err := m.saveBookmarks(); err != nil {
		return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorToFolder(full)
	return m.toast.show("folder "+full, toastInfo)
}

// deleteFolder is x on a folder row. An empty one goes at once; one with
// anything in it asks first — how many bookmarks and folders go with it
// — and then the whole tree goes (revised 2026-09-21: an import lands as
// one tree, and row by row was no way to undo it; before, a folder with
// anything in it was refused). Only that tree: the folders above it
// stay, empty or not.
func (m *AppModel) deleteFolder(path string) tea.Cmd {
	books, folders := 0, 0
	for _, b := range m.bookmarks {
		if inFolder(b.Folder, path) {
			books++
		}
	}
	for _, f := range m.folderNames() {
		if f != path && inFolder(f, path) {
			folders++
		}
	}
	if books+folders == 0 {
		return m.deleteFolderTree(path)
	}
	return m.confirm.ask(confirmPopup{glyph: glyphWarn, title: "Delete folder",
		lines:  []string{path, "and everything in it: " + plural(books, "bookmark") + ", " + plural(folders, "folder")},
		accept: "delete", warn: true, action: confirmDeleteFolder, folder: path}, m.layer()+1)
}

// deleteFolderTree takes path and everything under it out, and writes
// the file.
func (m *AppModel) deleteFolderTree(path string) tea.Cmd {
	books := 0
	kept := m.bookmarks[:0]
	for _, b := range m.bookmarks {
		if inFolder(b.Folder, path) {
			books++
			continue
		}
		kept = append(kept, b)
	}
	m.bookmarks = kept
	keptFolders := m.folders[:0]
	for _, f := range m.folders {
		if !inFolder(f, path) {
			keptFolders = append(keptFolders, f)
		}
	}
	m.folders = keptFolders
	if err := m.saveBookmarks(); err != nil {
		return m.toast.show("bookmarks.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.bookmarkEntries())
	msg := "folder " + path + " removed"
	if books > 0 {
		msg += ", " + plural(books, "bookmark") + " with it"
	}
	return m.toast.show(msg, toastInfo)
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
	if err := m.saveBookmarks(); err != nil {
		return tea.Batch(m.input.close(), m.toast.show("bookmarks.yaml: "+err.Error(), toastError))
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorTo(len(m.bookmarks) - 1)
	return tea.Batch(m.input.close(), m.toast.show("added "+oneLine(nameOr(b.Title, b.URL)), toastInfo))
}

// ---- Import (2026-09-21): a browser's export, into a folder of its own

// importDir is where the picker opens for a browser's export: Downloads,
// where every browser writes one, else home.
func importDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	if st, err := os.Stat(filepath.Join(home, "Downloads")); err == nil && st.IsDir() {
		return filepath.Join(home, "Downloads")
	}
	return home
}

// startImport is I on the Bookmarks screen: the file first, picked rather
// than typed (filepicker.go).
func (m *AppModel) startImport() tea.Cmd {
	return m.picker.open(glyphBookmark, "Import bookmarks", importDir(), m.layer())
}

// pickerKey drives the file picker; a pick goes to whichever of the two
// things that open it is waiting — a page's file chooser, or the
// Bookmarks import.
func (m AppModel) pickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	path, done := m.picker.update(msg)
	if !done {
		return m, nil
	}
	closeCmd := m.picker.close()
	if m.upload != nil {
		return m, tea.Batch(closeCmd, m.uploadPicked(path))
	}
	cmd := m.importPicked(path)
	return m, tea.Batch(closeCmd, cmd)
}

// uploadPicked hands the chosen file to the page's chooser.
func (m *AppModel) uploadPicked(path string) tea.Cmd {
	f := m.upload
	m.upload = nil
	if f == nil {
		return nil
	}
	_, ft := m.tabByID(f.tabID)
	if ft == nil {
		return nil
	}
	node, files := f.node, []string{path}
	return ft.press(func(ctx context.Context) error { return page.SetFiles(ctx, node, files) })
}

// importPicked reads the chosen file at once — a wrong file is news here,
// not after a name was typed — and asks for the folder the bookmarks go
// under. The name is required: an import lands in a folder of its own,
// never loose among what is there.
func (m *AppModel) importPicked(path string) tea.Cmd {
	f, err := os.Open(path)
	if err != nil {
		return m.toast.show(err.Error(), toastError)
	}
	defer f.Close()
	imp, err := store.ParseNetscape(f)
	if err != nil {
		return m.toast.show(filepath.Base(path)+": "+err.Error(), toastError)
	}
	m.pendingImport = &imp
	prompt := fmt.Sprintf("%s: %s in %s. The folder to put them under (required)",
		filepath.Base(path), plural(len(imp.Bookmarks), "bookmark"), plural(len(imp.Folders), "folder"))
	return m.input.ask(inputPopup{title: "Import bookmarks", glyph: glyphFolder,
		prompt: prompt, accept: "import", action: inputImportName}, m.layer())
}

// importBookmarks puts the pending import under name — a folder that must
// not exist yet, so two imports never run together and one is undone by
// deleting its folder tree. An empty or a taken name keeps the box open.
func (m *AppModel) importBookmarks(name string) tea.Cmd {
	root := cleanFolderPath(name)
	if root == "" {
		return m.toast.show("a folder name is needed: the import goes under it", toastInfo)
	}
	for _, f := range m.folderNames() {
		if f == root {
			return m.toast.show("folder "+root+" exists; pick another name", toastInfo)
		}
	}
	imp := m.pendingImport
	if imp == nil {
		return m.input.close()
	}
	m.pendingImport = nil
	m.folders = append(m.folders, root)
	for _, f := range imp.Folders {
		m.folders = append(m.folders, root+"/"+f)
	}
	for _, b := range imp.Bookmarks {
		if b.Folder == "" {
			b.Folder = root
		} else {
			b.Folder = root + "/" + b.Folder
		}
		m.bookmarks = append(m.bookmarks, b)
	}
	closeCmd := m.input.close()
	if err := m.saveBookmarks(); err != nil {
		return tea.Batch(closeCmd, m.toast.show("bookmarks.yaml: "+err.Error(), toastError))
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorToFolder(root)
	return tea.Batch(closeCmd, m.toast.show(fmt.Sprintf("imported %s and %s under %s",
		plural(len(imp.Bookmarks), "bookmark"), plural(len(imp.Folders), "folder"), root), toastInfo))
}

// cleanFolderPath trims a typed folder path to its parts: blanks and
// empty levels dropped, so " a / /b " is a/b.
func cleanFolderPath(path string) string {
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "/")
}

// ---- Rename (2026-09-21): r on a bookmark's row, or a folder's

// startRename is r on the Bookmarks screen: a bookmark's title, or a
// folder's own name, in a box holding the current one to edit — most
// renames change part of a name (inputPopup.ask).
func (m *AppModel) startRename(e listEntry) tea.Cmd {
	if e.isFolder {
		m.renameFolder, m.renameRef = e.folder, -1
		return m.input.ask(inputPopup{title: "Rename folder", glyph: glyphFolder, prompt: "name",
			value: folderBase(e.folder), accept: "rename", action: inputRename}, m.layer())
	}
	if e.ref < 0 || e.ref >= len(m.bookmarks) {
		return nil
	}
	m.renameFolder, m.renameRef = "", e.ref
	return m.input.ask(inputPopup{title: "Rename bookmark", glyph: glyphBookmark, prompt: "title",
		value: m.bookmarks[e.ref].Title, accept: "rename", action: inputRename}, m.layer())
}

// renameGiven is the box answered. A bookmark takes the title. A folder
// takes the name — one level, no slash, not one already there — and
// every bookmark and folder under it follows to the new path, folded or
// not. An empty, taken or slashed name keeps the box open.
func (m *AppModel) renameGiven(value string) tea.Cmd {
	name := strings.TrimSpace(value)
	if name == "" {
		return m.toast.show("a name is needed", toastInfo)
	}
	if m.renameFolder == "" {
		if m.renameRef < 0 || m.renameRef >= len(m.bookmarks) {
			return m.input.close()
		}
		m.bookmarks[m.renameRef].Title = name
		if err := m.saveBookmarks(); err != nil {
			return tea.Batch(m.input.close(), m.toast.show("bookmarks.yaml: "+err.Error(), toastError))
		}
		m.lists.setEntries(m.bookmarkEntries())
		m.lists.cursorTo(m.renameRef)
		return tea.Batch(m.input.close(), m.toast.show("renamed to "+oneLine(name), toastInfo))
	}
	if strings.Contains(name, "/") {
		return m.toast.show("a name, not a path: the folder stays where it is", toastInfo)
	}
	old := m.renameFolder
	to := name
	if i := strings.LastIndex(old, "/"); i >= 0 {
		to = old[:i+1] + name
	}
	if to == old {
		return m.input.close()
	}
	for _, f := range m.folderNames() {
		if f == to {
			return m.toast.show("folder "+to+" exists; pick another name", toastInfo)
		}
	}
	move := func(p string) string { return to + strings.TrimPrefix(p, old) }
	for i, b := range m.bookmarks {
		if inFolder(b.Folder, old) {
			m.bookmarks[i].Folder = move(b.Folder)
		}
	}
	for i, f := range m.folders {
		if inFolder(f, old) {
			m.folders[i] = move(f)
		}
	}
	folded := map[string]bool{}
	for f, v := range m.foldedFolders {
		if inFolder(f, old) {
			f = move(f)
		}
		folded[f] = v
	}
	m.foldedFolders = folded
	if err := m.saveBookmarks(); err != nil {
		return tea.Batch(m.input.close(), m.toast.show("bookmarks.yaml: "+err.Error(), toastError))
	}
	m.lists.setEntries(m.bookmarkEntries())
	m.lists.cursorToFolder(to)
	return tea.Batch(m.input.close(), m.toast.show("folder "+old+" is now "+to, toastInfo))
}
