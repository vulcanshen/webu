# webu — 設計文件

終端機瀏覽器。Chromium 在背景 headless 跑，webu 取它的 accessibility tree 轉成自己的
IR（Intermediate Representation，中間表示法），以 TUI 呈現；操作經 CDP（Chrome DevTools
Protocol）打回 Chromium。一句話：**把 screen reader 的輸出畫成 TUI，而不是唸出來。**

u-family 成員（kbu / filu / sshu 之後），依 VTP（`thoughts/tui-design`）設計。

## 三份文件

| 檔 | 回答的問題 | 讀的順序 |
|---|---|---|
| [`docs/function.md`](docs/function.md) | 哪些事 Chromium 做、哪些事 webu 做、做到什麼程度；翻譯層（AX tree → IR）的 role 白名單與 fallback；Chromium 的取得與執行模式；shell 功能清單 | 1 |
| [`docs/ui.md`](docs/ui.md) | 版面、三個面板的職責、五個 popup、DevTools popup、色帶、存檔、每項功能落到哪個 surface 哪一版 | 2 |
| [`docs/ux.md`](docs/ux.md) | core-key 語意、兩種模式（一般 / 選取）、文字輸入、每個 focus 的 Space menu、hotkey 分層全表、`?` 內容、浮層行為、時間軸、goto popup | 3 |

三份都以 2026-09-20 的討論定案，決定處標日期；擱置與 v2 項目在各自的待決 / 擱置段落，
不散落在正文。

## 定案摘要

- **架構**：自帶釘死版本的 Chromium、`--headless=new` 無視窗、webu 自己的持久 profile；不 attach 使用者的 Chrome、不開放版本覆寫
- **翻譯層**：以 AX role 白名單為單位保證，未支援 role fallback 成純文字、不隱藏、Enter 仍可 click；每 role 一份 fixture，站級只做 smoke
- **多媒體**：只畫佔位框；ASCII 轉換 / 內建 viewer / 外部工具全部擱置
- **CAPTCHA**：第一版不支援，明講；交棒機制擱置
- **版面**：側欄 28 欄固定，`[1]` 三項（Bookmarks / Shortcuts / History，Enter 開 popup）、`[2]` Tabs（cursor 反白 vs 綠字 = `[3]` 正在顯示）、`[3]` 永遠是頁面、第一列 URL
- **語意**：Enter = 滑鼠左鍵、Space = 右鍵選單、Esc 只做取消 / 關閉；`P` / `N` 前後頁；`Alt+v` 選取模式（字元游標、Yellow 邊框、`/` 搜尋自動進入）
- **文字輸入**：所有 textbox Enter 開 input popup（Enter 確認 / Tab no-op / Esc 取消）；有值的 textbox Enter 開選單（Submit / Edit / Clear / Yank）
- **hotkey**：小寫 = item、大寫 = panel 或全域；全域 `B S H P N`；`[3]` item operation 無 letter hotkey、menu-only

## 還開著的

| 項目 | 去處 |
|---|---|
| link 色帶 | `ui.md` §4，畫出來再挑 |
| `[3]` item operation 的 letter hotkey | `ux.md` §8，用了再說 |

## 目錄

```
README.md            本檔
docs/function.md     功能邊界
docs/ui.md           版面與 surface
docs/ux.md           互動語意
tools/axdump/        AX tree 驗證程式（chromedp），docs/function.md §3 實測數據的來源
```

## 下一步：開發

- 技術棧：Go + Bubble Tea + Lipgloss + bubbletea-overlay + chromedp，同 u-family；尚未 `git init`、尚未 `go mod init`
- 之後對照 kbu / filu / sshu 的慣例補 `docs/webu-implementation.md`
- 第一個 milestone：`docs/function.md` §11 —— v1 role 白名單每個 role 一份 fixture 通過，Hacker News smoke 能登入、能點、能填表
- Chromium 下載器與 profile 目錄先於任何 UI；`[3]` 頁面渲染先於側欄與 popup
- `tools/axdump`：`cd tools/axdump && go run . <url>` 看任何頁面的 AX tree
