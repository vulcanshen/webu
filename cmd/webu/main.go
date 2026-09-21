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
	// `webu browser update` is the one door to a new revision (function.md
	// §9): a rebuilt binary carries a new pin, and this fetches it.
	if len(args) == 2 && args[0] == "browser" && args[1] == "update" {
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
	// terminal the TUI is drawing on. Failing to open it is not fatal: the
	// lines are dropped instead.
	var logw io.Writer = io.Discard
	if f, err := os.OpenFile(filepath.Join(filepath.Dir(profile), "webu.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		defer f.Close()
		logw = f
	}
	b, err := browser.Launch(exe, profile, logw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "webu:", err)
		os.Exit(1)
	}

	// webu's own files. None of them is fatal: a file that will not parse
	// is news for the toast, not a reason to withhold the browser.
	bookmarks, folders, bmErr := store.LoadBookmarks()
	cfg, cfgErr := store.LoadConfig()
	history, histErr := store.LoadHistory()
	session, sessErr := store.LoadSession()
	for _, e := range []struct {
		name string
		err  error
	}{{"bookmarks.yaml", bmErr}, {"config.yaml", cfgErr}, {"history.yaml", histErr}, {"session.yaml", sessErr}} {
		if e.err != nil {
			fmt.Fprintf(os.Stderr, "webu: %s: %v (running without it)\n", e.name, e.err)
		}
	}
	app := ui.New(b, startURLs...).WithStore(bookmarks, folders, cfg, history).WithSession(session)
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Whatever door the program leaves through — q, an outside SIGINT or
	// SIGTERM, the terminal closing — Chromium goes too (u-family: leave
	// with no child behind).
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		p.Quit()
	}()

	final, runErr := p.Run()
	if a, ok := final.(ui.AppModel); ok {
		// The session is written on every way out, q or a signal, so the
		// tabs come back next time (ux.md §6).
		if err := store.SaveSession(a.Session()); err != nil {
			fmt.Fprintln(os.Stderr, "webu: session.yaml:", err)
		}
		a.Close()
	}
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
