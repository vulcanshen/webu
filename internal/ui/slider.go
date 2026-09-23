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

// sliderRange says the ends of the bar the way the popup's border says
// them: "0–255".
func sliderRange(n *ir.Node) string {
	return fmtNum(n.Min) + "–" + fmtNum(n.Max)
}

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
