// Command webu is a terminal browser: Chromium runs headless in the
// background, webu draws its accessibility tree as a TUI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/paths"
	"github.com/vulcanshen/webu/internal/store"
	"github.com/vulcanshen/webu/internal/ui"
	"github.com/vulcanshen/webu/internal/version"
)

// usage is `webu help`: the whole command line (function.md §12).
const usage = `webu — a terminal browser

  webu                     the last session's tabs
  webu <url|words> ...     open each in a new tab, the first in front;
                           words that are not a URL are searched
  webu version             webu's version and the Chromium revision it runs
  webu browser update      fetch the pinned Chromium again
  webu help                this
`

func main() {
	args := os.Args[1:]
	// `webu version` and `webu help` answer before anything else, so they
	// work with no Chromium and no config.
	if len(args) == 1 && args[0] == "version" {
		fmt.Printf("webu %s (chromium r%d)\n", version.Display(), browser.Revision)
		return
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usage)
		return
	}
	// What an older webu kept somewhere else moves to where tdp E3 puts it,
	// before anything reads it there; one that cannot be moved is used where
	// it is.
	moves := append(store.Relocate(), paths.RelocateCache()...)
	// `webu browser update` is the one door to a new revision (function.md
	// §9): a rebuilt binary carries a new pin, and this fetches it.
	if len(args) == 2 && args[0] == "browser" && args[1] == "update" {
		for _, mv := range moves {
			if mv.Err != nil {
				fmt.Fprintf(os.Stderr, "webu: %s could not be moved from %s to %s: %v; it is used where it is.\n", mv.What, mv.From, mv.To, mv.Err)
			} else {
				fmt.Fprintf(os.Stderr, "webu: moved %s from %s to %s\n", mv.What, mv.From, mv.To)
			}
		}
		if _, err := ensureChromium(true); err != nil {
			fmt.Fprintln(os.Stderr, "webu:", err)
			os.Exit(1)
		}
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

	exe, err := ensureChromium(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "webu:", err)
		os.Exit(1)
	}
	profile, err := browser.ProfileDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "webu:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(profile, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "webu:", err)
		os.Exit(1)
	}
	// chromedp's own log goes to a file next to the profile, never to the
	// terminal the TUI is drawing on (webu's own goes to the App Log, `!`).
	// Failing to open it is not fatal: the lines are dropped instead.
	var logw io.Writer = io.Discard
	if f, err := os.OpenFile(filepath.Join(filepath.Dir(profile), "webu.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		defer f.Close()
		logw = f
	}
	b, err := browser.Launch(exe, profile, logw)
	if err != nil {
		// A Chromium that is whole but will not start (a system library
		// missing, the sandbox refused) is not a cache that cannot be read:
		// something outside webu, said with the way to fetch it again
		// should it be the download after all (tdp E3). One cut short or
		// gone is not Installed, and was fetched again above.
		fmt.Fprintln(os.Stderr, "webu:", err)
		fmt.Fprintln(os.Stderr, "webu: if the downloaded Chromium is broken, `webu browser update` fetches it again")
		os.Exit(1)
	}

	// webu's own files. One that will not read is asked about on the first
	// frame (tdp E3: MOVE ASIDE, or leave), not gone on with — a later save
	// would write over it — and not a line on a terminal the screen is
	// about to cover.
	bookmarks, folders, bmErr := store.LoadBookmarks()
	cfg, cfgErr := store.LoadConfig()
	history, histErr := store.LoadHistory()
	session, sessErr := store.LoadSession()
	var unread []ui.Unread
	for _, e := range []struct {
		name string
		err  error
	}{{"config.yaml", cfgErr}, {"bookmarks.yaml", bmErr}, {"history.yaml", histErr}, {"session.yaml", sessErr}} {
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
	app := ui.New(b, startURLs...).WithStore(bookmarks, folders, cfg, history).WithSession(session).
		WithUnread(unread).WithMoves(moves)

	// Whatever door the program leaves through — q, an outside SIGINT or
	// SIGTERM, the terminal closing — Chromium goes too (terminu family: leave
	// with no child behind). Bubble Tea quits on SIGINT and SIGTERM itself;
	// the terminal closing is SIGHUP, which it leaves alone, so it is turned
	// into the SIGTERM it does know.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)
	go func() {
		<-sig
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	// The lib asks the terminal how wide an icon is before it takes the
	// screen (tdp E5); version, help and browser update never ask.
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
	b.Close()
	switch {
	case errors.Is(runErr, tea.ErrInterrupted):
		os.Exit(130)
	case runErr != nil:
		fmt.Fprintln(os.Stderr, "webu:", runErr)
		os.Exit(1)
	}
}

// ensureChromium returns the pinned Chromium's path, downloading it first
// when it is not there (or when force says to). The download is announced
// before it starts, with its size — a 200 MB fetch is not something to spring
// on somebody who typed a four-letter command (function.md §9 / §12).
func ensureChromium(force bool) (string, error) {
	if exe, ok := browser.Installed(); ok && !force {
		return exe, nil
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	size, err := browser.ArchiveSize(ctx)
	if err != nil {
		if errors.Is(err, browser.ErrUnsupportedPlatform) {
			return "", err
		}
		return "", fmt.Errorf("reaching the Chromium snapshot server: %w", err)
	}
	fmt.Fprintf(os.Stderr, "webu runs its own Chromium (r%d). Downloading %s once", browser.Revision, human(size))
	if dir, err := browser.InstallDir(); err == nil {
		fmt.Fprintf(os.Stderr, " into %s", dir)
	}
	fmt.Fprintln(os.Stderr, " …")
	last := time.Time{}
	exe, err := browser.Download(ctx, func(p browser.Progress) {
		if time.Since(last) < 200*time.Millisecond && p.Done != p.Total {
			return
		}
		last = time.Now()
		fmt.Fprintf(os.Stderr, "\r  %s", progressLine(p))
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("downloading Chromium: %w", err)
	}
	fmt.Fprintln(os.Stderr, "  done.")
	return exe, nil
}

func progressLine(p browser.Progress) string {
	if p.Total <= 0 {
		return human(p.Done)
	}
	pct := int(p.Done * 100 / p.Total)
	bar := strings.Repeat("█", pct/4) + strings.Repeat("░", 25-pct/4)
	return fmt.Sprintf("%s %3d%%  %s / %s", bar, pct, human(p.Done), human(p.Total))
}

func human(n int64) string {
	switch {
	case n < 0:
		return "an unknown number of bytes"
	case n < 1<<20:
		return fmt.Sprintf("%d KB", n>>10)
	default:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	}
}
