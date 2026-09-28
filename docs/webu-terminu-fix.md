# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.7/principle)（tdp v0.1.7）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與描述該行為的設計文件段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-28（對照 tdp v0.1.7）。v0.1.7 只改了 M2 一條。

---

## 1. Space menu 的 global 列上面還掛著 `global operation` 標題 —— M2

- **現況**：`internal/ui/app.go` `withGlobal()` 在 `menuItem{separator: true}` 之後加 `menuItem{header: true, label: "global operation"}`，
  再加 `Global operation` 那一列。
- **規則**：tdp v0.1.7 M2：Space menu 的 global 那一列**不加區塊標題** —— `global operation` 標題底下只有一列
  `Global operation`，是同一句話講兩次（user 2026-09-28：「一個 global operation 的 section 只有一個 Global operation 的項目」
  太奇怪）。它跟上面的區塊之間照樣用分隔線隔開；item 與 panel 兩區在 panel 的 Space menu 上照舊一律加標題（即使只剩其中一區）。
- **怎麼改**：`withGlobal()` 拿掉 `header: true, label: "global operation"` 那一列，保留分隔線與 `Global operation`。
  「原本扁平的 menu 補上 `panel operation` 標題」那段照舊（item / panel 標題規則沒變）。量形狀的測試（`keys_test.go` 裡
  數 header 或斷言 `global operation` 標題的）改寫；`?` key reference 從 Space menu 產生時若靠 header 分段，確認不受影響。
  `docs/ux.md`、README 兩份若有畫出 Space menu 的例子一起改。
- 連結：README 兩份、`dev-remarks.md`、`ui.md`、`ux.md` 開頭的 tdp 連結已改釘 `v0.1.7`（未 commit）。

修完不發版：等家族全部 app 與 tdp 都穩定後一起發（CHANGELOG 記在 `[Unreleased]`）。
