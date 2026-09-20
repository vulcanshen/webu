// Package browser owns the Chromium that webu drives: where its binary lives,
// how it is downloaded, how it is launched with webu's own profile, and how it
// is shut down. Nothing above this package knows there is a process at all —
// the UI sees a context it can run CDP actions against.
package browser

import (
	"path/filepath"

	"github.com/vulcanshen/webu/internal/paths"
)

// ProfileDir is Chromium's user-data-dir: webu's own, persistent, so a login
// survives a restart. It is never the user's Chrome profile (function.md §9).
func ProfileDir() (string, error) {
	dir, err := paths.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profile"), nil
}
