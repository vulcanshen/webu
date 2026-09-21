// Package store owns webu's own files (ui.md §6): bookmarks and settings
// under the config dir, the history and the session under the data dir.
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
// in an old file is ignored.
type Config struct {
	SearchEngine string `yaml:"search_engine,omitempty"`
	DownloadDir  string `yaml:"download_dir,omitempty"`
	Measure      int    `yaml:"measure,omitempty"` // text width cap in panel [2]; 0 is the default
}

// DefaultMeasure is how wide a paragraph flows before it wraps, whatever
// the terminal: past a hundred cells the eye loses the line.
const DefaultMeasure = 100

// TextWidth is the measure in force.
func (c Config) TextWidth() int {
	if c.Measure <= 0 {
		return DefaultMeasure
	}
	return c.Measure
}

// DefaultSearch is where a goto that is not a URL goes (ux.md §7).
const DefaultSearch = "https://duckduckgo.com/?q="

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

// dataFiles are what webu writes as it runs; they live under paths.Data.
// The rest — what the user writes — is under paths.Config (2026-09-21).
var dataFiles = map[string]bool{"history.yaml": true, "session.yaml": true}

func path(name string) (string, error) {
	dir, err := paths.Config()
	if dataFiles[name] {
		dir, err = paths.Data()
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// ---------------------------------------------------------------- yaml

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
	err := loadYAML("bookmarks.yaml", &f)
	return f.Bookmarks, f.Folders, err
}

func SaveBookmarks(list []Bookmark, folders []string) error {
	return saveYAML("bookmarks.yaml", bookmarksFile{Bookmarks: list, Folders: folders})
}

func LoadConfig() (Config, error) {
	var c Config
	err := loadYAML("config.yaml", &c)
	return c, err
}

func SaveConfig(c Config) error { return saveYAML("config.yaml", c) }

func LoadSession() (Session, error) {
	var s Session
	err := loadYAML("session.yaml", &s)
	return s, err
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
