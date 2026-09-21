# webu

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**語言**：[English](README.md) · 繁體中文

**終端機瀏覽器** —— `Tab` / `Enter` / `Esc` / `Space` / `?` 驅動一切。真正的 Chromium 在背景 headless 跑，webu 取它的 accessibility tree，轉成一頁「item 與文字流」畫在終端機裡；每個操作都經 Chrome DevTools Protocol 打回 Chromium，所以頁面就是真的頁面：能登入、JavaScript 會跑、cookie 留得住。一句話：**把 screen reader 的輸出畫成頁面，而不是唸出來。**

> _不確定的時候，就按_ **`Space`**。

webu 是 `u`-family 的成員，也是 [這份 TUI 設計原則](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) 在瀏覽器領域的實作 —— 與 [kbu](https://github.com/vulcanshen/kbu)（Kubernetes）、[filu](https://github.com/vulcanshen/filu)（filesystem）、[sshu](https://github.com/vulcanshen/sshu)（ssh）同一套設計系統。實作路上發現了什麼、做到哪，在 [`docs/webu-implementation.md`](docs/webu-implementation.md)；設計本身在 [`docs/function.md`](docs/function.md)、[`docs/ui.md`](docs/ui.md)、[`docs/ux.md`](docs/ux.md) —— **包含試過而被否決的做法**。

## Demo

![demo](docs/demo.gif)

Hacker News 畫成 item 與文字流，`j`/`k`/`l` 走；`Enter` 在一則新聞上列出它能做什麼、`Space` 列出整份選單；`L` 是 Chrome 的 Cmd+L，帶著目前的 URL；`B` 是書籤 screen —— 目錄樹，`Enter` 開新分頁；`I` 是 DevTools，走過 Network、Storage、Console；`?` 是 help。

## 五個鍵就能驅動 webu

| 鍵 | 行為 |
|---|---|
| **`Tab`** | 在 web screen 的兩個面板 `[1] Tabs`、`[2] Page` 之間切換焦點 |
| **`Enter`** | 該項目最直觀的操作：頁面 item 上是「它能做什麼」的選單、主要動作排第一；landmark、heading、書籤目錄列上是收合 / 展開；書籤或歷史紀錄上是開新分頁 |
| **`Space`** | *這裡能做什麼？* —— 目前焦點的 contextual 選單，分 `item operation` 與 `panel operation` 兩區。也能關掉任何 popup |
| **`Esc`** | 退回去 —— 關最上層 popup、離開 visual mode、清掉過濾、從 screen 回到 web |
| **`?`** | 全域 help —— 整套按鍵一張表 |

header 的 screen 用一個大寫字母切換 —— **`W` / `B` / `H` / `D` / `S`** —— `1` / `2` 直達 web screen 的兩個面板。每個字母熱鍵同時也是 `Space` 選單裡的一列，括號印的就是要按的鍵、一字不差，所以不想背就不用背。

## header 與兩個面板

```
 [W]eb ╱ [B]ookmarks ╱ [H]istory ╱ [D]ownloads ╱ [S]ettings
```

**`[W]eb`** —— `[1] Tabs` 與 `[2] Page` 並列。分頁清單一列一個 Chromium target：游標說你在哪、綠字說頁面面板正在顯示哪一個，同一個 URL 開兩個分頁會依開啟順序編號。頁面面板永遠是頁面：第一列是 URL，之後是頁面本身，畫成 **item** —— 連結、按鈕、輸入框、核取方塊、下拉選單、媒體、heading、landmark —— 段落文字在 item 之間以固定欄寬流動。`j`/`k` 一列一列走、`h`/`l` 在同一列裡左右移，所以一排連結是橫著走過去，不會被跳過。landmark 以一列帶名字的細線開頭（`▾ navigation Repository ────`），`Enter` 收合；heading 也一樣，收到下一個同級 heading 為止。導覽清單畫成一行。JSON、YAML、TOML、Markdown、純文字的回應畫成一個帶語法色的 code block，不用 Chrome 自己的 viewer。

**`[B]ookmarks`** —— 一棵樹：根層在前，之後每個目錄一列、其下是裡面的書籤。書籤上 `Enter` 開新分頁，目錄列上 `Enter` 收合 / 展開。`a` 在游標所在處加書籤 —— 先 URL、再 title，`[W]eb` 正在顯示的頁面當預設值，所以加目前頁就是 `a`、`Enter`、`Enter`。`A` 在同一處加目錄，路徑 `a/b/c` 一次開三層。`m` 透過目錄樹的 picker 搬書籤。

**`[H]istory`** —— 每個看過的頁面，新的在上，永久保留；`C` 是唯一的清除入口。**`[D]ownloads`** —— 本次 session 的下載與進度，下載中時 header 下面的分隔線兼進度條。**`[S]ettings`** —— 就地編輯 `config.yaml`，一列一個 key，帶著它的值與用途：`Enter` 開一個輸入框（目前生效的值當預設值），或切換開關。

## 安裝

> webu **只支援 macOS / Linux**（macOS 的 amd64 與 arm64、Linux 的 amd64 —— Chromium snapshot 沒有 Linux ARM 的 build）。沒有原生 Windows 版。

**Homebrew**（macOS / Linux）：

```bash
brew install vulcanshen/tap/webu
```

**安裝腳本**（把最新 release 的執行檔放進 `~/.local/bin`，root 則放 `/usr/local/bin`）：

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/install.sh | sh
```

**從原始碼**：

```bash
go install github.com/vulcanshen/webu/cmd/webu@latest
```

或 clone 下來 build：

```bash
git clone https://github.com/vulcanshen/webu.git
cd webu
make build     # → ./webu   (CGO_ENABLED=0, -trimpath, stripped)
./webu
```

`Makefile` 包了常用的事 —— `make build`、`make install`（→ `$GOBIN`）/ `make uninstall`、`make test`、`make fixtures`（用釘死的 Chromium 重抓 role fixture）、`make gif`（重錄 demo）、`make snapshot`（goreleaser 本機打包到 `dist/`）。跑 `make` 列出全部。

**Chromium 在第一次啟動時來，不在包裡。** release 只有 Go 執行檔。webu 只跑一個釘死 revision 的 Chromium —— 版本編在執行檔裡、不開放覆寫、也絕不用你自己的 Chrome —— 第一次啟動時下載一次到 cache 目錄（macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`，約 175–250 MB），畫面上會說大小與進度。升級後若釘的版本變了，`webu browser update` 重抓。`webu version` 兩個版本一起印。

**需要 Nerd Font**，不是選配：連結、媒體、面板與 header 的圖示都是 Nerd Font glyph，而且排版量的是它們的寬度。

### 移除

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

刪掉執行檔之後，下面三個目錄 —— 設定、資料、下載的 Chromium —— 逐一問、絕不假設。

## 快速開始

```bash
webu                              # 上次 session 的分頁，或一個空頁
webu https://news.ycombinator.com # 直接開一頁
webu go.dev lobste.rs             # 每個參數一個分頁，第一個在前；scheme 自動補
webu "terminal browser"           # 不像 URL 的字就拿去搜尋
webu help                         # 完整用法；webu version 印版本
```

命令列就是 Location 框：每個參數都在上次 session 還原的分頁之後開一個新分頁 —— 家族其他工具用不到、瀏覽器少不了的入口。

`L` 開 Location 輸入框：打 URL，或打要搜尋的字。`j`/`k` 走 item，`Enter` 看游標下那個能做什麼。在任何面板按 `Space` 讀選單 —— 它列的就是這個面板能做的全部事情。

## 你的資料放在哪

三個目錄，依內容分：

| | 是什麼 | 在哪 |
|---|---|---|
| 設定 | `config.yaml`、`bookmarks.yaml` —— 你寫的 | `~/.config/webu`（有設 `$XDG_CONFIG_HOME` 就在它底下；`$WEBU_CONFIG` 直接指定目錄） |
| 資料 | `history.yaml`、`session.yaml`、`downloads/`、Chromium 的 `profile/`（cookie、登入狀態）、`webu.log` —— webu 自己產生的 | `~/.webu/datas`（`$WEBU_DATA`） |
| cache | 釘死的 Chromium，隨時能重抓 | macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`（`$XDG_CACHE_HOME/webu`、`$WEBU_CACHE`） |

設定與書籤都是可以手改的 YAML；每個書籤帶一個 `folder` 路徑，另有 `folders:` 清單讓空目錄留得住。歷史是 YAML sequence，一次 append 一筆。每次寫檔都是原子的。

### 設定 —— `config.yaml`

```yaml
# 在 L 打的不是 URL 時，搜尋送去哪。預設 DuckDuckGo。
search_engine: https://duckduckgo.com/?q=
# 下載落在哪。預設 ~/.webu/datas/downloads。
download_dir: ~/Downloads
# 段落折行的欄寬。預設 100。
measure: 100
# 啟動時是否重新開啟上次離開時的分頁。預設 true；
# false 則從空白開始，或只開命令列給的網址。
restore_session: true
```

`[S]ettings` screen 編輯的就是這個檔。

## 按鍵

下面每個字母熱鍵同時也是該 surface `Space` 選單裡的一列。括號印的就是**要按的那個鍵**：`[A]dd folder` 是 shift+A、`[a]dd` 是裸的 `a`，標記沒寫的鍵不會有反應。

### 到處都通

```
 screen    W / B / H / D / S           screen 上按 Esc 回到 web
 面板      web 的 1 / 2  ·  Tab
 游標      j k    u d（半頁）          gg G      h l 沿著同一列
 頁面      P / N 上一頁 / 下一頁       L location      / 搜尋      V visual mode
 全域      Space 選單    ? help    q 離開    Ctrl+C 硬退
```

### `[1]` Tabs —— 小寫是游標那一列，大寫是整個面板

`Enter` 切到該分頁 · `w` 關閉 · `c` clone · `r` 重載 · `y` yank url · `T` 新分頁 · `X` 關掉其他 · `U` 復原關閉

### `[2]` Page

item 上 `Enter` 開它的操作 —— 連結的 Open / Open in new tab / Yank link url、按鈕的 Click、輸入框的 Edit 或 Submit / Edit / Clear / Yank、下拉選單的 Choose、landmark 與 heading 的 Collapse / Expand —— 每一種都另有 Yank text 與 Inspect。panel operation：`R` 重載 · `T` 新分頁 · `P` / `N` 上一頁 / 下一頁 · `/` 搜尋 · `V` visual mode · `L` location · `A` 加書籤 · `O` outline · `I` inspect（DevTools）· `Z` zoom · `Y` yank page url · `C` 關掉這個分頁。

輸入框的 Enter 開一行輸入框：`Enter` 把值寫回頁面、`Esc` 頁面不動。Location 框（`L`）開啟時帶著目前頁面的 URL 當預設值：`Tab` 接手編輯、`Backspace` 整個清掉，不像 URL 的字就送去搜尋引擎。

### 四個 screen

- **Bookmarks** —— `Enter` 開新分頁、或收合 / 展開目錄 · `a` 在這裡加書籤 · `m` 搬移 · `x` 刪除（空目錄也行） · `y` yank url · `A` 在這裡加目錄（`a/b/c` 一次開三層） · `/` 過濾
- **History** —— `Enter` 開新分頁 · `x` 刪除 · `y` yank url · `C` 清空 · `/` 過濾
- **Downloads** —— `Enter` 開檔 · `o` 來源開新分頁 · `x` 移除（進行中的會先停掉） · `y` yank path · `C` 清掉已完成的 · `/` 過濾
- **Settings** —— `Enter` 編輯文字設定（目前生效的值當預設值：`Tab` 接手、`Backspace` 清掉、清空後 Enter 就是回預設）或切換開關

### DevTools（`I`）

`h` / `l` 在 **Network**（`Enter` 看 request 的 header 與 body、`C` 清空、`/` 過濾）、**Storage**（cookie、local 與 session storage：`x` 刪、`y` yank 值、`C` 清掉這個站的資料、`/` 過濾）、**Console**（每筆完整折行顯示；`Enter` 看該筆的細節 —— 物件一個 property 一列；`i` 開提示列，在頁面裡 eval 的 REPL；`C` 清空、`/` 過濾）與 **Source**（頁面的 HTML，`/` grep）之間切換。`Esc` 關閉。

### Visual mode（`V` 或 `/`）

頁面停住、邊框轉黃。`h j k l` 逐字元移動，`w` / `e` / `b` 逐字、`0` / `$` 到行首行尾、`u` / `d` 半頁、`gg` / `G` 到頭尾；`v` / `V` 開始逐字元或逐列選取，`y` 複製到系統剪貼簿（`pbcopy`、`wl-copy`、`xclip` 或 `xsel`），`/` 搜尋、`n` / `N` 下一個 / 上一個，`Enter` 對游標下的 item 動作，`Esc` 離開。

## 特色

- **文字後面是一個真的瀏覽器** —— 一個釘死版本的 Chromium，headless，用 webu 自己的持久 profile：登入狀態重開還在、JavaScript 會跑、cookie 留得住，而且完全不碰你自己的 Chrome。頁面的問題全交給 Chromium 解，webu 只負責把答案畫出來。
- **讀的是 accessibility tree，不是 HTML** —— screen reader 會唸的，就是你看到的：role、name、state。webu 不認得的 role 畫成它的文字加一個標記，不藏起來、還能點。每個支援的 role 都有一份對著釘死版本抓下來的 fixture，所以升引擎是一個決定，不是慢慢漂掉。
- **item 與文字流** —— 連結、按鈕、欄位、heading、landmark 是游標的落點；段落在它們之間以固定欄寬流動，表格留著欄、code 留著行與語法色、導覽清單畫成一行。`h`/`l` 走一排連結，`j`/`k` 走列。
- **擋路的收起來** —— landmark 的細線與 heading 那一列可以把底下的東西收成一列、寫著藏了多少；Outline（`O`）跳進收合的區段時會先展開。
- **兩份選單、一張表** —— `Enter` 是 item 的操作、`Space` 是 item 加 panel，每個括號裡的字母都從按鍵處理讀的同一張表產生，所以選單裡沒有的熱鍵不可能存在。
- **一列 screen 的 header** —— Web、Bookmarks、History、Downloads、Settings 在同一列 chip 上，亮的那格就是你在的地方；每個清單 screen 是一個面板，按鍵在下邊框，有自己的 `Space` 選單。
- **書籤有目錄** —— 每個書籤一條路徑，目錄是樹上的列，picker 搬來搬去，目錄一旦存在就存在到你刪掉為止。
- **看得到的下載** —— header 下的分隔線是進度條，screen 列出每個下載與狀態，用桌面的開啟器開檔，`q` 在有下載進行中時會先問。
- **頁面問的，就地回答** —— `alert` / `confirm` / `prompt` 與 `beforeunload`、HTTP basic / digest 驗證、檔案上傳、`target=_blank` 開成新分頁並切過去、憑證錯誤變成一個問題，全都是 webu 自己形狀的 popup。
- **Location 照 Chrome 的做法** —— 任何面板按 `L`，目前 URL 當預設值，`Tab` 接手編輯，不是 URL 的就拿去搜尋。
- **vim 動作的 visual mode** —— 頁面凍住、游標逐字元走、`y` 把選取放進系統剪貼簿。
- **終端機裡的 DevTools** —— Network 有 request 細節與 body、Storage 可以改、Console 印物件的方式跟 Chrome 一樣還能 eval、頁面原始碼可以 grep。
- **非 HTML 當文字回答** —— JSON、YAML、TOML、Markdown、XML 與純文字回應變成一個帶語法色的 code block，依欄寬折行、絕不截斷。
- **session 還原** —— 下次啟動分頁都在，切到才載入。
- **畫面穩定** —— 每一列都剛好等於終端機寬度，任何尺寸、任何內容；有測試橫跨尺寸、面板與 screen 檢查。
- **unix-first、靜態執行檔** —— macOS + Linux；`CGO_ENABLED=0`。chromedp 的 log 進檔案，永遠不進 TUI 正在畫的終端機。

## 現況

**v0.1.0。** 第一版：釘死的 Chromium、頁面畫成 item 與文字流、兩份選單、header 的 screen（書籤目錄、歷史、下載、設定）、Location 框、visual mode、DevTools，以及頁面會問的每一件事。見 [CHANGELOG.md](CHANGELOG.md)。

還沒有的：
- **就地編輯書籤**（先刪再加），以及 History screen 的 fuzzy 搜尋（目前是子字串過濾）
- **hover** —— 滑鼠移過才展開的頁面維持收著；游標是鍵盤游標
- **iframe** —— 畫成佔位框，不走進去
- `<textarea>` 用你自己的 `$EDITOR` 編輯、上傳檔案的檔案選擇器（目前打路徑）
- **媒體** —— 圖片、影片、音訊是佔位框；yank URL 拿去別處開
- **CAPTCHA、passkey / WebAuthn、WebRTC** —— 遇到會明講，還沒解：目前沒有有視窗的瀏覽器可以交棒
- 滑鼠、Linux ARM build（沒有它的 Chromium snapshot）

## 用什麼做的

Go、[Bubble Tea](https://github.com/charmbracelet/bubbletea) 與 [Lip Gloss](https://github.com/charmbracelet/lipgloss)、浮層用 [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay)、Chrome DevTools Protocol 用 [chromedp](https://github.com/chromedp/chromedp)、語法色用 [chroma](https://github.com/alecthomas/chroma)。瀏覽器是 Chromium 專案自己的 snapshot build，以 revision 釘死。配色是 catppuccin-mocha。

## 文件

| 檔 | 回答的問題 | 讀的順序 |
|---|---|---|
| [`docs/function.md`](docs/function.md) | 哪些事 Chromium 做、哪些事 webu 做、做到什麼程度；翻譯層（AX tree → IR）的 role 白名單與 fallback；Chromium 的取得與執行模式；功能清單 | 1 |
| [`docs/ui.md`](docs/ui.md) | 版面、header 的 screen 與兩個面板的職責、popup、DevTools、色帶、存檔、每項功能落到哪個 surface 哪一版 | 2 |
| [`docs/ux.md`](docs/ux.md) | core-key 語意、兩種模式（一般 / 選取）、文字輸入、每個 focus 的 Space menu、hotkey 全表、`?` 內容、浮層行為、時間軸、Location 框 | 3 |
| [`docs/webu-implementation.md`](docs/webu-implementation.md) | 實作怎麼落地、CDP 實測出來的決定、做到哪、沒做哪 | — |
| [`docs/support.md`](docs/support.md) | 支援的 accessibility role，由 role 表產生 | — |

三份設計文件的每個決定都在原處標了日期；擱置與 v2 項目在各自的待決 / 擱置段落，不散落在正文。

## 開發

```
make build              → ./webu；首次啟動會下載釘死版本的 Chromium 到 cache 目錄
make test               所有測試；有下載過 Chromium 才會跑瀏覽器測試，否則 skip
make fixtures           用釘死的 Chromium 重抓 internal/ir 的 role fixture 與 docs/support.md
WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v     真站 smoke（Hacker News、GitHub）
make axdump URL=https://…                                任何頁面的 accessibility tree（用本機 Chrome）
make gif                從 .local/demos/demo.tape 重錄 docs/demo.gif（VHS、Nerd Font、網路）
```

發布與家族同一套：推 `v*` tag，GitHub Actions 在兩個平台跑測試，goreleaser 出檔並更新 Homebrew tap，release notes 取自 `CHANGELOG.md` 對應版本那一節。
