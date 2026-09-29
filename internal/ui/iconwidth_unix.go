//go:build darwin || linux

package ui

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

// DetectIconWidth measures how many cells the terminal moves the cursor for a
// Nerd Font icon and sets iconCells so the layout reserves the right space.
// Most fonts take 1 cell; CJK "full-width icon" fonts (Maple Mono NF CN,
// etc.) take 2 while lipgloss still measures 1 — that gap is what breaks the
// borders. A glyph drawn wider than its cell with the cursor moving one is
// one (tdp D6 v0.1.20). It probes with CPR: print an icon at column 1, ask
// the terminal where the cursor ended up. Any failure (not a tty, no CPR
// reply, timeout) leaves iconCells at its default of 1. WEBU_ICON_WIDTH (1
// or 2) overrides it. Call once, before tea.NewProgram. The family builds
// for unix only, so there is no other version of this file.
func DetectIconWidth() {
	if v := os.Getenv("WEBU_ICON_WIDTH"); v != "" { // manual override for flaky CPR
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 2 {
			iconCells = n
			return
		}
	}
	in, out := os.Stdin, os.Stdout
	if !term.IsTerminal(in.Fd()) || !term.IsTerminal(out.Fd()) {
		return
	}
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return
	}
	defer term.Restore(in.Fd(), state)

	icon := string(rune(0xf07b)) // nf-fa-folder — a representative icon
	if _, err := out.WriteString("\r" + icon + "\x1b[6n"); err != nil {
		return
	}
	col, ok := readCPRColumn(int(in.Fd()))
	out.WriteString("\r\x1b[2K") // wipe the probe before Bubble Tea takes the screen
	if ok && col >= 2 {
		iconCells = col - 1 // cursor started at column 1; the icon consumed col-1 cells
	}
}

// readCPRColumn reads a CPR reply "\x1b[<row>;<col>R" within a short deadline and
// returns col. Poll guards against terminals that never answer, so a missing
// reply can't hang startup.
func readCPRColumn(fd int) (int, bool) {
	deadline := time.Now().Add(200 * time.Millisecond)
	var buf []byte
	one := make([]byte, 1)
	for {
		ms := int(time.Until(deadline).Milliseconds())
		if ms <= 0 {
			return 0, false
		}
		n, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, ms)
		if err == unix.EINTR {
			continue
		}
		if err != nil || n == 0 {
			return 0, false
		}
		if _, err := unix.Read(fd, one); err != nil {
			return 0, false
		}
		buf = append(buf, one[0])
		if one[0] == 'R' {
			return parseCPRColumn(buf)
		}
		if len(buf) > 32 {
			return 0, false
		}
	}
}

// parseCPRColumn pulls col out of "…[<row>;<col>R".
func parseCPRColumn(buf []byte) (int, bool) {
	s := string(buf)
	open := strings.IndexByte(s, '[')
	semi := strings.IndexByte(s, ';')
	end := strings.IndexByte(s, 'R')
	if open < 0 || semi < open || end < semi {
		return 0, false
	}
	col, err := strconv.Atoi(s[semi+1 : end])
	if err != nil {
		return 0, false
	}
	return col, true
}
