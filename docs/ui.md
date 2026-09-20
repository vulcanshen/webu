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
 [B]ookmarks  [H]istory  [D]ownloads                       1 download in flight   ← header，chip 列
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
`[M]anage / [F]ile transfer / [S]SH` 的 chip chain）：`[B]ookmarks` `[H]istory` `[D]ownloads` 三個全域 popup 的常駐揭露，
開著的那個 chip 點亮；右端是狀態槽，有下載進行中時寫 `N download(s) in flight`（live 綠）。原本的 `[1]` Places 面板
內容永遠是三項、做成面板是浪費，併進 header 之後 Tabs 與 Page 並列。URL 收進 `[2]` 內部第一列（filu breadcrumb / sshu cwd 的做法）。

### 1.2 尺寸規則

| 項目 | 規則 | 前例 |
|---|---|---|
| 側欄寬 | 固定 **24 欄**（修訂 2026-09-20：28 是圖，24 是內容需要的） | sshu 固定左欄（manage 18 / ssh 30） |
| `[1]` 高 | header 與 footer 之間全部；分頁數不設上限，清單捲動 | — |
| `[2]` | 右側全部 | — |
| 窄寬 | `w < 72` 只畫**焦點那一側**：焦點在 `[2]` 畫頁面，`Tab` / `1` 到 Tabs 就改畫 Tabs；`Z` zoom 讓頁面佔滿（header / footer 仍在） | sshu §1.2 窄寬只畫 focus 側 |
| header / footer | 各 1 列，鎖死不 reflow | 通用 §1.3 |
| 寬度穩定 | 邊框 chip 文字固定；URL 用 filu `fitPathSegments` 縮，host 永不縮；分頁標題截斷不折行 | 通用 §1.2 |

---

## §2 header 與兩個面板的職責

### header — 全域動作的常駐揭露

一列 chip chain：**`[B]ookmarks`、`[H]istory`、`[D]ownloads`**（修訂 2026-09-20：原本是 `[1]` Places 面板，
三列 Bookmarks / Shortcuts / History。內容永遠三項、做成面板是浪費，改成 sshu 最上列
`[M]anage / [F]ile transfer / [S]SH` 那種 chip 列。Shortcuts 一併拿掉——有 Bookmarks、goto popup 又帶目前 URL
當 placeholder，它沒有自己的理由；Downloads 補上，就是原本排到 v2 的下載清單）。每個 chip 一個全域鍵開 popup（§3），
**`[2]` 不換內容**；開著的 chip 點亮（sshu active tab 的畫法），沒有 cursor、Tab 不停在這一列。
右端是狀態槽：有下載進行中時顯示 `N download(s) in flight`（live 綠），否則空。

Outline 與 DevTools **不在這裡**（決定 2026-09-20）：它們的作用對象是目前頁面，是 contextual
動作，走 `[2]` 的 Space menu panel operation（`ux.md` §A.1）。使用者體驗留在頁面上、不切面板。

VTP 定位：三個都是 non-contextual 動作（沒有作用對象），依 §A.2 走 `?` 入口 +
letter hotkey；header 就是 kbu statusbar chip 那種 Layer 2 ambient 揭露。

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

- 第一列：URL（純文字、不可 focus、縮法見 §1.2）；第二列分隔線；之後是 IR 渲染的頁面
- cursor 只停在 item（互動節點 + heading + landmark 的標題列），文字段落是 item 之間的 flow
- **Landmark（修訂 2026-09-20）**：每個 landmark 以一列帶名字的細線開頭（`▾ navigation Repository ────`），
  是 item，Enter → Collapse / Expand。頁面有 `main` 時，`main` 之外、也不在 `main` 裡的 landmark **預設摺疊**
  （`▸ banner · 14 items`），main 裡的全開；沒有 main 的頁全開。Outline 跳進摺疊的 landmark 會先把它打開
- **導覽清單一行流式**：navigation 裡、每項都短的 list 畫成 `Platform · Solutions · Resources` 一行折行
- **文字欄寬上限**：段落折行寬度 = min(面板寬, `config.yaml` 的 `measure`，預設 100)；表格、code、分隔線仍用整個面板寬
- **非 HTML 的回應**（修訂 2026-09-20）：JSON / 純文字 / XML / CSV 等依 `document.contentType` 整份畫成一個 code block，
  JSON 自動縮排；不畫 Chrome 自己的 JSON viewer（Pretty-print 表單）
- **新頁載入**：游標回 main 裡第一個 item、視窗在頂端；不跟著游標捲（同頁重畫才留位）
- 空狀態（沒有分頁）：置中事實「no page」+ 提示（該按什麼），sshu `empty.go` 的形狀
- 載入中：邊框 hint 顯示 loading；即時更新重畫時 cursor 留位（function.md §4 / §6）

---

## §3 Popup

五個 popup：三個由 header 開（Bookmarks / History / Downloads）、兩個由 `[2]` 的 Space menu
panel operation 開（Outline / DevTools）；加上既有的 Space menu / help / confirm / input / form / toast。
全部走 §6 Popup Convention：一個 popup 一個檔一個 PopupAnimator、title = glyph + 文字、
hint 嵌下邊框、`Esc` 只在一處解析。

### 3.1 五個 popup

| Popup | 類型 | 內容 | item operation | panel operation | 子 popup |
|---|---|---|---|---|---|
| **Bookmarks** | menu | 樹狀，目錄可摺疊（filu tree 畫法） | Enter 開啟（目前分頁）、Open in new tab、Edit、Delete、Move、Yank url | Add 目前頁、New folder、`/` 搜尋 | Edit → form；Delete → confirm；Move → 目錄 picker |
| **Downloads** | menu | 本次 session 的下載，新的在上：檔名 + 右欄是進度（`42%  1.2 MB of 3.0 MB`）、落地路徑、或 cancelled | Enter 用系統開啟器開檔、Open source in new tab、Remove（進行中先 `Browser.cancelDownload`；檔案不動）、Yank path | Clear 已完成 / 取消的 | — |
| **History** | menu，filu 原生 finder 形式 | 時間倒序、打字即 fuzzy、串流載入 | Enter 開啟、Open in new tab、Delete 該筆 | Clear | Clear → confirm |
| **DevTools** | viewport，**帶 tab bar**（見 3.2） | Storage / Network / Console 三分頁 | 依分頁 | 依分頁 | Network detail |
| **Outline** | menu | 目前頁的 landmark + heading，縮排表層級 | Enter：`[2]` cursor 跳到該節點 | — | — |

Shortcuts popup 拿掉了（2026-09-20）：它與 Bookmarks 形狀相近、只差「少量常用」這個 mindset，
而 goto popup 帶目前 URL 當 placeholder、Bookmarks 有 `/` 過濾之後，這個 mindset 不需要自己的入口。

Popup 疊 popup 的結構走 kbu §6.0.4：子 popup 獨立檔、獨立 animator，parent 在 `Open()`
reset 子 popup、route tick 與 update。

### 3.2 DevTools popup

u-family 第一個帶 tab bar 的 popup：kbu §8.2 的 starship chip chain 搬到 popup title 列，
`h/l` 切分頁。外殼一檔一 animator，三個分頁各一檔一 sub-model。尺寸接近全螢幕、無 padRow
（Network 欄位吃寬度）。

```
╭─ 󰙨 DevTools  Storage │ Network │ Console ──────────────────────────────────╮
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
| **Console** | `Runtime.consoleAPICalled`、`Runtime.exceptionThrown`、`Log.entryAdded` | viewport，等級 glyph，warn / error 用 override 色 | Enter → 該筆完整內容（detail 子 popup，訊息折行）、`[i]nsert` → Eval 輸入列（`Runtime.evaluate`，REPL）、`/` 過濾、Clear | **完整**（修訂 2026-09-20：Eval 提前到 v1；Enter 給 detail，因為清單裡長訊息被截斷看不到） |
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
| confirm | message | 憑證錯誤「要繼續嗎」、`beforeunload`、JS `confirm`、Delete / Clear |
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
| table header | 表頭 cell | Mauve `#cba6f7` 粗體 | 定案 2026-09-20 |
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

footer 一列：`space menu   ? help   tab/1-2 panels   q quit`，五個 core-key 全在此揭露。

**Nerd Font 是設計、必裝**（filu / sshu §3.1）：role glyph、live glyph、powerline chip 都靠它；README 要講。

---

## §6 存檔

| 資料 | 形式 | 位置 |
|---|---|---|
| Bookmarks | 巢狀 yaml，目錄 = 巢狀 | `~/.config/webu/bookmarks.yaml` |
| Downloads 清單 | 只在記憶體、本次 session；檔案本身落在 `download_dir` | — |
| History | append-only，時間、URL、標題；**無限保留**，History popup 的 Clear 是唯一清除入口 | `~/.config/webu/history` |
| Session | 離開時寫下所有分頁 URL，下次還原 | `~/.config/webu/session.yaml` |
| 設定 | v1 key：`download_dir`（預設 `~/Downloads`）、搜尋引擎（若 goto 當搜尋）；proxy v2 | `~/.config/webu/config.yaml` |
| Chromium | 釘死 revision | `~/.cache/webu/chromium-<rev>/` |
| profile | Chromium user-data-dir | `~/.config/webu/profile/` |

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
| 歷史 | History popup | ✓ |
| 書籤 | Bookmarks popup | ✓ |
| Outline | `[2]` Space menu → Outline popup | ✓ |
| cookie / storage | `[2]` Space menu → DevTools › Storage | ✓ |
| network log | `[2]` Space menu → DevTools › Network | ✓ 清單 + detail |
| console | `[2]` Space menu → DevTools › Console | ✓ viewport；Eval v2 |
| 清除某站資料 | DevTools › Storage 的 Clear site data | ✓ |
| view source | `[2]` Space menu → DevTools › Source（修訂 2026-09-20） | ✓ |
| print to PDF / 整頁截圖 | — | **移除**（2026-09-20） |
| 下載清單 | header `[D]ownloads` popup（修訂 2026-09-20：提前到 v1） | ✓ |
| 隱私分頁 | — | v2 |
| proxy | config.yaml 一個 key | v2 |
| reader mode | — | 看 Outline 夠不夠用 |
| import chrome bookmarks | CLI 子指令 | v2 |

---

## §8 待決

| # | 項目 | 去處 |
|---|---|---|
| 1 | ~~hotkey 字母~~ | 已在 `ux.md` §4 定案 |
| 2 | ~~非 URL 輸入~~ | **已決**：當搜尋，DuckDuckGo，`config.yaml` 的 `search_engine` 可換 |
| 3 | ~~下載目錄~~ | **已決**：預設 `~/Downloads`，`config.yaml` 可改，toast 說存到哪 |
| 4 | ~~憑證錯誤~~ | **已決**：第一版每次問，不存例外 |
| 5 | ~~`target=_blank` 新分頁是否自動切換~~ | **已決**：自動切換 |
| 6 | link 色帶 | 本文件 §4，畫出來再挑 |
