# Changelog

## [Unreleased]

### Added

- **Import bookmarks.** `I` on the Bookmarks screen opens a file picker
  — one directory at a time, type to narrow, Enter steps into a folder
  or takes a file, Backspace steps out — for the HTML every browser
  exports (Chrome, Firefox, Safari, Edge: the Netscape bookmark format).
  The file is read at once, a folder name is asked for and required, and
  the whole tree lands under it, empty folders included.
- **Rename.** `r` on a bookmark edits its title in a box holding the
  current one; on a folder, its name, and everything under it follows.

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
- **`x` on a bookmark folder takes the whole tree**, after asking — how
  many bookmarks and folders go with it. An empty folder still goes at
  once. It used to refuse a folder with anything in it, which left an
  import with no way back but row by row.
- **The page's chrome is one row each.** A banner, a navigation, a
  breadcrumb, a search, a sidebar, a footer: `▎ 󰍜 Docs +12` — a bar, a
  glyph for the kind, a word for where you are (the tab you are on by
  `aria-current` or by URL, a breadcrumb's last crumb, the site for a
  banner, the box for a search) and how much is behind it — none of it
  on the page, where a row of links invites walking sideways and costs
  a screen before the page begins. `Enter` lists what it holds and
  `Enter` again opens one — a link, a button, a field to type into, a
  select; a search with one box opens the box outright. The Space menu
  lists the same rows under item operation. main, article, region and
  form are the page itself and stay as they were.

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
