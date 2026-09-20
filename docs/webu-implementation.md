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
| 目錄 | cache：`WEBU_CACHE` > `XDG_CACHE_HOME/webu` > `os.UserCacheDir()/webu`；config：`WEBU_CONFIG` > `XDG_CONFIG_HOME/webu` > `os.UserConfigDir()/webu`（同 kbu / filu / sshu：macOS 沒設 XDG 時落在 `~/Library/…`，ui.md §6 寫的 `~/.config/webu` 是 XDG 拼法） |
| flag | Puppeteer 那組減 `--enable-automation`（否則 `navigator.webdriver` 為 true），加 `--disable-blink-features=AutomationControlled`、`--headless=new`、`--user-data-dir=<config>/profile`；三個 background throttling 關閉（function.md §6 的節流坑）。`TestLaunchAnswers` 斷言 `navigator.webdriver === false` |
| crashpad | macOS 上 Chromium 一律在 `~/Library/Application Support/Chromium/Crashpad` 放一個空資料庫，`--disable-breakpad` / `--disable-crash-reporter` / `--crash-dumps-dir` 都壓不掉；幾 KB，接受。`--crash-dumps-dir` 留著，Linux 吃 |
| 關閉 | `chromedp.Cancel`（讓 profile flush）再 allocator cancel（保底 kill）；`main.go` 接 SIGHUP / SIGINT / SIGTERM |

## §2 翻譯層（function.md §3）

### 2.1 role 表是單一宣告

`ir.Roles`：role → `{Kind, Action, Transparent, Skip, Display}`。`docs/support.md` 由
`ir.SupportDoc()` 產生，`TestSupportDoc` 保證不漂。未列的 role 走 `ir.Fallback`
（Unsupported：名字當文字、子節點保留、glyph 標記、focusable 就是 item、Enter 仍 click）。

### 2.2 實測出來的事（設計文件沒寫）

- **AX tree 沒有 block / inline**：`generic` 對 div 與 span 一視同仁，兩個相鄰 `<div>` 的字會黏成一行。
  `page.Capture` 多打一次 `DOMSnapshot.captureSnapshot(["display"])`，以 backendDOMNodeId 對回去；
  `ir.Capture{Nodes, Display}` 是翻譯層的輸入。透明容器若 display 是 block 級且裡面有 inline 內容，
  留一個 `Group` 保住斷行；inline kind（link / button …）若被排成 block，`Node.Block = true`
- **`url` 就在 AX node 上**：link 的 href、image 的 src 都是 AX property，不用 `DOM.getAttributes`
- **password**：Chromium 自己把 value 遮成 `•`；`Protected` 由此判定，輸入 popup 遮罩
- **contenteditable**：`generic` + `focusable` + `editable="richtext"`，視為多行 textbox（Slack / Gmail 的輸入面）
- **`<select>`**：`combobox`，option 在 `MenuListPopup` 子樹；IR 把 option 收成 Combobox 的 Children
- **`spinbutton`**（`type=number`）：值在 `valuetext`；當 textbox 編輯
- **HN 的表**：Chromium 給的是 `table` / `row` / `cell`（不是 LayoutTable），renderer 以「有沒有 header cell」判斷：
  沒有就是排版表，一列一行流式；有就對齊欄位
- **caption / rowgroup / Legend / Figcaption** 透明或跳過；`Canvas` / `Iframe` / `Video` / `Audio` 大寫開頭

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
- 表格 cell 內的 block 攤平成一行（`inCell`）；資料表欄寬從最寬的欄開始縮（同 sshu）
- 空格延後放：字與空格一起塞不下就換行，不會出現 `right(`
- textbox 畫成 `󰛿 name ____value____`，底床至少 12 格；checkbox `[x]`、radio `(•)`、combobox `name [value ▾]`、
  media `[󰋩 alt]` 一行 chip（ui.md 的「佔位框 + 尺寸」先做成一行，尺寸要另打 CDP，v1 不做）

## §4 動作（function.md §4、page/actions.go）

**每個 `page.*` 入口都經 `chromedp.Run(ctx, ActionFunc)`**：tab context 只有 Run 過才帶
executor，直接 `.Do(ctx)` 得到 `invalid context`。

| 動作 | CDP |
|---|---|
| Click | `scrollIntoViewIfNeeded` → `getBoxModel` → 真滑鼠 moved / pressed / released 在 box 中心；沒有 box 就 `el.click()` |
| Type | focus → select all → `Input.insertText`（框架會收到 input 事件）；空字串是 Backspace |
| Choose | option 元素 `selected = true` + 對 select dispatch `input` / `change` |
| Submit | focus + 送 Enter keydown / keyup（implicit submission） |
| Navigate | `chromedp.Navigate`（等 load event；錯誤文字折進 error）+ `WaitReady("body")` |

動作後 300 ms `settle` 再 capture；tab 上的 `Page.loadEventFired` / `navigatedWithinDocument`
/ `frameNavigated` 也觸發 250 ms 後 capture。`gen` 計數丟掉過期的 capture。游標留位：先比
backendDOMNodeId，找不到落到同序位。

## §A VTP in webu

依 ux.md §A 落地。已實作的入口：footer `space menu   ? help   tab/1-3 panels   q quit`；
`[3]` 的 Space menu item region 依 role（menu-only、無字母）、panel region `[R] [P] [N] [U] [Y]`
有效，其餘列以 disabled 呈現並註明 not in this build；`[2]` 的 `[w] [c] [r] [y]` / `[T] [X]`；
`[1]` 三列各只有一個動作，Space = Enter。help popup 列全域鍵。

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
- `[3]` 頁面：URL 列、分隔線、排版、游標、`j/k/u/d/gg/G`、捲動指示與 loading hint
- Enter = click；textbox 空 → input popup、有值 → Submit / Edit / Clear / Yank；select → option 清單
- `[2]` 分頁：新開 / 切換（綠字）/ 關閉 / clone / reload / yank / close others；`target=_blank` 尚未接 `Target.targetCreated`
- goto popup（`U` / `T`）：非 URL 當 DuckDuckGo 搜尋
- `P` / `N` / `R`、Yank url / text / value、Inspect（暫以 toast 呈現）
- 窄寬只畫焦點側；`TestViewFitsTheTerminal` 檢查四種尺寸每列寬度
- `[1]` 三個 popup（`listpopup.go` 一個 model 三種內容）：Enter 開啟、`o` 新分頁、`x` 刪（confirm）、`A` 加目前頁、
  History 的 `C` 清（confirm）、`/` 打字過濾（子字串，fuzzy 之後）；`[3]` 的 `[A] Add to…` 二選一
- `internal/store`：`bookmarks.yaml`（flat，`folder` 欄位先留著）、`config.yaml`（`search_engine` / `download_dir` / `shortcuts`）、
  `history`（append-only、tab 分隔、無限保留）、`session.yaml`；目錄解析在 `internal/paths`，browser / store 共用
- session：離開（q 或 signal）時 main 寫下所有分頁，啟動時還原成 pending（dim、不預載），切到才載
- 歷史：每個分頁 load 完的最終 URL + 標題記一筆，同 URL 的 settle 重抓不重複記
- 即時更新（function.md §6）：`page.Prepare` 在第一次導航前 `Runtime.addBinding` + 注入 MutationObserver
  （childList / characterData / subtree，150 ms 合併），`Runtime.bindingCalled` 走與 load event 同一條 settle 路
- 整合測試 `TestAppNavigatesAndFillsAForm`（本機頁：載入 → 點 → 回 → 打字 → Submit → 選 option）、
  `TestListPopupsAndSession`（無瀏覽器：B 開 / 過濾 / 刪 / 存檔）；
  真站 smoke `WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v`（Hacker News、GitHub）

### 未做（v1 清單，ui.md §7）

Bookmarks 的目錄樹與 Edit / Move、History fuzzy、Undo close、`target=_blank` 接 `Target.targetCreated`、
Outline、DevTools（Storage / Network / Console）、`/` 搜尋、選取模式、Zoom、View source、下載 toast、
憑證錯誤 confirm、`beforeunload` / JS dialog / HTTP auth / 檔案上傳 hook（function.md §5）、heading Fold、
Inspect 的 message popup、iframe 內容、`h/l` 在 table row 內移動、textarea 的 `$EDITOR` 鏈、
`download_dir` 的實際使用。
