// Package paths resolves where webu keeps things. One place, because the
// browser (its profile), the store (bookmarks, history, session) and the
// cache (Chromium itself) all have to agree.
//
// What goes where is what deleting it would cost (tdp E3): settings are
// what the user sets; state only puts things back as they were (the tabs
// open at the last quit); data cannot be had again (bookmarks, history,
// downloads, the browser profile, the log); the cache webu fetches again.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config is where webu keeps what the user sets: config.yaml —
// ~/.config/webu on every platform.
//
// WEBU__CONFIG overrides everything — it names the directory outright, for
// demo recordings and isolated tests. Otherwise XDG_CONFIG_HOME wins when
// set. The rest of the family lets os.UserConfigDir decide, which on macOS
// is ~/Library/Application Support; webu departs from that (2026-09-21):
// its settings are meant to be found, edited and synced by hand, and
// ~/.config is where a terminal user looks for them.
func Config() (string, error) {
	if p := os.Getenv("WEBU__CONFIG"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "webu"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "webu"), nil
}

// State is where webu keeps what only puts things back as they were: the
// session, the tabs open when webu last quit. It is the settings directory
// (tdp E3); WEBU__STATE overrides outright.
func State() (string, error) {
	if p := os.Getenv("WEBU__STATE"); p != "" {
		return p, nil
	}
	return Config()
}

// Data is where webu keeps what cannot be had again: the bookmarks, the
// history, downloads, the Chromium profile and its log — ~/.webu/datas,
// on every platform (revised 2026-09-21: settings under ~/.config/webu,
// data under ~/.webu, so a config dir can be checked in or synced without
// dragging a browser profile along). WEBU__DATA overrides outright.
func Data() (string, error) {
	if p := os.Getenv("WEBU__DATA"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".webu", "datas"), nil
}

// Downloads is where files land unless config.yaml says otherwise.
func Downloads() (string, error) {
	d, err := Data()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "downloads"), nil
}

// Cache is where the Chromium binary goes (function.md §9): a thing that can
// be re-downloaded, so it lives with the caches rather than with the config.
// WEBU__CACHE overrides outright; otherwise the system's cache directory
// (tdp E3) — macOS ~/Library/Caches/webu, Linux $XDG_CACHE_HOME/webu or
// ~/.cache/webu, which is os.UserCacheDir. While it is not there and the
// one an older webu used is (OldCache), that one is: Relocate could not
// move it (tdp E3).
func Cache() (string, error) {
	if p := os.Getenv("WEBU__CACHE"); p != "" {
		return p, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return Either(filepath.Join(dir, "webu"), OldCache()), nil
}

// OldCache is where an older webu kept Chromium and this one does not: on
// macOS, $XDG_CACHE_HOME/webu, read there before tdp E3 named
// ~/Library/Caches. Empty when there is no such place.
func OldCache() string {
	x := os.Getenv("XDG_CACHE_HOME")
	if x == "" || runtime.GOOS != "darwin" || os.Getenv("WEBU__CACHE") != "" {
		return ""
	}
	return filepath.Join(x, "webu")
}

// Either is path, or old while path is not there and old is: a thing an
// older webu kept at old that Relocate could not move is used where it is.
func Either(path, old string) string {
	if old != "" && old != path && !exists(path) && exists(old) {
		return old
	}
	return path
}

// Move is one thing an older webu kept somewhere else.
type Move struct {
	What     string // "bookmarks.yaml", "Chromium"
	From, To string
	// Err is why it could not be moved; it is used from From then.
	Err error
}

// Relocate moves what from to to when to is not there and from is (tdp
// E3: on the first start). It is one rename — a directory goes whole and
// nothing is copied — so a move that fails leaves from as it was. It
// reports whether there was anything to move.
func Relocate(what, from, to string) (Move, bool) {
	if from == "" || from == to || exists(to) || !exists(from) {
		return Move{}, false
	}
	mv := Move{What: what, From: from, To: to}
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		mv.Err = err
		return mv, true
	}
	mv.Err = os.Rename(from, to)
	return mv, true
}

// RelocateCache moves the Chromium an older webu fetched on macOS under
// XDG_CACHE_HOME into the system's cache directory.
func RelocateCache() []Move {
	old := OldCache()
	dir, err := os.UserCacheDir()
	if old == "" || err != nil {
		return nil
	}
	if mv, ok := Relocate("Chromium", old, filepath.Join(dir, "webu")); ok {
		return []Move{mv}
	}
	return nil
}

// Short writes a path the way a person would: the home directory as ~.
func Short(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

// exists reports whether path is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
