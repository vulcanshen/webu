package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vulcanshen/webu/internal/page"
)

// A console entry is shown whole: a long line wraps, a multi-line one
// keeps its lines, nothing ends in an ellipsis (revised 2026-09-21).
func TestConsoleShowsEntriesWhole(t *testing.T) {
	ansi := regexp.MustCompile("\x1b\\[[0-9;]*m")
	tab := devConsoleTab{entries: []page.ConsoleEntry{
		{Level: "log", Text: strings.Repeat("word ", 40), At: time.Now()},
		{Level: "error", Text: "Error: boom\n    at app.js:1:1", At: time.Now(), Where: "app.js:1"},
	}}
	rows := tab.view(60, 20, "")
	plain := ansi.ReplaceAllString(strings.Join(rows, "\n"), "")
	if strings.Contains(plain, "…") || !strings.Contains(plain, "at app.js:1:1") || strings.Count(plain, "word") != 40 {
		t.Errorf("entries should be whole:\n%s", plain)
	}
	for i, r := range rows {
		if w := dispW(r); w != 60 {
			t.Errorf("row %d is %d wide", i, w)
		}
	}
	// With the cursor on the last entry and room for only its rows, both
	// of its lines are on screen and the first entry is scrolled off.
	tab.cursor = 1
	plain = ansi.ReplaceAllString(strings.Join(tab.view(60, 2, ""), "\n"), "")
	if !strings.Contains(plain, "Error: boom") || !strings.Contains(plain, "at app.js:1:1") || strings.Contains(plain, "word") {
		t.Errorf("the cursor entry should be whole and alone:\n%s", plain)
	}
}
