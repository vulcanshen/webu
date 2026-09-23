package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/vulcanshen/webu/internal/ir"
)

// sliderW is the track's width in cells: enough to see where the thumb
// stands, not so much that the bar is the row.
const sliderW = 12

// sliderBar draws a slider's track with the thumb where its value stands
// between its minimum and maximum; at the left when the page gave no
// range, or a value that is not a number.
func sliderBar(n *ir.Node) string {
	at := 0
	if v, err := strconv.ParseFloat(strings.TrimSpace(n.Value), 64); err == nil && n.Max > n.Min {
		f := math.Max(0, math.Min(1, (v-n.Min)/(n.Max-n.Min)))
		at = int(math.Round(f * (sliderW - 1)))
	}
	return strings.Repeat("─", at) + "●" + strings.Repeat("─", sliderW-1-at)
}

// sliderValue is the number beside the bar: what the page said, on one
// line, or a dash when it said nothing.
func sliderValue(n *ir.Node) string {
	if v := oneLine(n.Value); v != "" {
		return v
	}
	return "–"
}

// sliderItems is the list Enter opens on a slider: every number on the
// bar, one a row, and the row where it stands now (user, 2026-09-23 —
// typing a number into a box was the wrong tool for a bar). A span too
// long for one number a row is walked in tens, hundreds…, so the list
// stays a list. A slider with no range is 0–100, Chromium's own default.
func sliderItems(n *ir.Node) (items []menuItem, at int) {
	lo, hi := math.Ceil(n.Min), math.Floor(n.Max)
	if hi < lo {
		lo, hi = 0, 100
	}
	step := 1.0
	for (hi-lo)/step > 10000 {
		step *= 10
	}
	now, _ := strconv.ParseFloat(strings.TrimSpace(n.Value), 64)
	for v := lo; v <= hi; v += step {
		hint := ""
		if v == now {
			hint = "current"
		}
		// The key is never a keystroke: "5" must not choose 5 on its way
		// past (spaceMenu.update's letter hotkeys).
		items = append(items, menuItem{label: fmtNum(v), key: "v:" + fmtNum(v), hint: hint})
	}
	at = int(math.Round((math.Max(lo, math.Min(hi, now)) - lo) / step))
	return items, max(0, min(at, len(items)-1))
}

// sliderRange says the ends of the bar the way the list's title says
// them: "0–255".
func sliderRange(n *ir.Node) string {
	return fmtNum(n.Min) + "–" + fmtNum(n.Max)
}

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
