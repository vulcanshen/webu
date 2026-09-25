# webu 開發者備忘

README 只介紹這個工具怎麼用；這份收的是 README 以前寫著、但屬於開發者的部分：webu 裡面怎麼運作、設計為什麼這樣定、文件怎麼讀、怎麼建置與發布。

webu 是 `u`-family 的成員，也是 [這份 TUI 設計原則](https://github.com/vulcanshen/thoughts/blob/main/tui-design/README.md) 在瀏覽器領域的實作 —— 與 [kbu](https://github.com/vulcanshen/kbu)、[filu](https://github.com/vulcanshen/filu)、[sshu](https://github.com/vulcanshen/sshu) 同一套設計系統。

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

## 0.3.0 的設計重點

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

## 牆在哪裡

網頁是二維的、終端機不是，所以一個密集的 app 頁（issue tracker 的看板、dashboard）分類是對的、但還是得在裡面移動 —— finder 與目錄是路，不是捲動。一個完全不宣告語意的站（全是 `div`、沒有 ARIA）給 webu 的只有文字和能點的東西；那種站 screen reader 也會壞，webu 不為單一站加 heuristic 去追。

還沒做的：

- 媒體（圖片、影片、音訊是佔位）
- `<textarea>` 交給使用者的 `$EDITOR`；小數 step 的 slider（目前只列整數）
- 游標在 frame 裡時只捲 frame，不捲外面的頁
- timer 類 live region 會讓 settling 的 spinner 轉滿 8 秒
- 滑鼠、Linux ARM

## 設計文件

| 檔案 | 回答什麼 | 順序 |
|---|---|---|
| [`function.md`](function.md) | Chromium 做什麼、webu 做什麼、做到哪；翻譯層（accessibility tree + 版面快照 → 一份文件）；role 表；frame；彈窗；Chromium 怎麼取得、怎麼跑 | 第 1 |
| [`ui.md`](ui.md) | 版面、header 的 screen 與兩個面板、頁面的三個畫面與四個 part、每種東西怎麼畫、popup、兩套配色、存檔 | 第 2 |
| [`ux.md`](ux.md) | core-key 語意、每種東西的 Enter、每個焦點的 `Space` 選單、finder、每種輸入的行為、熱鍵全表、時間軸 | 第 3 |
| [`webu-implementation.md`](webu-implementation.md) | 實際怎麼做的、量到了什麼、踩過的坑、測試、做到哪 | — |
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

## 發布

走家族的做法：推一個 `v*` tag，GitHub Actions 在兩個平台跑測試、goreleaser 打包並更新 Homebrew tap（`vulcanshen/homebrew-tap` 的 `webu.rb`），release note 是 `CHANGELOG.md` 對應的那一節。

發布後確認資產時看 `repos/<owner>/<repo>/releases/<id>/assets`；`gh release view` 與 `releases/tags/<tag>` 的 assets 剛發布時可能還是空的。**不要對已經成功的 run 重跑 release job** —— goreleaser 會因為資產已存在而失敗；真的要重來，先刪掉資產再重跑。
