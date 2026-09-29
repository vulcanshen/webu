# webu — terminu fix

webu 還沒符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.20/principle)（tdp v0.1.20）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

> **v0.1.20（2026-09-29）**：K11 / D3 —— 模式名夾在兩個框線接頭之間（雙線 `╡Drag╞`、單線 `┤Visual├`），模式色加粗、盡量一個詞，
> 放不下先截標題，panel 膠囊跟著外框換色；D6 —— icon 寬度量的是游標實際前進幾格，參考實作的完整清單、`<APP>_ICON_WIDTH`
> 覆寫、只在 unix 探測、做完的驗收。filu 的參考實作已完成，icon 寬度那一條現在可以做。本清單的每一條已照 v0.1.20 重新核對過。

> **v0.1.19（2026-09-29，這份清單寫完後才出）**：L5 —— focus 不能只靠顏色分辨（模式會換框色），家族預設雙線；K10 ——
> 子程序還沒準備好時可以不轉送一般的鍵，但 `Ctrl-C` 照樣轉送。本清單的每一條已照 v0.1.19 重新核對過。

盤點日期：2026-09-29（對照 tdp v0.1.18）。以 `main` 的 `3f97478` 為準（工作樹乾淨）。webu 在第九輪已對齊 v0.1.17，這一輪**只核對
v0.1.17 → v0.1.18 的改動**：K11 / D2 模式標示自己、F1 / D3 finder 的 focus、D2 失焦 panel 邊框上的 hint、D3 下框 hint 放不下時的捨棄、
D6 icon 的實際寬度、K10 / K9 PTY 的鍵。每條照內容對程式碼核對過，位置寫檔案與函式，不寫行號。finder 與 DevTools 的下框是在
scratch 複本裡 render 量的（沒有動 webu 的工作樹）。


## 先看

- **清單先 commit，再動程式**；程式的 commit 跟清單分開，commit 只加自己改的路徑。
- **每修一處補 model test，逐處 mutation**：把修正單獨改回舊行為，確認對應的測試會紅。量顏色要開顏色（`dim_test.go` 的
  `withColour`），預期值照實際輸出寫死、旁邊註明原色（lipgloss 會四捨五入）。
- **同一個 commit 同步 README 兩份與 `docs/dev-remarks.md`**（`ux.md` / `ui.md` 有寫到的那一段也一起）；CHANGELOG 記在 `[Unreleased]`。
- **修完拿 v0.1.20 的 rules 與 defaults 全文再逐條對一次**，不只看 CHANGELOG。`dev-remarks.md`「webu 照 tdp v0.1.17 逐條修完
  （2026-09-29）」那句這次沒動；修完改成 v0.1.20 與修完的日期。
- **不 push、不發版**：家族與 tdp 都穩定之前不發 release。
- 修完把這一輪寫進 terminu repo 的 `.local/family-fix/webu/README.md`：開頭的清單加第 10 點，另加一節「第十輪（v0.1.18）」。
- **第 5 條（icon 寬度）等 filu**：filu 的 `internal/ui/width.go` 是參考實作，filu 要先補完自己的內容列；webu 等 filu 做完再照搬。
  這一輪先做第 1–4 條，第 5 條留在清單上。第 4 條的「整組捨棄」量寬度時先用現在的 `dispW()`，第 5 條換成新的顯示寬度函式時一起換掉
  （`popup.go` 的 `fitLegend()` / `fitStatus()`）。


## 已修完（2026-09-29）

第 1–4、6、7 條已修完、已從本清單刪掉：`de2d7c3`（第 6 條，focus 雙線）、`240ad42`（第 7 條，進度條拿掉）、`c6c7aa5`
（第 1 條，visual mode 寫出模式名）、`0754472`（第 3 條，失焦 hint 變灰）、`a14771c`（第 2 條，finder 的 focus）、`bc5458f`
（第 4 條，hint 整組捨棄）。只剩第 5 條，等 filu。

## 5. icon 的實際寬度 —— D6（filu 已完成，照搬）

**filu 的參考實作已完成**（2026-09-29，`e1de220`，filu 第六輪）。照搬的東西（v0.1.20 的 D6 有同一份清單，細節在 terminu
`.local/family-fix/filu/README.md`「第六輪」最後的「D6 照搬清單」）：

- filu `internal/ui/width.go` 整個檔：`iconCells` / `IconCells()`、`isWideIcon()`、`iconCount()`、`dispWidth()`、`dispClip()`、
  `padDisp()`、`padDispRight()`、`truncate()`、`dispCutLeft()`、`compositeDisp()`（跟 `overlay.Composite` 同介面，直接換掉呼叫）、
  `centerDisp()`（取代 `lipgloss.Place`）、`blockWidth()`、`joinH()` / `joinV()`（取代 lipgloss 的 Join）。
- `iconwidth_unix.go` 的 `DetectIconWidth()`，在 `tea.NewProgram` 之前呼叫；手動覆寫用 `<APP>_ICON_WIDTH`（filu 是
  `FILU_ICON_WIDTH`）。探測只在 unix 做，Windows 預設一格、靠環境變數覆寫。
- 測試照 `d6_test.go`：icon 1 / 2 格下每一種 popup 各開一次，量**單獨的框**（並排的框量單一個）與**疊上去的整個畫面**每一列；
  `compositeDisp()` 的四種邊界（popup 列有 icon、被蓋的列有 icon、icon 被左 / 右框邊切半）。
- 驗收：`grep -n 'lipgloss.Width\|lipgloss.Size\|lipgloss.Place\|ansi.StringWidth\|ansi.Truncate' internal/ui/*.go` 只剩寬度函式本身。
- filu 的提醒：寬度改走 `dispWidth()` 後，在 `iconCells = 1` 的終端機上畫面完全不變（既有測試原封不動通過），只有探測到 2 才作用。


**現況**：webu 量寬度已經集中在少數幾個函式，但全都是 icon-blind（Nerd Font 的 icon 一律量成一格）。盤點：

- **量寬度的函式**（`width.go`）：`dispW()` = `lipgloss.Width`；`truncate()`、`truncateHead()` 逐字用 `dispW()`；`clipANSI()` 用
  `ansi.Truncate`；`padRight()`、`padLeft()`。`util.go` 的 `centerLine()`、`fitLines()`；`empty.go` 的 `wrapHint()`；`devnetdetail.go`
  的 `wrapWords()`；`pagepanel.go` 的 `fitURL()`；`listpanel.go` 的 `fitLegend()`；`chrome.go` `keyLegend()` 的 `plainW`。
- **拼接與疊放**：`chrome.go` 的 `joinVertical()` / `joinHorizontal()` 直接用 `lipgloss.JoinVertical` / `JoinHorizontal`（`View()` 的
  `[1]` 與 `[2]` 並排、finder 清單與預覽並排）；`overlay.Composite`（bubbletea-overlay，內部用 `lipgloss.Size`、`ansi.StringWidth`、
  `ansi.Truncate` / `TruncateLeft` 切背景那一列）用在 `app.go` `View()` 三處（頁面彈窗、float 疊層、toast）與 `devnetdetail.go`
  `over()`。背景列在 popup 左邊有 icon 時，切點會偏。
- **呼叫 `dispW()` 等函式的地方**（依檔案）：`app.go`（`pagePanel()`、`tabsPanel()`、`View()`、`targetRow()`、`openItemMenuAt()`、
  `sectionOpenHint()`）、`chrome.go`（`tabRow()`、`tabChainW()`、`panelChromeTone()`、`keyLegend()`）、`popup.go`（`drawPopupBoxPad()`）、
  `pagepanel.go`（`rowLine()`、`insideRow()`、`headRow()`、`boxRule()`、`chainW()`、`lineNum()`、`pageBody()`、`panelFrameFilled()`、
  `pagePopupFloats()`）、`render.go`（頁面排版：`wrap()`、`foldSegs()`、`fitSegs()`、`formField()`、`formLabelW()`、`formValueW()`、
  `table()`、`codeBlock()`、`landmarkRule()`、`block()`、`emit()`、`truncateNoEllipsis()`）、`sectionlist.go` `sectionRows()`、
  `sidebar.go` `tabsBody()`、`finder.go`（`listColumn()`、`hitRow()`、`previewColumn()`、`padLines()`、`view()`）、`listpanel.go`
  `panel()`、`filepicker.go`、`spacemenu.go`、`helppopup.go`、`inputpopup.go`、`editorpopup.go`、`confirm.go`、`messagepopup.go`、
  `toast.go`、`devtools.go` `body()`、`devnetwork.go`、`devstorage.go`、`devsource.go`、`devconsole.go`、`devnetdetail.go`、
  `empty.go`、`selectmode.go` `selectRows()`。
- **icon 在哪裡**：`theme.go` 的 glyph 表（BMP PUA `U+F0xx` 與補充 PUA-A `U+F0xxx`：標題、清單、part、頁面元素的 link / input / button /
  select / check / radio / media / frame / canvas / unsupported）、`sectionlist.go` 的 `glyphCode`、spinner 的八格（`U+F0A9E`–`U+F0AA5`）。
  膠囊的圓角（`capLeft` / `capRight`，`U+E0B6` / `U+E0B4`）在 CJK icon 字型上仍是一格，filu 的 `isWideIcon()` 已排除 `U+E0A0`–`U+E0D7`。
- **寫死 icon 格數的地方**：`theme.go` `loadingIcon(false)` 用一個空白佔 spinner 的位子（標題與後面的東西不位移），icon 佔兩格時
  要是兩個空白；`pagepanel.go` `pageBody()` 的 URL 列 `fitURL(…, innerW-3)` 與 `devnetdetail.go` `show()` 的 `innerW()-14`（註解寫
  「the icon's cell and its space」）把 icon 當一格扣掉，要改成量出來的格數。有 `dispW(glyph)` 的地方（例：`finder.go` `hitRow()`）
  換了函式就會跟著對。
- **頁面內容**：網頁文字裡的 CJK 字本身就是兩格，`lipgloss.Width` 已經量對，跟 icon 分開看、不動。網頁文字也可能自帶 PUA 字元
  （icon font 的字，例：APG tree 的 `U+F07B`，`dev-remarks.md`「已知的牆」），終端機一樣用 Nerd Font 畫，照 icon 算是對的。
- **不受影響**：visual mode 以字元（rune）為單位（`selectmode.go` 的 `col`、`render.go` `itemAtCol()` 數 rune），不量格數；`dim.go`
  `dimANSI()` 只改 SGR，不量寬度；splash 的 logo 是方塊點陣，沒有 icon。
- **測試**：L4 的畫面測試是 `app_test.go` `TestViewFitsTheTerminal`（100×30、72×20、60×15、40×10，兩種 focus 與四個 list screen），
  目前只跑 icon 一格；`render_test.go` 的排版 golden、`keys_test.go` `TestEveryPopupIsOneWidth` / `TestInputGroupLegendFits`、
  `loading_icon_test.go` 用 `dispW()` 或 `ansi.StringWidth` 量寬度。
- **啟動**：`cmd/webu/main.go` 目前沒有探測 icon 寬度。

**規則**：D6（v0.1.18）—— app 啟動時探測 icon 佔幾格，所有量寬度的地方（補空白、截斷、框線、疊 popup）都走同一個顯示寬度函式；L4 的
畫面測試也跑一次「icon 佔兩格」。參考實作：filu `internal/ui/width.go`（`DetectIconWidth()`、`isWideIcon()`、`dispWidth()`、`dispClip()`）。

**怎麼改**（等 filu 補完內容列、定案後照搬）：`width.go` 換成 filu 那一套 —— `dispW()` 改成 icon-aware 的顯示寬度、`clipANSI()` 換成
`dispClip()` 的做法、`truncate()` / `truncateHead()` / `padRight()` / `padLeft()` 都照它量；`joinHorizontal()` 換成 filu 的 `joinH()`；
`overlay.Composite` 的四處要有 icon-aware 的切法（filu 的 `view.go` 目前也還在用 `overlay.Composite`，照 filu 最後的做法）；
`loadingIcon(false)` 與 `fitURL()` 照 icon 格數。`cmd/webu/main.go` 在進 TUI 之前探測（filu 是 `cmd/filu/main.go` 呼叫
`ui.DetectIconWidth()`、探測在 `iconwidth_unix.go`），`webu version`、`webu help`、`webu browser update` 不探測。測試：
`TestViewFitsTheTerminal` 多跑一次 icon 兩格，並加上有頁面內容（排版 golden 的頁面）與開著 popup 的畫面；`render_test.go` 的寬度檢查
也跑兩種；mutation：`dispW()` 換回 `lipgloss.Width`，兩格那一輪要紅。文件：`dev-remarks.md` 運作方式寫明寬度一律走哪個函式、為什麼。


## 8. 模式名沒有夾在框線接頭之間 —— K11、D3（v0.1.20）

**現況**：`chrome.go` `panelChromeMode()` 在上框右側寫 ` Visual mode `（`selectmode.go` 的 `visualModeName`，跟 `?` 標題共用，前後
各一個空白，沒有接頭），兩個詞；放不下時整個不寫。

**規則**：K11（v0.1.20）—— 模式名夾在兩個框線接頭之間，像框上嵌了一個標籤；K11 也要求模式名**一律**顯示。D3 —— 接頭跟框同色、
線型跟著框（雙線 `╡` `╞`、單線 `┤` `├`）；模式名用模式色加粗；盡量一個詞；**放不下先截標題、模式名留著**；panel 膠囊跟著外框換成
模式色（webu 已經是）。

**怎麼改**：`panelChromeMode()` 把名字畫成 `╡Visual╞`（`[2]` 在 visual mode 時是 focus 的雙線），接頭用框色、名字用 Yellow 加粗；
名字改成一個詞 `Visual`（`?` 的標題要不要維持 `Visual mode`、拆成兩個常數由 webu 定）；放不下時先截膠囊後的標題、名字留著，不再
整個不寫。測試：visual mode 的上框含 `╡Visual╞`、接頭是框色、名字是 Yellow（量名字本身的 SGR）；內寬 20、24、25 時名字都在、
標題被截。參考 kbu `app.go` 的上框標籤（`248f883`）。

**裁定**（2026-09-29，照建議）：框上寫一個詞 `Visual`（D3：窄的 panel 放不下兩個詞）；`?` 的標題維持 `Visual mode`（標題是一句話，
不受一個詞的限制）。拆成兩個常數。


## 待確認

沒有。上一版兩題 user 2026-09-29 裁定：

1. **focus 的線型**：家族統一成雙線，程式跟文件走（第 6 條；v0.1.19 的 L5 寫明 focus 不能只靠顏色）。
2. **讀一節的進度條**：整條拿掉，只留後面的數字（第 7 條）。
