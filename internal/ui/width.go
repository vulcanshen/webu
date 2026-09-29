package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

// Everything drawn into a fixed slot goes through these. The card is a
// fixed-width box (tdp L2), so a field that miscounts its own width does not
// merely look off — it pushes the right border out and breaks the frame.
//
// Rule: measure and pad PLAIN text, then apply the style. The measure knows
// how to skip ANSI, but padding a styled string means the pad lands inside the
// styled span and picks up its background.
//
// Width is how far the terminal's cursor moves (tdp D6 v0.1.20). A Nerd Font
// icon is one cell to lipgloss and x/ansi, but some fonts (CJK ones such as
// Maple Mono NF CN) draw it two cells wide and move the cursor two: counted
// as one, every row with an icon pushes its right border out. iconCells is
// what DetectIconWidth measured at startup; at its default of 1 nothing here
// differs from lipgloss's measure. The reference is filu's width.go.

// iconCells is how many cells the terminal moves the cursor for a Nerd
// Font icon: 1, or 2 on a font that draws them wide.
var iconCells = 1

// isWideIcon reports whether r is a Nerd Font icon that such a font draws
// wide: the BMP Private Use Area and supplementary PUA-A (Material Design).
// The powerline caps (U+E0A0–E0D7, the capsules' round ends) are PUA too but
// stay one cell even on those fonts.
func isWideIcon(r rune) bool {
	if r >= 0xe0a0 && r <= 0xe0d7 {
		return false
	}
	return (r >= 0xe000 && r <= 0xf8ff) || (r >= 0xf0000 && r <= 0xffffd)
}

// iconCount counts the wide icons in s, ANSI skipped; 0 at once when icons
// are one cell, so the measure is lipgloss's on a normal font.
func iconCount(s string) int {
	if iconCells == 1 {
		return 0
	}
	n := 0
	for _, r := range ansi.Strip(s) {
		if isWideIcon(r) {
			n++
		}
	}
	return n
}

// lineW is the cells one line takes on screen.
func lineW(s string) int { return ansi.StringWidth(s) + iconCount(s)*(iconCells-1) }

// dispW is the terminal cell width of s: of its widest line, as lipgloss
// measures a block, with each icon taking what the terminal gives it.
func dispW(s string) int {
	if !strings.Contains(s, "\n") {
		return lineW(s)
	}
	return blockWidth(strings.Split(s, "\n"))
}

// blockWidth is the width of the widest line.
func blockWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, lineW(l))
	}
	return w
}

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

// clipANSI cuts a possibly-styled string to w cells without severing an escape
// sequence. truncate() is for plain text; using it on styled output would cut
// mid-ANSI and bleed the style into everything after it. The first w measured
// cells are at least w on screen; each icon among them takes one more, so it
// steps back until they fit — an icon anywhere in the line, not only at its
// start.
func clipANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispW(s) <= w {
		return s
	}
	for target := w; target > 0; target-- {
		if out := ansi.Truncate(s, target, ""); dispW(out) <= w {
			return out
		}
	}
	return ""
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
// icon or wide character cut in half is replaced by spaces, so the result is
// always dispW(s) − n wide.
func cutLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	total := dispW(s)
	if n >= total {
		return ""
	}
	// m measured cells hold at least n on screen; start where they would if
	// every icon so far were narrow, and step up past a wide one.
	m := max(n-iconCount(s)*(iconCells-1), 0)
	for dispW(ansi.Truncate(s, m, "")) < n {
		m++
	}
	rest := ansi.TruncateLeft(s, m, "")
	return strings.Repeat(" ", max(total-n-dispW(rest), 0)) + rest
}

// composite draws fg over bg — overlay.Composite, the same placement, but
// every width is the terminal's, so an icon in the popup or in the row it
// covers cannot push a line past the screen (tdp D6, L4). Left / Top at 0,
// Center at half the background less half the foreground, Right / Bottom
// flush; then moved by the offsets and kept on screen.
func composite(fg, bg string, xPos, yPos overlay.Position, xOff, yOff int) string {
	if fg == "" {
		return bg
	}
	if bg == "" {
		return fg
	}
	fgLines, bgLines := strings.Split(fg, "\n"), strings.Split(bg, "\n")
	fgW, bgW := blockWidth(fgLines), blockWidth(bgLines)
	fgH, bgH := len(fgLines), len(bgLines)
	if fgW >= bgW && fgH >= bgH {
		return fg
	}
	x := clampSpan(placeOffset(xPos, bgW, fgW)+xOff, bgW-fgW)
	y := clampSpan(placeOffset(yPos, bgH, fgH)+yOff, bgH-fgH)
	for i, line := range fgLines {
		if y+i >= bgH {
			break
		}
		row := bgLines[y+i]
		left := clipANSI(row, x)
		left += strings.Repeat(" ", x-dispW(left)) // an icon cut at x, or a short row
		right := cutLeft(row, x+dispW(line))
		bgLines[y+i] = left + line + right
	}
	return strings.Join(bgLines, "\n")
}

// placeOffset is where a span of size fg starts in one of size bg.
func placeOffset(p overlay.Position, bg, fg int) int {
	switch p {
	case overlay.Center:
		return bg/2 - fg/2
	case overlay.Right, overlay.Bottom:
		return bg - fg
	}
	return 0
}

// clampSpan keeps v between 0 and hi (either way round, as overlay does).
func clampSpan(v, hi int) int {
	lo := 0
	if lo > hi {
		lo, hi = hi, lo
	}
	return min(max(v, lo), hi)
}

// center centres s in a w × h area by the terminal's width — lipgloss.Place
// with Center, Center: the smaller half of the gap goes left and on top. In
// a direction s already fills it is left as it is; h 0 centres across only.
func center(w, h int, s string) string {
	lines := strings.Split(s, "\n")
	width := blockWidth(lines)
	if w > width {
		for i, l := range lines {
			gap := w - lineW(l)
			lines[i] = strings.Repeat(" ", gap/2) + l + strings.Repeat(" ", gap-gap/2)
		}
		width = w
	}
	if gap := h - len(lines); gap > 0 {
		blank := strings.Repeat(" ", width)
		out := make([]string, 0, h)
		for range gap / 2 {
			out = append(out, blank)
		}
		out = append(out, lines...)
		for len(out) < h {
			out = append(out, blank)
		}
		lines = out
	}
	return strings.Join(lines, "\n")
}

// joinH lays blocks side by side, top-aligned. Each block's lines are padded
// to that block's own width, so an icon in one column never shoves the next
// one. Replaces lipgloss.JoinHorizontal, whose measure is icon-blind.
func joinH(blocks ...string) string {
	rows := make([][]string, len(blocks))
	widths := make([]int, len(blocks))
	maxRows := 0
	for i, b := range blocks {
		rows[i] = strings.Split(b, "\n")
		widths[i] = blockWidth(rows[i])
		maxRows = max(maxRows, len(rows[i]))
	}
	var out strings.Builder
	for r := 0; r < maxRows; r++ {
		for i := range rows {
			if r < len(rows[i]) {
				l := clipANSI(rows[i][r], widths[i])
				out.WriteString(l + strings.Repeat(" ", widths[i]-dispW(l)))
			} else {
				out.WriteString(strings.Repeat(" ", widths[i]))
			}
		}
		if r < maxRows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// joinV stacks blocks, left-aligned, every line padded to the widest.
// Replaces lipgloss.JoinVertical (icon-blind).
func joinV(blocks ...string) string {
	var lines []string
	for _, b := range blocks {
		lines = append(lines, strings.Split(b, "\n")...)
	}
	w := blockWidth(lines)
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", w-lineW(l))
	}
	return strings.Join(lines, "\n")
}
