# webu — Implementation

> 設計權威是 `docs/function.md`（功能邊界）、`docs/ui.md`（版面）、`docs/ux.md`（互動）
> 與 VTP（`thoughts/tui-design`）。本文件只記**實作怎麼落地**：哪個套件負責什麼、實測
> 出來而設計文件沒寫的決定、與設計文件的偏離處、以及目前做到哪。章節編號對齊
> kbu / filu / sshu 的 implementation doc。

---

## §0 套件

```
cmd/webu/            進入點：version / browser update / 首次下載 / 啟動 Chromium / 進 TUI
internal/browser/    Chromium 的取得與執行：釘死 revision、cache / config / profile 目錄、flag、關閉
internal/ir/         翻譯層：role 單一宣告表、AX tree → IR、fixture、docs/support.md 產生器
internal/page/       CDP 端：Capture（AX tree + display map）、Click / Type / Choose / Submit / Navigate
internal/ui/         TUI：三面板、排版引擎 render.go、popup、tab 模型、key 路由
tools/axdump/        看任何頁面的 AX tree（用本機 Chrome，不是釘死那份）
```

依賴方向：`ui → page → ir`、`ui → browser`；`ir` 不依賴 CDP 以外的任何東西，
`ir` 的 fixture 測試是外部測試套件（`package ir_test`）以免與 `page` 成環。

## §1 Chromium（function.md §9）

| 項目 | 落地 |
|---|---|
| revision | `browser.Revision = 1701465`（V8 15.6、Chromium 156.0.8067.0 線）；三平台 HEAD 驗過都存在：Mac_Arm 175 MB、Mac 195 MB、Linux_x64 248 MB。Linux arm64 快照桶沒有，`ErrUnsupportedPlatform` 明講 |
| 下載 | `browser.Download`：串流到 `<dir>.zip.part`、解到 `<dir>.tmp`、最後 rename；中途死掉不會留下 `Installed()` 誤信的東西。解壓保留 symlink（Mac bundle 的 `Versions/Current` 是 link，複製成檔案就起不來） |
| 目錄 | cache：`WEBU_CACHE` > `XDG_CACHE_HOME/webu` > `os.UserCacheDir()/webu`；config：`WEBU_CONFIG` > `XDG_CONFIG_HOME/webu` > `~/.config/webu`（2026-09-21 起不走 `os.UserConfigDir`，macOS 也是 `~/.config`）；data：`WEBU_DATA` > `~/.webu/datas`（同 kbu / filu / sshu：macOS 沒設 XDG 時落在 `~/Library/…`，ui.md §6 寫的 `~/.config/webu` 是 XDG 拼法） |
| flag | Puppeteer 那組減 `--enable-automation`（否則 `navigator.webdriver` 為 true），加 `--disable-blink-features=AutomationControlled`、`--headless=new`、`--user-data-dir=<config>/profile`；三個 background throttling 關閉（function.md §6 的節流坑）。`TestLaunchAnswers` 斷言 `navigator.webdriver === false` |
| crashpad | macOS 上 Chromium 一律在 `~/Library/Application Support/Chromium/Crashpad` 放一個空資料庫，`--disable-breakpad` / `--disable-crash-reporter` / `--crash-dumps-dir` 都壓不掉；幾 KB，接受。`--crash-dumps-dir` 留著，Linux 吃 |
| 關閉 | `chromedp.Cancel`（讓 profile flush）再 allocator cancel（保底 kill）；`main.go` 接 SIGHUP / SIGINT / SIGTERM |
| chromedp 的 log | **絕不能到 stderr**（TUI 在畫終端機）：`Launch` 用 `WithBrowserOption(WithBrowserLogf/Errorf)` + `WithLogf/Errorf` 導到 `<config>/webu.log`；`unhandled node/page event`（Chrome 比 chromedp 新、多送的事件）直接丟掉，不記。實機第一次開 w3schools 就被這種 log 刷滿畫面 |

## §2 翻譯層（function.md §3）

### 2.1 role 表是單一宣告

`ir.Roles`：role → `{Kind, Action, Transparent, Skip, Display}`。`docs/support.md` 由
`ir.SupportDoc()` 產生，`TestSupportDoc` 保證不漂。未列的 role 走 `ir.Fallback`
（Unsupported：名字當文字、子節點保留、glyph 標記、focusable 就是 item、Enter 仍 click）。

### 2.2 實測出來的事（設計文件沒寫）

- **AX tree 沒有 block / inline**：`generic` 對 div 與 span 一視同仁，兩個相鄰 `<div>` 的字會黏成一行。
  `page.Capture` 多打一次 `DOMSnapshot.captureSnapshot(["display"])`，以 backendDOMNodeId 對回去；
  `ir.Capture{Nodes, Display, Protected}` 是翻譯層的輸入（`Protected`：同一份 snapshot 的 node 表裡 `type=password` 的 input，`page.passwordFields`；AX tree 不說、空的 password 欄位只有這裡認得出，2026-09-21）。透明容器若 display 是 block 級且裡面有 inline 內容，
  留一個 `Group` 保住斷行；inline kind（link / button …）若被排成 block，`Node.Block = true`
- **`url` 就在 AX node 上**：link 的 href、image 的 src 都是 AX property，不用 `DOM.getAttributes`
- **password**：Chromium 自己把 value 遮成 `•`；`Protected` 由此判定，輸入 popup 遮罩
- **contenteditable**：`generic` + `focusable` + `editable="richtext"`，視為多行 textbox（Slack / Gmail 的輸入面）
- **`<select>`**：`combobox`，option 在 `MenuListPopup` 子樹；IR 把 option 收成 Combobox 的 Children
- **`spinbutton`**（`type=number`）：值在 `valuetext`；當 textbox 編輯
- **HN 的表**：Chromium 給的是 `table` / `row` / `cell`（不是 LayoutTable），renderer 以「有沒有 header cell」判斷：
  沒有就是排版表，一列一行流式；有就對齊欄位
- **caption / rowgroup / Legend / Figcaption** 透明或跳過；`Canvas` / `Iframe` / `Video` / `Audio` 大寫開頭
- **依 response media type 調整**（修訂 2026-09-20）：`page.Capture` 多取 `document.contentType`；JSON（含 `+json`）、
  text/plain、CSV、XML、JS、CSS 這類文件，`ir.Build` 取樹裡最長的文字節點當內容（Chrome 的 JSON viewer 會在
  `<pre>` 旁加 Pretty-print 表單，不要它），JSON 自己 `json.Indent`，整份變成 `Document{Code}` 一個 code block。
  之前 jsonplaceholder 那頁只有一個 checkbox item、游標跳到頁尾，畫面看起來一片空
- **新頁載入**：`tab.apply` 以 `msg.url != lastVisit` 判斷是新頁 → 游標回 main 裡第一個 item（`firstItem`）、視窗回頂端；
  同頁重畫才留位與跟隨游標

### 2.3 fixture

`internal/ir/testdata/<group>.html` → `make fixtures`（`-capture -update`，只認釘死的 Chromium）
→ `<group>.json`（AX tree + display map）→ `<group>.golden`（`ir.Dump`）。`go test ./internal/ir`
不需要瀏覽器。`internal/ui/testdata/<group>.render` 是同一批 fixture 在 60 欄的純文字排版 golden。

## §3 排版引擎（ui/render.go）

IR → `layout{rows, items}`：block kind 斷行、inline kind 流式折行；每個 `seg` 記自己屬於哪個
item，item 記自己跨哪幾列，游標才能把整個 run 反白。決定：

- 相鄰兩個 item、或 item 後接字詞，補一個空格（AX tree 丟掉了元素間的空白）；標點不補
- `<label>` 包住的欄位：label 文字 = 欄位的 accessible name，renderer 把欄位前那段等於 name 的純文字吃掉
- 沒名字的 link 顯示 URL 最後一段（`vote`、`news.ycombinator.com`），整條 URL 在 Inspect
- 表格 cell 內的 block 攤平成一行（`inCell`）；資料表欄寬從最寬的欄開始縮（同 sshu）；**每格一個 item**（2026-09-21：`table()` 對每個 cell `newItem`、`cellSegs(td, id)` 期間 `r.cellItem = id` 讓格裡的 Link / Button / Textbox / Check / Combobox / Media 走 `itemOf` 不另開 item；`row.table` / `row.header` 讓 pagepanel 畫 `tableBg` / `tableHeaderBg`，`segTableHeader` 改 Text 粗體）；`enterCell`：沒東西 → `showCell`（message popup，`columnHeader` 當標題、`wrapWords` 折行、`messagePopup.scroll` j/k u/d G g）、只有一個目標且文字相同 → `enterOn(target)`、混合 → `openItemMenu`（`itemMenuItems` 的 Cell 列：Content + `targetItems`）；`enterOn` 是 enterItem 對互動節點那半段抽出來的
- 空格延後放：字與空格一起塞不下就換行，不會出現 `right(`
- **Heading 收合**（2026-09-21）：`renderer.suppress` 在收合的 heading 之後跳過節點，到 `foldLevel` 以上的 heading 或離開 `foldLm` 那層 landmark（`leaveLandmark`）為止；含終止 heading 的 wrapper 只走進去不畫自己（`hasHeadingUpTo`）；`walkSection` / `sectionItems` / `sectionHolds` 用同一套邊界算 `· N items` 與 Outline 的 reveal
- **Landmark 摺疊**（ui.md §2 修訂）：`renderOpts.fold` 是 user 的決定（by backendDOMNodeId，重抓後仍在）、
  沒決定時**全開**（修訂 2026-09-21，原本有 main 時 main 外預設摺疊）；`enterItem` 對 landmark 直接 dispatch `fold`，不開選單；每個 landmark 的標題列是 item
  （`item.folded`）；`tab.toggleFold` 重排並把游標留在原節點；`tab.reveal` 供 Outline 打開路徑上的 landmark
- **入口型 landmark**（2026-09-21 晚：banner / search / complementary / contentinfo 跟 navigation 同一套，user 定案「一套 style、icon 分種類」）：`isEntry`（五個 role）/ `entryRow` / `entryIcon` / `entryLabel`（navigation → `entryCurrent`；banner = 第一個不含 `#` 的連結文字；search = 第一個 textbox 的 Name；其他 = landmark 名字 → 第一個 heading）、`entryTargets`（Link / Button / Textbox / Check / Combobox，List 深度當縮排）、menu key `entry:i` → `actOn`（Textbox → `editField`、Combobox → `chooseOptionsFor`、其餘 click）；search 只有一個 textbox 時 Enter 直接 `editField`（沒有送出鈕的搜尋框目前送不出去——待補）；roles.go 五個 role 的 Display 改「page chrome」、`docs/support.md` 重生；**dialog / alertdialog** 也算入口（roles.go 加兩個 role → Landmark；icon `glyphDialog` dock_window；字：名字 → heading → 第一段文字截 40）；**search box 送出**（`editFieldAs(n, search)`、`inputPopup.search`：role searchbox 或 search landmark 裡的框；寫回後開 `confirmSubmitField` → `page.Submit`，Esc 留值不送）；**`spaceMenu` 捲動**（`top` / `visible()` = screenH-6、`scroll()` 在 `step` 後、view 只畫視窗；`step` 本來就環繞，`TestMenuScrollsAndWraps`）；**skip link**（`isSkipLink`：`#` 錨點 + 文字 skip 開頭、或 DOM class 含 skip → `Capture.Skip` → `Node.Skip`；inline 的 Link 遇到就 flush 後 `skipRow`（`entryLine` 與 entryRow 共用）；`enterOn` → `skipToContent`（`firstItem`，它本身永遠被跳過）；`entryTargets` 濾掉它；選單 Skip / Yank link url）；**skip 區塊與頁內錨點**（2026-09-21 晚）：Skip 標記改成 class / id / aria-label 含 skip 的任何元素，`build.convert` 把非 link 的標記容器（且 `anchorLinks`：有連結且全是 `#`）包成 `Landmark{navigation, Skip: true, Name: skipName}`；`page.anchors` 從 snapshot 讀 id → backend 與 parent 表（`Capture.Anchors` / `Parents` → `tab.anchors` / `parents`）；`tab.jumpToAnchor(frag)` 沿 parents 往上找到目標的第一個 item；`enterOn` / `actOn` 對 `sameFragment` 的連結先跳、跳不到才 confirm / click；`isSkipItem` 讓 `firstItem` 也跳過區塊
- **navigation 一行入口**（2026-09-21，取代一行流式：`inNav` / `inNavList` / `navList` 拿掉）：`renderer.navRow` 一列 `segNavBar` + `segNav`（focusColor 直槓、codeBg 底）、`+N` = `len(navTargets(n))`（子樹裡的 Link / Button，記 List 巢狀深度當縮排）；`enterItem` 對 navigation → `openItemMenu`（`optItemMenu` 回來了、只給它用）→ `itemMenuItems` 每個目標一列 key `nav:i` → `dispatch` 的 `nav:` 前綴 click 第 i 個（busy 時吞掉）；Space 的 item operation 是同一份列；**目前所在**（2026-09-21）：`page.attrMarks` 從同一份 DOM snapshot 讀 `aria-current`（→ `ir.Capture.Current` → `Node.Current`）與 breadcrumb 標記（class / itemtype / aria-label 含 breadcrumb → `Capture.Breadcrumb`）；`build.convert` 把沒包在 nav 裡的標記節點包成 `Landmark{navigation, Name: breadcrumb, Breadcrumb: true}`（`navDepth` 追蹤、標記用掉以免遞迴），nav 名字含 breadcrumb 也算；`render.entryCurrent`：Current 節點的 Text → breadcrumb 最後一節（最後一個 ListItem / Link）→ `urlMatch`（等於頁面 URL、或最長路徑前綴且超過 origin）→ landmark 名字；icon `glyphCrumb` = nf-md-map_marker_path
- **pagetab**（2026-09-22，入口行搬離頁面；名字是 user 2026-09-22 給的，先前兩個臨時名字是 bar 與 tray）：`layout.pagetab []capsule`（`kind` + `nodes` + label）：`renderer.chrome` 由 `block` 的 Landmark 分支與 `inline` 的 skip link 分支收集、`renderer.other` 由 `page()` 收有 main 時 main 之外不在 landmark 裡的節點（`holdsLandmark` 的 wrapper 走進去），`buildPagetab` 依 `pagetabKindOf` 分桶成固定順序 skip / header / nav / search / sidebar / footer / dialog×N / other；label = `capsuleText`：`capsule.title()`（`pagetabKind.word()`，dialog 用 `entryLabel` 的名字）+ `+len(capsuleTargets)`、**每節不帶 glyph**，漢堡只由 `withLead` 加在鏈的第一節（`fitPagetab` 量寬時也要算進去）；hint = `capsuleHint`（role 名字 / N navigations · at 目前所在（`entryCurrent`）· N inside）；選單 = `capsuleMenuItems`（多個 landmark 時每個一個 header 列、`targetRow` 的 key 用跨 landmark 的流水號、aria-current 的 hint 標 here）；`enterCapsule`：沒目標 → `showCapsule` 全文、skip 一個目標 → `actOn`（skip link 跳不到就 `skipToContent`）、search 一個框 → `editFieldAs`、其餘 `openItemMenu`；`dispatch` 的 `entry:i` 走 `tab.currentTargets()`；main 不畫 rule 也不是 item（`firstItem` 改用 `marks[main]`）、`sectionheader` / `sectionfooter` → Group、`isSkipLink` 加 jump to；`Esc`（沒 float / mode、focus 在頁面）與 page Space menu 的 `pagetab` 列共用 `togglePagetab()`（`pagetabItem` 依狀態給 `[Esc] Page chrome` / `[Esc] Back to the page`——core key 沒有字母可以讓 `bracketHotkey` 就地加括號，所以鍵寫進 label 自己，沒有 chrome 時 disabled 並說明）；不再 emit 列、不佔 item、不進 `marks`；`layout.fit = fitPagetab(pagetab, width)`（全部塞得下就全顯示，否則留位給 `+N`）；`tab.pagetab` 編碼 0 = 在頁面、i+1 = 第 i 顆、`pagetabMore` = `+N`（`onPagetab` / `onMore` / `pagetabIndex` / `pagetabSlot` / `pagetabSlots` / `focusPagetab` / `focusMore` / `leavePagetab` / `enterPagetab` / `stepPagetab` / `capsuleOf`）；`tab.current()` 先看 pagetab 的節、`+N` 上是 nil；`curFolded` 取代直接讀 `items[cursor].folded`；`moveItem`：頁面最上列再 `k` → `enterPagetab`、`h`/`l` → `stepPagetab`（繞）、`j` → `leavePagetab`、其他鍵先離開再照舊；`pagepanel.pagetabRow` 畫在 URL 下那列：整條是**一條 powerline 鏈** `pagetabChain`（借 chrome.go 的 `divider`、`capLeft` / `capRight`；`chainW` 量寬，`fitPagetab` 依鏈寬決定塞幾節，塞不下再從尾端砍），未點亮的節 Mauve（`headerColor`）字 + 畫布底（不給整條自己的底色）、hand 停的那節用同一個 Mauve 反色點亮（底 Mauve、字 baseHex、粗體）、面板沒 focus 時 borderDim、載入中整條 dim；hand 在 pagetab 上時頁面游標降成 curOff；`+N` Enter → `openPagetabMore`（`optPagetabMore`、key `pagetab:i`）→ `focusPagetab(i)`（該顆暫佔最後一格，`pagetabSlots`）+ `openItemMenuAt`（沿用同一層）；`apply` 重抓後依 backendDOMNodeId 把 hand 放回同一顆、`relayout` 後越界就 `leavePagetab`；Outline 選到入口 landmark → `focusPagetab`；`jumpToAnchor` / `jumpTo` / select mode 進出 / `cursorOn` 都 `leavePagetab`；`isSkipItem` / `entryRow` / `entryLine` / `skipRow` / `segNavBar` / `segNav` 拿掉，`firstItem` 不再需要跳過 skip
- **裸連結串流動**（2026-09-22）：`children()` 先跑 `linkRun(n.Children)` 標出連續兩個以上的 `bareBlockLink`（Link + Block + 沒有區塊子節點；中間的空白文字不算打斷），標到的走 `inline()` 不走 `block()`。量過七個站才定：內文裡的連結串最長 5–8（HN 全頁最長 2）、導覽 12–47，結構上分不開，所以不收合、只流動
- **抓取中的 spinner**（2026-09-22）：`theme.spinnerFrames`（md `circle_slice_1..8`，與 `glyphWeb` 同一個 Nerd Font 區塊所以同寬同方塊感——braille 試過，一格窄長、換上去整列會跳；`spinnerFrame()` 由時鐘算 frame、不累加計數，所以重複的 tick 只多一次重畫、不會跳格）取代 URL 列的 `glyphWeb`；glyph 與 URL 同用 `urlColor`；`spinTickMsg` / `spinCmd()`（`spinStep` 90ms）在 `Update` 的 `tea.KeyMsg`（使用者發動的導覽）與 `pageEventMsg`（頁面自己發動的）兩處武裝，之後靠 `m.fetching()`（任何 tab `loading`）自我續命
- **錨點落點**（2026-09-22）：`tab.landOn` 取代 `jumpToAnchor` 裡的 `cursor` + `scrollToCursor`，多做 `top = items[i].first`，讓目標成為畫面第一列
- **起點**（2026-09-22）：`firstItem` = main 的 `marks` 之後第一個非 landmark 的 item → 沒有 main 就找 `Level` 最小的 heading → 都沒有才 0；`apply` 的 fresh 分支把 `top` 設成該 item 的 `first`（原本固定 0，user 實機看了效果不好）
- **兩套配色**（2026-09-22）：`theme.go` 拆成「App palette」（webu 外殼，吃 VTP）與「Page palette」（文件結構，封閉集合、不參與 z-axis）兩塊，各帶自己的原則註解；page 側一律 `page` 前綴（`pageText` / `pageDim` / `pageLink` / `pageField` / `pageCode` / `pageCodeBg` / `pageKey` / `pageTableBg` / `pageTableHeadBg` / `pageHeadTop`），`linkColor` / `codeColor` / `codeBg` / `tableBg` / `tableHeaderBg` 拿掉；`segStyles` / `codeStyles` / 表格底色全部改指 page 側。mauve 現在 app（pagetab）與 page（code key）各有一份，是兩個獨立決定
- **heading 底色**（2026-09-22）：`row.heading` 存層級（`renderer.head` 在 Heading 分支進出、`emit` 蓋上去），`pagepanel` 多一個 `row.heading > 0` 分支（畫每個 seg 的底、並把底鋪到 `textWidth`）；`theme.headingBg(level)` = `lerpHex(pageHeadTop, crustHex, (level-1)/5)`，`hexRGB` 解析 `#rrggbb`
- **inline 標記**（2026-09-22）：roles `strong` / `emphasis` / `deletion` / `insertion` / `mark` → 新的 `ir.Span`（Role 說是哪一種；`deletion` / `insertion` 原本沒進表、落 Unsupported）；`render.textAttr` 位元欄掛在 `atom` 與 `seg` 上，`renderer.attr` 在 `inline()` 的 `ir.Span` 分支進出、`add()` 一律 OR 上去、`wrap()` 造每個 seg 時帶過去，`pagepanel.withAttr` 疊在 kind 的樣式上（游標列、code、表格四個分支都要）
- **`ir.Markdown()`**（2026-09-22，`internal/ir/markdown.go`）：block / inline 兩層；`children()` 把連續的 inline 子節點併成一段（label 與它的欄位是一行，跟版面同一條規則）；`mdWrite()` 只在需要時插空格（AX tree 會吃掉元素之間的空白，但作者寫在連結後面的標點要貼著）；`para()` 把 `<br>`（Name 是 `\n` 的 text）轉成 markdown 的硬斷行；清單整份是一個 block（項目之間空行會變成 loose list）；跨行的 code 一律 fenced 成 block；`fixtures_test.go` 同時比對 `.golden`（Dump）與 `.md`（Markdown）；app 的 `yankmd` key 進 page Space menu
- **measure**：`renderOpts.measure` 只管 `wrap()` 的 `textW`（0 = 不設上限）；`store.Measure` 是具名型別，`ParseMeasure` / `String` / `UnmarshalYAML` / `MarshalYAML` 收發 `full` 與數字兩種寫法，零值就是 full、**預設 full**（2026-09-22）
- **item 的 col**：`emit` 時記每個 item 在第一列的起始欄位；`tab.rowStep`（j/k 換列、找最近欄位）與
  `tab.alongRow`（h/l 同列）靠它
- textbox 畫成 `󰛿 name ____value____`，底床至少 12 格；checkbox `[x]`、radio `(•)`、combobox `name [value ▾]`、
  media `[󰋩 alt]` 一行 chip（ui.md 的「佔位框 + 尺寸」先做成一行，尺寸要另打 CDP，v1 不做）

## §4 動作（function.md §4、page/actions.go）

**每個 `page.*` 入口都經 `chromedp.Run(ctx, ActionFunc)`**：tab context 只有 Run 過才帶
executor，直接 `.Do(ctx)` 得到 `invalid context`。

| 動作 | CDP |
|---|---|
| Click | `scrollIntoViewIfNeeded` → `getBoxModel` → 真滑鼠 pressed / released 在 box 中心（trusted click，`target=_blank` 才開得了視窗；`el.click()` 是 untrusted、Chrome 不給開）；沒有 box 才 `el.click()`。**不送 `mouseMoved`**：實測 headless 下單獨的 mouseMoved 要等 renderer ack 逾時整整 5 秒，每個點擊都在付，開 dialog 的點擊晚 5 秒才看到 dialog；press + release 2 ms。代價是 hover 才出現的選單 v1 打不開（function.md §4 要的 hover 同步暫緩） |
| Type | focus → select all → `Input.insertText`（框架會收到 input 事件）；空字串是 Backspace |
| Choose | option 元素 `selected = true` + 對 select dispatch `input` / `change` |
| Submit | focus + 送 Enter keydown / keyup（implicit submission） |
| Navigate | `chromedp.Navigate`（等 load event；錯誤文字折進 error）+ `WaitReady("body")` |

動作後 300 ms `settle` 再 capture；tab 上的 `Page.loadEventFired` / `navigatedWithinDocument`
/ `frameNavigated` 也觸發 250 ms 後 capture。`gen` 計數丟掉過期的 capture。游標留位：先比
backendDOMNodeId，找不到落到同序位。

**Loading 與 debounce（ux.md §6 修訂）**：`tab.load`（導航）與 `tab.navigate`（back / forward）
都在按鍵當下設 `loading = true`、`gen++`；任何 capture 落地才清。`AppModel.busy()` 為真時
`dispatch` 吞掉 `back` / `forward` / `R` / `click` / `submit` / `edit` / `clear` / `choose`。畫面：`pageRows`
全部改 dim、游標用 curOff、URL 列圖示換 `glyphLive`、邊框 hint 寫 loading。chromedp 的 `NavigateBack`
沒有上一頁時回 `invalid navigation entry` → `navFailMsg` → 清 loading、toast「nothing to go back to」。

## §5 非 DOM 事件（function.md §5、page/hooks.go）

| 事件 | 落地 |
|---|---|
| alert / confirm / prompt / beforeunload | tab 的 `ListenTarget` 收 `Page.javascriptDialogOpening` → `dialogMsg`（goroutine 送、不丟）→ alert / confirm / beforeunload 開 confirm popup，prompt 開 input popup（帶預設值）；Enter / Esc 都會 `Page.handleJavaScriptDialog` 回答（alert 的 Esc 也是 accept）。dialog 開著時 settle capture 暫停，因為碰 renderer 的 CDP 呼叫會卡到 dialog 關掉 |
| `target=_blank` / `window.open` | `ListenBrowser` 收 `Target.targetCreated`（`Type=="page"` 且有 `OpenerID`、未 attached）→ `newTargetMsg` → `NewContext(WithTargetID)` + `Run` attach → `Prepare`（observer 也直接跑進現有 document）→ capture；加到 `[1]` 尾端並自動切換。實測 `rel=noopener` 預設下 `OpenerID` 仍有值 |
| 憑證錯誤 | **偏離 function.md §8**：`Security.certificateError` 事件已從協定移除，只剩 `Security.setIgnoreCertificateErrors`。做法：導航失敗字串含 `ERR_CERT` → confirm「這個分頁、開著期間都放行？」→ 對該 tab `setIgnoreCertificateErrors(true)` 後重載；同分頁後續導航不再問，新分頁會再問 |
| 下載 | 第一個 frame 對 browser 執行 `Browser.setDownloadBehavior(allow, downloadPath, eventsEnabled)`；`ListenBrowser` 收 `downloadWillBegin` → toast「downloading <name>」、`downloadProgress completed` → toast「saved <path>」、canceled → error toast。目錄 = `config.yaml` 的 `download_dir`，預設 `paths.Downloads()` = `~/.webu/datas/downloads`（2026-09-21），`MkdirAll` 後才 `setDownloadBehavior`（`pointDownloads`，Settings 改值後再跑一次） |
| HTTP basic / digest auth | `Prepare` 開 `Fetch.enable(handleAuthRequests)`；**這會讓每個 request 都 pause 一次**，tab 的 listener 收到 `requestPaused` 就在 goroutine 裡 `continueRequest`（Puppeteer 的 `page.authenticate` 同一做法；實測 GitHub repo 頁多花約 0.5 秒、HN 約 0.3 秒）；`authRequired` → `authMsg` → input popup 問帳號、再問密碼（遮罩）→ `continueWithAuth(ProvideCredentials)`；Esc → `CancelAuth`（頁面拿到 401） |
| 檔案上傳 | `Prepare` 開 `Page.setInterceptFileChooserDialog(true)`；`fileChooserOpened` → `fileMsg` → input popup 問路徑（`~` 展開、多檔以空白分隔、先 stat）→ `DOM.setFileInputFiles(backendNodeId)`；Esc 就是沒選。ui.md §3.3 寫的 sshu filepicker 形式先以路徑輸入代替 |
| Undo close、離開時下載中 | `[1]` 的 `U` 從 `m.closed`（最多 20 筆）重開；`q` 在 `downloading() > 0` 時先 confirm。`downloadWillBegin` / `downloadProgress`（帶 GUID、bytes、state）餵進 `m.dls`，header 右端數進行中的、`[D]ownloads` screen 列出（`downloads.go`：Enter 系統開啟器、`o` 來源開新分頁、`x` remove（進行中 `Browser.cancelDownload`）、`y` yank path、`C` 清已完成）。`c` 關分頁不問：下載是瀏覽器層事件、無法歸到某個分頁 |

## §A VTP in webu

依 ux.md §A 落地。已實作的入口：footer `space menu   ? help   tab/1-2 panels   q quit`；
`[2]` 的 **Enter = 滑鼠左鍵在 terminal 的對應**（`enterItem`，定案 2026-09-21；VTP §A.0.K「啟動該項目最直觀的操作、依 app context 而定」：textbox → `editField`（password 遮罩、prompt 寫 password）、select → `chooseOptions`、button / check / media / unsupported → click、link → `askOpenLink` 開 confirm（`confirmOpenLink`，帶 node id）確認後 click、landmark / heading → fold、其他 → message popup「尚未定義」；list screen 的 bookmark / history 列開新分頁、目錄列開合——每種列跨 surface 一致。2026-09-20 的「Enter 開 item 選單」作廢，`optItemMenu` 拿掉，`optionsKey` 只剩 select 與 move picker），
Space 開完整選單（同一份 item 列 + panel region `[R] [T] [P] [N] [/] [v] [L] [A] [O] [I] [Z] [Y] [C]`）；
`[1]` 的 `[c] [o] [r] [y]` / `[T] [X] [U]`（2026-09-21：close 從 `w` 改 `c`，clone 改 `[o] Open in new tab`）；header 五個 chip 是 screen（`screen` enum；`switchScreen` / `screenKey` / `listAction`，`W` `B` `H` `D` `S` 全域），list screen 的 Space menu = `listPanel.menuItems`，鍵與 `update` 同一組（2026-09-21）。help popup 列全域鍵。
options popup（`m.options`）以 `optionsKind` 區分兩種內容：item 選單、select 的 option 清單（Choose
在原浮層內換內容）；Add to… picker 隨 Shortcuts 拿掉，`[A]` 直接加進 Bookmarks。

### 選取模式（ux.md §1、ui/selectmode.go）

`/` 或 Space menu 的 Select text 進入（`Alt+v` 已拿掉，修訂 2026-09-20）；字元游標走 `t.lay.rows` 的純文字（rune index）。`hjkl` / `w e b`（vim 的
W / E / B：以空白切詞、跨列）/ `0 $` / `u d` / `gg G` / `v V` / `y`（無選取時 yank 整列）/ `/`
smart case / `n N` / Enter 點字元所屬的 item（`layout.itemAtCol`）後離開模式。進入時 IR 凍結：
pageMsg 存到 `tab.frozen`，離開時套用；item 游標落到最近的 item（`layout.nearestItem`）。
Space 開 cheatsheet（message popup，`passKeys`：按列出的鍵 = 關掉 popup 並執行）。URL 列搜尋中
換成 `/query  n/m`（Yellow）、邊框 `toneSelect`、footer 只列模式內的鍵。

### DevTools（ui.md §3.2、ui/devtools.go + devstorage / devnetwork / devconsole / devnetdetail）

- 外殼一檔一 animator：title 列放 `tabChain`（kbu 的 starship chip chain），`h/l` 切分頁，`/` 過濾（每分頁一份），
  `C` 清；開著時每 500 ms 一個 `devTickMsg` 從 tab 的 log 重讀（log 在 chromedp 的 goroutine 填、UI 讀，帶 mutex）
- 資料：`page.DevLog`（`internal/page/devlog.go`）每 tab 一份、`page.Observe` 在 attach 前註冊 listener，
  `Prepare` 開 `Network.enable` / `Runtime.enable` / `Log.enable`；ring 各 500 筆；主 frame `frameNavigated` 清空
  （同 Chrome 預設；preserve log v2）
- Storage：`Network.getCookies(url)` + `DOMStorage.getDOMStorageItems(origin, local/session)`，開啟與切到時重抓；
  `x` = `Network.deleteCookies(name, domain, path)` / `DOMStorage.removeDOMStorageItem`、`y` yank value、
  `C` = confirm 後 `Storage.clearDataForOrigin(origin, "all")`。`file://` 頁沒有 origin，cookie 列得到、storage 空
- Network：method / status / type / URL / size / ms；Enter → detail 子 popup（自己的 animator、在外殼 view 內 composite），
  headers 立刻、body 由 `Network.getResponseBody` 非同步補上（cdproto 已解 base64）
- Console：`consoleAPICalled`（args 轉文字：string 去引號、其餘 description）、`exceptionThrown`、`Log.entryAdded`；
  warn / error 用 override 色。Eval：Enter 開 input popup（`inputEval`）、`Runtime.evaluate(returnByValue,
  awaitPromise, replMode)`，輸入與結果以 level `input` / `result` 進同一份 log（`DevLog.Add`）；popup 每次執行後留著、值清空，Esc 才關
- **坑**：`tea.Sequence(t.act(...), fetch)` 不會等內層 Sequence 跑完（Bubble Tea 把巢狀 Sequence 當 message 交回去就往下走），
  刪 cookie 後的重抓會搶先。要先後執行就寫成一個 cmd（`storageThen`）

### Outline / Zoom / Inspect

- Outline：`layout.marks` 記每個 landmark / heading 的第一列；popup 是 spaceMenu 實例（同 sshu picker 的重用），
  開啟時游標停在「目前位置之前最後一個」項目；Enter → `tab.jumpTo`
- Zoom：`m.zoom` 讓 `[2]` 獨佔整個畫面（narrow 模式同一條路），寬度改變會重排
- 版面（修訂 2026-09-20）：header 一列（`header()`，借 sshu 的 `tabRow` / `tabChain`：目前 screen 的 chip 點亮，右端 `downloading()`）+ 分隔線（`headerRule()` = sshu `tabRule`，`downloadProgress()` 混合進行中下載的百分比當進度條）+ `[1] Tabs`（`sideW` 24）與 `[2] Page` 並列 + footer；`panelH = h - 3`。Places 面板連同 `side1Items` / `cur1` / Shortcuts 全部刪除
- Console 清單（2026-09-21）：`devConsoleTab.view` 每筆折行成多列（`wrapWords`，續行對齊文字欄），視窗以游標那筆為底往上填；`remoteText` 對帶 preview 的物件用 `previewText`；`ConsoleEntry.Detail` = `objectDetail`（`listProps` JS 在頁面裡列全部 own property，eval 結果同步取、console.log 的參數在 goroutine 取回再 `SetDetail(seq)`），detail popup 有 Detail 就顯示它、續行保留縮排
- Console eval（2026-09-21）：`Runtime.evaluate` 不用 `returnByValue`（`window` 會回 Object reference chain is too long），改 `generatePreview`；
  plain object / array 用 `callFunctionOn(JSON.stringify)` 印 JSON，其餘印 `Description {preview}`（`objectText` / `previewText`）
- View source 已搬進 DevTools › Source（`devsource.go`：`chromedp.OuterHTML("html")`、行號、`/` grep；
  修訂 2026-09-20，`V` 讓給 visual mode）；獨立的 viewer popup 一併移除
- Inspect：message popup 列 role / name / value / url / state / node id

**Bubble Tea 的 value receiver 陷阱**（踩過兩次）：會改 popup 狀態的 helper 若是 value receiver、
只回傳 `tea.Cmd`，改到的是沒人保留的副本。規則：這類 helper 用 pointer receiver（作用在呼叫者的
區域副本、再由呼叫者回傳），或先 `close()` 再 `dispatch()`，不要反過來。

## §B 專職化

沿用 ui.md §4。**link 色帶定案提案：teal `#94e2d5`**（family 內未被佔用；ui.md §4 留白說
「畫出來再挑」，這是畫出來的那個）。

## §9 實作狀態

### 已落地

- Chromium 下載 / 啟動 / 關閉、`webu version`、`webu browser update`、`webu <url>`
- role 白名單每 role 一份 fixture；`docs/support.md`
- `[2]` 頁面：URL 列、分隔線、排版、游標、`j/k/u/d/gg/G`、捲動指示與 loading hint
- Enter = 左鍵的對應（2026-09-21）：textbox 開 input popup（password 遮罩）、select 開 option 清單、button / check / media / unsupported click、link 先 confirm 再開、landmark / heading 開合、其他 notice；item 選單只在 Space
- `[1]` 分頁：新開 / 切換（綠字）/ 關閉（`c`）/ 同頁再開（`o`）/ reload / yank / close others；`target=_blank` 尚未接 `Target.targetCreated`
- goto popup（全域 `L` / `[1]` `[2]` 的 `T`）：非 URL 當搜尋（預設 Google，2026-09-21 起）；`L` 帶目前 URL 當 placeholder，Tab 接手編輯、Backspace 清掉（`inputPopup.update`）；label 2026-09-21 改成 `[L]ocation`（popup title「Location」）；`bracketHotkey` 仍支援 label 中段加括號（只對字母；數字鍵在 label 裡出現過曾印成 `dir[1]`，2026-09-21 修）
- `P` / `N` / `R`、Yank url / text / value、Inspect（暫以 toast 呈現）
- 窄寬只畫焦點側；`TestViewFitsTheTerminal` 檢查四種尺寸每列寬度
- header 四個 list screen（`listpanel.go` 一個 model 四種內容：Bookmarks / History / Downloads / Settings，`panelFrameLegend` 畫成單一面板、hint 在下邊框）：Enter 一律開**新分頁**並回 `[W]eb`（2026-09-21）、`x` 刪（confirm）、`y` yank、`A` 加目前頁、
  History 的 `C` 清（confirm）、`/` 打字過濾（子字串，fuzzy 之後）；Esc 先清過濾再回 Web（`handleKey`，用 `floatOwned()` 而非 `popupOpen()` 判斷，否則 confirm 收合動畫期間 Esc 會被吞）；
  Settings 的 Enter → `changeSetting`（`settings.go` 的 `settings` 表，依 config.yaml 順序：`search_engine` / `download_dir` / `measure` 是文字設定 → `settingBox`，目前值當 placeholder，`saveSetting(value, untouched)`：沒動過不改、`set` 回 error 就 toast 並留著框；`restore_session` 是開關（`toggle`）直接翻）→ `saveConfig` = `store.SaveConfig` + 刷新列 + `download_dir` 時 `pointDownloads`、`measure` 時 `applyMeasure` 重排所有分頁；`TestSettingsCoverConfig` 用 reflect 讀 `store.Config` 的 yaml tag，少一列就紅；`restore_session` 由 main.go 讀（false 就不 `WithSession`）；`[2]` 的 `[A]dd bookmark` 直接加
- list screen 第一列是欄位 header（`listPanel.columns`，`rows()` 少算一列）；每列帶 `ref` 指回 backing slice（bookmarks / history / dls / settings），刪除、搬移、開檔都用 ref，不靠顯示順序
- Bookmarks 目錄（`bookmarks.go`，2026-09-21）：`bookmarkEntries` 根層在前、每目錄一列 `isFolder` entry + 其下書籤；`folderNames` = `m.folders`（bookmarks.yaml 的 `folders:`）∪ 使用中的 ∪ 兩者的祖先，依路徑分段排序（`folderLess`：`a` < `a/b` < `a-x`）；目錄是路徑 `dev/go`，`depth` 決定縮排；`[A]`（`m.folderParent` = 游標所在目錄）→ `inputFolder` → `addFolder(parent, path)`（路徑一次開多層）；`[a]` → `startAddBookmark(folder)` → `inputBookmarkURL` → `inputBookmarkTitle`（`m.newBookmark`；placeholder 是目前頁，Enter 未動過就收下）；目錄列 Enter → `toggleFolder`（`m.foldedFolders`，`underFold` 藏起子樹，收合列帶 count）；`[m]` → `movePicker`（`optMoveTo`，數字鍵）→ `moveBookmark` 存檔並 `cursorTo(ref)`；目錄列 `x` → `deleteFolder`（空的直接 `deleteFolderTree`；非空先 confirm `confirmDeleteFolder`（`confirm.folder` 帶路徑、列書籤數 / 目錄數）→ `deleteFolderTree` 把 `inFolder` 的書籤與目錄全拿掉；上層不動；2026-09-21）；每次存檔 `saveBookmarks` 把 `folderNames()` 全寫進 `folders:`，所以路徑帶出來的上層也是正式目錄，不會隨下層消失；`store.LoadBookmarks` / `SaveBookmarks` 多了 folders 參數；**`[I] Import`**（2026-09-21）：`filepicker.go`（從 sshu 搬來、改成一次一個目錄：`enter(dir)` 目錄優先、`.` 開頭略過、打字 `fuzzyScore` 過濾、Enter 進目錄 / 選檔、空過濾 Backspace 回上層並把游標放回來時的目錄；起點 `importDir()` = `~/Downloads` 或 `~`）→ `pickerKey` → `importPicked(path)` 立刻 `store.ParseNetscape`（`store/netscape.go`：regexp 掃 `<h3> <a> <dl> </dl>` 四種 tag、entity 解碼、目錄名的 `/` 換 `-`、Firefox `place:` 略過；沒書籤也沒目錄就回 error）→ `inputImportName` 要根目錄名（`importBookmarks`：空的或已存在的 toast 並留框）→ 全部掛在 `名稱/…` 下、空目錄也進 `m.folders`、存檔、游標到根目錄；`TestImportBookmarks` 用 `HOME` 指到暫存目錄；**`[r] Rename`**（2026-09-21）：`startRename(e)` 開 input popup、框裡帶目前 title 或目錄本層名字（`value`，不是 placeholder：改名多半是改一部分）→ `inputRename` → `renameGiven`：書籤改 `Title`；目錄算出新路徑（parent + name，含 `/` / 撞名 / 空的都 toast 留框），`inFolder` 的書籤、`m.folders`、`m.foldedFolders` 一起換前綴，游標到新路徑
- `internal/store`：`bookmarks.yaml`（flat，`folder` 欄位 + 頂層 `folders:`）、`config.yaml`（`search_engine` / `download_dir` / `measure`；`shortcuts` 已拿掉）、
  `history.yaml`（YAML sequence、一次 append 一筆、無限保留）與 `session.yaml` 在 data dir（`store.dataFiles`，2026-09-21）；目錄解析在 `internal/paths`
  （`Config` = `~/.config/webu`、`Data` = `~/.webu/datas`、`Downloads`、`Cache`；`WEBU_CONFIG` / `WEBU_DATA` / `WEBU_CACHE` 覆寫），browser（profile、log 在 data）/ store 共用
- session：離開（q 或 signal）時 main 寫下所有分頁，啟動時還原成 pending（dim、不預載），切到才載
- splash 彩蛋（2026-09-21，`splash.go` 從 sshu 搬來）：`logoPixels` 由 `docs/icon.svg` 中心點取樣生成（D 底片 / U 框 / W E B 三個字母），stage 順序 bg → W → E → B → U rise；`V` 在 `panelKey` 全域 switch 與 `screenKey` 觸發（家族同鍵；visual mode 因此改成小寫 `v`，唯一的小寫 panel-level 鍵，2026-09-21），`handleKey` 最前面讓 splash 吃掉所有鍵、`View` 整幀換掉
- CLI（2026-09-21）：`webu <url|words>...` 每個參數一個新分頁、第一個在前（`New(b, start...)` → `firstFrame` 逐個 `openTab(resolveURL(u))`），`-` 開頭視為未知選項；`webu help` 印用法
- 歷史：每個分頁 load 完的最終 URL + 標題記一筆，同 URL 的 settle 重抓不重複記
- 即時更新（function.md §6）：`page.Prepare` 在第一次導航前 `Runtime.addBinding` + 注入 MutationObserver
  （childList / characterData / subtree，150 ms 合併），`Runtime.bindingCalled` 走與 load event 同一條 settle 路
- Visual mode（`v`）與 `/` 搜尋、Outline、Zoom、Inspect popup、DevTools 四分頁（Network / Storage / Console / Source，2026-09-21 起 Network 在前、開啟停在 Network）與 Network detail（§A）
- JS dialog、`target=_blank` 新分頁、憑證錯誤 confirm、下載 toast、HTTP auth、檔案上傳、Undo close、離開時下載中的 confirm（§5）
- 整合測試 `TestAppNavigatesAndFillsAForm`（本機頁：載入 → 點 → 回 → 打字 → Submit → 選 option）、
  `TestListPopupsAndSession`（無瀏覽器：B 開 / 過濾 / 刪 / 存檔）、`hooks_test.go` 五支（新視窗、三種
  dialog、自簽憑證 via `httptest.NewTLSServer`、下載、搜尋後 Enter 點連結）、`selectmode_test.go`
  （motion / smart case / yank / outline 純邏輯）；
  真站 smoke `WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v`（Hacker News、GitHub）

### 未做（v1 清單，ui.md §7）

~~Bookmarks 的 Edit~~（2026-09-21 以 `[r] Rename` 落地，URL 不改）、History fuzzy、hover 同步（§4 的 mouseMoved 坑）、
iframe 內容、`h/l` 在 table row 內移動、textarea 的 `$EDITOR` 鏈、檔案上傳的 filepicker popup、
Console Eval（v2）、preserve log（v2）。

ui.md §7 的 v1 表除了上面這些細項之外都已落地；README 的「下一步」與 CHANGELOG 尚未建立（尚未 tag）。
