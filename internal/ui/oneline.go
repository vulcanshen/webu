package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// A one-line value — a URL, a field's text, a password, a name, a query,
// a filter — and what comes into it (terminu, 2026-10-06; to be tdp's
// components/input). A line break or a tab pasted in stays in the value,
// and is drawn as a red `\n` or `\t`: two cells, never cut, told apart
// from a `\` and an `n` typed. Any other control character is dropped.
//
// Until then a paste went in as it came: its line break broke the box's
// row in two (user: show it as `\n` or `\t`, plainly — turning it into a
// space or dropping it would change the value without a word).
//
// Only a paste brings one: Bubble Tea hands a bracketed paste over whole,
// as one KeyRunes, while Tab, Enter and Ctrl-J pressed are keys of their
// own and do what they always did.

// takeText is what of rs goes into a one-line value: "\r\n" as one "\n",
// a line break or a tab as it is, any other control character — the rest
// of C0, DEL, C1 — dropped. With "\r\n" one rune, a rune is a unit:
// Backspace, a mask's dots and the measure need nothing more.
func takeText(rs []rune) string {
	var b strings.Builder
	for i, r := range rs {
		switch {
		case r == '\r' && i+1 < len(rs) && rs[i+1] == '\n':
			// The "\n" after it is the one kept.
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r >= 0x7f && r <= 0x9f:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cleanValue is takeText for a value that was there before the box
// opened — a page's prompt() default, a field's text, a bookmark's title,
// a setting: the same filter for both ways in, or an ESC in one would go
// straight to the terminal.
func cleanValue(s string) string { return takeText([]rune(s)) }

// unitShown is how a rune of a value is drawn, and whether it is a line
// break or a tab.
func unitShown(r rune) (string, bool) {
	switch r {
	case '\n', '\r':
		return `\n`, true
	case '\t':
		return `\t`, true
	}
	return string(r), false
}

// valueView draws v in at most w cells: what was typed in st, a line break
// or a tab in esc. What does not fit goes from the front when head — the
// end, where the caret is, stays in view — or from the back otherwise, a
// `…` where it went (truncateHead, truncate); a `\n` goes whole or not at
// all. It returns the width drawn.
//
// A lit value draws esc red (warnColor). One grey throughout — an offer,
// a filter nobody is typing into, the finder's query while its list has
// the keys (tdp D3) — is drawn plain, with no style for either, and its
// row greyed in one run.
func valueView(v string, w int, st, esc lipgloss.Style, head bool) (string, int) {
	if w <= 0 {
		return "", 0
	}
	rs := []rune(v)
	cells := func(r rune) int { s, _ := unitShown(r); return dispW(s) }
	total := 0
	for _, r := range rs {
		total += cells(r)
	}
	from, to, used := 0, len(rs), total
	// On the cut end: the `…`, and the spaces a `\n` too wide for what
	// is left leaves.
	pre, post := "", ""
	if total > w {
		// One cell for the `…`; whole units in the rest.
		used = 0
		if head {
			for from = len(rs); from > 0 && used+cells(rs[from-1]) <= w-1; from-- {
				used += cells(rs[from-1])
			}
			pre = "…" + strings.Repeat(" ", w-1-used)
		} else {
			for to = 0; to < len(rs) && used+cells(rs[to]) <= w-1; to++ {
				used += cells(rs[to])
			}
			post = strings.Repeat(" ", w-1-used) + "…"
		}
		used = w
	}
	var b, run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			b.WriteString(st.Render(run.String()))
			run.Reset()
		}
	}
	run.WriteString(pre)
	for _, r := range rs[from:to] {
		s, ctl := unitShown(r)
		if ctl {
			flush()
			b.WriteString(esc.Render(s))
			continue
		}
		run.WriteString(s)
	}
	run.WriteString(post)
	flush()
	return b.String(), used
}

// filterRow is a list's `/` filter row, w cells: lavender with caret at
// its end while it is being typed, grey once it only stands.
func filterRow(filter string, typing bool, w int, caret lipgloss.Style) string {
	if typing {
		edit := lipgloss.NewStyle().Foreground(editColor)
		v, vw := valueView(filter, w-4, edit, lipgloss.NewStyle().Foreground(warnColor), false)
		return edit.Render(" / ") + v + spaces(w-4-vw) + caret.Render(" ")
	}
	v, vw := valueView(filter, w-3, lipgloss.NewStyle(), lipgloss.NewStyle(), false)
	return lipgloss.NewStyle().Foreground(dimColor).Render(" / " + v + spaces(w-3-vw))
}
