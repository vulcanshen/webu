package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/page"
	"github.com/vulcanshen/webu/internal/store"
)

// The Bookmarks screen's folders (ui.md §2, 2026-09-21): a path on each
// bookmark — "dev/go" is go inside dev — plus the paths declared on their
// own in bookmarks.yaml so an empty folder exists. The screen and the Move
// select show them as a tree, indented by depth; Enter on a folder's row
// folds it.

// folderNames is every folder there is — declared, in use by a bookmark,
// or an ancestor of either — in tree order: a parent right before what is
// in it.
func (m *AppModel) folderNames() []string {
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
func (m *AppModel) underFold(f string) bool {
	parts := strings.Split(f, "/")
	for i := 1; i < len(parts); i++ {
		if m.foldedFolders[strings.Join(parts[:i], "/")] {
			return true
		}
	}
	return false
}

// bmRow is one row of the Bookmarks screen: a bookmark (ref is its index
// in m.bookmarks) or a folder's own row.
type bmRow struct {
	ref      int
	folder   string // the bookmark's folder, or the folder itself
	isFolder bool
}

// bmRowID is a row's ID in the list: what the cursor is kept by.
func bmRowID(r bmRow) string {
	if r.isFolder {
		return "f:" + r.folder
	}
	return "b:" + strconv.Itoa(r.ref)
}

// buildBookmarks registers the Bookmarks screen (ui.md §2): a tree of
// folders and bookmarks, Enter opening a bookmark in a new tab or a folder
// open or shut.
func (m *AppModel) buildBookmarks() {
	m.bmList = &terminu.List{
		Columns:    []terminu.Column{{Title: "Title", Flex: true}, {Title: "URL", Flex: true}},
		Filterable: true,
		EmptyFact:  "No bookmarks yet",
		EmptyHint:  "Press [a] to add one, or [Space] to see what you can do here",
		// The delete questions name the row under the cursor.
		OnMove: func(terminu.Row) tea.Cmd { m.bmPanel.Actions = m.bookmarkActions(); return nil },
	}
	m.bmPanel = &terminu.Panel{Label: "Bookmarks", Body: m.bmList}
	m.bmPanel.Actions = m.bookmarkActions()
	m.bmScreen = m.a.AddScreen(&terminu.Screen{Name: "Bookmarks", Key: "B", Desc: "the pages kept, in folders",
		Panels: []*terminu.Panel{m.bmPanel}})
}

// bmCurrent is the row under the cursor.
func (m *AppModel) bmCurrent() (bmRow, bool) {
	r, ok := m.bmList.Current()
	if !ok {
		return bmRow{}, false
	}
	switch {
	case strings.HasPrefix(r.ID, "f:"):
		return bmRow{folder: strings.TrimPrefix(r.ID, "f:"), isFolder: true, ref: -1}, true
	case strings.HasPrefix(r.ID, "b:"):
		i, err := strconv.Atoi(strings.TrimPrefix(r.ID, "b:"))
		if err != nil || i < 0 || i >= len(m.bookmarks) {
			return bmRow{}, false
		}
		return bmRow{ref: i, folder: m.bookmarks[i].Folder}, true
	}
	return bmRow{}, false
}

// refreshBookmarks lists the bookmarks as the screen draws them: the top
// level first, then each folder as a row of its own with what is in it
// under it, indented by depth. A folded folder is one row, with a count of
// what it hides.
func (m *AppModel) refreshBookmarks() {
	if m.bmList == nil {
		return
	}
	var rows []terminu.Row
	book := func(i, depth int) {
		b := m.bookmarks[i]
		title := strings.Repeat("  ", depth) + oneLine(nameOr(b.Title, b.URL))
		rows = append(rows, terminu.Row{ID: bmRowID(bmRow{ref: i}), Cells: []string{title, oneLine(b.URL)}})
	}
	for i, b := range m.bookmarks {
		if b.Folder == "" {
			book(i, 0)
		}
	}
	for _, f := range m.folderNames() {
		if m.underFold(f) {
			continue
		}
		depth := folderDepth(f) - 1
		name := strings.Repeat("  ", depth) + glyphFolder + " " + folderBase(f)
		if m.foldedFolders[f] {
			n := 0
			for _, b := range m.bookmarks {
				if inFolder(b.Folder, f) {
					n++
				}
			}
			rows = append(rows, terminu.Row{ID: bmRowID(bmRow{folder: f, isFolder: true}),
				Cells: []string{name + " " + terminu.Fg(terminu.Overlay0).Render("("+itoa(n)+" shut)"), ""}})
			continue
		}
		rows = append(rows, terminu.Row{ID: bmRowID(bmRow{folder: f, isFolder: true}), Cells: []string{name, ""}})
		for i, b := range m.bookmarks {
			if b.Folder == f {
				book(i, folderDepth(f))
			}
		}
	}
	m.bmList.SetRows(rows)
}

// bookmarkActions are the Bookmarks screen's (ux.md §A.1).
func (m *AppModel) bookmarkActions() []terminu.Action {
	onFolder := func() bool { r, ok := m.bmCurrent(); return ok && r.isFolder }
	return []terminu.Action{
		{Key: "Enter", Label: "Open", Desc: "a bookmark in a new tab; a folder: open or shut", Scope: terminu.ItemScope, Hint: "open",
			Do: m.do(func() tea.Cmd {
				r, ok := m.bmCurrent()
				switch {
				case !ok:
					return nil
				case r.isFolder:
					m.toggleFolder(r.folder)
					return nil
				}
				// A bookmark opens in a NEW tab, never over the one the web
				// screen was showing (revised 2026-09-21).
				return m.openTabBehind(m.bookmarks[r.ref].URL)
			})},
		{Key: "m", Label: "move", Desc: "this bookmark to another folder", Scope: terminu.ItemScope, Hidden: onFolder,
			Do: m.do(func() tea.Cmd { r, _ := m.bmCurrent(); return m.moveSelect(r.ref) })},
		{Key: "r", Label: "rename", Desc: "a bookmark's title, or a folder's name", Scope: terminu.ItemScope, Hint: "rename",
			Do: m.do(func() tea.Cmd { r, _ := m.bmCurrent(); return m.startRename(r) })},
		{Key: "y", Label: "yank", Desc: "its URL", Scope: terminu.ItemScope, Hidden: onFolder,
			Do: func() tea.Cmd { r, _ := m.bmCurrent(); return copyToClipboard(m.bookmarks[r.ref].URL) }},
		{Key: "x", Label: "Delete", Desc: "this bookmark", Scope: terminu.ItemScope, Hidden: onFolder,
			Ask: m.deleteAsk(),
			Do: m.do(func() tea.Cmd {
				if r, ok := m.bmCurrent(); ok && !r.isFolder {
					return m.deleteBookmark(m.bookmarks[r.ref])
				}
				return nil
			})},
		{Key: "x", Label: "Delete folder", Desc: "this folder and everything in it", Scope: terminu.ItemScope,
			Hidden: func() bool { return !onFolder() }, Ask: m.deleteAsk(),
			Do: m.do(func() tea.Cmd {
				if r, ok := m.bmCurrent(); ok && r.isFolder {
					return m.deleteFolderTree(r.folder)
				}
				return nil
			})},
		{Key: "a", Label: "add", Desc: "a bookmark, in the folder under the cursor", Scope: terminu.PanelScope, Hint: "add",
			Do: m.do(func() tea.Cmd {
				folder := ""
				if r, ok := m.bmCurrent(); ok {
					folder = r.folder
				}
				return m.startAddBookmark(folder)
			})},
		{Key: "A", Label: "Add folder", Desc: "a path like a/b/c makes each level", Scope: terminu.PanelScope,
			Do: m.do(func() tea.Cmd {
				parent := ""
				if r, ok := m.bmCurrent(); ok {
					parent = r.folder
				}
				return m.addFolderBox(parent)
			})},
		{Key: "I", Label: "Import", Desc: "a browser's exported bookmarks", Scope: terminu.PanelScope, Do: m.startImport},
	}
}

// deleteAsk is x's question for the row under the cursor (tdp F6): what
// goes, every time — a folder with what is in it.
func (m *AppModel) deleteAsk() *terminu.Ask {
	r, ok := m.bmCurrent()
	switch {
	case !ok:
		return &terminu.Ask{Title: "Delete", Question: "Delete it?", Verb: "delete"}
	case r.isFolder:
		books, folders := 0, 0
		for _, b := range m.bookmarks {
			if inFolder(b.Folder, r.folder) {
				books++
			}
		}
		for _, f := range m.folderNames() {
			if f != r.folder && inFolder(f, r.folder) {
				folders++
			}
		}
		ask := &terminu.Ask{Title: "Delete folder", Subject: r.folder, Question: "Delete " + r.folder + "?", Verb: "delete"}
		if books+folders > 0 {
			ask.Details = []string{"And everything in it: " + plural(books, "bookmark") + ", " + plural(folders, "folder") + "."}
			ask.Warn = []int{0}
			ask.Question = "Delete " + r.folder + " and everything in it?"
		}
		return ask
	}
	b := m.bookmarks[r.ref]
	return &terminu.Ask{Title: "Delete", Subject: oneLine(nameOr(b.Title, b.URL)), Details: []string{b.URL},
		Question: "Delete this bookmark?", Verb: "delete"}
}

// toggleFolder is Enter on a folder row: shut, or open again.
func (m *AppModel) toggleFolder(path string) {
	if m.foldedFolders == nil {
		m.foldedFolders = map[string]bool{}
	}
	m.foldedFolders[path] = !m.foldedFolders[path]
	m.refreshBookmarks()
	m.bmList.Select(bmRowID(bmRow{folder: path, isFolder: true}))
}

// moveSelect is m on a bookmark: the folders to move it to as a tree, the
// top level first, the one it is in chosen.
func (m *AppModel) moveSelect(ref int) tea.Cmd {
	if ref < 0 || ref >= len(m.bookmarks) {
		return nil
	}
	now := m.bookmarks[ref].Folder
	names := m.folderNames()
	opts := []string{"(no folder)"}
	sel := 0
	for i, f := range names {
		if f == now {
			sel = i + 1
		}
		opts = append(opts, strings.Repeat("  ", folderDepth(f)-1)+folderBase(f))
	}
	return m.a.Select(terminu.Select{Title: "Move to", Subject: oneLine(nameOr(m.bookmarks[ref].Title, m.bookmarks[ref].URL)),
		Glyph: glyphFolder, Options: opts, Selected: sel,
		Done: func(i int) tea.Cmd {
			cmd := m.moveBookmark(ref, i)
			m.sync()
			return cmd
		}})
}

// moveBookmark puts bookmark ref in the folder the select chose (0 is the
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
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	m.bmList.Select(bmRowID(bmRow{ref: ref}))
	return nil
}

// addFolderBox is A: a folder where the cursor is, given as a name or a
// path — "dir1/dir2/dir3" makes all three at once (revised 2026-09-21).
func (m *AppModel) addFolderBox(parent string) tea.Cmd {
	prompt := "Folder to add; a path like a/b/c makes each level"
	if parent != "" {
		prompt += ", inside " + parent
	}
	full := func(v string) string {
		p := cleanFolderPath(v)
		if p != "" && parent != "" {
			p = parent + "/" + p
		}
		return p
	}
	return m.a.Text(terminu.TextInput{Title: "Add folder", Glyph: glyphFolder, Name: "The folder", Prompt: prompt, Verb: "create",
		Check: func(v string) error {
			f := full(v)
			if f == "" {
				return errors.New("The folder needs a name")
			}
			for _, x := range m.folderNames() {
				if x == f {
					return errors.New("The folder " + f + " is there already")
				}
			}
			return nil
		},
		Done: m.cmdDone(func(v string) tea.Cmd { return m.addFolder(full(v)) })})
}

// addFolder makes the folder full and writes the file.
func (m *AppModel) addFolder(full string) tea.Cmd {
	m.folders = append(m.folders, full)
	if err := m.saveBookmarks(); err != nil {
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	m.bmList.Select(bmRowID(bmRow{folder: full, isFolder: true}))
	return m.toast("Folder " + full)
}

// deleteFolderTree takes path and everything under it out, and writes the
// file.
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
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	msg := "Folder " + path + " removed"
	if books > 0 {
		msg += ", " + plural(books, "bookmark") + " with it"
	}
	return m.toast(msg)
}

// deleteBookmark takes one bookmark out and writes the file.
func (m *AppModel) deleteBookmark(b store.Bookmark) tea.Cmd {
	for i, x := range m.bookmarks {
		if x == b {
			m.bookmarks = append(m.bookmarks[:i], m.bookmarks[i+1:]...)
			break
		}
	}
	if err := m.saveBookmarks(); err != nil {
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	return nil
}

// addBookmark puts a page in Bookmarks, at the top level, and writes the
// file; a page already there is not added twice.
func (m *AppModel) addBookmark(title, url string) tea.Cmd {
	for _, b := range m.bookmarks {
		if b.URL == url {
			return m.toast("Already bookmarked")
		}
	}
	m.bookmarks = append(m.bookmarks, store.Bookmark{Title: title, URL: url})
	if err := m.saveBookmarks(); err != nil {
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	return m.toast("Added " + oneLine(nameOr(title, url)))
}

// ---------------------------------------------------------- adding one

// startAddBookmark is a on the Bookmarks screen: a bookmark typed in, its
// URL and its title, into the folder the cursor is in. The page the web
// screen shows fills both at first, so adding the current page is a, then
// the button.
func (m *AppModel) startAddBookmark(folder string) tea.Cmd {
	url, title := "", ""
	if t := m.shownTab(); t != nil && t.url != "" && t.url != "about:blank" {
		url, title = t.url, t.title
	}
	urlField := &terminu.FormField{Label: "URL", Stacked: true, Value: func() string { return url }, Empty: "not set",
		Edit: func() terminu.Input {
			return terminu.TextInput{Title: "URL", Glyph: glyphPencil, Name: "The URL", Prompt: "The page to keep", Verb: "set", Old: url,
				Done: func(v string) tea.Cmd { url = v; return nil }}
		},
		Check: func() string {
			if strings.TrimSpace(url) == "" {
				return "A bookmark needs a URL"
			}
			for _, x := range m.bookmarks {
				if x.URL == m.resolveURL(url) {
					return "Already bookmarked: " + x.URL
				}
			}
			return ""
		}}
	titleField := &terminu.FormField{Label: "Title", Value: func() string { return title }, Empty: "the URL",
		Edit: func() terminu.Input {
			return terminu.TextInput{Title: "Title", Glyph: glyphPencil, Name: "The title", Prompt: "What to call it", Verb: "set", Old: title,
				Done: func(v string) tea.Cmd { title = v; return nil }}
		}}
	where := "the top level"
	if folder != "" {
		where = folder
	}
	return m.a.Form(&terminu.Form{Title: "Add bookmark", Subject: where, Glyph: glyphBookmark, Name: "the new bookmark",
		Fields: []*terminu.FormField{urlField, titleField}, Button: "Add",
		Submit: func() (tea.Cmd, error) {
			b := store.Bookmark{Folder: folder, URL: m.resolveURL(url), Title: strings.TrimSpace(title)}
			if b.Title == "" {
				b.Title = b.URL
			}
			m.bookmarks = append(m.bookmarks, b)
			if err := m.saveBookmarks(); err != nil {
				return nil, errors.New("bookmarks.yaml: " + err.Error())
			}
			m.refreshBookmarks()
			m.bmList.Select(bmRowID(bmRow{ref: len(m.bookmarks) - 1}))
			return m.toast("Added " + oneLine(b.Title)), nil
		}})
}

// ---- Import (2026-09-21): a browser's export, into a folder of its own

// importDir is where the file-picker opens for a browser's export, and for
// a page's upload: Downloads, where every browser writes one, else home.
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
// than typed.
func (m *AppModel) startImport() tea.Cmd {
	return m.a.PickFile(terminu.FilePicker{Title: "Import bookmarks", Glyph: glyphBookmark, Root: importDir(),
		Done: m.cmdDone(m.importPicked)})
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
		return m.a.Error("Import failed", err.Error())
	}
	defer f.Close()
	imp, err := store.ParseNetscape(f)
	if err != nil {
		return m.a.Error("Import failed", filepath.Base(path)+": "+err.Error())
	}
	prompt := fmt.Sprintf("%s: %s in %s. The folder to put them under",
		filepath.Base(path), plural(len(imp.Bookmarks), "bookmark"), plural(len(imp.Folders), "folder"))
	return m.a.Text(terminu.TextInput{Title: "Import bookmarks", Glyph: glyphFolder, Name: "The folder", Prompt: prompt, Verb: "import",
		Check: func(v string) error {
			root := cleanFolderPath(v)
			if root == "" {
				return errors.New("The folder needs a name: the import goes under it")
			}
			for _, x := range m.folderNames() {
				if x == root {
					return errors.New("The folder " + root + " is there already")
				}
			}
			return nil
		},
		Done: m.cmdDone(func(v string) tea.Cmd { return m.importBookmarks(cleanFolderPath(v), imp) })})
}

// importBookmarks puts an import under root — a folder that did not exist,
// so two imports never run together and one is undone by deleting its
// folder tree.
func (m *AppModel) importBookmarks(root string, imp store.Import) tea.Cmd {
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
	if err := m.saveBookmarks(); err != nil {
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	m.bmList.Select(bmRowID(bmRow{folder: root, isFolder: true}))
	return m.toast(fmt.Sprintf("Imported %s and %s under %s",
		plural(len(imp.Bookmarks), "bookmark"), plural(len(imp.Folders), "folder"), root))
}

// cleanFolderPath trims a typed folder path to its parts: blanks and empty
// levels dropped, so " a / /b " is a/b.
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
// folder's own name, the current one on offer.
func (m *AppModel) startRename(r bmRow) tea.Cmd {
	if r.isFolder {
		old := r.folder
		to := func(v string) string {
			if i := strings.LastIndex(old, "/"); i >= 0 {
				return old[:i+1] + strings.TrimSpace(v)
			}
			return strings.TrimSpace(v)
		}
		return m.a.Text(terminu.TextInput{Title: "Rename folder", Subject: old, Glyph: glyphFolder, Name: "The name",
			Prompt: "New name for " + old + "; what is in it moves with it", Verb: "rename", Old: folderBase(old),
			Check: func(v string) error {
				switch {
				case strings.TrimSpace(v) == "":
					return errors.New("The name can't be empty")
				case strings.Contains(v, "/"):
					return errors.New("The name is a name, not a path: the folder stays where it is")
				}
				for _, f := range m.folderNames() {
					if f == to(v) && f != old {
						return errors.New("The folder " + f + " is there already")
					}
				}
				return nil
			},
			Done: m.cmdDone(func(v string) tea.Cmd { return m.renameFolder(old, to(v)) })})
	}
	if r.ref < 0 || r.ref >= len(m.bookmarks) {
		return nil
	}
	ref := r.ref
	return m.a.Text(terminu.TextInput{Title: "Rename bookmark", Subject: oneLine(m.bookmarks[ref].Title), Glyph: glyphBookmark,
		Name: "The title", Prompt: "New title for " + oneLine(nameOr(m.bookmarks[ref].Title, m.bookmarks[ref].URL)), Verb: "rename", Old: m.bookmarks[ref].Title,
		Check: func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("The title can't be empty")
			}
			return nil
		},
		Done: m.cmdDone(func(v string) tea.Cmd {
			m.bookmarks[ref].Title = v
			if err := m.saveBookmarks(); err != nil {
				return m.saveFailed("bookmarks.yaml", err)
			}
			m.refreshBookmarks()
			m.bmList.Select(bmRowID(bmRow{ref: ref}))
			return m.toast("Renamed to " + oneLine(v))
		})})
}

// renameFolder moves folder old to to: every bookmark and folder under it
// follows, folded or not.
func (m *AppModel) renameFolder(old, to string) tea.Cmd {
	if to == old {
		return nil
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
		return m.saveFailed("bookmarks.yaml", err)
	}
	m.refreshBookmarks()
	m.bmList.Select(bmRowID(bmRow{folder: to, isFolder: true}))
	return m.toast("Folder " + old + " is now " + to)
}
