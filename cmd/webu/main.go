// Command webu is a terminal browser that reads: a page is fetched over
// HTTP, what it presents is picked out of the site around it, and webu
// draws that as a TUI (docs/reader.md).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/paths"
	"github.com/vulcanshen/webu/internal/reader"
	"github.com/vulcanshen/webu/internal/store"
	"github.com/vulcanshen/webu/internal/ui"
	"github.com/vulcanshen/webu/internal/version"
)

// usage is `webu help`: the whole command line (tdp E1).
const usage = `webu — a terminal browser

  webu                     the last session's tabs
  webu <url|words> ...     open each in a new tab, the first in front;
                           words that are not a URL are searched
  webu version             webu's version
  webu update              bring webu to its latest release
  webu help                this
`

// command runs what the command line asks for that is not the TUI (tdp
// E1): results on stdout, errors on stderr after "webu: ", a wrong use
// with the usage and status 2. It reports whether it handled args, and
// the exit status.
func command(args []string, stdout, stderr io.Writer) (status int, handled bool) {
	// version, update and help answer before anything else, so they work
	// with no config.
	if len(args) == 1 {
		switch args[0] {
		case "version", "-v", "--version":
			// The version alone, as its tag reads (harness project.4).
			fmt.Fprintln(stdout, version.Display())
			return 0, true
		case "help", "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0, true
		case "update":
			return terminu.SelfUpdate{App: "webu", Version: version.Version, Owner: "vulcanshen", Tap: "vulcanshen/tap",
				Source: "git pull && make install"}.Run(stdout, stderr), true
		}
	}
	// Anything else is what to open: URLs, or words to search for, a new
	// tab each — the command line is the Location box. An option webu has
	// not got is a wrong use.
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "webu: Unknown option %q\n\n%s", a, usage)
			return 2, true
		}
	}
	return 0, false
}

func main() {
	args := os.Args[1:]
	if status, handled := command(args, os.Stdout, os.Stderr); handled {
		os.Exit(status)
	}
	startURLs := args

	// What an older webu kept somewhere else moves to where tdp E3 puts it,
	// before anything reads it there; one that cannot be moved is used where
	// it is. The Chromium an older webu fetched goes: webu no longer runs
	// one (user, 2026-10-29), and the App Log says what went.
	moves := store.Relocate()
	dropped := paths.DropChromium()

	// webu's own files. One that will not read is asked about on the first
	// frame (tdp E3: MOVE ASIDE, or leave), not gone on with — a later save
	// would write over it — and not a line on a terminal the screen is
	// about to cover.
	bookmarks, folders, bmBad, bmErr := store.LoadBookmarksChecked()
	cfg, cfgBad, cfgErr := store.LoadConfigChecked()
	history, histBad, histErr := store.LoadHistoryChecked()
	session, sessBad, sessErr := store.LoadSessionChecked()
	// One that reads but holds values or entries webu cannot use is copied
	// to .bak and written again without them, and the first frame says so
	// (tdp E3, a value that does not fit). When the copy cannot be made the
	// file is left as it was, its defaults for this run only.
	var misfits []ui.Misfits
	for _, f := range []struct {
		name  string
		bad   []store.Misfit
		write func() error
	}{
		{"config.yaml", cfgBad, func() error { return store.SaveConfig(cfg) }},
		{"bookmarks.yaml", bmBad, func() error { return store.SaveBookmarks(bookmarks, folders) }},
		{"history.yaml", histBad, func() error { return store.SaveHistory(history) }},
		{"session.yaml", sessBad, func() error { return store.SaveSession(session) }},
	} {
		if len(f.bad) == 0 {
			continue
		}
		mf := ui.Misfits{Path: store.Path(f.name), Bad: f.bad}
		if mf.Copy, mf.Err = terminu.KeepAside(mf.Path); mf.Err == nil {
			if err := f.write(); err != nil {
				mf.Err = fmt.Errorf("%s could not be written again: %w; yours kept in %s", f.name, err, filepath.Base(mf.Copy))
				mf.Copy = ""
			}
		}
		misfits = append(misfits, mf)
	}
	// The reader's cookies (docs/reader.md). One that will not read is
	// asked about like the rest; meanwhile the jar is empty, and nothing
	// is written over the file before the answer.
	jar, jarErr := reader.OpenJar(store.Path("cookies.json"))
	if jarErr != nil {
		jar = reader.EmptyJar(store.Path("cookies.json"))
	}
	var unread []ui.Unread
	for _, e := range []struct {
		name string
		err  error
	}{{"config.yaml", cfgErr}, {"bookmarks.yaml", bmErr}, {"history.yaml", histErr}, {"session.yaml", sessErr},
		{"cookies.json", jarErr}} {
		if e.err != nil {
			unread = append(unread, ui.Unread{Name: e.name, Path: store.Path(e.name), Err: e.err})
		}
	}
	// restore_session: false starts empty, or on the command line's URLs.
	// The session is still written on the way out, so switching it back
	// on brings the latest one back.
	if !cfg.Restore() {
		session = store.Session{}
	}
	app := ui.New(startURLs...).WithStore(bookmarks, folders, cfg, history).WithSession(session).
		WithUnread(unread).WithMisfits(misfits).WithMoves(moves).WithDropped(dropped).WithReader(reader.NewClient(jar, ""))

	// The lib asks the terminal how wide an icon is before it takes the
	// screen (tdp E5); version and help never ask. Whatever door the
	// program leaves through — q, an outside SIGINT or SIGTERM, the
	// terminal closing (the lib turns SIGHUP into leaving) — Run returns
	// and the session is written.
	runErr := app.Run()
	// An outside SIGINT is leaving too, as a hangup is: the exit status is
	// 0, 1 or 2 (tdp E1).
	if errors.Is(runErr, tea.ErrInterrupted) {
		runErr = nil
	}
	status := 0
	// The session is written on every way out, so the tabs come back next
	// time — but not over a session.yaml that could not be read and was
	// not moved aside.
	if app.SessionWritable() {
		if err := store.SaveSession(app.Session()); err != nil {
			fmt.Fprintln(os.Stderr, "webu: session.yaml:", err)
			status = 1
		}
	}
	app.Close()
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "webu:", runErr)
		status = 1
	}
	os.Exit(status)
}
