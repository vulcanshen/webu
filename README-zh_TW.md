# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**語言**：[English](README.md) · 繁體中文

**把網頁當一份文件來讀的終端機瀏覽器。**

webu 開的是真正的網頁 —— 能登入、JavaScript 會跑、cookie 留得住 —— 再用你讀文章的方式排進終端機：先看目錄、一次讀一節，或整頁攤開。幾個鍵就跳到頁面上任何地方，填表單、回答頁面的對話框、走進 frame，全部用鍵盤。

> _不確定的時候，就按_ **`Space`**。

![demo](docs/demo.gif)

## 為什麼用 webu

- **它是真的瀏覽器。** 文字後面跑著一個釘死版本的 headless Chromium，有自己的持久 profile。網站登入之後會一直記得你、JavaScript 照跑，而且完全不碰你自己的 Chrome。
- **一頁就是一份文件。** 有標題的頁面一打開是目錄；`Enter` 讀一節、`n` / `p` 上下一節、`Esc` 回目錄。`Space` › `One sheet` 把整頁一次攤開。
- **header、正文、側欄、footer 分開放。** webu 依位置把每一頁切成幾個部分，文章就是文章，側欄在你要看之前不擋路。
- **幾個鍵找到任何東西。** `/` 搜整頁每一段文字，命中清單帶預覽，選了直接帶你過去。
- **表單就是表單。** 輸入框寫清楚它收什麼、textarea 開在編輯器裡、select 列出選項、slider 列出數字、日期和顏色送出前先檢查、檔案用 picker 選。
- **頁面自己的對話框與選單浮在上面**，等你回答 —— `alert`、`confirm`、HTTP 認證、上傳、憑證警告也一樣。
- **frame，包括別站的**，是一列，走得進去也走得出來。
- **瀏覽器該有的都有**：分頁、有目錄的書籤（能從任何瀏覽器匯入）、歷史、下載、DevTools（Network、Storage、Console、Source）、visual mode 選字複製到剪貼簿、整頁輸出成 markdown。

## 安裝

> **macOS**（Intel 與 Apple Silicon）與 **Linux**（amd64）。沒有 Windows 與 Linux ARM 版。

**Homebrew**：

```bash
brew install vulcanshen/tap/webu
```

**安裝腳本**（裝到 `~/.local/bin`，root 則是 `/usr/local/bin`）：

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/install.sh | sh
```

**Go**：

```bash
go install github.com/vulcanshen/webu/cmd/webu@latest
```

有兩件事要知道：

- **Chromium 在第一次啟動時下載**，只下載一次，放在 cache 目錄（約 175–250 MB），有進度列。升級後若需要新版 Chromium，執行 `webu browser update`。
- **終端機必須用 [Nerd Font](https://www.nerdfonts.com/)** —— 連結、欄位、面板與頁面的各部分都用它的 glyph 畫。

移除（刪設定、資料與下載的 Chromium 之前都會先問）：

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

## 快速開始

```bash
webu                              # 開回上次的分頁
webu https://news.ycombinator.com # 開一頁
webu go.dev lobste.rs             # 一個網址一個分頁
webu "terminal browser"           # 不是網址的字就拿去搜尋
webu help                         # 完整的命令列說明
```

接著：

- 在文件頁 —— `j` / `k` 在目錄上移動、`Enter` 讀一節、`n` 下一節、`Esc` 回目錄。
- 在任何頁 —— `/`、打一個字、`Enter`、`Enter`：你就到了。
- `L` 輸入網址、`P` / `N` 上一頁 / 下一頁、`q` 離開。

## 五個鍵

| 鍵 | 做什麼 |
|---|---|
| **`Enter`** | 滑鼠點一下會做的事 —— 開連結（先問）、按按鈕、在框裡打字、拉開清單。點了沒意義的地方就**走進去**：進一節、進清單裡的一件事、進 frame |
| **`Space`** | *這裡能做什麼？* —— 游標所在的東西與這個面板能做的全部。所有熱鍵都列在裡面，不用背 |
| **`Esc`** | 往上一層：關 popup、走出項目或 frame、回目錄 |
| **`Tab`** | 在分頁清單與頁面之間切換 |
| **`?`** | help —— 所有按鍵一張表 |

## 畫面

```
 [W]eb ╱ [B]ookmarks ╱ [H]istory ╱ [D]ownloads ╱ [S]ettings
```

用大寫字母切換：

- **`W` Web** —— 分頁清單與頁面並排。網址列底下是頁面各部分的切換列，再下面是目錄、一節或整頁，左邊有行號、下框顯示你讀到哪。
- **`B` Bookmarks** —— 目錄樹；`a` 加目前頁、`A` 加目錄、`m` 搬移、`r` 改名、`I` 匯入瀏覽器的書籤匯出檔。
- **`H` History** —— 所有看過的頁，新的在上。
- **`D` Downloads** —— 這次開啟以來的下載與進度。
- **`S` Settings** —— 就地編輯 `config.yaml`。

## 按鍵

下面每個字母同時也是該處 `Space` 選單裡的一列，括號印的就是要按的鍵：`[A]dd folder` 是 shift+A，`[a]dd` 是單按 `a`。

### 到處都通

```
 screen    W / B / H / D / S           screen 上 Esc 回到 web
 面板      1 / 2  ·  Tab
 游標      j k    u d（半頁）          gg G      h l 同列移動
 頁面      P / N 上一頁 / 下一頁       L 網址    / finder    v visual mode
 全域      Space 選單    ? help    q 離開    Ctrl+C 強制離開
```

### 分頁

`Enter` 切過去 · `c` 關閉 · `o` 在新分頁再開一次 · `r` 重新載入 · `y` 複製網址 · `T` 新分頁 · `X` 關閉其他 · `U` 重開剛關的

### 頁面

`Enter` 在連結上先問再開；在按鈕上按下；在輸入框上開框打字；在 select 上列出選項；在標題上收合；在一節、長的清單項目或 frame 上走進去。`Esc` 走出來。

`Space` 就是右鍵選單：連結開新分頁、複製連結或文字、送出、清除或編輯輸入框、檢查元素。對整頁：`R` 重新載入 · `T` 新分頁 · `P` / `N` 上一頁 / 下一頁 · `/` finder · `go` 跳到某行 · `n` / `p` 下一節 / 上一節 · `Sections` / `One sheet` · `v` visual mode · `L` 網址 · `A` 加書籤 · `I` DevTools · `Z` 縮放 · `Y` 複製網址 · `Yank markdown` · `C` 關這個分頁。

`L` 開網址框時會帶著目前的網址：`Tab` 接過來改、`Backspace` 清掉。

### 書籤、歷史、下載、設定

- **Bookmarks** —— `Enter` 開新分頁或收合目錄 · `a` 新增 · `A` 新增目錄（`a/b/c` 每層都建）· `m` 搬移 · `r` 改名 · `x` 刪除 · `y` 複製網址 · `I` 匯入 · `/` 過濾
- **History** —— `Enter` 開新分頁 · `x` 刪除 · `y` 複製網址 · `C` 清空 · `/` 過濾
- **Downloads** —— `Enter` 開檔 · `o` 開來源頁 · `x` 移除 · `y` 複製路徑 · `C` 清掉已完成的 · `/` 過濾
- **Settings** —— `Enter` 改值或切換開關

### DevTools（`I`）

`h` / `l` 切換 **Network**（`Enter` 看 request 的 header 與 body）、**Storage**（cookie、local 與 session storage —— `x` 刪除、`y` 複製、`C` 清除站台資料）、**Console**（`i` 在頁面裡執行 JavaScript）與 **Source**（頁面 HTML，`/` 搜尋）。`Esc` 關閉。

### Visual mode（`v`）

選字、複製。`h j k l`、`w e b`、`0 $`、`gg G` 移動；`v` / `V` 逐字或逐行開始選；`y` 複製到系統剪貼簿；`/` 搜尋、`n` / `N` 下一個；`Esc` 離開。

## 設定

`~/.config/webu/config.yaml` —— 也可以在 `S` 畫面裡改：

```yaml
# 在 L 打的字不是網址時送去哪裡搜尋。預設 DuckDuckGo（Google 會對
# headless 瀏覽器跳 CAPTCHA）。Brave 也可以：
# https://search.brave.com/search?q=
search_engine: https://html.duckduckgo.com/html/?q=
# 下載存到哪。預設 ~/.webu/datas/downloads。
download_dir: ~/Downloads
# 段落多寬換行：full，或欄數（20 以上）。
measure: full
# 啟動時開回上次的分頁。
restore_session: true
```

東西存在哪：

| | 內容 | 位置 |
|---|---|---|
| 設定 | `config.yaml`、`bookmarks.yaml` | `~/.config/webu`（`$XDG_CONFIG_HOME/webu`，或 `$WEBU_CONFIG`） |
| 資料 | 歷史、session、下載、瀏覽器 profile（cookie、登入）、log | `~/.webu/datas`（`$WEBU_DATA`） |
| cache | 下載的 Chromium | macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`（`$WEBU_CACHE`） |

設定與書籤都是可以手改的 YAML。

## 限制

- **圖片、影片、音訊**只是佔位 —— 複製網址到別處開。
- **CAPTCHA、passkey / WebAuthn、WebRTC** 不支援；webu 會明講，網址一個 yank 就拿到。
- **像 app 的頁面**（看板、dashboard）能用，但很密 —— 用 `/` 找，比捲動快。整站都是沒標語意的 `div` 的話，webu 只拿得到文字和能點的東西。
- 還不支援滑鼠。

## 更多

- [CHANGELOG.md](CHANGELOG.md) —— 每個版本改了什麼。
- [docs/dev-remarks.md](docs/dev-remarks.md) —— webu 內部怎麼運作、設計文件、從原始碼建置。
- webu 是 `u`-family 的一員 —— [kbu](https://github.com/vulcanshen/kbu)（Kubernetes）、[filu](https://github.com/vulcanshen/filu)（檔案）、[sshu](https://github.com/vulcanshen/sshu)（ssh）—— 共用同一套 [TUI 設計](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) 與同樣的按鍵。

## 授權

[GPL-3.0](LICENSE)
