# webu 開發者備忘

README 只介紹這個工具怎麼用；這份收的是屬於開發者的部分：webu 裡面怎麼運作、設計為什麼這樣定、文件怎麼讀、怎麼建置與發布。webu 遵循 [terminu design principle](https://github.com/vulcanshen/terminu)（tdp v0.7.4），畫面建在家族的共用 lib [terminu-go](https://github.com/vulcanshen/terminu-go)（v0.1.13）上，是 terminu family 在瀏覽器領域的成員。

---

## 運作方式

真正的 Chromium 在背景 headless 跑；webu 取它為 screen reader 維護的 **accessibility tree**，加上同一棵 DOM 的 **版面快照**（DOM snapshot），靠 `backendNodeId` 對接，合成一份能翻的文件。每個操作都經 Chrome DevTools Protocol 打回 Chromium，所以頁面就是真的頁面。

- **語意來自 accessibility tree，版面來自 DOM** —— screen reader 讀到的就是你看到的：role、名字、狀態；東西在頁面的哪裡來自版面快照。不讀 CSS 的顏色字型。webu 不認得的 role 畫成文字加標記，從不隱藏、仍然可點。
- **每個支援的 role 都有 fixture**，對著釘死的 Chromium revision 抓，所以引擎升版是一個決定而不是漂移。支援清單在 [`support.md`](support.md)，由 role 表產生。
- **一個釘死的 Chromium** —— revision 編進執行檔、不開放覆寫、永遠不碰使用者自己的 Chrome；webu 自己的持久 profile。頁面的問題都是 Chromium 的問題，webu 只畫答案。
- **一份登記、lib 產生其餘** —— webu 把每個畫面、panel、動作（鍵、名稱、說明、範圍、要不要先問、做什麼）登記給 terminu-go；Space menu、global operation popup、`?` 的 key reference、panel 下框的 hint、footer、confirm、淡化、`Esc` 一次一層都由 lib 從這一份登記產生，所以不在選單裡的熱鍵不可能存在。[2] 的動作跟著游標下的東西換，每次換頁、換游標都重新登記（`pageactions.go` 的 `syncPage`）。
- **畫面穩定** —— 任何尺寸、任何內容，每一列都恰好是終端機的寬；`TestFrameFitsTheTerminal` 在 icon 一格／兩格、powerline 符號一格／兩格下，跨尺寸、每個畫面、疊 popup 檢查（tdp L4、E5、E6）。
- **`[S]ettings` 與 `config.yaml` 同步** —— 檔案裡有的 key，Settings 一定有一列，有測試守著。
- **存檔** —— 書籤每筆帶 `folder` 路徑，另有 `folders:` 清單讓空目錄留得住；歷史是一次 append 一筆的 YAML sequence；每次寫檔都是原子的。分法：使用者寫的在 config、webu 產生的在 data、可重抓的在 cache。
- **unix-first、靜態執行檔** —— macOS + Linux，`CGO_ENABLED=0`。chromedp 的 log 進檔案，永遠不進 TUI 正在畫的終端機。
- **Chromium 不在 release 裡** —— release 只有 Go 執行檔；第一次啟動下載到 cache。Chromium snapshot bucket 沒有 Linux ARM 版，所以沒有 Linux ARM build。
- **Chromium 的啟動與關閉** —— flag 是 Puppeteer 那組減 `--enable-automation`，加 `--disable-blink-features=AutomationControlled`、`--headless=new`，三個 background throttling 關閉。下載串流到 `.zip.part`、解到 `.tmp`、最後 rename，保留 symlink（Mac bundle 的 `Versions/Current`）。關閉先 `chromedp.Cancel`（讓 profile flush）再 allocator cancel；不論從哪裡離開（`q`、外部的 SIGINT / SIGTERM、終端機關掉），Chromium 都跟著走：SIGINT、SIGTERM 由 Bubble Tea 自己收，終端機關掉的 SIGHUP 它不收，`main.go` 把它轉成對自己的 SIGTERM。
- **分頁的三種跑法** —— `press`（使用者的動作：設 loading / `settlingUntil` / `popupUntil`）、`act`（webu 自己的：Reveal、開 frame）、`unblock`（對話框 / auth 的回答：不排隊）。**action lock**（`tab.acting`）：讀 box 與按下之間曾被舊的 Reveal 捲走（六次漏一次），鎖住讓它們是一個動作；對話框開著時碰 renderer 會卡，所以回答不經鎖。動作後 300 ms `settle` 再 capture；`gen` 丟掉過期的。
- **hover 送了不等** —— headless 下單獨的 `mouseMoved` 要等 renderer ack 5 秒，但頁面當下就處理了，所以 hover 用 100 ms 的 ctx 送出就走。

### 量出來的事實（不再重查）

| 事實 | 影響 |
|---|---|
| headless 下單獨的 `mouseMoved` 要等 renderer ack 5 秒，但頁面當下就處理 | hover 送了不等（100 ms ctx） |
| `Security.certificateError` 已從協定移除 | 用 `setIgnoreCertificateErrors` |
| `Page.getFrameTree` 不列 OOPIF；`DOM.describeNode` 給 frame id；瀏覽器允許 attach iframe target；frame id 從 1 重編 | 跨站 frame 走自己的 session、id 加偏移 |
| chromedp 對 page target 開了 `Target.setAutoAttach(flatten)`，但不替 iframe session 建 executor、訊息全丟 | 自己 `NewContext(WithTargetID)` attach 一條 |
| 同站 frame 在自己那層讀整張 owner 表會找到自己（無限遞迴到 15 秒 timeout） | `frameOwners` 只看當前 document |
| 原生 `showModal` 砍掉 dialog 以外整棵樹、疊 modal 時連第一層也砍；aria-modal 不砍 | backdrop 用前一刻的版面；彈窗 stack push 不 drop |
| 數字經 float32（0.6 變 0.6000000238418579） | `cleanNum` 洗成六位 |
| password：Chromium 把 value 遮成 `•`、AX 不說；空欄位只有 snapshot 的 `type=password` 認得出 | password 從 snapshot 屬性判斷 |
| 沒有 option 的 ARIA combobox（Google 的搜尋框是 `<textarea role=combobox>`） | 當 Textbox |
| Google 對 headless 一律 reCAPTCHA（即使 UA 簽自己的名）；十二個搜尋引擎實測 DDG html 最乾淨 | 預設搜尋 |
| 內文裡的連結串最長 5–8（HN 全頁 2）、導覽 12–47 | 裸連結串只流動、不收合 |
| w3schools 沒有任何 landmark；沒 main 的頁把側欄連結欄當「節」 | part 靠幾何；`pruneNav` |
| 表單第一欄的 caret 掉到下一列 | `formValueW` 沒算 caret 前的空格（2026-09-23 修） |
| `tea.Sequence` 巢狀不等內層 | 先後執行寫成一個 cmd |
| Bubble Tea value receiver：改 popup 狀態的 helper 若是 value receiver 只回 `tea.Cmd`，改到的是副本 | pointer receiver，或先 `close()` 再 `dispatch()` |
| 有 timer 的頁面每秒指紋都不同 | spinner 轉滿 8 秒 grace（已知） |

### 程式碼目錄

```
cmd/webu/            進入點：version / browser update / 首次下載 / 啟動 Chromium / 進 TUI
internal/browser/    Chromium 的取得與執行：釘死 revision、目錄、flag、UA、關閉
internal/ir/         翻譯層：role 單一宣告表（roles.go）、AX tree + snapshot → IR（build.go）、
                     Dump / Markdown、fixture、docs/support.md 產生器
internal/page/       CDP 端：Capture（capture.go）、動作（actions.go）、hook（hooks.go）、
                     觀察與 DevTools 資料（observe.go / devlog.go）、跨站 frame 的 session（sessions.go）
internal/ui/         TUI，建在 terminu-go 上：app.go（AppModel、向 lib 登記、onMsg、sync）、
                     web.go（Web 畫面：[1] Tabs、版面）、pagebody.go（[2] 的 Body：網址列、內容、
                     頁面的彈窗）、pageactions.go（[2] 的動作、Enter、各種輸入）、pageask.go（頁面的
                     alert／confirm／prompt／登入）、visual.go + selectmode.go（visual mode）、finder.go
                     （/ 的索引與落點）、tab.go（分頁模型：capture / apply / settling / places / drill /
                     移動）、render.go（IR → rows / items / marks）、parts.go（四個 part）、section.go +
                     sectionlist.go（目錄與節）、pagepopup.go（頁面彈窗的判定）、bookmarks.go /
                     history.go / downloads.go / settings.go / devtools.go（其他畫面）、splash.go +
                     icon.svg（docs/icon.svg 的副本）、width.go（lib 的寬度函式）、theme.go（頁面的配色與 glyph）
internal/store/      config / bookmarks / history / session 的 YAML；Netscape 書籤匯入
internal/paths/      三個目錄的解析
tools/axdump/        看任何頁面的 AX tree（本機 Chrome）
```

依賴方向：`ui → page → ir`、`ui → browser`；`ir` 不依賴 CDP 以外的任何東西。`page/sessions.go` 用 `ir.CarriedFrom` 當 id 偏移的單位，是 page 對 ir 唯一的反向依賴（一個常數）。

### 實作落地

設計文件（`function.md`、`ui.md`、`ux.md`）講該怎樣；這一節記程式裡實際怎麼做、對應哪些名字。

#### Chromium（`function.md` §9）

| 項目 | 落地 |
|---|---|
| revision | `browser.Revision = 1701465`（Chromium 156 線）；Mac_Arm 175 MB、Mac 195 MB、Linux_x64 248 MB；Linux arm64 快照桶沒有 |
| 目錄 | cache `WEBU__CACHE` > `XDG_CACHE_HOME/webu` > `os.UserCacheDir()/webu`；config `WEBU__CONFIG` > `XDG_CONFIG_HOME/webu` > `~/.config/webu`；data `WEBU__DATA` > `~/.webu/datas` |
| UA（2026-09-23） | `Browser.getVersion` 拿自報字串，`HeadlessChrome/` → `Chrome/`，尾巴 ` webu/<ver>`；client hints brands `Chromium` + `webu`；每個分頁 `Prepare` 時 `Emulation.setUserAgentOverride`（`browser.identify` / `userAgent` / `uaMetadata`；`TestWebuSignsItsUserAgent`） |
| log | chromedp 的 log 全部進 `<data>/webu.log`，**絕不到 stderr**；`unhandled … event` 丟掉 |

#### Capture：一次抓什麼（`function.md` §3）

`page.Capture(ctx, frames)` → `ir.Capture`：

| 欄位 | 來源 | 用途 |
|---|---|---|
| `Nodes` | `Accessibility.getFullAXTree` | 語意 |
| `Display` | `DOMSnapshot.captureSnapshot(["display"])` | block / inline（`generic` 對 div 與 span 一視同仁） |
| `Boxes` | snapshot 的 layout bounds | 四個 part、彈窗的「疊在別的東西上」、sr-only |
| `Hidden` | box ≤ 1×1 | sr-only 丟掉（2026-09-22） |
| `Parents` / `Anchors` | snapshot 的 DOM 父子表、`id` | 頁內錨點；彈窗判定排除祖先 / 子孫 |
| `Protected` / `Types` / `Srcs` / `Current` / `Breadcrumb` / `Skip` | snapshot 的屬性（`attrMarks` / `attrValues`） | password；input popup 邊框；frame 的 src；aria-current；breadcrumb / skip 標記 |
| `FrameOf` / `Frames` / `Base` | snapshot 的 `ContentDocumentIndex` + `DOM.describeNode` + sessions | frame 是可進去的一層 |
| `Viewport` | `Page.getLayoutMetrics` | 頁面比視窗矮就不切 part |
| `ContentType` | `document.contentType` | 非 HTML 整份 code block；PDF 不支援 |

#### Build：AX tree 上實測出來的事

- **`url` 就在 AX node 上**（link 的 href、image 的 src）；`<iframe src>` 不在，從 snapshot 補。
- **contenteditable**：`generic` + `focusable` + `editable="richtext"` → 多行 Textbox。
- **`<select>`**：`combobox`，option 在 `MenuListPopup` 子樹。
- **spinbutton**：值在 `valuetext`。**slider / progressbar / meter**：`valuemin` / `valuemax` 進 `Min` / `Max`，值是 AX `value`。
- **listbox 的 `multiselectable` 傳到每個 option**（畫 check 還是 radio）；**`aria-expanded` 存在就是 `Expandable`**（tree 的枝 vs 葉）；`focused` / `modal` / `invalid` 各一個 bool。
- **Chromium 的 role 拼法**：`Iframe`、`Canvas`、`Date`、`DateTime`、`InputTime`、`ColorWell`、`DisclosureTriangle`、`LayoutTable*`、`MenuListPopup`、`sectionheader` / `sectionfooter` 都照它拼。日期時間顏色框的 picker 零件（年 / 月 / 日 spinbutton、picker 按鈕）是 UA shadow 漏進來的，Textbox 本來就不收 children 所以自然消失。
- **role 表**：`ir.Roles`：role → `{Kind, Action, Transparent, Display}`，`docs/support.md` 由 `SupportDoc` 產生、`TestSupportDoc` 守著不漂；未列的 → `Unsupported`。
- **非 HTML**：取樹裡最長的文字節點當內容、JSON `json.Indent`、整份 `Document{Code}`。
- **breadcrumb 沒包 nav 的**、**skip 區塊**：build 時包成 `Landmark{navigation}`。

password、沒有 option 的 combobox、float32 的數字見上面「量出來的事實」。

#### frame：三條路一個出口（2026-09-23）

- **同 process**：snapshot 的 `docs` 已含所有同 process 文件；`frameOwners(docs, strs, cur)` 只看**當前 document** 的 `ContentDocumentIndex`；`getFullAXTree.WithFrameID` 取樹。
- **跨站**：`DOM.describeNode(<iframe>)` 給 `FrameID`（= target id，`Target.getTargets` 列為 `iframe` 型、已 auto-attach）；`chromedp.NewContext(tabCtx, WithTargetID(frameID))` attach 一條自己的 session。在那條 session 上 `getFullAXTree` / `captureSnapshot` / `getBoxModel` / `Input.dispatchMouseEvent` 都正常，座標是 frame 自己的、Chromium 會轉。`page.Sessions`（`WithSessions` 掛在 tab ctx、`NewSessions` 的 `tab` 也帶著它所以巢狀派生得到）：`frame(id)` 懶 attach（5 秒沒回就放棄）、`forget`、`resolve`。
- **id 撞號**：每條 session 一個 slot，`Base = slot × ir.CarriedFrom`（2^40）；`ir.Build` splice 時 `offsetIDs`（已 carried 的跳過）；每個帶 node id 的動作走 `on(ctx, id, fn)` 解回該 session 與原 id（Reveal / Click / Type / Fill / Slide / Choose / Submit / SetFiles）。
- **box**：`attachFrames.keep` 把 frame 的 Boxes 平移到 owner 的 box、以 carried id 併進主頁（巢狀先在上一層平移），part 與彈窗判定看得到 frame 內容。
- **ui**：`openFrame(n)` 把 frame id 加進 `tab.frames`、`wantDrill`、重抓；`apply` 後 drill 進去；沒抓到就 toast。同站 / 跨站 / 巢狀在 ui 一行都不用分。

#### render：IR → layout（`render.go`）

`layout{rows, items, marks}`：block kind 斷行、inline kind 流式折行；每個 `seg` 記 item、item 記 first / last 列與起始欄（`j/k` 換列、`h/l` 同列靠它）；`marks` 記每個 block 的第一列（目錄、finder、起點都靠它；`markNext` 讓欠著的空列不算成它的）。

- 相鄰 item 之間補空格、標點不補；`<label>` 的文字 = 欄位的 name 就吃掉（`dropLabel`）；沒名字的 link 顯示 URL 最後一段。
- **同一個名字只印一次**：`namedByItsHeading` 的 region 不畫 rule；`dropLabelRow` 只認 landmark / heading 的 mark。
- **thing**（`thing()` / `firstLine`）：多於一行的 ListItem / article 只畫第一行、是 item；drill 時 `renderOpts.drill` 指定根、第一行成 header 列（`drillHead` / `drillBody`）。
- **form**：`formLabelW`（最寬 label，不截）/ `formValueW`（glyph + 最寬值 + 空格 + caret）決定框寬，擠不下就 `fitForm` stack；`formField` 畫一列；`fieldset` 畫 legend；`valueKind` 把 invalid 畫紅。
- **table**：每格一個 item（`cellItem`），欄寬從最寬的欄縮；`row.table` / `row.header` 讓 pagepanel 鋪底。
- **tabChain**（tablist）：`capLeft` + 段 + `dividerSoft` + `capRight`，`segTabOn/Off/Cap`；row 記 `heading = len(r.heads)`（所在節的深度），`rowLine` 用 `levelColor`。
- **treeItem**：`Level` 縮排、`Expandable` 決定 `▾`/`▸`/空。**optionText**：`Multi` 決定 check / radio。
- **slider / gauge**（`slider.go`）：12 格軌道 `●` / `━━━───`；`sliderItems` 造數字清單（>10000 格以十為步）。
- **DisclosureTriangle**：Button 分支換 `▸ `/`▾ ` 當 lead。**tooltip**：Group 分支 `glyphInfo` + segDim。
- **裸連結串**：`linkRun` 標出連續兩個以上的 `bareBlockLink` 走 inline。
- **inline 標記**：`textAttr` 位元欄疊在 kind 樣式上（`withAttr`）。
- **measure**：只管 `wrap()` 的 `textW`；`store.Measure` 收 `full` / 數字。
- **行號**（`lineNumW` / `lineNum`）：gutter 從文字寬扣、面板寬不變；`relayout` 先用上一頁的位數排、位數變了再排一次；drill 進一件事時從 body 起算、彈窗裡 gutter 0。

#### parts（`parts.go`，2026-09-23）

`splitParts(root, boxes, viewport)`：只看頂層區塊的幾何——最大面積是 body；`beside`（垂直重疊）是 others；前面 header、後面 footer。守門：頁面高 ≤ viewport、或最大區塊 < 25% 面積 → nil（整頁一塊）。`tab.parts` / `at` / `activePart`；`relayout` 從 `sansPopup(root)` 切、`base` 是 active part 的節點，`at` 是 `partWhole` 時是整頁。part 是 [2] 的 panel tab：第一個 tab 是整頁 `PAGE`，後面是這一頁有的 part（`syncPage` 填 `Panel.Tabs`／`Tab`），`h|l` 由 lib 換 tab、`partTab` 再 `showPart`。彈窗開著時不顯示 tab。

#### sections（`section.go` / `sectionlist.go`）

`sectionsOf(root, lay, url)`：從 `marks` 收 heading（scope 是 main）；`(top)` 是第一個標題前的內容；**一節的 `last` 延伸到下一個同級或更高級標題**（2026-09-23，原本是下一個任何標題——MDN「Try it」因此是空的）；`start()` 把 landmark 的 rule 併進下一節；`count` 數表 / code / 媒體 / 散文 / 連結；`pruneNav` 丟掉只有連結、沒子節的「節」（併進前一節，用 max 不縮短父節）；深度用 stack 算（h1 → h3 → h4 是三層）。`shapeOf`：≥ 3 個標題是 `shapeDoc`。`tab.listing()` / `read` / `sec` / `secTop` / `flat`；`openSection` / `closeSection` / `stepSection`（`siblingSection` 同 level）/ `sectionAt`（最內層）/ `rowRange`（讀一節時 `body..last`）；`recut` 重抓後用 heading 的 node id 留在原節；`headingTitle` 去掉標題尾巴的同頁錨點連結。讀一節時網址列下多一列 `☰ 2/26 節名`（`headRowOf`），下框右邊是 lib 的捲動位置 `N-M of T`。

#### drill、places、finder、go

- **drill**：`drillInto(n)` / `leaveDrill`；`chainTo` / `chainOf` 算祖先路徑；`enterItem` 對一行的 ListItem 直接 `firstItemIn` → `enterOn`；`Media` 有 `Frame` 就 `openFrame`；Code → `showCode`。
- **places**（`tab.go`）：`entry`（`Page.getNavigationHistory` 的 current entry id）、`places[entry] = place{at, flat, read, sec, top, cursor}`；`apply` 的 `fresh` = entry 不同（URL 判斷會被 settle 重設）。
- **finder**（`finder.go`）：`indexPage` 走四個 part、`partOf` 記 part、Combobox 的子節點跳過、Option 可命中；`hit{node, part, trail, under, text, line}`；清單、打字列、比對、預覽的框是 lib 的 `Finder`（比對照 components/input/finder），預覽是 `previewOf`；`goToHit` 用 `chainOf` 沿路 drill、`rowOf` 找列。`go`：lib 的 `NumberInput`（1 到行數）→ `goToLine`。
- **Esc 鏈**（`pageactions.go` `pageEsc`，lib 的 `Panel.OnEsc`）：drill → 彈窗（拒絕，toast）→ read（回目錄）→ 到頂什麼都不做。popup 的 `Esc` 是 lib 的。

#### 頁面的彈窗（`pagepopup.go`，2026-09-23）

`noticePopup()` 在 `apply` 時跑：`findPopup(prev, skip, root, boxes)` 走整棵樹找 `prev` 沒有的子樹根；`isPopup(c, chain, boxes, parents)`：`countItems > 0`、且（`Modal` / dialog / alertdialog / menu / `hasFocus`）或 `overlap`「疊在無關的 box 上一半以上」（`parents` 排除 DOM 祖先 / 子孫——`main` 在 Accept 之後曾被誤判）。`popups` 是 stack：同一次 capture 有新的就 push、不丟舊的；`popupLays` 存下層的 layout 畫在後面（`pagePopupFloats` 錯開 3 欄 1 列）；`back` / `backParts` / `backRead` / `backSec` / `backTop` 存彈窗出現前的版面與位置，答完復原；`popupUntil` = press 後 8 秒；載入時 prev 空 → 全部算新（載入就在的彈窗算）。`popupWidth` / `popupVisible`；`pageBody.Draw` 用 `backdrop()` 畫底下的頁面、`terminu.Dim` 淡化，再 `pagePopupFloats` 疊上去（`terminu.Composite`）。

#### 兩套配色（`theme.go`）

外殼的顏色（框、膠囊、cursor、hint、footer、popup 的層色）與淡化都是 terminu-go 的（components/color）：失焦的 panel 與 popup 底下的一切由 lib 改寫 SGR 淡化（tdp F8、T2）。`theme.go` 只剩頁面自己的配色（Page palette：`pageText` / `pageClick` / `pageInput` / `pageCode` / `pageMedia` / `pageInvalid` / `levelInk`…）與頁面內容用的 glyph；網址列與行號用 Blue（`urlColor`、`focusColor`），頁面上的 cursor 用 Subtext1 底，跟 lib 的 panel cursor 一樣。頁面內容照舊用 lip gloss 上色，lib 淡化時照樣改得到它的 SGR。

#### 動作（`page/actions.go`、`sessions.go`）

每個 `page.*` 入口都經 `chromedp.Run`（`run` / `on`）；`on(ctx, id, fn)` 先用 ctx 裡的 Sessions 解 id。分頁怎麼排這些動作見上面「分頁的三種跑法」。

| 動作 | CDP |
|---|---|
| Click | `reveal` → `getBoxModel` → `hover`（`mouseMoved`，100 ms、送了不等）→ pressed / released 在中心；沒 box 才 `el.click()` |
| Type | focus → select all → `Input.insertText`；空字串是 Backspace |
| Fill（2026-09-23） | `el.value = …` + `input` / `change`（date / time / color 等 insertText 打不進去的框） |
| Slide（2026-09-23） | `<input type=range>`：Fill；ARIA：focus 後 ArrowLeft / Right 走到 `aria-valuenow` 到位或不再動（上限 4096 步） |
| Choose | option `selected = true` + select 的 `input` / `change` |
| Submit | focus + Enter keydown / keyup |
| Key | Enter / Escape / ArrowLeft / ArrowRight |
| Entry | `Page.getNavigationHistory` 的 current entry |

#### 非 DOM 事件（`page/hooks.go`）

| 事件 | 落地 |
|---|---|
| JS dialog | `dialogMsg` → `pageask.go` 的 popup（Enter 答、Esc 取消；prompt 的 `e` 改答案）；`handleJavaScriptDialog` 回答；開著時 settle 暫停 |
| `target=_blank` | `Target.targetCreated`（`OpenerID` 有值）→ `NewContext(WithTargetID)` → `Prepare` → 加到尾端切換 |
| 憑證 | `ERR_CERT_*` → confirm → `setIgnoreCertificateErrors(true)` 重載；每分頁問一次 |
| 下載 | `setDownloadBehavior` + 事件 → toast / Downloads screen / header 進度條 |
| HTTP auth | `Fetch.enable(handleAuthRequests)`（每個 request pause 一次、goroutine 裡 continue）→ `pageask.go` 的登入 popup（`n` 名字、`p` 密碼、Enter 送出、Esc 取消）→ `continueWithAuth` |
| 檔案上傳 | `fileChooserOpened` → lib 的 file-picker（`importDir()` 起）→ `SetFiles` |

## 設計決定

0.2.x 把 accessibility tree 排成一長頁；0.3.0 把它讀成一份**文件**。各項的完整理由與日期在設計文件裡，這裡是摘要：

- **一份文件三個畫面。** 有三個以上標題的頁面開頁先進**目錄**：一列一節、依深度縮排、右欄是它有幾張表、幾段 code、幾個媒體、多少行。`Enter` 讀**一節** —— 一節包含它的子節，章包含節 —— `n`/`p` 在同深度的節之間走、`Esc` 回目錄。`Space` › `One sheet` 攤成一整張，`Sections` 再切回來。
- **四個 part，靠幾何切。** 頁面上最大的區塊是 **body**；在它旁邊的是 **others**；在它前面的是 **header**、後面的是 **footer**。不看標籤叫什麼 —— `nav` 也好、`div` 也好，位置決定。頁面比視窗矮、或沒有一塊獨大的頁面就是一塊（[2] 沒有 tab）。
- **finder，不是逐字搜。** `/` 的範圍是四個 part 裡每一個持有文字的區塊；清單上 `Enter` 是**去那裡**（切 part、開節、走進項目、游標落定），什麼都不按。`go` 加數字跳行；目錄上的行號就是節的序號。
- **一件事一列。** 佔好幾行的清單項目或 article 只畫第一行；`Enter` 走進去、`Esc` 出來，頁面巢多深就走多深。
- **表單畫成表單。** label 一欄對齊、值靠一邊、label 永不截斷（擠不下就疊成兩列）。每一種輸入都定義了互動；日期、時間、顏色框不合瀏覽器的形狀就擋下來，不讓瀏覽器無聲丟掉。
- **頁面自己的彈窗靠行為認** —— 按下之後出現、拿到 focus 或自己宣告或疊在別的東西上、裡面有東西可按 —— 從不看標籤。它要一個回答：`Esc` 不關。
- **frame 是一層。** 跨站 frame 是另一個 process、另一個 target，`Page.getFrameTree` 看不到；webu 用 `DOM.describeNode` 拿到它的 target、開一條自己的 session。frame 的 node id 從 1 重新數，所以帶進來時加上 `slot × 2^40`。巢狀 frame 一層一層接。
- **顏色是概念，不是元素。** 可按的 sapphire、可填的 mauve、code pink、媒體灰、pagetab rosewater、填錯的值紅；heading 與目錄用五個 hue 表深度。沒有括號、沒有 emoji，每個控制項以 glyph 開頭。配色 catppuccin-mocha。
- **頁面記得自己的位置**，每個 history entry 各記一份。還在長的頁面（SPA）spinner 一直轉到兩次看到的一樣為止。`Enter` 先 hover 再點。一次一個動作，快速走動不會把點擊拉離目標。
- **webu 簽自己的名字** —— Chromium 自報的 user agent，把 headless 版的 `HeadlessChrome/` 寫成 `Chrome/`、尾巴接 `webu/<version>`。不是偽裝，是一個真瀏覽器的名字。
- **預設搜尋是 DuckDuckGo 的 HTML 端點**，因為 Google 對每一次 headless 搜尋都回 reCAPTCHA。
- **沒有交棒給視窗** —— CAPTCHA、passkey、WebRTC 是牆，webu 明講；webu 必須能在沒有 display 的機器上跑，所以沒有視窗可以交棒。
- **畫面建在 terminu-go 上**（2026-10-18，tdp E8）：框、膠囊、Space menu、global operation popup、key reference、hint、footer、confirm、toast、每一種 input、viewer、finder、splash、淡化、icon 與 powerline 符號的寬度（探測、`WEBU__ICON_WIDTH`／`WEBU__POWERLINE_WIDTH`、`TERMINU__` 的兩個）全部是 lib 的；webu 原本自己做的那一份（popup、spacemenu、dim、width、iconwidth、splash、chrome、listpanel……）刪掉。webu 只畫 [2] 裡面的頁面。`go.mod` 寫 terminu-go 的 tag，開發時用不進版控的 `go.work` 接 `../terminu-go`。
- **頁面的四個 part 是 [2] 的 panel tab**（2026-10-18，WB2 暫定）：第一個 tab 是整頁 `PAGE`，後面是這一頁有的 part；`h|l` 換 tab（tdp K12：有 tab 的 panel，`h|l` 給 tab）。原本網址列下那條 pagetab、`Esc` 上 pagetab、`h|l` 在同一列的 item 之間走都拿掉；`j|k` 照閱讀順序一個 item 一個 item 走（先同一列、再下一列），到頭尾繞回（K12），`u|d` 半頁。
- **[2] 是 [1] 的明細**（2026-10-18，WB7 暫定）：[1] 的游標移到哪個分頁，[2] 就顯示哪個（還沒載入的分頁換到才載）；[1] 的 `Enter` 用 lib 的 ToDetails 把 focus 送到 [2]（tdp K3 唯一可以用 Enter 換 panel 的地方）。
- **DevTools 是一個畫面**（2026-10-18，WB4 暫定）：`I`，[1] 是 Network／Storage／Console／Source 四個 panel tab（各是 lib 的 List，含 panel filter），[2] 是游標那一筆的明細（request 的標頭與 body、console 的全文）；看的是 Web 畫面正在顯示的分頁。
- **visual mode 是 [2] 的 mode**（tdp K11）：lib 畫模式名與模式色、`Esc` 離開、`Space` 不作用、`Tab` 與數字鍵暫停（`Pause`）、`?` 列模式的鍵；模式裡的鍵（`h|j|k|l`、`w|b|e`、`0|$`、`u|d`、`gg|G`、`v|V`、`y`、`/`、`n|N`、`Enter`）是 `ModeKey`，做的事是 `selectMode` 的。`V` 登記在模式的鍵裡，lib 就不把它當 splash（`takesV`）。`/` 開 lib 的 TextInput 問要找什麼，`n|N` 在結果間走。
- **visual mode 是小寫 `v`**：`V` 保留給 splash（tdp S1）。讀一節時的 `n`／`p` 是小寫的 panel operation：大寫的 `N`／`P` 已經是上一頁／下一頁。`[go] Go to line` 用 `g` 開頭（K12 允許 `g` 開頭的兩鍵組合當熱鍵），開 lib 的 NumberInput 問行號。
- **[1] 與 [2] 的熱鍵各自歸各自的 panel**（tdp P3、M4）：`P`／`N`、`L`、`/`、`v` 作用在頁面上，只在 [2] 按得到；`T`（新分頁）兩邊都有。全域動作只有切畫面（`W`／`B`／`H`／`D`／`S`／`I`）與離開。
- **離開只在有下載進行中時先問**（tdp K9 的離開流程由 app 決定）：`syncQuit` 依下載數設 `App.Quit`。
- **Settings 是一列一個值的 panel**（lib 的 `Values`）：文字設定開 TextInput（目前生效的值當灰字提議，`Backspace` 再 `Enter` 是預設）、`input_tab_width` 開 NumberInput、開關是原地翻的 checkbox，設好就存（layout/panel 的一列一個值）。家族共用的 `hide_secrets_in_preview`、`input_tab_width`（tdp E3）也在這裡，交給 lib 的 `App.HideSecretsInPreview`、`App.InputTabWidth`。

## 已否決，不要重提

完整理由在設計文件原地、各自標日期；這裡是索引。

| 否決的做法 | 改成 | 出處 |
|---|---|---|
| 自訂 protocol / 新內容格式、自己 parse HTML、Chromium 像素轉字元 | 真 Chromium + AX tree + 版面快照 | `function.md` §0「排除的路線」 |
| 為單一網站寫 heuristic（2026-09-21） | 只認 AX tree 與 DOM 的版面事實 | `function.md` §0 |
| 交棒給有視窗的 Chromium 讓人過 CAPTCHA（2026-09-23） | 沒有交棒，牆明講 | `function.md` §9.1 |
| `Enter` 開該 item 的選單（2026-09-20） | `Enter` 是左鍵，點了沒意義就走進去（09-21） | `ux.md` §A.0.K |
| landmark 入口行（09-21）、URL 底下的 landmark 膠囊、Outline popup（09-22） | 四個 part 由幾何切（09-23） | `ui.md` §2 `[2]` |
| 一節只到下一個任何標題 | 一節包含它的子節（2026-09-23） | `ui.md` §2 `[2]` |
| heading 用六級底色（09-22） | 五個 hue 輪流表深度（09-23） | `ui.md` §2 `[2]` |
| 頁面的彈窗佔整個面板 | 浮在 `[2]` 上的框（2026-09-23） | `ui.md` §2 `[2]` |
| `/` 進 visual mode 搜尋 | `/` 是 finder（2026-09-23） | `ux.md` §1.1 |
| webu 自己的 popup、Space menu、淡化、寬度函式、splash（到 0.5.0） | terminu-go 的（2026-10-18，tdp E8） | 本檔「設計決定」 |
| 網址列下的 pagetab、`Esc` 上 pagetab（2026-09-23） | part 是 [2] 的 panel tab（2026-10-18） | 本檔「設計決定」 |
| DevTools popup（2026-09-21） | DevTools 畫面（2026-10-18） | 本檔「設計決定」 |

## 已知的牆與未做

網頁是二維的、終端機不是，所以一個密集的 app 頁（issue tracker 的看板、dashboard）分類是對的、但還是得在裡面移動 —— finder 與目錄是路，不是捲動。一個完全不宣告語意的站（全是 `div`、沒有 ARIA）給 webu 的只有文字和能點的東西；那種站 screen reader 也會壞，webu 不為單一站加 heuristic 去追。

還沒做的：

- 媒體（圖片、影片、音訊是佔位）
- `<textarea>` 交給使用者的 `$EDITOR`；小數 step 的 slider（目前只列整數）
- 游標在 frame 裡時只捲 frame，不捲外面的頁
- timer 類 live region 會讓 settling 的 spinner 轉滿 8 秒
- 只在 pointer-over 才打開的選單：hover 送了不等（見「運作方式」），這類選單 webu 目前開不出來
- 頁面名字自帶 Nerd Font glyph（APG 的 tree 用 U+F07B 當資料夾 icon）看起來像多一格空白 —— 先放著
- 滑鼠、Linux ARM

- lib 還沒有的：從程式切換畫面、移動 focus（WB6，暫時送合成的按鍵）；confirm、input 被 Esc 關掉時通知 app（WB3，頁面的問題暫時是 webu 自己的 popup）。

webu 照 tdp v0.7.4 與 terminu-go v0.1.13 重構（2026-10-18）；有意不照做的地方列在下一節，還沒符合的列在 `docs/webu-terminu-fix.md`（有的話）。

## 偏離 tdp

- **頁面自己的彈窗是 [2] 的內容，不是 webu 的 popup（tdp F1、K4、F3）。** 頁面跳出的 modal / alertdialog / menu / cookie 橫幅，webu 畫成浮在 [2] 內容上的框（`ui.md` §2.4、`pagebody.go` 的 `pagePopupFloats`），它是頁面的一部分：頁面放它上來要一個回答，webu 只是把頁面畫出來。所以它不是 lib 的 popup、不在 F1 的七類裡（F1 點名 webu 的頁面彈窗是 app 的偏離），上面的鍵照 panel [2] 走：
  - `Esc` 不關它（偏離 K4、F3）：關掉等於把頁面要的決定擱著；`Esc` 在它上面只 toast 說明（`pageEsc`）。
  - `Space` 是 [2] 的 Space menu，`?` 是 [2] 的 key reference：它的按鈕、連結是 [2] 的 item。
  - 它的寬度、疊層錯開（`popupWidth`、往右下錯開 3 欄 1 列）、底下的頁面淡化，都是頁面內容的畫法；webu 的 popup（lib 的）開在它上面時，它跟整個畫面一起被 lib 淡化。
- **頁面的 alert／confirm／prompt／beforeunload 與 HTTP 登入是 webu 自己的 popup（tdp F1、components/dialog/confirm）。** `pageask.go`，用 lib 的 `App.Popup`：框、標題、層色、開關動畫、淡化、`?`、`q` 是 lib 的，裡面的字與按鍵是 webu 的，長相照 confirm（要先看的內容在上、問句最後、`Enter` 接受、`Esc` 取消）。原因：頁面必須收到「取消」這個回答才會繼續，而 lib 的 confirm、TextInput 被 `Esc` 關掉時不通知 app（WB3，回報為 lib 要補）。lib 補上之後換回 lib 的 confirm 與 input。
- **goreleaser 排除 linux/arm64（tdp project 的範本檔）。** `.goreleaser.yaml` 的 builds 多一段 `ignore`（`TODO(tdp WB1)`）：Chromium 的 snapshot bucket 沒有 Linux ARM 版，webu 只跑釘死的 snapshot，這個平台的執行檔永遠抓不到引擎。範本沒有讓 app 排除平台的地方，回報為範本的缺漏。

## 設計文件導讀

| 檔案 | 回答什麼 | 順序 |
|---|---|---|
| [`function.md`](function.md) | Chromium 做什麼、webu 做什麼、做到哪；翻譯層（accessibility tree + 版面快照 → 一份文件）；role 表；frame；彈窗；Chromium 怎麼取得、怎麼跑 | 第 1 |
| [`ui.md`](ui.md) | 版面、header 的 screen 與兩個面板、頁面的三個畫面與四個 part、每種東西怎麼畫、popup、兩套配色、存檔 | 第 2 |
| [`ux.md`](ux.md) | core-key 語意、每種東西的 Enter、每個焦點的 `Space` 選單、finder、每種輸入的行為、熱鍵全表、時間軸；§A、§B 沿用 VTP 時期的分章，標題標出對應的 tdp 條目 | 第 3 |
| [`support.md`](support.md) | 支援的 accessibility role，由 role 表產生 | — |

設計本身 —— 包含試過而被否決的做法 —— 每條決定都在原地標了日期。

## 建置與開發

### 用什麼做的

Go、家族的 [terminu-go](https://github.com/vulcanshen/terminu-go)（建在 [Bubble Tea](https://github.com/charmbracelet/bubbletea) v1 上；頁面內容用 [Lip Gloss](https://github.com/charmbracelet/lipgloss) 上色）、Chrome DevTools Protocol 用 [chromedp](https://github.com/chromedp/chromedp)、語法色用 [chroma](https://github.com/alecthomas/chroma)。瀏覽器是 Chromium 專案自己的 snapshot 版，依 revision 釘死。

### 從原始碼建置

```bash
git clone https://github.com/vulcanshen/webu.git
cd webu
make build     # → ./webu（CGO_ENABLED=0、-trimpath、strip；版本是 dev）
./webu
```

頂層多出 tdp project 圖上沒有的兩樣：`tools/axdump`（看任何頁面的 accessibility tree，`make axdump`）與 `CLAUDE.md`（指向 2026-10-18 那次重構的交代，給 AI 協作時找回脈絡）。

同時改 terminu-go 時，在 repo 旁邊放 `../terminu-go`，用不進版控的 `go.work` 接上：`go work init . ../terminu-go`。`make check` 有 `go.work` 時會多跑 `check-lib`（關掉 `go.work`、照 `go.mod` 寫的 tag 編譯與測試），過不了就是用到了 lib 還沒發版的東西。

```
make build              → ./webu；第一次啟動會把釘死的 Chromium 下載到 cache
make install / uninstall → $GOBIN
make test / test-race   全部測試；需要瀏覽器的只在 Chromium 下載好之後跑，否則 skip
make check              commit 前：fmt-check、vet、test-race，有 go.work 時加 check-lib
make fixtures           用釘死的 Chromium 重抓 internal/ir 的 role fixture 與 docs/support.md
WEBU__SMOKE=1 go test ./internal/ui -run TestSmoke -v     真站 smoke（Hacker News、GitHub）
make axdump URL=https://…                                任何頁面的 accessibility tree，用本機 Chrome
make gif                從 .local/demos/demo.tape 重錄 docs/demo.gif（VHS、Nerd Font、網路）
make snapshot           goreleaser 本機打包到 dist/
```

`make` 列出全部：家族共用的 target 在 `family.mk`（tdp project 的範本，不在 webu 裡改），webu 自己的（`fixtures`、`axdump`）在 `Makefile` 最後一段。

- **fixture 與 golden**：`internal/ir/testdata/<group>.html` → `make fixtures`（只認釘死的 Chromium）→ `<group>.json` → `.golden`（`ir.Dump`）+ `.md`（`ir.Markdown`）；`internal/ui/testdata/<group>.render` 是同一批在 60 欄的排版。`-update` 重生 golden，改渲染時看 diff 決定。
- **`go test ./...`**：ir 三份 golden、ui 的排版 golden、store、paths、browser（UA）、page（sessions）。
- **瀏覽器測試**（需要釘死的 Chromium，沒有就 skip）：`hooks_test`（dialog、新視窗、憑證、下載、auth、上傳 picker、PDF、hover、iframe 同站）、`frame_test`（跨站 + 巢狀）、`popup_test`、`loading_test`、`lines_test`、`finder_test`、`editor_test`、`listbox_test`、`tree_test`、`tabs_test`、`slider_test`、`gauges_test`、`tooltip_test`、`section_test`、`parts_test`、`app_test`（back 回原位、combobox 沒 option）。
- **測試 driver**（`app_test.go`）：把訊息一個一個交給 lib 的 `App.Update`、在 goroutine 裡跑 command；`d.view()` 是去掉樣式的 `App.Frame()`，popup 開沒開看它的標題（例：`OPEN LINK`）；`d.press` 是按鍵後等開關動畫跑完（lib 在 popup 開完之前不收鍵）；`d.action(label)` 直接跑 focus 的 panel 的某個動作。`toast` 還在時第一個 `Esc` 先收 toast（tdp K4），測試要算進去。
- **重現 TUI bug** 用 Go driver；要手動跑 `./webu` 重現時，把 `WEBU__CONFIG`、`WEBU__DATA` 指到暫存目錄，不碰自己的設定、書籤與登入。
- **glyph 碼位**用 `fontTools` 讀已安裝 Nerd Font 的 cmap 查，不憑記憶。
- **demo gif**：`make gif`（family.mk，tape 在 `.local/demos/`）；vhs 0.12.0 曾在 macOS 上 2 秒就結束、不出檔也不報錯，0.12.1 正常。
- **實機截圖**：照 terminu-go 的 `scripts/tmux-shot.sh`，用自己的 tmux server（`tmux -L …`）與 `codepoint-widths` 試 icon 兩格；設定與資料指到暫存目錄（`WEBU__CONFIG`、`WEBU__DATA`）。

## 發布

照 tdp project 的「發布」：`make check` 通過 → CHANGELOG 的 `## Unreleased` 改成 `## vX.Y.Z — tdp vA.B.C` → commit、打 tag `vX.Y.Z` → push commit 與 tag。`release.yml`（範本）在 ubuntu 與 macOS 跑 `go test -race`、goreleaser 建置並推 Homebrew tap（`vulcanshen/homebrew-tap` 的 `webu.rb`），再用 CHANGELOG 那一段蓋過 release 的內文（awk 抓 `## <tag>` 那一段）。0.5.0 以前的 CHANGELOG 標題照當時的寫法（`## [0.5.0] — 2026-10-06`），不改。

- **token**：goreleaser 讀 `GITHUB_TOKEN`；workflow 在 secret `ACTION_TOKEN` 有設時用它（內建的 token 寫不了 tap）。
- **`make release-check`**：goreleaser 把 `brews` 標成 deprecated，只有這個警告不算失敗。
- 發布後確認資產時看 `repos/<owner>/<repo>/releases/<id>/assets`；`gh release view` 與 `releases/tags/<tag>` 的 assets 剛發布時可能還是空的。**不要對已經成功的 run 重跑 release job** —— goreleaser 會因為資產已存在而失敗；真的要重來，先刪掉資產再重跑。
- 發版前 `check-lib` 一定要過：`go.mod` 寫的 terminu-go 必須是打過的 tag。
