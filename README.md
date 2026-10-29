# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**Language**: English · [繁體中文](README-zh_TW.md)

**A terminal browser that reads a web page as a document.**

webu fetches a web page and lays out what the page itself presents in your terminal, the way you would read it — headings, lists, code, quotes, tables, links — with the site around it left out. Follow links, find text, fill in a search box or a sign-in, all from the keyboard. It does not run a page's JavaScript: a page its scripts build shows little.

> _When in doubt, hit_ **`Space`**.

![demo](docs/demo.gif)

## Why webu

- **The page, not the site.** What the page presents — the article, the documentation, the answer — and not its navigation, sidebars, footers or cookie notices. `w` shows the full page when you want it.
- **Drawn for a terminal.** A heading's words on its level's colour, lists with dashes and numbers, code coloured by its language, quotes, tables as grids, an icon before each link and image, line numbers down the left.
- **Read the way `less` reads.** `j|k|u|d|gg|G` scroll, `/` finds and `n|N` step through what it found; `f` is Focus mode, which walks what can be pressed — links, images, fields, buttons.
- **Forms as far as they go without JavaScript**: a search box, a plain sign-in. The cookies are kept, so you stay signed in.
- **Light.** No browser to download or run: a page is a request.
- **Everything else a browser has**: tabs, each marked with its site's icon in its favicon's colour; bookmarks in folders (importable from any browser), history, downloads; the page's source, its site's cookies and the requests that loaded it; visual-mode selection to the clipboard, and the page as markdown.

## Install

> **macOS** (Intel and Apple Silicon) and **Linux** (amd64 and arm64). No Windows build.

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

Four things to know:

- **A [Nerd Font](https://www.nerdfonts.com/) is required** in your terminal — links, fields, panels and site marks are drawn with its glyphs.
- **A truecolor terminal is required** — the colours, and how the layers under a popup fade, are drawn in 24-bit colour.
- **The terminal needs at least 60×20** — in a smaller window webu shows only its name, the size it needs and how to quit (`Ctrl-C` twice), and everything comes back as the window grows.
- **Icon width**: some terminal and font pairs move the cursor two cells for a Nerd Font icon instead of one, and some do the same for the powerline symbols (the ends of the capsules on the frames, the arrow at the end of the cursor). webu measures both when it starts; if the frames still come out crooked, set them yourself: `WEBU__ICON_WIDTH=2` (or `1`) and `WEBU__POWERLINE_WIDTH` for webu alone, or `TERMINU__ICON_WIDTH` and `TERMINU__POWERLINE_WIDTH`, which every terminu family app reads — set them once and they all follow (the `WEBU__` ones come first). Inside another family app's terminal (kbu, filu, sshu), that app sets the `TERMINU__` ones for webu.

To uninstall (it asks before removing your settings and data):

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

## Quick start

```bash
webu                                  # reopen the last session's tabs
webu https://go.dev/doc/effective_go  # read a page
webu go.dev lobste.rs                 # one tab each
webu "terminal browser"               # anything that is not a URL is searched
webu help                             # the whole command line
```

Then:

- `j|k|u|d` scroll, `/` finds a word, `n|N` go to the next and the one before.
- `f`, then `j|k` to a link and `Enter` follows it; `H|L` back and forward.
- `o` to go to an address, `w` for the full page, `q` to quit.

## Usage

### Five keys

| Key | What it does |
|---|---|
| **`Enter`** | In Focus mode, what a mouse click would do — follow a link, press a button, type into a box, choose an option. On the tab list, go to the page |
| **`Space`** | *What can I do here?* — a menu of everything for the thing under the cursor and for the panel. Every hotkey is listed there, so there is nothing to memorize |
| **`Esc`** | One step back up: close a popup, leave a mode, put away what `/` found. It never quits |
| **`Tab`** | The next panel: the tab list, then the page (`1` and `2` go straight there) |
| **`?`** | *Which keys work here?* — the keys of what is in front, to read: a panel's own keys and the core keys, or on a popup that popup's. What can be done is under `Space`; every Space menu ends in `Global operation`, the list of what works from anywhere |

### The screens

```
 [W]EB  [B]OOKMARKS  [H]ISTORY  [D]OWNLOADS  [S]ETTINGS
```

Switch with a capital letter, or from `Space` › `Global operation`:

- **`W` Web** — your tabs beside the page; the page shows the tab under the cursor in the tab list, and `Enter` there takes you to it. The page has its address on top, line numbers down the left and your position in the bottom border.
- **`B` Bookmarks** — a folder tree; `a` adds the current page, `A` a folder, `m` moves, `r` renames, `I` imports a browser's bookmark export.
- **`H` History** — every page you visited, newest first.
- **`D` Downloads** — this session's downloads and their progress.
- **`S` Settings** — `config.yaml`, a value a row, saved the moment it is set.

### Key bindings

Every letter below is also a row in that place's `Space` menu, printed exactly as you press it: `[A]dd folder` is `Shift-A`, `[a]dd` a bare `a`.

#### Everywhere

```
 screens   W|B|H|D|S
 panels    Tab, 1–2
 cursor    j|k    u|d (half page)      gg|G
 global    Space menu    ? keys    ! App Log    q|Ctrl-C quit
```

#### Tabs

The page follows the cursor. Each tab starts with its site's mark — the site's own icon, or a globe — in the colour of its favicon. `Enter` go to the page · `c` close · `o` open again in a new tab · `r` reload · `y` yank url · `T` new tab · `X` close the others · `U` reopen the last closed

#### Page

A page is fetched as it is, without running its JavaScript, and what it presents is shown, the site around it — navigation, sidebars, footers, cookie notices — left out ([`docs/reader.md`](docs/reader.md)). `j|k|u|d|gg|G` scroll; `/` finds text, `n|N` step through it and `Esc` puts it away; `f` is Focus mode, where `j|k` go from one link, image, field or button to the next, the address of a link or image is on the frame, and `Enter` presses it — a link opens at once. `H|L` go back and forward along the tab's own history, and under the address the tab's trail says the way it came — the titles of the pages it went through, this one last; `o` is the address box; `w` shows the full page, and again its content; `v` is Visual mode. A link to a file downloads it.

Forms work as far as a search box and a plain sign-in: in Focus mode `Enter` fills a field, ticks a box or chooses an option, and `Enter` on the button sends the form; a form with one box to type in offers to send itself. A sign-in that needs JavaScript, another site's account, a CAPTCHA or a second step does not work.

`Space` starts with **Source** (the page as it came), **Cookies** (its site's; `C` there clears them) and **Requests** (what loading the page asked for: each redirect, the headers both ways). Then `R` reload · `T` new tab · `H|L` back / forward · `o` open · `/` find · `n|N` next / previous match · `f` Focus mode · `v` Visual mode · `w` full page · `A` bookmark it · `Z` zoom · `Y` yank the url · `Yank markdown` · `C` close the tab. On an item in Focus mode: open a link in a new tab, yank a link, an image's address or the text, inspect it.

`o` opens the address box with the current URL on offer, in grey: `Tab` takes it to edit, `Backspace` drops it, typing starts afresh.

#### Bookmarks, History, Downloads, Settings

- **Bookmarks** — `Enter` open in a new tab or fold a folder · `a` add · `A` add folder (`a/b/c` makes every level) · `m` move · `r` rename · `x` delete · `y` yank url · `I` import · `/` filter
- **History** — `Enter` open in a new tab · `x` delete · `y` yank url · `C` clear · `/` filter
- **Downloads** — `Enter` open the file · `o` open its source · `x` remove · `y` yank path · `C` clear finished · `/` filter
- **Settings** — `Enter` edit a value or flip a switch

#### Visual mode (`v`)

Select text and copy it. `h|j|k|l`, `w|e|b`, `0|$`, `gg|G` move; `v` starts a selection; `y` copies to the system clipboard; `/` asks what to find and underlines every match, `n|N` step through them; `Enter` presses what the cursor is on; `Esc` drops a selection, and leaves when there is none; `Space` opens the mode's own menu. While it is on, the page's frame turns yellow and says *Visual* at its top right, *Visual>select* while selecting.

## Settings and data

`~/.config/webu/config.yaml` — or edit it on the `S` screen:

```yaml
# Where words typed at o go when they are not a URL. Default DuckDuckGo
# (Google answers a program with a CAPTCHA). Brave works too:
# https://search.brave.com/search?q=
search_engine: https://html.duckduckgo.com/html/?q=
# Where downloads land. Default ~/.webu/datas/downloads.
download_dir: ~/Downloads
# How wide a paragraph runs before it wraps: full, or a number of columns (20+).
measure: full
# Reopen the last session's tabs on launch.
restore_session: true
# The terminu family's own keys, the same in every family app:
# in the file preview of the import picker, show only the first line of a
# private key and nothing of a file only its owner may read.
hide_secrets_in_preview: false
# How many spaces Tab puts in a text of several lines (a textarea).
input_tab_width: 4
```

Where things are kept:

| | What | Where |
|---|---|---|
| settings | `config.yaml` | `~/.config/webu` (`$XDG_CONFIG_HOME/webu`, or `$WEBU__CONFIG`) |
| state | `session.yaml`, the tabs open when webu last quit; `cookies.json`, where the pages you read have signed you in (only you can read it; `S` › `Clear sign-in data` empties it) | with the settings (or `$WEBU__STATE`) |
| data | `bookmarks.yaml`, the history, downloads | `~/.webu/datas` (`$WEBU__DATA`) |

`config.yaml` and `bookmarks.yaml` are plain YAML you can edit by hand. When a new version of webu keeps something in a new place, the first start moves it there and says from where to where; if it cannot, webu says so and goes on using the old place. A file of webu's own that will not read (a broken `config.yaml`, say) is offered to be moved aside to `.bak` at the start, webu then starting as it does the first time — nothing opens before you answer; or webu quits and says why. `!` opens the App Log: what webu did and what went wrong this time, toasts included.

An older webu ran its own Chromium. The one it downloaded into the cache (macOS `~/Library/Caches/webu`, Linux `~/.cache/webu`) is deleted at the first start, the App Log saying so; its profile (`profile/`) and its log (`webu.log`) in `~/.webu/datas` are left for you, named in the App Log at every start until you delete them.

## Limitations

- **JavaScript does not run.** A page its scripts build — an app, a dashboard, many social sites — shows little or nothing, and a page's own dialogs and frames are not shown.
- **Images, video and audio** are an icon and their address — yank it and open it elsewhere.
- **CAPTCHAs, passkeys / WebAuthn**, and sign-ins that need JavaScript or another site's account do not work.
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
