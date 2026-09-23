package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// webu has TWO palettes, not one (2026-09-22, the user's call).
//
// The APP palette is webu's own chrome — panels, borders, the header
// chain, the pagetab, the cursor, menus, popups, the footer. It obeys
// the VTP in full: a few anchors, lightness as the z-axis, one reserved
// band per meaning, override colours outside the hierarchy.
//
// The PAGE palette is the document's own structure — headings, links,
// code, tables, quotes, fields. A web page is a structured document with
// a visual language of its own; webu does not read its CSS, but it still
// has to draw that structure, and drawing it out of the app's reserved
// bands is what kept forcing a choice between the two: a table header
// wanted mauve and so did a code key, a highlight wanted lavender and so
// did the selection. So the page's colours are a closed set of their
// own. They appear only inside panel [2]'s content, they take no part in
// webu's z-axis — a page is always at one depth, the canvas — and their
// bands are reserved among themselves alone. Where a page colour and an
// app colour share a hex today, that is two independent decisions
// landing on the same shade, not one band used twice.
//
// Anchors are catppuccin-mocha (ui.md §4 / VTP §B): assigned once,
// derived everywhere.

// ---- App palette: webu's own chrome.
var (
	// structural — panel chrome and the KEY half of every legend (§4.4).
	focusColor = lipgloss.Color("#89b4fa") // blue
	borderDim  = lipgloss.Color("#585b70") // surface2: unfocused border
	// cursor — "the current hand".
	handColor = lipgloss.Color("#bac2de") // subtext1
	// the field under edit in an input popup.
	editColor = lipgloss.Color("#b4befe") // lavender
	// neutral text.
	textColor = lipgloss.Color("#cdd6f4") // text
	dimColor  = lipgloss.Color("#6c7086") // overlay0: glyphs, hints, secondary
	// override — outside the brightness hierarchy (§2.4): red is "something
	// is wrong", peach is "worth catching, nothing broken".
	warnColor  = lipgloss.Color("#f38ba8") // red
	peachColor = lipgloss.Color("#fab387") // peach
	// the tab panel [2] is showing (ui.md §4): the ONE thing green says.
	liveColor = lipgloss.Color("#a6e3a1") // green
	// selection mode's frame (ux.md §1).
	selectColor = lipgloss.Color("#f9e2af") // yellow
	// the pagetab's segments — the page's own chrome, told apart from the
	// page's text without a ground of its own (2026-09-22). It was mauve
	// until mauve went to the page's input group, which needed a band
	// more than the pagetab needed that particular one (user).
	headerColor = lipgloss.Color("#f5e0dc") // rosewater
	// visual mode's swept text: lavender under it, the band the family
	// gives to "the thing you are changing" (an input under edit) — and a
	// selection is that.
	selectionBg = lipgloss.Color("#b4befe") // lavender
	// the cursor row in a list.
	rowSelColor = focusColor
	// the URL on panel [2]'s first row: "where you are", which is the
	// structural band's question, so it shares the hex on purpose (as
	// sshu's nestColor shares editColor). Named separately so the sharing
	// is deliberate and greppable.
	urlColor = focusColor
)

const (
	baseHex  = "#1e1e2e" // canvas; also dark text on a bright chip
	crustHex = "#11111b" // recessed background of an inactive capsule
)

// ---- Page palette: the document's own structure, inside panel [2].
//
// Nothing here derives from the app palette and nothing there derives
// from here. A page is drawn at one depth, so this set encodes KIND —
// heading, link, code, table, field — rather than elevation.
// A colour here stands for a CONCEPT, not for an element (user,
// 2026-09-22). There are more kinds of thing on a page than a palette can
// hold, and giving each its own hue would be a legend to memorise rather
// than a language to read. What the reader actually needs to know is what
// a thing IS FOR: can I press it, do I fill it in, is it code, is it
// furniture. Elements that answer the same way share a colour and are
// told apart by their glyph.
var (
	pageText = lipgloss.Color("#cdd6f4") // text: the page's prose
	// A placeholder, a marker, a role webu cannot draw: present,
	// secondary, never competing with the prose.
	pageDim = lipgloss.Color("#6c7086") // overlay0
	// Press it: a link and a button are one thing to the reader — the
	// glyph says which, and a link keeps its underline so it is still a
	// link on a terminal with the colours flattened.
	pageClick = lipgloss.Color("#74c7ec") // sapphire
	// Fill it in: a text field, a check box, a radio, a select. The
	// input group, which is one concept however many controls it has.
	pageInput = lipgloss.Color("#cba6f7") // mauve
	// One cell, where the next character would go. The field's glyph has
	// already said what this is, so the value needs no bed under it and
	// no dashes beside it — only a mark that there is somewhere to type
	// (user, 2026-09-22). It is what sshu's form shows in an empty
	// field: one lit cell and nothing else.
	pageInputBg = lipgloss.Color("#45475a") // surface1
	// Code, inline and block, keys included: one colour for one concept.
	// A block sits on a ground so it reads as a block.
	pageCode   = lipgloss.Color("#f5c2e7") // pink
	pageCodeBg = lipgloss.Color("#313244") // surface0
	// An image, a video, a frame: a marker that something is there which
	// a terminal cannot show. Secondary, never competing with the prose —
	// but not invisible either. It was a step darker for a day and that
	// was too far: on a page where the icons ARE the data, an alt that
	// says "Priority: Highest" is the only copy of that fact (user,
	// 2026-09-22).
	pageMedia = lipgloss.Color("#6c7086") // overlay0
	// A data table's ground, its header row one step up: the header is
	// told by ground, not by foreground (revised 2026-09-21).
	pageTableBg     = lipgloss.Color("#313244") // surface0
	pageTableHeadBg = lipgloss.Color("#45475a") // surface1
)

// levelInk is the colour a name wears for how deep it sits: five hues,
// cycled, so a sixth level starts over rather than running out of
// colours (user, 2026-09-22).
//
// This replaced a grey ground per level (headingBg, v0.2.1). A ground
// says depth by weight, which means six shades of one grey to tell apart
// — and it left the whole outline reading as a stack of bands. Depth is
// drawn on the ink instead, and the shape of the hierarchy is drawn by
// the tree that carries it.
//
// A full rainbow was the first try and red was wrong in it (user): red
// means something is wrong, in this app and everywhere else, and a top
// level heading is not an error. The cycle starts at peach and runs warm
// to cool, which also keeps the two halves of a deep outline apart at a
// glance.
//
// Sapphire and lavender were in it next and came out: a heading wearing
// a link's colour, in the same prose a link appears in, is a collision
// rather than a palette. Sapphire is still spoken for — it is the
// press-it band. Lavender is free again since the fill-it-in band moved
// to mauve, and could come back here if five hues ever prove too few.
// Pink is code and stayed out. Flamingo is a code block's numbers, which
// only ever appear on the code ground, so it never meets a heading on
// equal footing; teal was unused.
//
// None of these take part in the app's reserved bands — that is what the
// two palettes are for; they appear only inside a page's own structure.
var levelInk = []lipgloss.Color{
	lipgloss.Color("#fab387"), // peach
	lipgloss.Color("#f9e2af"), // yellow
	lipgloss.Color("#a6e3a1"), // green
	lipgloss.Color("#94e2d5"), // teal
	lipgloss.Color("#f2cdcd"), // flamingo
}

// levelColor is the ink for a depth, counting from 1.
func levelColor(depth int) lipgloss.Color {
	if depth < 1 {
		depth = 1
	}
	return levelInk[(depth-1)%len(levelInk)]
}

// lerpHex mixes two "#rrggbb" colours channel by channel; t is 0 for a,
// 1 for b, clamped either side.
func lerpHex(a, b string, t float64) lipgloss.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab := hexRGB(a)
	br, bg, bb := hexRGB(b)
	mix := func(x, y int) int { return x + int(float64(y-x)*t+0.5) }
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb)))
}

func hexRGB(s string) (int, int, int) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	v, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0, 0, 0
	}
	return int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)
}

// spinnerFrames stands in for the web glyph in panel [2]'s URL row while
// the page is on its way (2026-09-22). A page being fetched used to be
// told only by the text dimming, which reads as a hang rather than as
// work; something has to move.
//
// Braille dots were tried first — the terminal's own idiom for this —
// and they were wrong here: a braille cell is one column and tall, the
// web glyph is a Nerd Font one and square across two, so the row jumped
// a column whenever a fetch began and the shape changed with it. These
// are the Material Design circle slices, the same set the web glyph
// comes from: one square cell filling round, in place, at the same size.
// Codepoints read out of the installed Nerd Font's cmap, never
// remembered (family rule).
var spinnerFrames = []string{
	string(rune(0xf0a9e)), string(rune(0xf0a9f)), string(rune(0xf0aa0)), string(rune(0xf0aa1)),
	string(rune(0xf0aa2)), string(rune(0xf0aa3)), string(rune(0xf0aa4)), string(rune(0xf0aa5)),
}

// spinStep is how long one frame of it lasts.
const spinStep = 90 * time.Millisecond

// spinnerFrame is the frame due now. It is read from the clock rather
// than counted, so the animation is right however many redraws land —
// and a stray tick costs a redraw, never a jump.
func spinnerFrame() string {
	return spinnerFrames[(time.Now().UnixNano()/int64(spinStep))%int64(len(spinnerFrames))]
}

// Nerd Font glyphs. Never a PUA literal in source — built from the rune so
// the codepoint stays greppable and the file stays editor-safe (family
// rule). One role, one glyph, one table (ux.md §B).
var (
	capLeft  = string(rune(0xe0b6)) // powerline round-left  — chip start
	capRight = string(rune(0xe0b4)) // powerline round-right — chip end
	// The seams slant like a / (revised 2026-09-21: the filled right
	// triangle inherited from sshu read as an arrowhead here). A hard seam
	// is the lit block's own edge, so it is a FILLED shape: the upper-left
	// triangle in the block's colour over the next fill, which cuts the
	// block on a / line — a thin slash beside a square-ended block was
	// tried first and left the block square. A soft seam, between two
	// unlit chips, is the thin slash (chrome.go divider).
	dividerHard = string(rune(0xe0bc)) // ple-upper_left_triangle
	dividerSoft = string(rune(0xe0bb)) // ple-forwardslash_separator

	// Popup titles — the type signal half of a surface label. Every
	// codepoint below was read out of the Nerd Font glyph table
	// (glyphnames.json), not remembered: the first draft of this file had
	// six that were not what their comments said.
	glyphMenu   = string(rune(0xf0c9))  // nf-fa-bars            — Space menu
	glyphHelp   = string(rune(0xf059))  // nf-fa-question_circle — help
	glyphWarn   = string(rune(0xf071))  // nf-fa-warning         — confirm
	glyphInfo   = string(rune(0xf05a))  // nf-fa-info_circle     — toast
	glyphSearch = string(rune(0xf002))  // nf-fa-search          — goto / search
	glyphPencil = string(rune(0xf040))  // nf-fa-pencil          — input popup
	glyphList   = string(rune(0xf0279)) // nf-md-format_list_bulleted — outline

	// The page's four parts (parts.go), one Material Design family so
	// the set reads as one set. Worn twice: on the pagetab, where they
	// replaced a single hamburger that said only "this row is a menu",
	// and in the search list, where a hit has to say which part of the
	// page it was found in (user, 2026-09-23).
	glyphHeader = string(rune(0xf06fc)) // nf-md-page_layout_header
	glyphBody   = string(rune(0xf06fa)) // nf-md-page_layout_body
	glyphOthers = string(rune(0xf06fd)) // nf-md-page_layout_sidebar_left
	glyphFooter = string(rune(0xf06fb)) // nf-md-page_layout_footer

	// The header's three popups (ux.md §B: one role, one glyph).
	glyphBookmark = string(rune(0xf00c0)) // nf-md-bookmark
	glyphHistory  = string(rune(0xf02da)) // nf-md-history
	glyphDownload = string(rune(0xf01da)) // nf-md-download
	glyphSettings = string(rune(0xf0493)) // nf-md-cog
	glyphFolder   = string(rune(0xf024b)) // nf-md-folder — a bookmark folder's row
	glyphTable    = string(rune(0xf04eb)) // nf-md-table — a cell's content popup
	// The test tube is what ui.md §3.2 drew for DevTools (U+F0668).
	glyphDevTools = string(rune(0xf0668)) // nf-md-test_tube

	// Panel [1]: a tab still loading; also panel [2]'s URL row while the
	// page is on its way. The same codepoint as kbu's Logs live glyph
	// (ui.md §2): the family says "live" with one shape.
	glyphLive = string(rune(0xf0753)) // kbu logsLiveGlyph, U+F0753
	// Panel [2]'s URL row, at rest. It wears the structural band — blue,
	// the colour of the URL beside it — because the pair is one thing:
	// where you are (2026-09-22).
	glyphWeb = string(rune(0xf059f)) // nf-md-web

	// Page roles (ux.md §B: one role, one glyph).
	//
	// Every one of these leads its text — glyph, space, name — and nothing
	// is wrapped in brackets any more (user, 2026-09-22). "[ Sign in ]"
	// reads as text that has brackets in it, and worse, "[]" was doing
	// three jobs at once: a button's frame, an image's frame, and a check
	// box's state. One role, one glyph, and the ink says the rest.
	glyphLink        = string(rune(0xf0337)) // nf-md-link
	glyphInput       = string(rune(0xf060e)) // nf-md-form_textbox
	glyphButton      = string(rune(0xf12a8)) // nf-md-gesture_tap_button
	glyphSelect      = string(rune(0xf1400)) // nf-md-form_dropdown
	glyphCheckOn     = string(rune(0xf0135)) // nf-md-checkbox_marked_outline
	glyphCheckOff    = string(rune(0xf0131)) // nf-md-checkbox_blank_outline
	glyphCheckMixed  = string(rune(0xf06f2)) // nf-md-minus_box_outline
	glyphRadioOn     = string(rune(0xf043e)) // nf-md-radiobox_marked
	glyphRadioOff    = string(rune(0xf043d)) // nf-md-radiobox_blank
	glyphImage       = string(rune(0xf02e9)) // nf-md-image
	glyphVideo       = string(rune(0xf0567)) // nf-md-video
	glyphAudio       = string(rune(0xf0387)) // nf-md-music_note
	glyphFrame       = string(rune(0xf0614)) // nf-md-application_outline
	glyphCanvas      = string(rune(0xf01de)) // nf-md-drawing
	glyphUnsupported = string(rune(0xf0ba6)) // nf-md-help_rhombus_outline
)
