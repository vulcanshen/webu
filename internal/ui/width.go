package ui

import (
	"strings"

	terminu "github.com/vulcanshen/terminu-go"
)

// Everything drawn into a fixed slot goes through these, and every one of
// them measures with terminu-go's display width (tdp E5): an icon and a
// powerline symbol take what the terminal was found to give them at
// startup, so a row with icons in it cannot push a frame out.
//
// Rule: measure and pad PLAIN text, then apply the style. The measure knows
// how to skip ANSI, but padding a styled string means the pad lands inside the
// styled span and picks up its background.

// dispW is the terminal cell width of s: of its widest line.
func dispW(s string) int { return terminu.Width(s) }

// truncate clips s to at most w cells, marking the cut with a single-cell "…".
// w <= 0 yields "". A string that already fits is returned untouched.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispW(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := dispW(string(r))
		if used+rw > w-1 { // leave one cell for the ellipsis
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + strings.Repeat(" ", w-1-used) + "…"
}

// truncateHead cuts from the FRONT, keeping the tail. For a path that is the
// only useful direction: `~/.ssh/config.d/work` and `~/.ssh/config.d/team`
// differ at the end, and cutting there would leave two rows reading alike.
func truncateHead(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispW(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	r := []rune(s)
	used, i := 0, len(r)
	for ; i > 0; i-- {
		rw := dispW(string(r[i-1]))
		if used+rw > w-1 { // leave one cell for the ellipsis
			break
		}
		used += rw
	}
	return "…" + strings.Repeat(" ", w-1-used) + string(r[i:])
}

// clipANSI cuts a possibly-styled string to w cells without severing an
// escape sequence: truncate() is for plain text.
func clipANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispW(s) <= w {
		return s
	}
	return terminu.Cut(s, 0, w)
}

// padRight fits s into exactly w cells, truncating or right-padding as needed.
func padRight(s string, w int) string {
	s = truncate(s, w)
	return s + strings.Repeat(" ", max(0, w-dispW(s)))
}

// padLeft fits s into exactly w cells, right-aligned.
func padLeft(s string, w int) string {
	s = truncate(s, w)
	return strings.Repeat(" ", max(0, w-dispW(s))) + s
}

// cutLeft drops the first n cells of s and returns the rest, styles kept. An
// icon or wide character cut in half becomes spaces, so the result is always
// dispW(s) − n wide.
func cutLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	total := dispW(s)
	if n >= total {
		return ""
	}
	return terminu.Cut(s, n, total)
}

// spaces is n blank cells.
func spaces(n int) string { return strings.Repeat(" ", max(0, n)) }
