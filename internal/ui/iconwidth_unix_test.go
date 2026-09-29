//go:build darwin || linux

package ui

import "testing"

func TestParseCPRColumn(t *testing.T) {
	cases := []struct {
		in      string
		wantCol int
		wantOK  bool
	}{
		{"\x1b[1;2R", 2, true}, // icon consumed 1 cell (cursor at col 2)
		{"\x1b[1;3R", 3, true}, // icon consumed 2 cells (CJK font)
		{"\x1b[24;80R", 80, true},
		{"\x1b[1;R", 0, false}, // no column
		{"\x1b[1;2", 0, false}, // no terminator
		{"garbage", 0, false},
	}
	for _, c := range cases {
		col, ok := parseCPRColumn([]byte(c.in))
		if col != c.wantCol || ok != c.wantOK {
			t.Errorf("parseCPRColumn(%q) = (%d, %v), want (%d, %v)", c.in, col, ok, c.wantCol, c.wantOK)
		}
	}
}

// WEBU__ICON_WIDTH overrides the probe; the old WEBU_ICON_WIDTH is not
// read (tdp D6 v0.1.21). Not a terminal here, so without the override the
// probe leaves one cell.
func TestIconWidthOverride(t *testing.T) {
	was := iconCells
	t.Cleanup(func() { iconCells = was })
	t.Setenv("WEBU__ICON_WIDTH", "")
	t.Setenv("TERMINU__ICON_WIDTH", "") // the one a family app outside may have set
	t.Setenv("WEBU_ICON_WIDTH", "2")
	iconCells = 1
	DetectIconWidth()
	if iconCells != 1 {
		t.Errorf("the old name set %d cells", iconCells)
	}
	t.Setenv("WEBU__ICON_WIDTH", "2")
	DetectIconWidth()
	if iconCells != 2 {
		t.Errorf("WEBU__ICON_WIDTH=2 gave %d cells", iconCells)
	}
}

// The order is WEBU__ICON_WIDTH, then TERMINU__ICON_WIDTH (set by a family
// app with a PTY for what runs in it), then the probe; only 1 or 2 counts
// (tdp D6 v0.1.22). Not a terminal here, so the probe leaves 1.
func TestIconWidthOrder(t *testing.T) {
	was := iconCells
	t.Cleanup(func() { iconCells = was })
	for _, tc := range []struct {
		own, family string
		want        int
	}{
		{"1", "2", 1}, // the app's own wins
		{"", "2", 2},  // inside another family app's PTY
		{"3", "2", 2}, // an own value out of range counts as unset
		{"", "x", 1},  // so does a family one: the probe
		{"", "", 1},   // neither: the probe
		{"2", "", 2},
	} {
		t.Setenv("WEBU__ICON_WIDTH", tc.own)
		t.Setenv("TERMINU__ICON_WIDTH", tc.family)
		iconCells = 1
		DetectIconWidth()
		if iconCells != tc.want {
			t.Errorf("WEBU__ICON_WIDTH=%q TERMINU__ICON_WIDTH=%q: %d cells, want %d", tc.own, tc.family, iconCells, tc.want)
		}
	}
}
