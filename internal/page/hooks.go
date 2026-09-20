package page

import (
	"context"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/security"
	"github.com/chromedp/chromedp"
)

// The non-DOM hooks (function.md §5): what webu answers on the page's behalf
// once its own UI has asked the user.

// Source is the document's HTML as the page has it now (function.md §8
// view source).
func Source(ctx context.Context) (string, error) {
	var html string
	err := chromedp.Run(ctx, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
	return html, err
}

// HandleDialog answers an alert / confirm / prompt / beforeunload. text is
// the prompt's answer and ignored for the rest.
func HandleDialog(ctx context.Context, accept bool, text string) error {
	return run(ctx, func(ctx context.Context) error {
		p := cdppage.HandleJavaScriptDialog(accept)
		if text != "" {
			p = p.WithPromptText(text)
		}
		return p.Do(ctx)
	})
}

// IgnoreCertErrors makes THIS tab proceed past certificate errors from now
// on. The per-error question Chrome used to offer (Security.certificateError)
// is gone from the protocol; this is the switch that is left, so webu asks
// once per tab and then stops asking for that tab (function.md §8).
func IgnoreCertErrors(ctx context.Context) error {
	return run(ctx, func(ctx context.Context) error {
		return security.SetIgnoreCertificateErrors(true).Do(ctx)
	})
}

// SetDownloads points the browser's downloads at dir and turns on the
// events that report them (Browser.downloadWillBegin / downloadProgress,
// on the browser session). Browser-level, so it is run against the browser
// rather than a tab.
func SetDownloads(ctx context.Context, dir string) error {
	c := chromedp.FromContext(ctx)
	if c == nil || c.Browser == nil {
		return chromedp.ErrInvalidContext
	}
	return browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).
		WithDownloadPath(dir).WithEventsEnabled(true).
		Do(cdp.WithExecutor(ctx, c.Browser))
}
