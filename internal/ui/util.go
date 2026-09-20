package ui

import "strings"

// Small helpers shared by every surface. They live here rather than next to
// their first user so a second user does not have to know where the first
// one was.

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// centerLine centres styled within innerW, measuring plain (styled carries ANSI).
func centerLine(innerW int, plain, styled string) string {
	w := dispW(plain)
	if w >= innerW {
		return padRight(plain, innerW)
	}
	left := (innerW - w) / 2
	return strings.Repeat(" ", left) + styled + strings.Repeat(" ", innerW-left-w)
}

// plural renders a count with its noun, so a message reads as English rather
// than as "1 lines".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return itoa(n) + " " + noun + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// nameOr keeps a title from collapsing to a bare glyph when a name is empty.
func nameOr(name, fallback string) string {
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	return name
}

// fitLines forces body to exactly h lines, each padded to w cells — the
// invariant panelChrome relies on to keep its right border straight.
func fitLines(body []string, w, h int) []string {
	if len(body) > h {
		body = body[:max(0, h)]
	}
	out := make([]string, 0, max(0, h))
	for _, l := range body {
		out = append(out, l+strings.Repeat(" ", max(0, w-dispW(l))))
	}
	for len(out) < h {
		out = append(out, strings.Repeat(" ", max(0, w)))
	}
	return out
}
