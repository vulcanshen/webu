package ui

import (
	"bytes"
	"os"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/version"
)

// The splash draws the copy of the mark go:embed can reach; docs/icon.svg
// is the one that is edited. The two must be the same file (tdp E6), and
// the lib must take it and the words under it (tdp S5).
func TestTheSplashDrawsDocsIcon(t *testing.T) {
	docs, err := os.ReadFile("../../docs/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(docs, icon) {
		t.Error("internal/ui/icon.svg is not docs/icon.svg: copy docs/icon.svg over it")
	}
	if _, err := terminu.NewSplash(icon, "webu", version.Display(), "A terminal browser that reads web pages"); err != nil {
		t.Errorf("the splash: %v", err)
	}
}
