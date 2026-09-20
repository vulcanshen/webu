package ui

import "github.com/charmbracelet/lipgloss"

// Colour anchors (catppuccin-mocha) — ui.md §4 / VTP §B. Assigned once,
// derived everywhere. Each band is reserved: nothing borrows another's, or
// the user has to learn which meaning a colour carries where.
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
	// the tab panel [3] is showing (ui.md §4): the ONE thing green says.
	liveColor = lipgloss.Color("#a6e3a1") // green
	// selection mode's frame (ux.md §1).
	selectColor = lipgloss.Color("#f9e2af") // yellow
	// a link in the page. ui.md §4 left this band open ("pick one unused,
	// draw it, then decide"); teal is the anchor nothing else in the family
	// has claimed. Unique to links across the whole app.
	linkColor = lipgloss.Color("#94e2d5") // teal
	// the cursor row in a list.
	rowSelColor = focusColor
)

const (
	baseHex  = "#1e1e2e" // canvas; also dark text on a bright chip
	crustHex = "#11111b" // recessed background of an inactive capsule
)

// Nerd Font glyphs. Never a PUA literal in source — built from the rune so
// the codepoint stays greppable and the file stays editor-safe (family
// rule). One role, one glyph, one table (ux.md §B).
var (
	capLeft     = string(rune(0xe0b6)) // powerline round-left  — chip start
	capRight    = string(rune(0xe0b4)) // powerline round-right — chip end
	dividerHard = string(rune(0xe0b0)) // pl-left_hard_divider
	dividerSoft = string(rune(0xe0b1)) // pl-left_soft_divider

	// Popup titles — the type signal half of a surface label.
	glyphMenu   = string(rune(0xf0c9))  // nf-fa-bars            — Space menu
	glyphHelp   = string(rune(0xf059))  // nf-fa-question_circle — help
	glyphWarn   = string(rune(0xf071))  // nf-fa-warning         — confirm
	glyphInfo   = string(rune(0xf05a))  // nf-fa-info_circle     — toast
	glyphSearch = string(rune(0xf002))  // nf-fa-search          — goto / search
	glyphPencil = string(rune(0xf040))  // nf-fa-pencil          — input popup
	glyphList   = string(rune(0xf0279)) // nf-md-format_list_bulleted — outline

	// Panel [1] items, as ui.md §1.1 draws them.
	glyphBookmark = string(rune(0xf00c0)) // nf-md-bookmark
	glyphShortcut = string(rune(0xf024b)) // nf-md-folder
	glyphHistory  = string(rune(0xf02da)) // nf-md-history
	glyphDevTools = string(rune(0xf0668)) // nf-md-tools

	// Panel [2]: a tab still loading.
	glyphLive = string(rune(0xf0753)) // nf-md-... the kbu Logs live glyph

	// Page roles (ux.md §B: one role, one glyph).
	glyphLink        = string(rune(0xf0337)) // nf-md-link_variant
	glyphInput       = string(rune(0xf06ff)) // nf-md-form_textbox
	glyphImage       = string(rune(0xf02e9)) // nf-md-image
	glyphVideo       = string(rune(0xf0567)) // nf-md-video
	glyphAudio       = string(rune(0xf0759)) // nf-md-music_note
	glyphFrame       = string(rune(0xf0614)) // nf-md-application
	glyphCanvas      = string(rune(0xf0202)) // nf-md-drawing
	glyphUnsupported = string(rune(0xf0ba2)) // nf-md-help_rhombus_outline
)
