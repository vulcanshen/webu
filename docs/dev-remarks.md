# webu 開發者備忘

README 只介紹這個工具怎麼用；這份收的是屬於開發者的部分：webu 裡面怎麼運作、設計為什麼這樣定、文件怎麼讀、怎麼建置與發布。webu 遵循 [terminu design principle](https://github.com/vulcanshen/terminu)（tdp v1.1.0），畫面建在家族的共用 lib [terminu-go](https://github.com/vulcanshen/terminu-go)（v1.1.1）上，是 terminu family 在瀏覽器領域的成員。

---

## 運作方式

webu 是閱讀器（[`reader.md`](reader.md)，user 2026-10-28 定）：網頁用 Go 的 `net/http` 抓下來（cookie jar 存檔、跟著轉址、`x/net/html/charset` 轉成 UTF-8），`x/net/html` 解析，挑出這一頁直接給的內容（Readability；挑不到就是整頁減掉導覽、頁首、頁尾、側欄），建成 webu 的內容樹（`internal/ir`），再由 [2] 畫成 webu 自己的元件。網頁的 JavaScript 不跑。到 0.5.0 為止 webu 是在背景跑一個釘死版本的 headless Chromium、讀它的 accessibility tree；Chromium 在 2026-10-29 整個拿掉（user 定，空殼頁也不例外）。

- **一份登記、lib 產生其餘** —— webu 把每個畫面、panel、動作（鍵、名稱、說明、範圍、要不要先問、做什麼）登記給 terminu-go；Space menu、global operation popup、`?` 的 key reference、panel 下框的 hint、footer、confirm、淡化、`Esc` 一次一層都由 lib 從這一份登記產生，所以不在選單裡的熱鍵不可能存在。[2] 的動作跟著頁面與 Focus mode 的游標換，每次換頁、換游標都重新登記（`pageactions.go` 的 `syncPage`）。
- **畫面穩定** —— 任何尺寸、任何內容，每一列都恰好是終端機的寬；`TestTDP` 在各種尺寸、icon 一格／兩格下走過每個畫面與 popup，`TestEveryPopupFits` 把每一種 popup 開在畫面上再縮小畫面，`TestUnderTheLeastSize` 從最低尺寸以下一路縮到 1×1（tdp L4、tdp E5、tdp E6；單獨量 lib 的 popup 框是 lib 的測試，tdp E8）。
- **`[S]ettings` 與 `config.yaml` 同步** —— 檔案裡有的 key，Settings 一定有一列，有測試守著。
- **讀不了時**（tdp E3）—— `config.yaml`、`bookmarks.yaml`、`history.yaml`、`session.yaml`、`cookies.json` 讀不了，main 交給 UI（`ui.Unread`），第一個 frame 一個接一個開 lib 的 `MoveAside`：`Enter` 改名成 `.bak` 再從新的開始（`fresh`），`Esc` 離開、main 把路徑與原因印到 stderr。問的時候什麼都不開：`firstFrame` 等最後一個 `Enter` 才跑，跟第一次啟動一樣（E3-18），壞掉的 `history.yaml` 也不會在回答前被附加一筆造訪。`store` 的 Load 讀到一半出錯時什麼都不給 —— yaml 會先填好出錯前讀到的部分，壞掉的 `config.yaml` 前面一行 `restore_session: false` 就會在回答前丟掉上次的分頁。`session.yaml` 讀不了、又沒搬開時，離開時不寫回它（`SessionWritable`）；`cookies.json` 讀不了時 jar 是空的、回答前不寫檔。
- **不再用的檔**（tdp E3）—— 0.5.0 以前的 Chromium 留下三樣：快取裡的 Chromium 本體（`chromium-<rev>`，快取的新舊兩個位置都看）是快取，main 啟動時刪掉（`paths.DropChromium`，user 2026-10-29 選的），App Log 寫刪了哪裡、多大，刪不乾淨寫 Warning；資料目錄裡的 `profile/` 與 `webu.log` 是資料，不刪，每次啟動交給 lib 的 `App.NoLongerUsed` 在 App Log 寫一筆 INFO（`startNews`），CHANGELOG 也寫了。tdp E3 的「Files no longer used」只寫了設定、狀態、資料，沒寫不再用的快取，已回報 terminu。
- **頁面上的控制字元**（tdp components.5）—— [2] 的頁面是 webu 自己畫的：`rowLine` 與 visual mode 的 `selectRows` 把控制字元畫成紅色的名字（`writeNamed`、`controlName` 用 lib 的 `Escape` 的寫法），整列照舊截到 panel 寬；code block 的 Tab 畫成 `input_tab_width` 格。交給 lib 的名字與短字（清單的格子、menu、標題、toast）照原樣交，由 lib 畫；webu 自己的顏色用 `Row.Styled` 與 `App.Status` 的 `Span`。
- **存檔** —— 書籤每筆帶 `folder` 路徑，另有 `folders:` 清單讓空目錄留得住；歷史是一次 append 一筆的 YAML sequence；每次寫檔都是原子的。分法照 tdp E3「刪掉會怎樣」：設定（`config.yaml`）與狀態（`session.yaml` 上次開著的分頁、`cookies.json` 登入）在 config、刪了找不回來的資料（書籤、歷史、下載）在 data；快取沒有東西要放。舊版放在別處的，第一次啟動逐項搬過去（tdp E3；`store.Relocate` 一個檔一個檔，各一次 rename），Info toast 說從哪搬到哪，搬完把空了的舊目錄拿掉。新位置已經有的那一項不刪，留在舊位置，error popup 寫出兩個路徑請使用者自己合併。搬不動就照舊用舊位置（`paths.Either`：新位置沒有、舊位置有），error popup 寫出兩個路徑。
- **unix-first、靜態執行檔** —— macOS + Linux（amd64、arm64），`CGO_ENABLED=0`。
- **不論從哪裡離開**（`q`、外部的 SIGINT / SIGTERM、終端機關掉）都寫回 session：SIGINT、SIGTERM 由 Bubble Tea 自己收，終端機關掉的 SIGHUP 它不收，`main.go` 把它轉成對自己的 SIGTERM。

### 量出來的事實（不再重查）

| 事實 | 影響 |
|---|---|
| Google 對程式送來的搜尋一律 reCAPTCHA（即使 UA 簽自己的名）；十二個搜尋引擎實測 DDG html 最乾淨（2026-09-23） | 預設搜尋 |
| HN 的新聞列表是資料表格的樣子（Chromium 自己也這樣判）；沒有 `<th>`、`<caption>` 的表格多半是排版用 | 表格自己判：排版用的照區塊讀、資料表畫成格子 |
| 中央社這類清單頁 Readability 會先挑到選單 | 挑正文的規則還要調（`reader.md`「還沒定的」） |
| `tea.Sequence` 巢狀不等內層 | 先後執行寫成一個 cmd |
| Bubble Tea value receiver：改 popup 狀態的 helper 若是 value receiver 只回 `tea.Cmd`，改到的是副本 | pointer receiver，或先 `close()` 再 `dispatch()` |
| tmux 收到 `Escape` 會等一下看是不是組合鍵，後面緊接著的鍵可能被吃掉 | 實機腳本 `Escape` 之後等 1 秒以上 |

### 程式碼目錄

```
cmd/webu/            進入點：version / help / 刪掉舊的 Chromium / 讀檔 / 進 TUI
internal/reader/     閱讀器：Client（net/http、transport 記請求 requests.go）、Jar（cookie 存檔）、
                     Read（解析、挑正文，read.go）、tree.go（HTML → 內容樹）、form.go（表單組 request）、
                     markdown.go（Yank markdown）、icon.go（favicon 的主色）
internal/ir/         內容樹：Node、Kind
internal/ui/         TUI，建在 terminu-go 上：app.go（AppModel、向 lib 登記、onMsg、sync）、
                     web.go（Web 畫面：[1] Tabs、版面）、siteicon.go（[1] 的網站記號）、
                     pagebody.go（[2] 的 Body：網址列、內容、行號）、pageactions.go（[2] 的動作、Enter）、
                     reader.go（分頁載入、歷史、表單、下載）、readerrender.go（內容樹 → rows / items）、
                     readerkeys.go（捲動、/ 找字、Focus mode、[2] 的 panel 動作）、pageinfo.go（Source、
                     Cookies、Requests）、render.go（row、seg、layout 的型別）、visual.go + selectmode.go
                     （visual mode）、tab.go（分頁模型）、bookmarks.go / history.go / downloads.go /
                     settings.go（其他畫面）、splash.go + icon.svg（docs/icon.svg 的副本）、width.go
                     （lib 的寬度函式）、theme.go（頁面的配色與 glyph）
internal/store/      config / bookmarks / history / session 的 YAML；Netscape 書籤匯入
internal/paths/      設定、狀態、資料、快取四個目錄的解析，舊位置搬過來，舊的 Chromium 刪掉
tools/mddump/        實驗：抓一頁、Readability 挑內容、印成 markdown（自己的 go.mod）
```

依賴方向：`ui → reader → ir`、`ui → store、paths`；`ir` 不依賴任何東西。

### 實作落地

[`reader.md`](reader.md) 是閱讀器的規格；這一節記程式裡實際怎麼做、對應哪些名字。

#### 取頁與 cookie（`internal/reader`）

- **Client**：`http.DefaultTransport` 複製一份，`ResponseHeaderTimeout` 30 秒、沒有整體的 timeout（大檔下載不被切斷）；外面包一層 `recorder`，請求的 context 帶著 `reader.Requests`（`Record`）就記下每個請求，轉址每一跳一筆（Requests popup）。不送 `Accept-Encoding`：Go 自己要 gzip、自己解，brotli 它解不了，就不讓伺服器給。UA 是瀏覽器的字串、尾巴接 `webu`（`ui.readerUA`）。
- **Jar**：Go 的 `cookiejar`（`publicsuffix`）做比對，每一個收到的 cookie 另外寫進 `cookies.json`（`0600`、原子寫入），session cookie 也存，免得每次啟動重登。`Site` 是可註冊網域（IP 與 localhost 就是自己），`SiteCookies` / `ClearSite` 給 Cookies popup；`Clear` 給 Settings 的 `C`。值不進 App Log。
- **Read**：先整份轉成 UTF-8 留在 `Doc.Source`（Source popup），不是 HTML 的照原樣當一個 code block；HTML 先 `absolutize`（照 `<base>`）、讀 `<title>` 與 `<link rel=icon>`、`clean` 拿掉看不到的，Readability 挑得到就用（`Picked = "article"`），否則整頁減掉 `chrome`（導覽、頁首、頁尾、側欄）。`Whole` 是整個 `<body>`（`w`）。
- **tree.go**：HTML → 內容樹。連結沒字就不畫、圖片帶 alt、清單有 `Marker`、表格分排版用與資料、表單的欄位與表單本身另存一張表（`Doc.Field`），同一個欄位在正文與整頁裡是同一份。

#### [2] 的畫法（`readerrender.go`、`pagebody.go`）

`renderReader` 把內容樹畫成 `layout{rows, items, marks}`：block 斷行、inline 流式折行，每個 `seg` 記它是哪個 item；`row.ground`／`ink` 是整列的底色與字色，`row.chip` 是標題（字放在該層顏色上、左右各一格空白，不用膠囊的圓端：那是 tdp 的 title，P4）。清單一律短橫線，編號 1／a／i 輪流；code block 在 Mantle 底上、照語言上色、右上角寫語言；引用是 `▎` 加 Mantle 底；資料表格畫格子、表頭粗體；連結與圖片前面是 icon。行號（`lineNumW`／`lineNum`）從文字寬扣、面板寬不變，顏色是 Overlay0。`/` 找到的字加底線、目前那一筆是 cursor 色塊（`writeFound`）。

#### 麵包屑（`readerkeys.go` 的 `openBreadcrumb`）

照 kbu 的 Breadcrumb（user 2026-10-29）：[2] 的 panel 動作 `b`（`B` 是 Bookmarks 畫面鍵），歷史少於兩頁時 `Hidden`；開 lib 的 `Menu`，一列一頁（`tab.hist`，每一筆記網址、標題、離開時的位置），`1. 標題`，目前那一頁 `where the tab is now`、選了只是關掉選單，其他 `go to it`、`Clear`，選了就 `readerGo(i - histAt)`，不先問。原本是網址列下一列的位置列（`A › B › C`），user 說不好用，換掉了。

#### 按鍵（`readerkeys.go`）

閱讀時 `j|k|u|d|gg|G` 捲動（像 `less`，`readerScroll`）；`f` 進 Focus mode（lib 的 panel mode），`j|k` 在連結、圖片、欄位、按鈕之間走，`Enter` 交給 `readerEnter`；`/` 開 lib 的 TextInput，`n|N` 走找到的，`Esc` 收起來（`pageEsc`）；`H|L` 是分頁自己的歷史（`readerGo`，回到離開時的位置）；`o` 網址框；`w` 整頁與正文互換。

#### 這一頁的資訊（`pageinfo.go`）

[2] 的 Space menu 最上面三項，沒有熱鍵，各是 lib 的 viewer（照 tdp T1，從 menu 開的東西 menu 留在底下）：Source（`Doc.Source`，照 media type 上色）、Cookies（`Jar.SiteCookies`，viewer 的 `C` 先 Confirm 再 `ClearSite`）、Requests（`Doc.Requests`，照 `curl -v` 的 `>`、`<` 寫法）。

#### 網站記號（`siteicon.go`）

[1] 每列開頭：Nerd Font 有那個網站的 icon 就用（`siteGlyphs` 依網域結尾比對），沒有就地球；顏色是 favicon 的主色（`reader.Client.IconColor`：PNG、GIF、JPEG、ICO 裡的 PNG 或 1／4／8／24／32-bit bitmap，看得到的像素平均、接近黑的往白調一半），每個網站一個 session 問一次（`favTried`），拿到前是 Blue。游標那一列的記號照 color.1 換成 Base（lib 畫的）。

#### 兩套配色（`theme.go`）

外殼的顏色（框、膠囊、cursor、hint、footer、popup 的層色）與淡化都是 terminu-go 的（tdp color）：失焦的 panel 與 popup 底下的一切由 lib 改寫 SGR 淡化（tdp F8、tdp T2）。`theme.go` 只有頁面自己的配色（Page palette：`pageText` / `pageClick` / `pageInput` / `pageCode` / `pageMedia` / `pageInvalid` / `levelInk`…）與頁面內容用的 glyph；網址列是 [2] 的位置列（tdp panel.10，terminu 2026-10-29 確認）：Lavender、太長從前面截、只能看；行號 Overlay0。頁面內容用 lip gloss 上色，lib 淡化時照樣改得到它的 SGR。

## 設計決定

- **閱讀器**（user 2026-10-28）：[2] 只呈現這一頁直接給的內容，網站外殼不顯示，畫成 webu 自己的元件；markdown 只是中間的一步（現在剩 `Yank markdown`）。規格、元件、按鍵、表單做到哪都在 [`reader.md`](reader.md)。
- **Chromium 完全拿掉**（user 2026-10-29）：原本打算留給靠 JavaScript 才有內容的空殼頁，user 決定連那些也不要；換來的是不用下載、不用跑瀏覽器，也有 linux/arm64 的版本。
- **DevTools 縮成三個 popup**（user 2026-10-29）：不叫 DevTools、畫面整個拿掉；留得下來的（Source、Cookies、Requests）放在 [2] Space menu 的最上面，沒有熱鍵。Console、web storage、樣式／script／圖片的請求沒有了：沒有 JavaScript，webu 也不抓那些。
- **畫面建在 terminu-go 上**（2026-10-18，tdp E8）：框、膠囊、Space menu、global operation popup、key reference、hint、footer、confirm、toast、每一種 input、viewer、splash、淡化、icon 與 powerline 符號的寬度（探測、`WEBU__ICON_WIDTH`／`WEBU__POWERLINE_WIDTH`、`TERMINU__` 的兩個）全部是 lib 的。webu 只畫 [2] 裡面的頁面。`go.mod` 寫 terminu-go 的 tag，開發時用不進版控的 `go.work` 接 `../terminu-go`。
- **[2] 是 [1] 的明細**（2026-10-18 暫定，到現在沒有重談）：[1] 的游標移到哪個分頁，[2] 就顯示哪個（還沒載入的分頁換到才載）；[1] 的 `Enter` 用 lib 的 ToDetails 把 focus 送到 [2]（tdp K3 的明細）。
- **把使用者帶到新分頁**（tdp K3：focus 只在使用者預期會換的時候換）：`T`、連結的「開新分頁」，使用者按下去就預期看到那一頁，所以換到 Web 畫面、focus 到 [2]（lib 的 `App.ShowScreen`、`App.Focus`）；載入完成這種背景的事不搶 focus。書籤與歷史的 `Enter` 在背後開新分頁、留在清單上，toast 說開在哪裡。
- **從 menu 執行之後**（tdp T1）：打開東西來看、選一個值寫回、新增的，menu 留在底下；要去別的地方的 —— `T` 新分頁、`o` 網址、書籤的 `m` 搬到別的目錄 —— 設 `Action.Clear`，開的時候清掉 menu。
- **最低尺寸 60×20**（tdp L1，app 可以調低）：家族的標準是 60×32，最低尺寸不能比它大；webu 在 60 欄兩個 panel 並排（窄寬的門檻 `narrowW` 照 tdp screen.5 不超過 60，所以 webu 畫得出來的寬度都是並排），60×20 實機看過照樣好用，也放得下加書籤、登入這些表單，常見的 80×24 與分割視窗都能用。再小，lib 只畫名稱、`needs a terminal of at least 60×20` 與 `Press [Ctrl-C] twice to quit`（tdp screen.7）。
- **清單的欄**（tdp panel.5）：長的值（分頁、書籤、歷史的標題與 URL，下載的檔名與狀態）標 `Column.Shrink`，分短欄剩下的寬度、照自己的截法截；短欄（When 之類）照最寬的值，不縮。長欄縮到最少（12 格）還放不下，才照 `Column.Drop` 整欄收：歷史的 When。
- **App Log 記使用者做的事**（tdp note.7）：每個從 menu、熱鍵或 `Enter` 跑的操作，只是打開來看的也算，用 `did` 記一句過去式（`Reloaded …`、`Followed the link to …`、`Shut the folder …`）；跳了 toast 的不另外記，toast 自己就進去了；切換畫面由 lib 記 `Switched to …`。不記的：移動 cursor 與 focus、Space menu、`?`、`!`、清單的 `/` 篩選、進明細（[1] 的 `Enter`）、在 confirm 或輸入框按 `Esc` 取消的（`Done` 不會跑）、進入 visual mode；打進欄位的值也不記（可能是密碼），只記 `Typed into <欄位>`。
- **成功做完的用 Success toast**（tdp toast.2）：Copied、Saved、Opened、Added、Imported、Renamed、Cleared、搬了目錄、`… saved to config.yaml`；中性的告知（`Downloading`、開在新分頁、搬家）是 Info，失敗是 Error。
- **visual mode 是 [2] 的 mode**（tdp K11）：lib 畫模式名與模式色、`Space` 開模式的 Space menu（`v`、`y`、`Enter` 標 `Item`，放在 ITEM OPERATION）、`Tab` 與數字鍵暫停（`Pause`）、`?` 列模式的鍵；選取中是 lib 的一層（`EnterLayer(selectLayer())`，terminu-go v1.1.1），模式名 `Visual>select`、`Esc` 是 `unselect`：lib 只退這一層、跑它的 `OnExit` 清掉選取，回到 `Visual`，再按 `Esc` 才離開；選取中 `v` 不作用、不會退回 —— 退回只靠 `Esc`（tdp note.4「Once selecting, `v` does nothing」、K7 別名要完整），這一層的鍵沒有 `v`，hint 與 ITEM OPERATION 也就沒有；`y`、`Enter` 用 `LeaveMode` 一次離開整個模式；模式裡的鍵（`h|j|k|l`、`w|b|e`、`0|$`、`u|d`、`gg|G`、`v`、`y`、`/`、`n|N`、`Enter`）是 `ModeKey`，做的事是 `selectMode` 的。整行選取的 `V` 拿掉了（user 2026-10-29：照 lib 的 viewer），模式裡按 `V` 跟別處一樣開 splash（tdp S1）。`/` 開 lib 的 TextInput 問要找什麼，`n|N` 在結果間走。
- **搜尋不加顏色**（2026-10-19 user 定，寫進 tdp color 的 cursor 與 tdp note 的搜尋）：命中的字加底線、保留原本的顏色；目前那一筆是 panel 的 cursor 色塊（Subtext1 底、Base 粗體）；模式色 Yellow 只給選取範圍。閱讀時的 `/` 也一樣。
- **書籤是資料，上次的分頁是狀態**（2026-10-19，tdp E3「刪掉會怎樣」；書籤的位置 user 定）：書籤刪了找不回來，是資料，放 `~/.webu/datas`；`session.yaml` 刪了只是回到空的開始畫面，是狀態，放設定目錄，`WEBU__STATE` 可以另外指定。`cookies.json` 刪了只是要重新登入，也是狀態。都是可以手改的檔。
- **visual mode 是小寫 `v`**：`V` 保留給 splash（tdp S1）。
- **[1] 與 [2] 的熱鍵各自歸各自的 panel**（tdp P3、tdp M4）：`H`／`L`、`b`、`o`、`/`、`n`／`N`、`f`、`v`、`w` 作用在頁面上，只在 [2] 按得到；`T`（新分頁）兩邊都有。全域動作只有切畫面（`W`／`B`／`H`／`D`／`S`）與離開。
- **離開只在有下載進行中時先問**（tdp K9 的離開流程由 app 決定）：`syncQuit` 依下載數設 `App.Quit`。
- **Settings 是一列一個值的 panel**（lib 的 `Values`）：文字設定開 TextInput（目前生效的值當灰字提議，`Backspace` 再 `Enter` 是預設）、`input_tab_width` 開 NumberInput、`download_dir` 開 file-picker 選目錄（從最近一層存在的目錄開始）、開關是原地翻的 checkbox，設好就存（tdp panel.12）。數字帶單位、寫完整的字（`4 spaces`、`96 cells`，tdp number.2）；`input_tab_width` 的輸入框打字列下面有單位列（`NumberInput.Unit`），清空就是預設（`Optional`）。設定名稱只是一兩個字，`?` 最前面有「每一列」一節寫每個 key 管什麼（`Field.Desc`，tdp panel.12）。家族共用的 `hide_secrets_in_preview`、`input_tab_width`（tdp E3）也在這裡，交給 lib 的 `App.HideSecretsInPreview`、`App.InputTabWidth`。`C` 清除所有登入（`cookies.json`）。
- **預設搜尋是 DuckDuckGo 的 HTML 端點**，因為 Google 對程式送來的搜尋回 reCAPTCHA。

## 已否決，不要重提

完整理由在設計文件原地、各自標日期；這裡是索引。

| 否決的做法 | 改成 | 出處 |
|---|---|---|
| Chromium 留給靠 JavaScript 才有內容的空殼頁 | Chromium 完全拿掉（user 2026-10-29） | `reader.md` |
| DevTools 畫面（Network、Storage、Console、Source） | [2] Space menu 的 Source、Cookies、Requests（user 2026-10-29） | 本檔「設計決定」 |
| 標題用膠囊（圓端） | 底色加左右一格空白（user 2026-10-28：膠囊是 tdp 的 title，P4） | `reader.md` |
| 選取中再按 `v`（或 `V`）退回、`V` 整行選取 | 退回只靠 `Esc`、只有 `v`（user 2026-10-29：照 tdp 與 lib） | 本檔「設計決定」 |
| `Enter` 開該 item 的選單（2026-09-20） | `Enter` 是左鍵（09-21）；閱讀時是 Focus mode 的 `Enter` | `ux.md` §A.0.K |
| webu 自己的 popup、Space menu、淡化、寬度函式、splash（到 0.5.0） | terminu-go 的（2026-10-18，tdp E8） | 本檔「設計決定」 |
| `bookmarks.yaml` 跟設定一起放 config（2026-09-21）、`session.yaml` 放 data | 照 tdp E3 分：書籤在 data、session 在 config（2026-10-19） | 本檔「設計決定」 |

## 已知的牆與未做

webu 不跑 JavaScript，所以靠 script 才長出內容的網頁（app、dashboard、很多社群網站）看到的很少或什麼都沒有；網頁自己的對話框與 frame 不顯示；需要 JavaScript 或別的網站帳號的登入、CAPTCHA、兩步驟驗證都不行。

還沒做的（細節在 [`reader.md`](reader.md)「還沒定的」）：

- 挑正文的規則：中央社這類清單頁會挑到選單、Wikipedia 的 infobox
- 不在表單裡的 `<button>`：沒有 JavaScript 時多半什麼都不會發生，目前只 toast 說明
- 媒體（圖片、影片、音訊只有 icon 與網址）
- 頁面名字自帶 Nerd Font glyph 時看起來像多一格空白 —— 先放著
- `docs/demo.gif` 還是 Chromium 時期錄的：等 app 確認好、發版前再重錄（user 2026-10-29）
- 滑鼠：不做，terminu 家族只用鍵盤（2026-10-19 定案）

webu 照 tdp v0.7.4 與 terminu-go v0.1.13 重構（2026-10-18），再對齊 tdp v0.8.0、升到 terminu-go v0.2.0、v0.2.1、v0.2.2（tdp v0.8.3）（2026-10-19），再對齊 tdp v0.9.0、v0.9.1，升到 terminu-go v0.4.0、v0.4.2（2026-10-20），再對齊 tdp v0.9.2、v0.9.3，升到 terminu-go v0.4.3、v0.4.4（2026-10-21），再對齊 tdp v1.1.0、升到 terminu-go v1.1.0（2026-10-28），升到 terminu-go v1.1.1（2026-10-29，Visual>select 改用 lib 的 `EnterLayer`）。2026-10-28 起改成閱讀器，2026-10-29 拿掉 Chromium 與 DevTools 畫面。tdp 交給 webu 自己決定、webu 定得跟別的成員不一樣的地方列在下一節。

## tdp 交給 webu 決定的地方

目前沒有。Chromium 時期有一條：頁面自己的彈窗是 [2] 的內容、不是 webu 的 popup（tdp F1）；它隨 Chromium 拿掉了（2026-10-29）。

## 設計文件導讀

| 檔案 | 回答什麼 | 順序 |
|---|---|---|
| [`reader.md`](reader.md) | 閱讀器：`net/http` 取頁、挑正文、內容樹、元件與按鍵、表單做到哪、這一頁的資訊、階段；跟下面三份衝突的以它為準 | 最先 |
| [`function.md`](function.md) | 定位與牆、資料流、做到哪（完整度矩陣）、Shell 功能清單、已決事項、驗收、CLI | 第 1 |
| [`ui.md`](ui.md) | 版面、畫面 chip 列與兩個面板、[2] 怎麼畫、popup、配色、存檔 | 第 2 |
| [`ux.md`](ux.md) | core-key 語意、閱讀／Focus mode／Visual mode、每個焦點的 `Space` 選單、欄位與表單、熱鍵全表、時間軸 | 第 3 |

設計本身 —— 包含試過而被否決的做法 —— 每條決定都在原地標了日期。

## 建置與開發

### 用什麼做的

Go、家族的 [terminu-go](https://github.com/vulcanshen/terminu-go)（建在 [Bubble Tea](https://github.com/charmbracelet/bubbletea) v1 上；頁面內容用 [Lip Gloss](https://github.com/charmbracelet/lipgloss) 上色）、`net/http` 與 [`golang.org/x/net`](https://pkg.go.dev/golang.org/x/net) 的 `html`、`html/charset`、`publicsuffix`、挑正文用 [go-readability](https://codeberg.org/readeck/go-readability)（v2，Firefox 閱讀模式的 Go 版，MIT）、語法色用 [chroma](https://github.com/alecthomas/chroma)。

### 從原始碼建置

```bash
git clone https://github.com/vulcanshen/webu.git
cd webu
make build     # → ./webu（CGO_ENABLED=0、-trimpath、strip；版本是 dev）
./webu
```

開發用的小工具放在頂層的 `tools/`（tdp project 允許，不算 app 本身）：`tools/mddump` 是閱讀器的實驗工具（`make mddump URL=…`，有自己的 `go.mod`）：`net/http` 抓一頁、Readability 挑內容、`html-to-markdown/v2` 轉 markdown 印出來，當基準量 webu 自己的規則；`-whole` 照轉整個 `<body>` 對照，`-save`／`-file` 存下與重放 HTML。維護者自己的 Claude Code 檔案（`CLAUDE.md`、`.claude/`）照 tdp project.3 不進 git。

同時改 terminu-go 時，在 repo 旁邊放 `../terminu-go`，用不進版控的 `go.work` 接上：`go work init . ../terminu-go`。`make check` 有 `go.work` 時會多跑 `check-lib`（關掉 `go.work`、照 `go.mod` 寫的 tag 編譯與測試），過不了就是用到了 lib 還沒發版的東西。

```
make build              → ./webu
make install / uninstall → $GOBIN
make test / test-race   全部測試，不用網路（閱讀器的測試用 httptest）
make check              commit 前：fmt-check、vet、test-race，有 go.work 時加 check-lib
make mddump URL=https://…                                 閱讀器的實驗：一頁的內容印成 markdown
make gif                從 .local/demos/demo.tape 重錄 docs/demo.gif（VHS、Nerd Font、網路）
make snapshot           goreleaser 本機打包到 dist/
```

`make` 列出全部：家族共用的 target 在 `family.mk`（tdp project 的範本，不在 webu 裡改），webu 自己的（`mddump`）在 `Makefile` 最後一段。

- **`go test ./...`**：reader（取頁、轉址與請求紀錄、cookie 存檔與一個網站的 cookie、表單、favicon 的顏色）、store、paths（搬家、刪掉舊的 Chromium）、ui。
- **閱讀器的 UI 測試**（`internal/ui/reader_test.go`）：`httptest` 架一個小網站，走過讀頁、跟連結、`H|L`、`w`、`/` 找字、Focus mode、表單、登入的 cookie（`0600`）、搜尋框、下載、404、網站記號、Source／Cookies／Requests 三個 popup。
- **tdptest**（`internal/ui/tdp_test.go`，terminu-go 的 `tdptest`）：`TestTDP` 照第一次啟動在各種尺寸、icon 寬度走過每個畫面與 panel 的 Space menu、global operation popup、App Log、`?`，再縮到最低尺寸以下（tdp L1、L4、E5）；`TestTrickyTextFromOutside` 把 `tdptest.Tricky` 放進分頁標題、頁面文字與書籤，每個 frame 都驗沒有控制字元送到終端機。webu 自己的計時器用 `App.Tick`，tdptest 才推得動時間。
- **測試 driver**（`app_test.go`）：把訊息一個一個交給 lib 的 `App.Update`、在 goroutine 裡跑 command；`d.view()` 是去掉樣式的 `App.Frame()`，popup 開沒開看它的標題（例：`LOCATION`）；`d.press` 是按鍵後等開關動畫跑完（lib 在 popup 開完之前不收鍵），`Space` 寫成 `" "`；`d.action(label)` 直接跑 focus 的 panel 的某個動作。`toast` 還在時第一個 `Esc` 先收 toast（tdp K4），測試要算進去。測試用的小頁面用 `nodes_test.go` 的 `doc`、`hd`、`para`、`text`、`link` 建；分頁要有 `doc`（`reader.Doc`），[2] 的動作才不會當成沒有頁面。
- **重現 TUI bug** 用 Go driver；要手動跑 `./webu` 重現時，把 `WEBU__CONFIG`、`WEBU__DATA`、`WEBU__STATE`、`WEBU__CACHE` 指到暫存目錄，不碰自己的設定、書籤與登入。
- **glyph 碼位**用 `fontTools` 讀已安裝 Nerd Font 的 cmap 查，不憑記憶；原始碼裡寫成碼位（`""`、`"\U000f0198"`），不放字本身（tdp E4）。
- **demo gif**：`make gif`（family.mk，tape 在 `.local/demos/`）；vhs 0.12.0 曾在 macOS 上 2 秒就結束、不出檔也不報錯，0.12.1 正常。
- **實機截圖**：照 terminu-go 的 `scripts/tmux-shot.sh`，用自己的 tmux server（`tmux -L …`）與 `codepoint-widths` 試 icon 兩格；設定、狀態、資料、快取都指到暫存目錄。

## 發布

照 tdp project 的「發布」：`make check` 通過 → CHANGELOG 的 `## Unreleased` 改成 `## vX.Y.Z — tdp vA.B.C` → commit、打 tag `vX.Y.Z` → push commit 與 tag。`release.yml`（範本）在 ubuntu 與 macOS 跑 `go test -race`、goreleaser 建置並推 Homebrew tap（`vulcanshen/homebrew-tap` 的 `webu.rb`），再用 CHANGELOG 那一段蓋過 release 的內文（awk 抓 `## <tag>` 那一段）。0.5.0 以前的 CHANGELOG 標題照當時的寫法（`## [0.5.0] — 2026-10-06`），不改。

- **平台**：Chromium 拿掉之後沒有平台限制，`.goreleaser.yaml` 範本留給 app 的 `ignore` 是空的（`[]`），linux/arm64 也建；`install.sh` 在 app 自己的段不擋任何平台。
- **token**：goreleaser 讀 `GITHUB_TOKEN`；workflow 在 secret `ACTION_TOKEN` 有設時用它（內建的 token 寫不了 tap）。
- **`make release-check`**：goreleaser 把 `brews` 標成 deprecated，只有這個警告不算失敗。
- 發布後確認資產時看 `repos/<owner>/<repo>/releases/<id>/assets`；`gh release view` 與 `releases/tags/<tag>` 的 assets 剛發布時可能還是空的。**不要對已經成功的 run 重跑 release job** —— goreleaser 會因為資產已存在而失敗；真的要重來，先刪掉資產再重跑。
- 發版前 `check-lib` 一定要過：`go.mod` 寫的 terminu-go 必須是打過的 tag。
