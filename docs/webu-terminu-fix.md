# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.10/principle)（tdp v0.1.10）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/ux.md`、`docs/ui.md`、`docs/dev-remarks.md` 裡描述該行為的段落。
有意不修的，改寫成 `dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-28（對照 tdp v0.1.10）。v0.1.10 只改寫了 K11 一條。

---

## 1. visual mode 按 `Space` 開出可執行的 cheatsheet —— K11（tdp v0.1.10）

- **現況**：
  - `internal/ui/app.go` 的 `if m.sel.on` 那段（約 949 行）：模式裡按 `Space` 以
    `m.message.show(glyphMenu, "Visual mode", selectCheatsheet, true, m.layer())` 打開 cheatsheet；`passKeys` 為 true，
    按表上的鍵就關掉 cheatsheet 並執行那個鍵（約 926 行 `case m.message.anim.owns():`）。
  - `Space` 路由那段（約 870 行）註解「Visual mode's cheatsheet is that mode's Space menu for now」，並讓 `Space` 關掉
    `passKeys` 的 message；`helppopup.go` 約 188 行為 cheatsheet 另備一份 help。
  - `selectmode.go`：`selectCheatsheet`（約 537 行）、`selectKeys` 的註解寫它同時餵 cheatsheet 與 `?`；`selectLegendPairs()`
    的 footer 第一組是 `{"space", "menu"}`。
  - `?` 已經是模式自己的 key reference（`selectKeys`，唯讀），這部分符合。
  - 文件：`docs/ux.md` 約 29 行表格 visual mode 欄寫「cheatsheet，按列出的鍵即執行」；`docs/dev-remarks.md` 約 210 行
    「visual mode 的 `Space` 是 cheatsheet」列為定案。
- **規則**：tdp v0.1.10 K11 改寫：模式裡 **`Space` 不開任何 menu、不作用**，沒有可以執行的按鍵清單；`?` 是這個模式的
  key reference（唯讀）；模式自己的鍵直接按，揭露在 `?` 與 footer；footer 照樣顯示 `?`，`Space` 不必列。`Esc`、
  `q` / `Ctrl-C`、`Tab`（暫停但要回應，webu 已用 toast）照舊。（user 2026-09-28：模式下不應該還有 popup menu，只有 `?` 說明。
  這也取代了 2026-09-27「visual mode 的 cheatsheet 保留」的裁定。）
- **怎麼改**：
  - 模式裡的 `Space`（非打字時）改成不作用；拿掉 `selectCheatsheet` 與 `messagePopup.passKeys` 這條路（若沒有其他使用者），
    `Space` 路由與 `helppopup.go` 裡為 cheatsheet 開的分支一起拿掉。
  - `selectLegendPairs()` 拿掉 `{"space", "menu"}`，`?` 排第一；`selectKeys` 的註解改成只餵 `?`。
  - 守 cheatsheet 的測試（按 `Space` 開 cheatsheet、按表上的鍵即執行）改寫成「模式裡 `Space` 不作用、`?` 開模式的
    key reference」，不是刪掉；逐處 mutation 確認。
  - `docs/ux.md` 表格的 visual mode 欄、`docs/dev-remarks.md` 約 210 行那條定案、README 兩份若有提到 visual mode 的 `Space`，
    一起改；dev-remarks 那條改寫成「v0.1.10 起模式裡 `Space` 不作用」並註明舊裁定被取代。
- 連結：README 兩份、`dev-remarks.md`、`ui.md`、`ux.md` 開頭的 tdp 連結已改釘 `v0.1.10`（未 commit）。

修完不發版：等家族全部 app 與 tdp 都穩定後一起發（CHANGELOG 記在 `[Unreleased]`）。
