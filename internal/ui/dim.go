package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Everything below the topmost popup is drawn dim (tdp F8): only the layer
// being worked on is bright, since every popup is one width (F7) and the
// one on top hides the sides of the one below. dimANSI does it to text
// already drawn, so no view needs a dim variant of itself:
//
//   - a border in a popup layer colour (popupLayerColor) turns into the
//     dimmed version of that colour — dark, but still saying which layer
//     it is (F8, v0.1.9);
//   - every other foreground, and text with none, turns into dimColor —
//     warnings and streaming content included (the exception to T2);
//   - backgrounds go: a cursor bar or a table ground under a layer that
//     is not being worked on is one more bright thing.
//
// The layout does not move: only the SGR sequences change.
func dimANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/4)
	b.WriteString(dimFg)
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				b.WriteString(dimSGR(s[i+2 : j]))
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

var (
	dimFg     = fgSGR(dimColor)
	dimLayers = func() []layerDim {
		var ls []layerDim
		for layer := 1; layer <= 4; layer++ {
			c := popupLayerColor(layer)
			r, g, b := hexRGB(string(c))
			ls = append(ls, layerDim{r, g, b, fgSGR(lerpHex(string(c), baseHex, 0.55))})
		}
		return ls
	}()
)

// layerDim is a popup layer colour and the SGR of its dimmed version.
type layerDim struct {
	r, g, b int
	sgr     string
}

// layerDimOf is the dimmed version of the layer colour r;g;b is, if it is
// one. Near, not equal: the renderer rounds a hex colour on its way out
// (#94C3F5 is drawn as 147;195;245).
func layerDimOf(rgb []string) (string, bool) {
	if len(rgb) != 3 {
		return "", false
	}
	var v [3]int
	for i, p := range rgb {
		n, err := strconv.Atoi(p)
		if err != nil {
			return "", false
		}
		v[i] = n
	}
	near := func(a, b int) bool { return a-b <= 2 && b-a <= 2 }
	for _, l := range dimLayers {
		if near(v[0], l.r) && near(v[1], l.g) && near(v[2], l.b) {
			return l.sgr, true
		}
	}
	return "", false
}

// dimSGR rewrites one SGR's parameters: the attributes that reset, then
// the dim foreground — a layer colour's own dimmed version, or dimColor.
func dimSGR(params string) string {
	ps := strings.Split(params, ";")
	fg := dimFg
	for k := 0; k < len(ps); k++ {
		switch ps[k] {
		case "38", "48":
			if k+1 < len(ps) && ps[k+1] == "2" && k+4 < len(ps) {
				if ps[k] == "38" {
					if d, ok := layerDimOf(ps[k+2 : k+5]); ok {
						fg = d
					}
				}
				k += 4
			} else if k+1 < len(ps) && ps[k+1] == "5" {
				k += 2
			}
		}
	}
	return "\x1b[0m" + fg
}

func fgSGR(c lipgloss.Color) string { return "\x1b[38;2;" + rgbKey(c) + "m" }

// rgbKey is a hex colour as the r;g;b of a truecolor SGR.
func rgbKey(c lipgloss.Color) string {
	r, g, b := hexRGB(string(c))
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
