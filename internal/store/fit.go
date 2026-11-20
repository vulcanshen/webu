package store

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vulcanshen/webu/internal/paths"
	"gopkg.in/yaml.v3"
)

// Misfit is a value or an entry in one of webu's files that can be read
// but that webu cannot use (tdp E3, a value that does not fit): Key names
// it, Was is what the file says, Now what is used instead, "" when the
// entry is taken out. The start copies the file to .bak and writes it
// again without them.
type Misfit struct{ Key, Was, Now string }

// LoadConfigChecked is LoadConfig with the values webu cannot use set
// aside: each key is read on its own, so one wrong value (a word where a
// number goes, a search engine that is not a URL) costs only itself, its
// default used in its place. A file that is not YAML, or not keys and
// values, cannot be read.
func LoadConfigChecked() (Config, []Misfit, error) {
	var doc yaml.Node
	if err := loadYAML("config.yaml", &doc); err != nil {
		return Config{}, nil, err
	}
	var c Config
	if len(doc.Content) == 0 || doc.Content[0].Tag == "!!null" {
		return c, nil, nil // missing, empty or only comments
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return Config{}, nil, fmt.Errorf("line %d: not keys and values", top.Line)
	}
	var bad []Misfit
	misfit := func(k string, n *yaml.Node, now string) {
		bad = append(bad, Misfit{Key: k, Was: nodeText(n), Now: now})
	}
	for i := 0; i+1 < len(top.Content); i += 2 {
		k, n := top.Content[i].Value, top.Content[i+1]
		switch k {
		case "search_engine":
			// The words are appended to it: a URL, as Settings has it.
			var s string
			if n.Decode(&s) != nil || (s != "" && !strings.Contains(s, "://")) {
				misfit(k, n, DefaultSearch)
				continue
			}
			c.SearchEngine = s
		case "download_dir":
			var s string
			if n.Decode(&s) != nil {
				misfit(k, n, defaultDownloads())
				continue
			}
			c.DownloadDir = s
		case "restore_session":
			var b bool
			if n.Decode(&b) != nil {
				misfit(k, n, "true")
				continue
			}
			c.RestoreSession = &b
		case "hide_secrets_in_preview":
			var b bool
			if n.Decode(&b) != nil {
				misfit(k, n, "false")
				continue
			}
			c.HideSecretsInPreview = b
		case "input_tab_width":
			// 1–16 spaces, as Settings takes it; 0 is not set.
			var w int
			if n.Decode(&w) != nil || w < 0 || w > 16 {
				misfit(k, n, "4")
				continue
			}
			c.InputTabWidth = w
		default:
			// A key webu does not read is kept as it is (tdp E3).
			var v any
			if err := n.Decode(&v); err != nil {
				return Config{}, nil, err
			}
			if c.Rest == nil {
				c.Rest = map[string]any{}
			}
			c.Rest[k] = v
		}
	}
	return c, bad, nil
}

// defaultDownloads is where downloads land when download_dir is not set.
func defaultDownloads() string {
	if d, err := paths.Downloads(); err == nil {
		return paths.Short(d)
	}
	return "~/Downloads"
}

// nodeText is what the file says for a value, for the App Log.
func nodeText(n *yaml.Node) string {
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	raw, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(raw), "\n", " "))
}

// LoadBookmarksChecked is LoadBookmarks with the entries webu cannot use
// set aside: a bookmark with no URL is taken out; a folder is written as
// its levels, blanks and empty levels dropped ("a//b " is a/b), and one
// that comes to nothing, or is listed twice, is taken out. A bookmark's
// folder that comes to nothing ("/") is the top level.
func LoadBookmarksChecked() ([]Bookmark, []string, []Misfit, error) {
	var f bookmarksFile
	if err := loadYAML("bookmarks.yaml", &f); err != nil {
		return nil, nil, nil, err
	}
	var bad []Misfit
	var list []Bookmark
	for i, b := range f.Bookmarks {
		key := "bookmark " + strconv.Itoa(i+1)
		if strings.TrimSpace(b.URL) == "" {
			bad = append(bad, Misfit{Key: key + " (url " + strconv.Quote(b.URL) + ")"})
			continue
		}
		if clean := CleanFolder(b.Folder); clean != b.Folder {
			// Nothing left ("/") is the top level, as the location row
			// writes it: said another way, not a misfit.
			if clean != "" {
				bad = append(bad, Misfit{Key: key + " folder", Was: b.Folder, Now: clean})
			}
			b.Folder = clean
		}
		list = append(list, b)
	}
	var folders []string
	seen := map[string]bool{}
	for i, fo := range f.Folders {
		key := "folder " + strconv.Itoa(i+1)
		clean := CleanFolder(fo)
		if clean == "" || seen[clean] {
			bad = append(bad, Misfit{Key: key + " (" + strconv.Quote(fo) + ")"})
			continue
		}
		if clean != fo {
			bad = append(bad, Misfit{Key: key, Was: fo, Now: clean})
		}
		seen[clean] = true
		folders = append(folders, clean)
	}
	return list, folders, bad, nil
}

// CleanFolder is a folder path as its levels: blanks and empty levels
// dropped, so " a / /b " is a/b.
func CleanFolder(path string) string {
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "/")
}

// LoadHistoryChecked is LoadHistory with a visit that has no URL taken
// out.
func LoadHistoryChecked() ([]Visit, []Misfit, error) {
	all, err := loadHistory()
	if err != nil {
		return nil, nil, err
	}
	var bad []Misfit
	var out []Visit
	for i, v := range all {
		if strings.TrimSpace(v.URL) == "" {
			// Counted from the top of the file, oldest first, as it is
			// written.
			bad = append(bad, Misfit{Key: "visit " + strconv.Itoa(len(all)-i) + " (url \"\")"})
			continue
		}
		out = append(out, v)
	}
	return out, bad, nil
}

// LoadSessionChecked is LoadSession with a tab that has no URL taken out,
// and a shown tab that is not one of them the first.
func LoadSessionChecked() (Session, []Misfit, error) {
	var s Session
	if err := loadYAML("session.yaml", &s); err != nil {
		return Session{}, nil, err
	}
	var bad []Misfit
	var tabs []SessionTab
	for i, t := range s.Tabs {
		if strings.TrimSpace(t.URL) == "" {
			bad = append(bad, Misfit{Key: "tab " + strconv.Itoa(i+1) + " (url \"\")"})
			continue
		}
		tabs = append(tabs, t)
	}
	s.Tabs = tabs
	if s.Shown < 0 || (s.Shown > 0 && s.Shown >= len(s.Tabs)) {
		bad = append(bad, Misfit{Key: "shown", Was: strconv.Itoa(s.Shown), Now: "0"})
		s.Shown = 0
	}
	return s, bad, nil
}

// SaveHistory writes the visits, newest first as LoadHistory gives them,
// in place of the file.
func SaveHistory(visits []Visit) error {
	if len(visits) == 0 {
		return ClearHistory()
	}
	recs := make([]visitRec, 0, len(visits))
	for i := len(visits) - 1; i >= 0; i-- {
		v := visits[i]
		recs = append(recs, visitRec{At: v.At.UTC(), URL: v.URL, Title: v.Title})
	}
	return saveYAML("history.yaml", recs)
}
