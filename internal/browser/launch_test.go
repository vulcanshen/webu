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

// webu signs its own user agent: Chromium's string with the engine named
// the way a browser names it, and webu after it (launch.identify).
func TestWebuSignsItsUserAgent(t *testing.T) {
	got := userAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/131.0.6778.85 Safari/537.36", "0.3.0")
	want := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.85 Safari/537.36 webu/0.3.0"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	m := uaMetadata("HeadlessChrome/131.0.6778.85", "0.3.0")
	if len(m.Brands) != 2 || m.Brands[0].Brand != "Chromium" || m.Brands[0].Version != "131" ||
		m.Brands[1].Brand != "webu" || m.Brands[1].Version != "0.3.0" {
		t.Errorf("brands: %+v", m.Brands)
	}
	if m.FullVersionList[0].Version != "131.0.6778.85" || m.Platform == "" || m.Bitness != "64" {
		t.Errorf("full version list / platform: %+v %q", m.FullVersionList[0], m.Platform)
	}
}
