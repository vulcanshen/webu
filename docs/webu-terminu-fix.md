# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.6/principle)（tdp v0.1.6）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/ux.md`、`docs/ui.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-27（對照 tdp v0.1.6）。v0.1.6 只改了 K2 一條。

---

## 1. 單一輸入框的 `Tab` 不接受提議 —— K2

- **現況**：`internal/ui/inputpopup.go` `update()` 的 `case tea.KeyTab, tea.KeyShiftTab:` 只在多欄位（`len(m.more)+1 > 1`）時換欄，
  單一欄位時什麼都不做；灰字提議一律由 `case tea.KeyRight:`（`→`）接受。所以網址列（`L`）等單一輸入框按 `Tab` 沒反應。
  這是第二輪「`Tab` 只有一個意思」的裁定，當時 K2 寫的是「app **可以**用 `Tab` 接受提議」，不算違反。
- **規則**：tdp v0.1.6 K2 改成**單一輸入框**有灰字提議時，`Tab` **接受提議**；input group 裡 `Tab` 只換欄位，提議由 app 另外
  指定的鍵接受（`→` 可以留著）。（user 2026-09-27：「tab completion 只在單一 input 的時候有效」。）
- **怎麼改**：
  - `KeyTab`：單一欄位時，`value` 空著且有 `offer` 就接受（跟 `KeyRight` 同一段）；多欄位時照舊換欄。`KeyShiftTab` 單一欄位時不作用。
  - `KeyRight` 接受提議保留：input group 裡要靠它（加書籤、登入）。
  - 下框 hint 照焦點欄位：單一輸入框 `Tab accept`（`→` 也能接受，但 hint 只露 `Tab`，一個動作一個鍵）；input group
    `Tab next field` 與 `→ accept`。輸入框是輸入狀態，`?` 是字元（K8），沒有 key reference 要改。
  - **hint 的用語**（2026-09-27，user 定）：接受提議寫 `accept`（原本 `edit it`，說的是接下之後能做的事、不是這個鍵做的事；
    tdp K2 的用語就是 accepts the suggestion），拒絕提議的 `Bksp` 寫 `decline`（原本 `clear`，看起來像清掉已打的字）。
  - `docs/ux.md` 講「`→` 接受提議」的段落、README 兩份一起改。
  - 測試：改寫守「單欄 `Tab` 不作用」的測試（改名、反轉斷言），補「單欄 `Tab` 接受提議」「group 裡 `Tab` 換欄不接受提議」兩個，
    並逐處 mutation 確認會紅。
- 連結：README 兩份、`dev-remarks.md`、`ui.md`、`ux.md` 開頭的 tdp 連結改釘 `v0.1.6`（本檔已是）。

修完不發版：等家族全部 app 與 tdp 都穩定後一起發（CHANGELOG 記在 `[Unreleased]`）。
