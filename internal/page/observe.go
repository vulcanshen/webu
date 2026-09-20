package page

import (
	"context"

	"github.com/chromedp/cdproto/fetch"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// MutationBinding is the name of the function the injected observer calls
// when the DOM has changed. The tab's listener turns Runtime.bindingCalled
// with this name into a redraw (function.md §6: the third option, the one
// browser-use tools settle on).
const MutationBinding = "webuMutated"

// observerScript watches the whole document and reports at most once per
// 150 ms. Attributes are not watched: a hover that flips a class would keep
// the terminal repainting for nothing, and what the user reads is text and
// structure.
const observerScript = `(() => {
	let pending = null;
	const report = () => {
		if (pending) return;
		pending = setTimeout(() => { pending = null; try { ` + MutationBinding + `(""); } catch (e) {} }, 150);
	};
	new MutationObserver(report).observe(document, {subtree: true, childList: true, characterData: true});
})();`

// Prepare arms a tab: the binding the observer will call, the observer
// itself injected into every document the tab loads from now on, and — for
// a tab that already has a document, one the page opened itself — the
// observer run in that document too. Run once per tab.
func Prepare(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := enableDevDomains(ctx); err != nil {
				return err
			}
			// HTTP auth challenges and file choosers are webu's to answer
			// (function.md §5). Fetch with auth handling pauses every
			// request until it is continued — the tab's listener does that
			// (ContinueRequest); only a challenge reaches the UI.
			if err := fetch.Enable().WithHandleAuthRequests(true).Do(ctx); err != nil {
				return err
			}
			if err := cdppage.SetInterceptFileChooserDialog(true).Do(ctx); err != nil {
				return err
			}
			if err := runtime.AddBinding(MutationBinding).Do(ctx); err != nil {
				return err
			}
			if _, err := cdppage.AddScriptToEvaluateOnNewDocument(observerScript).Do(ctx); err != nil {
				return err
			}
			_, _, err := runtime.Evaluate(observerScript).Do(ctx)
			return err
		}),
	)
}
