# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.9/principle)（tdp v0.1.9）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與描述該行為的設計文件段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-28（對照 tdp v0.1.8，v0.1.9 的定案與 user 的裁定已併入各條；第 5 條改判符合、刪除，第 6 條是 v0.1.9 新帶出的）。v0.1.8 只改了 popup 的規則：F1（六類）、F7（尺寸與位置，新）、F8（層疊 dim，新），
連帶 T2（popup 蓋上時串流內容也 dim）、K3（錯誤寫進 input 預留的錯誤列）、D4（key reference 的寬度改照 F7）。

---

## 先看

- **這是橫跨每個 popup 的改動**，不是一條一條 popup 修。建議做成共用的四件，每個 popup 只呼叫：
  1. **一個寬度函式**：`min(terminal 寬 − 2, 120)`（外框），取代現在各 `view()` 自己算 `want` 再丟進 `popupInnerW` 的做法；
  2. **一個「打開時定高」的規則**：`open` / `ask` / `show` 時算一次內容高度（上限畫面高度扣留白），存在 popup 上，`view()` 只用它，
     內容變多就在框裡捲動、變少就留白；
  3. **一個「最上層以外全部 dim」的合成器**：`View()` 裡決定誰是最上層，底下的一切（base 畫面、頁面彈窗、下層 popup）用 dim 色畫；
  4. **input popup 裡一列預留的錯誤列**。
- **「放在最上層」要同時改三處**：按鍵路由、`closeTop`、繪製順序（D3）。F8 多了第四處「誰是亮的」—— 它必須跟 `closeTop` 認定的最上層
  是同一個（見第 3 條：現在繪製順序與 `closeTop` 的順序有幾處不一致）。
- **測試印 `View()` 量**：寬度、高度、哪些列是 dim 色，都用 `ansi.Strip(m.View())` 或直接檢查 ANSI 色碼量，不只看 model 欄位
  （第三輪的經驗：寬度、截斷這類只有看畫面才抓得到）。每一處改動做一次 mutation（拿掉那行、測試要紅），注意編譯不過的假象。
- **守舊尺寸的測試要改寫，不是刪掉**：例如 `keys_test.go` `TestInputGroupLegendFits` 量的是「打開時的寬度不變」，改成量「寬度 =
  `min(W − 2, 120)` 且不變」，「長網址整串在框裡」那段照留（100 欄時框夠寬）。
- **文件**：`docs/ui.md` §3 的 popup 表（「寬度依內容」）、`docs/ux.md` §A.2（「框的寬度依最長的一行（tdp D4）」）、§2.1
  （「框裡寫出原因」）、§2 表格裡「不合形狀 toast、框留著」、`dev-remarks.md` 裡描述 popup 寬度與 input 寬度的段落，跟著改。
- 修完**不發版**：等家族全部 app 與 tdp 都穩定後一起發（CHANGELOG 記在 `[Unreleased]`）。

### popup 盤點（寬高以 `internal/ui` 現在的程式為準；W、H 是 terminal 寬高，寬度都是框內寬）

| popup | 檔案 | F1 類別 | 寬度（現在） | 高度（現在） | 開著時會變嗎 |
|---|---|---|---|---|---|
| Space menu | `spacemenu.go` | menu | 依內容：`max(title+6, label+hint+4, 標題列, 下框 hint+1)` | 列數，上限 `H−6`，超過捲動 | 不變（`setItems` 只在打開時） |
| global operation popup | `spacemenu.go`（`globalMenu`） | menu | 同上 | 同上 | 不變 |
| options（select、slider、Move to…） | `spacemenu.go`（`options`） | menu | 同上 | 同上；slider 固定 10 列一窗 | 不變 |
| `?` key reference（`help`、`quitHelp`） | `helppopup.go` | note | 依最長一行（註解寫 tdp D4） | 條目數，上限 `H−6` | 不變 |
| confirm（`confirm`、`quitAsk`） | `confirm.go` | confirm | `max(title+6, 最長一行+4)` | 行數 | 不變 |
| message（Inspect、格內容、code 全文、visual mode cheatsheet） | `messagepopup.go` | note | `max(title+6, hint+1, 最長一行+4)`；格內容先折到 `min(72, W−12)` | 行數，上限 `H−6` | 不變 |
| input / input group（Location、欄位、設定、書籤、登入、JS prompt、console prompt…） | `inputpopup.go` | input | `openWidth()`：至少 44，打開時定一次；**但 `view()` 取 `max(width, refused+3)`** | 每欄 3 列 + 欄間 1 列；**送出失敗時多 2 列** | **會**：錯誤出現時變高、可能變寬 |
| editor（textarea） | `editorpopup.go` | input（多行，寫 / 移兩態） | `max(60, 最長一行+6, prompt+3)`，每次 `view()` 重算 | 2 + `max(3, min(行數, H−10))` | **會**：打字變寬、加行變高 |
| file picker | `filepicker.go` | **input 與 menu 同時**（見第 5 條） | 60 | 2 + 符合的項目數（上限 `H−9`） | **會**：過濾、換目錄時變高變矮 |
| finder `/` | `finder.go` | input → menu（依階段） | W ≥ 96：清單 + 預覽並排，合計 `min(W−2, W×19/20)`；否則上下疊，`min(W−2, W×9/10)−2` | `min(H−2, H×9/10)`，固定 | 不變 |
| finder `go` | `finder.go` | menu（數字當過濾，見待確認） | 56 | `max(4, min(H−6, H×3/5))`，固定 | 不變 |
| DevTools | `devtools.go` | note（帶 tab，見待確認） | `W−6` | `H−4` 列，固定 | 不變 |
| Network / Console detail | `devnetdetail.go` | note | `W−8` | `min(行數, H−6)` | **會**：body 到了從一行 `(loading body…)` 長成整份 |
| toast | `toast.go` | toast | 依訊息：`msg+4` | 一列 | — |

全部經過 `popupInnerW(W, want) = max(10, min(want, W−6))`：框內最寬 `W−6`，外框左右各留兩欄。全部 `overlay.Center` 垂直置中；
toast 在底部（`overlay.Bottom`，往上 2 列）。

不在表裡的：頁面自己的彈窗（`pagepopup.go`，F1 明寫不屬於六類、是 webu 的偏離）、splash（S 章）、Bookmarks / History /
Downloads / Settings 四個 screen（`listpanel.go`，是畫面不是 popup）。

---

## 1. 每個 popup 的寬度依內容各自算 —— F7、D4

- **現況**：每個 `view()` 自己算想要的寬度再丟進 `popupInnerW(screenW, want)`（`popup.go`），上限 `W−6`（外框 `W−4`，左右各留兩欄）；
  沒有 120 的上限。各 popup 的算法見上表：menu、help、confirm、message、toast 依內容；input 在 `ask()` 用 `openWidth()` 定一次
  （`95af693`）；editor 每次 `view()` 依最長一行重算；file picker 60、finder `go` 56；finder `/` 依畫面比例；DevTools `W−6`、detail `W−8`。
  `helppopup.go` `view()` 的註解還寫著「As wide as the longest line needs (tdp D4)」。
- **規則**：F7 寬度 `min(terminal 寬 − 2, 120)`，左右各留一欄、最寬 120 欄、水平置中；toast 同一條規則。D4：popup 寬度統一照 F7，
  說明太長就在框裡換行或截尾，不為它加寬框。
- **怎麼改**：
  - `popup.go` 換成一個寬度函式（例：`popupOuterW(W) = min(W−2, 120)`，框內再減 2），每個 popup 的 `view()` 都用它，拿掉各自的 `want`。
    `popupInnerW` 沒人用就刪。
  - **input**：`openWidth()` 與 `width` 欄位不再需要（F7 的單一寬度本來就不隨狀態變，比 `95af693`「打開時定一次」更進一步）；
    值打得比框長照舊捲動、尾端在畫面上。`view()` 的 `max(w, dispW(m.refused)+3)` 一起拿掉（錯誤列見第 4 條）。
  - **editor**：拿掉 `max(60, longest+6, …)`，長行照現在的做法在框裡橫向滑動。
  - **finder `/`**：清單 + 預覽合起來是一個 popup（F1：預覽不取得 focus），合計寬度照 F7；並排 / 上下疊的門檻（96 欄）可保留，但並排時
    兩框合計不超過 `min(W−2, 120)`。`go` 用同一個寬度。
  - **DevTools、detail**：照 F7（寬螢幕上不再是整個畫面寬）。Network 欄位的分配改從新的寬度算。
  - **menu**：label 靠左、說明靠右（D4），框變寬後說明離 label 遠是預期的；說明太長照現在的做法讓說明讓位（截尾）。
  - **message 的格內容**：`app.go` 折行寬 `min(72, max(20, m.w-12))` 改成從新的框內寬算。
  - **toast**：寬度照 F7，訊息在框裡靠左（或置中，app 決定），位置維持底部。
  - `helppopup.go` 的 D4 註解改寫。
  - 測試：`TestInputGroupLegendFits` 改寫（見「先看」）；新增一個跨尺寸（例：80、100、200 欄）打開每一種 popup、量 `ansi.Strip(View())`
    裡框的寬度 = `min(W−2, 120)` 且水平置中的測試。

## 2. 開著時高度會跟內容伸縮 —— F7

- **現況**：
  - **input**（`inputpopup.go` `view()`）：`m.refused != ""` 時多 `append` 一列空白與一列錯誤，框變高 2 列（錯誤列見第 4 條）。
  - **editor**（`editorpopup.go` `visible()`）：`max(3, min(len(e.lines), e.screenH-10))`，每加一行框就長一列，直到上限。
  - **file picker**（`filepicker.go` `view()`）：內容是查詢列 + 分隔線 + `m.matches` 的列，打字過濾、`Enter` 進目錄、`Bksp` 回上層都會讓框
    變高變矮（`visible()` 只是上限 `H−9`）。
  - **Network detail**（`devnetdetail.go`）：`show()` 時 `lines` 只有 headers 加一行 `(loading body…)`，`setBody()` 收到 `bodyMsg` 後
    `lines` 重建，`view()` 取 `min(len(lines), H−6)` 列，框在打開後長高。
  - 其他 popup 的內容在打開時就定了（menu 的 `setItems`、help 的 `entries`、confirm / message 的 `lines`），高度實際上不變；finder、
    DevTools 本來就用固定高度。
- **規則**：F7 高度依內容、**打開時定好**，之後不跟著內容伸縮；上限是畫面高度扣掉上下留白，超過就在框裡捲動。
- **怎麼改**：
  - 共用規則：打開時算一次內容列數、夾在上限內，存成 popup 的 `rows`；`view()` 內容不足就補空白列、超過就捲動。
    `capRows` 可留作上限的計算。
  - **editor**：打開時依初始行數定高（例：`max(3, min(行數, 上限))`）；之後加行就在框裡捲動（`follow()` 已經會讓游標那行留在窗裡）。
    要不要直接給 editor 一個固定的較大高度（它是「多行」框），由 app 決定，但定了就不變。
  - **file picker**：打開時定高（例：上限高度或當下目錄的項目數），過濾與換目錄只改內容、補空白列；「no match」照舊畫在第一列。
  - **Network detail**：打開時就用上限高度（body 一定比 headers 長的機率高），或 body 到了之後內容在已定的框裡捲動、框不長高。
  - 測試：每個會變的 popup 各一個「打開 → 讓內容變多 / 變少 → 框的列數不變」的測試（量 `View()` 的行數或框的上下框位置）。
    mutation：把 `view()` 改回用 `len(lines)` 算，測試要紅。

## 3. popup 開著時底下沒有任何 dim —— F8、T2

- **現況**：`app.go` `View()` 先畫 base（header、header rule、panel、footer），再依固定順序把每個 `isActive()` 的 popup 用
  `overlay.Composite` 疊上去：頁面彈窗 → `spaceMenu` → `globalMenu` → `devtools` → `options` → `message` → `finder` → `confirm` →
  `picker` → `input` → `editor` → `help` → `quitAsk` → `quitHelp` → `toast`。沒有任何一層被 dim：Space menu 開在 global operation popup
  底下時兩個一樣亮，底下的 base 畫面照原色（包括 header 的下載進度 `liveColor`、頁面的紅色錯誤、DevTools 裡持續更新的 log）。
  DevTools 的 detail 在 `devtools.view()` 裡用 `m.detail.over(out)` 疊上去，DevTools 本體也不 dim。
  唯一的「dim」是頁面彈窗自己的 backdrop（`pagepanel.go`：彈窗開著時 `[2]` 用 `t.backdrop()` 畫成 dim），那是頁面內容的一部分，跟 F8 無關。
- **規則**：F8 有 popup 開著時，最上層那個 popup 以外的一切 —— 底下的 popup 與整個 base 畫面 —— 都用 dim 色畫；永遠只有最上層是亮的。
  串流內容與警示色也一起 dim（T2 的例外）；toast 不觸發 dim（不是一層）；popup 邊框依層數的顏色照舊（見待確認）。
- **怎麼改**：
  - **一份順序**：現在繪製順序與 `closeTop`（`app.go`）的順序不一致 ——
    `closeTop` 由上而下是 `toast、quitHelp、quitAsk、help、input、editor、picker、finder、confirm、options、devtools、message、globalMenu、spaceMenu`；
    繪製由上而下是 `toast、quitHelp、quitAsk、help、editor、input、picker、confirm、finder、message、options、devtools、globalMenu、spaceMenu`
    （`input` / `editor`、`finder` / `confirm`、`message` / `options` / `devtools` 三處對調）。F8 的「最上層」必須跟 `closeTop` 關掉的那個
    是同一個，先確認這幾對能不能同時開著；能的就把兩份順序統一成一份（例：一個由下而上的 popup 清單，繪製、`closeTop`、dim 都讀它）。
  - **合成器**：`View()` 找出最上層的非 toast popup；在疊它之前，把目前為止合成出來的整張畫面（base + 頁面彈窗 + 下層 popup）轉成 dim
    （例：一個 ANSI-aware 的函式，拿掉 SGR、每個字元用 `dimColor` 重畫，版面不動），再疊最上層，最後疊 toast（toast 不 dim、也不讓別人 dim）。
    DevTools 的 detail 開著時，detail 是最上層、DevTools 本體要 dim —— 合成器要能看進 `devtools.view()`（例：`devtools` 回傳本體與 detail 兩張，
    由 `View()` 疊），不要在 `devtools.view()` 裡各自處理。
  - 關閉動畫中的 popup（`isActive()` 但不 `owns()`）：它還畫在畫面上，但已經不是最上層（F3）。誰算亮要跟 `closeTop` 用同一個判斷
    （`owns()`），不然關閉動畫的那 128ms 裡亮暗會跳兩次；決定後寫測試守住。
  - 開啟動畫中的 popup 是最上層（它已經 `owns()`）。
  - 串流：DevTools 在 eval 的 input 底下時 log 照樣在更新，要 dim；header 的下載進度、頁面在 popup 底下重畫（settle、載入）也要 dim。
  - `chrome.go` `tabRow` 的註解（「information arriving is not dimmed (tdp T2)」）補一句 popup 蓋上時的例外。
  - 測試：打開 Space menu → base 的每一列都是 dim 色、menu 是亮的；再開 global operation popup → Space menu 也變 dim；開 toast → 亮暗不變；
    `Esc` 一層 → 下一層變回亮的。串流：下載中開 popup，header 的進度列是 dim 色。mutation：拿掉 dim 那一步，測試要紅。

## 4. input 沒有預留錯誤列；送出失敗的錯誤大多丟到 toast —— F7、K3

- **現況**：
  - `inputpopup.go` `view()`：只有 `m.refused != ""` 時才 `append` 空白列 + 錯誤列，框變高 2 列、並用 `max(w, dispW(m.refused)+3)` 變寬。
  - 用到 `refuse()` 的只有加書籤（`bookmarks.go` `bookmarkGiven()`：「a bookmark needs a URL」）。
  - 其他送出失敗、框留著的，錯誤寫在 **toast**：
    - `app.go` `inputKey` 的 `inputFill`：不合形狀 `toast("wants YYYY-MM-DD")`（`ux.md` §2 的表也這樣寫）；
    - `settings.go` `saveSetting()`：`s.set` 失敗 `toast(key+": "+err)`，框留著；
    - `bookmarks.go` `importBookmarks()`：「a folder name is needed…」「folder … exists; pick another name」；
    - `bookmarks.go` `renameGiven()`：「a name is needed」「a name, not a path…」「folder … exists; pick another name」。
  - `inputFolder`：`inputKey` 先 `m.input.close()` 再 `addFolder()`，「already a folder」時框已經關了、打的字沒了（K3：不合格就不送出、框留著）。
- **規則**：F7 input popup 打開時高度就含一列錯誤列，沒有錯誤時空白；送出失敗時錯誤寫在這一列，框的高度不變。K3：不合格就不送出，
  focus 跳到第一個不合格的欄位，錯誤（哪個欄位、為什麼）寫在預留的錯誤列。
- **怎麼改**：
  - `inputPopup.view()` 固定在最後（下框之上）畫一列錯誤列：沒有錯誤時空白，有就用 `warnColor` 寫 `m.refused`（太長截尾，不加寬框）；
    高度從打開就包含它（跟第 2 條的「打開時定高」一起做）。
  - 上面列出的每個 toast 改成 `m.input.refuse(i, why)`，框留著、focus 在出錯的欄位；單欄框就是第 0 欄。
  - `inputFolder`：先檢查（重名、空）再決定關框，失敗時 `refuse`；成功才 `close()`。
  - editor 要不要也預留一列：見待確認。
  - 測試：`app_test.go` 裡等 `toast.isActive()` 當作「被拒絕」的（設定的 `measure` 填 `wide`、匯入名稱重複）改成斷言錯誤列有字、toast 沒開、
    框的高度與送出前相同；加書籤那個（`app_test.go` 約 430 行）補「框的高度不變」。`ux.md` §2 表格與 §2.1 的說法一起改。

## 6. finder 的 `Tab` 不切換、清單上的 `Esc` 退回打字 —— F1（v0.1.9）

- **現況**：`finder.go`：打字階段（`finderInput`）`Enter` 把 keyboard 交給清單（`finderNav`）；清單上 `Esc` 回到打字（`escape()` 分層，
  `app.go` `closeTop` 的 finder 分支），打字階段 `Esc` 才關掉。`Tab` 在 finder 裡沒有作用。
- **規則**：v0.1.9 F1：finder 的 `Tab` 在打字與結果清單之間切換 focus；`Esc` 關掉整個 finder —— 階段不是一層（K4）。
- **怎麼改**：`Tab`（打字與清單兩邊都是）切換 `finderInput` / `finderNav`；`Esc` 不論在哪個階段都關掉 finder（`escape()` 不再分層）。
  打字階段的 `Enter` 維持「把 keyboard 交給清單」還是改成直接去第一筆，由 app 定（K3：`Enter` 做最自然的事），寫進下框 hint 與 `?`。
  下框 hint（`finder.go` 的 `{"Esc", "query"}`）、`helpFinder`（`?`）、`ux.md` §1.1 與 `ui.md` §3 的 finder 說明一起改。`finder_test.go`
  守「清單上 `Esc` 回打字」的測試改寫成新規則。`go`（跳行）沒有打字階段，只照 `Esc` 關掉。

---

## tdp v0.1.9 定案（2026-09-28，回答本檔與其他 app 共同的待確認）

v0.1.9 只補了 v0.1.8 popup 規則的細節。本檔的條目與「待確認」照下面改讀；修的時候以這裡為準。

- **F8 邊框**：底下那幾層 popup 的**邊框也一起 dim，但保留層色** —— 畫成它自己層色（D2）的 dim 版本，不是統一的 dim 色。
  內容照 F8 用 dim 色。只有最上層是亮的。
- **F1 input 附候選清單**：邊打字邊篩選的清單仍算 input：可列印的鍵一律是字元（`j`、`k` 也是），只有方向鍵在候選之間移動，
  `Enter` 送出選中的那一筆。不必拆成兩階段。
- **F1 finder**：`Tab` 在打字與結果清單之間切換 focus；`Esc` 關掉整個 finder（階段不是一層）。
- **F1 多步驟**：流程的每一步是自己的 popup，疊起來（F4 保留 source），不在同一個框裡換內容；每一步有自己打開時定好的高度。
- **F7 錯誤列**：只有**送出可能失敗**的 input 預留錯誤列；送出不會失敗的（例：多行編輯器）不必。
- **F7 terminal 類例外**：PTY popup 寬高用滿可用範圍（terminal 寬 − 2 × 高 − 2），不受 120 欄上限。

- **本檔**：待確認「F8 邊框怎麼讀」照上面定案；file picker（原第 5 條）、finder `/` 打字時 ↑↓ 移動，照「input 附候選清單」判斷：
  方向鍵移動、`j`/`k` 是字元即符合，不必拆階段；editor 送出不會失敗，不預留錯誤列。DevTools、頁面彈窗那兩題仍待決定。

## 已定案（2026-09-28）

- **F8 邊框**：v0.1.9 定案 —— 下層 popup 的邊框也 dim，畫成自己層色的 dim 版本（第 3 條照這個做）。
- **editor 不預留錯誤列**：v0.1.9 定案 —— 送出不會失敗的 input 不必（第 4 條不動 editor）。
- **file picker 是 input 附候選清單**：v0.1.9 定案 —— 字母一律過濾（字元）、`↑` `↓` 移動、`Enter` 選，符合 F1，原第 5 條刪除。
  finder `/` 打字階段的 `↑` `↓` 移動預選，同理符合。
- **DevTools 是 menu**（user 裁定）：Network、Console 的列上 `Enter` 開 detail、Storage 的列上 `x` / `y` 執行，有可執行的列就是 menu；
  分頁切換（`h` `l`）與 `/` 過濾是 menu 的熱鍵與模式。`ui.md` §3 的類別欄照這個寫，不寫偏離。
- **finder `go` 是 menu**（user 裁定）：一打開 keyboard 就在清單上，`j` `k` 移動、`Enter` 去那一行；數字過濾是 menu 的熱鍵。它沒有打字
  階段，不是「input 附候選清單」。
- **頁面彈窗補寫 F7、F8**（user 裁定）：`dev-remarks.md`「偏離 tdp」那條已補 —— 它不屬於六類，寬度、疊層、backdrop 是頁面內容的畫法，
  不照 F7、F8；webu 的 popup 開在它上面時，它跟整個畫面一起照 F8 dim。

## 已經符合、不用修的（對照 v0.1.8）

- **F1 各 popup 的類別**：Space menu、global operation popup、options 是 menu（`j` `k`、`Enter`、熱鍵）；confirm 與 `quitAsk` 是 confirm
  （`Enter` 接受、`Esc` 取消，下框 `Enter <動詞> · Esc cancel`）；input / input group 是 input；editor 是多行 input，寫 / 移兩態屬於 K3、K8
  的多行文字規則；`?`（`help`、`quitHelp`）與 message（Inspect、格內容、code 全文）是 note；visual mode 的 cheatsheet（message 的 `passKeys`）
  是帶熱鍵的 note（按列出的鍵就關掉並執行，沒有可用 `Enter` 執行的清單，F1 允許 note 有自己的熱鍵）；Network / Console detail 是 note；
  toast 是 toast（`Esc` 或時間到收掉）。
- **finder `/` 依階段換類別**：打字時是 input（`finderInput`，`typing()` 為 true），`Enter` 把 keyboard 交給清單（`finderNav`，menu），`Esc` 回到
  打字；旁邊的預覽不取得 focus，跟清單算同一個 popup —— 正是 F1 的例子。
- **terminal 類**：webu 沒有跑在框裡的子程序，不適用。
- **F7 高度打開時就定、之後不變的**：menu（`setItems` 只在打開時呼叫）、`?`、confirm、message 的內容在打開時就定了；finder 與 DevTools
  用固定高度（DevTools 的過濾列出現時從清單扣一列，框不變）。上限都是畫面高度扣留白（`capRows`、各自的 `visible()`），超過在框裡捲動。
- **F7 位置**：所有 popup 用 `overlay.Center` 垂直置中；toast 固定在畫面底部（`overlay.Bottom`）。
- **F8 toast 不觸發 dim**：toast 最後才疊、不改變別人的畫法（加上 dim 合成器後要維持，見第 3 條）。
- **頁面彈窗不觸發 F8**：它是 `[2]` 的內容（F1、`dev-remarks.md` 偏離），開著時只有 `[2]` 用 backdrop dim，`[1]`、header、footer 照常 ——
  這個行為加了第 3 條的合成器後要維持（合成器只看 webu 自己的 popup）。反過來，webu 的 popup 開在頁面彈窗上時，第 3 條的合成器會把頁面彈窗
  跟 base 一起 dim（現在它是亮的，歸在第 3 條）。
- **T2**：webu 不做失焦變暗（panel 失焦只換邊框，D2 第 6 點），T2 的本體不適用；popup 蓋上時的例外歸在第 3 條。
- **K3 的其他部分**：`Enter` 一律送出整組（`inputPopup.update()`），加書籤空 URL 時 focus 回到 URL 欄、框留著；`Enter` 不代替 `Tab` 換欄；
  editor 寫入態的 `Enter` 是換行、移動態的 `Enter` 才設值。只差錯誤寫的位置（第 4 條）。
- **D4 的其他部分**：menu 列的 `[k]label` 左、說明右且 dim、熱鍵括號規則、menu 內的鍵與下框 hint、標題 `[N] label`，v0.1.7 已對齊，這版沒改。
- 連結：README 兩份、`dev-remarks.md`、`ui.md`、`ux.md` 開頭的 tdp 連結改釘 `v0.1.9`。
