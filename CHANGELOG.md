# Changelog

## [Unreleased]

### Changed

- **Enter is a click; Space is the menu.** Enter on an item does what a
  left click would, in terminal terms: a text box opens to type, a select
  drops its list, a button or check box is pressed, a landmark or heading
  collapses / expands. A link asks first — its text and URL in a confirm
  — and opens on Enter again. An item with no Enter action yet says so.
  The item's other operations — Open in new tab, Submit, Clear, Yank —
  are in the Space menu, no longer behind Enter.
- **`[1] Tabs` closes on `c`, not `w`.** A terminal has no window for a
  `w` to close, and the page panel's close is `C` already. The clone row
  becomes `[o] Open in new tab`, the same `o` as a download's source.

### Fixed

- **An empty password box is known as one**, and its input popup masks
  the typing from the first keystroke. It was told apart by the dots in
  its value, which an empty one has none of; now the DOM snapshot says
  which inputs are `type=password`.

## [0.1.0] — 2026-09-21

The first release. webu is a terminal browser: a pinned Chromium runs
headless in the background, webu takes its accessibility tree, turns it
into a layout of its own and draws that as a TUI — a screen reader's
output as a page rather than as speech. Every action goes back through the
Chrome DevTools Protocol, so the page is the real page: it logs in, it
runs its JavaScript, it keeps its cookies.

### Added

- **A pinned Chromium, fetched once.** The revision is compiled into the
  binary and never overridden; the first launch downloads it into the
  cache directory with a progress line, and `webu browser update` fetches
  a new pin. Nothing of the user's own Chrome is touched.
- **The page, as items and flow.** Links, buttons, text boxes, check
  boxes, selects, media, headings and landmarks are items the cursor stops
  on; paragraphs flow between them at a measured width. `j`/`k` step by
  row, `h`/`l` along one, `u`/`d` by half a page. Landmarks and headings
  collapse on Enter. Navigation lists flow on one line. JSON, YAML, TOML,
  Markdown and other non-HTML responses render as a code block with syntax
  colour.
- **Enter opens what an item can do; Space opens the whole menu**, item
  and panel operations both, with every hotkey printed in its bracket.
- **Two panels and a header of screens.** `[1] Tabs` beside `[2] Page`
  under a chip row: `[W]eb`, `[B]ookmarks`, `[H]istory`, `[D]ownloads`,
  `[S]ettings`, each a screen of its own. Bookmarks nest in folders;
  history is kept for good; downloads report progress on the header's
  rule; settings edit `config.yaml` in place.
- **Location on `L`**, as Chrome's Cmd+L, with the page's URL on offer;
  words that are not a URL go to the configured search engine.
- **Visual mode on `v`**: walk the text by character with vim's motions,
  select, yank to the system clipboard; `/` searches the page.
- **DevTools on `I`**: Network with request details, Storage with cookie
  and storage editing, a Console that lists every entry whole and evaluates
  what you type, and the page's Source.
- **What a page asks for, answered in place**: alert / confirm / prompt,
  HTTP auth, file uploads, `target=_blank`, certificate errors, downloads.
- **Session restore** on the next launch, tabs unloaded until switched to.
- **The u-family easter egg** — the icon, revealed pixel by pixel, on `V`, the family's key.
