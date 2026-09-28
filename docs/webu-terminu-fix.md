# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.12/principle)（tdp v0.1.12）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與描述該行為的設計文件段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-28（對照 tdp v0.1.11）。v0.1.11 改了 K3（沒有項目的內容 panel 上的 `Enter`）、F6（picker 的選擇可以算確認）、
F7（高度只在 loading 與使用者自己的操作時可變，loading 要在 popup 標題後面放轉圈的 loading icon）、F8（dim 是把每個顏色——前景與背景——
往底色淡化，不准剝色、丟背景、把前景換成同一個 dim 色）與 D2（dim 的算法 `c × 0.45 + base × 0.55`，filu 的 `dim.go` 是參考實作）。
webu 要修三條：**F8 / D2 的 dim 做法**（第 1 條）、**DevTools 裡等資料的三處沒有 loading icon**（第 2 條），以及對照全文時找到、
v0.1.9 就該修的**item menu 裡的 select / slider 在同一個框裡換內容**（第 3 條）。其餘逐條對照過，見「已經符合」。

F7 點名的 loading icon 就是 webu 自己 URL 列的那個（第 2 條「loading icon 的樣子」寫給家族其他 app 照做）。

---

## 先看

- **做成一個共用的 dim，不要一處一處修。** 把 filu 的 `internal/ui/dim.go`（`dimANSI` / `dimSGR` / `dimRGB` / `xterm256` / `ansi16`）
  換進 webu 的 `internal/ui/dim.go`，改寫**已經畫好的字串**裡每一個 SGR 顏色碼。`app.go` `View()` 的兩處呼叫（base 畫面與關閉中的上層框）
  不用動；`dimLayers`、`layerDimOf`、`dimFg` 被它取代（popup 邊框本來就是層色，淡化後自然是「層色的 dim 版」，不必再特別比對層色）。
  base 用 `theme.go` 的 `baseHex`，沒有前景的文字給 `dim(textColor)` = `#6d7187`。
- **要看顏色才驗得到。** 測試的輸出不是終端機，lipgloss 預設不畫顏色：照 `dim_test.go` 的 `withColour(t)` 打開 TrueColor。
  改完把 `View()` 印出來，**不要 strip**，直接貼到終端機看：Space menu 打開時，header 的畫面 chip（`[W]eb` 那一段的藍底）、
  `[1] Tabs` / `[2] Page` 的膠囊、`[1]` 的 cursor bar、`[2]` 的游標列、pagetab 鏈、code block 與表格的底色都要還在，只是變暗；
  DevTools 開著 Network detail 時，DevTools 標題裡的 `Network` 膠囊也要還在。至少 80 × 40（L1）看一次。
- **測試要能看背景。** `dim_test.go` 的 `fgs()` / `has()` 只比對前景（`38;2;…`），要另外一個比對背景（`48;2;…`）的：膠囊、cursor bar、
  選取、code block 靠的是背景。
- **預期值寫死成條文算出來的數字**，不要用被測的 `dimRGB()` / `lerpHex()` 算（filu 那輪「同源自比」抓到三次）。
- **逐處 mutation**：每一處修正單獨改回舊行為（`dimSGR` 丟掉背景那一支、把所有前景換成同一個色、丟掉 bold / reverse、
  loading icon 不畫、select 的選項清單改回在 item menu 的框裡換內容），跑對應的測試，確認會紅。
- **守舊 dim 的測試改寫，不是刪掉**：`dim_test.go` `TestDimANSI` 量的是「警示色變成 `dimColor`」「沒有 `48;`（背景拿掉）」，
  `TestOnlyTheTopPopupIsBright` 量的是「header 裡有 `dimColor`」，改成量 D2 的淡化結果（警示色是 `dim(warnColor)`、背景是 `dim(背景)`、
  header 的進度是 `dim(liveColor)`）；「哪一層亮」「toast 不 dim」「關閉中那一刻底下就亮回來」「邊框是層色的 dim 版」那幾段照留。
- **描述 dim 與 loading 的文件一起改**：`ui.md` §3「只有最上層是亮的」（「暗的內容用 dim 色」）與「高度打開時定好」（Network detail 那句）；
  `dev-remarks.md`「運作方式」裡 `dim.go` 那段（「其他前景換成 `dimColor`、背景拿掉」）；`dev-remarks.md`「偏離 tdp」頁面彈窗那條的
  「照 F8 變暗」後面可補「照 D2 淡化」。帶日期的歷史紀錄不改，照慣例在後面補一句新的日期與做法。README 兩份沒有寫 dim 與 loading，不用改。
- **不發版**：等家族全部 app 與 tdp 都穩定後一起發（CHANGELOG 記在 `[Unreleased]`）。

---

## 1. dim 把前景全換成 `dimColor`、丟掉背景與字型屬性，拆掉膠囊、cursor bar 與選取 —— F8、D2

- **現況**：`app.go` `View()`（2848 行起）在最上層之前把整個畫面過 `dimANSI`（2898 行），關閉中的上層框也過一次（2902 行）。
  `dim.go` 的做法：
  - `dimANSI`（25 行）開頭寫一個 `dimColor` 前景，之後每個 SGR 交給 `dimSGR`。
  - `dimSGR`（93 行）**每個 SGR 都改寫成 `\x1b[0m` + 一個前景**：
    - 前景是四個 popup 層色之一（`layerDimOf`，±2 容許）→ 那個層色往 base 混 55%（這一支剛好就是 D2 的算法）；
    - **其他前景一律 `dimColor`（Overlay0 `#6c7086`）**：警示色、下載進度的 `liveColor`、URL 的藍、標題的各層色相、語法色都變成同一個灰；
    - **背景全部丟掉**：`48;2;…`、`48;5;…` 被跳過，`40–47` / `100–107` 也因為輸出只剩 reset + 前景而消失；
    - **字型屬性全部丟掉**：reset 之後沒有補回 bold、italic、underline、strikethrough、reverse（D2 要 bold、reverse 不動）；
    - 16 色、256 色的前景（`30–37`、`90–97`、`38;5;N`）不換 RGB，直接變 `dimColor`。終端機不支援 truecolor 時 lipgloss 會降成 256 色，
      那時連層色邊框都認不出來，全變 `dimColor`。
  - 沒有前景的文字：`dimColor`（D2 要 `dim(Text #cdd6f4)` = `#6d7187`，兩者很接近，但要照算式）。
  - 每一列開頭不補預設字色（filu 每個 `\n` 之後補一次）：`overlay.Composite` 逐列切開再拼，webu 現在靠「每個 SGR 都改寫」撐住，換成 filu 的做法後
    要連同這一點一起搬。

  靠背景畫出來、因此現在一 dim 就壞掉的東西（壞法都一樣：背景沒了、字變同一個灰）：
  - **header 的畫面 chip**（`chrome.go` `tabChain`，89 行）：目前畫面那一段是 base 色字壓在 `focusColor` 背景上，接縫是「左段色前景 + 右段色背景」
    的三角形。任何 popup 一開，`[W]eb` 那一段的藍底消失，只剩兩個灰色半圓夾著灰字。
  - **panel 標題膠囊**（`chrome.go` `panelChip`，286 行：base 字、bold、壓在邊框色背景上）：`[1] Tabs`、`[2] Page` 與 list 畫面的標題膠囊，本體不見。
  - **DevTools 標題裡的分頁鏈**（`devtools.go` `body()` 268–272 行用 `tabChain`）：Network detail 開著時 DevTools 在底下被 dim，
    `Network` / `Storage` / `Console` / `Source` 那條膠囊鏈變成灰字。
  - **`[2]` 的 pagetab 鏈與 part 鏈**（`pagepanel.go` `pagetabChain` 632 行、`partChain` 685 行）：填色段靠背景。
  - **cursor bar**（base 字壓在 `handColor` / `borderDim` / `editColor` 背景上）：`spacemenu.go` 223、227 行（Space menu、global operation popup、
    options 的 cursor——被 confirm、message、input、`?` 蓋住時看不出停在哪一列）、`sidebar.go` 21–22 行、`listpanel.go` 368 行、
    `pagepanel.go` `rowStyles` 247–248 行與標題列的 `levelColor` 背景（311–313 行）、`devnetwork.go` 48 行、`devstorage.go` 102 行、
    `devconsole.go` 59 行、`filepicker.go` 243 行、`finder.go` 500 行。
  - **輸入的游標塊**（base 字壓在 `editColor` 背景上）：`inputpopup.go` 195 行、`editorpopup.go` 246–249 行、`finder.go` 468 行、
    `devtools.go` 264 行（filter 列）——input 上開 `?`、有下載時按 `q` 的 quit confirm 蓋上來時，游標塊消失。
  - **visual mode 的選取**（`selectmode.go` 444–447 行：`selectionBg`、`borderDim`、`selectColor` 背景）：模式裡按 `?` 開 key reference，
    底下的選取反白消失。
  - **頁面內容的底色**：code block（`pageCodeBg`，`pagepanel.go` 255、267 行）、表格與表頭（`pageTableBg` / `pageTableHeadBg`，346–350 行）、
    頁面輸入框（`segCaret` 的 `pageInputBg`，420 行）、tablist 選中的那一段（333–334 行）、section list 的 chip（`sectionlist.go` 49 行）。
  - **頁面的字型屬性**（`pagepanel.go` `withAttr` 388 行）：`<mark>` 的 reverse 反白、連結的底線、標題的粗體，dim 之後全部變成一般灰字。
  - **頁面自己的彈窗**（`pagePopupFloats`，130 行）：它在 dim 之前就合成進 base 畫面，所以 webu 的 popup 開在它上面時它跟著 dim（這是對的，
    `dev-remarks.md`「偏離 tdp」已寫明）；但它的游標列（`st.cur`，`handColor` 背景）一樣消失。
- **規則**：F8：dim 是把每一個顏色——前景與背景——往底色淡化，形狀與版面不動；不可以剝色重畫、丟背景、把前景換成同一個 dim 色；
  邊框是自己層色的 dim 版。D2：`dim(c) = c × 0.45 + base × 0.55`，base `#1e1e2e`，前景背景都算，16 / 256 色先換 RGB，
  沒有前景的文字給 `dim(Text)`；bold、reverse、游標移動、文字本身不動。
- **怎麼改**：
  - `dim.go` 換成 filu 的 `dimANSI` / `dimSGR` / `dimRGB` / `xterm256` / `ansi16`：每個 `38;2` / `48;2` / `38;5` / `48;5` / `30–37` / `90–97` /
    `40–47` / `100–107` 都換成淡化後的 truecolor，其他參數（`1`、`3`、`4`、`7`、`9`）原樣保留；`0` 與 `39` 之後補 `dim(textColor)`；
    每個 `\n` 之後補一次 `dim(textColor)`。
  - 刪掉 `dimLayers`、`layerDimOf`、`dimFg`（統一的淡化已經給出層色的 dim 版；D2 的邊框表 `lerp(…, K/N)` 算出的層色，淡化後仍各自不同）。
    `fgSGR` / `rgbKey` 若沒有別人用，一起刪。
  - `app.go` `View()` 的呼叫點、`floats()` 的順序、頁面彈窗先合成再 dim 的順序都不動。
  - 測試（`dim_test.go`）：
    - `TestDimANSI` 改寫：輸入含前景、背景、bold + reverse、256 色、沒設色的文字；斷言版面不變（`ansi.Strip` 相同）、每個前景 / 背景都是
      寫死的 `c × 0.45 + base × 0.55`、`1` 與 `7` 還在、沒設色的文字是 `#6d7187`。
    - 新增：開 Space menu，`View()` 第一列（header）裡目前畫面那一段的背景是 `dim(focusColor)`；`[2]` 的膠囊背景是 `dim(邊框色)`；
      `[1]` 的 cursor bar 背景是 `dim(handColor)`。
    - 新增：開 DevTools、Network detail 疊上去，DevTools 標題那一列還有 `dim(focusColor)` 的背景（分頁鏈）。
    - 新增：頁面彈窗開著時開 Space menu，彈窗的游標列背景是 `dim(handColor)`（頁面彈窗仍然跟著 dim）。
    - `TestOnlyTheTopPopupIsBright` 的 header 斷言改成 `dim(liveColor)`，其餘照留。

---

## 2. DevTools 裡等資料的三處沒有 loading icon —— F7

- **現況**：打開時內容還不確定、要等 Chromium 回答的地方有三處，都沒有在標題後面放轉圈的 icon：
  - **Network detail**（`devnetdetail.go` `show`，113 行）：打開時只有 headers，body 用 `bodyMsg` 晚一步到（`app.go` 315 行 `setBody`）。
    等 body 時框裡寫一列 `(loading body…)`，標題是 ` <glyphDevTools> <URL> `（`view`，256 行），不轉。高度一打開就是最大高度（120 行），body 到了
    在框裡捲動——高度這部分已經符合。
  - **DevTools 的 Storage 分頁**（`devtools.go` `update` 166–171 行：切到 Storage 就 `devFetchStorage`；`app.go` 1224 行 `fetchStorage`，
    `storageMsg` 在 309 行落地）：等回答時照畫上一次的資料，第一次則是三個空的 `cookies · 0` / `local storage · 0` / `session storage · 0`，
    看不出是「還在讀」還是「真的沒有」。
  - **DevTools 的 Source 分頁**（切到 Source 就 `devFetchSource`；`devsource.go` `view` 70 行）：等回答時框裡寫一列 `  loading…`，不轉。
  DevTools 本身與 Network detail 的高度都打開時就定好（DevTools 滿版，`rows()`；detail 最大高度），所以高度沒有問題，缺的只有揭露。
- **規則**：F7：打開時內容還不確定（串流、載入中）的 popup，loading 期間要揭露——popup 標題後面放一個輪轉的 loading icon（webu 載入網址時的
  那個 icon）；loading 結束，icon 消失。
- **loading icon 的樣子**（webu 是來源，家族照這個做）：
  - **字形**：Nerd Font 的 Material Design circle slice，八格依序 `nf-md-circle_slice_1` … `_8` = U+F0A9E、U+F0A9F、U+F0AA0、U+F0AA1、
    U+F0AA2、U+F0AA3、U+F0AA4、U+F0AA5——一個圓一片一片填滿，滿了再從第一片開始（`theme.go` `spinnerFrames`，212 行）。程式碼裡寫成
    code point（`string(rune(0xf0a9e))`），不寫 PUA 字面。
  - **速度**：一格 90 ms（`spinStep`，218 行），一圈 720 ms。
  - **哪一格由時鐘決定，不是計數**：`spinnerFrames[(now / 90ms) % 8]`（`spinnerFrame`，223 行）。tick 不帶資料，排在**下一格該出現的時刻**
    （`pagepanel.go` `spinCmd`，30 行），晚到的 tick 就畫下一格，不會跳格也不會因為多一次重畫而走快。tick 鏈只在有東西在 loading 時續（`app.go` 282 行）。
  - **寬度**：一格（`lipgloss.Width` 算 1），跟它取代的靜止 glyph `nf-md-web`（U+F059F，`glyphWeb`）同寬，換上換下不位移（L2）。
    點字（braille）試過又放棄：它是細長的一欄，跟旁邊方正的 Nerd Font glyph 形狀不同，一開始 loading 那一列就跳一下。
  - **顏色**：跟旁邊的字同色。URL 列是 `urlColor`（Blue `#89b4fa`），icon 與網址一個顏色；放在 popup 標題後面時，用標題的顏色（該層的層色、bold）。
  - **在 webu 的位置**：`[2]` 的 URL 列 ` <icon> <URL>`（`pagepanel.go` 66–75 行）：`t.working()`（loading 或 settling）時 icon 是 spinner，否則是 `glyphWeb`。
- **怎麼改**：
  - 三處都在標題後面放 `spinnerFrame()`：Network detail 是 ` <glyphDevTools> <URL> <icon> `，body 落地（`setBody`）就不畫；DevTools 是 ` <glyphDevTools> DevTools <icon> `，
    再接分頁鏈，Storage / Source 的回答落地就不畫。icon 不在時那一格留空白，`titleW`（`devtools.go` 270 行）與 detail 標題的截寬（`fitURL(…, innerW−12)`）
    都先把這一格算進去，分頁鏈與上框不因 icon 出現、消失而左右移（L2）。
  - 給 `devDetailPopup` 一個「body 還沒到」的狀態（例：`waiting bool`，`show` 設、`setBody` 清），給 `devtoolsPopup` 一個「Storage / Source 在讀」的狀態
    （fetch 時設、`storageMsg` / `devSourceMsg` 清；回答屬於別的分頁或別的 tab 時不清錯）。
  - tick 鏈：`spinTickMsg` 現在只在 `m.fetching()`（有分頁在載入）時續；改成「有分頁在載入，**或** DevTools / detail 在等回答」時續，
    fetch 的地方（`app.go` 1224–1226 行、1257–1259 行）也要 `spinCmd()` 起鏈。DevTools 自己的 500 ms `devTickMsg` 太慢，不能拿來轉 icon。
  - 框裡的 `(loading body…)` / `  loading…` 可以留（說的是框裡這一塊是什麼），但揭露 loading 的是標題的 icon。
  - 測試：打開 Network detail、body 還沒送到時標題裡有 spinner 的某一格、`bodyMsg` 之後沒有，框高前後不變；Storage、Source 同樣；
    切換 icon 前後標題列的寬度與分頁鏈的位置不變。

---

## 3. item menu 裡選了 select 或 slider，選項清單在同一個框裡換內容 —— F1（v0.1.9 多步驟）、F7、F4

- **現況**：表格的格、chrome 的膠囊這類「一格裡有好幾個目標」的東西，`Enter` 打開 item menu（`options` popup，`optItemMenu`，
  `app.go` `openItemMenuAt` 2331 行），一列一個目標（`targetItems`）。在 item menu 選了一個目標，走 `entry:` → `actOn`（1936–1945 行、2343 行）：
  - 目標是 **select**（`ir.Combobox`）→ `chooseOptionsFor`（2399 行）用 `m.options.setItems` 把**同一個 options 框**的內容換成選項清單，
    框已經開著就直接換（2421–2422 行註解「swapped in place under the open float」）。
  - 目標是 **slider** → `editFieldAs` → `slideMenu`（2496 行），同樣在同一個框裡換成數字清單，並把窗口設成 10 列（2500 行）、框已開就直接換（2504–2505 行）。
  結果：選欄位的那一步與選值的那一步擠在同一個框裡；框的高度跟著新清單的列數或 10 列窗口變；`Esc` 關掉整個框，回不到上一步的 item menu。
- **規則**：F1（v0.1.9 起）：多步驟的流程每一步是自己的 popup，疊起來（F4 保留 source），不在同一個框裡換內容——每一步有自己打開時定好的高度（F7）。
  F7：高度打開時定好，只有 loading 與「使用者在這個 popup 裡的動作讓列數改變」可以變；換成另一步的清單不是這兩種。F4：取消上層回到 source。
- **怎麼改**：
  - 選項清單另開一個 popup，疊在 item menu 上（例：第二個 menu 實例 `choices`，layer + 1），自己的 animator、自己打開時定高
    （select：選項數，上限照 `capRows`；slider：10 列窗口）。
  - `floats()` 加在 `options` 之上、`closeTop` 與按鍵路由讀同一份順序（D3「放在最上層要同時改三處」，webu 已經收成 `floats()` 一份）；
    測試的 `settle` / 等動畫的名稱清單記得加它。
  - `Esc` 只關選項清單、回到 item menu（F4）；選了一個值照現在的 `closeStack()` 關整疊（T1）。
  - 從 panel `[2]` 的 Space menu 或 `Enter` 直接選 select / slider 時（沒有 item menu 在底下），行為不變：選項清單疊在 Space menu 上或單獨打開。
  - 測試：在一格有 select 的表格上 `Enter` → 選 select 那一列 → 選項清單是另一個 popup（item menu 仍 `isActive()` 且被 dim）、item menu 的內容沒被換掉；
    `Esc` 回到 item menu；選一個值後整疊關閉。slider 同樣。量框高：選項清單打開後高度不變。

---

## 已經符合、不用修的（對照 v0.1.11）

- **K3（沒有項目的內容 panel 上的 `Enter`）**：webu 沒有「內容區、沒有項目」的 panel。`[1]` 是分頁清單、`[2]` 是頁面的項目（cursor 走 node）、
  list 畫面是清單，全部是有項目的 panel；`[2]` 還沒有頁面、載入中、載入失敗時是**空清單**的空狀態（D1：一句事實 + 點名按鍵，`L` / `T` / `R`），
  `Enter` 不作用跟空清單一樣。DevTools 的 Source 分頁是 note popup 裡的 viewport，不是 panel。
- **F6（picker 的選擇算確認）**：webu 已經這樣做。`[2]` 上 `Enter` 一個連結先跳 `Open link` confirm（`askOpenLink`）；但從 item menu 選了連結，
  `actOn`（`app.go` 2339–2356 行）直接點，不再 confirm——「清單本身就是看過一次」。其他 confirm（刪書籤資料夾、清歷史、清網站資料、憑證錯誤、
  離開、頁面的 `confirm()`）都不是從 picker 來的。
- **F7 其他 popup 的高度**：
  - finder（`/`、`go`）：打開時把整頁建成索引（`indexPage`，同步），結果清單是打字篩出來的——內容打開時就確定，沒有 loading；框的幾何只看畫面大小，
    打字篩選改的是清單內容不是框。
  - file picker：`os.ReadDir` 同步讀本機目錄，沒有 loading；高度打開時定好（`filepicker.go` 64 行），過濾、換目錄只換內容。
    v0.1.11 允許使用者自己的操作改高度，webu 選擇不改，一樣符合。
  - DevTools 滿版定高；Network / Console 分頁是持續更新的 log，框不變（見「待確認」第 1 題）。Console detail、message、confirm、input、editor、
    help、quitAsk、quitHelp、Space menu、global operation popup：內容打開時就確定、開著時不重建（Space menu 只在 `openMenu` 設一次）。
  - 頁面載入不是 popup：`[2]` 的 URL 列轉 spinner，就是 F7 引用的那個 icon。
- **F8 的其他部分**：只有最上層亮（`floats()` 裡最後一個 `owns()`）、關閉中的上層畫暗、toast 不觸發也不被 dim、底下的串流（header 的下載進度）與
  警示色一起 dim、畫的順序 / `closeTop` / 按鍵路由同一份清單——這些 v0.1.8 那輪已經做好，這次只換「怎麼 dim」。
- **頁面自己的彈窗**：它是 `[2]` 的內容（`dev-remarks.md`「偏離 tdp」），不照 F7、F8；它在 `View()` 裡比 webu 的 popup 先合成、在 `dimANSI` 之前，
  所以 webu 的 popup 開在它上面時它照樣跟整個畫面一起 dim。第 1 條換掉 `dimANSI` 的內部做法，這個順序不動，它就會照 D2 淡化（游標列、層色邊框都保留）。
- **v0.1.9「每一步是自己的 popup」的其他流程**：加書籤與 HTTP 驗證是 input group（一步）；searchbox 是 input → confirm「Search」兩個 popup；
  書籤匯入是 picker → input 兩個 popup；Space menu → global operation popup、options → message（Inspect）都是疊上去的。只有第 3 條那兩處是在同一個框裡換。
- **K1–K11、M1–M9、L1–L5、F1–F5、X、T、S**：前幾輪（v0.1.0–v0.1.10）已逐條修完，這次對照全文沒有看到新的違反；v0.1.11 沒有改這些條文。

---

## tdp v0.1.12 定案（2026-09-28，回答四個 app 在 v0.1.11 盤點時的共同問題）

修的時候以這裡為準；本檔的條目與「待確認」照下面改讀。

- **F7 loading icon 一定要放**：popup 在 loading 時，標題後面一定放輪轉的 loading icon，**跟高度會不會變無關**。
  loading 指**整個 popup** 的內容還沒到；若只是**某一個項目**本身是持續進來的資料流，loading 的是那個項目，怎麼揭露由 app 決定。
- **F7 使用者操作造成的高度變化是「允許」不是「要求」**：app 可以維持原高；原則是揭露的資訊要正確。
- **D2 淡化絕不讓顏色變亮**：每個通道取原值與淡化值較小的那個；比 base 還暗的顏色（例：`#000000`）維持原色。
- **D2 / D6 truecolor**：家族要求 truecolor terminal，dim 一律輸出 24-bit，不必照色彩深度降階；README 的需求段跟 Nerd Font
  並列寫上「需要 truecolor terminal」（D6，家族預設）。
- **D3 loading icon 規格**：Nerd Font `nf-md-circle_slice_1`–`_8`（U+F0A9E–U+F0AA5）八格；一格 90ms；由時鐘決定哪一格
  （`frames[(now / 90ms) % 8]`），tick 只在有東西 loading 時續排；寬一格；顏色跟旁邊的字，放在 popup 標題後面時用該層層色（bold）。

- **本檔**：
  - 待確認第 1 題（DevTools Network / Console 一直流進來的 log）照 F7 定案：那是**項目**層級的資料流，不是 popup 的 loading，
    怎麼揭露由 webu 決定；但第 2 條的 Network detail 等 body、Storage、Source 是**整個 popup 的內容還沒到**，要放 icon。
  - 新增：dim helper 要照 D2 加「絕不變亮」這一步。
  - 新增：README 兩份的需求段補「需要 truecolor terminal」。
  - D3 的 loading icon 規格就是 webu 的 `spinnerFrames`，其他 app 照它做。
  - 待確認第 2 題（頁面彈窗的 backdrop 仍是舊 dim 畫法）仍是 webu 自己的決定。

## 4. 頁面彈窗的 backdrop 把前景收成一個 dim 色 —— F8、D2（user 2026-09-28 裁定照改）

- **現況**：頁面自己的彈窗開著時，底下的頁面用 `pagepanel.go` `rowStyles` 的 loading 樣式畫（258–270 行）：所有字 `dimColor`、游標改成
  `borderDim` 的 curOff；下層的頁面彈窗也這樣畫（`pagePopupFloats` 150 行）。它是頁面內容的畫法、寫在偏離裡不照 F8，但跟 F8 v0.1.11 禁止的
  做法同形（前景收成一個 dim 色、背景與字型屬性不見）。
- **裁定**：換成 D2 的淡化 —— backdrop 先照原色畫，再過同一個 `dimANSI`；code block、表格底色、標題色相、游標列都保留，只是變暗，跟 webu 的
  popup 蓋上來時一致。「正在離開的頁面」（載入中）用的是同一套樣式，但它不是 popup、不在 tdp 範圍，**不動**。
- **怎麼改**：`pagepanel.go` 在畫彈窗底下的頁面與下層彈窗時，改成「原色畫 → `dimANSI`」，不再借 loading 樣式；loading 樣式留給載入中。
  `dev-remarks.md`「偏離 tdp」頁面彈窗那條補一句：它的 backdrop 也照 D2 淡化（2026-09-28）。
- 測試：頁面彈窗開著時，backdrop 裡一個 code block 的背景是寫死的 `dim(pageCodeBg)`、不是沒有背景；載入中的頁面仍是原本的 loading 樣式。

## 5. README 沒寫需要 truecolor 終端機 —— D6（v0.1.12）

- **現況**：README 兩份的需求段只寫 Nerd Font。
- **規則**：v0.1.12 D6：家族要求 truecolor terminal；README 的需求段跟 Nerd Font 並列寫上。
- **怎麼改**：README 兩份「Two things to know」／「有兩件事要知道」加一條「需要 truecolor 終端機」。

## 已定案（2026-09-28）

- **DevTools 的 Network / Console 不放 loading icon**：v0.1.12 F7 定案 —— 一直流進來的 log 是**項目**層級的資料流，不是整個 popup 的 loading，
  怎麼揭露由 webu 決定；webu 不放（沒有結束的一刻，轉不停的 icon 像卡住）。第 2 條的 Network detail 等 body、Storage、Source 是整個 popup 的
  內容還沒到，要放。
- **頁面彈窗的 backdrop 照 D2 淡化**（user 裁定）：見第 4 條。
