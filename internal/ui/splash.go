package ui

import (
	_ "embed"

	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/version"
)

// icon is webu's version of the family's mark: the U frame with W, E and
// B in it. docs/icon.svg is the one to edit; go:embed cannot reach docs/,
// so this is a copy, and TestTheSplashDrawsDocsIcon keeps the two the same
// (tdp E6).
//
//go:embed icon.svg
var icon []byte

// splash is the family's easter egg, V on a panel (tdp S): the lib draws
// and plays it.
func splash() terminu.Splash {
	s, err := terminu.NewSplash(icon, "webu", version.Display(), "A terminal browser on a real Chromium")
	if err != nil {
		panic(err) // the embedded icon or the words are wrong: a mistake in the program
	}
	return s
}
