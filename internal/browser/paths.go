// Package browser owns the Chromium that webu drives: where its binary lives,
// how it is downloaded, how it is launched with webu's own profile, and how it
// is shut down. Nothing above this package knows there is a process at all —
// the UI sees a context it can run CDP actions against.
package browser

import (
	"os"
	"path/filepath"
)

// ConfigDir is where webu keeps what the user would miss: bookmarks, history,
// session, config.yaml and the Chromium profile (ui.md §6).
//
// WEBU_CONFIG overrides everything — it names the directory outright, for demo
// recordings and isolated tests. Otherwise XDG_CONFIG_HOME wins on every
// platform when set, so a macOS user can opt into ~/.config/webu instead of
// ~/Library/Application Support; without it os.UserConfigDir decides. The same
// rule as kbu, filu and sshu, so the family's files sit side by side.
func ConfigDir() (string, error) {
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

// CacheDir is where the Chromium binary goes (function.md §9): a thing that can
// be re-downloaded, so it lives with the caches rather than with the config.
// WEBU_CACHE overrides outright; XDG_CACHE_HOME next; then os.UserCacheDir.
func CacheDir() (string, error) {
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

// ProfileDir is Chromium's user-data-dir: webu's own, persistent, so a login
// survives a restart. It is never the user's Chrome profile (function.md §9).
func ProfileDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profile"), nil
}
