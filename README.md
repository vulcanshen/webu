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
- **A page is a document.** A page with headings opens on its table of contents; `Enter` reads a section, `n|p` move to the next or previous one, `Esc` goes back. `Space` › `One sheet` shows the whole page at once.
- **Header, body, sidebars, footer — kept apart.** webu splits every page into its parts by where they sit; each part is a tab of the page panel, so the article is the article and the sidebar stays out of the way until you `h|l` over to it.
- **Find anything in a few keys.** `/` searches every block of text on the page, lists the hits with a preview, and takes you straight there.
- **Forms that behave like forms.** Text boxes say what they take, a textarea opens in an editor, selects list their options, sliders list their numbers, dates and colours are checked before they are sent, files come from a picker.
- **The page's own dialogs and menus float** over it and wait for your answer — and so do `alert`, `confirm`, HTTP auth, uploads and certificate warnings.
- **Frames, including other sites',** are one row you can step into and out of.
- **Everything else a browser has**: tabs, bookmarks in folders (importable from any browser), history, downloads, a DevTools screen (Network, Storage, Console, Source), visual-mode selection to the clipboard, and the whole page as markdown.

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

Building from source is in [docs/dev-remarks.md](docs/dev-remarks.md).

Five things to know:

- **Chromium is downloaded on the first launch**, once, into a cache directory (about 175–250 MB), with a progress line. After an upgrade that needs a newer Chromium, run `webu browser update`.
- **A [Nerd Font](https://www.nerdfonts.com/) is required** in your terminal — links, fields, panels and parts are drawn with its glyphs.
- **A truecolor terminal is required** — the colours, and how the layers under a popup fade, are drawn in 24-bit colour.
- **The terminal needs at least 60×20** — in a smaller window webu shows only its name, the size it needs and how to quit (`Ctrl-C` twice), and everything comes back as the window grows.
- **Icon width**: some terminal and font pairs move the cursor two cells for a Nerd Font icon instead of one, and some do the same for the powerline symbols (the ends of the capsules on the frames, the arrow at the end of the cursor). webu measures both when it starts; if the frames still come out crooked, set them yourself: `WEBU__ICON_WIDTH=2` (or `1`) and `WEBU__POWERLINE_WIDTH` for webu alone, or `TERMINU__ICON_WIDTH` and `TERMINU__POWERLINE_WIDTH`, which every terminu family app reads — set them once and they all follow (the `WEBU__` ones come first). Inside another family app's terminal (kbu, filu, sshu), that app sets the `TERMINU__` ones for webu.

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

- On a documentation page — `j|k` down the table of contents, `Enter` to read a section, `n` for the next, `Esc` back to the list.
- On any page — `/`, type a word, `Enter`, `Enter`: you are there. `h|l` show the page's header, body, sidebars and footer one at a time; the first tab is the whole page.
- `L` to go to an address, `P|N` for back and forward, `q` to quit.

## Usage

### Five keys

| Key | What it does |
|---|---|
| **`Enter`** | What a mouse click would do — follow a link (it asks first), press a button, type into a box, open a list. Where a click has no meaning, it **goes in**: into a section, a list item, a frame |
| **`Space`** | *What can I do here?* — a menu of everything for the thing under the cursor and for the panel. Every hotkey is listed there, so there is nothing to memorize |
| **`Esc`** | One step back up: close a popup, leave an item or frame, back to the table of contents. It never quits |
| **`Tab`** | The next panel: the tab list, then the page (`1` and `2` go straight there) |
| **`?`** | *Which keys work here?* — the keys of what is in front, to read: a panel's own keys and the core keys, or on a popup that popup's. What can be done is under `Space`; every Space menu ends in `Global operation`, the list of what works from anywhere |

### The screens

```
 [W]EB  [B]OOKMARKS  [H]ISTORY  [D]OWNLOADS  [S]ETTINGS  [I] DEVTOOLS
```

Switch with a capital letter, or from `Space` › `Global operation`:

- **`W` Web** — your tabs beside the page; the page shows the tab under the cursor in the tab list, and `Enter` there takes you to it. The page's frame names its parts as tabs (`PAGE HEADER BODY OTHERS FOOTER`); under the address bar is the table of contents, a section, or the whole page, with line numbers down the left and your position in the bottom border.
- **`B` Bookmarks** — a folder tree; `a` adds the current page, `A` a folder, `m` moves, `r` renames, `I` imports a browser's bookmark export.
- **`H` History** — every page you visited, newest first.
- **`D` Downloads** — this session's downloads and their progress.
- **`S` Settings** — `config.yaml`, a value a row, saved the moment it is set.
- **`I` DevTools** — what the page on the web screen fetched, keeps, logged and is made of.

### Key bindings

Every letter below is also a row in that place's `Space` menu, printed exactly as you press it: `[A]dd folder` is `Shift-A`, `[a]dd` a bare `a`.

#### Everywhere

```
 screens   W|B|H|D|S|I
 panels    Tab, 1–2
 cursor    j|k    u|d (half page)      gg|G      h|l the panel's tabs
 global    Space menu    ? keys    ! App Log    q|Ctrl-C quit
```

#### Tabs

The page follows the cursor. `Enter` go to the page · `c` close · `o` open again in a new tab · `r` reload · `y` yank url · `T` new tab · `X` close the others · `U` reopen the last closed

#### Page

webu is becoming a reader ([`docs/reader.md`](docs/reader.md)): a page is fetched as it is, without running its JavaScript, and what it presents is shown as markdown for now, the site around it — navigation, sidebars, footers, cookie notices — left out; `f` shows the full page, and again its content. `Enter` on a link opens it at once. Forms work as far as a search box and a plain sign-in: fill the fields with `Enter`, then `Enter` on the button sends it; a form with one box to type in offers to send itself. A link to a file downloads it.

`j|k` walk the items in reading order — along a row, then down — and come round at the ends. `h|l` show the whole page, or one of its parts. `Enter` on a link asks, then opens; on a button presses; on a box opens it to type; on a select lists its options; on a heading folds it; on a section, a long list item or a frame goes in. `Esc` comes back out.

`Space` is the right-click menu: open a link in a new tab, yank a link or text, submit, clear or yank a box, inspect an element. For the page: `R` reload · `T` new tab · `P|N` back / forward · `/` finder · `go` go to a line by its number · `n|p` next / previous section · `Sections` / `One sheet` · `v` visual mode · `L` location · `A` bookmark it · `Z` zoom · `Y` yank the url · `Yank markdown` · `C` close the tab.

`L` opens the address box with the current URL on offer, in grey: `Tab` takes it to edit, `Backspace` drops it, typing starts afresh. The page's own questions — `alert`, `confirm`, a prompt, a sign-in — wait for `Enter`; `Esc` answers no.

#### Bookmarks, History, Downloads, Settings

- **Bookmarks** — `Enter` open in a new tab or fold a folder · `a` add · `A` add folder (`a/b/c` makes every level) · `m` move · `r` rename · `x` delete · `y` yank url · `I` import · `/` filter
- **History** — `Enter` open in a new tab · `x` delete · `y` yank url · `C` clear · `/` filter
- **Downloads** — `Enter` open the file · `o` open its source · `x` remove · `y` yank path · `C` clear finished · `/` filter
- **Settings** — `Enter` edit a value or flip a switch

#### DevTools (`I`)

Of the page the web screen shows. `h|l` switch between **Network** (the requests; the one under the cursor in full, headers and body, in the panel beside it), **Storage** (cookies, local and session storage — `x` delete, `y` yank, `C` clear site data, `R` read again), **Console** (`i` runs a line of JavaScript in the page) and **Source** (the page's HTML). `/` filters each list.

#### Visual mode (`v`)

Select text and copy it. `h|j|k|l`, `w|e|b`, `0|$`, `gg|G` move; `v|V` start a selection by character or line; `y` copies to the system clipboard; `/` asks what to find and underlines every match, `n|N` step through them; `Enter` presses what the cursor is on; `Esc` drops a selection, and leaves when there is none; `Space` opens the mode's own menu. While it is on, the page's frame turns yellow and says *Visual* at its top right, *Visual>select* while selecting.

## Settings and data

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
# The terminu family's own keys, the same in every family app:
# in the file preview of the upload and import pickers, show only the first
# line of a private key and nothing of a file only its owner may read.
hide_secrets_in_preview: false
# How many spaces Tab puts in a text of several lines (a textarea).
input_tab_width: 4
```

Where things are kept:

| | What | Where |
|---|---|---|
| settings | `config.yaml` | `~/.config/webu` (`$XDG_CONFIG_HOME/webu`, or `$WEBU__CONFIG`) |
| state | `session.yaml`, the tabs open when webu last quit; `cookies.json`, where the pages you read have signed you in (only you can read it; `S` › `Clear sign-in data` empties it) | with the settings (or `$WEBU__STATE`) |
| data | `bookmarks.yaml`, the history, downloads, the browser profile (cookies, logins), Chromium's log (`webu.log`) | `~/.webu/datas` (`$WEBU__DATA`) |
| cache | the downloaded Chromium | macOS `~/Library/Caches/webu`, Linux `~/.cache/webu` (`$XDG_CACHE_HOME/webu`; or `$WEBU__CACHE`) |

`config.yaml` and `bookmarks.yaml` are plain YAML you can edit by hand. When a new version of webu keeps something in a new place, the first start moves it there and says from where to where; if it cannot, webu says so and goes on using the old place. A file of webu's own that will not read (a broken `config.yaml`, say) is offered to be moved aside to `.bak` at the start, webu then starting as it does the first time — nothing opens before you answer; or webu quits and says why. `!` opens the App Log: what webu did and what went wrong this time, toasts included.

## Limitations

- **Images, video and audio** are placeholders — yank the URL and open it elsewhere.
- **CAPTCHAs, passkeys / WebAuthn and WebRTC** are not supported; webu says so and puts the URL a yank away.
- **App-like pages** (boards, dashboards) work, but they are dense — use `/` to get around rather than scrolling. A site built entirely from unlabelled `div`s gives webu only its text and its clickable things.
- No mouse: webu, like the rest of the terminu family, is keyboard only.

## More

- [CHANGELOG.md](CHANGELOG.md) — what changed in each release.
- [docs/dev-remarks.md](docs/dev-remarks.md) — how webu works inside, the design documents, and building from source.

## terminu family

webu follows [terminu design](https://github.com/vulcanshen/terminu), the design the terminu family of terminal apps share.

- [terminu design](https://github.com/vulcanshen/terminu) (tdp): terminal UIs you can use without reading the docs; the apps of the family are listed there
- [terminu-go](https://github.com/vulcanshen/terminu-go): a Go library that draws and drives a terminal UI the way terminu design says; webu is built on it
- [terminu-harness](https://github.com/vulcanshen/terminu-harness): a Claude Code plugin that makes TUI apps with terminu design and terminu-go

## License

[GPL-3.0](LICENSE)
