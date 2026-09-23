# webu — supported roles

Generated from `internal/ir/roles.go` by `go test ./internal/ir -run TestSupportDoc -update`. Do not edit by hand.

webu guarantees its translation per AX role, not per site (function.md §3). A role listed here is drawn and acted on as described; a role that is not falls back: its name is drawn as text, its children are kept, an unsupported glyph marks it, and Enter still clicks it.

## Drawn

| AX role | IR kind | Enter | Display |
|---|---|---|---|
| `Audio` | media | click | placeholder; menu: yank url |
| `Canvas` | media | click | placeholder; nothing inside can be read |
| `ColorWell` | textbox | edit | a colour box; Enter asks for #rrggbb |
| `Date` | textbox | edit | a date box; Enter asks for YYYY-MM-DD |
| `DateTime` | textbox | edit | a date-and-time, month or week box; Enter asks for it in the browser's shape |
| `DescriptionList` | list | — | <dl>: terms and definitions |
| `DisclosureTriangle` | button | click | a summary with its triangle; Enter opens or shuts what follows |
| `Iframe` | media | click | one row; Enter opens the frame's document in place — another site's through a session of its own — and Esc comes back out |
| `InputTime` | textbox | edit | a time box; Enter asks for HH:MM |
| `LayoutTableCell` | cell | — | a layout-table cell: its content, a space apart from the next |
| `LayoutTableRow` | group | — | a layout-table row: one line of flow (the Hacker News shape) |
| `LineBreak` | text | — | a line break inside text |
| `RootWebArea` | document | — | the page; its name is the title |
| `StaticText` | text | — | a run of text |
| `Video` | media | click | placeholder; menu: yank url |
| `alert` | group | — | an alert, drawn in place as it appears |
| `alertdialog` | landmark | — | page chrome: a dialog, a capsule on the bar under the URL; its buttons behind Enter |
| `article` | landmark | — | landmark region, enters the outline |
| `banner` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `blockquote` | quote | — | indented quotation |
| `button` | button | click | tap glyph + name; Enter presses |
| `cell` | cell | — | one table cell |
| `checkbox` | check | click | box glyph, ticked or not, + name; Enter toggles |
| `code` | code | — | monospace; a block when it spans lines |
| `columnheader` | cell | — | a header cell |
| `combobox` | combobox | choose | name + dropdown glyph + value; Enter lists the options — one with no options is a textbox (Google's search box) |
| `complementary` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `contentinfo` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `definition` | quote | — | <dd>: indented under its term |
| `deletion` | span | — | <del> / <s>: struck through |
| `dialog` | landmark | — | page chrome: a dialog, a capsule on the bar under the URL; its buttons behind Enter |
| `emphasis` | span | — | <em> / <i>: italic |
| `figure` | group | — | a figure: its image and caption, on their own lines |
| `form` | landmark | — | landmark region, enters the outline |
| `grid` | table | — | columns aligned, widths shrunk to fit |
| `gridcell` | cell | — | one grid cell |
| `group` | group | — | a plain container kept for its line break |
| `heading` | heading | click | title with its level; cursor stops here |
| `image` | media | click | glyph + alt, or "no alt" |
| `img` | media | click | glyph + alt, or "no alt" |
| `insertion` | span | — | <ins>: underlined |
| `link` | link | click | glyph + name; Enter opens, menu: open in new tab, yank url |
| `list` | list | — | the container of list items |
| `listbox` | group | — | a list of options to pick from, one per row (2026-09-23) |
| `listitem` | listitem | — | one item, with the marker Chromium drew |
| `log` | group | — | a log, drawn in place as it grows |
| `main` | landmark | — | landmark region, enters the outline |
| `mark` | span | — | <mark>: reversed, the way a page highlights a hit |
| `menu` | group | — | a menu: its items, one per line; one that opens after a press is a popup |
| `menubar` | group | — | a menu bar: its items, in a row |
| `menuitem` | button | click | tap glyph + name; Enter presses |
| `menuitemcheckbox` | button | click | tap glyph + name; Enter presses |
| `menuitemradio` | button | click | tap glyph + name; Enter presses |
| `meter` | gauge | — | a filled bar with its value; reads only |
| `navigation` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `option` | option | click | one choice of a combobox, listed behind it; one of a listbox is a row of its own, a radio or check glyph + name, Enter picks |
| `paragraph` | paragraph | — | a block of text, wrapped to width |
| `progressbar` | gauge | — | a filled bar with its value; reads only |
| `radio` | check | click | dot glyph, filled or not, + name; Enter selects |
| `region` | landmark | — | landmark region, enters the outline |
| `row` | row | — | one table row |
| `rowheader` | cell | — | a header cell |
| `search` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `searchbox` | textbox | edit | name ____value____; Enter edits in a popup |
| `sectionfooter` | group | — | an article's or section's footer: its lines, in place |
| `sectionheader` | group | — | an article's or section's header: its lines, in place |
| `separator` | separator | — | a horizontal rule |
| `slider` | textbox | edit | a bar with its value; Enter lists its numbers |
| `spinbutton` | textbox | edit | a number field; edits like a textbox |
| `strong` | span | — | <strong> / <b>: bold |
| `switch` | check | click | box glyph, ticked or not, + name; Enter toggles |
| `tab` | button | click | one tab of the strip; Enter chooses it |
| `table` | table | — | columns aligned, widths shrunk to fit |
| `tablist` | group | — | a strip of tabs, the chosen one lit |
| `tabpanel` | group | — | a tab's content, in place |
| `term` | group | — | <dt>: on its own line |
| `textbox` | textbox | edit | name ____value____; Enter edits in a popup |
| `tooltip` | group | — | a tooltip, drawn in place while the page shows it |
| `tree` | group | — | a tree: its items, indented by level |
| `treeitem` | group | click | indent + ▸/▾ for a branch + name; Enter clicks — opens a branch, picks a leaf |

## Invisible

These roles carry nothing a terminal can use; the node vanishes and its children flow into the parent. A container the page laid out as a block still keeps its line break.

| AX role | Why |
|---|---|
| `Figcaption` | the caption text flows into the figure |
| `InlineTextBox` | a text run's line boxes; the StaticText above it is what is read |
| `LabelText` | a <label>: invisible, the input carries the name |
| `LayoutTable` | a table used for layout: invisible |
| `Legend` | a fieldset's title flows into the group |
| `ListMarker` | the bullet or number; becomes the item's marker |
| `MenuListPopup` | the option list under a <select>; its options are read, it is not drawn |
| `caption` | the table's name already carries it |
| `generic` | div / span: invisible; a block one keeps its line break |
| `none` | invisible |
| `presentation` | invisible |
| `rowgroup` | thead / tbody: invisible |
| `status` | a status line (<output>, role=status): its text flows where it is, as it changes |
| `subscript` | invisible |
| `superscript` | invisible |
| `time` | invisible |
| `timer` | a timer: its text flows where it is, as it counts |

## Everything else

name as text, children kept, unsupported glyph; Enter still clicks.
