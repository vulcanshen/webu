# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**Language**: English · [繁體中文](README-zh_TW.md)

**A terminal browser that reads a web page as a document.**

webu opens real web pages — logins, JavaScript, cookies and all — and lays them out in your terminal the way you would read them: a table of contents, one section at a time, or the whole page in one run. Jump anywhere with a few keystrokes, fill in forms, answer the page's dialogs, step into frames — all from the keyboard.

> _When in doubt, hit_ **`Space`**.

![demo](docs/demo.gif)

## Why webu

- **It is a real browser.** A pinned Chromium runs headless behind the text, with its own persistent profile. Sites log you in and keep you logged in, JavaScript runs, and your own Chrome is never touched.
- **A page is a document.** A page with headings opens on its table of contents; `Enter` reads a section, `n` / `p` move to the next or previous one, `Esc` goes back. `Space` › `One sheet` shows the whole page at once.
- **Header, body, sidebars, footer — kept apart.** webu splits every page into its parts by where they sit, so the article is the article and the sidebar stays out of the way until you want it.
- **Find anything in a few keys.** `/` searches every block of text on the page, lists the hits with a preview, and takes you straight there.
- **Forms that behave like forms.** Text boxes say what they take, a textarea opens in an editor, selects list their options, sliders list their numbers, dates and colours are checked before they are sent, files come from a picker.
- **The page's own dialogs and menus float** over it and wait for your answer — and so do `alert`, `confirm`, HTTP auth, uploads and certificate warnings.
- **Frames, including other sites',** are one row you can step into and out of.
- **Everything else a browser has**: tabs, bookmarks in folders (importable from any browser), history, downloads, DevTools (Network, Storage, Console, Source), visual-mode selection to the clipboard, and the whole page as markdown.

## Install

> **macOS** (Intel and Apple Silicon) and **Linux** (amd64). No Windows or Linux ARM build.

**Homebrew**:

```bash
brew install vulcanshen/tap/webu
```

**Install script** (into `~/.local/bin`, or `/usr/local/bin` as root):

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/install.sh | sh
```

**Go**:

```bash
go install github.com/vulcanshen/webu/cmd/webu@latest
```

Two things to know:

- **Chromium is downloaded on the first launch**, once, into a cache directory (about 175–250 MB), with a progress line. After an upgrade that needs a newer Chromium, run `webu browser update`.
- **A [Nerd Font](https://www.nerdfonts.com/) is required** in your terminal — links, fields, panels and parts are drawn with its glyphs.

To uninstall (it asks before removing your settings, data and the downloaded Chromium):

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

## Quick start

```bash
webu                              # reopen the last session's tabs
webu https://news.ycombinator.com # open a page
webu go.dev lobste.rs             # one tab each
webu "terminal browser"           # anything that is not a URL is searched
webu help                         # the whole command line
```

Then:

- On a documentation page — `j` / `k` down the table of contents, `Enter` to read a section, `n` for the next, `Esc` back to the list.
- On any page — `/`, type a word, `Enter`, `Enter`: you are there.
- `L` to go to an address, `P` / `N` for back and forward, `q` to quit.

## Five keys

| Key | What it does |
|---|---|
| **`Enter`** | What a mouse click would do — follow a link (it asks first), press a button, type into a box, open a list. Where a click has no meaning, it **goes in**: into a section, a list item, a frame |
| **`Space`** | *What can I do here?* — a menu of everything for the thing under the cursor and for the panel. Every hotkey is listed there, so there is nothing to memorize |
| **`Esc`** | One step back up: close a popup, leave an item or frame, back to the table of contents |
| **`Tab`** | Switch between the tab list and the page |
| **`?`** | Help — every key in one list |

## The screens

```
 [W]eb ╱ [B]ookmarks ╱ [H]istory ╱ [D]ownloads ╱ [S]ettings
```

Switch with a capital letter:

- **`W` Web** — your tabs beside the page. Under the address bar is the page's parts strip; below it the table of contents, a section, or the whole page, with line numbers down the left and your position in the bottom border.
- **`B` Bookmarks** — a folder tree; `a` adds the current page, `A` a folder, `m` moves, `r` renames, `I` imports a browser's bookmark export.
- **`H` History** — every page you visited, newest first.
- **`D` Downloads** — this session's downloads and their progress.
- **`S` Settings** — `config.yaml`, edited in place.

## Key bindings

Every letter below is also a row in that place's `Space` menu, printed exactly as you press it: `[A]dd folder` is shift+A, `[a]dd` a bare `a`.

### Everywhere

```
 screens   W / B / H / D / S           Esc on a screen goes back to the web
 panels    1 / 2  ·  Tab
 cursor    j k    u d (half page)      gg G      h l along a row
 page      P / N back / forward        L location    / finder    v visual mode
 global    Space menu    ? help    q quit    Ctrl+C force quit
```

### Tabs

`Enter` switch to it · `c` close · `o` open again in a new tab · `r` reload · `y` yank url · `T` new tab · `X` close the others · `U` reopen the last closed

### Page

`Enter` on a link asks, then opens; on a button presses; on a box opens it to type; on a select lists its options; on a heading folds it; on a section, a long list item or a frame goes in. `Esc` comes back out.

`Space` is the right-click menu: open a link in a new tab, yank a link or text, submit, clear or edit a box, inspect an element. For the page: `R` reload · `T` new tab · `P` / `N` back / forward · `/` finder · `go` go to line · `n` / `p` next / previous section · `Sections` / `One sheet` · `v` visual mode · `L` location · `A` bookmark it · `I` DevTools · `Z` zoom · `Y` yank the url · `Yank markdown` · `C` close the tab.

`L` opens the address box with the current URL on offer: `Tab` takes it to edit, `Backspace` clears it.

### Bookmarks, History, Downloads, Settings

- **Bookmarks** — `Enter` open in a new tab or fold a folder · `a` add · `A` add folder (`a/b/c` makes every level) · `m` move · `r` rename · `x` delete · `y` yank url · `I` import · `/` filter
- **History** — `Enter` open in a new tab · `x` delete · `y` yank url · `C` clear · `/` filter
- **Downloads** — `Enter` open the file · `o` open its source · `x` remove · `y` yank path · `C` clear finished · `/` filter
- **Settings** — `Enter` edit a value or flip a switch

### DevTools (`I`)

`h` / `l` switch between **Network** (`Enter` a request's headers and body), **Storage** (cookies, local and session storage — `x` delete, `y` yank, `C` clear site data), **Console** (`i` to evaluate JavaScript in the page) and **Source** (the page's HTML, `/` to grep). `Esc` closes.

### Visual mode (`v`)

Select text and copy it. `h j k l`, `w e b`, `0 $`, `gg G` move; `v` / `V` start a selection by character or line; `y` copies to the system clipboard; `/` searches with `n` / `N`; `Esc` leaves.

## Configuration

`~/.config/webu/config.yaml` — or edit it on the `S` screen:

```yaml
# Where words typed at L go when they are not a URL. Default DuckDuckGo
# (Google shows a headless browser a CAPTCHA). Brave works too:
# https://search.brave.com/search?q=
search_engine: https://html.duckduckgo.com/html/?q=
# Where downloads land. Default ~/.webu/datas/downloads.
download_dir: ~/Downloads
# How wide a paragraph runs before it wraps: full, or a number of columns (20+).
measure: full
# Reopen the last session's tabs on launch.
restore_session: true
```

Where things are kept:

| | What | Where |
|---|---|---|
| settings | `config.yaml`, `bookmarks.yaml` | `~/.config/webu` (`$XDG_CONFIG_HOME/webu`, or `$WEBU_CONFIG`) |
| data | history, session, downloads, the browser profile (cookies, logins), the log | `~/.webu/datas` (`$WEBU_DATA`) |
| cache | the downloaded Chromium | macOS `~/Library/Caches/webu`, Linux `~/.cache/webu` (`$WEBU_CACHE`) |

Settings and bookmarks are plain YAML you can edit by hand.

## Limitations

- **Images, video and audio** are placeholders — yank the URL and open it elsewhere.
- **CAPTCHAs, passkeys / WebAuthn and WebRTC** are not supported; webu says so and puts the URL a yank away.
- **App-like pages** (boards, dashboards) work, but they are dense — use `/` to get around rather than scrolling. A site built entirely from unlabelled `div`s gives webu only its text and its clickable things.
- No mouse support yet.

## More

- [CHANGELOG.md](CHANGELOG.md) — what changed in each release.
- [docs/dev-remarks.md](docs/dev-remarks.md) — how webu works inside, the design documents, and building from source.
- webu belongs to the `u`-family — [kbu](https://github.com/vulcanshen/kbu) (Kubernetes), [filu](https://github.com/vulcanshen/filu) (files), [sshu](https://github.com/vulcanshen/sshu) (ssh) — which share one [TUI design](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) and the same keys.

## License

[GPL-3.0](LICENSE)
