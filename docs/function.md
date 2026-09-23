# webu — Function

> 本文件只講「功能怎麼實現」：webu 站在 Chromium 上，哪些事 Chromium 做、哪些事
> webu 做、做到什麼程度。版面與浮層在 `ui.md`、VTP 落點在 `ux.md`（皆待寫）。
>
> 縮寫首次出現皆展開。CDP = Chrome DevTools Protocol；a11y = accessibility；
> AX tree = accessibility tree；IR = Intermediate Representation（中間表示法）；
> SPA = Single Page Application；SSE = Server-Sent Events。

---

## §0 定位

**一句話**：把 screen reader 的輸出畫成 TUI，而不是唸出來。

webu 不寫 HTML 引擎、不定義新 protocol。它借 Chromium 的引擎，取 Chromium 為
screen reader 維護的 accessibility tree，轉成自己的 IR，在終端機以 TUI 呈現；
使用者的操作透過 CDP 打回 Chromium 執行。

### 排除的路線

| 路線 | 為什麼不走 |
|---|---|
| 自訂 protocol / 新內容格式（gemtext 式） | 沒有網站會 follow；Gemini 做了十年只有幾千個站 |
| 自己 parse HTML、忽略 CSS / JS（w3m 式） | 現代 SPA 的 HTML 是空殼，不跑 JS 沒內容；DOM 是 div 湯，語意節點稀疏 |
| Chromium 像素轉字元（browsh / carbonyl 式） | 得到的是縮小的 GUI，沒有語意，套不上 VTP |

### 與同類工具的差異

| | w3m / lynx | browsh / carbonyl | webu |
|---|---|---|---|
| 引擎 | 自寫 HTML 引擎 | Chromium | Chromium |
| JS | 不跑 | 跑 | 跑 |
| 畫的東西 | DOM 模擬視覺排版 | 像素轉字元 | AX tree 重組成語意結構 |
| 登入 / session | 自己的 cookie，JS 登入流程即死 | Chromium 的 | Chromium 的，webu 自己的持久 profile，登入一次即記住 |
| 成本 | 幾 MB | 幾百 MB | 幾百 MB，首次下載釘死版本的 Chromium 150–200 MB |

---

## §1 架構：三層

Chrome 這個產品 = Chromium 引擎 + Chrome 的 UI shell。
webu = Chromium 引擎 + webu 自己的 shell + 一層 Chrome 沒有的**翻譯層**。

| 層 | 誰做 | 內容 |
|---|---|---|
| **引擎層** | Chromium 全包，webu 零程式碼 | 網路、TLS、redirect、cookie、cache、JS、DOM、CSS layout、表單語意、SPA 路由、WebSocket、SSE、service worker、localStorage、iframe、Shadow DOM、每個分頁的前進後退堆疊、下載本體、上傳本體、HTTP auth 挑戰 |
| **Shell 層** | webu，Chrome 的 UI 殼做的事 | 分頁管理、網址列、書籤、全域歷史、頁內搜尋、reader mode、對話框呈現、檔案選擇器、下載清單、設定、session 還原。每項都是「拿 CDP 資料自己畫」或「自己存一個檔」，見 §8 |
| **翻譯層** | webu 的核心，Chrome 沒有這層 | AX tree → IR 的 heuristic、cursor 節點 → CDP 動作的分派、viewport 同步、DOM 變動偵測與重畫、重畫後 cursor 留位。見 §3 / §4 / §6 |

翻譯層不是 UX，是語意抽取；webu 好不好用八成由它決定。

---

## §2 資料流

```
webu ─ 輸入 URL ─ Enter
  │
  ▼
Chromium（背景）：request → response → 跑 JS → layout → 產出 AX tree
  │
  ▼  CDP Accessibility.getFullAXTree
webu：AX tree → IR → 顯示
  │
  ▼
使用者：移動 cursor、看動作、執行
  │
  ▼  CDP DOM / Input / Runtime
webu：用 backendDOMNodeId 叫 Chromium 對該節點 click / 填字 / 選項
  │
  ▼
Chromium：頁面變了（換頁、或只是展開一個選單）
  │
  └──▶ 回到「AX tree → IR → 顯示」
```

webu 從頭到尾**不自己發任何 HTTP request**。表單送出是 Chromium 發的，
SPA 的 fetch / XHR 是頁面 JS 發的，PUT / DELETE / GraphQL 一律同理。

### 每一步對應的 CDP 呼叫

| 步驟 | CDP |
|---|---|
| 導航 | `Page.navigate`，等 `Page.loadEventFired`；SPA 另等 network idle 或 main landmark 有內容，只等 body ready 會拿到空樹 |
| 取樹 | `Accessibility.enable` → `Accessibility.getFullAXTree`；iframe 逐 frame 取（`Page.getFrameTree` + `frameId`） |
| 局部取樹 | `Accessibility.getPartialAXTree(backendNodeId)`，節點上萬時的優化 |
| click | `DOM.resolveNode(backendNodeId)` → `Runtime.callFunctionOn(el.click())`；或 `DOM.scrollIntoViewIfNeeded` + `DOM.getBoxModel` + `Input.dispatchMouseEvent` |
| 填字 | `DOM.focus(backendNodeId)` → `Input.insertText` |
| select | `Runtime.callFunctionOn` 設 value 並 dispatch `change` |
| 送鍵 | `Input.dispatchKeyEvent`（例如把 Esc 轉給頁面關 dialog） |
| 前進後退 | `Page.getNavigationHistory` / `Page.navigateToHistoryEntry` |
| 分頁 | `Target.setDiscoverTargets` / `Target.targetCreated` / `Target.attachToTarget` |
| 元素截圖 | `DOM.getBoxModel` → `Page.captureScreenshot(clip)` |

---

## §3 翻譯層 — AX tree → IR

### 為什麼取 AX tree 而不是 HTML

AX tree 是 Chromium 為 screen reader 維護的語意樹：JS 已跑完、CSS 只留下語意結果、
`display:none` / `aria-hidden` 的節點已剪掉、Shadow DOM 已穿透、每個節點帶
role / name / state / backendDOMNodeId。

role 的詞彙（navigation、main、heading、link、button、textbox、combobox、
checkbox、table、list …）就是 IR 需要的 block 型別，別人已定好。

採用問題被別人解掉了：WCAG（Web Content Accessibility Guidelines）逼大站把
這棵樹做到能用，且有法規壓力。webu 不推任何格式，只吃 screen reader 吃的那份。

### 實測

chromedp 對真實頁面取 `getFullAXTree`，濾掉 `generic` / `none` / `StaticText` /
`InlineTextBox`：

| 站 | raw AX node | 有語意的 node |
|---|---|---|
| news.ycombinator.com | 1604 | 496 |
| github.com/chromedp/chromedp | 1273 | 228 |

HN 是 table 排版，樹上是 `LayoutTable / row / cell / link`；GitHub 有正規 landmark
（`banner` / `main` / `navigation "Repository"`）與 `heading` / `button`。
兩者代表翻譯層要面對的兩端。驗證程式見附錄。

### IR 定義原則

- **只有語意，沒有排版**：不帶寬度、顏色、字型、座標。渲染完全是 TUI 的事。
- **一個 role 一個 block 型別**，型別壓在十個左右：document / landmark / heading /
  text / link / button / input（textbox、textarea、checkbox、radio、select 的細分
  由 state 帶）/ list / table / image。
- **每個互動節點帶 `backendDOMNodeId`**：這是 IR 與 Chromium 之間唯一的連結，
  動作分派靠它。
- **來源可換**：AX tree 是第一個來源。之後接純 fetch + Readability 的輕量路徑，或
  網站直接給的 markdown，只要產出同一份 IR，渲染端不動。

### Heuristic 清單（翻譯層的實際工程量）

| 情境 | 處理 |
|---|---|
| table 排版當 list 用（HN） | 一個 row 裡有 N 個 link → 視為一個 list item，第一個 link 是主體，其餘是 meta |
| 沒有 landmark 的站 | 以 heading 層級切段；找不到 main 就把最大的文字區塊當 main |
| `div` + onclick、無 role（a11y 做爛的站） | `DOMDebugger.getEventListeners` 找有 click handler 的節點，或 `cursor:pointer`，補成 button |
| 一頁幾千個節點（新聞站、Gmail） | landmark 可摺疊（Enter 開合；預設全開，2026-09-21）、Outline |
| 點擊後新出現的子樹、無 `dialog` role | 視為浮層 |
| SVG chart | 只取 `title` / `aria-label`，path 無數值，不假裝能讀 |

### 支援白名單：以 AX role 為單位

**決定（2026-09-20）**：不以站為單位保證品質，以 **AX role 白名單**為單位。明確宣告
支援哪些 role、各怎麼顯示、各能怎麼互動；未列的一律 fallback。一開始不好用沒關係，
支援度是明確的，逐版添補。

單位是 role 不是 HTML tag：webu 看到的樹上沒有 tag；role 詞彙更少（`a` / `area` /
`div role=link` 都是 `link`）；ARIA 自訂元件自動納入；tag → role 的映射 Chromium
已做完。

**Fallback 規則**（白名單能運作的前提）：

- 未支援的 role：accessible name 當純文字畫出來，子節點遞迴。**不隱藏任何內容**。
- 節點 focusable：cursor 可停，**Enter 仍是 click**（click 對任何節點有效，Enter =
  左鍵的紅利），未支援的互動元件至少能點。
- Space menu 第一列寫「role: slider，尚未支援，只能 click」，disabled 語意同 sshu：
  停得上去才問得到原因。
- 顯示上加固定 glyph 標記 unsupported，使用者知道這裡有東西沒被理解，回報 issue 有依據。

**v1 白名單**（初擬）：

| role | 顯示 | 互動 |
|---|---|---|
| RootWebArea | document 根 | — |
| banner / navigation / main / complementary / contentinfo / region / form / search | landmark 區段，進 outline | — |
| heading | 標題，帶 level | cursor 可停 |
| paragraph / StaticText | 文字流，依寬折行 | — |
| link | glyph + name | Enter click；Space menu：Open in new tab、Yank url（href 不在 AX node 上，另抓 `DOM.getAttributes`） |
| button | `[ name ]` | Enter click |
| textbox / searchbox | `name ____value____` | Enter click 後進輸入態 |
| checkbox / radio / switch | `[x] name` | Enter click |
| combobox（`<select>`） | `name [value]` | Enter 列出 option 子節點，選完設 value |
| list / listitem | `•` 縮排 | — |
| table / row / cell / columnheader / rowheader | 欄位對齊，欄寬收縮同 sshu | cursor 停在 row |
| LayoutTable / LayoutTableRow / LayoutTableCell | 透明容器（HN 這類用 table 排版） | — |
| generic / none | 透明容器 | — |
| image / video / audio / canvas / iframe | 佔位框（§4 多媒體） | Enter click；Space menu：Yank url |
| separator | 水平線 | — |
| code / pre | 等寬區塊 | — |
| blockquote | 縮排引用 | — |

**v1.x 候選**：dialog / alertdialog、tab / tablist / tabpanel、menu / menuitem、
details（disclosure）、textarea 多行、status / alert（live region）、progressbar、
tree / treeitem、grid、slider、spinbutton、article、figure、iframe 內容遞迴。

支援表在程式碼裡是**單一宣告**（同 sshu 的 action table），`docs/support.md` 由它產生，
文件與程式碼不會漂移。

### 測試策略

- **每個 role 一份 fixture**：一小段 HTML → 真 Chromium → dump AX tree JSON → 斷言
  產出的 IR。白名單內每個 role 都要有。
- **站級 fixture 降成 smoke test**：真實頁面的 AX tree 存一份，測整棵樹跑完不崩、
  未支援 role 以 fallback 形式看得見。
- Chromium 版本釘死（§9），fixture 不會因為引擎升版而漂。

---

## §4 互動分派

### 兩個核心語意（決定 2026-09-20，細節在 `ux.md`）

- **Enter = 滑鼠左鍵點一下。** 對任何節點都是對它 click，頁面自己決定會發生什麼。
  **修訂（2026-09-20，實機試用後）**：Enter 改為開該 item 的 item operation 選單，click 是選單第一列；
  Space 開完整選單。分派表不變，只是多了一層揭露。細節在 `ux.md` §A.0.K。
  與 filu「Enter 只進目錄、不交給外部程式」不同：對象不同，網頁裡的 drill into
  就是左鍵進入，沒有第二種代價要區分。VTP §A.0.K 只要求同一 app 內跨 surface 不變。
- **Space menu = 滑鼠右鍵 context menu。** 對節點的 item operation 對應右鍵選單
  （新分頁開啟、複製連結 …），對頁面的 panel operation 對應空白處右鍵（重新載入、
  上一頁 …）。使用者從滑鼠帶來的心智模型直接可用。

click 之後還要第二步的只有兩種，與滑鼠使用者體驗一致：textbox 點了要打字（webu
接手輸入），`<select>` 點了要選（webu 從 AX tree 的 option 節點列出，選完設 value）。

### role → 預設動作 → CDP

| cursor 所在 role | 預設動作 | CDP |
|---|---|---|
| link | 開啟 | click |
| button | 按下 | click |
| textbox / searchbox | 編輯 | focus + insertText |
| textarea | 編輯（可交給 `$EDITOR`） | focus + insertText |
| checkbox / radio / switch | 切換 | click |
| combobox / select | 選項 | callFunctionOn 設 value + change |
| `<form>` 內任一節點 | 整張表單填寫 / 送出 | 逐欄 insertText，submit button click 或 `form.requestSubmit()` |
| image / video / canvas / embed | 佔位框；Enter 同樣是 click，頁面自己決定 | click |

### 多媒體：只畫佔位框

**決定（2026-09-20）**：image / video / audio / canvas / iframe embed 一律畫成**佔位框**：
固定形狀，內容是類型 glyph、alt 或 name（沒有就寫 no alt）、尺寸（`DOM.getBoxModel`）。
不用終端機圖片協定、不做 screencast、不轉 ASCII、不開外部工具或內建 viewer。

佔位框上的 Enter 與其他節點一樣是 click，頁面自己決定會發生什麼（播放、開 lightbox），
結果若落在 DOM 上就會重畫出來。Space menu 的 Yank url 讓使用者拿去別處開。

**擱置、日後再議**（討論過的方案與已知限制，避免重查）：

- 半格色塊 / ASCII 轉換：元素截圖 `captureScreenshot(clip)` → pixterm `ansimage` 或 image2ascii 類 lib
- 以內建 Chromium 開媒體：SingletonLock 不允許同 profile 雙實例，需第二實例 + 臨時 profile +
  `Network.getCookies` / `setCookies` 複製該 domain cookie。優點是私有資源（Jira 附件、
  private repo 圖片）開得起來、PDF viewer 免費；限制是開源 Chromium 無 H.264 / AAC，
  MP4 多半播不出，YouTube 的 VP9 / AV1 可
- 外部工具 Open with：私有資源拿不到 cookie 會 401，且要使用者設定工具

### Viewport 同步（最容易忽略）

TUI 的捲動不等於 Chromium 的 viewport 捲動。頁面靠「元素進入畫面」觸發 lazy load
與無限捲動，所以 **cursor 移到某節點時，同步對它 `scrollIntoViewIfNeeded`**，
否則永遠載不到下一頁。hover 才出現的選單同理：focus 到節點時送一次
`mouseMoved` 到它的座標。

### Cursor 留位

重畫後 cursor 靠 `backendDOMNodeId` 留在原節點。React / Vue 若整段 list 換掉而非
就地更新，id 全部變新的；第二層錨點用 `role + name + 父節點內序位` 做指紋，
id 找不到就用指紋找，兩層都失敗才落到最近的 item。

---

## §5 非 DOM 事件的 hook

原生對話框與 OS 層互動不在 AX tree 上，但 CDP 都有 hook，webu 接手畫成自己的 UI：

| 事件 | CDP | webu |
|---|---|---|
| `alert` / `confirm` / `prompt` / `beforeunload` | `Page.javascriptDialogOpening` → `Page.handleJavaScriptDialog` | 畫成浮層，回答後送回 |
| HTTP basic / digest auth | `Fetch.enable(handleAuthRequests)` → `Fetch.authRequired` → `Fetch.continueWithAuth` | 帳密輸入，遮罩 |
| 檔案上傳 | `Page.setInterceptFileChooserDialog` → `Page.fileChooserOpened` → `DOM.setFileInputFiles` | 自己的檔案選擇器 |
| 下載 | `Browser.setDownloadBehavior(allow, eventsEnabled)` → `Browser.downloadWillBegin` / `downloadProgress` | 下載清單與進度 |
| 新分頁（`target=_blank`） | `Target.targetCreated` | 分頁清單 |
| 網頁自己的 Esc 語意 | `Input.dispatchKeyEvent(Escape)` | 關 TUI 浮層時同步送給頁面 |

---

## §6 即時更新（long polling / SSE / WebSocket）

傳輸方式與 webu 無關：都是頁面 JS 在 Chromium 內跑，資料最後落在 DOM。
webu 只需要「DOM 變了」的通知與重畫策略。

### 變動偵測

| 選項 | 評估 |
|---|---|
| `DOM.childNodeInserted` 等事件 | 要先 `DOM.getDocument` 拉整棵樹才會發；dashboard 一秒改幾百個 text node 會淹掉 |
| `Accessibility.nodesUpdated` | 直接給變動的 AX node，最理想；標記 experimental，穩定性待實測 |
| 注入 MutationObserver + `Runtime.addBinding` 回呼 | `Page.addScriptToEvaluateOnNewDocument` 注入，頁面端 debounce 100–200ms 再叫；browser-use 類工具的做法，最穩 |

**選第三個**，第二個作為之後的優化。

### 重畫

- 通知後重抓：先全抓（HN 規模幾十毫秒，每秒一次撐得住），節點上萬再改
  `getPartialAXTree` 只抓變動子樹。
- 新舊 IR diff，只重畫變動 row；cursor 依 §4 留位。

### 兩個坑

1. **分頁 timer 節流**：Chrome 對 hidden tab 的 `setTimeout` 節流到每秒一次，
   隱藏超過五分鐘更狠；SSE / WebSocket 是 push 不受影響，**long polling 與
   `setInterval` 輪詢會被拖慢**。webu 走 `--headless=new`，頁面 `visibilityState`
   應為 visible、不觸發節流，但多分頁時非當前 target 是否被視為 hidden 要實測；
   保險起見對每個 target 開 `Emulation.setFocusEmulationEnabled(true)`。
2. **cursor 錨點失效**：見 §4 指紋錨點。

### 邊界

- 更新的是數字、狀態文字、表格列、告警清單：完整，與 GUI 同步。Grafana 的
  stat panel、Jira board 卡片、CI pipeline 狀態表皆屬此類。
- 更新的是 chart：不行。canvas 無內容可讀；SVG 在 DOM 裡但線與柱只是 path，
  AX tree 上無數值。

---

## §7 完整度矩陣

| 程度 | 內容 |
|---|---|
| **完整，與 GUI 無差** | 所有 DOM 驅動的互動：登入、表單、POST、SPA、即時更新、多分頁、OAuth 跳轉、Shadow DOM、iframe |
| **完整，但 webu 要接 hook 畫 UI** | alert / confirm / prompt、離頁確認、HTTP auth、檔案上傳、下載、hover 選單（§5） |
| **有損** | 圖片與多媒體只給佔位框；a11y 做爛的站靠 click handler 補救；拖拉排序；顏色與位置才有意義的資訊 |
| **做不到** | 影片音訊：佔位框，Yank url 拿去別處開。canvas / WebGL app、CAPTCHA、passkey / Touch ID（WebAuthn 是 OS 層對話框）、WebRTC：交棒給同 profile 的有視窗 Chromium（§9.1） |

### 開發者日常網站的估計

| 類別 | 例子 |
|---|---|
| 完整可用 | GitHub、GitLab、Jira、Confluence、Stack Overflow、Hacker News、各種 docs 站、Google 搜尋、大多數後台管理系統、論壇 |
| 可用但笨重 | Gmail、Slack web：a11y 好但節點極多，靠 outline 與摺疊 |
| 看得到字、主功能廢掉 | Grafana 與任何以圖表為主的 dashboard |
| 不可用 | YouTube、Google Docs（canvas 渲染）、Figma、地圖 |

以「開發者一天會開的頁面」算，八到九成落在前兩類。

### 免費附送

站在 CDP 上，GUI 瀏覽器要開 devtools 才看得到的東西可直接當功能：
network log、console 訊息、cookie 檢視、`Page.printToPDF`、整頁截圖。
目標用戶是開發者，這些是賣點而非附件。

---

## §8 Shell 功能清單

Chromium 引擎只負責「給一個 URL，把頁面跑起來」；使用者感覺是「瀏覽器」的其餘部分
全是 Chrome 的 UI 殼在做，webu 換掉了那個殼所以要自己做。每項都是「拿 CDP 資料自己畫」
或「自己存一個檔」。**做到哪一級在 `ui.md` / `ux.md` 決定**，因為每一項都是一個 surface；
本節只列清單、做法、成本。

| 功能 | 怎麼做 | 存檔 | 成本 |
|---|---|---|---|
| 分頁 | 一個 target 一個分頁。`Target.createTarget` / `closeTarget`；`target=_blank` 自動產生新 target，監聽 `targetCreated` | 無 | 低 |
| 輸入網址 | goto popup。非 URL 輸入當搜尋，預設 DuckDuckGo html 版（`ux.md` §7；2026-09-23 起 —— Google 對 headless 回 reCAPTCHA，09-21 曾改 Google） | 無 | 低 |
| 上一頁下一頁 | `Page.getNavigationHistory` / `navigateToHistoryEntry`，每分頁各一份 | 無 | 極低 |
| 頁內搜尋 | 對 IR 文字做，命中後 cursor 停到最近的 item；折行後高亮要處理。不用 `DOM.performSearch` | 無 | 低到中 |
| 憑證錯誤 | 自簽憑證會讓導航直接失敗。`Security.certificateError` 事件接住、confirm「要繼續嗎」，等同 Chrome「進階 → 繼續前往」。ops 用戶必要 | 無；第一版每次問，不記住 | 低 |
| 錯誤頁 | DNS 失敗、連線拒絕：`Page.navigate` 回 errorText，畫成空狀態（事實 + 提示） | 無 | 低 |
| 歷史 | Chrome 的 History 資料庫在它跑著時鎖住，只能自存：時間、URL、標題，append-only。查詢用 filu 的原生 finder 做 fuzzy | 自存 | 存低、UI 中 |
| 書籤 | URL、標題、可選 tag，一個 yaml | 自存 | 低 |
| session 還原 | 離開時寫下所有分頁 URL，下次開回來。與「離開時殺 Chromium」綁定 | 自存 | 低 |
| 下載 | `Browser.setDownloadBehavior` 指定目錄 + 事件回報進度。toast 開始 / 完成 + `[D]ownloads` screen（2026-09-20 / 21） | `config.yaml` 的 `download_dir`，預設 `~/.webu/datas/downloads`（2026-09-21） | toast 低、清單中 |
| Proxy | `--proxy-server` flag；macOS 吃系統 proxy，Linux 不一定 | 設定檔 | 低 |
| network log | `Network.enable` 後每個 request 有事件；面板列 method / status / URL / 耗時，可看 body。量大要過濾 | 無 | 中高 |
| console | `Runtime.consoleAPICalled` + `Log.entryAdded`，一個 viewport | 無 | 低到中 |
| cookie 檢視 | `Network.getCookies`，一張表 | 無 | 低 |
| print to PDF | `Page.printToPDF` 存檔，一個呼叫 | 無 | 極低；**不做**（2026-09-20） |
| 整頁截圖 | `Page.captureScreenshot(fullPage)` 存檔 | 無 | 極低；**不做**（2026-09-20） |
| view source | `DOM.getOuterHTML` 進 viewport | 無 | 極低 |
| 清除某站資料 | `Storage.clearDataForOrigin`，登入壞掉時用 | 無 | 極低 |
| 隱私分頁 | `Target.createBrowserContext` 隔離 context，cookie 不落地 | 無 | 低 |
| reader mode | IR 層只留 main landmark，是過濾不是殼功能 | 無 | 低 |
| 對話框 / 檔案選擇 / auth | §5 hook | 無 | 必要，不算選項 |
| 設定 | profile 目錄、下載目錄、proxy。Chromium 版本與路徑不開放 | 自存 | 低 |

**不做，明講**：密碼管理與自動填入（Chromium 的密碼儲存在 headless 無 UI，靠 profile
持久保存 cookie 讓登入不常發生）；擴充套件；多 profile。

**成本觀察**：同一「級」內成本不均。print to PDF、截圖、view source、清除資料是一個 CDP
呼叫加存檔的十行功能；network log 才是真的要花時間的。分級時以成本為準，不以「像不像
devtools」為準。

---

## §9 Chromium 的取得與執行模式

**自帶、釘死版本、headless、無視窗。** 不 attach 使用者的 Chrome：Chrome 136 起
`--remote-debugging-port` 拒絕配預設 user-data-dir，企業管控的 Chrome 也常以
policy 關掉 remote debugging，那條路已死；而且沒裝 Chrome 的機器也要能跑。
Playwright、Puppeteer、Electron 都是自帶 Chromium，這是標準做法。

| 項目 | 決定 |
|---|---|
| 二進位 | 首次啟動下載釘死 revision 的 Chromium 到 `~/.cache/webu/chromium-<rev>/`，明講大小（150–200 MB）。go-rod 的 `launcher` 套件可單獨用來下載，抓完把 websocket URL 交給 chromedp 的 `RemoteAllocator` |
| 版本覆寫 | **不開放**。翻譯層的 heuristic 與 fixture 只對釘死的版本負責 |
| 執行模式 | 一律 `--headless=new`：完全沒有視窗、不進 Dock、在背景 offscreen render。與有視窗的 Chromium 是同一個引擎 |
| profile | webu 自己的持久目錄，登入一次即記住；不碰使用者任何 Chrome profile |
| 啟動 flag | 拿掉 chromedp 預設的 `--enable-automation`，否則 `navigator.webdriver` 為 true，Google 登入等站會擋 |
| User-Agent | **webu 自己的名字，不是偽裝**（2026-09-23）：拿 Chromium 自報的字串，把引擎名從 `HeadlessChrome/<ver>` 寫成每個 Chromium 系瀏覽器都這樣寫的 `Chrome/<ver>`，尾巴簽 `webu/<version>`；client hints（`Sec-CH-UA`）同一套：brands 是 `Chromium` 與 `webu`。「Headless」是模式的名字不是瀏覽器的名字，而頁面看到它就回 bot-check 而不是頁面（Google 的 `/sorry/`）。每個分頁在 prepare 時套上（`Emulation.setUserAgentOverride`） |
| 離開時 | 殺掉 Chromium，同 u-family「離開時放掉所有子行程」慣例 |
| 安全更新 | Chrome 四週一版修安全漏洞，釘死的版本會老、且握著真實 session cookie。要有 `webu browser update` 並在版本過舊時提醒 |
| Linux 無頭伺服器 | 二進位仍需 libnss3、libatk、字型等系統函式庫；附檢查腳本或 apt 清單 |

### §9.1 逃生口

TUI 做不了的東西分兩種，出口不同：

| 種類 | 例子 | 出口 | 登入狀態 |
|---|---|---|---|
| **內容本身是多媒體** | 影片、音訊、大圖、地圖 | 第一版不做出口：佔位框 + Yank url。內建 viewer 方案擱置（§4 多媒體） | 不需要共享 |
| **卡在 session 裡的關卡** | CAPTCHA、passkey / Touch ID、canvas app、拖拉 | **第一版不支援**：偵測到 reCAPTCHA / hCaptcha / Turnstile iframe 就畫佔位框明講「webu 做不到」+ Yank url。交棒機制擱置（下） | 必須共享，不然過了關 webu 也不知道 |

不做終端機內的圖片協定或 screencast 視覺模式（§4 多媒體）。

**交棒機制（擱置，日後再議）**：

1. webu 記下所有分頁的 URL（與 session 還原共用同一份），關掉 headless 實例
2. 以同 profile 啟動有視窗的 Chromium，只開該 URL
3. 使用者處理完、關掉視窗 → Chromium 行程結束 → webu 偵測到，重啟 headless、還原所有分頁
4. 代價：頁面內的記憶體狀態（未送出的表單、SPA 的 in-memory state）會掉；cookie / localStorage 不會

**沒有 display 的環境**（SSH 進遠端）：Browser 這列是 disabled，按下去說明原因；
Yank url 仍可用，使用者拿到本機開。

**隱形 CAPTCHA 多半會過**：reCAPTCHA v3 與 Cloudflare managed challenge 只看瀏覽器像不像正常人；
webu 拿掉 `--enable-automation`、走 `--headless=new`、profile 持久帶 cookie，大多直接通過。
真的跳出圖片挑戰的才過不了。

替代方案只有「用系統瀏覽器另開 URL、登入不共享」，那 CAPTCHA 就真的過不了。

---

## §10 待決事項

| # | 決定 | 傾向 |
|---|---|---|
| 1 | ~~attach 還是 headless~~ | **已決（2026-09-20）**：自帶釘死版本的 Chromium、不開放覆寫、一律 headless 無視窗，見 §9 |
| 2 | ~~Shell 功能做到哪~~ | **已決**：逐項落點與版次在 `ui.md` §7；四個附帶問題已決（非 URL 當搜尋、下載目錄 config、憑證每次問、歷史與書籤是 header 開的 popup） |
| 3 | ~~逃生口~~ | **已決（2026-09-20）**：多媒體只畫佔位框；CAPTCHA 等 session 關卡第一版不支援、明講並給 Yank url；交棒機制擱置（§9.1） |
| 4 | ~~翻譯層品質底線~~ | **已決（2026-09-20）**：不以站為單位保證，以 **AX role 白名單**為單位（§3）。未支援 role 一律 fallback、不隱藏，Enter 仍可 click。站只當 smoke test |

其他不用決定，Chromium 已經替 webu 決定了。

**function.md 到此為止**：功能面全部定案（2026-09-20）。

---

## §11 MVP 驗收

**驗收單位是 role 白名單**：v1 白名單內每個 role 都有 fixture 且通過（§3）。

以下三個站是 smoke test，三種 a11y 型態，各要能**登入、能點、能填表**，未支援的 role
要以 fallback 形式看得見、不崩：

| 站 | 測什麼 |
|---|---|
| Hacker News | 靜態、table 排版、無 landmark → 翻譯層 heuristic |
| GitHub | 半 SPA、正規 landmark → 基本盤 |
| 任一 Vue / React 後台 | 純 SPA、空殼 HTML → JS 執行與等待策略 |

加一個有 SSE 或 WebSocket 即時更新的 dashboard，驗 §6 的偵測與 cursor 留位。

---

## §12 CLI

| 指令 | 行為 |
|---|---|
| `webu` | 還原上次 session 的分頁（不預先載入，切到才載）；`config.yaml` 的 `restore_session: false` 則從空白開始，session 仍照存 |
| `webu <url|words> ...` | 還原 session 並為每個參數多開一個分頁（第一個在前、焦點在它）；參數的解讀同 Location 框：沒 scheme 的 host 補 `https://`、不像 URL 的字當搜尋（修訂 2026-09-21：原本只收一個 URL） |
| `webu browser update` | 更新釘死的 Chromium revision（§9） |
| `webu version` | webu 版本與 Chromium revision |
| `webu help`（`-h` / `--help`） | 用法；其他 `-` 開頭的參數視為未知選項，印用法並以 2 離開 |

首次啟動：先下載 Chromium（§9，明講大小），再進 TUI。

---

## 附錄 — AX tree 驗證程式（chromedp）

§3 實測數據的來源。`go mod init` 後 `go get github.com/chromedp/chromedp`，
`go run . <url>`。

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

func val(v *accessibility.Value) string {
	if v == nil || v.Value == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err != nil {
		return string(v.Value)
	}
	return s
}

func main() {
	url := os.Args[1]
	limit := 60
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var nodes []*accessibility.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			nodes, err = accessibility.GetFullAXTree().Do(ctx)
			return err
		}),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	byID := map[accessibility.NodeID]*accessibility.Node{}
	for _, n := range nodes {
		byID[n.NodeID] = n
	}
	skip := map[string]bool{"generic": true, "none": true, "StaticText": true, "InlineTextBox": true}
	printed, semantic := 0, 0
	var walk func(id accessibility.NodeID, depth int)
	walk = func(id accessibility.NodeID, depth int) {
		n := byID[id]
		if n == nil {
			return
		}
		d := depth
		if !n.Ignored && !skip[val(n.Role)] {
			semantic++
			if printed < limit {
				fmt.Printf("%*s%s %q\n", depth*2, "", val(n.Role), val(n.Name))
				printed++
			}
			d = depth + 1
		}
		for _, c := range n.ChildIDs {
			walk(c, d)
		}
	}
	walk(nodes[0].NodeID, 0)
	fmt.Printf("\n[raw AX nodes: %d] [semantic after skipping generic/text: %d] [printed: %d]\n", len(nodes), semantic, printed)
}
```
