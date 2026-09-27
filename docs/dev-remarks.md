# webu 開發者備忘

README 只介紹這個工具怎麼用；這份收的是屬於開發者的部分：webu 裡面怎麼運作、設計為什麼這樣定、文件怎麼讀、怎麼建置與發布。webu 遵循 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.0/principle)（tdp），是 terminu family 在瀏覽器領域的成員。

---

## 運作方式

真正的 Chromium 在背景 headless 跑；webu 取它為 screen reader 維護的 **accessibility tree**，加上同一棵 DOM 的 **版面快照**（DOM snapshot），靠 `backendNodeId` 對接，合成一份能翻的文件。每個操作都經 Chrome DevTools Protocol 打回 Chromium，所以頁面就是真的頁面。

- **語意來自 accessibility tree，版面來自 DOM** —— screen reader 讀到的就是你看到的：role、名字、狀態；東西在頁面的哪裡來自版面快照。不讀 CSS 的顏色字型。webu 不認得的 role 畫成文字加標記，從不隱藏、仍然可點。
- **每個支援的 role 都有 fixture**，對著釘死的 Chromium revision 抓，所以引擎升版是一個決定而不是漂移。支援清單在 [`support.md`](support.md)，由 role 表產生。
- **一個釘死的 Chromium** —— revision 編進執行檔、不開放覆寫、永遠不碰使用者自己的 Chrome；webu 自己的持久 profile。頁面的問題都是 Chromium 的問題，webu 只畫答案。
- **兩個選單、一張表** —— `Enter` 是 item 的操作，`Space` 是 item 加 panel；每個括號裡的字母由同一張表產生、key handler 也讀同一張，所以不在選單裡的熱鍵不可能存在。
- **畫面穩定** —— 任何尺寸、任何內容，每一列都恰好是終端機的寬；有測試跨尺寸、面板、screen 檢查。
- **`[S]ettings` 與 `config.yaml` 同步** —— 檔案裡有的 key，Settings 一定有一列，有測試守著。
- **存檔** —— 書籤每筆帶 `folder` 路徑，另有 `folders:` 清單讓空目錄留得住；歷史是一次 append 一筆的 YAML sequence；每次寫檔都是原子的。分法：使用者寫的在 config、webu 產生的在 data、可重抓的在 cache。
- **unix-first、靜態執行檔** —— macOS + Linux，`CGO_ENABLED=0`。chromedp 的 log 進檔案，永遠不進 TUI 正在畫的終端機。
- **Chromium 不在 release 裡** —— release 只有 Go 執行檔；第一次啟動下載到 cache。Chromium snapshot bucket 沒有 Linux ARM 版，所以沒有 Linux ARM build。
- **Chromium 的啟動與關閉** —— flag 是 Puppeteer 那組減 `--enable-automation`，加 `--disable-blink-features=AutomationControlled`、`--headless=new`，三個 background throttling 關閉。下載串流到 `.zip.part`、解到 `.tmp`、最後 rename，保留 symlink（Mac bundle 的 `Versions/Current`）。關閉先 `chromedp.Cancel`（讓 profile flush）再 allocator cancel；不論從哪裡離開（`q`、外部的 SIGINT / SIGTERM、終端機關掉），Chromium 都跟著走。
- **分頁的三種跑法** —— `press`（使用者的動作：設 loading / `settlingUntil` / `popupUntil`）、`act`（webu 自己的：Reveal、開 frame）、`unblock`（對話框 / auth 的回答：不排隊）。**action lock**（`tab.acting`）：讀 box 與按下之間曾被舊的 Reveal 捲走（六次漏一次），鎖住讓它們是一個動作；對話框開著時碰 renderer 會卡，所以回答不經鎖。動作後 300 ms `settle` 再 capture；`gen` 丟掉過期的。
- **hover 送了不等** —— headless 下單獨的 `mouseMoved` 要等 renderer ack 5 秒，但頁面當下就處理了，所以 hover 用 100 ms 的 ctx 送出就走。

### 量出來的事實（不再重查）

| 事實 | 影響 |
|---|---|
| headless 下單獨的 `mouseMoved` 要等 renderer ack 5 秒，但頁面當下就處理 | hover 送了不等（100 ms ctx） |
| `Security.certificateError` 已從協定移除 | 用 `setIgnoreCertificateErrors` |
| `Page.getFrameTree` 不列 OOPIF；`DOM.describeNode` 給 frame id；瀏覽器允許 attach iframe target；frame id 從 1 重編 | 跨站 frame 走自己的 session、id 加偏移 |
| chromedp 對 page target 開了 `Target.setAutoAttach(flatten)`，但不替 iframe session 建 executor、訊息全丟 | 自己 `NewContext(WithTargetID)` attach 一條 |
| 同站 frame 在自己那層讀整張 owner 表會找到自己（無限遞迴到 15 秒 timeout） | `frameOwners` 只看當前 document |
| 原生 `showModal` 砍掉 dialog 以外整棵樹、疊 modal 時連第一層也砍；aria-modal 不砍 | backdrop 用前一刻的版面；彈窗 stack push 不 drop |
| 數字經 float32（0.6 變 0.6000000238418579） | `cleanNum` 洗成六位 |
| password：Chromium 把 value 遮成 `•`、AX 不說；空欄位只有 snapshot 的 `type=password` 認得出 | password 從 snapshot 屬性判斷 |
| 沒有 option 的 ARIA combobox（Google 的搜尋框是 `<textarea role=combobox>`） | 當 Textbox |
| Google 對 headless 一律 reCAPTCHA（即使 UA 簽自己的名）；十二個搜尋引擎實測 DDG html 最乾淨 | 預設搜尋 |
| 內文裡的連結串最長 5–8（HN 全頁 2）、導覽 12–47 | 裸連結串只流動、不收合 |
| w3schools 沒有任何 landmark；沒 main 的頁把側欄連結欄當「節」 | part 靠幾何；`pruneNav` |
| 表單第一欄的 caret 掉到下一列 | `formValueW` 沒算 caret 前的空格（2026-09-23 修） |
| `tea.Sequence` 巢狀不等內層 | 先後執行寫成一個 cmd |
| Bubble Tea value receiver：改 popup 狀態的 helper 若是 value receiver 只回 `tea.Cmd`，改到的是副本 | pointer receiver，或先 `close()` 再 `dispatch()` |
| 有 timer 的頁面每秒指紋都不同 | spinner 轉滿 8 秒 grace（已知） |

### 程式碼目錄

```
cmd/webu/            進入點：version / browser update / 首次下載 / 啟動 Chromium / 進 TUI
internal/browser/    Chromium 的取得與執行：釘死 revision、目錄、flag、UA、關閉
internal/ir/         翻譯層：role 單一宣告表（roles.go）、AX tree + snapshot → IR（build.go）、
                     Dump / Markdown、fixture、docs/support.md 產生器
internal/page/       CDP 端：Capture（capture.go）、動作（actions.go）、hook（hooks.go）、
                     觀察與 DevTools 資料（observe.go / devlog.go）、跨站 frame 的 session（sessions.go）
internal/ui/         TUI：app.go（key 路由、Space menu、dispatch）、tab.go（分頁模型：capture / apply /
                     settling / places / drill / 移動）、render.go（IR → rows / items / marks）、
                     parts.go（四個 part）、section.go + sectionlist.go（目錄與節）、finder.go（/ 與 go）、
                     pagepopup.go（頁面的彈窗）、pagepanel.go（[2] 的畫法）、editorpopup.go、slider.go、
                     fill.go、inputpopup.go / spacemenu.go / popup.go（webu 的浮層）、bookmarks.go /
                     listpanel.go / settings.go（screen）、devtools*.go、selectmode.go、theme.go
internal/store/      config / bookmarks / history / session 的 YAML；Netscape 書籤匯入
internal/paths/      三個目錄的解析
tools/axdump/        看任何頁面的 AX tree（本機 Chrome）
```

依賴方向：`ui → page → ir`、`ui → browser`；`ir` 不依賴 CDP 以外的任何東西。`page/sessions.go` 用 `ir.CarriedFrom` 當 id 偏移的單位，是 page 對 ir 唯一的反向依賴（一個常數）。

## 設計決定

0.2.x 把 accessibility tree 排成一長頁；0.3.0 把它讀成一份**文件**。各項的完整理由與日期在設計文件裡，這裡是摘要：

- **一份文件三個畫面。** 有三個以上標題的頁面開頁先進**目錄**：一列一節、依深度縮排、右欄是它有幾張表、幾段 code、幾個媒體、多少行。`Enter` 讀**一節** —— 一節包含它的子節，章包含節 —— `n`/`p` 在同深度的節之間走、`Esc` 回目錄。`Space` › `One sheet` 攤成一整張，`Sections` 再切回來。
- **四個 part，靠幾何切。** 頁面上最大的區塊是 **body**；在它旁邊的是 **others**；在它前面的是 **header**、後面的是 **footer**。不看標籤叫什麼 —— `nav` 也好、`div` 也好，位置決定。頁面比視窗矮、或沒有一塊獨大的頁面就是一塊。
- **finder，不是逐字搜。** `/` 的範圍是四個 part 裡每一個持有文字的區塊；清單上 `Enter` 是**去那裡**（切 part、開節、走進項目、游標落定），什麼都不按。`go` 加數字跳行；目錄上的行號就是節的序號。
- **一件事一列。** 佔好幾行的清單項目或 article 只畫第一行；`Enter` 走進去、`Esc` 出來，頁面巢多深就走多深。
- **表單畫成表單。** label 一欄對齊、值靠一邊、label 永不截斷（擠不下就疊成兩列）。每一種輸入都定義了互動；日期、時間、顏色框不合瀏覽器的形狀就擋下來，不讓瀏覽器無聲丟掉。
- **頁面自己的彈窗靠行為認** —— 按下之後出現、拿到 focus 或自己宣告或疊在別的東西上、裡面有東西可按 —— 從不看標籤。它要一個回答：`Esc` 不關。
- **frame 是一層。** 跨站 frame 是另一個 process、另一個 target，`Page.getFrameTree` 看不到；webu 用 `DOM.describeNode` 拿到它的 target、開一條自己的 session。frame 的 node id 從 1 重新數，所以帶進來時加上 `slot × 2^40`。巢狀 frame 一層一層接。
- **顏色是概念，不是元素。** 可按的 sapphire、可填的 mauve、code pink、媒體灰、pagetab rosewater、填錯的值紅；heading 與目錄用五個 hue 表深度。沒有括號、沒有 emoji，每個控制項以 glyph 開頭。配色 catppuccin-mocha。
- **頁面記得自己的位置**，每個 history entry 各記一份。還在長的頁面（SPA）spinner 一直轉到兩次看到的一樣為止。`Enter` 先 hover 再點。一次一個動作，快速走動不會把點擊拉離目標。
- **webu 簽自己的名字** —— Chromium 自報的 user agent，把 headless 版的 `HeadlessChrome/` 寫成 `Chrome/`、尾巴接 `webu/<version>`。不是偽裝，是一個真瀏覽器的名字。
- **預設搜尋是 DuckDuckGo 的 HTML 端點**，因為 Google 對每一次 headless 搜尋都回 reCAPTCHA。
- **沒有交棒給視窗** —— CAPTCHA、passkey、WebRTC 是牆，webu 明講；webu 必須能在沒有 display 的機器上跑，所以沒有視窗可以交棒。

## 已否決，不要重提

完整理由在設計文件原地、各自標日期；這裡是索引。

| 否決的做法 | 改成 | 出處 |
|---|---|---|
| 自訂 protocol / 新內容格式、自己 parse HTML、Chromium 像素轉字元 | 真 Chromium + AX tree + 版面快照 | `function.md` §0「排除的路線」 |
| 為單一網站寫 heuristic（2026-09-21） | 只認 AX tree 與 DOM 的版面事實 | `function.md` §0 |
| 交棒給有視窗的 Chromium 讓人過 CAPTCHA（2026-09-23） | 沒有交棒，牆明講 | `function.md` §9.1 |
| `Enter` 開該 item 的選單（2026-09-20） | `Enter` 是左鍵，點了沒意義就走進去（09-21） | `ux.md` §A.0.K |
| landmark 入口行（09-21）、URL 底下的 landmark 膠囊、Outline popup（09-22） | 四個 part 由幾何切（09-23） | `ui.md` §2 `[2]` |
| 一節只到下一個任何標題 | 一節包含它的子節（2026-09-23） | `ui.md` §2 `[2]` |
| heading 用六級底色（09-22） | 五個 hue 輪流表深度（09-23） | `ui.md` §2 `[2]` |
| 頁面的彈窗佔整個面板 | 浮在 `[2]` 上的框（2026-09-23） | `ui.md` §2 `[2]` |
| `/` 進 visual mode 搜尋 | `/` 是 finder（2026-09-23） | `ux.md` §1.1 |

## 已知的牆與未做

網頁是二維的、終端機不是，所以一個密集的 app 頁（issue tracker 的看板、dashboard）分類是對的、但還是得在裡面移動 —— finder 與目錄是路，不是捲動。一個完全不宣告語意的站（全是 `div`、沒有 ARIA）給 webu 的只有文字和能點的東西；那種站 screen reader 也會壞，webu 不為單一站加 heuristic 去追。

還沒做的：

- 媒體（圖片、影片、音訊是佔位）
- `<textarea>` 交給使用者的 `$EDITOR`；小數 step 的 slider（目前只列整數）
- 游標在 frame 裡時只捲 frame，不捲外面的頁
- timer 類 live region 會讓 settling 的 spinner 轉滿 8 秒
- 只在 pointer-over 才打開的選單：hover 送了不等（見「運作方式」），這類選單 webu 目前開不出來
- 頁面名字自帶 Nerd Font glyph（APG 的 tree 用 U+F07B 當資料夾 icon）看起來像多一格空白 —— 先放著
- 滑鼠、Linux ARM

尚未符合 tdp 的地方：逐條列在 [`webu-terminu-fix.md`](webu-terminu-fix.md)。

## 偏離 tdp

- **Space menu 的 global operation 區只有一列（M2）。** tdp M2 要 Space menu 的 `global operation` 區列出全部全域動作；webu 的全域動作約十個（切畫面 `W` `B` `H` `D` `S`、`P` / `N`、`L`、`v`、`q`……），全部列進每一個 Space menu，會比 panel 自己的動作還長。所以 webu 的 global 區只放一列 `[?] global operation…`，按下去打開 `?` menu，全域動作的完整清單在那裡、可以直接執行（M4）。2026-09-26 定案；程式目前還沒有 global 區，見 fix 檔。
- **頁面自己的彈窗 `Esc` 不關（K4、F3）。** 頁面跳出的 modal / alertdialog / menu / cookie 橫幅，webu 畫成浮在頁面上的框（`ui.md` §2.4），但它不是 webu 的 popup，是頁面的：頁面放它上來要一個回答，`Esc` 關掉等於把那個決定擱著。所以 `Esc` 在它上面只 toast 說明，要按裡面的按鈕、或等頁面自己收掉（`ux.md` §5；`app.go` `togglePagetab`）。webu 自己的 popup 照 K4、F3。

## 設計文件導讀

| 檔案 | 回答什麼 | 順序 |
|---|---|---|
| [`function.md`](function.md) | Chromium 做什麼、webu 做什麼、做到哪；翻譯層（accessibility tree + 版面快照 → 一份文件）；role 表；frame；彈窗；Chromium 怎麼取得、怎麼跑 | 第 1 |
| [`ui.md`](ui.md) | 版面、header 的 screen 與兩個面板、頁面的三個畫面與四個 part、每種東西怎麼畫、popup、兩套配色、存檔 | 第 2 |
| [`ux.md`](ux.md) | core-key 語意、每種東西的 Enter、每個焦點的 `Space` 選單、finder、每種輸入的行為、熱鍵全表、時間軸；§A、§B 沿用 VTP 時期的分章，標題標出對應的 tdp 條目 | 第 3 |
| [`support.md`](support.md) | 支援的 accessibility role，由 role 表產生 | — |

設計本身 —— 包含試過而被否決的做法 —— 每條決定都在原地標了日期。

## 用什麼做的

Go、[Bubble Tea](https://github.com/charmbracelet/bubbletea) 與 [Lip Gloss](https://github.com/charmbracelet/lipgloss)、浮層用 [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay)、Chrome DevTools Protocol 用 [chromedp](https://github.com/chromedp/chromedp)、語法色用 [chroma](https://github.com/alecthomas/chroma)。瀏覽器是 Chromium 專案自己的 snapshot 版，依 revision 釘死。

## 建置與開發

```bash
git clone https://github.com/vulcanshen/webu.git
cd webu
make build     # → ./webu（CGO_ENABLED=0、-trimpath、strip）
./webu
```

```
make build              → ./webu；第一次啟動會把釘死的 Chromium 下載到 cache
make install / uninstall → $GOBIN
make test               全部測試；需要瀏覽器的只在 Chromium 下載好之後跑，否則 skip
make fixtures           用釘死的 Chromium 重抓 internal/ir 的 role fixture 與 docs/support.md
WEBU_SMOKE=1 go test ./internal/ui -run TestSmoke -v     真站 smoke（Hacker News、GitHub）
make axdump URL=https://…                                任何頁面的 accessibility tree，用本機 Chrome
make gif                從 .local/demos/demo.tape 重錄 docs/demo.gif（VHS、Nerd Font、網路）
make snapshot           goreleaser 本機打包到 dist/
```

`make` 列出全部。

- **fixture 與 golden**：`internal/ir/testdata/<group>.html` → `make fixtures`（只認釘死的 Chromium）→ `<group>.json` → `.golden`（`ir.Dump`）+ `.md`（`ir.Markdown`）；`internal/ui/testdata/<group>.render` 是同一批在 60 欄的排版。`-update` 重生 golden，改渲染時看 diff 決定。
- **瀏覽器測試**（需要釘死的 Chromium，沒有就 skip）：`hooks_test`（dialog、新視窗、憑證、下載、auth、上傳 picker、PDF、hover、iframe 同站）、`frame_test`（跨站 + 巢狀）、`popup_test`、`loading_test`、`finder_test`、`section_test`、`parts_test`、`app_test` 等。
- **測試 driver**（`app_test.go`）：`newDriver` / `startAt` / `until`（20 秒）/ `cursorOn` / `key`；`until` 的條件要寫 closure（`d.m` 是值，綁方法會綁到舊副本）。
- **重現 TUI bug** 用 Go driver；要手動跑 `./webu` 重現時，把 `WEBU_CONFIG`、`WEBU_DATA` 指到暫存目錄，不碰自己的設定、書籤與登入。
- **glyph 碼位**用 `fontTools` 讀已安裝 Nerd Font 的 cmap 查，不憑記憶。
- **demo gif**：vhs 0.12.0 曾在 macOS 上 2 秒就結束、不出檔也不報錯；遇到時改用 0.11.x：`make gif VHS=<vhs 0.11 的路徑>`。

## 發布

走家族的做法：推一個 `v*` tag，GitHub Actions 在兩個平台跑測試、goreleaser 打包並更新 Homebrew tap（`vulcanshen/homebrew-tap` 的 `webu.rb`），release note 是 `CHANGELOG.md` 對應的那一節。

發布後確認資產時看 `repos/<owner>/<repo>/releases/<id>/assets`；`gh release view` 與 `releases/tags/<tag>` 的 assets 剛發布時可能還是空的。**不要對已經成功的 run 重跑 release job** —— goreleaser 會因為資產已存在而失敗；真的要重來，先刪掉資產再重跑。
