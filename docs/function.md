# webu — Function

> 本文件講「功能怎麼實現」：webu 站在 Chromium 上，哪些事 Chromium 做、哪些事 webu 做、
> 做到什麼程度。版面與 surface 在 `ui.md`、互動語意在 `ux.md`、實作落地在
> `webu-implementation.md`。每條決定標日期；被推翻的寫在「修訂」裡，不刪。
>
> 縮寫首次出現皆展開。CDP = Chrome DevTools Protocol；a11y = accessibility；
> AX tree = accessibility tree；IR = Intermediate Representation（中間表示法）；
> SPA = Single Page Application；OOPIF = out-of-process iframe（另一個 process 的 frame）。

---

## §0 定位

**一句話**：把 screen reader 讀到的東西，畫成一份能翻、能找、能填的文件。

webu 不寫 HTML 引擎、不定義新 protocol。它借 Chromium 的引擎，取 Chromium 為
screen reader 維護的 accessibility tree，加上同一棵 DOM 的版面快照，轉成自己的 IR，
在終端機以 TUI 呈現；使用者的操作透過 CDP 打回 Chromium 執行。

0.3.0（2026-09-23，分支 `sections`）把「頁面」重新定義了一次：一頁不再是一張從頭捲到尾的
紙，而是**一份文件** —— 有目錄（section list）、一次讀一節、也能攤成一整張；有四個
**part**（header / body / others / footer，由幾何切出來，不靠標籤名）；有 **finder**
（`/`）在整頁上找東西、Enter 直接落過去；表單畫成表單、清單裡的一件事就是一件事、
彈窗浮在頁面上等你回答、frame 是可以走進去的一層（跨站的也是）。0.2.x 之前的 webu
是「把 AX tree 排成一頁」；0.3.0 是「把它讀成一份文件」。

### 排除的路線

| 路線 | 為什麼不走 |
|---|---|
| 自訂 protocol / 新內容格式（gemtext 式） | 沒有網站會 follow；Gemini 做了十年只有幾千個站 |
| 自己 parse HTML、忽略 CSS / JS（w3m 式） | 現代 SPA 的 HTML 是空殼，不跑 JS 沒內容；DOM 是 div 湯，語意節點稀疏 |
| Chromium 像素轉字元（browsh / carbonyl 式） | 得到的是縮小的 GUI，沒有語意，套不上 VTP |
| 為單一網站寫 heuristic | 追不完，而且一改版就壞。webu 只認 AX tree 與 DOM 的版面事實，Jira 的 div 湯是 Jira 的問題（定案 2026-09-21） |

### 與同類工具的差異

| | w3m / lynx | browsh / carbonyl | webu |
|---|---|---|---|
| 引擎 | 自寫 HTML 引擎 | Chromium | Chromium |
| JS | 不跑 | 跑 | 跑 |
| 畫的東西 | DOM 模擬視覺排版 | 像素轉字元 | AX tree + 版面快照重組成一份文件：目錄、節、part、表單、彈窗、frame |
| 登入 / session | 自己的 cookie，JS 登入流程即死 | Chromium 的 | Chromium 的，webu 自己的持久 profile，登入一次即記住 |
| 成本 | 幾 MB | 幾百 MB | 幾百 MB，首次下載釘死版本的 Chromium 175–250 MB |

### 牆在哪裡（2026-09-23 明講）

網頁是二維的，眼睛在 dashboard 上平行過濾；終端機是一維的，任何線性化都會把「一瞥」變成
「翻頁」。這不是 heuristic 能補的。webu 的解法是 screen reader 使用者的解法：**不讀，跳** ——
目錄、part、finder、入口選單。文件型的頁面靠目錄就夠；app 型的頁面（Jira、後台）靠 `/`。
一個 ARIA 都不給、全部自己用 div 做的站，連 screen reader 都會壞，webu 也不追。

---

## §1 架構：三層

Chrome 這個產品 = Chromium 引擎 + Chrome 的 UI shell。
webu = Chromium 引擎 + webu 自己的 shell + 一層 Chrome 沒有的**翻譯層**。

| 層 | 誰做 | 內容 |
|---|---|---|
| **引擎層** | Chromium 全包，webu 零程式碼 | 網路、TLS、redirect、cookie、cache、JS、DOM、CSS layout、表單語意、SPA 路由、WebSocket、SSE、service worker、localStorage、iframe（含跨站 process 隔離）、Shadow DOM、每個分頁的前進後退堆疊、下載本體、上傳本體、HTTP auth 挑戰 |
| **Shell 層** | webu，Chrome 的 UI 殼做的事 | 分頁管理、網址列、書籤、全域歷史、下載清單、設定、session 還原、對話框呈現、檔案選擇器、DevTools。每項都是「拿 CDP 資料自己畫」或「自己存一個檔」，見 §8 |
| **翻譯層** | webu 的核心，Chrome 沒有這層 | AX tree + DOM 快照 → IR；幾何 → part 與彈窗；標題 → 節；cursor 節點 → CDP 動作；跨站 frame 的 session；viewport 同步；變動偵測、settling、重畫後 cursor 留位。見 §3 / §4 / §6 |

翻譯層不是 UX，是語意抽取；webu 好不好用八成由它決定。

**教條（2026-09-22 定案）**：**語意來自 AX tree，版面來自 DOM snapshot，靠 `backendDOMNodeId` 對接。**
AX tree 說「這是什麼、叫什麼、什麼狀態」；DOM snapshot 說「它是 block 還是 inline、在頁面上的哪個
位置、多大、DOM 裡誰是誰的祖先」。webu 不讀 CSS 的顏色字型，只讀這兩份的事實。

---

## §2 資料流

```
webu ─ 輸入 URL ─ Enter
  │
  ▼
Chromium（背景）：request → response → 跑 JS → layout → 產出 AX tree
  │
  ▼  CDP Accessibility.getFullAXTree + DOMSnapshot.captureSnapshot + Page.getLayoutMetrics
webu：AX tree + 版面快照 → IR → 切 part、切節、找彈窗 → 排版 → 顯示
  │
  ▼
使用者：目錄裡挑一節、游標走 item、/ 找東西、Enter
  │
  ▼  CDP DOM / Input / Runtime（跨站 frame 走它自己的 session）
webu：用 backendDOMNodeId 叫 Chromium 對該節點 hover + click / 填字 / 設值 / 選項
  │
  ▼
Chromium：頁面變了（換頁、彈窗、展開一個選單、frame 裡的東西動了）
  │
  └──▶ MutationObserver 通知 → 重抓 → 指紋不同就再等（settling）→ 回到「→ IR → 顯示」
```

webu 從頭到尾**不自己發任何 HTTP request**。表單送出是 Chromium 發的，SPA 的 fetch / XHR
是頁面 JS 發的，PUT / DELETE / GraphQL 一律同理。

### 每一步對應的 CDP 呼叫

| 步驟 | CDP |
|---|---|
| 導航 | `Page.navigate`，等 `Page.loadEventFired` + body ready；之後靠 settling（§6）等頁面真的長出來 |
| 取樹 | `Accessibility.getFullAXTree`（主 frame；同 process 的 frame 用 `frameId` 再取一次） |
| 取版面 | `DOMSnapshot.captureSnapshot(["display"])`：每個節點的 display、layout box、DOM 父子、屬性（`type`、`aria-current`、`class`、`src`、`id`）；`Page.getLayoutMetrics` 拿 viewport 大小 |
| 跨站 frame | `DOM.describeNode(<iframe>)` 拿它的 frame id（= target id）；`Target.attachToTarget(flatten)` 開一條 webu 自己的 session；在那條 session 上做同一套取樹取版面（§3.5） |
| hover + click | `DOM.scrollIntoViewIfNeeded` → `DOM.getBoxModel` → `Input.dispatchMouseEvent` mouseMoved（送了不等回應）→ mousePressed / mouseReleased 在 box 中心；沒有 box 才 `el.click()` |
| 填字 | `DOM.focus` → 全選 → `Input.insertText`（框架會收到 input 事件） |
| 整串設值（日期、時間、顏色、range） | `Runtime.callFunctionOn`：`el.value = …` + dispatch `input` / `change` |
| ARIA slider | `DOM.focus` → `Input.dispatchKeyEvent` ArrowLeft / ArrowRight 一步一步走，讀 `aria-valuenow` 到位為止 |
| select | option `selected = true` + 對 select dispatch `input` / `change` |
| 送鍵 | `Input.dispatchKeyEvent`（Enter 給欄位 = implicit submission；Escape；箭頭） |
| 前進後退 | `Page.getNavigationHistory` / `Page.navigateToHistoryEntry`；每個 entry 記一個游標位置（§4） |
| 分頁 | `Target.setDiscoverTargets` / `targetCreated` / `attachToTarget` |
| 檔案 | `Page.setInterceptFileChooserDialog` → `DOM.setFileInputFiles` |

---

## §3 翻譯層 — AX tree + DOM snapshot → IR

### 3.1 為什麼取 AX tree 而不是 HTML

AX tree 是 Chromium 為 screen reader 維護的語意樹：JS 已跑完、CSS 只留下語意結果、
`display:none` / `aria-hidden` 的節點已剪掉、Shadow DOM 已穿透、每個節點帶
role / name / state / backendDOMNodeId。role 的詞彙就是 IR 需要的 block 型別，別人已定好；
WCAG 逼大站把這棵樹做到能用。webu 不推任何格式，只吃 screen reader 吃的那份。

### 3.2 為什麼還要 DOM snapshot

AX tree 刻意不帶版面：`generic` 對 div 與 span 一視同仁，兩個相鄰 `<div>` 的字會黏成一行；
它也不說一個區塊在頁面的哪裡。而 0.3.0 的三件核心功能全靠幾何：

| 靠幾何 | 怎麼用 |
|---|---|
| **四個 part**（§3.4） | 最大面積的頂層區塊是 body；在它旁邊的是 others；在它前面的是 header、後面的是 footer。標籤叫 `nav` 還是 `div` 不重要 |
| **彈窗**（§4.3） | 按下之後新出現、疊在別的東西上、裡面有東西可按 → 是彈窗；不看 `role=dialog` 有沒有 |
| **sr-only** | 排在 1×1 點上的元素是寫給 screen reader 的（「Show subtasks for …」、issue key 再唸一次），丟掉 |

另外 snapshot 帶的 DOM 屬性補 AX tree 沒說的事：`type=password`（AX 不說，空欄位認不出）、
`type=email/date/color/…`（popup 邊框要寫）、`aria-current`、breadcrumb / skip 的 class 標記、
`<iframe src>`、錨點 `id` 與 DOM 父子表（頁內跳轉、彈窗判定的「無關的東西」）。

### 3.3 IR 定義原則

- **只有語意與幾何，沒有樣式**：不帶顏色、字型；寬度在排版時才決定。
- **一個 role 一個 Kind**：Document / Landmark / Heading / Paragraph / Text / Link / Button /
  Textbox / Check / Combobox / Option / List / ListItem / Table / Row / Cell / Media / Separator /
  Code / Quote / Span / Group / Gauge / Unsupported。細分由狀態帶（`Multiline`、`Protected`、
  `InputType`、`Multi`、`Expandable`、`Frame`、`Min` / `Max`、`Invalid`、`Focused`、`Modal` …）。
- **每個互動節點帶 `backendDOMNodeId`**：IR 與 Chromium 之間唯一的連結，動作靠它分派。
  跨站 frame 的節點 id 從 1 重編、會和主頁撞號，所以帶一個偏移（§3.5）。
- **來源可換**：AX tree + snapshot 是第一個來源；只要產出同一份 IR，渲染端不動。

### 3.4 role 表：以 AX role 為單位、全部宣告

**決定（2026-09-20）**：不以站為單位保證品質，以 **AX role 表**為單位。每個 role 宣告它變成
什麼 Kind、Enter 做什麼、怎麼畫。表在程式碼裡是**單一宣告**（`internal/ir/roles.go`），
`docs/support.md` 由它產生，文件與程式碼不會漂移。

0.3.0 把表補齊了（2026-09-23，user 拿 W3C ARIA Authoring Practices 的範例逐一實測後裁定）：

| 一群 | role | 怎麼畫 / Enter |
|---|---|---|
| 文件結構 | RootWebArea、landmark 八種、heading、paragraph、list / listitem、table / row / cell、blockquote、code / pre、separator、figure、article、sectionheader / sectionfooter | 見 `ui.md` §2：heading 依層級上色、清單裡的一件事一列（Enter 走進去）、資料表每格一停、code 是可停的區塊 |
| 可按的 | link、button、checkbox / radio / switch、menuitem 三種、treeitem、option、tab、DisclosureTriangle（`<summary>`） | Enter = 左鍵：link 先 confirm 再開；其餘直接 click。tab 畫成一條 pagetab 式的鏈；tree 依層縮排帶 `▾`/`▸`；listbox 的 option 是 radio / check 列 |
| 可填的 | textbox / searchbox / spinbutton、textarea、contenteditable、combobox（`<select>`；沒有 option 的 ARIA combobox 當 textbox）、slider、`<input type=date/datetime-local/time/month/week/color>`（Chromium 的 role 是 Date / DateTime / InputTime / ColorWell） | 每種輸入的行為在 `ux.md` §2：一行框、多行 editor、密碼遮罩、搜尋框送出、slider 列數字、日期顏色照瀏覽器的形狀整串設值 |
| 唯讀的值 | progressbar、meter | `Gauge`：填到值的 bar + 數字，不是停靠點 |
| 容器 | group、menu / menubar、tree、listbox、tablist、tabpanel、alert / log、tooltip | Group：只保住斷行；tooltip 畫成 dim 旁白；alert / log 就地畫 |
| 透明 | generic / none、LayoutTable 三種、MenuListPopup、status、timer、caption、rowgroup … | 節點消失、子節點留下 |
| 媒體 | image / video / audio / canvas、Iframe | 佔位一列；Iframe 是可以走進去的一層（§3.5） |
| 標記 | strong / emphasis / deletion / insertion / mark | 終端機屬性：粗、斜、刪除線、底線、反白 |

**Fallback 規則**（表能運作的前提）：未列的 role → `Unsupported`：accessible name 當文字畫出來、
子節點遞迴、**不隱藏任何內容**；固定 glyph 標記；focusable 的是 item、**Enter 仍是 click**；
Space menu 第一列寫「role: xxx, not supported yet — only click」。

### 3.5 frame：一件事，走進去才是一頁

`<iframe>` 在頁面上是一列（`󰘔 名字`），Enter 走進去、Esc 出來，跟清單裡的一件事一樣
（2026-09-23）。三種 frame 走三條路，使用者看不出差別：

| frame | 怎麼取 | 節點 id |
|---|---|---|
| 同 process（同站） | snapshot 本來就含所有同 process 的文件，`ContentDocumentIndex` 說哪個元素持有哪份；`getFullAXTree(frameId)` 取樹 | 同一個 process 裡唯一，不動 |
| 跨站（OOPIF） | 主 frame 的 `Page.getFrameTree` **根本不列它**、snapshot 也沒它的文件；但 `DOM.describeNode(<iframe>)` 給 frame id，那就是一個 `iframe` 型 target 的 id，瀏覽器允許對它 `Target.attachToTarget`。webu 對每個打開的跨站 frame 開一條自己的 session（`page.Sessions`，跟著分頁一起死），在那條 session 上取樹、取版面、點東西（用 frame 自己的座標，Chromium 會轉） | 從 1 重編、會撞主頁 → 每條 session 一個 slot，id 一律加 `slot × 2^40`（`ir.CarriedFrom`）；動作時解回原 session 與原 id |
| 巢狀 | 每一層都用同一套找 frame、抓 frame；frame 的 session 從帶著 Sessions 的 context 派生 | 同上，已 carried 的 id 不再加 |

frame 的版面 box 平移到 `<iframe>` 元素在頁面上的位置後併進主頁的 box 表，所以 part 切分與
彈窗判定看得到 frame 裡的內容。attach 五秒沒回應就放棄（那一列留著、說進不去）；哪次讀不到
就丟掉 session，下次重新 attach（frame 換站會是同 frame id 的新 target）。

### 3.6 非 HTML 的回應

JSON（含 `+json`）、text/plain、CSV、XML、JS、CSS 依 `document.contentType` 整份畫成一個
code block（JSON 自動縮排），不畫 Chrome 的 JSON viewer。**PDF 不支援**（2026-09-23）：headless
沒有 viewer、AX tree 是空的，畫成「unsupported media type」一頁 + Yank url。

### 3.7 測試策略

- **每個 role 一份 fixture**：一小段 HTML → 釘死版本的 Chromium → AX tree + snapshot 存成 JSON
  → 三份 golden：`.golden`（IR dump）、`.md`（markdown，人看得懂）、`.render`（60 欄的排版）。
  `go test ./internal/ir` 不需要瀏覽器；`make fixtures` 只認釘死的 Chromium。
- **行為用瀏覽器測**：`internal/ui` 的測試在 `httptest` / `file://` 的小頁面上跑真的 Chromium
  ——彈窗、表單、frame（含跨站、巢狀）、slider、tabs、tree、listbox、popup 疊層、載入指示、
  finder、section、back 回到原位。沒裝 Chromium 就 skip。
- **站級只做 smoke**（`WEBU_SMOKE=1`：Hacker News、GitHub）：整棵樹跑完不崩、未支援 role 看得見。
- Chromium 版本釘死（§9），fixture 不會因為引擎升版而漂。

---

## §4 互動分派

### 4.1 兩個核心語意（定案 2026-09-21，0.3.0 不變）

- **Enter = 滑鼠左鍵在 terminal 的對應**：先 hover（送 mouseMoved 不等回應）再按下——hover 才出現的
  選單因此也開得了（2026-09-23）。頁面自己決定會發生什麼。左鍵沒有對應的才自定義：
  heading / landmark 開合、清單裡的一件事走進去、frame 走進去、section list 開一節。
- **Space = 滑鼠右鍵 context menu**：item operation 對應右鍵選單，panel operation 對應空白處右鍵。

### 4.2 role → 預設動作 → CDP

| cursor 所在 | Enter | CDP |
|---|---|---|
| link | 先 confirm（連結文字 + URL），Enter 才開；同頁 `#` 錨點直接跳、不問 | hover + click |
| button / check / radio / switch / menuitem / treeitem / tab / option / `<summary>` | 按下 | hover + click |
| disabled 的任何東西 | toast 說它是 disabled，不按 | — |
| textbox（一行）/ password / searchbox / spinbutton | input popup；邊框寫它收什麼（email、tel、number…）；搜尋框寫回後問要不要送出 | focus + insertText；送出 = Enter keydown |
| textarea / contenteditable | editor popup（多行、寫 / 移兩態） | 同上 |
| `<input type=date/datetime-local/time/month/week/color>` | input popup，邊框寫形狀（`YYYY-MM-DD`、`#rrggbb`），不合形狀不送 | `el.value = …` + input / change |
| slider（`<input type=range>` 或 ARIA） | 數字清單（10 列一窗、游標在目前值） | range：設 value；ARIA：ArrowLeft / Right 走到 aria-valuenow 到位 |
| `<select>` | option 清單 | option selected + input / change |
| `<input type=file>` | 檔案 picker（一次一個目錄） | setFileInputFiles |
| image / video / canvas | 按下，頁面決定 | click |
| iframe | 走進去（§3.5） | — |
| 清單裡的一件事（多於一行的 listitem / article） | 走進去；只有一行的直接對裡面的東西做事 | — |
| heading / landmark 標題列 | 收合 / 展開 | — |
| 資料表的格 | 純文字 → 內容 popup；只有一個目標 → 那個東西的 Enter；混合 → 選單 | — |
| code block | 全文 popup | — |
| progressbar / meter / tooltip / timer / status | 不是停靠點 | — |

### 4.3 彈窗：靠行為認，不靠標籤

**決定（2026-09-23）**：頁面自己的彈窗（modal、alertdialog、menu button 的選單、cookie 橫幅）
**浮在頁面上**，用 webu 自己的 popup 框、對話框的寬度、底下的頁面變暗當 backdrop；回答完回到
原本的 section 與位置。判定條件（`pagepopup.go`）：

1. 按下之後（或載入後）8 秒內，樹裡**新出現**的子樹根；
2. 裡面**有東西可按**（沒有的是 toast、聊天泡泡、tooltip）；
3. 而且滿足其一：頁面把 focus 放進去、宣告 modal / dialog / alertdialog / menu、或**疊在無關的
   東西上**（用 DOM 父子表排除祖先與子孫，再看幾何有沒有蓋住別的 box 一半以上）。

彈窗會**疊**（一層一層往右下錯開）；載入時就在的也算；`Esc` 不關（toast 說它要一個回答）、
`/` 與 `go` 在裡面沒意義；原生 `showModal` 會讓 Chromium 砍掉 dialog 以外整棵樹，所以 backdrop
用彈窗出現前那刻的版面。

### 4.4 一次一個動作

`page.Click` 靠座標：讀 box 與按下之間若被舊的 Reveal 捲走，就按在別的東西上（量過：六次
漏一次）。每個分頁一把鎖，讀 box 與按下是同一個動作；對話框 / auth 的回答不排隊（否則死鎖）。

### 4.5 Viewport 同步

TUI 的捲動不等於 Chromium 的 viewport。頁面靠「元素進入畫面」觸發 lazy load 與無限捲動，所以
**cursor 移到某節點時同步對它 `scrollIntoViewIfNeeded`**。frame 裡的節點只捲 frame 內部
（外頁不跟，已知未做）。

### 4.6 Cursor 留位與回到原位

- 重畫後 cursor 靠 `backendDOMNodeId` 留在原節點；id 找不到落到最近的 item。
- **每個 history entry 記一個位置**（2026-09-23）：part、目錄 / 節 / 攤平、捲動、游標。上一頁回到
  離開時的地方；「是不是新頁」看 `Page.getNavigationHistory` 的 entry，不看 URL（同頁重畫
  的 settle 曾把游標一直重設）。
- 新頁的起點：main 裡第一個 item、視窗捲到它；沒有 main 停在最上層 heading；都沒有停第一列。
  文件型（≥ 3 個標題）的頁面先進**目錄**。

---

## §5 非 DOM 事件的 hook

原生對話框與 OS 層互動不在 AX tree 上，但 CDP 都有 hook，webu 接手畫成自己的 UI：

| 事件 | CDP | webu |
|---|---|---|
| `alert` / `confirm` / `prompt` / `beforeunload` | `Page.javascriptDialogOpening` → `handleJavaScriptDialog` | confirm / input popup，回答後送回；開著時 settle 暫停（碰 renderer 會卡） |
| HTTP basic / digest auth | `Fetch.enable(handleAuthRequests)` → `authRequired` → `continueWithAuth` | 帳號、密碼（遮罩）兩個框 |
| 檔案上傳 | `Page.setInterceptFileChooserDialog` → `fileChooserOpened` → `DOM.setFileInputFiles` | 檔案 picker（同 Import bookmarks 那個，`~/Downloads` 起、一次一檔） |
| 下載 | `Browser.setDownloadBehavior` → `downloadWillBegin` / `downloadProgress` | toast + Downloads screen + header 進度條 |
| 新分頁（`target=_blank` / `window.open`） | `Target.targetCreated` | 加到清單尾端並切換 |
| 憑證錯誤 | 導航錯誤 `ERR_CERT_*` → `Security.setIgnoreCertificateErrors` | confirm 一次、該分頁放行 |

---

## §6 即時更新與 settling

傳輸方式與 webu 無關：頁面 JS 在 Chromium 內跑，資料最後落在 DOM。webu 只需要「DOM 變了」的
通知與重畫策略。

- **偵測**：`Page.addScriptToEvaluateOnNewDocument` 注入 MutationObserver（childList /
  characterData / subtree，150 ms 合併），`Runtime.addBinding` 回呼；`Accessibility.nodesUpdated`
  留作日後優化。
- **settling（2026-09-23）**：一次 capture 落地不代表頁面好了——SPA 會再長好幾輪。每次 capture 算
  一個指紋（樹的內容，不含 node id）；指紋和上次不同、又在按下 / 導航後 8 秒的寬限內，就還在
  「工作中」：spinner 繼續轉、不重設游標。兩次一樣才算落地。代價：有 timer / 時鐘的頁面每次
  動作後 spinner 會轉滿 8 秒（live region 也進了指紋；已知，user 未表態）。
- **重畫**：全抓（HN 規模幾十毫秒）；cursor 依 §4.6 留位；使用者在 visual mode 裡時凍結、離開才套用。

### 兩個坑

1. **分頁 timer 節流**：Chrome 對 hidden tab 的 `setTimeout` 節流；webu 關掉三個 background
   throttling flag。
2. **cursor 錨點失效**：React / Vue 整段換掉時 id 全新 → 落到最近的 item。

---

## §7 完整度矩陣

| 程度 | 內容 |
|---|---|
| **完整，與 GUI 無差** | 所有 DOM 驅動的互動：登入、表單（每種 input 都有定義的互動）、POST、SPA、即時更新、多分頁、OAuth 跳轉、Shadow DOM、iframe（同站、跨站、巢狀）、hover 才出現的選單、頁面自己的彈窗 |
| **完整，但 webu 要接 hook 畫 UI** | alert / confirm / prompt、離頁確認、HTTP auth、檔案上傳、下載（§5） |
| **有損** | 圖片與多媒體只給佔位列；a11y 做爛的站只剩文字與能點的東西；拖拉排序；顏色與位置才有意義的資訊；密度——一個 dashboard 攤成一維後要翻，靠 `/` 與目錄跳 |
| **做不到** | 影片音訊、canvas / WebGL app（地圖、Google Docs）、CAPTCHA、passkey / Touch ID、WebRTC：佔位明講 + Yank url，沒有第二條路（§9.1） |

### 開發者日常網站的估計

| 類別 | 例子 |
|---|---|
| 文件型，讀得好 | MDN、Wikipedia、各種 docs 站、blog、W3C、go.dev：目錄 → 一節 → 攤平 |
| app 型，進得去 | GitHub、GitLab、Jira、Confluence、後台管理系統：分類都在，但要用 `/` 找、翻 part |
| 看得到字、主功能廢掉 | Grafana 與任何以圖表為主的 dashboard |
| 不可用 | YouTube、Google Docs、Figma、地圖 |

### 免費附送

站在 CDP 上，GUI 瀏覽器要開 devtools 才看得到的東西直接當功能：network log、console（含 REPL）、
cookie / storage、view source。目標用戶是開發者，這些是賣點而非附件。

---

## §8 Shell 功能清單

| 功能 | 怎麼做 | 存檔 |
|---|---|---|
| 分頁 | 一個 target 一個分頁；`target=_blank` 監聽 `targetCreated` | 無 |
| 輸入網址 | Location popup（`L`）：目前 URL 當提議；非 URL 當搜尋，預設 DuckDuckGo html 版（2026-09-23：Google 對 headless 一律回 reCAPTCHA，即使 webu 用自己的 UA；十二個引擎實測後選 DDG html：最乾淨、Brave 最豐富、Mojeek / Startpage / Qwant 對 headless 什麼都不給） | `config.yaml` `search_engine` |
| 上一頁下一頁 | `Page.navigateToHistoryEntry`，每個 entry 記位置（§4.6） | 無 |
| 頁內搜尋 | **finder**（`/`，`ux.md` §1）：整頁四個 part 的節點索引、輸入 / 清單 / 預覽三區、Enter 直接落過去；visual mode 裡的 `/` 才是逐字搜 | 無 |
| 行號 | 每個畫面都有行號欄；`go` + 數字 + Enter 跳行（目錄上行號就是節的序號） | 無 |
| 憑證錯誤 | §5 | 無；每分頁問一次 |
| 錯誤頁 | DNS 失敗、連線拒絕：`Page.navigate` 的 errorText 畫成空狀態 | 無 |
| 歷史 | 自存：時間、URL、標題，append-only，無限保留 | `history.yaml` |
| 書籤 | 目錄樹、搬移、改名、匯入瀏覽器的匯出檔 | `bookmarks.yaml` |
| session 還原 | 離開時寫下所有分頁，下次開回來（不預載） | `session.yaml` |
| 下載 | §5 | `download_dir` |
| network / console / storage / source | DevTools popup（`I`） | 無 |
| 設定 | `config.yaml` 每個 key 都有一列 | `config.yaml` |
| Yank | 頁面 URL、連結 URL、文字、欄位值、整頁 markdown | 剪貼簿 |

**不做，明講**：密碼管理與自動填入；擴充套件；多 profile；print to PDF；整頁截圖；隱私分頁；proxy 設定（v2）。

---

## §9 Chromium 的取得與執行模式

**自帶、釘死版本、headless、無視窗。** 不 attach 使用者的 Chrome（Chrome 136 起
`--remote-debugging-port` 拒絕配預設 user-data-dir；企業管控常關 remote debugging；沒裝 Chrome
的機器也要能跑）。Playwright、Puppeteer、Electron 都是自帶 Chromium。

| 項目 | 決定 |
|---|---|
| 二進位 | 首次啟動下載釘死 revision 的 Chromium 到 cache 目錄（175–250 MB，明講）；`webu browser update` 換新 pin |
| 版本覆寫 | **不開放**。翻譯層的 heuristic 與 fixture 只對釘死的版本負責 |
| 執行模式 | 一律 `--headless=new`：沒有視窗、不進 Dock；與有視窗的 Chromium 是同一個引擎 |
| profile | webu 自己的持久目錄，登入一次即記住；不碰使用者的 Chrome |
| 啟動 flag | 拿掉 `--enable-automation`（否則 `navigator.webdriver` 為 true）；關 background throttling |
| User-Agent | **webu 自己的名字，不是偽裝**（2026-09-23）：拿 Chromium 自報的字串，把 `HeadlessChrome/<ver>` 寫成每個 Chromium 系瀏覽器都這樣寫的 `Chrome/<ver>`，尾巴簽 `webu/<version>`；client hints 的 brands 是 `Chromium` 與 `webu`。「Headless」是模式不是瀏覽器的名字，而頁面看到它就回 bot-check 而不是頁面 |
| 離開時 | 殺掉 Chromium |
| 安全更新 | 釘死的版本會老、且握著真實 session cookie：`webu browser update`，版本過舊時提醒 |
| Linux 無頭伺服器 | 仍需 libnss3、libatk、字型等系統函式庫 |

### §9.1 逃生口

| 種類 | 例子 | 出口 |
|---|---|---|
| 內容本身是多媒體 | 影片、音訊、大圖、地圖 | 佔位列 + Yank url |
| 卡在 session 裡的關卡 | CAPTCHA、passkey / Touch ID、canvas app、拖拉 | **不支援，明講**：佔位 + Yank url，登入不共享 |

**沒有交棒機制（決定 2026-09-23）**。曾擱置一個方案：關掉 headless、同 profile 開有視窗的
Chromium 讓人過關、關窗後接回來。拿掉，因為 webu 讀的是 AX tree 不是畫面，所以它**必須**能在
沒有 display 的機器上跑（SSH 進去的 server 是主要場景）；一個只在桌面才存在的出口不是出口，是
分岔的行為。隱形 CAPTCHA（reCAPTCHA v3、Cloudflare managed challenge）多半直接過；真的跳出圖片
挑戰的才過不了。

---

## §10 已決事項

| 決定 | 日期 |
|---|---|
| 自帶釘死版本的 Chromium、不開放覆寫、一律 headless | 2026-09-20 |
| 以 AX role 表為單位保證支援度；未支援 fallback 不隱藏 | 2026-09-20；表補齊 2026-09-23 |
| Enter = 左鍵、Space = 右鍵選單，分開 | 2026-09-21 |
| 語意來自 AX tree、版面來自 DOM snapshot、靠 backendNodeId 對接 | 2026-09-22 |
| 一頁是一份文件：目錄 / 一節（含子節）/ 攤平；四個 part 由幾何切 | 2026-09-22 / 23 |
| 彈窗靠行為認、浮在頁面上、必須回答 | 2026-09-23 |
| 沒有交棒機制；CAPTCHA 是硬牆；預設搜尋 DuckDuckGo html | 2026-09-23 |
| UA 簽自己的名字 | 2026-09-23 |
| frame 是可以走進去的一層，跨站的走自己的 session | 2026-09-23 |
| 不為單一網站加 heuristic | 2026-09-21 |

---

## §11 驗收

- role 表內每個 role 都有 fixture 且三份 golden 通過；`docs/support.md` 與表一致。
- `internal/ui` 的瀏覽器測試全綠（`go test ./...`，需要釘死的 Chromium）。
- 三個 smoke 站：Hacker News（table 排版、無 landmark）、GitHub（半 SPA、正規 landmark）、
  任一 Vue / React 後台（純 SPA）；各要能登入、能點、能填表，未支援 role 看得見、不崩。
- 五個 W3C APG pattern 頁（tabs、menu button、tree、listbox、slider）與 dialog / alertdialog
  範例是 role 與彈窗的手動驗證網址。

---

## §12 CLI

| 指令 | 行為 |
|---|---|
| `webu` | 還原上次 session 的分頁（不預載，切到才載）；`restore_session: false` 則從空白開始 |
| `webu <url|words> ...` | 每個參數多開一個分頁（第一個在前）；解讀同 Location：沒 scheme 補 `https://`、不像 URL 的字當搜尋 |
| `webu browser update` | 更新釘死的 Chromium |
| `webu version` | webu 版本與 Chromium revision |
| `webu help` | 用法；其他 `-` 開頭視為未知選項 |

看任何頁面的 AX tree：`make axdump URL=…`（用本機 Chrome，不是釘死那份）。
