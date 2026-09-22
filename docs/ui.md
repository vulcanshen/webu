# webu — UI

> 本文件講**版面與 surface**：面板、popup、色帶、chrome 三件套、存檔。互動語意
> （core-key、Space menu 內容、hotkey 字母、導覽詞彙）在 `ux.md`。功能與 Chromium
> 邊界在 `function.md`。
>
> 依 VTP（`thoughts/tui-design`）與 kbu / filu / sshu 的 implementation doc 對照撰寫；
> 每條版面決定都標前例。

---

## §1 版面

### 1.1 Grid

```
 [W]eb  [B]ookmarks  [H]istory  [D]ownloads  [S]ettings    1 download in flight   ← header，chip 列
────────────────────────────────────────────────────────────────────────────────  ← 分隔線；下載中兼進度條
╭ [1] Tabs ────────────────────╮╭ [2] Page ─────────────────────────────────────╮
│ ▸ chromedp/chromedp · GitHub ││ 󰖟 github.com/chromedp/chromedp    ← 第一列 URL │
│   Hacker News                ││ ───────────────────────────────────────────── │
│   MDN: ARIA roles            ││ # chromedp                                    │
│                              ││ 󰌷 Code · 󰌷 Issues 42 · 󰌷 Pull requests 3     │
│                              ││ [ Go to file ]  [ Code ]                      │
│                              ││                                               │
│                              ││ A faster, simpler way to drive browsers …     │
│                              ││                                               │
╰──────────────────────────────╯╰───────────────────────────────────────────────╯
 space menu   ? help   tab/1-2 panels   q quit                      ← footer
```

兩個面板並列，編號左到右（kbu / filu 慣例）。頂部**一列 header**（修訂 2026-09-20，sshu 最上列
`[M]anage / [F]ile transfer / [S]SH` 的 chip chain，chip 之間的分隔改成右上左下的斜線：亮 chip 的色塊用 `U+E0BC`（左上實心三角）收邊、色塊本身跟著斜，其餘 chip 之間畫細斜線 `U+E0BB`；不用 sshu 的實心右三角，修訂 2026-09-21）：`[W]eb` `[B]ookmarks` `[H]istory` `[D]ownloads` `[S]ettings` 五個 screen 的 chip，
目前的那個點亮（修訂 2026-09-21：list 從 popup 改成 screen——`[W]eb` 是下圖左右並列的 Tabs / Page，其他四個各佔滿整個 body）；右端是狀態槽，有下載進行中時寫 `N download(s) in flight`（live 綠）。原本的 `[1]` Places 面板
內容永遠是三項、做成面板是浪費，併進 header 之後 Tabs 與 Page 並列。URL 收進 `[2]` 內部第一列（filu breadcrumb / sshu cwd 的做法）。

### 1.2 尺寸規則

| 項目 | 規則 | 前例 |
|---|---|---|
| 側欄寬 | 固定 **24 欄**（修訂 2026-09-20：28 是圖，24 是內容需要的） | sshu 固定左欄（manage 18 / ssh 30） |
| `[1]` 高 | header 與 footer 之間全部；分頁數不設上限，清單捲動 | — |
| `[2]` | 右側全部 | — |
| 窄寬 | `w < 72` 只畫**焦點那一側**：焦點在 `[2]` 畫頁面，`Tab` / `1` 到 Tabs 就改畫 Tabs；`Z` zoom 讓頁面佔滿（header / footer 仍在） | sshu §1.2 窄寬只畫 focus 側 |
| header + 分隔線 + footer | 各 1 列，共 3 列 chrome，鎖死不 reflow（修訂 2026-09-20：分隔線是 sshu `tabRule`，讓 chip 列與面板 title chip 不會讀成同一排按鈕；有下載進行中時從左端以 live 綠填到混合百分比，是最薄的進度條） | 通用 §1.3 |
| 寬度穩定 | 邊框 chip 文字固定；URL 用 filu `fitPathSegments` 縮，host 永不縮；分頁標題截斷不折行 | 通用 §1.2 |

---

## §2 header 與兩個面板的職責

### header — 全域動作的常駐揭露

一列 chip chain：**`[W]eb`、`[B]ookmarks`、`[H]istory`、`[D]ownloads`、`[S]ettings`**（修訂 2026-09-20：原本是 `[1]` Places 面板，
三列 Bookmarks / Shortcuts / History。內容永遠三項、做成面板是浪費，改成 sshu 最上列
`[M]anage / [F]ile transfer / [S]SH` 那種 chip 列。Shortcuts 一併拿掉——有 Bookmarks、goto popup 又帶目前 URL
當 placeholder，它沒有自己的理由；Downloads 補上，就是原本排到 v2 的下載清單）。每個 chip 一個全域鍵切換 screen（修訂 2026-09-21：原本 B / H / D 開 popup，
現在各自是佔滿 body 的 screen，`[W]eb` 回到 Tabs / Page，`[S]ettings` 新增）；目前的 chip 點亮（sshu active tab 的畫法），header 沒有 cursor、Tab 不停在這一列。
右端是狀態槽：有下載進行中時顯示 `N download(s) in flight`（live 綠），否則空。header 下面一列分隔線（sshu `tabRule`），下載中兼作進度條。

Outline 與 DevTools **不在這裡**（決定 2026-09-20）：它們的作用對象是目前頁面，是 contextual
動作，走 `[2]` 的 Space menu panel operation（`ux.md` §A.1）。使用者體驗留在頁面上、不切面板。

VTP 定位：五個都是 non-contextual 動作（沒有作用對象），依 §A.2 走 `?` 入口 +
letter hotkey；header 就是 kbu statusbar chip 那種 Layer 2 ambient 揭露。

### list screen — Bookmarks / History / Downloads / Settings

一個面板佔滿 header 與 footer 之間（`listpanel.go`）：title chip = glyph + 名字，邊框 hint 列該 screen 的鍵
（Enter / `x` / `y` / `A` / `C` / `/` / Esc），Space menu 是同一份（item / panel 兩 region）。
**第一列是欄位 header**（Blue：Title / URL、Title / When URL、File / Progress、Setting / Value——它是 chrome 在命名欄位，
與 footer 命名按鍵同一個色帶；修訂 2026-09-21，原本 dim），游標從第二列起，不貼著 title chip。列本身：title 文字色、其後的 URL / 路徑 / 值 dim。
Enter 開 bookmark / history **一律新分頁**並回 `[W]eb`，不佔原分頁（修訂 2026-09-21）。空狀態走 sshu empty.go 形狀。
Bookmarks 依目錄分組（2026-09-21）：根層在前，之後每個目錄一列 `󰉋 name`（可停游標，像 filu 的目錄列），其下的書籤再縮排；
目錄**可以巢狀**（路徑 `dev/go`，樹狀縮排，父目錄緊接著子目錄）：`[A]dd folder` 在游標所在目錄下開新目錄（游標在目錄列上就開在它裡面、
在書籤上就開在同一層；路徑 `a/b/c` 一次開三層）；`[a]dd` 在同一處加書籤（URL、title 兩個 input popup，正在顯示的頁面當 placeholder，
Enter 直接收下）；目錄列 **Enter = 展開 / 收合**（收起來一列 `▸ 󰉋 dev · 3 bookmarks`，狀態只在 session 內）；`[m]` 搬移
（options popup 當 picker：`(no folder)` + 目錄樹，縮排表階層，數字鍵選，游標跟著搬過去）；目錄列上 `x` 只刪空目錄
（沒有書籤、沒有子目錄），而且只刪那一層——上層不會因為變空而跟著消失（`bookmarks.yaml` 的 `folders:` 寫下每一層，含路徑帶出來的）；`/` 過濾時攤平只列命中的書籤。修訂 2026-09-21：原 `[F]` / `[f]` / `[A] Add this page` 拿掉。
Settings：一列一個 key（依 `config.yaml` 的順序：`search_engine`、`download_dir`、`measure`、`restore_session`），右欄是生效的值加一句用途。文字設定 Enter → input popup，**目前生效的值當 placeholder**
（同 Location：Tab 接手、Backspace 清掉；沒動過就 Enter 不改，清空後 Enter = 預設）；開關 Enter 直接切換。都寫回 `config.yaml`、立即生效
（`download_dir` 重新 `setDownloadBehavior`；`measure` 立刻重排所有分頁；`search_engine` 下一次搜尋生效；`restore_session` 下次啟動生效）。收不下的值（`measure` 不是數字、`search_engine` 沒有 scheme）toast 說原因、框留著。
footer 在 screen 上是 `space menu   ? help   esc web   q quit`。

### `[1]` — 分頁清單

一列一個 Chromium target：標題截斷；載入中掛 live glyph（kbu Logs 的 `U+F0753`）。

兩個獨立的視覺狀態：

| 狀態 | 意思 | 呈現 |
|---|---|---|
| cursor 反白 | 你在哪 | 反白列 |
| 綠字 | `[2]` 現在顯示的是哪個分頁 | 文字 Green |

**Enter 才切換 `[2]`**，cursor 移動不切換。任何時候恰有一列是綠的（沒有分頁時零列）。

`target=_blank` 產生的新分頁加到清單尾端並**自動切換**（`[2]` 顯示它、綠字移過去），同 Chrome。

### `[2]` — 頁面

**單一職責，永遠是頁面。** 邊框 chip 固定 `[2]`，不放模式名、不放頁標題（標題在 `[1]`，
一個元素一個語意 §B）。

- 第一列：URL（純文字、不可 focus、縮法見 §1.2）；第二列分隔線 = **pagetab**（user 命名 2026-09-22），頁面的 chrome 以一條 powerline 鏈放在這條線上（見下）；之後是 IR 渲染的頁面
- cursor 只停在 item（互動節點 + heading + landmark 的標題列），文字段落是 item 之間的 flow
- **Heading 收合（2026-09-21）**：是 item 的 heading（裡面沒有 link 等 item 的）Enter 直接 Collapse / Expand，範圍到下一個同級或更高級 heading、或所在 landmark 結束為止；收合列 `▸ # 標題 · N items`；Outline 跳進被收合的區段會先展開。用詞與 landmark 列、Bookmarks 目錄列一致：Collapse / Expand
- **Landmark（修訂 2026-09-20）**：每個 landmark 以一列帶名字的細線開頭（`▾ navigation Repository ────`），
  是 item，**Enter 直接開合**（修訂 2026-09-21：原本開 item menu 選 Collapse / Expand；Space menu 仍列這兩項）。
  **全部預設展開**（修訂 2026-09-21：原本有 `main` 時 main 之外的預設摺疊，實機看了像頁面壞掉）；收合的畫成
  `▸ banner · 14 items`，Outline 跳進摺疊的 landmark 會先把它打開
- **page chrome 一種一節，併成一條鏈放在 URL 底下的 pagetab 上**（2026-09-22 從頁面裡的一行搬上去、按種類收、頁面本體從內容開始；2026-09-21：banner / navigation / breadcrumb / search / complementary / contentinfo 一套 style，取代原本的「導覽清單一行流式」與這些 landmark 的分隔線）：`字 +N`——字是種類固定字彙 skip / header / nav / search / sidebar / footer（dialog 用名字、other = 有 main 時 main 之外不在 landmark 裡的東西）、**icon 只在整條鏈的開頭掛一個漢堡**（`withLead`）、順序固定；「你在哪一格」在面板下框 hint；main 不再畫 rule（chrome 都走了，main 就是頁面的起點）；`sectionheader` / `sectionfooter` 當 Group；skip link 的字也認 jump to。原本的——icon 分種類（page_layout_header / menu / map_marker_path / search / table_of_contents / page_layout_footer）；字是你在裡面的哪一格（navigation 目前的 tab、breadcrumb 最後一節）或它是什麼（banner 站名、search 框名、其他 landmark 名字或第一個 heading）；**一條 powerline 鏈**（`pagetabChain`，與 header `[W]eb` 那條同一套幾何：`capLeft` 起、相鄰 `divider`（同色細斜線 / 異色實心斜接縫）、`capRight` 收）、未點亮的節：底色維持畫布色（不給整條自己的底，才不會讀成橫貫面板的色帶）、字用 Mauve（2026-09-22 user 定：一眼看出這列是頁面的外框、不是內文第一行；Mauve 原本是表格表頭的字色，表頭改靠底色後空出來，現在 code block 的 key 與 pagetab 共用）、hand 停的那一節用游標色（handColor）點亮、面板沒 focus 時 surface2；固定順序、塞不下的收成最後一顆 `+N`，Enter 列出、選一顆就把 hand 放上去（暫佔最後一格）並開它的清單；`Esc`（沒有 popup / mode 時）上 tray、再 `Esc` 回頁面；頁面最上列按 `k` 也上得去、`h`/`l` 走（會繞）、`j` 回頁面、其他移動鍵先回頁面；hand 在 tray 上時頁面游標降成未 focus 色，說 `j` 會回哪裡；Enter 清單：多個 landmark 時每個一段 header 列、aria-current 的列 hint 標 here；沒目標的那一節 Enter 看全文；內容不畫，Enter 開 item operation 選單列出裡面的 link / button / 輸入框 / 勾選 / select；search 只有一個框時 Enter 直接開框；skip link 進 skip 那一節、Enter 跳到內文。main / article / region / form 是內容，維持分隔線 + 可收合
- **一串裸連結流動排版**（2026-09-22）：連續兩個以上、裡面沒有區塊的 block 連結不各佔一列，流成一段（`linkRun` / `bareBlockLink`）。一列一個是 CSS `display:block`，不是頁面的結構
- **新頁起點**（2026-09-22 改）：游標到 main 裡第一個 item、**視窗也捲到那一列**；沒有 main 的頁停在最上層的 heading，兩者皆無才停第一列
- **文字欄寬上限**：段落折行寬度 = min(面板寬, `config.yaml` 的 `measure`，預設 100)；表格、code、分隔線仍用整個面板寬
- **非 HTML 的回應**（修訂 2026-09-20）：JSON / 純文字 / XML / CSV 等依 `document.contentType` 整份畫成一個 code block，
  JSON 自動縮排；不畫 Chrome 自己的 JSON viewer（Pretty-print 表單）
- **新頁載入**：游標回 main 裡第一個 item、**視窗捲到它**（修訂 2026-09-22，原本停頁頂）；不跟著游標捲（同頁重畫才留位）
- 空狀態（沒有分頁）：置中事實「no page」+ 提示（該按什麼），sshu `empty.go` 的形狀
- 載入中：邊框 hint 顯示 loading；即時更新重畫時 cursor 留位（function.md §4 / §6）

---

## §3 Popup

兩個 popup 由 `[2]` 的 Space menu panel operation 開（Outline / DevTools；修訂 2026-09-21：Bookmarks / History /
Downloads 不再是 popup，是 header 的 screen，見 §2）；加上既有的 Space menu / help / confirm / input / form / toast。
全部走 §6 Popup Convention：一個 popup 一個檔一個 PopupAnimator、title = glyph + 文字、
hint 嵌下邊框、`Esc` 只在一處解析。

### 3.1 screen 與 popup

| Popup | 類型 | 內容 | item operation | panel operation | 子 popup |
|---|---|---|---|---|---|
| **Bookmarks**（screen） | 單一面板 | 根層 + 每目錄一組（目錄列可停游標；樹狀摺疊 v2） | Enter 開**新分頁**（2026-09-21）、Rename（title；目錄列改名連底下一起搬）、Delete、Move（picker）、Yank url | Add 目前頁、New folder、Import、`/` 搜尋 | Rename → input；Delete → confirm；Move → 目錄 picker；Import → file picker → input（根目錄名） |
| **Downloads**（screen） | 單一面板 | 本次 session 的下載，新的在上：檔名 + 右欄是進度（`42%  1.2 MB of 3.0 MB`）、落地路徑、或 cancelled | Enter 用系統開啟器開檔、Open source in new tab、Remove（進行中先 `Browser.cancelDownload`；檔案不動）、Yank path | Clear 已完成 / 取消的 | — |
| **History**（screen） | 單一面板，filu 原生 finder 形式 | 時間倒序、打字即 fuzzy、串流載入 | Enter 開**新分頁**、Delete 該筆、Yank url | Clear | Clear → confirm |
| **Settings**（screen） | 單一面板 | 一列一個 key，目前 `download_dir`（空 = 預設） | Enter → input popup 改值 | — | input |
| **DevTools** | viewport，**帶 tab bar**（見 3.2） | Network / Storage / Console / Source 四分頁 | 依分頁 | 依分頁 | Network detail |
| **Outline** | menu | 目前頁的 landmark + heading，縮排表層級 | Enter：`[2]` cursor 跳到該節點 | — | — |

Shortcuts popup 拿掉了（2026-09-20）：它與 Bookmarks 形狀相近、只差「少量常用」這個 mindset，
而 goto popup 帶目前 URL 當 placeholder、Bookmarks 有 `/` 過濾之後，這個 mindset 不需要自己的入口。

Popup 疊 popup 的結構走 kbu §6.0.4：子 popup 獨立檔、獨立 animator，parent 在 `Open()`
reset 子 popup、route tick 與 update。

### 3.2 DevTools popup

u-family 第一個帶 tab bar 的 popup：kbu §8.2 的 starship chip chain 搬到 popup title 列，
`h/l` 切分頁，順序 Network / Storage / Console / Source、開啟停在 Network（修訂 2026-09-21：原 Storage 在前）。外殼一檔一 animator，四個分頁各一檔一 sub-model。尺寸接近全螢幕、無 padRow
（Network 欄位吃寬度）。

```
╭─ 󰙨 DevTools  Network │ Storage │ Console │ Source ─────────────────────────╮
│ github.com                                                                 │
│ cookies · 9                                                                │
│ ▸ logged_in     yes         .github.com  /  2027-09-20  HttpOnly Secure    │
│   _gh_sess      eyJhbGci…   github.com   /  session     HttpOnly Secure    │
│ local storage · 3                                                          │
│   theme         dark                                                       │
│ session storage · 0                                                        │
╰─ [D]elete  [Y]ank value  [C]lear site data  /: filter  h/l: tab ──────────╯
```

| 分頁 | 資料來源（CDP） | 顯示 | 動作 | v1 |
|---|---|---|---|---|
| **Storage** | cookies `Network.getCookies(url)`；local / session `DOMStorage.getDOMStorageItems` | 三個 section 各一張表；cookie 欄位 name、value 截斷、domain、path、expires、flags | Delete 該筆（`Network.deleteCookies` / `DOMStorage.removeDOMStorageItem`）、Yank value、Clear site data（`Storage.clearDataForOrigin`）、`/` 過濾 | **完整** |
| **Network** | `Network.enable` 後 `requestWillBeSent` / `responseReceived` / `loadingFinished` / `loadingFailed`；WebSocket frame 事件 | 一列一 request：method、status、type、URL 縮短、size、耗時 | Enter → detail 子 popup（headers + body，`Network.getResponseBody`）、`/` 過濾、Clear | 清單 + detail |
| **Console** | `Runtime.consoleAPICalled`、`Runtime.exceptionThrown`、`Log.entryAdded` | viewport，等級 glyph，warn / error 用 override 色；**每筆完整顯示、折行**（修訂 2026-09-21：原本一筆一列截斷加 `…`，stack trace 與多行 log 看不到；物件參數印 preview `{a: 1, b: Array(3)}`，原本只印 `Object`；Enter 的 detail 對物件列出**全部** own property（`Runtime.callFunctionOn` 在頁面裡走一遍 `getOwnPropertyNames`、getter 取值，一列一個 `name: value`），`window` 也列得完） | Enter → 該筆完整內容（detail 子 popup，訊息折行）、`[i]nsert` → Eval 輸入列（`Runtime.evaluate`，REPL；結果以 reference + preview 取回：plain object / array 印 JSON，其餘印 `Window {window: Window, …}`——修訂 2026-09-21，原本 `returnByValue` 讓 `window` 回「Object reference chain is too long」）、`/` 過濾、Clear | **完整**（修訂 2026-09-20：Eval 提前到 v1；Enter 給 detail，因為清單裡長訊息被截斷看不到） |
| **Source** | `DOM.getOuterHTML` | viewport，行號 | `/` grep（只留含關鍵字的行） | **完整**（修訂 2026-09-20：原本是 `[2]` 的 `[V] View source`，搬進來把 `V` 讓給 visual mode） |

實作註記：
- `Network.enable` 要在導航前開，否則抓不到第一波 request → 每個 target attach 時就開；
  每 target 一個 ring buffer（上限 500 筆）壓住記憶體
- 換頁清 console 與 network（同 Chrome 預設）；「preserve log」開關 v2
- IndexedDB、Cache Storage、Service Worker 不做

### 3.3 其他 popup（沿用 u-family）

| Popup | 類型 | webu 用途 |
|---|---|---|
| Space menu | menu | item / panel 兩 region，內容在 `ux.md` |
| `?` help | viewport | 全域動作表 |
| goto | menu（filu goto picker 形式） | 輸入 URL / 從 Bookmarks / History 挑 |
| file picker | menu（filu finder 形式，sshu 的 identity picker） | Bookmarks 的 Import 選瀏覽器匯出檔（2026-09-21） |
| confirm | message | 憑證錯誤「要繼續嗎」、`beforeunload`、JS `confirm`、Delete / Clear、link 的 Enter「Open link」（連結文字 + URL，2026-09-21） |
| input | input | 單行 textbox 編輯、JS `prompt`、HTTP auth（遮罩，同 sshu askpass：從所有浮層拿走鍵盤） |
| form | form | `<form>` 整張填寫、Bookmark edit |
| select options | menu | `<select>` 的 option 清單 |
| toast | message | 下載開始 / 完成、yank 回饋 |
| file picker | menu（sshu filepicker） | 檔案上傳 |
| pty | pty | textarea 的 Edit in `$EDITOR` |

---

## §4 色帶

錨點沿用 catppuccin-mocha（filu §2.1 / kbu §2）。§2.3 專職化：一條色帶一個意思。

| 色帶 | 意思 | 值 | 備註 |
|---|---|---|---|
| Blue | focus（結構色） | `#89b4fa` | 同 kbu §8.0，邊框 + chip |
| Surface2 | unfocused | `#585b70` | 同 kbu |
| **Green** | **`[2]` 正在顯示的分頁**（`[1]` 專用） | `#a6e3a1` | 全 app 只表達這一件事，不拿去表達成功 / 線上 |
| popup layer scale | 浮層層級 | 依 §2.5 插值 | 同 kbu |
| override | warn / error（console 等級、憑證錯誤） | 同 kbu §2.4 | 不參與 z-axis |
| link 色 | `[2]` 內 link | Sapphire `#74c7ec` + 底線 | 定案 2026-09-20（先 Teal、看過後改 Sapphire）；底線讓色彩被壓平的終端機仍看得出是連結 |
| code | inline code 與 code block | Pink `#f5c2e7`；block 另鋪 Surface0 `#313244` 底、寬到 measure | 不用 Peach：Peach 是 override 的「值得注意」（console warn、4xx），code 不是警告 |
| table | 資料表整列 | 底 surface0 `#313244`；表頭列底 surface1 `#45475a`、文字 Text `#cdd6f4` 粗體——表頭靠底色分、不靠前景色（原 Mauve 文字，2026-09-20 定、2026-09-21 改） | 修訂 2026-09-21 |
| visual mode 選取 | 掃過的字元 | Lavender `#b4befe` 底、Base 字 | 與「正在改的欄位」同一條帶：選取中的文字就是正在動的東西 |
| code 內的語法色 | JSON / YAML / TOML / Markdown / XML / JS / CSS 文件（依 content type，chroma 切 token） | key Mauve（與表頭同義：值的名字）、string Pink、number Flamingo `#f2cdcd`、常數與關鍵字 Sky、註解 dim 斜體、標點 Overlay2 `#9399b2`；Markdown 標題 / 粗體 / 斜體用字重 | 修訂 2026-09-20；只分「不同種類的東西」，避開 app 保留的 Green / Yellow / Red / Peach |

focus 二態同 kbu §8.4：雙線 `╔═╗` + Blue ↔ 圓角細線 `╭─╮` + Surface2，cell 數相同、零位移。

---

## §5 Chrome 三件套（kbu §8 照搬）

| 件 | webu |
|---|---|
| Border title chip | `[1] Tabs`、`[2] Page`（修訂 2026-09-20：先是三個面板補齊 label，再把 `[1] Places` 併進 header 後重新編號）。文字固定 |
| Panel tab bar | 面板都沒有；tab bar 只出現在 DevTools popup |
| Border hint | 只承載 tab-contextual 鍵（§6.6.2）。`[2]` 的 loading 狀態、`X of Y` 捲動指示放這裡 |

footer 一列：`space menu   ? help   tab/1-2 panels   q quit`，五個 core-key 全在此揭露；list screen 上是 `space menu   ? help   esc web   q quit`。

### 5.1 icon 與 splash 彩蛋

`docs/icon.svg` 是 u-family mark 的 webu 版：藍 U 框住拼出 **WEB** 的金色字。`V`（任何面板、任何 screen；文字框與 visual mode 內除外）
觸發 splash 彩蛋，kbu / filu / sshu 同款、同鍵——visual mode 因此改成小寫 `v`（2026-09-21）：底片掃入 → W 散點浮現 → E → B → 藍 U 框自底升起——**揭示順序唸出 w-e-b**——然後名稱 /
版本 / tagline / 落款 / Esc 提示分兩拍淡入，任意鍵放回原畫面。像素畫由 icon.svg 逐格生成（中心點取樣 25×25，裁到圖案上下各留一列）；
splash 期間鍵盤完全歸它（2026-09-21）。`docs/social-preview.png` 用同一張 icon，由 `.claude/commands/social-preview.md` 生成。

**Nerd Font 是設計、必裝**（filu / sshu §3.1）：role glyph、live glyph、powerline chip 都靠它；README 要講。

---

## §6 存檔

| 資料 | 形式 | 位置 |
|---|---|---|
| Bookmarks | flat yaml：`bookmarks:`（各帶 `folder`）+ `folders:`（自己宣告的目錄，含空的；2026-09-21） | `~/.config/webu/bookmarks.yaml` |
| Downloads 清單 | 只在記憶體、本次 session；檔案本身落在 `download_dir` | — |
| History | YAML sequence，一次 append 一筆（時間、URL、標題）；**無限保留**，History screen 的 Clear 是唯一清除入口 | `~/.webu/datas/history.yaml` |
| Session | 離開時寫下所有分頁 URL，下次還原 | `~/.webu/datas/session.yaml` |
| 下載檔案 | `download_dir`，預設 | `~/.webu/datas/downloads/` |
| 設定 | v1 key：`search_engine`（預設 Google）、`download_dir`（預設 `~/.webu/datas/downloads`）、`measure`（預設 100）、`restore_session`（預設 true）；proxy v2。**規則（2026-09-21）：檔案裡有的 key，Settings screen 一定有一列**（`TestSettingsCoverConfig` 守著）——只能改檔才能設的 key，多數人永遠找不到 | `~/.config/webu/config.yaml` |
| Chromium | 釘死 revision | `~/.cache/webu/chromium-<rev>/` |
| profile | Chromium user-data-dir | `~/.webu/datas/profile/` |
| log | chromedp 的 log（絕不進終端機） | `~/.webu/datas/webu.log` |

分法（修訂 2026-09-21）：**使用者寫的**（設定、書籤）在 `~/.config/webu`；**webu 自己產生的**（歷史、session、下載、profile、log）
在 `~/.webu/datas`。`WEBU_CONFIG` / `WEBU_DATA` 各自覆寫；原本全部在 `~/.config/webu`，history 是 tab 分隔的純文字。

v2：`webu import chrome-bookmarks`（Chrome 的 Bookmarks 是純 JSON，Chrome 跑著也讀得到，
與 History 不同）。

---

## §7 Shell 功能落點

function.md §8 的清單各落到哪個 surface、哪一版：

| 功能 | Surface | v1 |
|---|---|---|
| 分頁 | `[1]` | ✓ |
| 輸入網址 | goto popup | ✓ |
| 上一頁 / 下一頁 | `P` / `N` 全域鍵 + `[2]` Space menu | ✓ |
| 頁內搜尋 | `[2]` 內 `/`，命中高亮 | ✓ |
| 憑證錯誤 | confirm popup | ✓ |
| 錯誤頁 | `[2]` 空狀態形狀 | ✓ |
| session 還原 | 無 UI，啟動時還原 | ✓ |
| 下載 | toast（開始 / 完成） | ✓ |
| 歷史 | History screen | ✓ |
| 書籤 | Bookmarks screen | ✓ |
| Outline | `[2]` Space menu → Outline popup | ✓ |
| cookie / storage | `[2]` Space menu → DevTools › Storage | ✓ |
| network log | `[2]` Space menu → DevTools › Network | ✓ 清單 + detail |
| console | `[2]` Space menu → DevTools › Console | ✓ viewport；Eval v2 |
| 清除某站資料 | DevTools › Storage 的 Clear site data | ✓ |
| view source | `[2]` Space menu → DevTools › Source（修訂 2026-09-20） | ✓ |
| print to PDF / 整頁截圖 | — | **移除**（2026-09-20） |
| 下載清單 | header `[D]ownloads` screen（修訂 2026-09-20：提前到 v1） | ✓ |
| 設定 | header `[S]ettings` screen，目前 `download_dir`（2026-09-21） | ✓ |
| 隱私分頁 | — | v2 |
| proxy | config.yaml 一個 key | v2 |
| reader mode | — | 看 Outline 夠不夠用 |
| import chrome bookmarks | CLI 子指令 | v2 |

---

## §8 待決

| # | 項目 | 去處 |
|---|---|---|
| 1 | ~~hotkey 字母~~ | 已在 `ux.md` §4 定案 |
| 2 | ~~非 URL 輸入~~ | **已決**：當搜尋，預設 Google（修訂 2026-09-21，原 DuckDuckGo），`config.yaml` 的 `search_engine` 可換 |
| 3 | ~~下載目錄~~ | **已決**：預設 `~/.webu/datas/downloads`（修訂 2026-09-21，原 `~/Downloads`），`config.yaml` / Settings screen 可改，toast 說存到哪 |
| 4 | ~~憑證錯誤~~ | **已決**：第一版每次問，不存例外 |
| 5 | ~~`target=_blank` 新分頁是否自動切換~~ | **已決**：自動切換 |
| 6 | link 色帶 | 本文件 §4，畫出來再挑 |
