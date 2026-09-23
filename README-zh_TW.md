# webu

<p align="center"><img src="docs/icon.svg" width="128" alt="webu icon" /></p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/webu)](https://github.com/vulcanshen/webu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/webu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)

**語言**：[English](README.md) · 繁體中文

**把網頁當一份文件來讀的終端機瀏覽器。** 真正的 Chromium 在背景 headless 跑；webu 取它為 screen reader 維護的 accessibility tree，加上同一棵 DOM 的版面快照，把兩者合成一份能翻的文件：有目錄、一次讀一節、也能攤成一整張；頁面的四個 part（header、body、others、footer）靠它們在頁面上的位置分開；一個 finder 幾個鍵就到頁面上任何東西；表單畫成表單、頁面自己的彈窗浮在上面、frame 是可以走進去的一層。每個操作都經 Chrome DevTools Protocol 打回 Chromium，所以頁面就是真的頁面：能登入、JavaScript 會跑、cookie 留得住。

> _不確定的時候，就按_ **`Space`**。

webu 是 `u`-family 的成員，也是 [這份 TUI 設計原則](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) 在瀏覽器領域的實作 —— 與 [kbu](https://github.com/vulcanshen/kbu)（Kubernetes）、[filu](https://github.com/vulcanshen/filu)（filesystem）、[sshu](https://github.com/vulcanshen/sshu)（ssh）同一套設計系統。設計本身 —— **包含試過而被否決的做法，每條決定標日期** —— 在 [`docs/function.md`](docs/function.md)、[`docs/ui.md`](docs/ui.md)、[`docs/ux.md`](docs/ux.md)；實作怎麼落地、路上量到了什麼，在 [`docs/webu-implementation.md`](docs/webu-implementation.md)。

## Demo

![demo](docs/demo.gif)

一篇 Wikipedia 文章開成它的目錄 —— 一列一節、依深度縮排、各帶它裝了什麼與多長；`Enter` 讀一節、`n` 下一節、`Esc` 回目錄。再 `Esc` 上到 pagetab：頁面的四個 part，`l` 走、面板跟著切。`/` 是 finder —— 打一個字，命中清單帶預覽，`Enter` 進清單、再 `Enter` 就到那裡。`L` 是 Location 框；搜尋落在結果頁上，它自己也是一份文件、每個結果一節。`?` 是 help。

## 0.3.0 改了什麼

0.2.x 把 accessibility tree 排成一長頁；0.3.0 把它讀成一份**文件**：

- **一份文件三個畫面。** 有標題的頁面開頁先進**目錄**：一列一節、依深度縮排、右欄是它有幾張表、幾段 code、幾個媒體、多少行。`Enter` 讀**一節** —— 一節包含它的子節，章包含節 —— `n`/`p` 在同深度的節之間走、`Esc` 回目錄。`Space` › `One sheet` 攤成一整張，`Sections` 再切回來。標題不夠多的頁面一開始就是一整張。
- **四個 part，靠幾何切。** 頁面上最大的區塊是 **body**；在它旁邊的是 **others**（側欄、廣告欄）；在它前面的是 **header**、後面的是 **footer**。不看標籤叫什麼 —— `nav` 也好、`div` 也好，位置決定。四節是 URL 底下的一條鏈，一節一個 glyph；`Esc` 上去、`h`/`l` 走到哪內容就切到哪、`Enter` 回頁面。頁面比視窗矮、或沒有一塊獨大的頁面就是一塊，鏈只剩一條線。
- **finder，不是逐字搜。** `/` 開一個三欄的 finder，範圍是整頁 —— 四個 part 裡每一個持有文字的區塊 —— 命中依 part 列出、每個帶預覽。`Enter` 進清單，清單上再 `Enter` **去那裡**：切 part、開節、走進項目、游標落定 —— 什麼都不按。`go` 加數字跳行；每個畫面都有行號，目錄上的行號就是節的序號。
- **一件事一列。** 佔好幾行的清單項目或 article 只畫它的第一行；`Enter` 走進去（第一行變成面板的 header 列）、`Esc` 出來，頁面巢多深就走多深。只有一行的項目就是它裡面那個連結或按鈕。
- **表單畫成表單。** `<form>` 是畫進頁面裡的一個框：label 一欄對齊、值靠一邊、label 永不截斷（擠不下就疊成兩列）、fieldset 的 legend 粗體、填錯的值紅字、必填有標記。每一種輸入都定義了互動：一行框的邊框寫它收什麼（`email`、`number`、`date · YYYY-MM-DD`、`password`）；**textarea** 是有寫 / 移兩態的 editor popup；**slider** 是一條 bar 加數值，`Enter` 十個一窗列出數字；**日期、時間、顏色**框不合瀏覽器的形狀就擋下來，不讓瀏覽器無聲丟掉；**file** 用檔案 picker 回答；搜尋框 `Enter` 時問要不要送出。
- **頁面自己的彈窗浮起來。** modal、alertdialog、按鈕開出來的 menu、cookie 橫幅 —— 靠行為認（按下之後出現、拿到 focus 或自己宣告或疊在別的東西上、裡面有東西可按），從不看標籤。它用 webu 自己的 popup 框浮在變暗的頁面上，頁面疊幾層它就疊幾層，而且要一個回答：`Esc` 不關。它走了，你回到原本的地方。
- **frame 是一層。** `<iframe>` 是一列；`Enter` 走進它的文件、`Esc` 出來。**別站的** frame 是另一個 process、另一個 target —— webu 對它開一條自己的 session，在裡面讀、在裡面點，跟任何地方一樣；frame 裡再嵌 frame 也一樣走得進去。
- **真實網頁會用到的 role 全部到齊**：tabs（畫成 pagetab 式的鏈，選中的用所在節的顏色點亮）、menu、tree（縮排、`▾`/`▸`）、listbox（radio 或 check 列）、slider、progressbar / meter（填到值的 bar，唯讀）、`<details>`（開合的三角）、tooltip（`Enter` hover 時出現的旁白）、timer 與 status（在句子裡流動、跟著變的文字）。webu 不認得的仍然畫出來、標記、可點。
- **顏色是概念，不是元素。** 可按的 sapphire、可填的 mauve、code pink、媒體灰、pagetab rosewater、填錯的值紅；heading 與目錄用五個 hue 表深度。沒有括號、沒有 emoji，每個控制項以 glyph 開頭。
- **頁面記得自己的位置。** 上一頁回到離開時的地方 —— part、節、捲動、游標 —— 每個 history entry 各記一份。還在長的頁面（SPA 慢慢填）spinner 一直轉到兩次看到的一樣為止，不會把你放在半成品上。`Enter` 先 hover 再點，所以滑過才開的選單開得了。一次一個動作，快速走動不會把點擊拉離目標。
- **webu 簽自己的名字** —— Chromium 自報的字串、把 headless 版的 `HeadlessChrome/` 寫成 `Chrome/`、尾巴接 `webu/<version>`。不是偽裝，是一個真瀏覽器的名字。預設搜尋是 DuckDuckGo 的 HTML 端點，因為 Google 對每一次 headless 搜尋都回 reCAPTCHA。CAPTCHA 是一面牆、webu 會明講 —— 沒有交棒給視窗這回事，因為 webu 必須能在沒有視窗的機器上跑。

## 五個鍵就能驅動 webu

| 鍵 | 行為 |
|---|---|
| **`Tab`** | 在 web screen 的兩個面板 `[1] Tabs`、`[2] Page` 之間切換焦點 |
| **`Enter`** | 該項目最直觀的操作 —— 就是滑鼠左鍵，先 hover 再按：連結先問、按鈕直接按、輸入框開框打字、select 拉開清單；滑鼠沒有對應的就**進去**：從目錄開一節、走進清單裡的一件事、走進 frame |
| **`Space`** | *這裡能做什麼？* —— 目前焦點的 contextual 選單，分 `item operation` 與 `panel operation` 兩區。也能關掉 webu 自己的任何 popup |
| **`Esc`** | **往上一層**：關最上層 popup → 走出項目或 frame → 回目錄 → 上 pagetab → 再回頁面。頁面自己的彈窗不歸它關：那要一個回答 |
| **`?`** | 全域 help —— 整套按鍵一張表 |

header 的 screen 用一個大寫字母切換 —— **`W` / `B` / `H` / `D` / `S`** —— `1` / `2` 直達兩個面板。每個字母熱鍵同時也是 `Space` 選單裡的一列，括號印的就是要按的鍵、一字不差，所以不想背就不用背。

## header 與兩個面板

```
 [W]eb ╱ [B]ookmarks ╱ [H]istory ╱ [D]ownloads ╱ [S]ettings
```

**`[W]eb`** —— `[1] Tabs` 與 `[2] Page` 並列。分頁清單一列一個 Chromium target；游標說你在哪、綠字說頁面面板現在顯示的是哪一個。頁面面板永遠是頁面：第一列是 URL（旁邊的地球在載入時轉）、第二列是 **pagetab** —— 頁面的四個 part —— 底下是三個畫面之一：**目錄**、**一節**、**一整張**。每個畫面左邊都有行號。頁面本身是 **item** —— 連結、按鈕、輸入框、勾選、select、heading、表格的每一格、清單裡的每一件事、frame —— 中間是依欄寬流動的文字。`j`/`k` 換列、`h`/`l` 同列走、`u`/`d` 半頁。heading `Enter` 收合。面板下框說你在哪：`2/26 · Try it · 40%`，讀到哪填到哪。

**`[B]ookmarks`** —— 目錄樹：目錄是一列，`Enter` 開書籤到新分頁、或收合目錄；`a` 在這裡加書籤（帶目前頁）、`A` 加目錄（`a/b/c` 一次三層）、`m` 搬移、`r` 改名、`I` 從檔案 picker 匯入瀏覽器的書籤匯出檔。

**`[H]istory`** —— 每一頁，新的在上，永久保留；`C` 是唯一會讓它變短的鍵。**`[D]ownloads`** —— 本次 session 的下載與進度，header 底下那條線在下載時兼作進度條。**`[S]ettings`** —— 就地編輯 `config.yaml`，一列一個 key、帶目前值與用途。

## 安裝

> webu **只支援 macOS / Linux**（macOS 的 amd64 與 arm64、Linux 的 amd64 —— Chromium snapshot 沒有 Linux ARM 版）。沒有 Windows 原生版。

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

或 clone 後編譯：

```bash
git clone https://github.com/vulcanshen/webu.git
cd webu
make build     # → ./webu（CGO_ENABLED=0、-trimpath、strip）
./webu
```

`Makefile` 包了常用的事 —— `make build`、`make install`（→ `$GOBIN`）/ `make uninstall`、`make test`、`make fixtures`（用釘死的 Chromium 重抓 role fixture）、`make gif`（重錄 demo）、`make snapshot`（goreleaser 本機打包到 `dist/`）。`make` 列出全部。

**Chromium 是第一次啟動時下載的，不在包裡。** release 只有 Go 執行檔。webu 只跑一個釘死 revision 的 Chromium —— 編進執行檔、不開放覆寫、永遠不碰你自己的 Chrome —— 第一次啟動下載一次到 cache 目錄（macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`；約 175–250 MB），有進度列。升級後若 pin 了新 revision，`webu browser update` 去抓。`webu version` 印兩個版本。

**Nerd Font 是必要的**，不是選配：連結、欄位、媒體、part、面板與 header 都用 Nerd Font 的 glyph 畫，排版量的是它們的寬。

### 移除

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh
```

移除執行檔，然後對下面三個目錄 —— 設定、資料、下載的 Chromium —— 逐一問你，不自作主張。

## 快速開始

```bash
webu                              # 上次 session 的分頁，或一個空頁
webu https://news.ycombinator.com # 直接開一頁
webu go.dev lobste.rs             # 一個參數一個分頁，第一個在前；scheme 自動補
webu "terminal browser"           # 不像 URL 的字當搜尋
webu help                         # 完整命令列；webu version 印版本
```

在文件頁上：`j`/`k` 走目錄、`Enter` 讀一節、`n` 下一節、`Esc` 回目錄。在任何頁上：`/`、打一個字、`Enter`、`Enter` —— 你就在它上面了。任何面板按 `Space`，選單列的就是這個面板能做的全部。

## 你的資料放在哪

三個目錄，依內容分：

| | 是什麼 | 在哪 |
|---|---|---|
| settings | `config.yaml`、`bookmarks.yaml` —— 你寫的 | `~/.config/webu`（設了 `$XDG_CONFIG_HOME` 就是 `$XDG_CONFIG_HOME/webu`；`$WEBU_CONFIG` 直接指定） |
| data | `history.yaml`、`session.yaml`、`downloads/`、Chromium 的 `profile/`（cookie、登入）、`webu.log` —— webu 產生的 | `~/.webu/datas`（`$WEBU_DATA`） |
| cache | 釘死的 Chromium，可重新下載 | macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`（`$XDG_CACHE_HOME/webu`、`$WEBU_CACHE`） |

設定與書籤是可以手改的 YAML；每個書籤帶一個 `folder` 路徑，另有 `folders:` 清單讓空目錄留得住。歷史是一次 append 一筆的 YAML sequence。每次寫檔都是原子的。

### 設定 —— `config.yaml`

```yaml
# 在 L 打的字不是 URL 時，搜尋去哪：字會接在後面。預設 DuckDuckGo 的 HTML
# 端點 —— Google 對 headless 瀏覽器的搜尋一律回 reCAPTCHA。Brave
#（search.brave.com/search?q=）也可以。
search_engine: https://html.duckduckgo.com/html/?q=
# 下載落在哪。預設 ~/.webu/datas/downloads。
download_dir: ~/Downloads
# 段落多寬換行，單位是格。預設 full。
measure: full        # 或一個 20 以上的數字
# 下次啟動要不要開回上次離開時的分頁。預設 true；false 從空白開始，
# 或開命令列給的 URL。
restore_session: true
```

`[S]ettings` screen 改的是同一個檔，檔案裡有的 key 那裡一定有一列 —— 有測試守著。

## 按鍵

下面每個字母熱鍵同時是該畫面 `Space` 選單裡的一列。括號印的是**你要按的鍵、一字不差**：`[A]dd folder` 是 shift+A，`[a]dd` 是裸的 `a`，括號沒寫的不會觸發。

### 到處都通

```
 screen    W / B / H / D / S           screen 上 Esc 回到 web
 面板      web 的 1 / 2  ·  Tab
 游標      j k    u d（半頁）          gg G      h l 同列走
 頁面      P / N 上一頁 / 下一頁       L location    / finder    v visual mode
 全域      Space menu    ? help    q quit    Ctrl+C 硬退
```

### `[1]` Tabs —— 小寫是游標那一列，大寫是整個面板

`Enter` 切到該分頁 · `c` 關 · `o` 同頁再開一個 · `r` 重載 · `y` yank url · `T` 新分頁 · `X` 關其他 · `U` 復原關閉

### `[2]` Page

item 上的 `Enter` 就是滑鼠左鍵 —— 先 hover 再按：輸入框開框打字（密碼框遮罩、邊框寫這個框收什麼）、select 拉開清單、slider 列出數字、按鈕與勾選直接按、heading 收合。連結先問 —— confirm 裡是連結文字與 URL —— 再 `Enter` 才開；同頁的錨點直接跳。滑鼠沒有對應的地方 `Enter` 就進去：目錄裡的一節、佔好幾行的清單項目、frame。`Esc` 往上一層。

`Space` 是右鍵選單：連結的 Open / Open in new tab / Yank link url、輸入框的 Submit / Edit / Clear / Yank、select 的 Choose，每個 item 都有 Yank text 與 Inspect。面板操作：`R` 重載 · `T` 新分頁 · `P` / `N` 上一頁 / 下一頁 · `/` finder · `go` 跳行 · `n` / `p` 下一節 / 上一節 · `Sections` / `One sheet` · `Esc` page parts · `v` visual mode · `L` location · `A` 加書籤 · `I` inspect（DevTools）· `Z` zoom · `Y` yank page url · `Yank markdown` · `C` 關這個分頁。

finder（`/`）列出整頁四個 part 裡每一個持有文字的區塊，打字即篩、帶預覽，`Enter` 進清單、再 `Enter` 去那裡。Location 框（`L`）開啟時帶著目前頁的 URL 當提議：`Tab` 接進來改、`Backspace` 清掉、不像 URL 的字送去搜尋引擎。

### 四個 screen

- **Bookmarks** —— `Enter` 開新分頁，或收合 / 展開目錄 · `a` 在這裡加書籤 · `m` 搬移 · `r` 改名 · `x` 刪除（有東西的目錄先問）· `y` yank url · `A` 在這裡加目錄（`a/b/c` 一次開三層）· `I` 匯入瀏覽器匯出檔 · `/` 過濾
- **History** —— `Enter` 開新分頁 · `x` 刪除 · `y` yank url · `C` 清除 · `/` 過濾
- **Downloads** —— `Enter` 開檔 · `o` 來源開新分頁 · `x` 移除（進行中的會停掉）· `y` yank 路徑 · `C` 清掉已完成的 · `/` 過濾
- **Settings** —— `Enter` 改文字設定（目前生效的值當提議：`Tab` 接手、`Backspace` 清掉、清空後 Enter 就是預設）或翻開關

### DevTools（`I`）

`h` / `l` 在 **Network**（`Enter` 看 request 的 header 與 body、`C` 清、`/` 過濾）、**Storage**（cookie、local 與 session storage：`x` 刪、`y` yank 值、`C` 清站資料、`/` 過濾）、**Console**（每筆完整折行；`Enter` 看該筆的詳細；`i` 是 REPL，在頁面裡求值；`C` 清、`/` 過濾）與 **Source**（頁面 HTML、`/` grep）之間切換。`Esc` 關。

### Visual mode（`v`）

頁面靜止、外框變黃。`h j k l` 逐字元、`w` / `e` / `b` 逐字、`0` / `$` 到行首行尾、`u` / `d` 半頁、`gg` / `G` 到頭尾；`v` / `V` 開始逐字或逐行選取，`y` 複製到系統剪貼簿（`pbcopy`、`wl-copy`、`xclip` 或 `xsel`），`/` 在文字裡搜、`n` / `N` 下一個，`Enter` 對游標所在的 item 動作，`Esc` 離開。

## 特色

- **文字後面是一個真的瀏覽器** —— 一個釘死的 Chromium、headless、webu 自己的持久 profile：登入撐得過重啟、JavaScript 會跑、cookie 留著、你自己的 Chrome 一根汗毛都不碰。頁面的問題都是 Chromium 的問題；webu 只畫答案。
- **語意來自 accessibility tree，版面來自 DOM** —— screen reader 讀到的就是你看到的：role、名字、狀態；東西在頁面的哪裡來自同一棵 DOM 的版面快照，靠 node id 對接。不讀 CSS 的顏色字型。webu 不認得的 role 畫成文字加標記，從不隱藏、仍然可點。每個支援的 role 都有對著釘死版本抓的 fixture，引擎升版是一個決定而不是漂移。
- **把頁面讀成文件** —— 目錄、一節、一整張；四個 part 靠幾何切；整頁的 finder；行號；每個 history entry 記著位置。
- **item 與文字流** —— 連結、按鈕、欄位、heading、表格的格、清單裡的事、frame 是游標的停靠點；文字依欄寬在它們之間流動；表格保留欄位、每格一停、全文在 `Enter` 後面；code 保留行與語法色、`Enter` 整份開出來。
- **每一種輸入都有定義** —— 一行框說它收什麼、textarea 兩態 editor、slider 列數字、日期顏色照瀏覽器的形狀、勾選切換、select 列清單、檔案走 picker、搜尋框問要不要送出。
- **頁面自己的彈窗，浮起來** —— modal、alertdialog、menu、橫幅靠行為認，頁面疊幾層就疊幾層，回答它而不是關掉它。
- **frame，包括別站的** —— 一列，`Enter` 進去；跨站 frame 有 webu 自己的 session；巢狀一樣走得進去。
- **頁面問的事就地回答** —— `alert` / `confirm` / `prompt` 與 `beforeunload`、HTTP basic / digest auth、檔案上傳、`target=_blank` 開新分頁並切過去、憑證錯誤當成一個問題 —— 全是 webu 自己形狀的 popup。
- **兩個選單、一張表** —— `Enter` 是 item 的操作，`Space` 是 item 加 panel，每個括號裡的字母由同一張表產生、key handler 也讀同一張，所以不在選單裡的熱鍵不可能存在。
- **一列 screen 的 header** —— Web、Bookmarks（有目錄、能匯入）、History、Downloads、Settings 一排 chip，每個清單 screen 是一個面板、按鍵寫在下框。
- **終端機裡的 DevTools** —— Network 帶 request 細節與 body、Storage 可改、Console 印物件像 Chrome 那樣並能求值、頁面原始碼可 grep。
- **非 HTML 當文字回答** —— JSON、YAML、TOML、Markdown、XML 與純文字回應變成一個有語法色的 code block；PDF 明講不支援。
- **輸出 markdown** —— `Space` › `Yank markdown` 把整頁當 markdown 放進剪貼簿。
- **自己的名字** —— user agent 寫 `webu/<version>`，不是偽裝。
- **畫面穩定** —— 任何尺寸、任何內容，每一列都恰好是終端機的寬；有測試跨尺寸、面板、screen 檢查。
- **unix-first、靜態執行檔** —— macOS + Linux；`CGO_ENABLED=0`。chromedp 的 log 進檔案，永遠不進 TUI 正在畫的終端機。

## 現況

**v0.3.0** —— 頁面重新定義成一份文件；role 表補齊；每一種輸入、彈窗、frame 都處理了。見 [CHANGELOG.md](CHANGELOG.md)。

牆在哪裡，明講：網頁是二維的、終端機不是，所以一個密集的 app 頁（issue tracker 的看板、dashboard）分類是對的、但還是得在裡面移動 —— finder 與目錄是路，不是捲動。一個完全不宣告語意的站（全是 `div`、沒有 ARIA）給 webu 的只有文字和能點的東西；那種站 screen reader 也會壞，webu 不追。

還沒有的：
- **媒體** —— 圖片、影片、音訊是佔位；yank URL 到別處開
- **CAPTCHA、passkey / WebAuthn、WebRTC** —— 遇到時明講，URL 一個 Yank 就拿到。沒有第二條路：webu 是 headless 的、必須能在沒有 display 的機器上跑，沒有視窗可以交棒
- `<textarea>` 交給你自己的 `$EDITOR`（目前是內建的 editor popup）；小數 step 的 slider 只列整數
- 游標在 frame 裡時只捲 frame，不捲外面的頁
- 滑鼠、Linux ARM（沒有它的 Chromium snapshot）

## 用什麼做的

Go、[Bubble Tea](https://github.com/charmbracelet/bubbletea) 與 [Lip Gloss](https://github.com/charmbracelet/lipgloss)、浮層用 [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay)、Chrome DevTools Protocol 用 [chromedp](https://github.com/chromedp/chromedp)、語法色用 [chroma](https://github.com/alecthomas/chroma)。瀏覽器是 Chromium 專案自己的 snapshot 版，依 revision 釘死。配色 catppuccin-mocha。

## 文件

| 檔案 | 回答什麼 | 順序 |
|---|---|---|
| [`docs/function.md`](docs/function.md) | Chromium 做什麼、webu 做什麼、做到哪；翻譯層（accessibility tree + 版面快照 → 一份文件）；role 表；frame；彈窗；Chromium 怎麼取得、怎麼跑 | 第 1 |
| [`docs/ui.md`](docs/ui.md) | 版面、header 的 screen 與兩個面板、頁面的三個畫面與四個 part、每種東西怎麼畫、popup、兩套配色、存檔 | 第 2 |
| [`docs/ux.md`](docs/ux.md) | core-key 語意、每種東西的 Enter、每個焦點的 `Space` 選單、finder、每種輸入的行為、熱鍵全表、時間軸 | 第 3 |
| [`docs/webu-implementation.md`](docs/webu-implementation.md) | 實際怎麼做的、量到了什麼、踩過的坑、測試、做到哪 | — |
| [`docs/support.md`](docs/support.md) | 支援的 accessibility role，由 role 表產生 | — |

設計文件是繁體中文，每條決定都在原地標了日期。

## 開發

```
make build              → ./webu；第一次啟動會把釘死的 Chromium 下載到 cache
make test               全部測試；需要瀏覽器的只在 Chromium 下載好之後跑，否則 skip
make fixtures           用釘死的 Chromium 重抓 internal/ir 的 role fixture 與 docs/support.md
WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v     真站 smoke（Hacker News、GitHub）
make axdump URL=https://…                                任何頁面的 accessibility tree，用本機 Chrome
make gif                從 .local/demos/demo.tape 重錄 docs/demo.gif（VHS、Nerd Font、網路）
```

發布走家族的做法：推一個 `v*` tag，GitHub Actions 在兩個平台跑測試、goreleaser 打包並更新 Homebrew tap，release note 是 `CHANGELOG.md` 對應的那一節。
