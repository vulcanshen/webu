# webu — supported roles

Generated from `internal/ir/roles.go` by `go test ./internal/ir -run TestSupportDoc -update`. Do not edit by hand.

webu guarantees its translation per AX role, not per site (function.md §3). A role listed here is drawn and acted on as described; a role that is not falls back: its name is drawn as text, its children are kept, an unsupported glyph marks it, and Enter still clicks it.

## Drawn

| AX role | IR kind | Enter | Display |
|---|---|---|---|
| `Audio` | media | click | placeholder; menu: yank url |
| `Canvas` | media | click | placeholder; nothing inside can be read |
| `DescriptionList` | list | — | <dl>: terms and definitions |
| `Iframe` | media | click | placeholder; the frame's content is not entered (v1) |
| `LayoutTableCell` | cell | — | a layout-table cell: its content, a space apart from the next |
| `LayoutTableRow` | group | — | a layout-table row: one line of flow (the Hacker News shape) |
| `LineBreak` | text | — | a line break inside text |
| `RootWebArea` | document | — | the page; its name is the title |
| `StaticText` | text | — | a run of text |
| `Video` | media | click | placeholder; menu: yank url |
| `alertdialog` | landmark | — | page chrome: a dialog, a capsule on the bar under the URL; its buttons behind Enter |
| `article` | landmark | — | landmark region, enters the outline |
| `banner` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `blockquote` | quote | — | indented quotation |
| `button` | button | click | [ name ]; Enter presses |
| `cell` | cell | — | one table cell |
| `checkbox` | check | click | [x] name; Enter toggles |
| `code` | code | — | monospace; a block when it spans lines |
| `columnheader` | cell | — | a header cell |
| `combobox` | combobox | choose | name [value]; Enter lists the options |
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
| `image` | media | click | placeholder: glyph, alt or (no alt) |
| `img` | media | click | placeholder: glyph, alt or (no alt) |
| `insertion` | span | — | <ins>: underlined |
| `link` | link | click | glyph + name; Enter opens, menu: open in new tab, yank url |
| `list` | list | — | the container of list items |
| `listitem` | listitem | — | one item, with the marker Chromium drew |
| `main` | landmark | — | landmark region, enters the outline |
| `mark` | span | — | <mark>: reversed, the way a page highlights a hit |
| `navigation` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `option` | option | — | one choice of a combobox |
| `paragraph` | paragraph | — | a block of text, wrapped to width |
| `radio` | check | click | (o) name; Enter selects |
| `region` | landmark | — | landmark region, enters the outline |
| `row` | row | — | one table row |
| `rowheader` | cell | — | a header cell |
| `search` | landmark | — | page chrome: a capsule on the bar under the URL; what it holds is behind Enter |
| `searchbox` | textbox | edit | name ____value____; Enter edits in a popup |
| `sectionfooter` | group | — | an article's or section's footer: its lines, in place |
| `sectionheader` | group | — | an article's or section's header: its lines, in place |
| `separator` | separator | — | a horizontal rule |
| `spinbutton` | textbox | edit | a number field; edits like a textbox |
| `strong` | span | — | <strong> / <b>: bold |
| `switch` | check | click | [x] name; Enter toggles |
| `table` | table | — | columns aligned, widths shrunk to fit |
| `term` | group | — | <dt>: on its own line |
| `textbox` | textbox | edit | name ____value____; Enter edits in a popup |

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
| `subscript` | invisible |
| `superscript` | invisible |
| `time` | invisible |

## Everything else

name as text, children kept, unsupported glyph; Enter still clicks.
