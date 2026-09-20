// Package store owns webu's own files (ui.md §6): bookmarks, shortcuts and
// settings, the history log, and the session. Every one is small and
// hand-editable; every loader treats a missing file as the empty state and
// a broken one as news, never as a reason not to start.
package store

import (
	"bufio"
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

// Shortcut is one of the user's own quick links — the new-tab-page kind,
// flat and few (ui.md §3.1). They live in config.yaml.
type Shortcut struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

// Config is config.yaml. Read-only from webu's side except for shortcuts,
// which the Shortcuts popup edits; a hand-written comment elsewhere in the
// file does not survive that write, which is the price of one file.
type Config struct {
	SearchEngine string     `yaml:"search_engine,omitempty"`
	DownloadDir  string     `yaml:"download_dir,omitempty"`
	Measure      int        `yaml:"measure,omitempty"` // text width cap in panel [3]; 0 is the default
	Shortcuts    []Shortcut `yaml:"shortcuts,omitempty"`
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

func path(name string) (string, error) {
	dir, err := paths.Config()
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

type bookmarksFile struct {
	Bookmarks []Bookmark `yaml:"bookmarks"`
}

func LoadBookmarks() ([]Bookmark, error) {
	var f bookmarksFile
	err := loadYAML("bookmarks.yaml", &f)
	return f.Bookmarks, err
}

func SaveBookmarks(list []Bookmark) error {
	return saveYAML("bookmarks.yaml", bookmarksFile{Bookmarks: list})
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

// The history file is append-only text, one visit per line — time, URL,
// title, tab-separated — so a crash mid-write costs at most one line, and
// so it can be read with grep. It is kept forever; the History popup's
// Clear is the only way it shrinks (ui.md §6).

func AppendVisit(v Visit) error {
	p, err := path("history")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := v.At.UTC().Format(time.RFC3339) + "\t" + clean(v.URL) + "\t" + clean(v.Title) + "\n"
	_, err = f.WriteString(line)
	return err
}

func clean(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// LoadHistory returns every visit, newest first.
func LoadHistory() ([]Visit, error) {
	p, err := path("history")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Visit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) < 2 {
			continue
		}
		at, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}
		v := Visit{At: at, URL: parts[1]}
		if len(parts) == 3 {
			v.Title = parts[2]
		}
		out = append(out, v)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, sc.Err()
}

// ClearHistory empties the log.
func ClearHistory() error {
	p, err := path("history")
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// DeleteVisit removes every line matching the visit (same time and URL).
func DeleteVisit(v Visit) error {
	all, err := LoadHistory()
	if err != nil {
		return err
	}
	if err := ClearHistory(); err != nil {
		return err
	}
	for i := len(all) - 1; i >= 0; i-- {
		x := all[i]
		if x.URL == v.URL && x.At.Equal(v.At) {
			continue
		}
		if err := AppendVisit(x); err != nil {
			return err
		}
	}
	return nil
}
