package store

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRoundTrips(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())

	if b, _, err := LoadBookmarks(); err != nil || len(b) != 0 {
		t.Fatalf("empty state: %v %v", b, err)
	}
	want := []Bookmark{{Title: "HN", URL: "https://news.ycombinator.com/"}, {Title: "Go", URL: "https://go.dev", Folder: "dev"}}
	if err := SaveBookmarks(want, []string{"dev", "later"}); err != nil {
		t.Fatal(err)
	}
	got, folders, err := LoadBookmarks()
	if err != nil || len(got) != 2 || got[1].Folder != "dev" || len(folders) != 2 || folders[1] != "later" {
		t.Fatalf("bookmarks: %+v %v %v", got, folders, err)
	}

	cfg, _ := LoadConfig()
	if cfg.Search() != DefaultSearch {
		t.Errorf("default search %q", cfg.Search())
	}
	if !cfg.Restore() {
		t.Error("restore_session should default to on")
	}
	cfg.SearchEngine = "https://www.google.com/search?q="
	off := false
	cfg.RestoreSession = &off
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	cfg2, _ := LoadConfig()
	if cfg2.Search() != cfg.SearchEngine || cfg2.Restore() {
		t.Errorf("config: %+v", cfg2)
	}

	s := Session{Tabs: []SessionTab{{URL: "https://a", Title: "A"}, {URL: "https://b"}}, Shown: 1}
	if err := SaveSession(s); err != nil {
		t.Fatal(err)
	}
	if s2, _ := LoadSession(); len(s2.Tabs) != 2 || s2.Shown != 1 || s2.Tabs[0].Title != "A" {
		t.Errorf("session: %+v", s2)
	}
}

// Each file goes where tdp E3 puts it: the session (state) with the
// settings, the bookmarks and the history (data) under the data dir.
func TestFilesGoWhereE3PutsThem(t *testing.T) {
	cfg, data := t.TempDir(), t.TempDir()
	t.Setenv("WEBU__CONFIG", cfg)
	t.Setenv("WEBU__DATA", data)
	t.Setenv("WEBU__STATE", "")
	if err := SaveBookmarks([]Bookmark{{Title: "Go", URL: "https://go.dev"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := SaveSession(Session{Tabs: []SessionTab{{URL: "https://a"}}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(Config{SearchEngine: "x"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(data, "bookmarks.yaml"), filepath.Join(cfg, "session.yaml"), filepath.Join(cfg, "config.yaml")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	state := t.TempDir()
	t.Setenv("WEBU__STATE", state)
	if err := SaveSession(Session{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "session.yaml")); err != nil {
		t.Errorf("WEBU__STATE: %v", err)
	}
}

// bookmarks.yaml and session.yaml where an older webu kept them move on the
// first start, one by one (tdp E3), and an old directory left empty goes;
// one already in its new place is not overwritten, the old copy stays and
// the Move says both are there.
func TestRelocateMovesTheOldFiles(t *testing.T) {
	cfg, data := t.TempDir(), t.TempDir()
	t.Setenv("WEBU__CONFIG", cfg)
	t.Setenv("WEBU__DATA", data)
	t.Setenv("WEBU__STATE", "")
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(cfg, "bookmarks.yaml"), "bookmarks:\n  - title: Go\n    url: https://go.dev\n")
	write(filepath.Join(data, "session.yaml"), "tabs:\n  - url: https://a\nshown: 0\n")
	// Before the move each is read where it is.
	if b, _, err := LoadBookmarks(); err != nil || len(b) != 1 {
		t.Fatalf("old bookmarks.yaml not read: %+v %v", b, err)
	}
	moved := Relocate()
	if len(moved) != 2 {
		t.Fatalf("moved %+v", moved)
	}
	for _, mv := range moved {
		if mv.Err != nil {
			t.Errorf("%s: %v", mv.What, mv.Err)
		}
	}
	if b, _, _ := LoadBookmarks(); len(b) != 1 {
		t.Errorf("bookmarks after the move: %+v", b)
	}
	if s, _ := LoadSession(); len(s.Tabs) != 1 {
		t.Errorf("session after the move: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(data, "bookmarks.yaml")); err != nil {
		t.Error("bookmarks.yaml is not under the data dir")
	}
	if _, err := os.Stat(filepath.Join(cfg, "session.yaml")); err != nil {
		t.Error("session.yaml is not under the config dir")
	}
	if moved := Relocate(); len(moved) != 0 {
		t.Errorf("moved twice: %+v", moved)
	}
	write(filepath.Join(cfg, "bookmarks.yaml"), "bookmarks: []\n")
	moved = Relocate()
	if len(moved) != 1 || !moved[0].Both || moved[0].Err != nil {
		t.Errorf("a file in both places says so: %+v", moved)
	}
	if b, _, _ := LoadBookmarks(); len(b) != 1 {
		t.Errorf("the one in the new place is used, not overwritten: %+v", b)
	}
	if _, err := os.Stat(filepath.Join(cfg, "bookmarks.yaml")); err != nil {
		t.Error("the old copy should stay for the user to merge")
	}

	// A directory a move leaves empty goes.
	old := filepath.Join(t.TempDir(), "old")
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WEBU__CONFIG", old)
	write(filepath.Join(old, "bookmarks.yaml"), "bookmarks: []\n")
	if err := os.Remove(filepath.Join(data, "bookmarks.yaml")); err != nil {
		t.Fatal(err)
	}
	Relocate()
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the emptied old directory is still there: %v", err)
	}
}

func TestHistoryIsNewestFirstAndDeletable(t *testing.T) {
	t.Setenv("WEBU__CONFIG", t.TempDir())
	t.Setenv("WEBU__DATA", t.TempDir())
	t0 := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := AppendVisit(Visit{At: t0.Add(time.Duration(i) * time.Minute), URL: "https://x/" + string(rune('a'+i)), Title: "T\tab"}); err != nil {
			t.Fatal(err)
		}
	}
	h, err := LoadHistory()
	if err != nil || len(h) != 3 {
		t.Fatalf("history: %v %v", h, err)
	}
	if h[0].URL != "https://x/c" || h[0].Title != "T ab" {
		t.Errorf("newest first, tabs cleaned: %+v", h[0])
	}
	if err := DeleteVisit(h[1]); err != nil {
		t.Fatal(err)
	}
	h, _ = LoadHistory()
	if len(h) != 2 || h[0].URL != "https://x/c" || h[1].URL != "https://x/a" {
		t.Errorf("after delete: %+v", h)
	}
	if err := ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if h, _ = LoadHistory(); len(h) != 0 {
		t.Errorf("after clear: %+v", h)
	}
}

// An old config.yaml that still says measure (removed 2026-10-29) reads as
// ever, the key told as retired and written back with the file, not
// dropped by a save; a key webu never knew is kept too (tdp E3).
func TestARetiredKeyIsKept(t *testing.T) {
	var c Config
	if err := yaml.Unmarshal([]byte("measure: 96\nsearch_engine: https://example.com/?q=\nmine: kept\n"), &c); err != nil || c.SearchEngine != "https://example.com/?q=" {
		t.Fatalf("read back as %+v (%v)", c, err)
	}
	if got := c.Retired(); len(got) != 1 || got[0] != "measure" {
		t.Errorf("retired keys: %v", got)
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "measure: 96") || !strings.Contains(string(out), "mine: kept") {
		t.Errorf("a save should keep what webu does not read:\n%s", out)
	}
}

// A file that cannot be read gives nothing of itself, not the part read
// before the error (tdp E3-18): moved aside, webu starts as on its first
// start, and until then nothing half-read has been acted on — a
// restore_session: false above a bad line must not drop the last session.
func TestAnUnreadableFileGivesNothing(t *testing.T) {
	cfgDir, data := t.TempDir(), t.TempDir()
	t.Setenv("WEBU__CONFIG", cfgDir)
	t.Setenv("WEBU__DATA", data)
	t.Setenv("WEBU__STATE", "")
	write := func(dir, name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(cfgDir, "config.yaml", "restore_session: false\ndownload_dir: /x\ninput_tab_width: [4\n")
	if c, err := LoadConfig(); err == nil || c.RestoreSession != nil || c.DownloadDir != "" {
		t.Errorf("config.yaml: %+v, %v", c, err)
	}
	write(data, "bookmarks.yaml", "bookmarks:\n  - title: A\n    url: https://a\nfolders: wide\n")
	if b, f, err := LoadBookmarks(); err == nil || len(b) != 0 || len(f) != 0 {
		t.Errorf("bookmarks.yaml: %+v %v, %v", b, f, err)
	}
	write(cfgDir, "session.yaml", "tabs:\n  - url: https://a\nshown: wide\n")
	if s, err := LoadSession(); err == nil || len(s.Tabs) != 0 {
		t.Errorf("session.yaml: %+v, %v", s, err)
	}
}

// A value or an entry webu cannot use is not a file that cannot be read
// (tdp E3, a value that does not fit): the rest of the file is read, the
// default used in its place or the entry taken out, and each one named.
// A key webu never read is kept, as one it no longer reads is.
func TestValuesThatDoNotFit(t *testing.T) {
	cfgDir, data := t.TempDir(), t.TempDir()
	t.Setenv("WEBU__CONFIG", cfgDir)
	t.Setenv("WEBU__DATA", data)
	t.Setenv("WEBU__STATE", "")
	write := func(dir, name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(cfgDir, "config.yaml", "restore_session: maybe\ndownload_dir: /x\ninput_tab_width: wide\nsearch_engine: duckduckgo\nrefesh: 3\nmeasure: 80\nhide_secrets_in_preview: true\n")
	c, bad, err := LoadConfigChecked()
	if err != nil || c.DownloadDir != "/x" || !c.HideSecretsInPreview || !c.Restore() || c.TabWidth() != 4 || c.Search() != DefaultSearch {
		t.Errorf("config.yaml: the rest read, the defaults in place of the three: %+v, %v", c, err)
	}
	want := []Misfit{{"restore_session", "maybe", "true"}, {"input_tab_width", "wide", "4"}, {"search_engine", "duckduckgo", DefaultSearch}}
	if !slices.Equal(bad, want) {
		t.Errorf("config.yaml's misfits: %+v, want %+v", bad, want)
	}
	if u, r := c.Unknown(), c.Retired(); !slices.Equal(u, []string{"refesh"}) || !slices.Equal(r, []string{"measure"}) {
		t.Errorf("never read %v, no longer read %v", u, r)
	}
	// Written again, the keys webu does not read stay; the misfits do not.
	if err := SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(cfgDir, "config.yaml"))
	if s := string(raw); !strings.Contains(s, "refesh: 3") || !strings.Contains(s, "measure: 80") || strings.Contains(s, "maybe") || strings.Contains(s, "wide") {
		t.Errorf("config.yaml written again:\n%s", s)
	}
	if _, bad, err := LoadConfigChecked(); err != nil || len(bad) != 0 {
		t.Errorf("written again, nothing more to say: %+v, %v", bad, err)
	}

	write(data, "bookmarks.yaml", "bookmarks:\n  - title: A\n    url: https://a\n    folder: \" dev / /go \"\n  - title: no url\n  - title: B\n    url: https://b\n    folder: /\nfolders: [dev, \"\", dev, \"a//b\"]\n")
	list, folders, bad, err := LoadBookmarksChecked()
	if err != nil || len(list) != 2 || list[0].Folder != "dev/go" || list[1].Folder != "" || !slices.Equal(folders, []string{"dev", "a/b"}) {
		t.Errorf("bookmarks.yaml: %+v %v, %v", list, folders, err)
	}
	want = []Misfit{{"bookmark 1 folder", " dev / /go ", "dev/go"}, {Key: "bookmark 2 (url \"\")"},
		{Key: "folder 2 (\"\")"}, {Key: "folder 3 (\"dev\")"}, {"folder 4", "a//b", "a/b"}}
	if !slices.Equal(bad, want) {
		t.Errorf("bookmarks.yaml's misfits: %+v, want %+v", bad, want)
	}

	write(data, "history.yaml", "- at: 2026-11-01T10:00:00Z\n  url: https://a\n- at: 2026-11-02T10:00:00Z\n  url: \"\"\n")
	visits, bad, err := LoadHistoryChecked()
	if err != nil || len(visits) != 1 || visits[0].URL != "https://a" || !slices.Equal(bad, []Misfit{{Key: "visit 2 (url \"\")"}}) {
		t.Errorf("history.yaml: %+v, %+v, %v", visits, bad, err)
	}
	if err := SaveHistory(visits); err != nil {
		t.Fatal(err)
	}
	if v, bad, err := LoadHistoryChecked(); err != nil || len(v) != 1 || len(bad) != 0 {
		t.Errorf("history.yaml written again: %+v, %+v, %v", v, bad, err)
	}

	write(cfgDir, "session.yaml", "tabs:\n  - url: https://a\n  - url: \"\"\nshown: 5\n")
	s, bad, err := LoadSessionChecked()
	if err != nil || len(s.Tabs) != 1 || s.Shown != 0 || !slices.Equal(bad, []Misfit{{Key: "tab 2 (url \"\")"}, {"shown", "5", "0"}}) {
		t.Errorf("session.yaml: %+v, %+v, %v", s, bad, err)
	}
}
