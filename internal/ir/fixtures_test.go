package ir_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
	"github.com/vulcanshen/webu/internal/page"
)

// Every role in the whitelist has a fixture (function.md §3): a small HTML
// file, what the PINNED Chromium reports for it (the AX tree and the layout
// display map), and the IR the translator must produce from that.
//
//	go test ./internal/ir                     checks JSON → IR against the goldens
//	go test ./internal/ir -update             rewrites the goldens from the current Build
//	go test ./internal/ir -capture -update    re-captures the JSON with webu's Chromium first
//
// The JSON is committed so the check needs no browser; the capture is the
// only step that does, and it refuses any Chromium but the pinned one —
// a fixture from another build would be answering for a different engine.
var (
	capture = flag.Bool("capture", false, "re-capture testdata/*.json with the pinned Chromium")
	update  = flag.Bool("update", false, "rewrite testdata/*.golden from the current Build")
)

func TestFixtures(t *testing.T) {
	htmls, err := filepath.Glob("testdata/*.html")
	if err != nil || len(htmls) == 0 {
		t.Fatal("no fixtures under testdata/")
	}
	sort.Strings(htmls)

	var b *browser.Browser
	if *capture {
		exe, ok := browser.Installed()
		if !ok {
			t.Fatal("-capture needs the pinned Chromium; run webu once to download it")
		}
		b, err = browser.Launch(exe, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
	}

	for _, html := range htmls {
		name := strings.TrimSuffix(filepath.Base(html), ".html")
		t.Run(name, func(t *testing.T) {
			jsonPath := filepath.Join("testdata", name+".json")
			if *capture {
				c := captureTree(t, b, html)
				raw, err := json.MarshalIndent(c, "", " ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(jsonPath)
			if err != nil {
				t.Fatalf("%s has no captured tree yet: run `make fixtures`", name)
			}
			var c ir.Capture
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			got := ir.Dump(ir.Build(c))
			goldenPath := filepath.Join("testdata", name+".golden")
			if *update {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%s has no golden yet: run with -update", name)
			}
			if got != string(want) {
				t.Errorf("IR differs from golden\n--- want\n%s--- got\n%s", want, got)
			}
		})
	}
}

// captureTree loads one fixture in a fresh tab and returns what the
// translator would see.
func captureTree(t *testing.T, b *browser.Browser, html string) ir.Capture {
	t.Helper()
	abs, err := filepath.Abs(html)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := chromedp.NewContext(b.Ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var c ir.Capture
	err = chromedp.Run(ctx,
		chromedp.Navigate("file://"+abs),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			c, err = page.Capture(ctx)
			return err
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
