package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDirsHonourOverrides(t *testing.T) {
	t.Setenv("WEBU__CONFIG", "/tmp/wc")
	t.Setenv("WEBU__CACHE", "/tmp/wk")
	t.Setenv("WEBU__DATA", "/tmp/wd")
	if d, _ := Config(); d != "/tmp/wc" {
		t.Errorf("Config %q", d)
	}
	if d, _ := Data(); d != "/tmp/wd" {
		t.Errorf("Data %q", d)
	}
	if d, _ := Downloads(); d != filepath.Join("/tmp/wd", "downloads") {
		t.Errorf("Downloads %q", d)
	}
	if d, _ := State(); d != "/tmp/wc" {
		t.Errorf("State is the settings directory (tdp E3): %q", d)
	}
	t.Setenv("WEBU__STATE", "/tmp/ws")
	if d, _ := State(); d != "/tmp/ws" {
		t.Errorf("State %q", d)
	}
	t.Setenv("WEBU__STATE", "")
	t.Setenv("WEBU__DATA", "")
	t.Setenv("HOME", "/tmp/h")
	if d, _ := Data(); d != filepath.Join("/tmp/h", ".webu", "datas") {
		t.Errorf("Data under HOME %q", d)
	}
	t.Setenv("WEBU__CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if d, _ := Config(); d != filepath.Join("/tmp/h", ".config", "webu") {
		t.Errorf("Config under HOME %q", d)
	}
	if d, _ := Cache(); d != "/tmp/wk" {
		t.Errorf("Cache %q", d)
	}
	t.Setenv("WEBU__CONFIG", "")
	t.Setenv("WEBU__CACHE", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/x")
	t.Setenv("XDG_CACHE_HOME", "/tmp/y")
	if d, _ := Config(); d != filepath.Join("/tmp/x", "webu") {
		t.Errorf("Config under XDG %q", d)
	}
	if d, _ := Cache(); d != systemCache() {
		t.Errorf("Cache is the system's cache directory (tdp E3): %q", d)
	}
}

// systemCache is where tdp E3 puts the cache: os.UserCacheDir, which is
// ~/Library/Caches on macOS and $XDG_CACHE_HOME or ~/.cache on Linux.
func systemCache() string {
	dir, _ := os.UserCacheDir()
	return filepath.Join(dir, "webu")
}

// The Chromium an older webu fetched goes, from the cache and from the one
// under XDG_CACHE_HOME before tdp E3 on macOS, each directory with it once
// empty; anything else in the cache stays (user, 2026-10-29).
func TestChromiumIsDropped(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("WEBU__CACHE", "")
	t.Setenv("XDG_CACHE_HOME", xdg)
	if dropped := DropChromium(); len(dropped) != 0 {
		t.Errorf("nothing to drop, dropped %+v", dropped)
	}
	cache := systemCache()
	old := filepath.Join(xdg, "webu")
	for _, dir := range []string{filepath.Join(cache, "chromium-1", "bin"), filepath.Join(old, "chromium-2")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cache, "chromium-1", "bin", "chrome"), []byte("12345"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "other"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// On Linux the cache is under XDG_CACHE_HOME: one directory, both in it.
	dropped := DropChromium()
	if len(dropped) != 2 || dropped[0].Path != filepath.Join(cache, "chromium-1") || dropped[0].Size != 5 || dropped[0].Err != nil {
		t.Fatalf("dropped %+v", dropped)
	}
	if exists(filepath.Join(cache, "chromium-1")) || !exists(filepath.Join(cache, "other")) {
		t.Errorf("chromium-1 should go and other stay")
	}
	if runtime.GOOS == "darwin" && exists(old) {
		t.Errorf("the old cache, left empty, should go")
	}
}

// A move that fails leaves the old place as it was and says why.
func TestRelocateThatFails(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "old", "bookmarks.yaml")
	if err := os.MkdirAll(filepath.Dir(from), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(from, []byte("bookmarks: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The new place's directory is a file: nothing can go under it.
	block := filepath.Join(dir, "new")
	if err := os.WriteFile(block, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	mv, ok := Relocate("bookmarks.yaml", from, filepath.Join(block, "bookmarks.yaml"))
	if !ok || mv.Err == nil {
		t.Fatalf("Relocate %+v %v", mv, ok)
	}
	if !exists(from) {
		t.Error("the old file is gone")
	}
	if p := Either(filepath.Join(block, "bookmarks.yaml"), from); p != from {
		t.Errorf("used from %q", p)
	}
}

// Short writes the home directory as ~.
func TestShort(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	for in, want := range map[string]string{
		"/tmp/h":              "~",
		"/tmp/h/.webu/datas":  "~/.webu/datas",
		"/tmp/hh/.webu/datas": "/tmp/hh/.webu/datas",
		"/var/x":              "/var/x",
	} {
		if got := Short(in); got != want {
			t.Errorf("Short(%q) = %q, want %q", in, got, want)
		}
	}
}

// A Chromium fetched under XDG_CACHE_HOME on macOS, before tdp E3, is used
// while the system's cache directory has none: the move did not happen.
func TestAnOldMacCacheIsStillRead(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	home, xdg := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("WEBU__CACHE", "")
	t.Setenv("XDG_CACHE_HOME", xdg)
	if d, _ := Cache(); d != filepath.Join(home, "Library", "Caches", "webu") {
		t.Errorf("nothing anywhere: the system's cache directory, got %q", d)
	}
	if err := os.MkdirAll(filepath.Join(xdg, "webu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if d, _ := Cache(); d != filepath.Join(xdg, "webu") {
		t.Errorf("the old cache is read while the new one is not there: %q", d)
	}
	if err := os.MkdirAll(filepath.Join(home, "Library", "Caches", "webu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if d, _ := Cache(); d != filepath.Join(home, "Library", "Caches", "webu") {
		t.Errorf("once the new one is there it wins: %q", d)
	}
}

// The family names its variables APP__NAME and keeps no old name (tdp D6
// v0.1.21): WEBU_CONFIG, WEBU_DATA and WEBU_CACHE are not read.
func TestOldNamesAreNotRead(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/x")
	t.Setenv("XDG_CACHE_HOME", "/tmp/y")
	for _, v := range []string{"WEBU__CONFIG", "WEBU__DATA", "WEBU__CACHE"} {
		t.Setenv(v, "")
	}
	t.Setenv("WEBU_CONFIG", "/tmp/old")
	t.Setenv("WEBU_DATA", "/tmp/old")
	t.Setenv("WEBU_CACHE", "/tmp/old")
	if d, _ := Config(); d != filepath.Join("/tmp/x", "webu") {
		t.Errorf("Config %q", d)
	}
	if d, _ := Data(); d != filepath.Join("/tmp/h", ".webu", "datas") {
		t.Errorf("Data %q", d)
	}
	if d, _ := Cache(); d != systemCache() {
		t.Errorf("Cache %q", d)
	}
}
