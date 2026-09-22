# Changelog

## [Unreleased]

### Added

- **Yank the page as markdown.** The page's Space menu offers the whole
  page on the clipboard as markdown — headings with their level, lists
  nested and tight, pipe tables, fenced code, quotes, images, and the
  inline markup as `**`, `*`, `~~`, `==`. What markdown has no word for
  — a button, a field, a check box — is written as a bracketed note
  rather than dropped, so a form still reads as a form. The same writer
  gives the IR fixtures a second golden each, one a person can read.

### Changed

- **`measure` defaults to `full`, and takes the word.** The setting that
  caps how wide a paragraph flows now reads `full` — the panel's own
  width, whatever the terminal gives it — as well as a number of cells,
  in `config.yaml` and on the Settings screen alike, and full is what it
  starts as. It was a hundred cells, which left a column of unused panel
  on any terminal wider than that; the person who wants a cap can still
  say so.
- **Two palettes, not one.** webu's own chrome and the page's content now
  have separate colour systems. The app palette — panels, borders, the
  header chain, the pagetab, the cursor, menus — keeps the whole
  protocol: a few anchors, lightness as depth, one reserved band per
  meaning. The page palette — headings, links, code, tables, fields — is
  a closed set of its own that appears only inside the page, takes no
  part in webu's depth (a page is always at one depth), and reserves its
  bands among themselves alone. A web page is a structured document with
  a visual language of its own, and drawing it out of the app's bands
  kept forcing a choice between the two: a table header wanted mauve and
  so did a code key, a highlight wanted lavender and so did the
  selection. Nothing on screen moves for this on its own — the hexes are
  the ones that were already there — but the two can now be changed
  without negotiating with each other.
- **A heading wears a ground that fades by level.** h1 sits on the
  brightest, h6 on the crust, the four between them interpolated the way
  a popup's border is interpolated by depth. The ground runs to the text
  width, so a heading reads as a band rather than as a tinted word, and
  the `#` marks stay: on a terminal with its colours flattened they are
  what still says the level.
- **A page's own markup is drawn with the terminal's.** `<strong>` is
  bold, `<em>` italic, `<del>` struck through, `<ins>` underlined and
  `<mark>` reversed, composed with whatever the run already is — markup
  inside a heading or a table cell keeps both. `<del>` and `<ins>` had
  no entry in the role table at all until now, so struck-out text
  arrived wearing the "unsupported" glyph.

## [0.2.0] — 2026-09-22

### Changed

- **The page's chrome is on the pagetab under the URL, not in the
  page.** Skip links, banner, navigation, breadcrumb, search, sidebar,
  footer and dialog — the entries that were one row each at the top of
  the page — are now segments of a chain on the rule between the URL
  and the page, so the page itself starts at its content and main needs
  no rule of its own. One segment per kind, in a fixed order — `skip`,
  `header`, `nav`, `search`, `sidebar`, `footer`, then each dialog by
  its name — every navigation on the page behind the one `nav`, and,
  when the page has a main, whatever lies outside it in no landmark (a
  promo strip, a cookie banner that is a plain div) behind `other`.
  They are drawn as one powerline chain, the way the header draws its
  screens, a menu glyph at its head and each segment reading `word +N`:
  the word is the kind's, the count what is inside. Where you are in a
  navigation is the panel's hint while the hand is on it. The segments
  the width leaves out sit behind a `+N`, whose Enter lists them and
  puts the one chosen under the hand. `Esc` goes up onto the pagetab
  and back and the page's Space menu carries that row too, `k` from the
  top of the page goes up as well, `h`/`l` walk it and wrap, `j` comes
  back down; Enter on a segment is its list — each
  navigation under its name, the current link marked — and Space is its
  menu, as before. A segment with nothing to open shows its text, and
  the Outline still reaches them all.
- **A column of bare links flows like words.** A page that puts one
  link per line — a sidebar of them, a menu — was costing a row each: a
  hundred and twenty rows of one word before the content began, on a
  documentation site that declares no landmarks at all. A run of two or
  more such links now flows, several to a row, the way a paragraph's
  links do. The row each was CSS, not structure, and webu does not do
  CSS layout. Nothing is hidden and nothing is guessed: a run of links
  that is content flows too, and reads the same.
- **A page opens where its reading starts.** The window scrolls to
  main, not only the cursor; a page that declares no main opens on its
  first heading, which is what the page is about, and everything above
  it stays one `k` away.
- **An anchor scrolls to its target.** Following a link into the page —
  from the pagetab, a table of contents, a skip link — puts the target
  on the first row of the window rather than merely somewhere on it,
  which looked like nothing had happened whenever the target was on
  screen already.
- **The page panel says when it is fetching.** The globe beside the URL
  fills round while a page is on its way and comes back to rest when it
  lands, so a slow site reads as working rather than as stuck. The
  glyph wears the same blue as the URL, the two being one thing — where
  you are — and its frames come from the same Nerd Font set, so the row
  keeps its width and the shape stays square.
- **An article's own header and footer are its lines**, not an
  "unsupported" row: this Chromium names them `sectionheader` and
  `sectionfooter`. A "Jump to content" link is a skip link too.

## [0.1.1] — 2026-09-21

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
- **A dialog is chrome too** — a cookie banner, a modal: one row with
  its name or its first words, its buttons behind `Enter`.
- **A skip link is chrome too.** "Skip to main content" is one row of
  the same style, and a block of them — "Skip to: Top Bar · Sidebar ·
  Main Content" — is one row with its count; `Enter` does what they
  say. A new page never starts on either.
- **A link into the page lands the cursor.** A same-page anchor — a
  skip link's target, a table of contents entry — moves the cursor to
  what it names, no confirm and no reload; the page is asked only when
  the anchor has nothing to stop on.
- **A search box offers to search.** `Enter` in the box writes the
  value and asks; `Enter` again presses Enter in the field, `Esc` keeps
  the value unsent. A search box is one by `type=search`, or any box in
  a search landmark.
- **Menus scroll.** A menu taller than the terminal keeps the cursor's
  row in view; `j`/`k` wrap at the ends, as everywhere in the family.
- **Every cell of a table is a stop.** A cell cut to its column is the
  normal case, so `h`/`l` walk the row and `Enter` shows the cell in
  full, under its column's header, scrolling when long; a cell that is
  one link asks to open it, as the link would. The table sits on a
  ground of its own, its header row on a deeper one — the header is
  told by ground now, not by mauve text.

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
