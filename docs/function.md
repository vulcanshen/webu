# webu — Function

> 本文件講 webu 的功能做到哪裡：頁面怎麼取得、變成什麼、外殼有哪些功能、哪些做不到、怎麼驗收、命令列。
> 閱讀器的規格（挑正文的規則、元件的畫法、按鍵、表單、這一頁的資訊）在 [`reader.md`](reader.md)；
> 版面在 `ui.md`、互動語意在 `ux.md`、程式裡怎麼做在 `dev-remarks.md`。每條決定標日期。
>
> 到 0.5.0 為止，webu 在背景跑一個釘死版本的 headless Chromium、讀它的 accessibility tree；
> 2026-10-28 改成閱讀器，2026-10-29 Chromium 整個拿掉（user 定）。本文件 2026-10-29 照現在的 webu
> 改寫：Chromium 時期的設計在 git 歷史裡，否決的做法列在 `dev-remarks.md`「已否決，不要重提」。

---

## §0 定位

**一句話**：打開網址，看到這一頁直接給的內容，乾淨、好讀，用鍵盤跟著連結走（user，2026-10-28）。

webu 不寫 HTML 引擎、不跑 JavaScript、不定義新 protocol。它用 Go 的 `net/http` 抓頁面、`x/net/html`
解析，挑出這一頁直接給的內容（文章、文件、答案），建成自己的內容樹，在終端機畫成 webu 自己的元件。
網站外殼（導覽列、側欄、頁尾、cookie 告示、浮層）不顯示，要看時 `w` 切到整頁。

### 排除的路線

| 路線 | 為什麼不走 |
|---|---|
| 自訂 protocol / 新內容格式（gemtext 式） | 沒有網站會 follow；Gemini 做了十年只有幾千個站 |
| 跑 Chromium、讀它的 accessibility tree（到 0.5.0 的 webu） | 2026-10-29 整個拿掉，原本要留給靠 JavaScript 才有內容的空殼頁，user 決定那些也不要：換來的是不用下載、不用跑瀏覽器，也有 linux/arm64 的版本 |
| Chromium 像素轉字元（browsh / carbonyl 式） | 得到的是縮小的 GUI，沒有語意，套不上 tdp（terminu design principle） |
| 為單一網站寫規則 | 追不完，而且一改版就壞（定案 2026-09-21）。挑正文只用 Readability 與 `reader.md` 的一般規則 |

### 與同類工具的差異

| | w3m / lynx | browsh / carbonyl | webu |
|---|---|---|---|
| 引擎 | 自寫 HTML 引擎 | Chromium | 沒有引擎：`net/http` 抓、`x/net/html` 解析 |
| JS | 不跑 | 跑 | 不跑 |
| 畫的東西 | DOM 模擬視覺排版 | 像素轉字元 | 這一頁直接給的內容（Readability 挑；`w` 整頁），建成內容樹、畫成 webu 的元件 |
| 登入 / session | 自己的 cookie，JS 登入流程即死 | Chromium 的 | 自己的 cookie（`cookies.json`），GET 表單與簡單的帳號密碼 POST；JS 登入一樣不行 |
| 成本 | 幾 MB | 幾百 MB | 一個靜態執行檔，不下載、不跑瀏覽器 |

### 牆在哪裡（2026-10-28 明講）

終端機擅長的是讀文字：文件、文章、README、新聞、搜尋結果。網頁 app（信箱、社群、後台）在終端機裡比不過
真的瀏覽器；Chromium 時期 webu 大部分的複雜度花在那裡，閱讀器不做。2026-10-29 起 JavaScript 完全不跑，
靠 script 才長出內容的頁（app、dashboard、很多社群網站）看到的很少或什麼都沒有：這是接受的代價，不追。

---

## §2 資料流

```
網址，或要搜尋的字（o、命令列、書籤、歷史）
  │
  ▼  net/http GET：cookies.json 的 cookie、跟著轉址、UA 是瀏覽器的字串尾巴接 webu
回應
  ├─ HTML、其他文字（text/*、JSON、XML），或狀態碼 400 以上 → 讀
  └─ 其他（PDF、圖片、壓縮檔……）→ 存進下載資料夾，畫面上的頁不動
  │
  ▼  轉成 UTF-8 → x/net/html 解析 → 拿掉看不到的（script、style、藏起來的、對話框、cookie 告示）
  ▼  Readability 挑正文；挑不到：整頁減掉導覽、頁首、頁尾、側欄
內容樹（internal/ir）：正文一棵、整個 <body> 一棵（w）；同一個欄位在兩棵裡是同一份
  │
  ▼
[2]：畫成 webu 的元件；j|k 捲動、/ 找字；f 進 Focus mode，在連結、圖片、欄位、按鈕之間走
  │
  ▼  Enter
連結 → 下一個 GET（指到這一頁的 #錨點 直接捲過去）
表單 → GET 把欄位組進網址，或 POST 帶上隱藏欄位 → 回到「回應」
```

- **webu 自己發每一個 request**，只有這幾種：頁面（含每一次轉址）、表單送出、[1] 網站記號要的 favicon
  （每個網站一次 session 一次）。頁面上的樣式、script、圖片不抓；JavaScript 不跑，頁面自己的 fetch、XHR、
  WebSocket 也就都沒有。Requests popup 記的是載入這一頁的請求，favicon 不算。
- 伺服器 30 秒內沒開始回應就放棄；回應本身不限時間（大檔下載不被切斷）。一頁最多讀進 16 MB，超過的切掉。
- HTML 以外的文字整份是一個 code block，照原樣、不重排；JSON 照 JSON 上色。
- 狀態碼 400 以上照樣顯示伺服器給的內容，toast 說狀態。
- 挑正文的規則（表格、清單頁、浮層、程式碼、藏起來的元素）、元件的畫法、表單送出時怎麼組值，在 `reader.md`。

---

## §7 完整度矩陣

| 程度 | 內容 |
|---|---|
| **完整** | 讀文章、文件、README、新聞、搜尋結果，正文或整頁（`w`）；HTML 以外的文字（純文字、JSON、XML）；跟連結走、頁內 `#錨點`、分頁自己的上一頁下一頁（回到離開時的位置）、頁內找字；GET 表單（站內搜尋、篩選）；簡單的帳號密碼 POST 登入，cookie 存檔，下次啟動還登入著；不是網頁的回應下載下來 |
| **有損** | 圖片、影片、音訊只有 icon 與網址（圖片帶 alt），Yank 出去用別的程式開；PDF 這類檔案直接下載；挑正文挑錯的頁（中央社這類清單頁會先挑到選單、Wikipedia 的 infobox，`reader.md`「還沒定的」）要 `w` 看整頁；太寬的表格切掉加 `…`；不在表單裡的按鈕多半要 JavaScript，按了只 toast 說明 |
| **做不到** | 靠 JavaScript 才長出內容的頁；網頁自己的對話框（alert、confirm、prompt）與 frame；靠 JavaScript 或第三方的登入、CAPTCHA、passkey / WebAuthn、兩步驟驗證；HTTP 認證；檔案上傳；憑證不對的站（不問要不要放行，[2] 寫出連不上的原因）。能做的是 `Y` 複製網址，帶到別的地方開 |

### 哪些頁讀得好

| 類別 | 例子 |
|---|---|
| 文章型，讀得好 | 文件、文章、README、新聞：Readability 挑得到正文 |
| 清單頁，走備援 | HN、搜尋結果、新聞首頁：Readability 判斷不是文章，改成整頁去掉導覽、頁首、頁尾、側欄 |
| 讀不到 | 網頁 app（信箱、社群、後台、dashboard）：內容是 JavaScript 長出來的 |

---

## §8 Shell 功能清單

| 功能 | 怎麼做 | 存檔 |
|---|---|---|
| 分頁 | 每個分頁有自己的頁面與歷史；連結在同一個分頁開，開新分頁是 `T` 或連結的 Space menu「Open in new tab」 | 無 |
| 輸入網址 | 網址框（`o`）：目前的網址當灰字提議；不像網址的字當搜尋，預設 DuckDuckGo 的 HTML 版（2026-09-23：Google 對程式送來的搜尋一律回 reCAPTCHA；十二個引擎實測，DDG html 最乾淨） | `config.yaml` `search_engine` |
| 上一頁下一頁 | `H\|L`：分頁自己的歷史，回到離開時的位置；`b` 麵包屑列出這個分頁走過的每一頁，選一頁直接跳過去 | 無 |
| 正文／整頁 | `w` 切換 | 無 |
| 頁內搜尋 | `/`：所有命中的字加底線，`n\|N` 走，`Esc` 收掉（finder 拿掉，2026-10-28） | 無 |
| 行號 | [2] 的頁面左邊有行號 | 無 |
| 錯誤頁 | 抓不到（DNS、連線、TLS 憑證……）：[2] 用紅字寫原因，App Log 記一筆；重新載入失敗時原本的頁留著，toast 說沒載到 | 無 |
| 登入 | GET 表單與簡單的帳號密碼 POST（`reader.md`「表單」）；每收到一個 cookie 就寫檔，session cookie 也存；Settings 的 `C` 清除登入資料 | `cookies.json`（狀態，`0600`） |
| 這一頁的資訊 | [2] Space menu 最上面三項，沒有熱鍵：Source（抓下來的原始內容）、Cookies（這個網站的，`C` 清掉）、Requests（`curl -v` 的寫法）。2026-10-29 取代 DevTools；Console 與 web storage 沒有了 | 無 |
| 歷史 | 自存：時間、URL、標題，append-only，無限保留 | `history.yaml` |
| 書籤 | 目錄樹、搬移、改名、匯入瀏覽器的匯出檔 | `bookmarks.yaml` |
| session 還原 | 離開時寫下所有分頁，下次開回來（不預載，切到才載） | `session.yaml` |
| 下載 | 不是網頁的回應（§2）：toast、Downloads 一列（`x` 取消）、header 下的進度條；有下載在跑時離開先問 | `download_dir` |
| 設定 | `config.yaml` 每個 key 都有一列 | `config.yaml` |
| Yank | 頁面 URL、連結 URL、圖片網址、文字、欄位值（密碼不給）、畫面上那一份（正文或整頁）的 markdown | 剪貼簿 |

**不做，明講**：密碼管理與自動填入；擴充套件；多 profile；print to PDF；整頁截圖；隱私分頁；proxy 設定（v2）。

---

## §10 已決事項

| 決定 | 日期 |
|---|---|
| 閱讀器，不是瀏覽器：只顯示這一頁直接給的內容，網站外殼不顯示，`w` 切整頁 | 2026-10-28 |
| 取頁用 Go 的 `net/http`，`x/net/html` 解析；挑正文用 Readability 加 webu 自己的規則 | 2026-10-28 |
| 表單做到第 2 層：GET 表單、簡單的帳號密碼 POST；靠 JavaScript 或第三方的登入不做 | 2026-10-28 |
| cookie 存在狀態目錄的 `cookies.json`（`0600`），值不進 App Log | 2026-10-28 |
| 連結直接開，不先問；不是網頁的回應下載 | 2026-10-28 |
| Chromium 完全拿掉，空殼頁也不例外 | 2026-10-29 |
| DevTools 縮成「這一頁的資訊」：Source、Cookies、Requests | 2026-10-29 |
| Enter = 左鍵（閱讀時是 Focus mode 的 `Enter`）、Space = 右鍵選單，分開 | 2026-09-21 |
| CAPTCHA 是硬牆；預設搜尋 DuckDuckGo html | 2026-09-23 |
| UA 尾巴簽 webu 的名字 | 2026-09-23 |
| 不為單一網站加規則 | 2026-09-21 |

---

## §11 驗收

- `make check` 通過：`fmt-check`、`vet`、`go test -race ./...`，有 `go.work` 時加 `check-lib`。測試不用網路：
  閱讀器的測試在 `httptest` 架的小網站上跑，測試頁是照真實網站樣子寫的小頁面（真實網站的 HTML 有著作權，
  不放進 repo）。
- `internal/reader`：取頁、轉址與請求紀錄、cookie 存檔與一個網站的 cookie、挑正文（文章拿掉網站外殼、
  清單頁與它的表格、藏起來的不顯示）、表單送出、favicon 的顏色。
- `internal/ui/reader_test.go`：讀頁、跟連結、`H|L`、`w`、`/` 找字、Focus mode、表單、登入的 cookie
  （`0600`）、搜尋框、下載、404、網站記號、Source／Cookies／Requests。
- `internal/paths`：舊位置搬過來、舊的 Chromium 刪掉。
- tdptest（`TestTDP` 等）：每個畫面與 popup 在各種尺寸、icon 寬度下都畫得出來，縮到最低尺寸以下也不壞；
  頁面與標題裡的控制字元不送到終端機。
- 挑正文的實際效果：`make mddump URL=…` 對真實網站對照（第 1 階段用了 8 個網站，`reader.md`）。

---

## §12 CLI

| 指令 | 行為 |
|---|---|
| `webu` | 還原上次 session 的分頁（不預載，切到才載）；`restore_session: false` 則從空白開始 |
| `webu <url\|words> ...` | 每個參數開一個分頁，接在還原的分頁後面，第一個在前；解讀同網址框（`o`）：有 `://` 照用，`localhost`、`127.` 開頭補 `http://`，像網域的（沒有空白、有 `.`）補 `https://`，其他當搜尋 |
| `webu version` | webu 的版本 |
| `webu help` | 用法（`-h`、`--help` 同）；其他 `-` 開頭的參數是未知選項：印用法、exit 2 |

看一頁挑出來的內容：`make mddump URL=…`（`tools/mddump`：`net/http` 抓、Readability 挑、印成 markdown；
`-whole` 照轉整個 `<body>` 對照）。
