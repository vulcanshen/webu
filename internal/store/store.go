// Package store owns webu's own files (ui.md §6): the settings and the
// session under the config dir, the bookmarks and the history under the
// data dir.
// Every one is small and hand-editable; every loader treats a missing file
// as the empty state and a broken one as news, never as a reason not to
// start.
package store

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vulcanshen/webu/internal/paths"
	"gopkg.in/yaml.v3"
)

// Bookmark is one entry of bookmarks.yaml. Folders (ui.md §3.1) are a flat
// string for now — the tree is drawn later; the file format already has a
// place for it so nothing has to migrate.
type Bookmark struct {
	Title  string `yaml:"title"`
	URL    string `yaml:"url"`
	Folder string `yaml:"folder,omitempty"`
}

// Config is config.yaml, hand-written. webu reads it and does not write
// it back — a shortcuts list used to live here and be edited from a popup;
// it went with the Places panel (2026-09-20), and a `shortcuts:` key left
// in an old file is ignored; so is `measure:`, the width a paragraph
// wrapped at, which the reader never honoured (removed 2026-10-29, user).
type Config struct {
	SearchEngine string `yaml:"search_engine,omitempty"`
	DownloadDir  string `yaml:"download_dir,omitempty"`
	// RestoreSession: reopen the tabs that were open when webu last quit.
	// Absent means yes; a pointer so that "not set" and "set to false"
	// are told apart, since the default is the true side (2026-09-21).
	RestoreSession *bool `yaml:"restore_session,omitempty"`
	// HideSecretsInPreview is the family's key (tdp E3): the file-picker's
	// preview shows only the first line of a private key, and nothing of a
	// file only its owner may read.
	HideSecretsInPreview bool `yaml:"hide_secrets_in_preview,omitempty"`
	// InputTabWidth is the family's key (tdp E3): how many spaces Tab puts
	// in a textarea; 0 is the default, 4.
	InputTabWidth int `yaml:"input_tab_width,omitempty"`
}

// Restore reports whether the last session's tabs come back on launch.
func (c Config) Restore() bool {
	return c.RestoreSession == nil || *c.RestoreSession
}

// TabWidth is input_tab_width in force: 4 when unset.
func (c Config) TabWidth() int {
	if c.InputTabWidth > 0 {
		return c.InputTabWidth
	}
	return 4
}

// DefaultSearch is where a goto that is not a URL goes (ux.md §7).
//
// DuckDuckGo's HTML endpoint (2026-09-23): Google was the default from
// 2026-09-21, and Google answers a headless Chromium's search with its
// reCAPTCHA — even once webu signs its own user agent — which is a page
// webu can only draw a placeholder for. The html endpoint is the one
// DuckDuckGo keeps for clients without JavaScript, and of the twelve
// engines tried it draws the cleanest page (measured, 2026-09-23).
const DefaultSearch = "https://html.duckduckgo.com/html/?q="

// Search is the engine prefix in force.
func (c Config) Search() string {
	if c.SearchEngine == "" {
		return DefaultSearch
	}
	return c.SearchEngine
}

// Visit is one line of the history log.
type Visit struct {
	At    time.Time
	URL   string
	Title string
}

// Session is what was open when webu last quit.
type Session struct {
	Tabs  []SessionTab `yaml:"tabs"`
	Shown int          `yaml:"shown"`
}

type SessionTab struct {
	URL   string `yaml:"url"`
	Title string `yaml:"title,omitempty"`
}

// files say where each file goes (tdp E3, what deleting it would cost) and
// where an older webu kept it, if somewhere else: config.yaml is a
// setting; session.yaml is state, under paths.Data until 2026-10-19;
// bookmarks.yaml and history.yaml are data, bookmarks.yaml under
// paths.Config until 2026-10-19. cookies.json, the reader's sign-ins
// (docs/reader.md), is state: deleted, you are signed out, and sign in
// again.
var files = map[string]struct{ dir, old func() (string, error) }{
	"config.yaml":    {dir: paths.Config},
	"session.yaml":   {dir: paths.State, old: paths.Data},
	"bookmarks.yaml": {dir: paths.Data, old: paths.Config},
	"history.yaml":   {dir: paths.Data},
	"cookies.json":   {dir: paths.State},
}

// path is where name is read and written: where it goes, or where an older
// webu kept it while Relocate could not move it from there.
func path(name string) (string, error) {
	p, old, err := places(name)
	if err != nil {
		return "", err
	}
	return paths.Either(p, old), nil
}

// Path is where one of webu's files is read and written ("" when the
// directories cannot be told), for saying which file could not be read.
func Path(name string) string {
	p, _ := path(name)
	return p
}

// places are where name goes and where an older webu kept it ("" when
// that is the same place).
func places(name string) (string, string, error) {
	f := files[name]
	dir, err := f.dir()
	if err != nil {
		return "", "", err
	}
	p := filepath.Join(dir, name)
	if f.old == nil {
		return p, "", nil
	}
	odir, err := f.old()
	if err != nil {
		return p, "", nil
	}
	if old := filepath.Join(odir, name); old != p {
		return p, old, nil
	}
	return p, "", nil
}

// Relocate moves to where they go now the files an older webu kept
// somewhere else (tdp E3, on the first start), one by one. One that cannot
// be moved stays, and is read and written where it is; one already in its
// new place stays in the old one too, for the user to merge. An old
// directory a move left empty goes.
func Relocate() []paths.Move {
	var moved []paths.Move
	var left []string
	for _, name := range []string{"bookmarks.yaml", "session.yaml"} {
		p, old, err := places(name)
		if err != nil {
			continue
		}
		if mv, ok := paths.Relocate(name, old, p); ok {
			moved = append(moved, mv)
			if mv.Err == nil && !mv.Both {
				left = append(left, filepath.Dir(old))
			}
		}
	}
	for _, dir := range left {
		paths.RemoveIfEmpty(dir)
	}
	return moved
}

// ---------------------------------------------------------------- yaml

// loadYAML reads one of webu's files. A Load that gets an error from it
// returns nothing of the file: yaml fills in what it read before the
// error, and half a file acted on while MOVE ASIDE is still being asked
// is not the first start that Enter promises (tdp E3).
func loadYAML(name string, into any) error {
	p, err := path(name)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return yaml.Unmarshal(raw, into)
}

// saveYAML writes atomically: a file half-written when the machine goes
// down is a file that will not load.
func saveYAML(name string, v any) error {
	p, err := path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// bookmarksFile is bookmarks.yaml: the bookmarks, each naming its folder,
// and the folders themselves — so a folder made before anything is put in
// it survives (2026-09-21).
type bookmarksFile struct {
	Bookmarks []Bookmark `yaml:"bookmarks"`
	Folders   []string   `yaml:"folders,omitempty"`
}

// LoadBookmarks returns the bookmarks and the folders declared on their own.
func LoadBookmarks() ([]Bookmark, []string, error) {
	var f bookmarksFile
	if err := loadYAML("bookmarks.yaml", &f); err != nil {
		return nil, nil, err
	}
	return f.Bookmarks, f.Folders, nil
}

func SaveBookmarks(list []Bookmark, folders []string) error {
	return saveYAML("bookmarks.yaml", bookmarksFile{Bookmarks: list, Folders: folders})
}

func LoadConfig() (Config, error) {
	var c Config
	if err := loadYAML("config.yaml", &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func SaveConfig(c Config) error { return saveYAML("config.yaml", c) }

func LoadSession() (Session, error) {
	var s Session
	if err := loadYAML("session.yaml", &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

func SaveSession(s Session) error { return saveYAML("session.yaml", s) }

// -------------------------------------------------------------- history

// The history is history.yaml under the data dir (2026-09-21; it was a
// tab-separated log under the config dir): a YAML sequence, one visit
// appended at a time as its own block, so a crash mid-write costs at most
// one entry and the file is still one sequence any YAML reader takes
// whole. It is kept forever; the History screen's Clear is the only way it
// shrinks (ui.md §6).

type visitRec struct {
	At    time.Time `yaml:"at"`
	URL   string    `yaml:"url"`
	Title string    `yaml:"title,omitempty"`
}

func AppendVisit(v Visit) error {
	p, err := path("history.yaml")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal([]visitRec{{At: v.At.UTC(), URL: clean(v.URL), Title: clean(v.Title)}})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(raw)
	return err
}

// clean keeps a title or URL on one line: a tab or newline inside one
// would be a second YAML line with no meaning.
func clean(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// LoadHistory returns every visit, newest first.
func LoadHistory() ([]Visit, error) {
	p, err := path("history.yaml")
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var recs []visitRec
	if err := yaml.Unmarshal(raw, &recs); err != nil {
		return nil, err
	}
	out := make([]Visit, 0, len(recs))
	for i := len(recs) - 1; i >= 0; i-- {
		r := recs[i]
		out = append(out, Visit{At: r.At, URL: r.URL, Title: r.Title})
	}
	return out, nil
}

// ClearHistory empties the log.
func ClearHistory() error {
	p, err := path("history.yaml")
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// DeleteVisit removes every entry matching the visit (same time and URL)
// and writes the rest back, oldest first as they were.
func DeleteVisit(v Visit) error {
	all, err := LoadHistory()
	if err != nil {
		return err
	}
	var keep []visitRec
	for i := len(all) - 1; i >= 0; i-- {
		x := all[i]
		if x.URL == v.URL && x.At.Equal(v.At) {
			continue
		}
		keep = append(keep, visitRec{At: x.At.UTC(), URL: x.URL, Title: x.Title})
	}
	if len(keep) == 0 {
		return ClearHistory()
	}
	return saveYAML("history.yaml", keep)
}
