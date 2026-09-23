# webu — UI

> 本文件講**版面與 surface**：面板、頁面的三個畫面、pagetab、popup、色帶、存檔。互動語意
> （core-key、Space menu 內容、hotkey）在 `ux.md`；功能與 Chromium 邊界在 `function.md`；
> 落地在 `webu-implementation.md`。依 VTP（`thoughts/tui-design`）撰寫，每條版面決定標日期。

---

## §1 版面

### 1.1 Grid

```
 [W]eb  [B]ookmarks  [H]istory  [D]ownloads  [S]ettings    1 download in flight   ← header，chip 列
────────────────────────────────────────────────────────────────────────────────  ← 分隔線；下載中兼進度條
╭ [1] Tabs ────────────────────╮╭ [2] Page ─────────────────────────────────────╮
│ ▸ <iframe> HTML inline fra…  ││ 󰖟 developer.mozilla.org/en-US/docs/Web/HTML/… │  ← URL 列
│   Hacker News                ││ 󰛼 header  󰛺 body  󰛽 others  󰛻 footer ─────── │  ← pagetab：四個 part
│   Popups                     ││  1  <iframe> HTML inline frame element 300 l… │
│                              ││  2    Try it                         󰋩 1  41 l… │  ← 目錄：一列一節
│                              ││  3      HTML Demo: <iframe>              6 l… │
│                              ││  4      OUTPUT                    󰋩 1  12 l… │
│                              ││  5    Attributes                      223 l… │
│                              ││                                               │
╰──────────────────────────────╯╰────────────────────── 1/26 · <iframe> HTML … ─╯  ← 下框 hint：在哪
 space menu   ? help   tab/1-2 panels   q quit                      ← footer
```

兩個面板並列，編號左到右。頂部一列 header：五個 screen 的 chip chain（sshu 的 tab 列作法，
chip 之間右上左下的斜線，2026-09-21），目前的點亮；右端狀態槽寫 `N download(s) in flight`；
header 下一列分隔線，下載中兼進度條。URL 在 `[2]` 內部第一列。

### 1.2 尺寸規則

| 項目 | 規則 |
|---|---|
| 側欄寬 | 固定 24 欄 |
| `[1]` 高 | header 與 footer 之間全部；分頁清單捲動 |
| 窄寬 | `w < 72` 只畫焦點那一側；`Z` zoom 讓頁面佔滿（header / footer 仍在） |
| chrome | header + 分隔線 + footer 共 3 列，鎖死不 reflow |
| 寬度穩定 | 每一列都恰好是終端機寬（`TestViewFitsTheTerminal` 跨尺寸檢查）；URL 縮 path、host 永不縮；分頁標題截斷不折行 |
| 文字欄寬 | 段落折行寬 = min(面板寬, `measure`)；`measure` 收 `full`（預設）或 ≥ 20 的數字；表格、code、分隔線用整個面板寬 |
| 行號欄 | 每個畫面都有（2026-09-23）；寬度依行數，量兩次至多（先用上一頁的位數排一次，位數變了再排一次） |

---

## §2 header 與兩個面板

### header — 全域動作的常駐揭露

`[W]eb` `[B]ookmarks` `[H]istory` `[D]ownloads` `[S]ettings`，各一個全域鍵切換 screen；header
沒有 cursor、Tab 不停。DevTools 不在這裡：它的作用對象是目前頁面，走 `[2]` 的 Space menu。

### list screen — Bookmarks / History / Downloads / Settings

一個面板佔滿 header 與 footer 之間：title chip、下邊框 hint 列該 screen 的鍵、Space menu 同一份
（item / panel 兩 region）。第一列是欄位 header（Blue）。Enter 開 bookmark / history 一律新分頁並回
`[W]eb`。Bookmarks 是目錄樹（可巢狀、可收合、可搬移、可改名、可匯入瀏覽器的匯出檔）；Settings
一列一個 key，`config.yaml` 有的 key 一定有一列（`TestSettingsCoverConfig` 守著）。細節 2026-09-21
起未變，見 `ux.md` §A.1。

### `[1]` — 分頁清單

一列一個 Chromium target：標題截斷；載入中掛 live glyph。cursor 反白 = 你在哪；綠字 = `[2]`
現在顯示的分頁；同 URL 的分頁依開啟順序編號。**Enter 才切換 `[2]`**。`target=_blank` 的新分頁加到
尾端並自動切換。

### `[2]` — 頁面（0.3.0 重新定義）

**單一職責，永遠是頁面。** 第一列 URL（前面的地球在抓取中換成圓餅填滿的動畫，只要頁面還在
長就一直轉——settling，`function.md` §6）；第二列是 **pagetab**；之後是頁面的三個畫面之一。

#### 2.1 pagetab：頁面的四個 part

一頁被幾何切成四塊（`parts.go`，2026-09-23）：**body** 是最大面積的頂層區塊、**others** 在它
旁邊（側欄、廣告欄）、**header** 在它前面、**footer** 在它後面。不看標籤叫什麼——`nav` 也好、
`div` 也好，位置決定它是什麼。pagetab 是四節的 powerline 鏈，每節一個 glyph（`page_layout_header`
/ `_body` / `_sidebar_left` / `_footer`），面板一次只顯示一個 part：

- `Esc` 上去（整條反色 rosewater、含到面板邊的那條線；目前的 part 深底 rosewater 字）、`h`/`l`
  走到哪**內容就切到哪**、`Enter` = 確認回頁面、`j` 也回頁面（2026-09-23 user 定案：走就是選）。
- **兩個守門**：頁面比視窗矮（`Page.getLayoutMetrics`）、或最大區塊不到頁面面積的 25% → 整頁一塊、
  pagetab 只畫一條線（一個 part 沒有東西可選）。
- 修訂史：2026-09-21 是「入口行」（每個 landmark 一列在頁面裡）；09-22 搬上 URL 底下成「膠囊」，
  一種 landmark 一顆、字是固定字彙（skip / header / nav / search / sidebar / footer / dialog /
  other）、只在鏈頭掛一個漢堡；09-23 改成**四個 part 由幾何切**，理由：沒宣告 landmark 的站（w3schools）
  什麼都收不到、宣告了的站字又跟 URL 重複，而「東西在哪」是每個站都有的事實。膠囊、Outline popup、
  `other` 膠囊、skip 膠囊全部拿掉；skip link 與頁內錨點仍直接跳。

#### 2.2 三個畫面：目錄、一節、一整張

有 ≥ 3 個標題的頁面是**文件**（`shapeDoc`），開頁先進**目錄**（section list，`section.go`）：

| 畫面 | 內容 | 鍵 |
|---|---|---|
| **目錄** | 一列一節：序號（就是 `go` 的行號）、依深度縮排的標題、右欄 `󰓫 N`（表）`󰅩 N`（code）`󰋩 N`（媒體）與行數；標題用該深度的 ink（五個 hue 輪流，`levelColor`），游標列用同色反白 | `j`/`k`/`u`/`d`/`gg`/`G` 走、Enter 開一節、Esc 上 pagetab |
| **一節** | 面板 header 列變成 `☰ 2/26 節名`（該深度的 ink），內容從節的本文開始（標題不再印第二次）；**一節包含它的子節**（2026-09-23：MDN 的「Try it」是 h2、內容全在它的 h4 底下，不含子節就是空的；章包含節，h1 = 整篇），下框 hint `2/26 · 節名 · 40%`，下框本身填成進度條 | `n`/`p` 同深度的下一節 / 上一節、Esc 回目錄 |
| **一整張** | 從頭到尾一張紙 | Space menu 的 `One sheet` / `Sections` 切換 |

不到三個標題、表單、app、搜尋結果：`shapeOne`，就是一整張。沒有內文只有連結的「節」（w3schools 的
側欄目錄）不列進目錄、行數併進前一節（`pruneNav`）。

#### 2.3 頁面怎麼畫

- **item 與 flow**：cursor 只停在 item（可按的、可填的、heading、landmark 標題列、清單裡的一件事、
  frame、code block、資料表的每一格）；文字段落是 item 之間的 flow。`j`/`k` 換列，`h`/`l` 同列的
  item 之間走（一列連結才走得動）。
- **顏色是概念不是元素**（2026-09-23 定案）：**可按的**（link、button、tab…）sapphire、**可填的**
  （每種 input）mauve、**code** pink、**媒體** overlay0、**pagetab** rosewater、**填錯的值** 紅；
  heading 與目錄用五個 hue 輪流表深度。不再有 `[ xxx ]` 括號、不用 emoji、每個控制項以 glyph 開頭；
  glyph 一律從字型的 cmap 讀碼位、不憑記憶。
- **heading**：`#`×層級 + 文字，用該深度的 ink（2026-09-23 改：09-22 曾是六級底色從 surface2
  lerp 到 crust，改成五個 hue 後底色拿掉）；Enter 收合到下一個同級或更高級 heading。
- **清單裡的一件事**（listitem、article、card）**一列**（2026-09-23）：第一行當它的名字，Enter 走進去
  （面板 header 列變成它的第一行）、Esc 出來、路徑無上限；只有一行的不走進去，Enter 直接對裡面的
  東西做事。理由：一張卡片攤成十幾列沒有標籤的文字，是一件看不見的事。
- **表單畫成表單**（2026-09-22 / 23）：`<form>` 是 sshu 的 popup 框畫進頁面裡（不是浮在上面）：label
  一欄對齊、值靠一邊、值前面是欄位 glyph；label **不截斷**——擠不下就 stack 成兩列；`<fieldset>`
  的 legend 是粗體分組標題；`aria-invalid` 的值畫紅；必填在 label 上標 `*`。
- **資料表**：整表 surface0 底、表頭列 surface1 底 + 粗體，每格一停，長了 Enter 看全文。沒有表頭的
  是排版表，一列一行流式。
- **code block**：pink 字、surface0 底、保留行、chroma 語法色；是一個可停的區塊，Enter 開全文 popup。
- **tabs**（tablist）：一條 pagetab 式的鏈，選中的 tab 用所在節的 ink 填底、其餘描字；只有選中的
  panel 在樹裡，所以畫面就只顯示那一個。
- **tree**：依 `aria-level` 縮排，`▾`/`▸` 依 `aria-expanded`；**listbox** 的 option 是 radio（單選）
  或 check（多選）列；**menu** 的 menuitem 是按鈕列，從按鈕開出來的 menu 是彈窗。
- **slider**：`Red ──────●───── 128`（12 格軌道）；**progressbar / meter**：`Upload ━━━━━━────── 50%`
  （0..1 顯示百分比、其他 `v/max`、沒值 `…`），不是停靠點。
- **`<details>`**：`▸ Show more` / `▾ Already open`，Enter 開合。**tooltip**：`󰋼 文字` dim 旁白，只在
  頁面顯示它時出現（Enter 就是 hover）。**timer / status**：文字在句子裡流動、跟著跳。
- **frame**：`󰘔 名字` 一列，Enter 走進去（同站、跨站、巢狀都一樣；進不去的說進不去 + Yank url）。
- **裸連結串流動**：連續兩個以上、裡面沒有區塊的 block 連結流成一段（一列一個是 CSS，不是結構）。
- **同一個名字只印一次**（2026-09-23）：region 的名字 = 裡面第一個標題 → 不畫 rule；走進一件事時
  第一行是 header 列不重印；空的節只剩標題列。
- **inline 標記**：`<strong>` 粗、`<em>` 斜、`<del>` 刪除線、`<ins>` 底線、`<mark>` 反白。
- **markdown**：Space menu 的 `Yank markdown` 把整頁寫成 markdown 進剪貼簿。
- **sr-only 丟掉**：排在 1×1 點上的元素是寫給 screen reader 的（2026-09-22）。
- **非 HTML**：JSON / 純文字 / XML 整份一個 code block；PDF 是「unsupported media type」一頁。
- **起點**：main 裡第一個 item、視窗捲到它；沒 main 停最上層 heading。載入中 URL 列的 glyph 轉、
  頁面不 dim（dim 只給「正在離開的頁面」）。

#### 2.4 頁面自己的彈窗：浮在頁面上

頁面跳出的 modal / alertdialog / menu / cookie 橫幅（`function.md` §4.3 的判定）畫成 webu 自己的
popup 框浮在 `[2]` 上（`pagepopup.go`，2026-09-23）：對話框的寬度（`max(20, min(panelW−8, 76))`）、
底下的頁面 dim 當 backdrop（原生 `showModal` 會砍掉 dialog 以外整棵樹，所以用彈窗出現前那刻的
版面）、疊層往右下錯開 3 欄 1 列。裡面沒有行號、沒有 `go`、沒有 `/`（它要的是一個回答，不是
一頁）；`Esc` 不關；答完回到原本的節與位置。修訂：第一版把彈窗當成整個面板，user 2026-09-23
打回：「popup 變成一頁，不是 popup」。

---

## §3 Popup

全部走 u-family 的 Popup Convention：一個 popup 一個檔一個 animator、title = glyph + 文字、hint
嵌下邊框、`Esc` 只在 `closeTop` 一處解析、`Space` 在浮層上 = 關掉它、動畫 8 × 16 ms。

| Popup | 類型 | 用途 |
|---|---|---|
| Space menu | menu | item / panel 兩 region；捲動、環繞、`u`/`d` 半窗、`gg`/`G` 首尾（2026-09-23） |
| `?` help | viewport | 全域動作表 |
| Location | input | `L`：目前 URL 當提議（Tab 接手、Backspace 清掉）；非 URL 當搜尋 |
| input | input | 一行的欄位：**邊框寫型別**（`email`、`number`、`date · YYYY-MM-DD`、`password`、`email · invalid`），**框裡一行是欄位名**（2026-09-23 user 定：邊框是 chrome 說這是哪種框，框內那行說是哪一個欄位）；JS `prompt`；HTTP auth（遮罩）；設定值 |
| editor | 大框 | textarea / contenteditable：多行、**寫 / 移兩態**（Esc 出到框層 hjkl 走、`i`/`a`/`A`/`o` 回寫、Enter 設值、再 Esc 取消） |
| **finder** | 三區（輸入 / 清單 / 預覽，filu 的 `/` 形式；寬 ≥ 96 欄並排、否則上下） | `/`：整頁四個 part 的節點索引（「持有文字的最小區塊」各一次），字面比對優先、名字模糊比對；清單列前綴 part 的 glyph；打字時 Enter 進清單，清單上 Enter = **去那裡**（切 part、開節、沿路走進去、游標落定），不按 |
| go | 清單 | `go` 後打數字：只管**行號**（目錄上是節的序號）；數字當前綴篩 |
| options | menu | `<select>` 的 option；slider 的數字（10 列一窗、游標在目前值置中）；書籤搬移的目錄 picker |
| file picker | menu（filu finder 形式） | `<input type=file>`、書籤匯入：一次一個目錄，Enter 進目錄 / 選檔，空過濾時 Backspace 回上層 |
| confirm | message | 連結 Open、搜尋框 Search、憑證、`beforeunload`、JS confirm、刪除 / 清除 |
| message | viewport | 表格格內容、code block 全文、Inspect、visual mode cheatsheet |
| page popup | 浮窗 | 頁面自己的彈窗（§2.4） |
| DevTools | 帶 tab bar 的 viewport | Network / Storage / Console / Source；`h`/`l` 切分頁 |
| toast | message | 下載、yank、拒絕的操作（disabled、popup 裡的 `/`）… |

DevTools 的四個分頁與 2026-09-21 相同：Network（清單 + detail：headers / body）、Storage（cookie /
local / session：刪、yank、清站資料）、Console（每筆完整折行、物件列 own property、`i` REPL）、
Source（HTML + `/` grep）。實作註記在 `webu-implementation.md`。

---

## §4 色帶

**兩套配色（2026-09-22 定案）**：**app 配色**是 webu 的外殼（面板、邊框、header、pagetab、游標、
選單、popup、footer），完整吃 VTP——少數錨點、明度當 z-axis、一個語意一條色帶；**page 配色**是文件
自己的結構，封閉集合，只出現在 `[2]` 的內容區、不參與 z-axis。錨點 catppuccin-mocha。

| 色帶 | 意思 | 值 |
|---|---|---|
| Blue | focus（結構）、行號、URL、抓取中的 glyph | `#89b4fa` |
| Surface2 | unfocused | `#585b70` |
| Green | `[1]` 裡 `[2]` 正在顯示的分頁（全 app 只表達這一件事） | `#a6e3a1` |
| Mauve | header 鏈；page 側：**可填的** | `#cba6f7` |
| Rosewater | pagetab（Esc 上去時整條反色） | `#f5e0dc` |
| Sapphire | page 側：**可按的**（連結底線） | `#74c7ec` |
| Pink | code；code block 另鋪 surface0 底 | `#f5c2e7` |
| Overlay0 | 媒體、frame、dim 旁白 | `#6c7086` |
| Red（warn） | 填錯的值（`aria-invalid`）；override 色不參與層級 | — |
| 五個 hue 輪流 | 標題深度：目錄的縮排、節的 header 列、heading、tab 鏈、游標在標題上時的底 | `levelInk` |
| Lavender | visual mode 的選取 | `#b4befe` |
| Surface0 / Surface1 | 資料表底 / 表頭底 | `#313244` / `#45475a` |
| popup layer scale | 浮層層級，依 §2.5 插值 | — |
| 語法色 | key Mauve、string Pink、number Flamingo、常數 Sky、註解 dim 斜體、標點 Overlay2 | — |

focus 二態同 kbu §8.4：雙線 `╔═╗` + Blue ↔ 圓角細線 `╭─╮` + Surface2，零位移。visual mode 邊框 Yellow。

---

## §5 Chrome 三件套

| 件 | webu |
|---|---|
| Border title chip | `[1] Tabs`、`[2] Page`；讀一節 / 走進一件事時 `[2]` 的第二列變成 `☰ 2/26 節名` 或那件事的第一行 |
| Panel tab bar | 面板都沒有；tab bar 只出現在 DevTools popup |
| Border hint | `[2]` 下框：在哪（`2/26 · 節名 · 40%`、part 名）、loading；讀一節時下框本身是進度條 |

footer：`space menu   ? help   tab/1-2 panels   q quit`；list screen 上 `space menu   ? help   esc web   q quit`。

`docs/icon.svg` 是 u-family mark 的 webu 版；`V` 觸發 splash 彩蛋（家族同鍵，visual mode 因此是小寫 `v`）。
**Nerd Font 是設計、必裝**：role glyph、part glyph、spinner、powerline 鏈都靠它，排版量它的寬。

---

## §6 存檔

| 資料 | 形式 | 位置 |
|---|---|---|
| 設定 | `config.yaml`：`search_engine`（預設 DuckDuckGo html）、`download_dir`、`measure`（`full` 或數字）、`restore_session` | `~/.config/webu`（`$XDG_CONFIG_HOME/webu`、`$WEBU_CONFIG`） |
| 書籤 | flat yaml：`bookmarks:`（各帶 `folder`）+ `folders:` | 同上 |
| 歷史 | YAML sequence，一次 append 一筆；無限保留 | `~/.webu/datas`（`$WEBU_DATA`） |
| session | 離開時寫下所有分頁 | 同上 |
| 下載檔案 | `download_dir`，預設 `~/.webu/datas/downloads` | 同上 |
| profile / log | Chromium user-data-dir；chromedp 的 log（絕不進終端機） | 同上 |
| Chromium | 釘死 revision，可重新下載 | macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`（`$WEBU_CACHE`） |

分法：**使用者寫的**在 config、**webu 產生的**在 data、**可重抓的**在 cache。每次寫檔原子。
