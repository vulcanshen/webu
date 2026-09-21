// Package paths resolves where webu keeps things. One place, because the
// browser (its profile), the store (bookmarks, history, session) and the
// cache (Chromium itself) all have to agree.
package paths

import (
	"os"
	"path/filepath"
)

// Config is where webu keeps what the user writes: config.yaml and
// bookmarks.yaml (ui.md §6). What webu produces as it runs is under Data.
//
// WEBU_CONFIG overrides everything — it names the directory outright, for demo
// recordings and isolated tests. Otherwise XDG_CONFIG_HOME wins on every
// platform when set, so a macOS user can opt into ~/.config/webu instead of
// ~/Library/Application Support; without it os.UserConfigDir decides. The same
// rule as kbu, filu and sshu, so the family's files sit side by side.
func Config() (string, error) {
	if p := os.Getenv("WEBU_CONFIG"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "webu"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "webu"), nil
}

// Data is where webu keeps what it produces as it runs: the history, the
// session, downloads, the Chromium profile and its log — ~/.webu/datas,
// on every platform (revised 2026-09-21: settings under ~/.config/webu,
// data under ~/.webu, so a config dir can be checked in or synced without
// dragging a browser profile along). WEBU_DATA overrides outright.
func Data() (string, error) {
	if p := os.Getenv("WEBU_DATA"); p != "" {
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
// WEBU_CACHE overrides outright; XDG_CACHE_HOME next; then os.UserCacheDir.
func Cache() (string, error) {
	if p := os.Getenv("WEBU_CACHE"); p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "webu"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "webu"), nil
}
