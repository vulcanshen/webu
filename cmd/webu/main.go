// Command webu is a terminal browser that reads: a page is fetched over
// HTTP, what it presents is picked out of the site around it, and webu
// draws that as a TUI (docs/reader.md).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/paths"
	"github.com/vulcanshen/webu/internal/reader"
	"github.com/vulcanshen/webu/internal/store"
	"github.com/vulcanshen/webu/internal/ui"
	"github.com/vulcanshen/webu/internal/version"
)

// usage is `webu help`: the whole command line (function.md §12).
const usage = `webu — a terminal browser

  webu                     the last session's tabs
  webu <url|words> ...     open each in a new tab, the first in front;
                           words that are not a URL are searched
  webu version             webu's version
  webu help                this
`

func main() {
	args := os.Args[1:]
	// `webu version` and `webu help` answer before anything else, so they
	// work with no config.
	if len(args) == 1 && args[0] == "version" {
		fmt.Printf("webu %s\n", version.Display())
		return
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usage)
		return
	}
	// Anything else is what to open: URLs, or words to search for, a new
	// tab each — the command line is the Location box (function.md §12).
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "webu: unknown option %s\n\n%s", a, usage)
			os.Exit(2)
		}
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
	bookmarks, folders, bmErr := store.LoadBookmarks()
	cfg, cfgErr := store.LoadConfig()
	history, histErr := store.LoadHistory()
	session, sessErr := store.LoadSession()
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
		WithUnread(unread).WithMoves(moves).WithDropped(dropped).WithReader(reader.NewClient(jar, ""))

	// Whatever door the program leaves through — q, an outside SIGINT or
	// SIGTERM, the terminal closing — the session is written. Bubble Tea
	// quits on SIGINT and SIGTERM itself; the terminal closing is SIGHUP,
	// which it leaves alone, so it is turned into the SIGTERM it does know.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)
	go func() {
		<-sig
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	// The lib asks the terminal how wide an icon is before it takes the
	// screen (tdp E5); version and help never ask.
	runErr := app.Run()
	// The session is written on every way out, q or a signal, so the tabs
	// come back next time (ux.md §6) — but not over a session.yaml that
	// could not be read and was not moved aside.
	if app.SessionWritable() {
		if err := store.SaveSession(app.Session()); err != nil {
			fmt.Fprintln(os.Stderr, "webu: session.yaml:", err)
		}
	}
	app.Close()
	switch {
	case errors.Is(runErr, tea.ErrInterrupted):
		os.Exit(130)
	case runErr != nil:
		fmt.Fprintln(os.Stderr, "webu:", runErr)
		os.Exit(1)
	}
}
