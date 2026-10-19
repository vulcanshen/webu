package store

import (
	"os"
	"path/filepath"
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
// first start (tdp E3); one already in its new place is left alone, and
// the old copy stays.
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
	if moved := Relocate(); len(moved) != 0 {
		t.Errorf("a file already in its new place is not overwritten: %+v", moved)
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

// TestMeasure: the setting reads and writes both spellings — full for
// the panel's own width, a number for a cap — and refuses the rest
// (2026-09-22).
func TestMeasure(t *testing.T) {
	for _, in := range []string{"full", "FULL", " full ", ""} {
		if m, err := ParseMeasure(in); err != nil || m != MeasureFull {
			t.Errorf("%q should be full: %v %v", in, m, err)
		}
	}
	if m, err := ParseMeasure("96"); err != nil || m != 96 {
		t.Errorf("a number of cells: %v %v", m, err)
	}
	for _, in := range []string{"wide", "19", "-4", "96px"} {
		if _, err := ParseMeasure(in); err == nil {
			t.Errorf("%q should be refused", in)
		}
	}
	if MeasureFull.String() != "full" || Measure(96).String() != "96" {
		t.Errorf("spelling: %q %q", MeasureFull, Measure(96))
	}
	// Both spellings survive a trip through config.yaml.
	for _, doc := range []struct {
		yaml string
		want Measure
	}{{"measure: full\n", MeasureFull}, {"measure: 96\n", 96}} {
		var c Config
		if err := yaml.Unmarshal([]byte(doc.yaml), &c); err != nil || c.Measure != doc.want {
			t.Fatalf("%q read back as %v (%v)", doc.yaml, c.Measure, err)
		}
		out, err := yaml.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		var back Config
		if err := yaml.Unmarshal(out, &back); err != nil || back.Measure != doc.want {
			t.Errorf("round trip of %q gave %q -> %v (%v)", doc.yaml, out, back.Measure, err)
		}
	}
	// A cap only caps: full is zero, which the renderer reads as no cap.
	if (Config{}).TextWidth() != 0 || (Config{Measure: 96}).TextWidth() != 96 {
		t.Error("TextWidth should be 0 for full and the number otherwise")
	}
}
