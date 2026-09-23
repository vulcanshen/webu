# webu — Implementation

> 設計權威是 `docs/function.md`（功能邊界）、`docs/ui.md`（版面）、`docs/ux.md`（互動）與
> VTP（`thoughts/tui-design`）。本文件只記**實作怎麼落地**：哪個套件負責什麼、實測出來而設計
> 文件沒寫的事、與設計文件的偏離、以及目前做到哪。0.3.0（2026-09-23）整份重寫。

---

## §0 套件

```
cmd/webu/            進入點：version / browser update / 首次下載 / 啟動 Chromium / 進 TUI
internal/browser/    Chromium 的取得與執行：釘死 revision、目錄、flag、UA、關閉
internal/ir/         翻譯層：role 單一宣告表（roles.go）、AX tree + snapshot → IR（build.go）、
                     Dump / Markdown、fixture、docs/support.md 產生器
internal/page/       CDP 端：Capture（capture.go）、動作（actions.go）、hook（hooks.go）、
                     觀察與 DevTools 資料（observe.go / devlog.go）、跨站 frame 的 session（sessions.go）
internal/ui/         TUI：app.go（key 路由、Space menu、dispatch）、tab.go（分頁模型：capture / apply /
                     settling / places / drill / 移動）、render.go（IR → rows / items / marks）、
                     parts.go（四個 part）、section.go + sectionlist.go（目錄與節）、finder.go（/ 與 go）、
                     pagepopup.go（頁面的彈窗）、pagepanel.go（[2] 的畫法）、editorpopup.go、slider.go、
                     fill.go、inputpopup.go / spacemenu.go / popup.go（webu 的浮層）、bookmarks.go /
                     listpanel.go / settings.go（screen）、devtools*.go、selectmode.go、theme.go
internal/store/      config / bookmarks / history / session 的 YAML；Netscape 書籤匯入
internal/paths/      三個目錄的解析
tools/axdump/        看任何頁面的 AX tree（本機 Chrome）
```

依賴方向：`ui → page → ir`、`ui → browser`；`ir` 不依賴 CDP 以外的任何東西。`page/sessions.go`
用 `ir.CarriedFrom` 當 id 偏移的單位，是 page 對 ir 唯一的反向依賴（一個常數）。

---

## §1 Chromium（function.md §9）

| 項目 | 落地 |
|---|---|
| revision | `browser.Revision = 1701465`（Chromium 156 線）；Mac_Arm 175 MB、Mac 195 MB、Linux_x64 248 MB；Linux arm64 快照桶沒有 |
| 下載 | 串流到 `.zip.part`、解到 `.tmp`、最後 rename；保留 symlink（Mac bundle 的 `Versions/Current`） |
| 目錄 | cache `WEBU_CACHE` > `XDG_CACHE_HOME/webu` > `os.UserCacheDir()/webu`；config `WEBU_CONFIG` > `XDG_CONFIG_HOME/webu` > `~/.config/webu`；data `WEBU_DATA` > `~/.webu/datas` |
| flag | Puppeteer 那組減 `--enable-automation`，加 `--disable-blink-features=AutomationControlled`、`--headless=new`；三個 background throttling 關閉 |
| UA（2026-09-23） | `Browser.getVersion` 拿自報字串，`HeadlessChrome/` → `Chrome/`，尾巴 ` webu/<ver>`；client hints brands `Chromium` + `webu`；每個分頁 `Prepare` 時 `Emulation.setUserAgentOverride`（`browser.identify` / `userAgent` / `uaMetadata`；`TestWebuSignsItsUserAgent`） |
| 關閉 | `chromedp.Cancel`（profile flush）再 allocator cancel；SIGHUP / SIGINT / SIGTERM |
| log | chromedp 的 log 全部進 `<data>/webu.log`，**絕不到 stderr**；`unhandled … event` 丟掉 |

---

## §2 翻譯層（function.md §3）

### 2.1 Capture：一次抓什麼

`page.Capture(ctx, frames)` → `ir.Capture`：

| 欄位 | 來源 | 用途 |
|---|---|---|
| `Nodes` | `Accessibility.getFullAXTree` | 語意 |
| `Display` | `DOMSnapshot.captureSnapshot(["display"])` | block / inline（`generic` 對 div 與 span 一視同仁） |
| `Boxes` | snapshot 的 layout bounds | 四個 part、彈窗的「疊在別的東西上」、sr-only |
| `Hidden` | box ≤ 1×1 | sr-only 丟掉（2026-09-22） |
| `Parents` / `Anchors` | snapshot 的 DOM 父子表、`id` | 頁內錨點；彈窗判定排除祖先 / 子孫 |
| `Protected` / `Types` / `Srcs` / `Current` / `Breadcrumb` / `Skip` | snapshot 的屬性（`attrMarks` / `attrValues`） | password；input popup 邊框；frame 的 src；aria-current；breadcrumb / skip 標記 |
| `FrameOf` / `Frames` / `Base` | snapshot 的 `ContentDocumentIndex` + `DOM.describeNode` + sessions（§2.4） | frame 是可進去的一層 |
| `Viewport` | `Page.getLayoutMetrics` | 頁面比視窗矮就不切 part |
| `ContentType` | `document.contentType` | 非 HTML 整份 code block；PDF 不支援 |

### 2.2 Build：實測出來的事

- **`url` 就在 AX node 上**（link 的 href、image 的 src）；`<iframe src>` 不在，從 snapshot 補。
- **password**：Chromium 把 value 遮成 `•`、AX 不說；空欄位只有 snapshot 的 `type=password` 認得出。
- **contenteditable**：`generic` + `focusable` + `editable="richtext"` → 多行 Textbox。
- **`<select>`**：`combobox`，option 在 `MenuListPopup` 子樹；**沒有 option 的 ARIA combobox 是 Textbox**
  （Google 的搜尋框是 `<textarea role=combobox>`，2026-09-23）。
- **spinbutton**：值在 `valuetext`。**slider / progressbar / meter**：`valuemin` / `valuemax` 進 `Min` / `Max`，
  值是 AX `value`；數字經過 float32（0.6 變 0.6000000238418579）→ `cleanNum` 洗成六位。
- **listbox 的 `multiselectable` 傳到每個 option**（畫 check 還是 radio）；**`aria-expanded` 存在就是 `Expandable`**
  （tree 的枝 vs 葉）；`focused` / `modal` / `invalid` 各一個 bool。
- **Chromium 的 role 拼法**：`Iframe`、`Canvas`、`Date`、`DateTime`、`InputTime`、`ColorWell`、
  `DisclosureTriangle`、`LayoutTable*`、`MenuListPopup`、`sectionheader` / `sectionfooter` 都照它拼。
  日期時間顏色框的 picker 零件（年 / 月 / 日 spinbutton、picker 按鈕）是 UA shadow 漏進來的，
  Textbox 本來就不收 children 所以自然消失。
- **role 表**：`ir.Roles`：role → `{Kind, Action, Transparent, Display}`，`docs/support.md` 由 `SupportDoc`
  產生、`TestSupportDoc` 守著不漂；未列的 → `Unsupported`。
- **非 HTML**：取樹裡最長的文字節點當內容、JSON `json.Indent`、整份 `Document{Code}`。
- **breadcrumb 沒包 nav 的**、**skip 區塊**：build 時包成 `Landmark{navigation}`。

### 2.3 fixture 與 golden

`internal/ir/testdata/<group>.html` → `make fixtures`（只認釘死的 Chromium）→ `<group>.json`
→ `.golden`（`ir.Dump`）+ `.md`（`ir.Markdown`）；`internal/ui/testdata/<group>.render` 是同一批在
60 欄的排版。`-update` 重生 golden，改渲染時看 diff 決定。

### 2.4 frame：三條路一個出口（2026-09-23）

- **同 process**：snapshot 的 `docs` 已含所有同 process 文件；`frameOwners(docs, strs, cur)` 只看**當前
  document** 的 `ContentDocumentIndex`（讀整張表會在 frame 裡找到自己的 owner、無限遞迴到 15 秒 timeout）；
  `getFullAXTree.WithFrameID` 取樹。
- **跨站**（量測）：主 frame 的 `Page.getFrameTree` **不列** OOPIF、snapshot 沒它的文件；
  `DOM.describeNode(<iframe>)` 給 `FrameID`（= target id，`Target.getTargets` 列為 `iframe` 型、已 auto-attach）；
  `chromedp.NewContext(tabCtx, WithTargetID(frameID))` 可以 attach（chromedp 對 page target 開了
  `Target.setAutoAttach(flatten)`，但它不替 iframe session 建 executor、訊息全丟；自己 attach 一條才有）；
  在那條 session 上 `getFullAXTree` / `captureSnapshot` / `getBoxModel` / `Input.dispatchMouseEvent`
  都正常，座標是 frame 自己的、Chromium 會轉。`page.Sessions`（`WithSessions` 掛在 tab ctx、`NewSessions`
  的 `tab` 也帶著它所以巢狀派生得到）：`frame(id)` 懶 attach（5 秒沒回就放棄）、`forget`、`resolve`。
- **id 撞號**：frame 的 backendNodeId 從 1 重編。每條 session 一個 slot，`Base = slot × ir.CarriedFrom`
  （2^40）；`ir.Build` splice 時 `offsetIDs`（已 carried 的跳過）；每個帶 node id 的動作走 `on(ctx, id, fn)`
  解回該 session 與原 id（Reveal / Click / Type / Fill / Slide / Choose / Submit / SetFiles）。
- **box**：`attachFrames.keep` 把 frame 的 Boxes 平移到 owner 的 box、以 carried id 併進主頁（巢狀先在上一層
  平移），part 與彈窗判定看得到 frame 內容。
- **ui**：`openFrame(n)` 把 frame id 加進 `tab.frames`、`wantDrill`、重抓；`apply` 後 drill 進去；沒抓到就
  toast。同站 / 跨站 / 巢狀在 ui 一行都不用分。

---

## §3 排版與頁面模型（ui/render.go、tab.go、parts.go、section.go）

### 3.1 render：IR → layout

`layout{rows, items, marks}`：block kind 斷行、inline kind 流式折行；每個 `seg` 記 item、item 記 first / last
列與起始欄（`j/k` 換列、`h/l` 同列靠它）；`marks` 記每個 block 的第一列（目錄、finder、起點都靠它；
`markNext` 讓欠著的空列不算成它的）。決定：

- 相鄰 item 之間補空格、標點不補；`<label>` 的文字 = 欄位的 name 就吃掉（`dropLabel`）；沒名字的 link 顯示 URL 最後一段。
- **同一個名字只印一次**：`namedByItsHeading` 的 region 不畫 rule；`dropLabelRow` 只認 landmark / heading 的 mark。
- **thing**（`thing()` / `firstLine`）：多於一行的 ListItem / article 只畫第一行、是 item；drill 時
  `renderOpts.drill` 指定根、第一行成 header 列（`drillHead` / `drillBody`）。
- **form**：`formLabelW`（最寬 label，不截）/ `formValueW`（glyph + 最寬值 + 空格 + caret）決定框寬，擠不下就
  `fitForm` stack；`formField` 畫一列；`fieldset` 畫 legend；`valueKind` 把 invalid 畫紅。
- **table**：每格一個 item（`cellItem`），欄寬從最寬的欄縮；`row.table` / `row.header` 讓 pagepanel 鋪底。
- **tabChain**（tablist）：`capLeft` + 段 + `dividerSoft` + `capRight`，`segTabOn/Off/Cap`；row 記 `heading =
  len(r.heads)`（所在節的深度），`rowLine` 用 `levelColor`。
- **treeItem**：`Level` 縮排、`Expandable` 決定 `▾`/`▸`/空。**optionText**：`Multi` 決定 check / radio。
- **slider / gauge**（`slider.go`）：12 格軌道 `●` / `━━━───`；`sliderItems` 造數字清單（>10000 格以十為步）。
- **DisclosureTriangle**：Button 分支換 `▸ `/`▾ ` 當 lead。**tooltip**：Group 分支 `glyphInfo` + segDim。
- **裸連結串**：`linkRun` 標出連續兩個以上的 `bareBlockLink` 走 inline。
- **inline 標記**：`textAttr` 位元欄疊在 kind 樣式上（`withAttr`）。
- **measure**：只管 `wrap()` 的 `textW`；`store.Measure` 收 `full` / 數字。
- **行號**（`lineNumW` / `lineNum`）：gutter 從文字寬扣、面板寬不變；`relayout` 先用上一頁的位數排、位數變了
  再排一次；drill 進一件事時從 body 起算、彈窗裡 gutter 0。

### 3.2 parts（parts.go，2026-09-23）

`splitParts(root, boxes, viewport)`：只看頂層區塊的幾何——最大面積是 body；`beside`（垂直重疊）是 others；
前面 header、後面 footer。守門：頁面高 ≤ viewport、或最大區塊 < 25% 面積 → nil（整頁一塊）。
`tab.parts` / `at` / `activePart`；`relayout` 從 `sansPopup(root)` 切、`base` 是 active part 的節點；
pagetab 由 `partChain(labels, at, hand, focused, dimmed)` 畫（lit 整條 / dim 兩種）；`stepPagetab` 走到哪
就 `relayout` 到哪；`enterPagetab` 在彈窗時 false。

### 3.3 sections（section.go / sectionlist.go）

`sectionsOf(root, lay, url)`：從 `marks` 收 heading（scope 是 main）；`(top)` 是第一個標題前的內容；
**一節的 `last` 延伸到下一個同級或更高級標題**（2026-09-23，原本是下一個任何標題——MDN「Try it」
因此是空的）；`start()` 把 landmark 的 rule 併進下一節；`count` 數表 / code / 媒體 / 散文 / 連結；
`pruneNav` 丟掉只有連結、沒子節的「節」（併進前一節，用 max 不縮短父節）；深度用 stack 算（h1 → h3 → h4
是三層）。`shapeOf`：≥ 3 個標題是 `shapeDoc`。`tab.listing()` / `read` / `sec` / `secTop` / `flat`；
`openSection` / `closeSection` / `stepSection`（`siblingSection` 同 level）/ `sectionAt`（最內層）/
`rowRange`（讀一節時 `body..last`）；`recut` 重抓後用 heading 的 node id 留在原節；`sectionHint` /
`readPct` 給下框；`headingTitle` 去掉標題尾巴的同頁錨點連結。

### 3.4 drill、places、finder、go

- **drill**：`drillInto(n)` / `leaveDrill`；`chainTo` / `chainOf` 算祖先路徑；`enterItem` 對一行的 ListItem
  直接 `firstItemIn` → `enterOn`；`Media` 有 `Frame` 就 `openFrame`；Code → `showCode`。
- **places**（tab.go）：`entry`（`Page.getNavigationHistory` 的 current entry id）、`places[entry] =
  place{at, flat, read, sec, top, cursor}`；`apply` 的 `fresh` = entry 不同（URL 判斷會被 settle 重設）。
- **finder**（finder.go）：`indexPage` 走四個 part、`partOf` 記 part、Combobox 的子節點跳過、Option 可命中；
  `hit{node, part, trail, under, text, line}`；`hitScore` 字面優先、模糊 ≤ 64；`goToHit` 用 `chainOf` 沿路 drill、
  `rowOf` 找列；`geometry` 96 欄以上並排。`go`：`finderGo` 列 `lineText(n)`、`goToLine`。
- **Esc 鏈**（app.go `escKey`）：pagetab → drill → 彈窗（拒絕）→ read → pagetab。

### 3.5 頁面的彈窗（pagepopup.go，2026-09-23）

`noticePopup()` 在 `apply` 時跑：`findPopup(prev, skip, root, boxes)` 走整棵樹找 `prev` 沒有的子樹根；
`isPopup(c, chain, boxes, parents)`：`countItems > 0`、且（`Modal` / dialog / alertdialog / menu / `hasFocus`）
或 `overlap`「疊在無關的 box 上一半以上」（`parents` 排除 DOM 祖先 / 子孫——`main` 在 Accept 之後曾被
誤判）。`popups` 是 stack：同一次 capture 有新的就 push、不丟舊的（Chromium 疊 modal 時連第一層也砍）；
`popupLays` 存下層的 layout 畫在後面（`pagePopupFloats` 錯開 3 欄 1 列）；`back` / `backParts` /
`backRead` / `backSec` / `backTop` 存彈窗出現前的版面與位置，答完復原；`popupUntil` = press 後 8 秒；
載入時 prev 空 → 全部算新（載入就在的彈窗算）。`popupWidth` / `popupVisible`；`pageBody` 用 `backdrop()`。

### 3.6 兩套配色

`theme.go`：App palette（`focusColor` / `handColor` / `headerColor` / `pagetabColor`…）與 Page palette
（`pageText` / `pagePress` / `pageFill` / `pageCode` / `pageMedia` / `pageInvalid` / `levelInk`…）分開；
glyph 碼位一律用 `fontTools` 讀 `~/Library/Fonts/HackNerdFontMono-Regular.ttf` 的 cmap 查，不憑記憶
（`glyphHeader/Body/Others/Footer` = `page_layout_*`、`glyphPopup`、`glyphUpload`、`glyphInfo`…）。

---

## §4 動作（page/actions.go、sessions.go）

每個 `page.*` 入口都經 `chromedp.Run`（`run` / `on`）；`on(ctx, id, fn)` 先用 ctx 裡的 Sessions 解 id。

| 動作 | CDP |
|---|---|
| Click | `reveal` → `getBoxModel` → `hover`（`mouseMoved`，**100 ms timeout、送了不等**：headless 的 ack 要 5 秒、頁面當下就處理了；量過）→ pressed / released 在中心；沒 box 才 `el.click()` |
| Type | focus → select all → `Input.insertText`；空字串是 Backspace |
| Fill（2026-09-23） | `el.value = …` + `input` / `change`（date / time / color 等 insertText 打不進去的框） |
| Slide（2026-09-23） | `<input type=range>`：Fill；ARIA：focus 後 ArrowLeft / Right 走到 `aria-valuenow` 到位或不再動（上限 4096 步） |
| Choose | option `selected = true` + select 的 `input` / `change` |
| Submit | focus + Enter keydown / keyup |
| Key | Enter / Escape / ArrowLeft / ArrowRight |
| Entry | `Page.getNavigationHistory` 的 current entry |

**tab 的三種跑法**：`press`（使用者的動作：設 loading / `settlingUntil` / `popupUntil`）、`act`（webu 自己的：
Reveal、開 frame）、`unblock`（對話框 / auth 的回答：不排隊）。**action lock**（`tab.acting`）：讀 box 與按下
之間曾被舊的 Reveal 捲走（六次漏一次），鎖住讓它們是一個動作；對話框開著時碰 renderer 會卡，所以回答
不經鎖。動作後 300 ms `settle` 再 capture；`gen` 丟掉過期的。

---

## §5 非 DOM 事件（page/hooks.go）

| 事件 | 落地 |
|---|---|
| JS dialog | `dialogMsg` → confirm / input popup；`handleJavaScriptDialog` 回答；開著時 settle 暫停 |
| `target=_blank` | `Target.targetCreated`（`OpenerID` 有值）→ `NewContext(WithTargetID)` → `Prepare` → 加到尾端切換 |
| 憑證 | `ERR_CERT_*` → confirm → `setIgnoreCertificateErrors(true)` 重載；每分頁問一次 |
| 下載 | `setDownloadBehavior` + 事件 → toast / Downloads screen / header 進度條 |
| HTTP auth | `Fetch.enable(handleAuthRequests)`（每個 request pause 一次、goroutine 裡 continue）→ 兩個框 → `continueWithAuth` |
| 檔案上傳 | `fileChooserOpened` → picker（`filepicker.go`，`importDir()` 起）→ `SetFiles` |

---

## §6 量出來的事實（不再重查）

| 事實 | 影響 |
|---|---|
| headless 下單獨的 `mouseMoved` 要等 renderer ack 5 秒，但頁面當下就處理 | hover 送了不等（100 ms ctx） |
| `Security.certificateError` 已從協定移除 | 用 `setIgnoreCertificateErrors` |
| `Page.getFrameTree` 不列 OOPIF；`DOM.describeNode` 給 frame id；瀏覽器允許 attach iframe target；frame id 從 1 重編 | §2.4 |
| 原生 `showModal` 砍掉 dialog 以外整棵樹、疊 modal 時連第一層也砍；aria-modal 不砍 | backdrop 用前一刻的版面；push 不 drop |
| 數字經 float32 | `cleanNum` |
| Google 對 headless 一律 reCAPTCHA（即使 UA 簽自己的名）；十二個搜尋引擎實測 DDG html 最乾淨 | 預設搜尋 |
| 內文裡的連結串最長 5–8（HN 全頁 2）、導覽 12–47 | 裸連結串只流動、不收合 |
| w3schools 沒有任何 landmark；沒 main 的頁把側欄連結欄當「節」 | part 靠幾何；`pruneNav` |
| 表單第一欄的 caret 掉到下一列 | `formValueW` 沒算 caret 前的空格（2026-09-23 修） |
| `tea.Sequence` 巢狀不等內層 | 先後執行寫成一個 cmd |
| Bubble Tea value receiver：改 popup 狀態的 helper 若是 value receiver 只回 `tea.Cmd`，改到的是副本 | pointer receiver，或先 `close()` 再 `dispatch()` |
| 同站 frame 在自己那層讀整張 owner 表會找到自己 | `frameOwners` 只看當前 document |
| 有 timer 的頁面每秒指紋都不同 | spinner 轉滿 8 秒 grace（已知） |

---

## §7 測試

- `go test ./...`：ir 三份 golden、ui 的排版 golden、store、paths、browser（UA）、page（sessions）。
- 瀏覽器測試（需要釘死的 Chromium，沒有就 skip）：`hooks_test`（dialog、新視窗、憑證、下載、auth、上傳 picker、
  PDF、hover、iframe 同站）、`frame_test`（跨站 + 巢狀）、`popup_test`、`loading_test`、`lines_test`、`finder_test`、
  `editor_test`、`listbox_test`、`tree_test`、`tabs_test`、`slider_test`、`gauges_test`、`tooltip_test`、`section_test`、
  `parts_test`、`app_test`（back 回原位、combobox 沒 option）。
- `WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v`：Hacker News、GitHub。
- 測試 driver（`app_test.go`）：`newDriver` / `startAt` / `until`（20 秒）/ `cursorOn` / `key`；`until` 的條件要寫
  closure（`d.m` 是值，綁方法會綁到舊副本）。
- 重現 TUI bug 用 Go driver；真要跑 `./webu` 就在 tmux 裡用 `-e WEBU_CONFIG= -e WEBU_DATA=` 指到暫存目錄、
  只用 pane id 操作；不用 vhs 做診斷。

---

## §8 狀態（0.3.0，2026-09-23）

### 已落地

function.md §3 的 role 表全部、§4 每種輸入、§4.3 彈窗、§3.5 三種 frame、§6 settling、`ui.md` §2 的
三個畫面與四個 part、finder / go、行號、places、action lock、UA、DuckDuckGo、PDF 不支援、editor popup、
檔案 picker、tooltip / timer / gauge、tabs / tree / listbox / menu。

### 未做（已知，先放著）

- 游標停在 frame 裡的東西時只捲 frame 內部，外頁不跟著捲到 frame。
- 有 timer / 時鐘的頁面每次動作後 spinner 轉滿 8 秒（live region 進了指紋）。
- 非整數 step 的 slider 清單只列整數。
- 頁面名字自帶 Nerd Font glyph（APG 的 tree 用 U+F07B 當資料夾 icon）看起來像多一格空白——user 裁定先放著。
- textarea 的 `$EDITOR` 鏈、mouse、Linux ARM、多媒體、CAPTCHA（`function.md` §9.1）。
