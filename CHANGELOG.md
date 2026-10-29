# Changelog

Each release names the version of terminu design (tdp) it follows.

## Unreleased

**webu is a reader, and no longer runs Chromium** ([`docs/reader.md`](docs/reader.md)).
A page is fetched as it is, over plain HTTP, and its JavaScript is not
run; what the page itself presents is shown, and the site around it is
left out: navigation, sidebars, footers, cookie notices. A heading's
words are on its level's colour, a space either side; a list item has a
dash or its number (1., a., i. by depth); code and quotes
are on their own ground, code coloured by its language; a link and an
image have an icon before their words, their address on the frame when
in Focus. A table the page laid itself out with reads as lines, a table
of data as a grid. In the tab list a tab is its page's title, after its
site's mark: the site's own icon where Nerd Fonts has one (GitHub,
Wikipedia, Go…), else a globe, in the colour of the site's favicon, and
blue until that is known. Nothing is downloaded at the first start, and
there is a Linux arm64 build.

- The page's keys are a reader's: `j|k|u|d|gg|G` scroll; `/` finds text,
  every hit underlined, `n|N` step through them, `Esc` puts it away; `f`
  is Focus mode, where `j|k` go from one link, image, field or button to
  the next and `Enter` presses it; `v` is Visual mode, from the top of
  the screen; `H|L` go back and forward along the tab's own history, to
  where you were, and under the address the tab's trail names the pages
  it went through to this one; `o` is the address box; `w` shows the
  full page, and again its content. A link opens at once, without asking
  first.
- Forms work as far as a search box and a plain sign-in: `Enter` fills a
  field (a password never shown, nothing typed going to the App Log),
  ticks a box, chooses an option; `Enter` on the button sends the form,
  its hidden fields with it. A form with one box to type in offers to send
  itself, as Enter in it does on the page. Sign-ins that need JavaScript,
  another site's account, a captcha or a second step do not work.
- `Space` on the page starts with Source, Cookies and Requests, what is
  left of DevTools for a page read without a browser: the page's HTML as
  it came; its site's cookies, `C` clearing them; and what loading the
  page asked for, each redirect, with the headers both ways.
- Cookies are kept in `cookies.json` with the state, a file only you can
  read; `S` › `Clear sign-in data` signs you out everywhere. A file that
  will not read is offered to be moved aside, as webu's other files are.
- A link to a file downloads it into the download folder, its row in
  Downloads, `x` stopping it.
- **Gone with Chromium**: what a page's JavaScript draws, and the pages it
  builds; a page's own dialogs, frames, HTTP sign-in, file uploads and
  certificate questions; DevTools and `I`; the table of contents, the
  sections and the page's parts (`h|l`); the cursor on every item in
  reading order, and with it `go` (go to a line) and the finder; `webu
  browser update`, and the Chromium revision in `webu version`. `L`
  (Location) is now `o`, and `P|N` (back, forward) are now `H|L`.
- **Sign in again**: what the old browser profile kept is not read.
  The Chromium an older webu downloaded into the cache is deleted at the
  first start, the App Log saying where and how big; its profile
  (`~/.webu/datas/profile`) and its log (`~/.webu/datas/webu.log`) are
  data, so they are left for you, named in the App Log at every start
  until you delete them.

webu is rebuilt on [terminu-go](https://github.com/vulcanshen/terminu-go),
the terminu family's shared library, following terminu design (tdp)
v1.1.0: the frames, the menus, the popups and the keys are now the same
code as the rest of the family's.

**Two files move, and webu moves them for you**: `bookmarks.yaml` goes
from `~/.config/webu` to `~/.webu/datas`, with the history (it is data:
deleted, it is gone), and `session.yaml`, the tabs open when webu last
quit, from `~/.webu/datas` to `~/.config/webu` (a new `WEBU__STATE` can put
it elsewhere). The first start moves each one that is in its old place
and not yet in the new one, says from where to where, and takes away an
old folder the move left empty. One that cannot be moved stays and is
still used where it is, and webu names both places so you can move it
yourself; one already in both places is left as it is in the old one,
webu using the new one and naming both for you to merge.

**webu needs a terminal of at least 60×20.** In a smaller window it shows
only its name, the size it needs and that `Ctrl-C` twice quits, no other
key acting, and everything comes back as the window grows. From 60
columns up the tab list and the page are always side by side.

**A file of webu's own that will not read is no longer gone on with.**
At the start webu names it and why, and offers to move it aside to
`.bak` (`.bak.1` and on, never over another) and start as on the first
start; nothing opens before you answer, so nothing is written to it
meanwhile. `Esc` quits instead, the reason on the terminal.

- **`!` opens the App Log**: what webu did and what went wrong since it
  started — what you did (a page reloaded, a link followed, a tab closed,
  a screen switched to; never what you typed into a field), every toast,
  a page that failed to load and why, the tabs reopened from the last
  session. The footer says `!:log`, or how many
  errors you have not seen yet.
- **Control characters from outside are never sent to the terminal**:
  in a page's text, a tab's title, a bookmark or a toast
  each one shows as its name in red (`\t`, `\n`, `\x1b`), nothing dropped
  — a title or link text is no longer cut at a line break. A Tab in a
  code block or a line of source is
  `input_tab_width` spaces.
- **The page follows the tab list's cursor**; `Enter` on the tab list
  takes you to the page.
- `/` in visual mode asks for what to find.
- **Visual mode has a layer**: selecting, its name is `Visual>select` and
  `Esc` drops the selection, back to `Visual`; `Esc` again leaves.
  Once selecting, `v` does nothing: `Esc` is what drops the selection.
  `V` no longer selects whole rows; in Visual mode it opens the splash,
  as everywhere else.
  `Space` in the mode opens its own menu: Select, Copy and Press first.
- Adding a bookmark is a form: its URL and its title, then Add.
- Settings has the family's `hide_secrets_in_preview` and
  `input_tab_width`, and a switch flips in place. `download_dir` is
  picked in the file picker, and a value in `config.yaml` that a setting
  will not take shows the setting's name in red. A number reads with its
  unit: `4 spaces`, `96 cells`.
- `Enter` on a bookmark or a visit opens it in a new tab and stays on the
  list, saying where it went.
- What a Space menu row opens — a question, a box to type in, a viewer —
  keeps the menu under it, and `Esc` goes back to the menu. A row that
  goes somewhere else (a new tab, Location, moving a bookmark) puts the
  menu away as it opens. Deleting a bookmark folder is asked every time,
  empty or not.
- Something done — copied, saved, added, imported, renamed — says so in
  a green Success toast; a toast is as long as its message needs.
- A list's long values — titles, URLs, a line of text — share the room
  the short ones leave and are cut; short values — a status, a size, a
  time — stay whole. Only when the long ones are at their least does a
  column go, the one a row reads best without first (a visit's time).
- Settings' `?` starts with what each setting is, and `input_tab_width`'s
  box writes its unit under the number.
- Every viewer — Inspect, Source, Cookies, Requests — can select and
  copy (`v`).
- A settings, bookmarks, history or session file that will not read is
  shown in an error popup on the first screen, not printed on the
  terminal behind it. Something that failed — a save, the download — is
  an error popup naming what failed.
- Zoomed (`Z`), the panel with the focus takes the whole width, and `Tab`
  shows the other one.
- In visual mode only the selection is Yellow: the search underlines
  every match in its own colour, and the one it is on is the cursor's
  block.
- `Shift-Tab` no longer moves between panels; `PgUp` and `PgDn` scroll
  the page and visual mode like `u` and `d`.
- The powerline symbols' width is measured too, and can be set with
  `WEBU__POWERLINE_WIDTH` or the family's `TERMINU__POWERLINE_WIDTH`.
- The repository takes the terminu family's shape (tdp project): the
  Makefile includes the shared `family.mk`, and the release workflow,
  the goreleaser config, `install.sh`, `uninstall.sh` and `.gitignore`
  are the family's templates. `uninstall.sh` now finds the settings,
  state, data and cache through `WEBU__CONFIG`, `WEBU__STATE`,
  `WEBU__DATA` and `WEBU__CACHE` too, and on macOS asks about a
  Chromium a still older webu left under `XDG_CACHE_HOME`. `install.sh`
  and the Homebrew caveats say a truecolor terminal is needed.
- Releases can come out as a beta first, `vX.Y.Z-beta.N`: a GitHub
  pre-release that never reaches the Homebrew tap, so `brew` and
  `install.sh` keep the stable version. The released binaries' build info
  no longer ends in `+dirty` (`go version -m webu`).
- The repository no longer tracks `CLAUDE.md` and `.claude/`, Claude
  Code's files for its maintainer; its test pages no longer carry a path
  of one machine, and its documents no longer point to notes kept off
  GitHub.
- The READMEs' terminu family section links tdp, terminu-go and
  terminu-harness, instead of listing the other apps.

## [0.5.0] — 2026-10-06

**The environment variables are renamed, and the old names are no
longer read**: `WEBU_CONFIG` is now `WEBU__CONFIG`, `WEBU_DATA` is
`WEBU__DATA`, `WEBU_CACHE` is `WEBU__CACHE`, `WEBU_ICON_WIDTH` is
`WEBU__ICON_WIDTH` (two underscores after the app's name, as across
the family). If you set the old ones, set the new names: webu
otherwise goes back to its defaults without a word.

Frames, popups, hints and keys, brought in line with the [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.23/principle)
(tdp v0.1.23) that kbu, filu, sshu and locku share.

### Changed

- **Inside another terminu family app, webu takes the icon width that app
  passes on** (`TERMINU__ICON_WIDTH`), where its own probe would be
  answered by that app and always read one cell. `WEBU__ICON_WIDTH`
  still comes first.
- **The panel with the focus has a double border**, the other a round
  one, so the focus shows without relying on colour.
- **Reading a section no longer fills the bottom border as a bar**; the
  border still says where you are and how far in (`2/26 · Intro · 40%`).
- **Visual mode names itself** at the top right of the page's frame,
  besides turning it yellow: `╡Visual╞`, set into the border. On a
  narrow panel the title is cut before the name.
- **The page's bottom border goes grey when the focus is elsewhere**: a
  key there is no longer blue while pressing it would go to the tab list.
- **The search shows which side has the keys**: on its list, the query
  row goes grey and the row under the hand turns the box's colour, bold.
- **Every popup is one width**: the terminal less a column on either
  side, at most 120 columns, whatever it holds — menus, questions, text
  boxes, help, the search, DevTools and the toast alike.
- **A popup keeps the height it opens with.** The text box of a
  textarea, the file picker and a request's detail no longer grow or
  shrink as lines are added, a filter narrows the list, or the body
  arrives; what does not fit scrolls inside the box.
- **A refused entry says why inside its box**, on a row kept for it:
  a date or colour in the wrong shape, a bad setting, a folder name
  that is taken or missing. The box stays, with what was typed.
- **Only the popup you are in is bright.** Everything under it — the
  popups below, the page and the header, a download's progress too —
  fades toward the background: every colour stays itself, only darker,
  so the capsules, the cursor bars, code blocks and a popup's border
  colour still show under it. A toast changes nothing. The page under
  one of its own dialogs fades the same way, instead of turning grey.
- **DevTools says when it is waiting**: the loading icon turns after
  its title while Storage or Source is being read, and after a
  request's title while its body is on its way.
- **A select's options and a slider's numbers open over the menu that
  asked for them**, in a box of their own: `Esc` goes back to that menu,
  where it used to close both.
- **Hints and the footer read `key:description`**, one space apart:
  `Space:menu ?:help Tab/1–2:panels q:quit`, `j/k:move Enter:run
  Esc:close`. Keys go by the name on the key cap — `Backspace`, not
  `Bksp`; `h/j/k/l`, not `hjkl`. What `[2]`'s bottom border says about
  where you are (`loading`, `2/26 · Intro · 40%`) is not a key and is
  drawn in the plain hint colour.
- **`?` writes keys the same way**, only without the colon — `j/k`,
  `q/Ctrl-C`, `0–9` — and draws them in the hints' blue.
- **A key named in a sentence is in square brackets**: `Press [T] to open
  one`, `this popup wants an answer: [Esc] does not close it`, and the same
  in menu descriptions and prompts.
- **`?` dims the keys the Space menu dims**: a key that is there but
  cannot be pressed now — close the other tabs with only one open, back
  with no page — is listed and dimmed, where it used to look like any
  other.
- **A row is dimmed whenever it cannot run**, instead of lit and
  answering with a note: `Previous` and `Next` with no page behind or
  ahead, `Search` and `Visual mode` before the page has been read,
  `Open file` on a download still running or cancelled, History's
  `Clear` with nothing recorded, Downloads' `Clear done` with nothing
  finished.
- **`?` on DevTools dims the keys of the other tabs**, and `Enter` on an
  empty list; `?` on a message that fits dims `j/k`.
- **`?` only reads.** It shows the keys of what is in front: on a panel,
  that panel's keys — taken from its Space menu, so the two always agree
  — and the core keys; on a popup, that popup's. Nothing in it runs. The
  global operations are the list the last row of every Space menu opens.
- **The global operations open over the Space menu** instead of in its
  place: `Esc` goes back to the Space menu, and running one closes both.
  The row is called `Global operation`, under a divider with no heading
  of its own. The screen you are on
  is dimmed in it. It lists what acts on the app — the screens and quitting; back,
  forward, location, search and visual mode are the page's, in `[2]`'s
  Space menu.
- **The Space menu is titled with its panel**: `[1] Tabs`, `[2] Page`.
- **In visual mode `Space` opens nothing**, and `?` lists the mode's
  keys, to read; the keys themselves are pressed directly. The sheet
  `Space` used to open, where pressing a listed key ran it, is gone.
- **Adding a bookmark is one box**, URL and title together, and so is
  **signing in**, name and password. `Tab` moves between the fields and
  `Enter` sends them all; an empty URL keeps the box open on that field
  and says why.
- **Offered values**: in a box of one field (the current URL at `L`) `Tab`
  accepts the offer; in a box of several `Tab` moves between fields and
  `→` accepts — `→` works in a single box too. The legend says `accept`,
  and `decline` for `Backspace` refusing it.
- **In the search, `Tab` moves between the query and the hits, and `Esc`
  closes it** from either — it no longer steps back from the hits to the
  query first.

### Fixed

- **A line break or a tab pasted into a one-line box stays in it, drawn
  as a red `\n` or `\t`**, where it used to break the box's row in two:
  the location, a field of the page, a password, a name, the search, a
  filter. Other control characters in a paste are dropped, and so are
  they in what a box opens with, such as a page's `prompt()` default.
  `Enter` does not take a value with one, and says which field can't
  have it, where it used to send the value as it was — Location and the
  console too, which now keep a row for that. A search or a filter only
  finds nothing.
- **Pasting several lines into a textarea's box gives several lines**, as
  typing them does; they used to land on one line. A pasted tab indents
  as `Tab` does.
- Resizing the terminal smaller while a popup is open no longer crashes
  webu: a popup still at the old size is cut at the screen's edge for the
  moment it takes to redraw.
- Frames no longer come out crooked where the terminal moves the cursor
  two cells for an icon: webu measures how far it moves when it starts,
  and `WEBU__ICON_WIDTH` overrides it.
- A hint too long for its box loses whole items from its end: the
  search's list beside its preview used to end in half a key, like `Esc`
  without `:close`.
- `Tab` or `1` while the page is zoomed ends the zoom: the focus used
  to move to the tab list without the tab list coming back on screen.
- Adding a folder with a name that is taken no longer throws away what
  was typed: the box stays open and says so.
- `Esc` and the keys now go to the popup drawn on top: a question that
  came up over the search, or a message opened from a menu of options,
  used to leave `Esc` closing the one underneath it.
- **Quitting with a download in flight no longer replaces the question
  on screen.** Pressing `q` or `Ctrl+C` while another question was open
  — a page's `confirm()`, deleting a folder, clearing history — asked
  about quitting in its place; cancelling the quit then left the first
  question gone, and a page's `confirm()` never got its answer. The quit
  question now opens over it, and `Esc` comes back to it.

## [0.4.0] — 2026-09-27

The keys, brought in line with the [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.0/principle)
(tdp v0.1.0) that kbu, filu, sshu and locku share.

### Changed

- **`q` quits from anywhere but a text box** — over a menu, help, a
  confirm or DevTools, and in visual mode. **`Ctrl+C` is the same way
  out**: it asks first while a download runs, and a second `Ctrl+C` on
  that question leaves at once.
- **`Space` opens and closes the Space menu, and nothing else.** On help,
  a confirm or any other popup it does nothing, so a stray `Space` no
  longer cancels a question.
- **`?` on a panel opens the `?` menu**: everything the app can do from
  anywhere — the screens, back and forward, location, search, visual
  mode, quit — run straight from the list, with the core keys under it.
  **`?` on a popup shows that popup's own keys.**
- **Every Space menu ends in a `global operation` row** that opens the
  `?` menu.
- **A row that cannot run does nothing.** It stays in the menu, dimmed,
  saying what it does; Enter and its key no longer answer with a toast.
- **An empty list's menu has no item rows** — no Open or Delete on a
  History, Downloads or Bookmarks screen with nothing on it.
- **A box or question opened from a menu comes back to the menu** on
  `Esc`; finishing it closes both.
- **Visual mode's footer shows `space` and `?`** like every other.
- **`Tab`, `1` and `2` in visual mode say to leave the mode first**
  instead of doing nothing silently.

### Fixed

- A menu row whose label already carries its key (`[/] Search`,
  `[n] Next section`) no longer shows the key twice.
- The search list's legend no longer offers `Space` to close it.

## [0.3.1] — 2026-09-25

### Changed

- **The icon is redrawn**: the W is now a frame of its own, the E and
  the B standing between its strokes, inside the navy U. The social
  preview draws the new mark.

## [0.3.0] — 2026-09-23

The page, redefined as a document. 0.2.x drew the accessibility tree as
one long page; 0.3.0 reads it — a table of contents, one section at a
time, or the whole sheet; four parts told apart by where they sit; a
finder over all of it; forms as forms; the page's own popups floating;
frames you step into, other sites' included; every role a real page
leans on. The design docs and the README were rewritten for it.

### Added

- **Three screens for a document.** A page with three or more headings
  opens on its table of contents: one row per section, indented by
  depth in one of five hues, with how many tables, code blocks and
  media it holds and how many lines. `Enter` reads one section, whose
  header takes the panel's second row and whose bottom border fills as
  you read; a section holds its sub-sections, the way a chapter holds
  its parts. `n`/`p` step to the next and previous section at the same
  depth, `Esc` is back to the list; `Space` › `One sheet` reads the
  page in one run and `Sections` cuts it again. A column of links
  under a heading is not a section. The Outline popup is gone: the
  list is that screen.
- **Four parts, by geometry.** The biggest top-level block is the body;
  what sits beside it is others; what comes before is the header,
  after it the footer — no tag names consulted. The pagetab under the
  URL is these four, one glyph each: `Esc` goes up to it and the whole
  strip lights, `h`/`l` walk it and the panel switches as you go,
  `Enter` comes back down. A page that fits its window, or has no
  block holding a quarter of it, is one part.
- **A finder.** `/` is a three-column popup — input, hits, preview —
  over every block on the page that carries text, from all four parts,
  literal matches first and fuzzy ones after. `Enter` moves into the
  list and `Enter` there goes there: switches part, opens the section,
  steps into the item, lands the cursor; it presses nothing. `go` and a number jumps to a line.
- **Line numbers on every screen**, counting what is shown; on the
  table of contents they are the sections.
- **A list item is one thing.** An item or article that runs over
  several lines is its first line; `Enter` steps into it, its first
  line becoming the panel's header, `Esc` steps out, as deep as the
  page nests. A one-line item is the link or button it holds.
- **Forms drawn as forms.** A `<form>` is a box drawn into the page:
  labels in one column, values against one edge, labels never cut (the
  row stacks), a fieldset's legend in bold, a required field marked,
  an invalid value in red.
- **Every input, defined.** A one-line box whose border names what it
  takes (`email`, `number`, `phone number`, `password`, `email ·
  invalid`) and whose line names the field; a textarea in an editor
  popup with a writing mode and a moving mode; a slider as a bar with
  its value, `Enter` listing its numbers ten to a window with the
  current one under the cursor; date, time, month, week and colour
  boxes that say their shape on the border and refuse a value not in
  it; a file input answered with the file picker; a search box that
  offers to search; a checkbox or radio that toggles.
- **The page's own popups float.** A modal, an alert dialog, a menu
  opened from a button, a cookie banner — recognised by behaviour: it
  appeared after a press (or on load), it holds something to press,
  and it takes focus, declares itself, or sits over something. It
  floats over the dimmed page in webu's frame, stacks as the page
  stacks them, carries no line numbers, finder or go, and wants an
  answer — `Esc` does not close it. When it goes, the reader is back
  where they were.
- **Frames are a level.** An `<iframe>` is one row; `Enter` steps into
  its document, `Esc` steps out. A frame from another site is another
  process: webu opens a session of its own on that target and reads,
  lays out and clicks inside it; a frame inside a frame is reached the
  same way; a frame that answers nothing stays one row that says so.
- **Roles.** Tabs as a strip like the pagetab, the chosen one lit in
  its section's colour; menus and menu items; trees indented with
  `▾`/`▸`; listboxes as radio or check rows; sliders; progress bars and
  meters as filled bars; `<details>` with its triangle; tooltips as a
  dim aside that appears when `Enter` hovers; timers and status lines
  flowing in place; date, time and colour inputs.
- **The place is kept.** Going back lands where the reader left —
  part, section, scroll, cursor — per history entry.
- **A page still building keeps the spinner turning** until two looks
  agree, and a page that arrives empty is still on its way.
- **Enter hovers before it clicks**, so a menu that opens on
  mouse-over opens.
- **One action at a time** on a page, so a fast walk cannot pull a
  click off its target.
- **webu signs its own user agent**: Chromium's own string with
  `Chrome/` where a headless build says `HeadlessChrome/`, and
  `webu/<version>` on the end; client hints name `Chromium` and `webu`.
- **A PDF says it is not supported**, with its URL a Yank away.
- **A combobox with no options is a text box** — Google's search box
  calls itself one.

### Changed

- **Colour stands for a concept, not an element**: things you press
  are sapphire, things you fill are mauve, code pink, media grey, the
  pagetab rosewater, a wrong value red; headings and the table of
  contents wear five hues by depth. No brackets, no emoji; every
  control leads with its glyph, read from the font's own table.
- **The default search is DuckDuckGo's HTML endpoint.** Google answers
  a headless browser's search with a reCAPTCHA, whatever the user
  agent says.
- **There is no hand-off.** The design note that reserved a window for
  a CAPTCHA is withdrawn: webu has to run where there is no display.
  A CAPTCHA is a placeholder and a URL.
- **`Esc` is one step up**: popup, then item or frame, then the table
  of contents, then the pagetab, then the page. Inside a page popup it
  says the popup wants an answer.
- **A name is printed once.** A region named by its first heading
  draws no rule; a drilled item's first line is its header, not its
  first row again; an empty section is only its title.
- **Elements laid out at a point are dropped** — the sr-only pattern,
  written for screen readers alone.
- **Menus know `u`/`d` and `gg`/`G`.**
- **The design docs and both READMEs were rewritten** to describe this
  version rather than the one before it.

### Fixed

- A form's widest value wrapped its caret to a row of its own.
- A same-process frame read from inside itself captured itself without
  end.
- A click landed off its target one time in six after a fast cursor
  walk.
- A settle re-capture reset the cursor on every mutation of a page
  that kept mutating.

## [0.2.1] — 2026-09-22

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
- **A menu row whose key is a core key names it.** `[Enter] Switch to`,
  `[Enter] Open in new tab`, `[Esc] Page chrome`, `[/] Search`,
  `[/] Filter`. A letter is bracketed where it stands in the label and a
  core key has no letter to bracket, so those rows were leaving their
  key to the hint or to nothing at all — and a panel operation nobody
  can find is not disclosed.

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
