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
- **一頁就是一份文件。** 有標題的頁面一打開是目錄；`Enter` 讀一節、`n|p` 上下一節、`Esc` 回目錄。`Space` › `One sheet` 把整頁一次攤開。
- **header、正文、側欄、footer 分開放。** webu 依位置把每一頁切成幾個部分，每個部分是頁面面板的一個 tab；文章就是文章，側欄在你用 `h|l` 過去之前不擋路。
- **幾個鍵找到任何東西。** `/` 搜整頁每一段文字，命中清單帶預覽，選了直接帶你過去。
- **表單就是表單。** 輸入框寫清楚它收什麼、textarea 開在編輯器裡、select 列出選項、slider 列出數字、日期和顏色送出前先檢查、檔案用 picker 選。
- **頁面自己的對話框與選單浮在上面**，等你回答 —— `alert`、`confirm`、HTTP 認證、上傳、憑證警告也一樣。
- **frame，包括別站的**，是一列，走得進去也走得出來。
- **瀏覽器該有的都有**：分頁、有目錄的書籤（能從任何瀏覽器匯入）、歷史、下載、DevTools 畫面（Network、Storage、Console、Source）、visual mode 選字複製到剪貼簿、整頁輸出成 markdown。

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

從原始碼建置見 [docs/dev-remarks.md](docs/dev-remarks.md)。

有五件事要知道：

- **Chromium 在第一次啟動時下載**，只下載一次，放在 cache 目錄（約 175–250 MB），有進度列。升級後若需要新版 Chromium，執行 `webu browser update`。
- **終端機必須用 [Nerd Font](https://www.nerdfonts.com/)** —— 連結、欄位、面板與頁面的各部分都用它的 glyph 畫。
- **終端機必須支援 truecolor** —— 顏色、以及 popup 底下各層變暗的樣子，都用 24-bit 色畫。
- **終端機至少要 60×20** —— 視窗比這小時，webu 只顯示名稱、需要的大小與怎麼離開（按兩次 `Ctrl-C`），視窗拉大就回到原本的畫面。
- **icon 寬度**：有些終端機與字型的組合，印一個 Nerd Font 的 icon 游標會前進兩格而不是一格，有些連 powerline 符號（框上膠囊的兩端、游標尾端的箭頭）也一樣。webu 啟動時兩種都會量；框線仍然歪掉的話，自己設定：`WEBU__ICON_WIDTH=2`（或 `1`）與 `WEBU__POWERLINE_WIDTH` 只給 webu，`TERMINU__ICON_WIDTH` 與 `TERMINU__POWERLINE_WIDTH` 則是 terminu 家族每個 app 都讀的共用變數 —— 設一次全家族都跟著（`WEBU__` 的優先）。在家族其他 app（kbu、filu、sshu）的終端機裡執行時，那個 app 會替 webu 設好 `TERMINU__` 的兩個。

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

- 在文件頁 —— `j|k` 在目錄上移動、`Enter` 讀一節、`n` 下一節、`Esc` 回目錄。
- 在任何頁 —— `/`、打一個字、`Enter`、`Enter`：你就到了。`h|l` 一次看頁面的一個部分（header、正文、側欄、footer），第一個 tab 是整頁。
- `L` 輸入網址、`P|N` 上一頁 / 下一頁、`q` 離開。

## 使用方式

### 五個鍵

| 鍵 | 做什麼 |
|---|---|
| **`Enter`** | 滑鼠點一下會做的事 —— 開連結（先問）、按按鈕、在框裡打字、拉開清單。點了沒意義的地方就**走進去**：進一節、進清單裡的一件事、進 frame |
| **`Space`** | *這裡能做什麼？* —— 游標所在的東西與這個面板能做的全部。所有熱鍵都列在裡面，不用背 |
| **`Esc`** | 往上一層：關 popup、走出項目或 frame、回目錄。永遠不會離開 webu |
| **`Tab`** | 下一個面板：分頁清單、頁面（`1`、`2` 直接到） |
| **`?`** | *這裡能按哪些鍵？* —— 最前面那一層的按鍵，只能看：面板自己的鍵加上 core key，在 popup 上則是這個 popup 的鍵。能做的事在 `Space` 裡；每個 Space menu 最後一列是 `Global operation`，列著到處都能做的事 |

### 畫面

```
 [W]EB  [B]OOKMARKS  [H]ISTORY  [D]OWNLOADS  [S]ETTINGS  [I] DEVTOOLS
```

用大寫字母切換，或從 `Space` › `Global operation` 選：

- **`W` Web** —— 分頁清單與頁面並排；頁面顯示分頁清單游標所在的那一頁，在清單上按 `Enter` 就過去。頁面的框上用 tab 寫出它的各部分（`PAGE HEADER BODY OTHERS FOOTER`）；網址列底下是目錄、一節或整頁，左邊有行號、下框顯示你讀到哪。
- **`B` Bookmarks** —— 目錄樹；`a` 加目前頁、`A` 加目錄、`m` 搬移、`r` 改名、`I` 匯入瀏覽器的書籤匯出檔。
- **`H` History** —— 所有看過的頁，新的在上。
- **`D` Downloads** —— 這次開啟以來的下載與進度。
- **`S` Settings** —— `config.yaml`，一列一個值，設好就存。
- **`I` DevTools** —— Web 畫面上那一頁抓了什麼、存了什麼、印了什麼、原始碼是什麼。

### 按鍵

下面每個字母同時也是該處 `Space` 選單裡的一列，括號印的就是要按的鍵：`[A]dd folder` 是 `Shift-A`，`[a]dd` 是單按 `a`。

#### 到處都通

```
 screen    W|B|H|D|S|I
 面板      Tab、1–2
 游標      j|k    u|d（半頁）          gg|G      h|l 面板的 tab
 全域      Space 選單    ? 按鍵    ! App Log    q|Ctrl-C 離開
```

#### 分頁

頁面跟著游標換。`Enter` 到頁面 · `c` 關閉 · `o` 在新分頁再開一次 · `r` 重新載入 · `y` 複製網址 · `T` 新分頁 · `X` 關閉其他 · `U` 重開剛關的

#### 頁面

webu 正在改成閱讀器（[`docs/reader.md`](docs/reader.md)）：網頁照原樣抓下來、不跑它的 JavaScript，顯示這一頁直接給的內容，網站外殼 —— 導覽列、側欄、頁尾、cookie 告示 —— 不顯示。標題的字放在它那一層的顏色上，清單項目前面是短橫線或編號，程式碼與引用有自己的底色，連結與圖片前面有 icon。分頁清單裡每個分頁前面是網站的記號 —— 網站自己的 icon，沒有就是地球 —— 顏色取自它的 favicon。`j|k|u|d|gg|G` 捲動；`/` 找字、`n|N` 在找到的之間跳；`f` 是 Focus mode，`j|k` 在連結、圖片、欄位、按鈕之間跳，`Enter` 按下去（連結直接開，網址寫在框上）；`v` 是 Visual mode；`H|L` 上一頁、下一頁；`o` 開網址框；`w` 切到整頁，再按一次回到正文。表單做到搜尋框與簡單的帳密登入：在 Focus mode 用 `Enter` 填欄位，在按鈕上按 `Enter` 送出；只有一個可以打字的欄位時，填完會問要不要送出。連到檔案的連結會下載它。

`j|k` 照閱讀順序一個一個走 —— 先同一列、再往下 —— 到頭尾繞回。`h|l` 看整頁或其中一個部分。`Enter` 在連結上先問再開；在按鈕上按下；在輸入框上開框打字；在 select 上列出選項；在標題上收合；在一節、長的清單項目或 frame 上走進去。`Esc` 走出來。

`Space` 就是右鍵選單：連結開新分頁、複製連結或文字、送出、清除或複製輸入框的值、檢查元素。對整頁：`R` 重新載入 · `T` 新分頁 · `P|N` 上一頁 / 下一頁 · `/` finder · `go` 依行號跳到某行 · `n|p` 下一節 / 上一節 · `Sections` / `One sheet` · `v` visual mode · `L` 網址 · `A` 加書籤 · `Z` 縮放 · `Y` 複製網址 · `Yank markdown` · `C` 關這個分頁。

`L` 開網址框時會用灰字提議目前的網址：`Tab` 接過來改、`Backspace` 不要它、直接打字就從頭打。頁面自己的問題 —— `alert`、`confirm`、要你輸入的 prompt、登入 —— 等你按 `Enter`；`Esc` 就是回答「不」。

#### 書籤、歷史、下載、設定

- **Bookmarks** —— `Enter` 開新分頁或收合目錄 · `a` 新增 · `A` 新增目錄（`a/b/c` 每層都建）· `m` 搬移 · `r` 改名 · `x` 刪除 · `y` 複製網址 · `I` 匯入 · `/` 過濾
- **History** —— `Enter` 開新分頁 · `x` 刪除 · `y` 複製網址 · `C` 清空 · `/` 過濾
- **Downloads** —— `Enter` 開檔 · `o` 開來源頁 · `x` 移除 · `y` 複製路徑 · `C` 清掉已完成的 · `/` 過濾
- **Settings** —— `Enter` 改值或切換開關

#### DevTools（`I`）

看的是 Web 畫面上那一頁。`h|l` 切換 **Network**（各個 request；游標那一個的 header 與 body 完整顯示在旁邊的面板）、**Storage**（cookie、local 與 session storage —— `x` 刪除、`y` 複製、`C` 清除站台資料、`R` 重新讀取）、**Console**（`i` 在頁面裡執行一行 JavaScript）與 **Source**（頁面 HTML）。每個清單都能用 `/` 過濾。

#### Visual mode（`v`）

選字、複製。`h|j|k|l`、`w|e|b`、`0|$`、`gg|G` 移動；`v` 開始選取；`y` 複製到系統剪貼簿；`/` 問你要找什麼、找到的都加底線，`n|N` 在找到的之間走；`Enter` 按下游標所在的東西；`Esc` 先放掉選取，沒有選取時才離開；`Space` 開模式自己的選單。進入後頁面的框變黃，右上角寫著 *Visual*，選取中寫 *Visual>select*。

## 設定與資料

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
# terminu 家族共用的 key，每個家族 app 都一樣：
# 上傳與匯入的檔案預覽裡，私鑰只顯示第一行、只有擁有者能讀的檔不顯示內容。
hide_secrets_in_preview: false
# 多行文字（textarea）裡按 Tab 插入幾個空白。
input_tab_width: 4
```

東西存在哪：

| | 內容 | 位置 |
|---|---|---|
| 設定 | `config.yaml` | `~/.config/webu`（`$XDG_CONFIG_HOME/webu`，或 `$WEBU__CONFIG`） |
| 狀態 | `session.yaml`，上次離開時開著的分頁；`cookies.json`，讀過的網頁讓你登入的狀態（只有你自己能讀；`S` › `Clear sign-in data` 清空） | 跟設定放一起（或 `$WEBU__STATE`） |
| 資料 | `bookmarks.yaml`、歷史、下載、瀏覽器 profile（cookie、登入）、Chromium 的 log（`webu.log`） | `~/.webu/datas`（`$WEBU__DATA`） |
| cache | 下載的 Chromium | macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`（`$XDG_CACHE_HOME/webu`；或 `$WEBU__CACHE`） |

`config.yaml` 與 `bookmarks.yaml` 都是可以手改的 YAML。新版的 webu 把東西換到新位置時，第一次啟動會搬過去，並說從哪裡搬到哪裡；搬不動時 webu 會說，並照舊用原本的位置。webu 自己的檔讀不了時（例：壞掉的 `config.yaml`），啟動時會問要不要把它搬到 `.bak`、像第一次打開一樣開始 —— 回答之前什麼都不開；不要的話 webu 離開並說原因。`!` 打開 App Log：這次開 webu 以來做了什麼、出了什麼錯，toast 也在裡面。

## 限制

- **圖片、影片、音訊**只是佔位 —— 複製網址到別處開。
- **CAPTCHA、passkey / WebAuthn、WebRTC** 不支援；webu 會明講，網址一個 yank 就拿到。
- **像 app 的頁面**（看板、dashboard）能用，但很密 —— 用 `/` 找，比捲動快。整站都是沒標語意的 `div` 的話，webu 只拿得到文字和能點的東西。
- 不支援滑鼠：webu 跟 terminu 家族其他成員一樣，只用鍵盤。

## 更多

- [CHANGELOG.md](CHANGELOG.md) —— 每個版本改了什麼。
- [docs/dev-remarks.md](docs/dev-remarks.md) —— webu 內部怎麼運作、設計文件、從原始碼建置。

## terminu family

webu 遵循 [terminu design](https://github.com/vulcanshen/terminu)，terminu 家族的終端機 app 共用的設計。

- [terminu design](https://github.com/vulcanshen/terminu)（tdp）：不看文件就能用的 terminal UI；家族的 app 都列在那裡
- [terminu-go](https://github.com/vulcanshen/terminu-go)：照 terminu design 畫出並驅動終端機介面的 Go 函式庫，webu 建在它上面
- [terminu-harness](https://github.com/vulcanshen/terminu-harness)：照 terminu design 用 terminu-go 做 TUI app 的 Claude Code plugin

## 授權

[GPL-3.0](LICENSE)
