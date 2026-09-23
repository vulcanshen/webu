# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**Language**: English · [繁體中文](README-zh_TW.md)

**A terminal browser that reads a page as a document.** A real Chromium runs headless in the background; webu takes what it maintains for screen readers — the accessibility tree — and the layout of the same DOM, and turns the two into a document you can move through: a table of contents, one section at a time, or the whole sheet; the page's four parts (header, body, others, footer) told apart by where they sit; a finder that reaches anything on the page in a few keystrokes; forms drawn as forms, the page's own popups floating over it, frames you can step into. Every action goes back through the Chrome DevTools Protocol, so the page is the real page: it logs in, it runs its JavaScript, it keeps its cookies.

> _When in doubt, hit_ **`Space`**.

webu is a member of the `u`-family and a browser-domain implementation of [this TUI Design Principle](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) — the same design system as [kbu](https://github.com/vulcanshen/kbu) (Kubernetes), [filu](https://github.com/vulcanshen/filu) (filesystem) and [sshu](https://github.com/vulcanshen/sshu) (ssh). The design — including what was tried and rejected, dated in place — is in [`docs/function.md`](docs/function.md), [`docs/ui.md`](docs/ui.md) and [`docs/ux.md`](docs/ux.md); how it was actually built, and what was measured on the way, in [`docs/webu-implementation.md`](docs/webu-implementation.md).

## Demo

![demo](docs/demo.gif)

A Wikipedia article opens as its table of contents — one row per section, indented by depth, each with what it holds and how long it is; `Enter` reads one, `n` the next, `Esc` is back to the list. `Esc` again is the pagetab: the page's four parts, `l` walking them and the panel switching as it goes. `/` is the finder — type a word, the hits list with a preview, `Enter` into the list and `Enter` again goes there. `L` is the Location box; a search lands on a results page, itself a document whose sections are the results. `?` is the help.

## What changed in 0.3.0

0.2.x drew the accessibility tree as one long page. 0.3.0 reads it as a **document**:

- **Three screens for a document.** A page with headings opens on its **table of contents**: sections by depth, with their tables, code blocks, media and line counts. `Enter` reads **one section** — and a section holds its sub-sections, the way a chapter holds its parts — `n`/`p` step between siblings, `Esc` is back to the list. `Space` › `One sheet` reads the page in one run, and `Sections` cuts it again. A page without enough headings is one sheet from the start.
- **Four parts, by geometry.** The biggest block of the page is its **body**; what sits beside it is **others** (sidebars, ad columns); what comes before is the **header**, after it the **footer**. No tag names are consulted — `nav` or `div`, position decides. The four are a chain under the URL, one glyph each; `Esc` goes up to it, `h`/`l` walk it and the panel switches as you go, `Enter` comes back down. A page that fits the window, or has no dominant block, is one part and the chain is a plain rule.
- **A finder, not a search.** `/` opens a three-column finder over the whole page — every block that carries text, from all four parts — with the hits listed by part and a preview of each. `Enter` moves into the list, and `Enter` there **goes there**: switches part, opens the section, steps into the item, lands the cursor. It presses nothing. `go` + a number jumps to a line; every screen has line numbers, and on the table of contents the numbers are the sections.
- **A thing is one row.** A list item or article that runs over several lines is drawn as its first line; `Enter` steps into it (its first line becomes the panel's header), `Esc` steps out, as deep as the page nests. A one-line item is just its link or button.
- **Forms are forms.** A `<form>` is a box drawn into the page: labels aligned in one column, values against one edge, labels never cut (the row stacks instead), a fieldset's legend in bold, an invalid value in red, a required field marked. Every kind of input has a defined interaction: a one-line box whose border says what it takes (`email`, `number`, `date · YYYY-MM-DD`, `password`); a **textarea** in an editor popup with a writing mode and a moving mode; a **slider** as a bar with its value, `Enter` listing its numbers ten at a time; a **date**, **time** or **colour** box that refuses a value not in the browser's shape rather than let the browser drop it; a **file** input answered with a file picker; a search box that offers to search on `Enter`.
- **The page's own popups float.** A modal, an alert dialog, a menu that opens from a button, a cookie banner — found by behaviour (it appeared after a press, it takes focus or declares itself or sits over something, and it has something to press), never by tag. It floats over the dimmed page in webu's own popup frame, stacks when the page stacks them, and wants an answer: `Esc` does not close it. When it goes, you are back where you were.
- **Frames are a level.** An `<iframe>` is one row; `Enter` steps into its document, `Esc` steps out. A frame from **another site** is another process and another target — webu opens a session of its own on it, and reads and clicks inside it like anywhere else; a frame inside a frame is reached the same way.
- **Every role a real page leans on**: tabs (drawn as a strip like the pagetab, the chosen one lit in the section's colour), menus, trees (indented, `▾`/`▸`), listboxes (radio or check rows), sliders, progress bars and meters (a filled bar, read-only), `<details>` (a triangle that opens and shuts what follows), tooltips (an aside that appears when `Enter` hovers), timers and status lines (text that flows and changes). What webu does not know is still drawn, marked, and clickable.
- **Colour is a concept, not an element.** Things you press are sapphire, things you fill are mauve, code is pink, media grey, the pagetab rosewater, a wrong value red; headings and the table of contents wear five hues by depth. No brackets, no emoji; every control leads with its glyph.
- **The page keeps its place.** Going back lands where you left — part, section, scroll, cursor — per history entry. A page that is still building (a SPA filling in) keeps the spinner turning until two looks agree, instead of landing you on a half-built page. Enter hovers before it clicks, so a menu that opens on mouse-over opens. One action at a time, so a fast walk cannot pull a click off its target.
- **webu signs its own user agent** — Chromium's own string with `Chrome/` where a headless build says `HeadlessChrome/`, and `webu/<version>` on the end. Not a disguise: a real browser's name. The default search is DuckDuckGo's HTML endpoint, because Google answers every headless search with a reCAPTCHA. A CAPTCHA is a wall webu says so about — there is no hand-off to a window, because webu must run where there is no window.

## Five keys to drive webu

| Key | Behavior |
|---|---|
| **`Tab`** | Move focus between the two panels of the web screen: `[1] Tabs` and `[2] Page` |
| **`Enter`** | The item's most intuitive operation — a click, as a mouse would, after a hover: a link asks first, a button is pressed, a box opens to type, a select drops its list; and where a mouse has no equivalent, **go in**: open a section from the table of contents, step into a list item, a frame |
| **`Space`** | *What can I do here?* — the contextual menu for whatever has focus: `item operation` and `panel operation`. Also closes any of webu's popups |
| **`Esc`** | **One step up**: close the top popup → out of the item or frame → back to the table of contents → up to the pagetab → back down. The page's own popup is not closed by it: it wants an answer |
| **`?`** | Global help — the whole key vocabulary in one list |

The header's screens are switched with a single shifted letter — **`W` / `B` / `H` / `D` / `S`** — and `1` / `2` address the two panels. Every letter hotkey is also a row in the `Space` menu, with the key printed in its bracket exactly as you press it, so there is nothing to memorize unless you want to.

## The header and the two panels

```
 [W]eb ╱ [B]ookmarks ╱ [H]istory ╱ [D]ownloads ╱ [S]ettings
```

**`[W]eb`** — `[1] Tabs` beside `[2] Page`. The tabs list is one row per Chromium target; the cursor says where you are, green says which one the page panel is showing. The page panel is always the page: its first row is the URL (the globe beside it turns while the page is on its way), its second row the **pagetab** — the page's four parts — and under them one of three screens: the **table of contents**, **one section**, or the **whole sheet**. Line numbers run down the left of every one of them. The page itself is **items** — links, buttons, boxes, checks, selects, headings, a table's every cell, a list's every thing, a frame — with prose flowing between them at a measured width. `j`/`k` step by row, `h`/`l` along one, `u`/`d` half a page. Headings collapse on `Enter`. The panel's bottom border says where you are: `2/26 · Try it · 40%`, and fills as you read.

**`[B]ookmarks`** — a tree: folders as rows, `Enter` opening a bookmark in a new tab or collapsing a folder; `a` adds a bookmark here (the current page on offer), `A` a folder (`a/b/c` makes every level), `m` moves, `r` renames, `I` imports a browser's bookmarks export through a file picker.

**`[H]istory`** — every page visited, newest first, kept for good; `C` is the one way it shrinks. **`[D]ownloads`** — this session's downloads with their progress, and the rule under the header doubles as the progress bar while one runs. **`[S]ettings`** — `config.yaml` edited in place, each row a key with its value and what it does.

## Install

> webu is **macOS / Linux only** (amd64 and arm64 on macOS, amd64 on Linux — the Chromium snapshot bucket has no Linux ARM build). No native Windows build.

**Homebrew** (macOS / Linux):

```bash
brew install vulcanshen/tap/webu
```

**Install script** (drops the latest release binary into `~/.local/bin`, or `/usr/local/bin` as root):

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/install.sh | sh
```

**From source**:

```bash
go install github.com/vulcanshen/webu/cmd/webu@latest
```

or clone and build:

```bash
git clone https://github.com/vulcanshen/webu.git
cd webu
make build     # → ./webu   (CGO_ENABLED=0, -trimpath, stripped)
./webu
```

A `Makefile` wraps the common tasks — `make build`, `make install` (→ `$GOBIN`) / `make uninstall`, `make test`, `make fixtures` (re-capture the role fixtures against the pinned Chromium), `make gif` (re-record the demo), `make snapshot` (a goreleaser dry run into `dist/`). Run `make` to list them.

**Chromium comes on the first launch, not in the box.** The release is the Go binary alone. webu runs one pinned Chromium revision — compiled into the binary, never overridden, never your own Chrome — and the first launch downloads it once into the cache directory (macOS `~/Library/Caches/webu`, Linux `~/.cache/webu`; about 175–250 MB), saying so with a progress line. After an upgrade that pins a new revision, `webu browser update` fetches it. `webu version` prints both versions.

**A Nerd Font is required**, not optional: links, fields, media, the parts, the panels and the header are drawn with Nerd Font glyphs, and the layout measures them.

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

Removes the binary, then asks — never assumes — about each of the three directories below: settings, data, and the downloaded Chromium.

## Quick start

```bash
webu                              # the last session's tabs, or an empty page
webu https://news.ycombinator.com # straight to a page
webu go.dev lobste.rs             # one tab each, the first in front; the scheme is filled in
webu "terminal browser"           # words that are not a URL are searched
webu help                         # the whole command line; webu version prints the versions
```

On a documentation page: `j`/`k` down the table of contents, `Enter` to read a section, `n` for the next, `Esc` back to the list. On any page: `/`, a word, `Enter`, `Enter` — you are on it. `Space` on any panel lists exactly what that panel can do.

## Where your data lives

Three directories, by what is in them:

| | What | Where |
|---|---|---|
| settings | `config.yaml`, `bookmarks.yaml` — what you write | `~/.config/webu` (`$XDG_CONFIG_HOME/webu` when set; `$WEBU_CONFIG` names it outright) |
| data | `history.yaml`, `session.yaml`, `downloads/`, the Chromium `profile/` (cookies, logins), `webu.log` — what webu produces | `~/.webu/datas` (`$WEBU_DATA`) |
| cache | the pinned Chromium, re-downloadable | macOS `~/Library/Caches/webu`, Linux `~/.cache/webu` (`$XDG_CACHE_HOME/webu`, `$WEBU_CACHE`) |

Settings and bookmarks are hand-editable YAML; bookmarks carry a `folder` path each, plus a `folders:` list so an empty folder survives. The history is a YAML sequence appended one visit at a time. Every write is atomic.

### Settings — `config.yaml`

```yaml
# Where a search goes when what you typed at L is not a URL: the words are
# appended to it. Default DuckDuckGo's HTML endpoint — Google answers a
# headless browser's search with a reCAPTCHA. Brave (search.brave.com/search?q=)
# works too.
search_engine: https://html.duckduckgo.com/html/?q=
# Where downloads land. Default ~/.webu/datas/downloads.
download_dir: ~/Downloads
# How wide a paragraph flows before it wraps, in cells. Default full.
measure: full        # or a number of cells, 20 or more
# Reopen the tabs that were open when webu last quit. Default true;
# false starts empty, or on the URLs given on the command line.
restore_session: true
```

The `[S]ettings` screen edits the same file, and every key the file knows is a row there — a test holds that.

## Key bindings

Every letter hotkey below is also a row in that surface's `Space` menu. The bracket shows the key **exactly as you press it**: `[A]dd folder` is shift+A, `[a]dd` is a bare `a`, and nothing fires that the marking does not name.

### Everywhere

```
 screens   W / B / H / D / S           Esc on a screen goes back to the web
 panels    1 / 2 of the web  ·  Tab
 cursor    j k    u d (half page)      gg G      h l along a row
 page      P / N previous / next       L location    / finder    v visual mode
 global    Space menu    ? help    q quit    Ctrl+C force quit
```

### `[1]` Tabs — lower case is the row, upper case is the panel

`Enter` switch to the tab · `c` close · `o` open in new tab · `r` reload · `y` yank url · `T` new tab · `X` close others · `U` undo close

### `[2]` Page

`Enter` on an item is a click, as a mouse would — a hover, then the press: a box opens to type (a password box masked, the border naming what the box takes), a select drops its list, a slider lists its numbers, a button or check box is pressed, a heading collapses. A link asks first — its text and URL in a confirm — and opens on `Enter` again; a link into the same page just jumps. Where a mouse has no equivalent, `Enter` goes in: a section from the table of contents, a list item that runs over several lines, a frame. `Esc` is one step back up.

`Space` is the right-click menu: a link's Open / Open in new tab / Yank link url, a text box's Submit / Edit / Clear / Yank, a select's Choose, plus Yank text and Inspect on every item. Panel operations: `R` reload · `T` new tab · `P` / `N` back / forward · `/` finder · `go` go to line · `n` / `p` next / previous section · `Sections` / `One sheet` · `Esc` page parts · `v` visual mode · `L` location · `A` add bookmark · `I` inspect (DevTools) · `Z` zoom · `Y` yank page url · `Yank markdown` · `C` close this tab.

The finder (`/`) lists every block on the page that carries text, from all four parts, filtered as you type, with a preview; `Enter` into the list, `Enter` again goes there. The Location box (`L`) opens with the page's own URL on offer: `Tab` takes it to edit, `Backspace` clears it, and words that are not a URL go to the search engine.

### The screens

- **Bookmarks** — `Enter` open in a new tab, or collapse / expand a folder · `a` add a bookmark here · `m` move · `r` rename · `x` delete (a folder with anything in it asks first) · `y` yank url · `A` add a folder here (`a/b/c` makes each level) · `I` import a browser's export · `/` filter
- **History** — `Enter` open in a new tab · `x` delete · `y` yank url · `C` clear · `/` filter
- **Downloads** — `Enter` open the file · `o` source in a new tab · `x` remove (a running download is stopped) · `y` yank path · `C` clear the finished ones · `/` filter
- **Settings** — `Enter` edit a text setting (the value in force is on offer: `Tab` takes it, `Backspace` clears it, an emptied line means the default) or flip a switch

### DevTools (`I`)

`h` / `l` switch between **Network** (`Enter` a request's headers and body, `C` clear, `/` filter), **Storage** (cookies, local and session storage: `x` delete, `y` yank the value, `C` clear site data, `/` filter), **Console** (every entry whole, wrapped; `Enter` an entry's detail; `i` the prompt, a REPL that evaluates in the page; `C` clear, `/` filter) and **Source** (the page's HTML, `/` grep). `Esc` closes.

### Visual mode (`v`)

The page holds still and the frame turns yellow. `h j k l` move by character, `w` / `e` / `b` by word, `0` / `$` to either end of the line, `u` / `d` half a page, `gg` / `G` to the ends; `v` / `V` start selecting by character or by line, `y` copies to the system clipboard (`pbcopy`, `wl-copy`, `xclip` or `xsel`), `/` searches the text with `n` / `N`, `Enter` acts on the item under the cursor, `Esc` leaves.

## Features

- **A real browser behind the text** — one pinned Chromium, headless, with webu's own persistent profile: logins survive a restart, JavaScript runs, cookies are kept, and nothing of your own Chrome is touched. Every page problem is Chromium's to solve; webu only draws the answer.
- **Semantics from the accessibility tree, layout from the DOM** — what a screen reader would read is what you see: roles, names, states; where things sit on the page comes from the same DOM's layout snapshot, joined by node id. No CSS is read for colour or font. A role webu does not know is drawn as its text with a marker, never hidden, and still clickable. Every supported role has a fixture captured against the pinned revision, so an engine bump is a decision rather than a drift.
- **A page read as a document** — table of contents, one section, one sheet; four parts by geometry; a finder over all of it; line numbers; the place kept per history entry.
- **Items and flow** — links, buttons, fields, headings, a table's cells, a list's things and frames are stops for the cursor; prose flows between them at a measured width; tables keep their columns with every cell a stop and its full text behind `Enter`; code keeps its lines and its syntax colour and opens whole on `Enter`.
- **Every input, defined** — one-line boxes that say what they take, a textarea editor with two modes, sliders as bars that list their numbers, dates and colours in the browser's shape, checks that toggle, selects that list, files through a picker, search boxes that offer to search.
- **The page's own popups, floating** — modals, alert dialogs, menus and banners found by behaviour, stacked as the page stacks them, answered rather than dismissed.
- **Frames, including other sites'** — one row, `Enter` to step in, a session of webu's own on a cross-site frame, nested frames reached the same way.
- **What a page asks, answered in place** — `alert` / `confirm` / `prompt` and `beforeunload`, HTTP basic and digest auth, file uploads, `target=_blank` as a new tab that is switched to, a certificate error as a question, all as popups in webu's own shape.
- **Two menus, one table** — `Enter` is the item's operation, `Space` is item and panel together, and the letter in every bracket is generated from the same table the key handler reads, so a hotkey that is not in the menu cannot exist.
- **A header of screens** — Web, Bookmarks (in folders, importable), History, Downloads, Settings on one chip row, each list screen a single panel with its keys in the bottom border.
- **DevTools in the terminal** — Network with request details and bodies, Storage editable, a Console that prints objects the way Chrome's does and evaluates what you type, the page's source with grep.
- **Non-HTML answered as text** — JSON, YAML, TOML, Markdown, XML and plain responses become one syntax-coloured code block; a PDF says plainly that it is not supported.
- **Markdown out** — `Space` › `Yank markdown` puts the whole page on the clipboard as markdown.
- **Its own name** — a user agent that says `webu/<version>`, not a disguise.
- **Frame stability** — every rendered line is exactly the terminal width at every size, with any content; a test checks it across sizes, panels and screens.
- **unix-first, static binary** — macOS + Linux; `CGO_ENABLED=0`. The chromedp log goes to a file, never to the terminal the TUI is drawing on.

## Status

**v0.3.0** — the page redefined as a document; the role table complete; every input, popup and frame handled. See [CHANGELOG.md](CHANGELOG.md).

Where the wall is, said plainly: a web page is two-dimensional and a terminal is not, so a dense app page (an issue tracker's board, a dashboard) is classified correctly but still has to be moved through — the finder and the table of contents are the way, not scrolling. A site that declares no semantics at all (everything a `div`, no ARIA) gives webu text and clickable things and nothing more; that site breaks screen readers too, and webu does not chase it.

Not there yet:
- **media** — images, video and audio are placeholders; yank the URL and open it elsewhere
- **CAPTCHA, passkeys / WebAuthn, WebRTC** — said plainly when met, with the URL a Yank away. There is no second way: webu is headless and has to run where there is no display, so there is no window to hand off to
- a `<textarea>` in your own `$EDITOR` (the built-in editor popup is what there is); a slider with a fractional step lists whole numbers
- the cursor inside a frame scrolls the frame, not the page around it
- mouse support, a Linux ARM build (no Chromium snapshot for it)

## Built with

Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss), [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay) for the floats, [chromedp](https://github.com/chromedp/chromedp) for the Chrome DevTools Protocol, and [chroma](https://github.com/alecthomas/chroma) for syntax colour. The browser is the Chromium project's own snapshot build, pinned by revision. Colours are catppuccin-mocha.

## Docs

| File | Answers | Read |
|---|---|---|
| [`docs/function.md`](docs/function.md) | What Chromium does and what webu does, and how far; the translation layer (accessibility tree + layout snapshot → a document); the role table; frames; popups; how Chromium is fetched and run | 1st |
| [`docs/ui.md`](docs/ui.md) | The layout, the header's screens and the two panels, the page's three screens and four parts, how each thing is drawn, the popups, the two palettes, the files | 2nd |
| [`docs/ux.md`](docs/ux.md) | Core-key semantics, what `Enter` does on each thing, every focus's `Space` menu, the finder, every input's behaviour, the hotkey table, the timeline | 3rd |
| [`docs/webu-implementation.md`](docs/webu-implementation.md) | How it was actually built, what was measured, the pitfalls, the tests, what is done and what is not | — |
| [`docs/support.md`](docs/support.md) | The supported accessibility roles, generated from the role table | — |

The design docs are in Traditional Chinese, with every decision dated in place.

## Development

```
make build              → ./webu; the first launch downloads the pinned Chromium into the cache
make test               every test; the browser-backed ones run only once Chromium is downloaded, else skip
make fixtures           re-capture internal/ir's role fixtures and docs/support.md against the pinned Chromium
WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v     real-site smokes (Hacker News, GitHub)
make axdump URL=https://…                                any page's accessibility tree, through the local Chrome
make gif                re-record docs/demo.gif from .local/demos/demo.tape (VHS, a Nerd Font, network)
```

Releases are the family's: push a `v*` tag, GitHub Actions runs the tests on both platforms, goreleaser builds the archives and updates the Homebrew tap, and the release notes are the matching section of `CHANGELOG.md`.
