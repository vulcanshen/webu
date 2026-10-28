package ui

import (
	"os"
	"testing"
)

// TestMain: the tests from before the reader (docs/reader.md) drive their
// pages through Chromium, and keep doing so until stage 4 settles which
// pages it still draws; the reader tests make their own tabs readers.
func TestMain(m *testing.M) {
	chromiumPagesDefault = true
	os.Exit(m.Run())
}
