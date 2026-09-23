package browser

import (
	"runtime"

	"context"
	"fmt"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/vulcanshen/webu/internal/version"
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
	// UserAgent is what webu calls itself to a page, and UAMeta the same
	// for the client-hint headers (identify). Empty when Chromium could
	// not be asked: the page then sees Chromium's own.
	UserAgent string
	UAMeta    *emulation.UserAgentMetadata
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
	b := &Browser{Ctx: ctx, allocCancel: allocCancel}
	b.identify(ef)
	return b, nil
}

// identify works out what webu says it is, from what Chromium says it is.
//
// webu is a browser: Chromium with a terminal for a face. Chromium's own
// string calls the engine "HeadlessChrome", which is the name of a mode,
// not of a browser — and a page told "headless" answers with its
// bot-check instead of itself. So the engine is named the way every
// Chromium-based browser names it, Chrome/<version>, and webu signs
// after it, the way Edge and Vivaldi sign theirs (user, 2026-09-23).
// Not a disguise: every part of it is true, and "webu" is in it.
func (b *Browser) identify(errf func(string, ...any)) {
	var product, ua string
	err := chromedp.Run(b.Ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		_, product, _, ua, _, err = cdpbrowser.GetVersion().Do(ctx)
		return err
	}))
	if err != nil {
		errf("identify: %v", err)
		return
	}
	b.UserAgent = userAgent(ua, version.Version)
	b.UAMeta = uaMetadata(product, version.Version)
}

// userAgent is Chromium's string with the engine named as a browser
// names it, and webu's name and version after it.
func userAgent(chromium, ver string) string {
	return strings.Replace(chromium, "HeadlessChrome/", "Chrome/", 1) + " webu/" + ver
}

// uaMetadata is the same identity for Sec-CH-UA and navigator.userAgentData:
// the brands are Chromium at its version and webu at its own; the
// platform is the one this is running on.
func uaMetadata(product, ver string) *emulation.UserAgentMetadata {
	full := product
	if i := strings.IndexByte(full, '/'); i >= 0 {
		full = full[i+1:]
	}
	major := full
	if i := strings.IndexByte(major, '.'); i >= 0 {
		major = major[:i]
	}
	platform := map[string]string{"darwin": "macOS", "linux": "Linux", "windows": "Windows"}[runtime.GOOS]
	arch := map[string]string{"arm64": "arm", "amd64": "x86"}[runtime.GOARCH]
	return &emulation.UserAgentMetadata{
		Brands: []*emulation.UserAgentBrandVersion{
			{Brand: "Chromium", Version: major}, {Brand: "webu", Version: ver}},
		FullVersionList: []*emulation.UserAgentBrandVersion{
			{Brand: "Chromium", Version: full}, {Brand: "webu", Version: ver}},
		Platform:     platform,
		Architecture: arch,
		Bitness:      "64",
	}
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
