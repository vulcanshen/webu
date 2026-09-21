# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**Language**: English · [繁體中文](README-zh_TW.md)

**A terminal browser** — `Tab` / `Enter` / `Esc` / `Space` / `?` drive everything. A real Chromium runs headless in the background; webu takes its accessibility tree, turns it into a page of items and flowing text, and draws that in your terminal. Every action goes back through the Chrome DevTools Protocol, so the page is the real page: it logs in, it runs its JavaScript, it keeps its cookies. A screen reader's output, drawn as a page rather than read aloud.

> _When in doubt, hit_ **`Space`**.

webu is a member of the `u`-family and a browser-domain implementation of [this TUI Design Principle](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) — the same design system as [kbu](https://github.com/vulcanshen/kbu) (Kubernetes), [filu](https://github.com/vulcanshen/filu) (filesystem) and [sshu](https://github.com/vulcanshen/sshu) (ssh). See [`docs/webu-implementation.md`](docs/webu-implementation.md) for what was found on the way and where things stand, and [`docs/function.md`](docs/function.md), [`docs/ui.md`](docs/ui.md), [`docs/ux.md`](docs/ux.md) for the design — including the approaches that were tried and rejected.

## Demo

![demo](docs/demo.gif)

Hacker News as items and flow, `j`/`k`/`l` walking them; `Enter` on a story lists what it can do, `Space` lists the whole menu; `L` is Chrome's Cmd+L with the page's URL on offer; `B` is the Bookmarks screen — folders as a tree, `Enter` opening one in a new tab; `I` is DevTools with Network, Storage and Console; `?` is the help.

## Five keys to drive webu

| Key | Behavior |
|---|---|
| **`Tab`** | Move focus between the two panels of the web screen: `[1] Tabs` and `[2] Page` |
| **`Enter`** | The item's most intuitive operation — a click, as a mouse would: a text box opens to type, a select drops its list, a button is pressed, a link asks first; on a landmark, heading or bookmark folder, collapse or expand; on a bookmark or a history entry, open it in a new tab |
| **`Space`** | *What can I do here?* — the contextual menu for whatever has focus: `item operation` and `panel operation`. Also closes any popup |
| **`Esc`** | Back out — close the top popup, leave visual mode, clear a filter, return from a screen to the web |
| **`?`** | Global help — the whole key vocabulary in one list |

The header's screens are switched with a single shifted letter — **`W` / `B` / `H` / `D` / `S`** — and `1` / `2` address the two panels of the web screen. Every letter hotkey is also a row in the `Space` menu, with the key printed in its bracket exactly as you press it, so there is nothing to memorize unless you want to.

## The header and the two panels

```
 [W]eb ╱ [B]ookmarks ╱ [H]istory ╱ [D]ownloads ╱ [S]ettings
```

**`[W]eb`** — `[1] Tabs` beside `[2] Page`. The tabs list is one row per Chromium target; the cursor says where you are, green says which one the page panel is showing, and two tabs on the same URL are numbered in the order they were opened. The page panel is always the page: its first row is the URL, then the page as **items** — links, buttons, text boxes, checks, selects, media, headings, landmarks — with paragraphs flowing between them at a measured width. `j`/`k` step by row and `h`/`l` along one, so a row of links is walked sideways rather than skipped. Landmarks open as a named rule (`▾ banner ────`) and collapse on `Enter`; so do headings, down to the next heading of their level. A navigation is one row, an entry — `▎ 󰍜 Main +12` — that lists its links on `Enter`, and `Enter` again opens one. A JSON, YAML, TOML, Markdown or plain-text response is drawn as one code block with syntax colour instead of Chrome's own viewer.

**`[B]ookmarks`** — a tree: the top level first, then each folder as a row of its own with what it holds beneath it. `Enter` on a bookmark opens it in a new tab; on a folder it collapses or expands. `a` adds a bookmark where the cursor is — its URL, then its title, with the page the web is showing on offer, so the current page is `a`, `Enter`, `Enter`. `A` adds a folder there, and a path like `a/b/c` makes every level. `m` moves a bookmark through a picker of the tree. `I` imports a browser's bookmarks export — the HTML every browser writes — through a file picker, into a folder you name.

**`[H]istory`** — every page visited, newest first, kept for good; `C` is the one way it shrinks. **`[D]ownloads`** — this session's downloads with their progress, and the rule under the header doubles as the progress bar while one runs. **`[S]ettings`** — `config.yaml` edited in place, each row a key with its value and what it does: `Enter` opens a box with the value in force on offer, or flips a switch.

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

**A Nerd Font is required**, not optional: links, media, the panels and the header are drawn with Nerd Font glyphs, and the layout measures them.

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

The command line is the Location box: every argument opens a new tab after whatever the last session restores — something the rest of the family has no need of, and a browser cannot do without.

`L` opens the Location box; type a URL, or words to search for. `j`/`k` walk the items, `Enter` shows what the one under the cursor can do. Press `Space` on any panel and read the menu — it lists exactly what that panel can do.

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
# appended to it. Default Google.
search_engine: https://www.google.com/search?q=
# Where downloads land. Default ~/.webu/datas/downloads.
download_dir: ~/Downloads
# How wide a paragraph flows before it wraps, in cells. Default 100.
measure: 100
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
 page      P / N previous / next       L location      / search      v visual mode
 global    Space menu    ? help    q quit    Ctrl+C force quit
```

### `[1]` Tabs — lower case is the row, upper case is the panel

`Enter` switch to the tab · `c` close · `o` open in new tab · `r` reload · `y` yank url · `T` new tab · `X` close others · `U` undo close

### `[2]` Page

`Enter` on an item is a click, as a mouse would: a text box opens to type (a password box masked), a select drops its list, a button or check box is pressed, a landmark or heading collapses / expands. A link asks first — its text and URL in a confirm — and opens on `Enter` again. `Space` is the right-click menu: a link's Open / Open in new tab / Yank link url, a text box's Submit / Edit / Clear / Yank, a select's Choose, plus Yank text and Inspect on every item. Panel operations: `R` reload · `T` new tab · `P` / `N` back / forward · `/` search · `v` visual mode · `L` location · `A` add bookmark · `O` outline · `I` inspect (DevTools) · `Z` zoom · `Y` yank page url · `C` close this tab.

A text box's Enter opens a one-line box: `Enter` writes the value back, `Esc` leaves the page untouched. The Location box (`L`) opens with the page's own URL on offer: `Tab` takes it to edit, `Backspace` clears it, and words that are not a URL go to the search engine.

### The screens

- **Bookmarks** — `Enter` open in a new tab, or collapse / expand a folder · `a` add a bookmark here · `m` move · `r` rename (a folder too: what is in it follows) · `x` delete (a folder too: one with anything in it asks first, then goes with its whole tree) · `y` yank url · `A` add a folder here (`a/b/c` makes each level) · `I` import a browser's export into a folder of its own · `/` filter
- **History** — `Enter` open in a new tab · `x` delete · `y` yank url · `C` clear · `/` filter
- **Downloads** — `Enter` open the file · `o` source in a new tab · `x` remove (a running download is stopped) · `y` yank path · `C` clear the finished ones · `/` filter
- **Settings** — `Enter` edit a text setting (the value in force is on offer: `Tab` takes it, `Backspace` clears it, an emptied line means the default) or flip a switch

### DevTools (`I`)

`h` / `l` switch between **Network** (`Enter` a request's headers and body, `C` clear, `/` filter), **Storage** (cookies, local and session storage: `x` delete, `y` yank the value, `C` clear site data, `/` filter), **Console** (every entry whole, wrapped; `Enter` an entry's detail — an object listed property by property; `i` the prompt, a REPL that evaluates in the page; `C` clear, `/` filter) and **Source** (the page's HTML, `/` grep). `Esc` closes.

### Visual mode (`v` or `/`)

The page holds still and the frame turns yellow. `h j k l` move by character, `w` / `e` / `b` by word, `0` / `$` to either end of the line, `u` / `d` half a page, `gg` / `G` to the ends; `v` / `V` start selecting by character or by line, `y` copies to the system clipboard (`pbcopy`, `wl-copy`, `xclip` or `xsel`), `/` searches with `n` / `N`, `Enter` acts on the item under the cursor, `Esc` leaves.

## Features

- **A real browser behind the text** — one pinned Chromium, headless, with webu's own persistent profile: logins survive a restart, JavaScript runs, cookies are kept, and nothing of your own Chrome is touched. Every page problem is Chromium's to solve; webu only draws the answer.
- **The accessibility tree, not the HTML** — what a screen reader would read is what you see: roles, names, states. A role webu does not know is drawn as its text with a marker, never hidden, and still clickable. Every supported role has a fixture captured against the pinned revision, so an engine bump is a decision rather than a drift.
- **Items and flow** — links, buttons, fields, headings and landmarks are stops for the cursor; prose flows between them at a measured width, tables keep their columns, code keeps its lines and its syntax colour, a navigation is one row that lists its links on `Enter`. `h`/`l` walk a row of links; `j`/`k` step rows.
- **Collapse what is in the way** — a landmark's rule and a heading's row fold everything under them into one line that says what it hides, and the Outline (`O`) jumps into a folded section by opening it first.
- **Two menus, one table** — `Enter` is the item's operations, `Space` is item and panel together, and the letter in every bracket is generated from the same table the key handler reads, so a hotkey that is not in the menu cannot exist.
- **A header of screens** — Web, Bookmarks, History, Downloads, Settings on one chip row, the lit chip the one you are on; each list screen is a single panel with its keys in the bottom border and its own `Space` menu.
- **Bookmarks in folders** — a path per bookmark, folders as rows of a tree, a picker to move between them, and a folder that exists until you delete it.
- **Downloads you can watch** — progress on the rule under the header, a screen that lists them with their state, opening the file with the desktop's opener, and `q` that asks before cutting one off.
- **What a page asks, answered in place** — `alert` / `confirm` / `prompt` and `beforeunload`, HTTP basic and digest auth, file uploads, `target=_blank` as a new tab that is switched to, a certificate error as a question, all as popups in webu's own shape.
- **Location as Chrome does it** — `L` from any panel, the current URL on offer, `Tab` to edit it, anything that is not a URL searched.
- **Visual mode with vim's motions** — the page freezes, the cursor walks characters, `y` lands the selection on the system clipboard.
- **DevTools in the terminal** — Network with request details and bodies, Storage editable, a Console that prints objects the way Chrome's does and evaluates what you type, the page's source with grep.
- **Non-HTML answered as text** — JSON, YAML, TOML, Markdown, XML and plain responses become one syntax-coloured code block, folded at the measure, never cut.
- **Session restore** — the tabs come back on the next launch, unloaded until switched to.
- **Frame stability** — every rendered line is exactly the terminal width at every size, with any content; a test checks it across sizes, panels and screens.
- **unix-first, static binary** — macOS + Linux; `CGO_ENABLED=0`. The chromedp log goes to a file, never to the terminal the TUI is drawing on.

## Status

**v0.1.0.** The first release: the pinned Chromium, the page as items and flow, the two menus, the header of screens with bookmarks in folders, history, downloads and settings, the Location box, visual mode, DevTools, and everything a page can ask for. See [CHANGELOG.md](CHANGELOG.md).

Not there yet:
- **editing a bookmark's URL** in place (`r` renames; for the URL, delete and add it again for now) and fuzzy search on the History screen (it is a substring filter)
- **hover** — pages that reveal on mouse-over stay closed; the cursor is a keyboard cursor
- **iframes** — drawn as a placeholder; their content is not walked
- a `<textarea>` in your own `$EDITOR`, and a file picker for uploads (a path is typed for now)
- **media** — images, video and audio are placeholders; yank the URL and open it elsewhere
- **CAPTCHA, passkeys / WebAuthn, WebRTC** — said plainly when met, not solved: there is no windowed browser to hand off to yet
- mouse support, a Linux ARM build (no Chromium snapshot for it)

## Built with

Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss), [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay) for the floats, [chromedp](https://github.com/chromedp/chromedp) for the Chrome DevTools Protocol, and [chroma](https://github.com/alecthomas/chroma) for syntax colour. The browser is the Chromium project's own snapshot build, pinned by revision. Colours are catppuccin-mocha.

## Docs

| File | Answers | Read |
|---|---|---|
| [`docs/function.md`](docs/function.md) | What Chromium does and what webu does, and how far; the translation layer's role whitelist and fallback; how Chromium is fetched and run; the feature list | 1st |
| [`docs/ui.md`](docs/ui.md) | The layout, the header's screens and the two panels, the popups, DevTools, the colour bands, the files, which surface and version each feature lands in | 2nd |
| [`docs/ux.md`](docs/ux.md) | Core-key semantics, the two modes, text entry, every focus's `Space` menu, the whole hotkey table, the help, how floats behave, the timeline, the Location box | 3rd |
| [`docs/webu-implementation.md`](docs/webu-implementation.md) | How it was actually built, what the CDP work turned up, what is done and what is not | — |
| [`docs/support.md`](docs/support.md) | The supported accessibility roles, generated from the role table | — |

The three design docs are in Traditional Chinese, with every decision dated in place.

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
