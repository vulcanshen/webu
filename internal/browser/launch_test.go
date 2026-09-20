package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// TestLaunchAnswers needs the pinned Chromium on this machine; without it the
// test is skipped rather than failed, so a fresh clone still goes green.
func TestLaunchAnswers(t *testing.T) {
	exe, ok := Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var webdriver bool
	var ua string
	err = chromedp.Run(b.Ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Evaluate(`navigator.webdriver === true`, &webdriver),
		chromedp.Evaluate(`navigator.userAgent`, &ua),
	)
	if err != nil {
		t.Fatal(err)
	}
	if webdriver {
		t.Error("navigator.webdriver is true: --enable-automation leaked in")
	}
	if ua == "" {
		t.Error("empty user agent")
	}
	t.Logf("user agent: %s", ua)
}
