# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**語言**：[English](README.md) · 繁體中文

**把網頁當一份文件來讀的終端機瀏覽器。**

webu 把網頁抓下來，用你讀文章的方式把這一頁直接給的內容排進終端機 —— 標題、清單、程式碼、引用、表格、連結 —— 網站外殼不顯示。跟著連結走、找字、填搜尋框或簡單的登入，全部用鍵盤。它不跑網頁的 JavaScript：靠 script 才長出內容的網頁，看到的會很少。

> _不確定的時候，就按_ **`Space`**。

![demo](docs/demo.gif)

## 為什麼用 webu

- **讀的是這一頁，不是整個網站。** 顯示這一頁要給你的 —— 文章、文件、答案 —— 導覽列、側欄、頁尾、cookie 告示都不顯示。想看整頁時按 `w`。
- **為終端機畫的。** 標題的字放在它那一層的顏色上，清單前面是短橫線或編號，程式碼依語言上色，引用、表格畫成格子，連結與圖片前面有 icon，左邊有行號。
- **像 `less` 一樣讀。** `j|k|u|d|gg|G` 捲動、`/` 找字、`n|N` 在找到的之間跳；`f` 是 Focus mode，在能按的東西之間走 —— 連結、圖片、欄位、按鈕。
- **表單做到不需要 JavaScript 的程度**：搜尋框、簡單的帳密登入。cookie 會存著，登入後一直有效。
- **輕。** 不用下載或執行瀏覽器：一頁就是一個請求。
- **瀏覽器該有的都有**：分頁，每個分頁前面是網站的 icon、顏色取自它的 favicon；有目錄的書籤（能從任何瀏覽器匯入）、歷史、下載；這一頁的原始碼、這個網站的 cookie、載入它發出的請求；visual mode 選字複製到剪貼簿、整頁輸出成 markdown。

## 安裝

> **macOS**（Intel 與 Apple Silicon）與 **Linux**（amd64 與 arm64）。沒有 Windows 版。

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

有四件事要知道：

- **終端機必須用 [Nerd Font](https://www.nerdfonts.com/)** —— 連結、欄位、面板與網站記號都用它的 glyph 畫。
- **終端機必須支援 truecolor** —— 顏色、以及 popup 底下各層變暗的樣子，都用 24-bit 色畫。
- **終端機至少要 60×20** —— 視窗比這小時，webu 只顯示名稱、需要的大小與怎麼離開（按兩次 `Ctrl-C`），視窗拉大就回到原本的畫面。
- **icon 寬度**：有些終端機與字型的組合，印一個 Nerd Font 的 icon 游標會前進兩格而不是一格，有些連 powerline 符號（框上膠囊的兩端、游標尾端的箭頭）也一樣。webu 啟動時兩種都會量；框線仍然歪掉的話，自己設定：`WEBU__ICON_WIDTH=2`（或 `1`）與 `WEBU__POWERLINE_WIDTH` 只給 webu，`TERMINU__ICON_WIDTH` 與 `TERMINU__POWERLINE_WIDTH` 則是 terminu 家族每個 app 都讀的共用變數 —— 設一次全家族都跟著（`WEBU__` 的優先）。在家族其他 app（kbu、filu、sshu）的終端機裡執行時，那個 app 會替 webu 設好 `TERMINU__` 的兩個。

移除（刪設定與資料之前都會先問）：

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

## 快速開始

```bash
webu                                  # 開回上次的分頁
webu https://go.dev/doc/effective_go  # 讀一頁
webu go.dev lobste.rs                 # 一個網址一個分頁
webu "terminal browser"               # 不是網址的字就拿去搜尋
webu help                             # 完整的命令列說明
```

接著：

- `j|k|u|d` 捲動、`/` 找字、`n|N` 跳到下一個與上一個。
- `f`，再用 `j|k` 走到連結、`Enter` 開它；`H|L` 上一頁、下一頁。
- `o` 輸入網址、`w` 看整頁、`q` 離開。

## 使用方式

### 五個鍵

| 鍵 | 做什麼 |
|---|---|
| **`Enter`** | 在 Focus mode 裡，滑鼠點一下會做的事 —— 開連結、按按鈕、在框裡打字、選選項。在分頁清單上，到那一頁 |
| **`Space`** | *這裡能做什麼？* —— 游標所在的東西與這個面板能做的全部。所有熱鍵都列在裡面，不用背 |
| **`Esc`** | 往上一層：關 popup、離開模式、收起 `/` 找到的。永遠不會離開 webu |
| **`Tab`** | 下一個面板：分頁清單、頁面（`1`、`2` 直接到） |
| **`?`** | *這裡能按哪些鍵？* —— 最前面那一層的按鍵，只能看：面板自己的鍵加上 core key，在 popup 上則是這個 popup 的鍵。能做的事在 `Space` 裡；每個 Space menu 最後一列是 `Global operation`，列著到處都能做的事 |

### 畫面

```
 [W]EB  [B]OOKMARKS  [H]ISTORY  [D]OWNLOADS  [S]ETTINGS
```

用大寫字母切換，或從 `Space` › `Global operation` 選：

- **`W` Web** —— 分頁清單與頁面並排；頁面顯示分頁清單游標所在的那一頁，在清單上按 `Enter` 就過去。頁面最上面是網址，左邊有行號，下框顯示你讀到哪。
- **`B` Bookmarks** —— 目錄樹；`a` 加目前頁、`A` 加目錄、`m` 搬移、`r` 改名、`I` 匯入瀏覽器的書籤匯出檔。
- **`H` History** —— 所有看過的頁，新的在上。
- **`D` Downloads** —— 這次開啟以來的下載與進度。
- **`S` Settings** —— `config.yaml`，一列一個值，設好就存。

### 按鍵

下面每個字母同時也是該處 `Space` 選單裡的一列，括號印的就是要按的鍵：`[A]dd folder` 是 `Shift-A`，`[a]dd` 是單按 `a`。

#### 到處都通

```
 screen    W|B|H|D|S
 面板      Tab、1–2
 游標      j|k    u|d（半頁）          gg|G
 全域      Space 選單    ? 按鍵    ! App Log    q|Ctrl-C 離開
```

#### 分頁

頁面跟著游標換。每個分頁前面是網站的記號 —— 網站自己的 icon，沒有就是地球 —— 顏色取自它的 favicon。`Enter` 到頁面 · `c` 關閉 · `o` 在新分頁再開一次 · `r` 重新載入 · `y` 複製網址 · `T` 新分頁 · `X` 關閉其他 · `U` 重開剛關的

#### 頁面

網頁照原樣抓下來、不跑它的 JavaScript，顯示這一頁直接給的內容，網站外殼 —— 導覽列、側欄、頁尾、cookie 告示 —— 不顯示（[`docs/reader.md`](docs/reader.md)）。`j|k|u|d|gg|G` 捲動；`/` 找字、`n|N` 在找到的之間跳、`Esc` 收起來；`f` 是 Focus mode，`j|k` 在連結、圖片、欄位、按鈕之間跳，停在連結上時，inspect toast 寫出它會去的網址，`Enter` 按下去 —— 連結直接開。`H|L` 沿著這個分頁自己的歷史上一頁、下一頁，`b` 是它的麵包屑 —— 這個分頁走過的每一頁，寫標題、標出目前這一頁，選一頁就跳過去；`o` 開網址框；`w` 切到整頁，再按一次回到正文；`v` 是 Visual mode。連到檔案的連結會下載它。

表單做到搜尋框與簡單的帳密登入：在 Focus mode 裡，`Enter` 填欄位、勾選、選選項，在按鈕上按 `Enter` 送出；只有一個可以打字的欄位時，填完會問要不要送出。需要 JavaScript 的登入、用別的網站帳號登入、CAPTCHA、兩步驟驗證都不行。

`Space` 選單最上面是 **Source**（這一頁原始的內容）、**Cookies**（這個網站的 cookie；在裡面按 `C` 清掉）、**Requests**（載入這一頁發出的請求：每一次轉址、兩個方向的 header）。接著是 `R` 重新載入 · `T` 新分頁 · `H|L` 上一頁 / 下一頁 · `b` 麵包屑 · `o` 開網址 · `/` 找字 · `n|N` 下一個 / 上一個找到的 · `f` Focus mode · `v` Visual mode · `w` 整頁 · `A` 加書籤 · `Z` 縮放 · `Y` 複製網址 · `Yank markdown` · `C` 關這個分頁。在 Focus mode 裡，`o` 把游標所在的連結開在新分頁，`y` 複製它的網址。

`o` 開網址框時會用灰字提議目前的網址：`Tab` 接過來改、`Backspace` 不要它、直接打字就從頭打。

#### 書籤、歷史、下載、設定

- **Bookmarks** —— `Enter` 開新分頁或收合目錄 · `a` 新增 · `A` 新增目錄（`a/b/c` 每層都建）· `m` 搬移 · `r` 改名 · `x` 刪除 · `y` 複製網址 · `I` 匯入 · `/` 過濾
- **History** —— `Enter` 開新分頁 · `x` 刪除 · `y` 複製網址 · `C` 清空 · `/` 過濾
- **Downloads** —— `Enter` 開檔 · `o` 開來源頁 · `x` 移除 · `y` 複製路徑 · `C` 清掉已完成的 · `/` 過濾
- **Settings** —— `Enter` 改值或切換開關

#### Visual mode（`v`）

選字、複製。`h|j|k|l`、`w|e|b`、`0|$`、`gg|G` 移動；`v` 開始選取；`y` 複製到系統剪貼簿；`/` 問你要找什麼、找到的都加底線，`n|N` 在找到的之間走；`Enter` 按下游標所在的東西；`Esc` 先放掉選取，沒有選取時才離開；`Space` 開模式自己的選單。進入後頁面的框變黃，右上角寫著 *Visual*，選取中寫 *Visual>select*。

## 設定與資料

`~/.config/webu/config.yaml` —— 也可以在 `S` 畫面裡改：

```yaml
# 在 o 打的字不是網址時送去哪裡搜尋。預設 DuckDuckGo（Google 會對
# 程式送來的搜尋跳 CAPTCHA）。Brave 也可以：
# https://search.brave.com/search?q=
search_engine: https://html.duckduckgo.com/html/?q=
# 下載存到哪。預設 ~/.webu/datas/downloads。
download_dir: ~/Downloads
# 段落多寬換行：full，或欄數（20 以上）。
measure: full
# 啟動時開回上次的分頁。
restore_session: true
# terminu 家族共用的 key，每個家族 app 都一樣：
# 匯入的檔案預覽裡，私鑰只顯示第一行、只有擁有者能讀的檔不顯示內容。
hide_secrets_in_preview: false
# 多行文字（textarea）裡按 Tab 插入幾個空白。
input_tab_width: 4
```

東西存在哪：

| | 內容 | 位置 |
|---|---|---|
| 設定 | `config.yaml` | `~/.config/webu`（`$XDG_CONFIG_HOME/webu`，或 `$WEBU__CONFIG`） |
| 狀態 | `session.yaml`，上次離開時開著的分頁；`cookies.json`，讀過的網頁讓你登入的狀態（只有你自己能讀；`S` › `Clear sign-in data` 清空） | 跟設定放一起（或 `$WEBU__STATE`） |
| 資料 | `bookmarks.yaml`、歷史、下載 | `~/.webu/datas`（`$WEBU__DATA`） |

`config.yaml` 與 `bookmarks.yaml` 都是可以手改的 YAML。新版的 webu 把東西換到新位置時，第一次啟動會搬過去，並說從哪裡搬到哪裡；搬不動時 webu 會說，並照舊用原本的位置。webu 自己的檔讀不了時（例：壞掉的 `config.yaml`），啟動時會問要不要把它搬到 `.bak`、像第一次打開一樣開始 —— 回答之前什麼都不開；不要的話 webu 離開並說原因。`!` 打開 App Log：這次開 webu 以來做了什麼、出了什麼錯，toast 也在裡面。

舊版的 webu 會自己跑一個 Chromium。它下載到 cache 的那一份（macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`）在第一次啟動時刪掉，App Log 會記一筆；它的 profile（`profile/`）與 log（`webu.log`）在 `~/.webu/datas` 裡留給你處理，刪掉之前每次啟動 App Log 都會提一次。

## 限制

- **不跑 JavaScript。** 靠 script 才長出內容的網頁 —— app、dashboard、很多社群網站 —— 看到的很少或什麼都沒有，網頁自己的對話框與 frame 也不顯示。
- **圖片、影片、音訊**只有 icon 與網址 —— 複製網址到別處開。
- **CAPTCHA、passkey / WebAuthn**，以及需要 JavaScript 或別的網站帳號的登入都不行。
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
