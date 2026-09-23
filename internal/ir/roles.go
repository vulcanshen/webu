package ir

// The support table: every AX role webu understands, in one place. It is the
// single declaration function.md §3 asks for — docs/support.md is generated
// from it (SupportDoc), the translator dispatches on it, and the Space
// menu's "role: slider, not supported yet" line is what happens when a role
// is NOT in it.
//
// A role's entry says what Kind it becomes and how the user acts on it. What
// it LOOKS like is the renderer's business (ui theme), not this table's.

// Action is what Enter does on a node of this role (function.md §4).
type Action uint8

const (
	None   Action = iota // nothing to press — a container or text
	Click                // a mouse click on the node
	Edit                 // click, then webu takes over typing (input popup)
	Choose               // click, then webu lists the options
)

func (a Action) String() string {
	switch a {
	case Click:
		return "click"
	case Edit:
		return "edit"
	case Choose:
		return "choose"
	}
	return "—"
}

// Spec is one row of the table.
type Spec struct {
	Kind   Kind
	Action Action
	// Transparent: the node itself vanishes and its children take its place
	// in the parent. For containers that carry no meaning a terminal can use.
	// A transparent node the page laid out as a block still keeps a Group
	// around inline children (build.go), so the line break survives.
	Transparent bool
	// Skip: the node and everything under it are dropped. Only for nodes
	// whose content is represented elsewhere (an InlineTextBox repeats its
	// StaticText; a ListMarker becomes the ListItem's Marker).
	Skip bool
	// Display is the one-line description docs/support.md prints.
	Display string
}

// Roles is the v1 whitelist. Order is not significant; the doc generator
// sorts. Anything absent falls back to Unsupported (build.go), which is not
// an error — it is the documented behaviour for the long tail.
var Roles = map[string]Spec{
	"RootWebArea": {Kind: Document, Display: "the page; its name is the title"},

	"banner":        {Kind: Landmark, Display: "page chrome: a capsule on the bar under the URL; what it holds is behind Enter"},
	"navigation":    {Kind: Landmark, Display: "page chrome: a capsule on the bar under the URL; what it holds is behind Enter"},
	"main":          {Kind: Landmark, Display: "landmark region, enters the outline"},
	"complementary": {Kind: Landmark, Display: "page chrome: a capsule on the bar under the URL; what it holds is behind Enter"},
	"contentinfo":   {Kind: Landmark, Display: "page chrome: a capsule on the bar under the URL; what it holds is behind Enter"},
	"region":        {Kind: Landmark, Display: "landmark region, enters the outline"},
	"form":          {Kind: Landmark, Display: "landmark region, enters the outline"},
	"search":        {Kind: Landmark, Display: "page chrome: a capsule on the bar under the URL; what it holds is behind Enter"},
	"article":       {Kind: Landmark, Display: "landmark region, enters the outline"},
	"dialog":        {Kind: Landmark, Display: "page chrome: a dialog, a capsule on the bar under the URL; its buttons behind Enter"},
	"alertdialog":   {Kind: Landmark, Display: "page chrome: a dialog, a capsule on the bar under the URL; its buttons behind Enter"},

	"heading":    {Kind: Heading, Action: Click, Display: "title with its level; cursor stops here"},
	"paragraph":  {Kind: Paragraph, Display: "a block of text, wrapped to width"},
	"StaticText": {Kind: Text, Display: "a run of text"},
	"LineBreak":  {Kind: Text, Display: "a line break inside text"},

	"link":          {Kind: Link, Action: Click, Display: "glyph + name; Enter opens, menu: open in new tab, yank url"},
	"button":        {Kind: Button, Action: Click, Display: "tap glyph + name; Enter presses"},
	"textbox":       {Kind: Textbox, Action: Edit, Display: "name ____value____; Enter edits in a popup"},
	"searchbox":     {Kind: Textbox, Action: Edit, Display: "name ____value____; Enter edits in a popup"},
	"spinbutton":    {Kind: Textbox, Action: Edit, Display: "a number field; edits like a textbox"},
	"checkbox":      {Kind: Check, Action: Click, Display: "box glyph, ticked or not, + name; Enter toggles"},
	"radio":         {Kind: Check, Action: Click, Display: "dot glyph, filled or not, + name; Enter selects"},
	"switch":        {Kind: Check, Action: Click, Display: "box glyph, ticked or not, + name; Enter toggles"},
	"combobox":      {Kind: Combobox, Action: Choose, Display: "name + dropdown glyph + value; Enter lists the options — one with no options is a textbox (Google's search box)"},
	"option":        {Kind: Option, Display: "one choice of a combobox"},
	"MenuListPopup": {Transparent: true, Display: "the option list under a <select>; its options are read, it is not drawn"},

	"list":            {Kind: List, Display: "the container of list items"},
	"listitem":        {Kind: ListItem, Display: "one item, with the marker Chromium drew"},
	"ListMarker":      {Skip: true, Display: "the bullet or number; becomes the item's marker"},
	"DescriptionList": {Kind: List, Display: "<dl>: terms and definitions"},
	"term":            {Kind: Group, Display: "<dt>: on its own line"},
	"definition":      {Kind: Quote, Display: "<dd>: indented under its term"},

	"table":        {Kind: Table, Display: "columns aligned, widths shrunk to fit"},
	"grid":         {Kind: Table, Display: "columns aligned, widths shrunk to fit"},
	"row":          {Kind: Row, Display: "one table row"},
	"cell":         {Kind: Cell, Display: "one table cell"},
	"columnheader": {Kind: Cell, Display: "a header cell"},
	"rowheader":    {Kind: Cell, Display: "a header cell"},
	"gridcell":     {Kind: Cell, Display: "one grid cell"},
	"rowgroup":     {Transparent: true, Display: "thead / tbody: invisible"},
	"caption":      {Skip: true, Display: "the table's name already carries it"},

	"LayoutTable":     {Transparent: true, Display: "a table used for layout: invisible"},
	"LayoutTableRow":  {Kind: Group, Display: "a layout-table row: one line of flow (the Hacker News shape)"},
	"LayoutTableCell": {Kind: Cell, Display: "a layout-table cell: its content, a space apart from the next"},

	"image":  {Kind: Media, Action: Click, Display: "glyph + alt, or \"no alt\""},
	"img":    {Kind: Media, Action: Click, Display: "glyph + alt, or \"no alt\""},
	"Video":  {Kind: Media, Action: Click, Display: "placeholder; menu: yank url"},
	"Audio":  {Kind: Media, Action: Click, Display: "placeholder; menu: yank url"},
	"Canvas": {Kind: Media, Action: Click, Display: "placeholder; nothing inside can be read"},
	"Iframe": {Kind: Media, Action: Click, Display: "one row; Enter opens the frame's document in place, Esc comes back out — a frame from another site cannot be entered and says so"},

	"separator": {Kind: Separator, Display: "a horizontal rule"},

	// Inline runs the page marks up: drawn with the terminal's own text
	// attributes, the way a markdown reader draws them (2026-09-22).
	// <del> and <ins> used to fall through to Unsupported, so struck-out
	// text arrived wearing the unsupported glyph.
	"strong":     {Kind: Span, Display: "<strong> / <b>: bold"},
	"emphasis":   {Kind: Span, Display: "<em> / <i>: italic"},
	"deletion":   {Kind: Span, Display: "<del> / <s>: struck through"},
	"insertion":  {Kind: Span, Display: "<ins>: underlined"},
	"mark":       {Kind: Span, Display: "<mark>: reversed, the way a page highlights a hit"},
	"code":       {Kind: Code, Display: "monospace; a block when it spans lines"},
	"blockquote": {Kind: Quote, Display: "indented quotation"},

	"group": {Kind: Group, Display: "a plain container kept for its line break"},

	// A menu that opens from a button is a popup (ui pagepopup); its
	// items are things to press, drawn as buttons are (2026-09-23).
	"menu":             {Kind: Group, Display: "a menu: its items, one per line; one that opens after a press is a popup"},
	"menubar":          {Kind: Group, Display: "a menu bar: its items, in a row"},
	"menuitem":         {Kind: Button, Action: Click, Display: "tap glyph + name; Enter presses"},
	"menuitemcheckbox": {Kind: Button, Action: Click, Display: "tap glyph + name; Enter presses"},
	"menuitemradio":    {Kind: Button, Action: Click, Display: "tap glyph + name; Enter presses"},
	// A header or footer inside an article or section (this Chromium's
	// names for them): the article's own lines, in place, not chrome.
	"sectionheader": {Kind: Group, Display: "an article's or section's header: its lines, in place"},
	"sectionfooter": {Kind: Group, Display: "an article's or section's footer: its lines, in place"},
	"figure":        {Kind: Group, Display: "a figure: its image and caption, on their own lines"},
	"Figcaption":    {Transparent: true, Display: "the caption text flows into the figure"},
	"Legend":        {Transparent: true, Display: "a fieldset's title flows into the group"},

	"generic":       {Transparent: true, Display: "div / span: invisible; a block one keeps its line break"},
	"none":          {Transparent: true, Display: "invisible"},
	"presentation":  {Transparent: true, Display: "invisible"},
	"LabelText":     {Transparent: true, Display: "a <label>: invisible, the input carries the name"},
	"time":          {Transparent: true, Display: "invisible"},
	"superscript":   {Transparent: true, Display: "invisible"},
	"subscript":     {Transparent: true, Display: "invisible"},
	"InlineTextBox": {Skip: true, Display: "a text run's line boxes; the StaticText above it is what is read"},
}

// Fallback is what an unlisted role gets.
var Fallback = Spec{Kind: Unsupported, Action: Click,
	Display: "name as text, children kept, unsupported glyph; Enter still clicks"}

// Lookup is the table with the fallback applied.
func Lookup(role string) Spec {
	if s, ok := Roles[role]; ok {
		return s
	}
	return Fallback
}

// blockDisplays are the CSS display values that put a box on a line of its
// own. Anything else — inline, inline-block, inline-flex, table-cell,
// contents — flows.
var blockDisplays = map[string]bool{
	"block": true, "flex": true, "grid": true, "flow-root": true, "list-item": true,
	"table": true, "table-row": true, "table-row-group": true, "table-header-group": true,
	"table-footer-group": true, "table-caption": true, "-webkit-box": true, "math": true,
}

// IsBlockDisplay reports whether a computed display value starts a new line.
func IsBlockDisplay(display string) bool { return blockDisplays[display] }
