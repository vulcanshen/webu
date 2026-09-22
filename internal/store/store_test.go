package store

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRoundTrips(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())

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

func TestHistoryIsNewestFirstAndDeletable(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
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
