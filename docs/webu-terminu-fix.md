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
