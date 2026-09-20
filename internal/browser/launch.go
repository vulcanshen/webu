package browser

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Browser is one running Chromium. Ctx is the browser-level context: a tab is
// chromedp.NewContext(b.Ctx), and every CDP action runs against a tab.
type Browser struct {
	Ctx         context.Context
	allocCancel context.CancelFunc
}

// Launch starts exe headless on profile and waits until it answers.
//
// The flag list starts from what Puppeteer settles on for automation and
// then departs from it in the ways function.md §9 asks for:
//
//   - `--headless=new`: no window, not in the Dock, offscreen render. The old
//     headless mode is a different engine and is not what a user's Chrome runs.
//   - NO `--enable-automation`: with it navigator.webdriver is true and Google
//     login among others refuses the session. AutomationControlled is disabled
//     as a blink feature for the same reason.
//   - the three background-throttling flags stay OFF: a page in a tab the user
//     is not looking at still has to poll (function.md §6).
//   - `--user-data-dir` is webu's own profile, so a login survives a restart
//     and the user's Chrome is never touched.
//
// logw takes chromedp's own log lines. They MUST go somewhere other than
// stderr: the TUI owns the terminal, and chromedp's default is log.Printf,
// which paints over it — the first real-site run showed "unhandled node
// event" (a CDP event newer than this chromedp knows) across the page.
// nil discards.
func Launch(exe, profile string, logw io.Writer) (*Browser, error) {
	if logw == nil {
		logw = io.Discard
	}
	var mu sync.Mutex
	logf := func(prefix string) func(string, ...any) {
		return func(format string, args ...any) {
			line := fmt.Sprintf(format, args...)
			// A CDP event this chromedp has no case for is not news: the
			// page works without it, and w3schools sends one a second.
			if strings.Contains(line, "unhandled node event") || strings.Contains(line, "unhandled page event") {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			fmt.Fprintf(logw, "%s %s%s\n", time.Now().Format("2006-01-02 15:04:05"), prefix, line)
		}
	}
	lf, ef := logf(""), logf("ERROR: ")
	opts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(exe),
		chromedp.UserDataDir(profile),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-breakpad", true),
		// Honoured on Linux, where the crash database would otherwise land
		// in the default profile. macOS ignores it and every other switch
		// tried (--disable-breakpad, --disable-crash-reporter): Chromium
		// there always keeps an empty Crashpad database under
		// ~/Library/Application Support/Chromium — a few KB, and nothing
		// webu can redirect.
		chromedp.Flag("crash-dumps-dir", filepath.Join(profile, "Crashpad")),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "Translate"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
		chromedp.WindowSize(1280, 900),
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	// The Browser's errf is what every Target inherits (chromedp
	// browser.go), so the browser option is the one that matters; the
	// context options cover the rest.
	ctx, _ := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(lf), chromedp.WithErrorf(ef),
		chromedp.WithBrowserOption(chromedp.WithBrowserLogf(lf), chromedp.WithBrowserErrorf(ef)))
	// Run with no actions starts the process and opens the first target; an
	// error here is the browser failing to come up at all.
	if err := chromedp.Run(ctx); err != nil {
		allocCancel()
		return nil, fmt.Errorf("start chromium: %w", err)
	}
	return &Browser{Ctx: ctx, allocCancel: allocCancel}, nil
}

// Close asks Chromium to quit and waits for it, then releases the allocator.
// Graceful first — Browser.close lets the profile flush cookies and storage —
// and the allocator's cancel is the kill behind it, so a browser that will not
// answer still goes (u-family: leave with no child left behind).
func (b *Browser) Close() {
	if b == nil {
		return
	}
	_ = chromedp.Cancel(b.Ctx)
	b.allocCancel()
}
