# webu — 設計文件

終端機瀏覽器。Chromium 在背景 headless 跑，webu 取它的 accessibility tree 轉成自己的
IR（Intermediate Representation，中間表示法），以 TUI 呈現；操作經 CDP（Chrome DevTools
Protocol）打回 Chromium。一句話：**把 screen reader 的輸出畫成 TUI，而不是唸出來。**

u-family 成員（kbu / filu / sshu 之後），依 VTP（`thoughts/tui-design`）設計。

## 三份文件

| 檔 | 回答的問題 | 讀的順序 |
|---|---|---|
| [`docs/function.md`](docs/function.md) | 哪些事 Chromium 做、哪些事 webu 做、做到什麼程度；翻譯層（AX tree → IR）的 role 白名單與 fallback；Chromium 的取得與執行模式；shell 功能清單 | 1 |
| [`docs/ui.md`](docs/ui.md) | 版面、header 的 screen 與兩個面板的職責、popup、DevTools popup、色帶、存檔、每項功能落到哪個 surface 哪一版 | 2 |
| [`docs/ux.md`](docs/ux.md) | core-key 語意、兩種模式（一般 / 選取）、文字輸入、每個 focus 的 Space menu、hotkey 分層全表、`?` 內容、浮層行為、時間軸、goto popup | 3 |

三份都以 2026-09-20 的討論定案，決定處標日期；擱置與 v2 項目在各自的待決 / 擱置段落，
不散落在正文。

## 定案摘要

- **架構**：自帶釘死版本的 Chromium、`--headless=new` 無視窗、webu 自己的持久 profile；不 attach 使用者的 Chrome、不開放版本覆寫
- **翻譯層**：以 AX role 白名單為單位保證，未支援 role fallback 成純文字、不隱藏、Enter 仍可 click；每 role 一份 fixture，站級只做 smoke
- **多媒體**：只畫佔位框；ASCII 轉換 / 內建 viewer / 外部工具全部擱置
- **CAPTCHA**：第一版不支援，明講；交棒機制擱置
- **版面**：最上列 header `[W]eb [B]ookmarks [H]istory [D]ownloads [S]ettings`（sshu 式 screen chip 列，右端數進行中的下載；`[W]eb` 之外的四個各佔滿整個 body）、`[W]eb` 是側欄 24 欄固定 `[1]` Tabs（cursor 反白 vs 綠字 = `[2]` 正在顯示）、`[2]` 永遠是頁面、第一列 URL
- **語意**：Enter = 該 item 的 item operation 選單（第一列是主要動作，再 Enter 執行）、Space = 完整選單（item + panel）、Esc 只做取消 / 關閉；`P` / `N` 前後頁；選取模式（字元游標、Yellow 邊框）由 `/` 搜尋或 Space menu 的 Select text 進入（修訂 2026-09-20）
- **文字輸入**：所有 textbox Enter 開 input popup（Enter 確認 / Tab no-op / Esc 取消）；有值的 textbox Enter 開選單（Submit / Edit / Clear / Yank）
- **hotkey**：小寫 = item、大寫 = panel 或全域；全域 `W B H D S P N L`；`[2]` item operation 無 letter hotkey、menu-only

## 還開著的

| 項目 | 去處 |
|---|---|
| link 色帶 | `ui.md` §4，畫出來再挑 |
| `[2]` item operation 的 letter hotkey | `ux.md` §8，用了再說 |

## 目錄

```
README.md                    本檔
docs/function.md             功能邊界
docs/ui.md                   版面與 surface
docs/ux.md                   互動語意
docs/webu-implementation.md  實作怎麼落地、實測出來的決定、做到哪
docs/support.md              支援的 AX role（由 internal/ir/roles.go 產生）
cmd/webu/                    進入點
internal/browser/            Chromium 下載、profile、啟動、關閉
internal/ir/                 AX tree → IR，每個 role 一份 fixture
internal/page/               CDP 端：擷取與動作
internal/ui/                 TUI
tools/axdump/                看任何頁面的 AX tree（用本機 Chrome）
```

## 開發

技術棧：Go + Bubble Tea + Lipgloss + bubbletea-overlay + chromedp，同 u-family。

```
make build              → ./webu；首次啟動會下載釘死版本的 Chromium（約 175–250 MB）到 cache 目錄
make test               所有測試；有下載過 Chromium 才會跑整合測試，否則 skip
make fixtures           用釘死的 Chromium 重抓 internal/ir 的 role fixture 與 docs/support.md
WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v     真站 smoke（Hacker News、GitHub）
make axdump URL=https://…                                任何頁面的 AX tree
```

第一個 milestone（`docs/function.md` §11）已達：v1 role 白名單每個 role 一份 fixture 通過，
本機頁面能點、能填表、能選 option，Hacker News 與 GitHub 畫得出來。做到哪、沒做哪，見
`docs/webu-implementation.md` §9。
