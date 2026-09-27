# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.4/principle)（tdp v0.1.4）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）、`docs/ux.md`、`docs/ui.md` 與 `docs/dev-remarks.md` 裡描述該行為的段落。
有意不修的，改寫成 `dev-remarks.md`「偏離 tdp」的一條並附理由。

webu 照 tdp v0.1.0 修完、發布 v0.4.0（2026-09-27）；這份只列 tdp v0.1.1–v0.1.4 帶出來的差距。

盤點日期：2026-09-27（對照 tdp v0.1.4）。行號以當天的 `main` 為準。


## 先看：locku、webu、sshu 修完的經驗（2026-09-27 更新，含 locku 對照 v0.1.4）

locku（對照到 v0.1.4）、webu（v0.1.0 那一輪）與 sshu（對照到 v0.1.4）已照 tdp 修完，修的時候發現這些，這裡的各條也適用：

- **每修一條補一個 model test，並逐處 mutation**：把每一個修正單獨改回舊行為，確認對應的測試會紅（新測試用到新欄位時，
  「拿舊程式碼整個跑一次」會編譯不過，所以改成逐處改回）。mutation 沒被抓到，先看是不是用被改的函式量了自己、或改在走不到的分支上。
  測試用不接瀏覽器的 model：`New(nil, "")` 加暫存的 `WEBU_CONFIG` / `WEBU_DATA`（`keys_test.go` 的 `keysDriver`）。
- **守舊規則的測試要改寫，不是刪掉。** 改名並反轉斷言，測試名稱寫出新規則。本檔第 1、2 條會讓 `keys_test.go` 的
  `TestQuestionMarkMenu`、`TestSpaceMenuEndsInGlobal`，以及用 `?` 打開 `?` menu 的幾個測試（`TestSpaceClosesOnlyTheSpaceMenu`、
  `TestQuitFromEverywhere`、`TestMenuStaysUnderWhatItOpened`）一起紅，逐一改成量新規則。
- **（sshu）離開的 confirm 用自己的 popup，不借共用的 confirm**（tdp D3）。`Ctrl-C` 在任何地方都叫得出離開流程，所以它可能
  疊在另一個 confirm 上；借用同一個會把使用者正在回答的問題蓋掉。見第 5 條。
- **（sshu）「最上面」只能有一個答案，「放在最上層」要改三個地方：按鍵路由、`closeTop`、繪製順序。** 少改一個就是一種 bug：
  路由錯了 `Enter` 會確認底下看不見的問題，`closeTop` 錯了 `Esc` 關錯層，繪製錯了畫面上看不到。繪製順序只有讀渲染出來的
  畫面（`View()`）才測得到。第 2、5 條都會動到這三處。
- **（sshu）F3 別只看 toast**：判斷「popup 還在不在」一律用 `owns()`，整個 `closeTop()` 一起看。webu 目前每一層都用 `owns()`；
  新加的 popup（第 1 條的 key reference 若另開一個、第 5 條的 quit confirm）照做。
- **（sshu）global operation 用一份清單。** 全域動作只有一個來源（webu 已是 `globalMenuItems()`），global operation popup
  只讀它；所有 Space menu 走同一個組法（webu 已是 `withGlobal`）。目前所在的畫面那一列照 M6 變暗，不拿掉（第 3 條）。
- **（sshu）panel 的 key reference 從它的 Space menu 讀。** 由 Space menu 的列產生（只收按得出來的鍵：單一字元或 `Enter`），
  再接 core key，兩邊不會不一致（第 1 條）。
- **（sshu）模式的按鍵清單不能用 `j`/`k` 移動**（tdp K11）。visual mode 的鍵正是 `h j k l u d`；webu 的 cheatsheet 沒有游標、
  不移動，每個鍵都是「執行」，符合。日後若讓它可以捲動或加游標，只用方向鍵。
- **（sshu）K3 改成一律送出時，驗證要把「有沒有填」一起問。** webu 的 input popup 都是單一欄位、`Enter` 本來就一律送出，
  目前不受影響；日後若出現多欄位的 input popup，照這條做。
- **（webu）行號很快就過期；換註解編號照內容，不照行號。** 前幾條一改，後面引用的行號全部位移。每一列在該檔的註解裡找
  「現在」那串，數量跟清單一致就直接換，不一致的逐一看。
- **（webu）改完一條就回頭檢查別的 surface 有沒有說錯的提示**：下框 hint、`?` 的說明、footer、README 的按鍵表。
- **（locku、webu）F4 在按鍵路由處理**（`handleKey` / `stackTop`）：執行 menu 的一列時，在 **dispatch 回傳的 model** 上判斷有沒有
  開出新框，不要在舊 model 上關。第 2 條讓 menu 疊兩層，`stackTop` 要分得出兩層。
- **（locku v0.1.4）fix 清單先定案再改程式。** 動手前讀完本檔，把 user 的裁定寫回清單（「待確認」改成「已定案」）先
  commit，之後才改程式；程式的 commit 跟清單分開。
- **（locku v0.1.4）離開的 confirm 自己的 `?` 要另開一個 help popup**（第 5 條）。help 可能在 quit confirm 底下（在 help 上按
  `q`），也可能在它上面（在 quit confirm 上按 `?`）；共用同一個 help 就得記「誰先開」。locku 另開 `quitHelp`，順序固定成
  `quitHelp` > `quitAsk` > `help` > 其他，路由、`closeTop`、繪製三處都照這個順序寫一次。
- **（locku v0.1.4）「有框握著鍵盤」的判斷要把 quit confirm 算進去。** 從 global operation popup 執行離開、而需要先問
  （webu：有下載進行中）時，若判斷不認 quit confirm，就會把底下的 global operation popup 與 Space menu 清掉，`Esc` 回不去
  （第 2、5 條）。
- **（locku v0.1.4）global 列執行完要清掉整疊（T1），測得到的那一拍要選對。** locku 唯一的全域動作是離開，執行完 app 就結束，
  看不出 popup 有沒有清，要斷言「送出 Quit 的那一刻整疊也一起關」。webu 有切畫面等動作，用它們量。
- **（locku v0.1.4）沒有熱鍵的列用一個按不出來的 key。** `Global operation` 列 commit 一個多字元字串（`hotkeyIndex` 配不到、
  `bracketHotkey` 不加括號），在 Space menu 的分支裡攔下來開 popup，不進 dispatch（第 2 條，列名也拿掉 `…`）。
- **（locku v0.1.4）key reference 從 Space menu 的列讀時，`[Enter] Edit` 這種 label 先去掉 `[Enter] ` 再放進說明**；panel 自己
  有 `Enter` 時，core key 那段的通用 `Enter` 列不重複（第 1 條）。
- **（locku v0.1.4）D4 依內容算寬以後，最長那一行會貼著右框**：算寬時多留一欄，也別小於下框 hint 的寬度。
- **（locku v0.1.4）新 popup 要一個不漏地接上**：動畫 tick、`WindowSizeMsg`、層數計算、測試裡把動畫跑完的目標清單。漏了一個，
  那個 popup 在測試裡會停在動畫中途、接不到鍵。
- **（locku v0.1.4）改完把畫面實際印出來看**：暫時寫一個測試 `t.Log(ansi.Strip(m.View()))`，看完就刪。貼框、疊層順序只有
  這樣看得到。
- **修完不發版。** 家族全部 app 與 tdp 都穩定下來之後才一起發；CHANGELOG 先記在 `[Unreleased]`。
- 完整紀錄：terminu repo 的 `.local/family-fix/locku/`、`.local/family-fix/webu/`、`.local/family-fix/sshu/`（本機）；
  sshu 的 `terminu-fix` 分支是 `?` 只讀、global operation popup、K3 的實作範例（`popupHelp()`、`globalActions`、`withGlobal`）。

---

## 1. panel 上的 `?` 打開可執行的 `?` menu —— K6、M4、F1

- **現況**：`internal/ui/app.go` `routeKey()` 的 `?` 分支（:867–879）：沒有 popup 時走 `openGlobalMenu()`（:1534），打開
  `globalMenu`，內容是 `helppopup.go` 的 `globalMenuItems()`（:164）—— 上半 `global operation` 十一列可以執行（`j`/`k`、`Enter`、
  熱鍵），下半 `key reference` 是 `note` 列（唯讀，`spacemenu.go` :22 的 `note`）。一個 popup 同時是 menu 又是唯讀清單。
  list screen（Bookmarks、History、Downloads、Settings）上的 `?` 也是這個 menu；panel 自己能按的鍵不在任何 `?` 裡。
  popup 上的 `?`（`floatHelp()`，`helppopup.go` :98）已經是該 popup 自己的按鍵、唯讀，符合。
- **規則**：`?` 打開**最前端那個 surface 的 key reference**：唯讀、可以捲動，沒有游標、不能執行。panel 上列出這個 panel 能按的鍵
  與 core key；popup 上只列這個 popup 的鍵（K6、M4）。能做的事在 `Space` 與 global operation popup；讀與做分成兩個框（F1）。
- **怎麼改**：
  - panel 上的 `?` 改開 `helpPopup`（跟 popup 上的 `?` 同一個 viewport），內容 = 這個 panel 的鍵 + core key。panel 的鍵從它的
    Space menu 列產生（`openMenu()` / `m.lists.menuItems()` 組出來的列，只收有鍵可按的：單一字元、`Enter`、`/`、`go` 這類寫在
    label 裡的 core key；menu-only 沒有字母的列不收），不另外手寫一份。core key 沿用 `globalMenuItems()` 下半那幾列的文字，
    `?` 那一列改成「this key reference; on a popup, its keys」之類。`[1]` Tabs、`[2]` Page、每個 list screen 各自一份。
  - `?` 再按一次關掉、`Esc` 也關（已是如此）。
  - `globalMenuItems()` 只留可執行的全域動作（見第 2 條）；`note` 列沒有人用之後，`menuItem.note` 與 `spaceMenu.view()` 裡 `it.note`
    的分支一起拿掉（`selectable()` 跟著簡化）。
  - key reference 的寬度依最長的說明計算（tdp D4）：`helpPopup.view()` 目前是 `keyW+44`（:138），長說明會被截掉。
  - 文件：README 兩份的 `?` 那一列（:89）、`ux.md` §A.0 表的 Non-contextual 列、§A.0.K 的 `?` 列、§A.2 整節、`ui.md` 浮層表的
    `?` menu 列（:161）改成「`?` 只讀；全域動作在 global operation popup」。

## 2. global operation popup 取代 Space menu，而不是疊在它上面 —— M2、M4、K4、F4、T1

- **現況**：每個 Space menu 最後一區是一列 `Global operation…`（`app.go` `withGlobal()` :1523–1531，key `globalmenu`）—— 一列符合 M2。
  但 `menuKey()` 的 `globalmenu` 分支（:1817–1823）先關掉 Space menu、再開 `globalMenu`（註解：「takes the Space menu's place rather
  than stacking on it」），所以 `Esc` 從 global 清單直接回到 panel，不是回到 Space menu。`openGlobalMenu()` 用 layer 1（:1535），
  跟 Space menu 同一層色。清單就是第 1 條的 `?` menu（混著唯讀的 key reference）。列名多了 `…`。
- **規則**：Space menu 的 global 區固定一列 `Global operation`（M2）；`Enter` 打開 global operation popup，**疊在 Space menu 上**，
  是一種 menu，列出全部全域動作、可以執行，離開在這裡（M4、K9）；`Esc` 只關最上層、回到 Space menu，底下原樣留著（K4、F4）；
  執行了會關掉整疊（T1）。
- **怎麼改**：
  - `menuKey()` 的 `globalmenu` 分支不再關 Space menu，`globalMenu` 疊上去、layer 2（`popupLayerColor`）。`closeTop()` 的順序已經
    是 `globalMenu` 先於 `spaceMenu`（:989），`Esc` 自然回到 Space menu；繪製順序也已是 `globalMenu` 在 `spaceMenu` 之上（:2857）。
  - 執行一列要清掉整疊：`globalMenuKey()`（:1543）現在只對 `globalMenu` 做 `keepSource`，改成沒開出新框時兩層 menu 一起關
    （`closeMenus()`）。`stackTop()`（:774）目前把兩個 menu 都算「1」，疊起來後要分得出 global popup 在 Space menu 之上
    （例：Space menu 1、global popup 2、options 3、動作開出的框 4），`handleKey` 的 F4 判斷才對。
  - `globalMenuItems()` 只留可執行的列（標題照 M4 是 popup 自己的，不需要 `global operation` 區塊標題；要不要保留由 app 定）。
  - `withGlobal()` 的列名改成 `Global operation`（去掉 `…`）。
  - global popup 上的 `?`（`floatHelp()` :116）：`Esc` 的說明改成「back to the Space menu」。global popup 上的 `Space` 維持不作用（K5）。
  - `?` 不再開 global popup（第 1 條）以後，`routeKey()` `?` 分支裡「`globalMenu` 開著就關掉它」那一格（:873）拿掉：global popup
    上的 `?` 跟其他 popup 一樣是它自己的按鍵。
  - 文件：`ux.md` §A.1（:62–63「`Enter` 打開 `?` menu … 偏離 tdp M2」）、§A.2、§5「從 Space menu、`?` menu 或 options 開出的框」、
    `ui.md` 浮層表，改成 global operation popup。

## 3. global operation 清單裡，目前所在畫面那一列沒有變暗 —— M4、M6

- **現況**：`helppopup.go` `globalMenuItems()`（:164）是固定的一份，`Web` / `Bookmarks` / `History` / `Downloads` / `Settings` 五列
  沒有一列帶 `disabled`；在 Bookmarks 上執行 `Bookmarks` 什麼都沒變。
- **規則**：global operation popup 裡目前所在畫面的切換列照 M6 變暗（說明維持原句，`Enter` 與熱鍵都不作用），不拿掉（M4）。
- **怎麼改**：`globalMenuItems()` 收目前的 `m.screen`，對應那一列 `disabled: true`（`screenKeys` :62 的對照）。`spaceMenu.update()`
  已經不執行 disabled 列，不用另外處理。

## 4. visual mode 的 `?` 打開 `?` menu，不是模式的 help —— K11

- **現況**：`app.go` `routeKey()` 的 `?` 分支（:867–879）排在 visual mode 的處理（:920）之前，visual mode 開著、沒有 popup 時
  `?` 走 `openGlobalMenu()`，打開整個 app 的 `?` menu。`ux.md` §A.0.K 的 visual mode 欄也寫 `` `?` menu ``。
  （visual mode 的 `Space` cheatsheet、`Esc`、`q` / `Ctrl-C`、`Tab` 的 toast、footer 的 `space` / `?` 都已符合 K11；cheatsheet
  沒有游標、不移動，每個鍵都是「執行」，也符合「模式的鍵跟導覽鍵重疊時只用方向鍵移動」。）
- **規則**：模式裡 `?` 是**這個模式的 help（唯讀）**（K11）。
- **怎麼改**：`?` 分支裡，`m.sel.on`、沒有 popup、不在打字時，打開 `helpPopup`（標題 `Visual mode`），內容是模式的鍵 —— 跟
  `selectCheatsheet`（`selectmode.go` :523）同一份來源轉成 `helpEntry`，不另外手寫。`ux.md` §A.0.K 的 visual mode `?` 欄改成
  「模式的 help（唯讀）」。

## 5. 離開的 confirm 借用共用的 confirm，蓋掉底下正在回答的 confirm —— F4、K4（tdp D3）

- **現況**：`app.go` `askQuit()`（:1264）在有下載進行中時用 `m.confirm.ask(...)`；`confirm.go` `ask()`（:55）是 `*m = c`，整個
  覆寫。`routeKey()` 的 `Ctrl-C`（:843）與 `q`（:849）只擋「已經是 quit confirm」的情況，所以另一個 confirm 開著時（刪書籤、
  清歷史、頁面的 `confirm()` / `beforeunload`、link Open……）按 `q` 或 `Ctrl-C`，那個 confirm 被 quit confirm 取代；在 quit
  confirm 上按 `Esc` 取消後，底下原本的問題不見了。頁面的 `confirm()`（`confirmDialog`）被蓋掉時，`closeTop()`（:971）看到的已經
  是 `confirmQuit`，`answerDialog` 永遠不會送出，頁面一直等著回答。
- **規則**：popup 開出 popup 時，`Esc` 只關最上層，底下的階層原樣留著（K4、F4）。tdp D3：離開的 confirm 用自己的 popup，疊在
  整疊最上面。
- **怎麼改**：另開一個 quit confirm（自己的 animator target，例 `quitAsk`），`askQuit()` 開它而不是 `m.confirm`。「放在最上層」
  三處一起改：按鍵路由（在 `help` 之後、其他 popup 之前拿鍵）、`closeTop()`、繪製順序（在 `confirm` 之上）；`floatOwned()`、
  `popupOpen()`、`closeStack()`、`layer()` 都要認得它。`Ctrl-C` / `q` 的「已經在問」判斷改看 `quitAsk.anim.owns()`。
  **quit confirm 的 `?` 另開一個 `quitHelp`**（2026-09-27 定案，照 locku v0.1.4）：help 可能在 quit confirm 底下（在 help 上按 `q`），
  也可能在它上面（在 quit confirm 上按 `?`），共用 `help` 就得記誰先開。順序固定 `quitHelp` > `quitAsk` > `help` > 其他，路由、
  `closeTop()`、繪製三處都照這個順序；`quitHelp` 的內容是 `helpConfirm`。補測試：頁面的 confirm 開著、有下載時按 `q` → `Esc` →
  頁面的 confirm 還在、答案照常送出；help 開著時按 `q` → `?` → `Esc` → `Esc` 回到 help。

## 6. 程式碼註解與測試訊息仍寫 `?` menu 與「偏離 tdp」—— 文件對齊

第 1、2 條改完後，下列註解與 `t.Error` 訊息描述的是舊規則，一起改（照內容比對，不照行號）：

| 位置 | 現在 | 改成 |
|---|---|---|
| `app.go:92` | `? on a panel: the global operations, then the core keys (tdp M4)` | global operation popup，從 Space menu 的 global 列打開（tdp M4） |
| `app.go:738` | `floatAboveGlobalMenu is the same for the ? menu` | the global operation popup |
| `app.go:867` | `? on a panel is the ? menu; on a float, that float's own keys` | `?` 是最前端 surface 的 key reference（tdp K6、M4） |
| `app.go:1518–1522` | `one row into the ? menu … (dev-remarks.md, 偏離 tdp)` | 一列 `Global operation`，打開 global operation popup（tdp M2）；不再是偏離 |
| `app.go:1533`、`:1539` | `the ? menu` | the global operation popup |
| `app.go:1818–1819` | `The ? menu takes the Space menu's place rather than stacking on it` | 疊在 Space menu 上，`Esc` 回到它（tdp M4、F4） |
| `helppopup.go:8–10` | `? on a panel is the ? menu instead (globalMenuItems)` | panel 上是 panel 的 key reference |
| `helppopup.go:160–163` | `globalMenuItems is the ? menu (tdp M4) … 偏離 tdp` | global operation popup 的列（tdp M4） |
| `spacemenu.go:20–22` | `the ? menu's key reference (tdp M4)` | 隨 `note` 一起拿掉（第 1 條） |
| `keys_test.go:118`、`:166–167`、`:320` | `? on a panel is the ? menu`、`the one row is webu's deviation`、`The ? menu's rows do the same` | 新規則 |
| `keys_test.go` 的 `t.Error` / `until` 訊息 | `the ? menu`（:49、:52、:75、:77、:125–162、:191、:193、:348–356） | global operation popup / key reference |

## 7. 加書籤、HTTP 驗證拆成兩個接連的框，應該是一個 input group —— K3

- **現況**：兩者都是一個接一個的單欄 input popup。加書籤：`bookmarks.go` `startAddBookmark()`（:291）先問 URL（`accept: "next"`），
  `bookmarkURLGiven()`（:302）關掉它再開標題框。HTTP 驗證：`app.go` 的 `authMsg`（:342）先問帳號，`inputAuthUser`（:2594）再開
  遮罩的密碼框。`Tab` 不在兩框之間移動；`Esc` 在第二個框只取消第二個。
- **定案**（2026-09-27，user）：兩組都是一個 input group —— 放進**同一個 popup**：加書籤是 URL + 標題，HTTP 驗證是帳號 + 密碼。
- **規則**：`Tab` 在欄位之間移動；`Enter` 一律送出整組；送出失敗時焦點跳到第一個不合格的欄位、說明錯在哪（K3）。
- **怎麼改**：
  - `inputPopup` 從一個值改成一組欄位（每欄：prompt、value、masked、offer），單欄的 popup 就是一欄，路由、層數、`closeTop`、
    `floatHelp` 都沿用，不另開一種 popup。
  - **`Tab` 只換欄，接下提議改用 `→`**（2026-09-27，user：「tab follow tdp」）。單欄 popup 裡 `Tab` 原本是「接下提議」
    （Location 的目前 URL、加書籤的目前頁）；改成**所有** input popup 的 `Tab` 都只在欄位之間移動（單欄時不作用），空欄位上接下
    提議的是 `→`（input popup 目前沒有游標移動，`→` 沒有別的用途）。提議仍是欄位裡的暗字，`Backspace` 拒絕它；加書籤的欄位空著送出
    就用提議（原本的行為），Location 不用（原本的行為）。加書籤的標題提議跟著 URL 欄：URL 是目前這頁就提議頁面標題，否則提議 URL。
  - 下框 hint：`Tab  next field`（多欄時）、`→  edit it`（有提議時，原本是 `Tab`）。
  - 驗證：加書籤的 URL 不可空；HTTP 驗證不驗（帳號可以是空的）。失敗時焦點移到那一欄、框裡寫出原因，不關框。
  - `Esc` 取消整組：HTTP 驗證走 `cancelAuth()`。
  - 文件：`ux.md` §2（文字輸入、§2.2 Location 的「Tab 接手」）與 `ui.md` 浮層表的 Location、input 列補上 input group 與 `→`；
    README 兩份的「`L` opens the address box … `Tab` takes it to edit」改成 `→`，Bookmarks `a` 的說明。
  - 測試：`app_test.go` 的加書籤（:431–434）與 `hooks_test.go` 的 HTTP 驗證（:164–170）改成量一個框兩欄；補送出失敗跳到 URL 欄。
