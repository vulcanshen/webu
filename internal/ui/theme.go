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
	// the tab panel [2] is showing (ui.md §4): the ONE thing green says.
	liveColor = lipgloss.Color("#a6e3a1") // green
	// selection mode's frame (ux.md §1).
	selectColor = lipgloss.Color("#f9e2af") // yellow
	// a link in the page. ui.md §4 left this band open ("pick one unused,
	// draw it, then decide"); teal is the anchor nothing else in the family
	// has claimed. Unique to links across the whole app; underlined too, so
	// a link is a link on a terminal with the colours flattened.
	linkColor = lipgloss.Color("#74c7ec") // sapphire (chosen 2026-09-20; was teal)
	// code, inline and block. Not peach: peach is the override band for
	// "worth catching" (console warnings, 4xx rows), and code is not a
	// warning. A block sits on surface0 so it reads as a block.
	codeColor = lipgloss.Color("#f5c2e7") // pink
	codeBg    = lipgloss.Color("#313244") // surface0
	// A data table sits on a ground of its own, its header row on a
	// deeper one: the header is told by ground, not by foreground
	// (revised 2026-09-21; it was mauve text).
	tableBg       = lipgloss.Color("#313244") // surface0
	tableHeaderBg = lipgloss.Color("#45475a") // surface1
	// mauve. It named a table's header cells until those went to a ground
	// of their own (2026-09-21); what wears it now is a key inside a code
	// block, and the pagetab's segments — the page's own chrome, told
	// apart from the page's text without a ground of its own (2026-09-22).
	headerColor = lipgloss.Color("#cba6f7") // mauve
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

	// The header's three popups (ux.md §B: one role, one glyph).
	glyphBookmark = string(rune(0xf00c0)) // nf-md-bookmark
	glyphHistory  = string(rune(0xf02da)) // nf-md-history
	glyphDownload = string(rune(0xf01da)) // nf-md-download
	glyphSettings = string(rune(0xf0493)) // nf-md-cog
	glyphFolder   = string(rune(0xf024b)) // nf-md-folder — a bookmark folder's row
	glyphCrumb    = string(rune(0xf0d20)) // nf-md-map_marker_path — a breadcrumb's entry row
	glyphHeader   = string(rune(0xf06fc)) // nf-md-page_layout_header — a banner's
	glyphFooter   = string(rune(0xf06fb)) // nf-md-page_layout_footer — a contentinfo's
	glyphSidebar  = string(rune(0xf0836)) // nf-md-table_of_contents — a complementary's
	glyphDialog   = string(rune(0xf10ac)) // nf-md-dock_window — a dialog's (a cookie banner, a modal)
	glyphTable    = string(rune(0xf04eb)) // nf-md-table — a cell's content popup
	glyphSkip     = string(rune(0xf04ad)) // nf-md-skip_next — a skip link's row
	// The test tube is what ui.md §3.2 drew for DevTools (U+F0668).
	glyphDevTools = string(rune(0xf0668)) // nf-md-test_tube

	// Panel [1]: a tab still loading; also panel [2]'s URL row while the
	// page is on its way. The same codepoint as kbu's Logs live glyph
	// (ui.md §2): the family says "live" with one shape.
	glyphLive = string(rune(0xf0753)) // kbu logsLiveGlyph, U+F0753
	// Panel [2]'s URL row, at rest.
	glyphWeb = string(rune(0xf059f)) // nf-md-web

	// Page roles (ux.md §B: one role, one glyph).
	glyphLink        = string(rune(0xf0337)) // nf-md-link
	glyphInput       = string(rune(0xf060e)) // nf-md-form_textbox
	glyphImage       = string(rune(0xf02e9)) // nf-md-image
	glyphVideo       = string(rune(0xf0567)) // nf-md-video
	glyphAudio       = string(rune(0xf0387)) // nf-md-music_note
	glyphFrame       = string(rune(0xf0614)) // nf-md-application_outline
	glyphCanvas      = string(rune(0xf01de)) // nf-md-drawing
	glyphUnsupported = string(rune(0xf0ba6)) // nf-md-help_rhombus_outline
)
