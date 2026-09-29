# webu — terminu fix

webu 還沒符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.19/principle)（tdp v0.1.19）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

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
- **修完拿 v0.1.19 的 rules 與 defaults 全文再逐條對一次**，不只看 CHANGELOG。`dev-remarks.md`「webu 照 tdp v0.1.17 逐條修完
  （2026-09-29）」那句這次沒動；修完改成 v0.1.19 與修完的日期。
- **不 push、不發版**：家族與 tdp 都穩定之前不發 release。
- 修完把這一輪寫進 terminu repo 的 `.local/family-fix/webu/README.md`：開頭的清單加第 10 點，另加一節「第十輪（v0.1.18）」。
- **第 5 條（icon 寬度）等 filu**：filu 的 `internal/ui/width.go` 是參考實作，filu 要先補完自己的內容列；webu 等 filu 做完再照搬。
  這一輪先做第 1–4 條，第 5 條留在清單上。第 4 條的「整組捨棄」量寬度時先用現在的 `dispW()`，第 5 條換成新的顯示寬度函式時一起換掉。


## 1. visual mode 沒有在框上寫出模式名 —— K11、D2（v0.1.18）

**現況**：webu 唯一的模式是 visual mode（其他候選見「已經符合」）。`app.go` `pagePanel()` 在 `m.sel.on` 時用 `toneSelect`，
`chrome.go` `panelChromeTone()` 把 `[2]` 的外框與 `[2] Page` 膠囊畫成 Yellow（`selectColor` `#f9e2af`），離開模式就回到 Blue ——
顏色已符合。但上框只有左邊的膠囊，接著一路 `─` 到 `╮`，**畫面上沒有任何地方寫出「現在在 visual mode」**：只有 footer 換成模式的鍵、
`?` 的標題是 `Visual mode`。focus 的線型：webu 的 focus 與非 focus 都是圓角 `╭─╮`（`panelChromeTone()` 一律畫 `╭`），模式裡線型沒變
—— 但設計文件寫的是雙線，見「待確認」第 1 題。

**規則**：K11（v0.1.18）—— 模式名一律顯示在模式所在的框（panel 或 popup）的**上框右側**，外框換成模式色；離開模式就恢復。focus 的
panel 在模式裡照樣是 focus 的線型，只有顏色換掉。D2 —— 模式的外框與右上角的模式名用 Yellow `#f9e2af`。

**怎麼改**：`panelChromeTone()`（或 `pagePanel()` 畫完之後）在上框右側放模式名，例如 ` Visual mode ─╮`（跟 `?` 的標題同一個名字），
用外框的 Yellow；只在 `m.sel.on` 時出現，離開就沒有。窄寬與 zoom 時是同一個 `pagePanel()`，一起有。膠囊加模式名放不下時，先不畫模式名
（跟膠囊放不下時不畫一樣），框線不能被推歪。測試：`sel.on` 時 `View()` 的 `[2]` 上框（去掉顏色）以 `Visual mode ─╮` 結尾、模式名是
Yellow；`sel.on` 為假時沒有；40 欄的窄寬也量一次每列寬度（L4）。mutation：拿掉模式名那一段、把顏色改回 `focusColor`，各自要紅。
文件：`ux.md` §1 兩種游標表的「視覺」與 §B 元素專職化「邊框 Yellow」補「上框右側寫 `Visual mode`」；`ui.md` §4 最後那句同樣補上；
README 兩份的「Visual mode」段可以補一句「`[2]` 的框變黃、右上角寫 Visual mode」（建議，使用者看得到的變化）。


## 2. finder 在清單上時，篩選列還是亮的 —— F1、D3（v0.1.18）

**現況**：webu 的 finder（有候選清單、`Tab` 在打字與清單之間切換）只有 `/` 的搜尋（`finder.go`）。`listColumn()`：

- 打字時（`finderInput`）：篩選列的 glyph 是 `handColor`、查詢字 `textColor`、後面 Lavender 的游標，數量 Overlay0；清單 cursor 列是
  `handColor`（Subtext1 `#bac2de`）底、深色字 —— 跟家族預設的「篩選列亮、cursor 列淡的反白」一樣（kbu 的淡反白也是 Subtext1）。
- `Tab` 到清單後（`finderNav`）：篩選列只少了游標，glyph、查詢字、數量的顏色**都跟打字時一樣** —— 兩邊都亮，看不出 focus 在清單上；
  清單 cursor 列換成 `focusColor`（Blue）底、深色字、不粗體。

**規則**：F1（v0.1.18）—— finder 的 focus 在哪一邊要看得出來，只有拿鍵的那一邊是亮的。D3 —— `Tab` 到清單後，篩選列整列用 Overlay0
`#6c7086` 畫（不用 F8 的淡化，也不畫反白與游標）；清單的 cursor 列換成 popup 層色底加深色粗體字。

**怎麼改**：`listColumn()` 在 `finderNav` 時把篩選列整列（glyph、查詢字、數量）畫成 `dimColor`，不畫游標；`cur` 在 `finderNav` 時用
`popupLayerColor(f.layer)` 底、`baseHex` 字、`Bold(true)`（`hitRow()` 用同一個 style）。打字時照舊。測試：同一個 finder 在兩個階段各
render 一次（開顏色），量篩選列的查詢字前景（打字：Text；清單：Overlay0）與 cursor 列背景（打字：Subtext1；清單：層色）、粗體；
mutation：篩選列不換色、cursor 列換回 Blue，各自要紅。文件：`ux.md` §1.1 finder 那一段補「focus 在哪一邊哪一邊亮」。

注意：D3 說清單 cursor 列「跟 menu 的 cursor 列一樣」，那是家族預設的 menu；webu 的 menu cursor 列（`spacemenu.go` `view()`）是
Subtext1 底，這條不動它。


## 3. 失焦的 `[2]` 邊框上的 hint 還用 Blue —— D2（v0.1.18）

**現況**：`app.go` `pagePanel()` 不論 focus 在不在 `[2]`，下框都用 `popup.go` `statusLegend()` 畫：狀態字（`loading`、`a popup is up:
answer it`、`2/26 · 節名 · 40%`、`1-20 of 80`、`12 items`）Overlay0，後面的鍵走 `hintLegend()`（鍵 Blue、冒號與說明 Overlay0）。
focus 換到 `[1]`（`Tab`、`1`）時 `[2]` 的手可以還在 pagetab 上（`panelKey()` 換 focus 不動 pagetab），這時失焦的 `[2]` 下框是
`12 items Enter:stay`，`Enter` 是 Blue。其他 panel 邊框：`[1]` 沒有 hint；list screen 只有一個 panel，一律是 focus（`listpanel.go`
`panel()` 傳 `toneFocus`）。

**規則**：D2（v0.1.18）—— 失焦 panel 邊框上的 hint：鍵 Overlay0 `#6c7086`、冒號與說明 Surface2 `#585b70`；Blue 是 focus 的顏色，只給
拿鍵的地方。

**怎麼改**：`statusLegend()` 與 `hintLegend()` 多一個「這個框有沒有 focus」的參數（或傳一組樣式）；`pagePanel()` 在 `tone ==
toneIdle` 時用失焦的配色：鍵 `dimColor`、冒號與說明 `borderDim`。狀態字照說明的顏色走（focus 時 Overlay0、失焦時 Surface2）——
第九輪 user 定狀態字用說明的顏色，這裡照同一個對應。visual mode（`toneSelect`）算 focus。測試：focus 在 `[1]`、`[2]` 的手在 pagetab
上時，`[2]` 下框 `Enter` 的前景是 Overlay0、`:stay` 是 Surface2；focus 回 `[2]` 時 `Enter` 是 Blue。mutation：失焦時照舊用 Blue，要紅。

讀一節時 `[2]` 下框的進度條（`pagepanel.go` `panelFrameFilled()` 的 `━`）不論 focus 都用 `toneColor(toneFocus)` 的 Blue —— 它不是 hint，
條文沒寫到，見「待確認」第 2 題。


## 4. 下框 hint 放不下時截在項目中間 —— D3（v0.1.18）

**現況**：

- `popup.go` `drawPopupBoxPad()`：`hint = clipANSI(hint, innerW-1)`，照格數硬切。所有走它的 popup 都是這樣：`spacemenu.go`（Space menu、
  global operation、options、choices）、`confirm.go`（含 quit confirm）、`inputpopup.go`、`editorpopup.go`、`filepicker.go`、`finder.go`
  （清單框）、`helppopup.go`、`messagepopup.go`、`toast.go`、`devnetdetail.go`、`pagepanel.go` `pagePopupFloats()`（頁面彈窗）。
- `devtools.go` `body()`：自己畫框，`clipANSI(hintLegend(pairs), innerW-1)`，同樣硬切。
- `pagepanel.go` `panelFrameFilled()`：`[2]` 的下框（狀態字加鍵）寬過 `innerW-4` 就整條不畫 —— 不是從尾端一組一組捨棄，狀態字會跟著
  後面的 `Enter:stay` 一起消失。
- scratch 實測：finder 並排時（寬 ≥ 96）清單框內寬只有 35–45 欄，清單階段的 hint `j/k/u/d:move Enter:go Tab:query Esc:close`（43 格）
  在寬 96 印成 `… Tab:query E`、寬 100 `… Esc`、寬 110 `… Esc:clo`，寬 120 才放得下。DevTools 四個分頁在寬 80–140 都放得下，其他 popup
  用全寬，目前沒看到被切，但都走同一個硬切。
- 已經符合的：footer（`chrome.go` `keyLegend()`）與 list screen 的下框（`listpanel.go` `fitLegend()`）都從尾端整組捨棄。

**規則**：D3（v0.1.18）—— 下框 hint 放不下時，從尾端整組捨棄（跟 D1 的 footer 一樣），不截在項目中間。

**怎麼改**：把 `listpanel.go` 的 `fitLegend()` 搬到 `popup.go` 當唯一的做法，popup 的 hint 以項目（`[][2]string`）傳進 `drawPopupBox()` /
`drawPopupBoxPad()`，照框寬整組捨棄；`devtools.go` `body()` 同一個函式；`panelFrameFilled()` 先捨棄後面的鍵、最後才捨棄狀態字。測試：
寬 100 的 finder 清單階段，下框最後一項是完整的（`Tab:query`，沒有半個 `Esc`）；`fitLegend()` 單元測試（剛好放得下、差一格、一項都放
不下）；`[2]` 在窄寬時 `Enter:stay` 先消失、狀態字留著。mutation：換回 `clipANSI()`，要紅。


## 5. icon 的實際寬度 —— D6（v0.1.18；等 filu 做完再照搬）

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


## 6. focus 只靠顏色分辨 —— L5（v0.1.19）

**現況**：`chrome.go` `panelChromeTone()` 一律畫圓角 `╭─╮`，focus 只換顏色；`ui.md` §4 與 `ux.md` §B 卻寫 focus 是雙線 `╔═╗`（程式從來
沒畫過）。第 1 條的 visual mode 外框換 Yellow 之後，只靠顏色就看不出 focus。

**規則**：L5（v0.1.19）—— focus 不能只靠顏色分辨；家族預設 focus 雙線 `╔═╗`、失焦圓角 `╭─╮`，兩者同寬（D2）。user 2026-09-29 裁定家族
統一成雙線（kbu、filu、locku 已經是）。

**怎麼改**：`panelChromeTone()` 依 focus 選框線：focus 雙線、失焦圓角；膠囊、下框 hint、狀態字的位置不動（同寬、切換不位移）。跟第 1
條一起做：visual mode 的 Yellow 框保留雙線。測試：focus 的 `[2]` 是雙線、`[1]` 是圓角，換 focus 前後每一列寬度不變。文件：`ui.md` §4、
`ux.md` §B 已經這樣寫，改完就對得上。


## 7. 讀一節時下框的進度條拿掉（user 裁定）

**現況**：讀一節時（`ui.md`「一節」那一列），`[2]` 的下框本身填成進度條：`pagepanel.go` `panelFrameFilled()` 把讀過的部分畫成 `━`，
固定用 focus 的 Blue，失焦也一樣。

**已定案**（user 2026-09-29）：**粗線進度條拿掉**，下框只留狀態字 `2/26 · 節名 · 40%`。

**怎麼改**：`panelFrameFilled()` 的 `pct` 那段拿掉（或整個函式併回 `panelFrame()`），呼叫端不再傳進度；狀態字照第 3 條的顏色。header
下載時的進度線（`chrome.tabRule`）是另一件事，不動。測試：讀一節時下框沒有 `━`、狀態字還在；原本守進度條的測試改寫成守「沒有」。
文件：`ui.md` 第 15、31 行不動（那是 header 的下載進度）；「一節」那一列的「下框本身填成進度條」與 Border hint 那一列的「讀一節時
下框本身是進度條」拿掉；dev-remarks 若有提一起改。


## 已經符合、不用修的（對照 v0.1.18 的改動）

- **K10、K9**：webu 沒有 PTY —— 沒有跑在 app 裡的 shell、編輯器或遠端 session（`exec.Command` 只用在交給桌面開檔與剪貼簿），「子程序
  還沒準備好時不轉送按鍵」與「PTY 裡 `q`、`Ctrl-C` 屬於子程序」都**不適用**。
- **K11 的其他候選**：照術語「模式」（進入後一部分鍵換意思、`Esc` 離開，不是輸入態）逐個看過，都不是模式：
  - list screen 與 DevTools 的 `/` 篩選、visual mode 裡的 `/` 搜尋：打字時是輸入態（K8），`Enter` 之後篩選留著但鍵沒換意思。
  - editor popup 的寫 / 移兩態：寫是輸入態；移是框本身的狀態（從寫 `Esc` 出來、再 `Esc` 關框），不是進入後再 `Esc` 回去的模式。
  - 讀一節、走進一件事或 frame、pagetab：是往下 / 往上的層級與游標位置，`Esc` 一層一層回來，鍵沒換意思（pagetab 上的 `h/l` 就是 core
    的「同列移動」）。
  - zoom、窄寬只畫一側、`Sections` / `One sheet`：版面切換，照 v0.1.14 的術語不是模式。
- **K11 的顏色與恢復**：visual mode 的外框與膠囊已經是 Yellow，離開就回到 Blue（只缺模式名，第 1 條）。
- **F1、D3 的打字階段與其他有清單的 popup**：finder 打字時篩選列亮、cursor 列 Subtext1 淡反白，已是家族預設。file picker 是附候選清單的
  input（沒有 `Tab` 切換，一直是打字階段），也是這個樣子。`go` 的清單只有一個階段（數字篩選與 `j/k` 同時有效，沒有 `Tab`），不是 F1
  說的那種 finder。DevTools 與 list screen 的 `/` 篩選打完 `Enter` 就回清單，也沒有 `Tab` 切換。
- **D2 的其他部分**：Yellow 就是 `selectColor` `#f9e2af`；focus 時 hint 與 footer 的鍵 Blue、冒號與說明 Overlay0（第九輪）。
- **D3 的 footer 與 list screen**：`keyLegend()`、`fitLegend()` 已經從尾端整組捨棄（第 4 條把 `fitLegend()` 推到所有 popup）。


## 待確認

沒有。上一版兩題 user 2026-09-29 裁定：

1. **focus 的線型**：家族統一成雙線，程式跟文件走（第 6 條；v0.1.19 的 L5 寫明 focus 不能只靠顏色）。
2. **讀一節的進度條**：整條拿掉，只留後面的數字（第 7 條）。
