# webu — UX

> 本文件講**互動語意**：core key、每個焦點的 Space menu、閱讀與兩個 mode（Focus、Visual）、頁面上的欄位怎麼填、
> 熱鍵全表、浮層行為、時間軸。閱讀器本身（取頁、挑正文、元件的畫法、表單做到哪）在 [`reader.md`](reader.md)，
> 版面在 `ui.md`，設計理由與實作在 [`dev-remarks.md`](dev-remarks.md)。
> 依 [terminu design principle](https://github.com/vulcanshen/terminu)（tdp v1.1.0）撰寫，畫面建在 terminu-go（v1.1.1）上；
> §A、§B 是 tdp 的對照，各條標出對應的 tdp 條目；每條決定標日期。

> **一份登記、lib 產生其餘**（2026-10-18）：webu 把每個畫面、panel、動作（鍵、名稱、說明、範圍、要不要先問、做什麼）
> 登記給 terminu-go；core key、Space menu、global operation popup、`?` 的 key reference、panel 下框的 hint、footer、
> confirm、變暗、`Esc` 一次一層都由 lib 從這份登記產生。本文件寫的是 webu 登記了什麼、各自做什麼。

---

## §A. tdp in webu

### §A.0 揭露對照（tdp M1、tdp P2）

| Track | 入口 | 入口自身怎麼被揭露 | 完整性 |
|---|---|---|---|
| **Contextual** | `Space` | footer 常駐 `Space:menu` | 當前 focus 的 contextual 動作 100% 在 Space menu 內；在 mode 裡（Focus、Visual）是 mode 的 Space menu，只列 mode 的鍵（tdp K11） |
| **Non-contextual** | `?` | footer 常駐 `?:help` | `?` 是最前端 surface 的 key reference，唯讀；全域動作 100% 在 global operation popup 內、可直接執行，由每個 Space menu 最後一列打開；header 五個 screen 的 chip 是 ambient 揭露 |

**規則（2026-09-22）：一個面板操作沒進 Space menu 就等於不存在。** 2026-10-18 起由 lib 保證：熱鍵就是登記的動作的鍵，
不在選單裡的熱鍵不可能存在（tdp M3）。沒有熱鍵的動作（Source、Cookies、Requests、Yank markdown）只在 Space menu 裡。
label 裡怎麼標鍵照 tdp M5，由 lib 產生。

### §A.0.K core-key 語意（tdp K 章）

| Core-key | 閱讀（不在任何 mode） | Focus mode（`f`） | Visual mode（`v`） |
|---|---|---|---|
| `Tab`、`1\|2` | `[1]` ↔ `[2]`；`1\|2` 直達（tdp K1、tdp K2） | 不作用，toast 說先按 `Esc` 離開模式（tdp K11） | 同 Focus mode |
| `Enter` | **滑鼠左鍵在 terminal 的對應**（2026-09-21 定案）。`[1]`：`Read`，`[2]` 換成這個分頁、focus 送到 `[2]`（tdp K3 的明細）。`[2]`：閱讀時沒有被選的項目，`Enter` 不作用；要按頁面上的東西先 `f`（2026-10-28）。清單畫面與 Settings：各自的 `Enter`（§A.1） | 按下 focus 的項目，細表在 §A.0.K.1 | 按下字元游標所在的項目，並離開模式；那裡沒有可以按的，toast `Nothing to press here` |
| `Esc` | **往上一層，一次一步，到頂不作用**（2026-09-23，tdp K4）：先關最上層的浮層（toast 也算，tdp F3）；`[2]` 有 `/` 找到的字時收掉；清單畫面上清掉篩選，不回 Web（回 Web 用 `W`，2026-10-18）；其他時候不作用 | 離開模式，focus 的項目不再標出 | 選取中（`Visual>select`）放掉選取、回到 `Visual`，游標留在原地；否則離開模式（tdp K11、tdp note.4，2026-10-29） |
| `Space` | panel 的 Space menu（§A.1）；再按關閉；其他浮層上不作用（tdp K5） | **mode 的 Space menu**（tdp K11）：ITEM OPERATION 是 `Enter` Press、`o` Open in a new tab、`y` Copy its address；FOCUS MODE 是 `j\|k`、`gg\|G`（只列出）與 `Esc` Leave；`Esc` 只關這個 menu | **mode 的 Space menu**：ITEM OPERATION 是 `v` Select（選取中沒有：`v` 不作用，tdp note.4）、`y` Copy、`Enter` Press；VISUAL MODE 是移動鍵（只列出）、`/` Find、`n\|N` Next match 與 `Esc` Leave（選取中是 Unselect） |
| `?` | 最前端 surface 的 key reference，唯讀、可捲動；再按關閉（tdp K6、tdp M4，§A.2） | mode 的 key reference：mode 的鍵（tdp K11） | 同 Focus mode |
| `!` | App Log：webu 做了什麼、出了什麼錯（tdp K1） | 不作用（tdp K11） | 同 Focus mode |
| `q`、`Ctrl-C` | 離開流程（§A.2，tdp K9） | 同左 | 同左 |

**修訂史**：2026-09-20 實機試用後 `Enter` 曾改成「開該 item 的選單」，09-21 定案改回左鍵；2026-10-28 改成閱讀器後，
閱讀時沒有游標，`Enter` 只在 Focus mode 與 Visual mode 裡按頁面上的東西。

#### §A.0.K.1 Focus mode 的 Enter 對每種東西

Focus mode 的 `j|k` 走過的是可以按的項目：連結、圖片、欄位、按鈕（`reader.md`，2026-10-28）。

| 東西 | `Enter` |
|---|---|
| 連結 | **直接開，不先問**（`reader.md`）：指向別頁的在這個分頁載入，離開 Focus mode；指向這一頁的 `#錨點` 跳過去，那一列捲到最上面；指向的不是網頁時下載（§6） |
| 圖片 | 不按；toast 說這是圖片、怎麼複製它的網址 |
| 一行的欄位（`<input>` 不是勾選框、radio、按鈕、隱藏欄位的都是：text、email、search、number、date…） | lib 的 input popup，框裡是欄位目前的值；寫回後頁面重畫（§2.1） |
| 密碼 | lib 的 password input，不顯示舊值 |
| `<textarea>` | lib 的 textarea |
| 勾選框、radio | 原地翻：勾選框勾或不勾，radio 選這一個 |
| `<select>` | lib 的 select，列出選項、目前選的有標記 |
| 送出表單的按鈕（`submit`、`image`） | 送出這個表單（§2.1） |
| 其他按鈕（不在表單裡的、`type=button`…） | 不按；toast 說它要網頁的 JavaScript，webu 不跑 |
| disabled 的（檔案欄位也是：不上傳檔案） | 不按；toast 說它 disabled |

### §A.1 Contextual track — Space menu（tdp K5、tdp M2）

region 固定叫 `item operation` / `panel operation`，一律有標題，即使只剩其中一區（2026-09-27）。每個 Space menu 最後
是分隔線與固定一列 `Global operation`，**不加區塊標題**（2026-09-28，tdp M2），`Enter` 打開 global operation popup（§A.2）。
menu 本身：`j|k` 走（環繞）、`u|d` 半窗、`gg|G` 首尾（2026-09-23）、`Enter` 執行、letter hotkey 在 menu 裡也有效。

不能執行的列照樣出現、變暗，說明維持原本那句、不另寫原因，`Enter` 與熱鍵都不作用（tdp M6）；沒有目標時那一列或那一區
不出現，空清單沒有 item，item operation 區連標題一起不出現（tdp M2）（2026-09-27）。

**`[1]` Tabs**

| item operation | panel operation |
|---|---|
| `Enter` Read（`[2]` 換成這個分頁、focus 到 `[2]`）、`c` Close、`o` Open in new tab（同一頁再開一個）、`r` Reload、`y` Yank URL | `T` Tab（新分頁，New tab 框，§2.2）、`X` Close others（只有一個分頁時變暗）、`U` Undo close（重開最後關掉的分頁，最多記 20 個；沒有關過的時變暗） |

**`[2]` Page — 閱讀時**（2026-10-28 起）：沒有被選的項目，所以沒有 item operation 區。panel operation 照 menu 的順序：

| 鍵 | label | 做什麼 | 變暗（tdp M6） |
|---|---|---|---|
| — | Source | 抓下來的原始內容（轉成 UTF-8），照 media type 上色 | 沒有頁面 |
| — | Cookies | 這個網站的 cookie；裡面 `C` 先問再清掉 | 沒有頁面 |
| — | Requests | 載入這一頁發出的請求，照 `curl -v` 的寫法 | 沒有頁面 |
| `R` | Reload | 這一頁 | 沒有分頁、導航中 |
| `T` | Tab | 新分頁，New tab 框（§2.2） | — |
| `H` | Back | 這個分頁自己的歷史，上一頁 | 沒有上一頁、導航中 |
| `L` | Forward | 下一頁 | 沒有下一頁、導航中 |
| `b` | Breadcrumb | 麵包屑選單（下面） | 導航中；歷史只有一頁時不出現 |
| `o` | Open | Location 框（§2.2） | — |
| `/` | Find | 找字（§1.1） | 沒有頁面 |
| `n`、`N` | Next match、Previous match | 找到的字的下一個、上一個 | 沒有在找 |
| `f` | Focus mode | §1 | 沒有頁面 |
| `v` | Visual mode | §1 | 沒有頁面 |
| `w` | Full page／Content only | 整頁（網站的外殼也在）與正文互換，label 跟著換（2026-10-28） | 沒有頁面 |
| `A` | Add bookmark | 這一頁直接加進書籤的最上層；已經有的 toast `Already bookmarked` | 沒有頁面 |
| `Z` | Zoom | `[2]` 獨佔畫面，再按回到兩個 panel | — |
| `Y` | Yank page url | 這一頁的網址 | 沒有分頁 |
| — | Yank markdown | 畫面上這一頁的內容（正文，或 `w` 之後的整頁）寫成 markdown | 沒有頁面 |
| `C` | Close | 這個分頁 | 沒有分頁 |

「導航中」是按了要去別的頁面、新的還沒到（§6）。`[2]` 下框的 hint 寫 `H:back L:forward`（user 2026-10-28：不然不知道怎麼上下頁）。

**這一頁的資訊**（user 2026-10-29，取代 DevTools 畫面）：Source、Cookies、Requests 放在 menu 最上面，沒有熱鍵，各開一個
lib 的 viewer（捲動、`/`、`v` 選取複製都是 viewer 的）。Cookies 列的是這個網站（可註冊網域，`en.wikipedia.org` 算
`wikipedia.org`；IP 與 localhost 就是自己）的 cookie，值只在這裡顯示、不進 App Log；`C` 先問（`Clear N cookies?`）再清掉。

**麵包屑**（user 2026-10-29，照 kbu）：`b` 開一個選單，一列一頁，列出這個分頁走過的每一頁（整條歷史，`L` 會去的也在），
`1. 標題`（沒有標題就是網址）。目前這一頁寫 `where the tab is now`，選了只關掉選單；其他寫 `go to it`，選了直接過去、
不先問（跟 `H|L` 一樣），回到離開那一頁時的位置。

**`[2]` Page — Focus mode**（2026-10-28）：mode 裡只有 mode 的鍵（tdp K11），`[2]` 的 panel operation 與全域熱鍵都不作用，
要用先 `Esc`。mode 的 Space menu：

| item operation | Focus mode |
|---|---|
| `Enter` Press（§A.0.K.1）、`o` Open in a new tab（focus 的連結或圖片的網址，開新分頁並換過去）、`y` Copy its address（同一個網址） | `j\|k` 下一個、上一個可以按的，`gg\|G` 第一個、最後一個（只列出）；`Esc` Leave |

**`[2]` Page — Visual mode**：mode 的 Space menu 見 §A.0.K，鍵見 §1。

**screen 內部**

| screen | item operation | panel operation |
|---|---|---|
| Bookmarks | 書籤：`Enter` Open（在背後開新分頁、留在清單上，toast 說開在哪）、`m` Move（選別的目錄）、`r` Rename、`y` Yank（URL）、`x` Delete（先問）；目錄：`Enter` 開合、`r` Rename、`x` Delete folder（先問；裡面有東西時寫出有幾個書籤、幾個目錄） | `a` Add（form，§2.1；加在游標所在的目錄）、`A` Add folder（`a/b/c` 一次建每一層，建在游標所在的目錄裡）、`I` Import（file picker 從 `~/Downloads` 起選瀏覽器匯出的書籤檔，再輸入要放進的目錄名：必填、不能跟現有的同名）、`/` Filter |
| History | `Enter` Open（在背後開新分頁）、`y` Yank（URL）、`x` Delete（這一筆） | `C` Clear（先問；沒有紀錄時變暗）、`/` Filter |
| Downloads | `Enter` Open（用桌面的 app 開檔；只有下載完成的列，其他變暗）、`o` Open source（下載的網址開新分頁）、`y` Yank path（存在哪；還在下載時是 URL）、`x` Remove（這一列；還在下載的停掉） | `C` Clear（已完成與已取消的列；沒有這種列時變暗）、`/` Filter |
| Settings | `Enter` Change（文字設定開輸入框、目前生效的值當灰字提議，`Backspace` 再 `Enter` 是預設；`input_tab_width` 開數字框；`download_dir` 選目錄；開關原地翻；設好就存） | `C` Clear sign-in data（先問；清掉所有網站的 cookie） |

delete 用 `x` 不用 `d`：`d` 是半頁（tdp K12）。

### §A.2 Non-contextual track — `?` key reference 與 global operation（tdp K6、tdp M4）

**`?` 只讀**（2026-09-27）：打開最前端 surface 的 key reference —— 唯讀、可捲動、沒有游標、不能執行。
面板上是這個面板的鍵：lib 從它此刻的 Space menu 產生，照 menu 的區塊標題分組，再接移動鍵與 core key；Space menu 裡變暗的列，
這裡也列出、變暗（2026-09-29，tdp M6）。Settings 的 `?` 最前面有「每一列」一節，寫每個 key 管什麼（tdp K6）。
浮層上是那個浮層自己的鍵；mode 裡是 mode 的鍵（tdp K11）。`Esc` 或再按 `?` 關掉。

**全域動作在 global operation popup**（2026-09-27，tdp M4、tdp F4、tdp T1）：每個 Space menu 最後一列 `Global operation` 打開它，
**疊在 Space menu 上**；`Esc` 回到 Space menu，執行一列就關掉整疊。目前所在畫面的那一列照 tdp M6 變暗、不作用，不拿掉。

| 全域動作 | 鍵 |
|---|---|
| screen | `W` Web、`B` Bookmarks、`H` History、`D` Downloads、`S` Settings |
| 離開 | `q\|Ctrl-C` |

**離開**（tdp K9）：只有下載進行中時先問（`Quit webu?`，寫出還有幾個下載、離開會停掉它們）；有浮層開著時第一個 `q` 或 `Ctrl-C`
不直接離開、先問；問的時候再按 `Ctrl-C` 立刻離開。`q` 在輸入框裡是字（tdp K8），在 form 上不作用（tdp K9）。

**只有作用在 app 上的才是全域動作**（2026-09-27，tdp M3、tdp P3）：`H|L`、`b`、`o`、`/`、`n|N`、`f`、`v`、`w` 作用在頁面上，
是 `[2]` 的 panel operation，只在 `[2]` 按得到；`T` 兩個 panel 都有。切面板的 `Tab`、`1|2` 是 core key。
splash 彩蛋 `V` 不揭露（tdp S2）。

全域熱鍵在沒有浮層開著的 panel 上可以直接按（tdp M4）；跟 panel 自己的熱鍵撞時 panel 的贏：`[2]` 上的 `H` 是 Back
（§4 撞字）。mode 裡、打字中不作用（tdp K11、tdp K8）。

---

## §B. 元素專職化（tdp P4）

| 元素 | 唯一語意 |
|---|---|
| 邊框 Blue 粗線 | focus（tdp panel.1） |
| `[1]` 綠字 | `[2]` 正在顯示的分頁 |
| `[1]` 灰字 | 從上次 session 還原、還沒載入的分頁（切到才載，§6） |
| `[1]` 每列開頭的記號 | 那個網站：Nerd Font 有它的 icon 就用，沒有是地球，顏色是 favicon 的主色；載入中換成轉圈的記號（2026-10-28） |
| 邊框 Yellow、上框右側寫模式名 | 在 mode 裡：`Focus`、`Visual`、`Visual>select`（tdp K11） |
| `[2]` 網址列 Lavender | 位置列：這個分頁在哪一頁，只能看（tdp panel.10，2026-10-29） |
| 字加底線 | `/` 找到的字；目前那一個是 cursor 色塊（tdp color.1；搜尋不加顏色，2026-10-19） |
| 下框右側 | 捲動位置：閱讀時是看得到的列 `N-M of T`（tdp panel.1） |
| glyph | 一種東西一個 glyph（連結、圖片、欄位…）；碼位查字型不憑記憶（tdp E4） |
| `y` / `Y` | 游標下的東西 / 整頁 |
| `r` / `R` | `[1]` 游標下的分頁 / `[2]` 這一頁 |
| `c` / `C` | `[1]` 游標下的分頁 / `[2]` 正在顯示的分頁 |

---

## §1 閱讀、Focus mode、Visual mode

2026-10-28 起（`reader.md` 的按鍵，user 定）`[2]` 有三種狀態：

| | 閱讀（不在任何 mode） | Focus mode（`f`） | Visual mode（`v`） |
|---|---|---|---|
| 游標 | 沒有：頁面捲動，像 `less` | 可以按的項目：連結、圖片、欄位、按鈕 | 字元 |
| 移動 | `j\|k` 一列、`u\|d` 半頁（`PgUp\|PgDn` 也是）、`gg\|G` 頭尾 | `j\|k` 下一個、上一個，頭尾繞回（tdp K12）；`gg\|G` 第一個、最後一個 | `h\|j\|k\|l`、`w\|b\|e`、`0\|$`、`u\|d`、`gg\|G` |
| 進入時 | — | focus 畫面上第一個項目；頁面沒有可以按的，toast `Nothing to press on this page` | 字元游標在畫面最上面那一列 |
| `Enter` | 不作用 | §A.0.K.1 | 按下字元游標所在的項目，離開模式 |
| 複製 | `Y` 網址、`Yank markdown` | `y` 連結或圖片的網址 | `v` 開始選、`y` 複製並離開（沒有選取時複製游標那一列） |
| `/` | 找字（§1.1） | — | 找字（§1.1） |
| 其他鍵 | `[2]` 的 panel operation（§A.1） | 只有上面的鍵與 `o`（開新分頁）；panel 與全域的熱鍵不作用（tdp K11） | 只有上面的鍵與 `n\|N`；`V` 跟別處一樣是 splash（tdp S1） |
| 框 | 有 focus 時 Blue | Yellow，上框右側 `Focus` | Yellow，`Visual`；選取中 `Visual>select` |
| 離開 | — | `Esc`；跟著連結走、送出表單、換到別頁時自己離開 | `Esc` 一次一層；`y`、`Enter` 一次離開整個模式 |

箭頭與 `j|k`、`h|l` 同（tdp K12）。兩個 mode 都暫停 `Tab` 與數字鍵（§A.0.K）。

**Visual mode 的選取**（tdp note.4；2026-10-29 改用 lib 的 layer）：第一個 `v` 進模式，第二個 `v` 從游標開始選，模式名變
`Visual>select`；選取中 `v` 不作用，退回只靠 `Esc`（tdp K7、tdp note.4）。整行選取的 `V` 拿掉了（user 2026-10-29：照 lib 的 viewer）。
`y` 寫系統剪貼簿：macOS 用 `pbcopy`，其他用 `wl-copy`、`xclip`、`xsel` 裡找得到的第一個，不用 OSC 52；都沒有時 toast 說要裝什麼。

### §1.1 找字（`/`）

2026-10-28 起 `/` 就是逐字找（`reader.md`）：

- **閱讀時**：`/` 開 lib 的 text input（`Find`），上次找的當灰字提議。找到的每一處加底線，目前那一個是 cursor 色塊（tdp color.1）；
  從畫面最上面那一列以下的第一個開始，捲到看得到。`n|N` 下一個、上一個，頭尾繞回。找不到時 toast `Not on the page: …`；
  空白送出就是不找。`Esc` 收掉（§A.0.K）；換頁時也收掉；終端機改寬重排後重找，目前那一個留在最近的位置。
- **Visual mode**：`/` 開同一種框，跳到字元游標起的第一個，`n|N` 在結果間走、頭尾繞回；Yellow 只給選取範圍（2026-10-19）。
- 兩邊都是 smart case：全小寫不分大小寫，有大寫就照大小寫。

---

## §2 文字輸入

### §2.1 頁面上的欄位與表單（2026-10-28，`reader.md`）

- 每種欄位 `Enter` 做什麼在 §A.0.K.1。輸入框都是 lib 的，行為照 tdp input 的各個元件；多行的 `Tab` 插 `input_tab_width` 格空白
  （tdp K8）。
- 欄位的值記在 webu 這邊（`reader.Field`），同一個欄位在正文和整頁裡是同一份；寫回後頁面重畫。打進欄位的值不進 App Log，
  只記 `Typed into <欄位>`。
- **送出**：按表單的送出按鈕，照 HTML 的規則組值：勾選框只在勾了時、按鈕只送按下的那個、不送檔案。GET 組進網址；POST 帶上隱藏欄位，
  收到的 cookie 存起來（登入）。
- **只有一個可以打字的欄位的表單**：填了值之後問 `Send the form, as Enter in the field does?` —— 頁面上在那一欄按 Enter 就是送出；
  `Esc` 不送，值留著。
- 做不到的：靠 JavaScript 的登入、第三方登入、驗證碼、兩步驟驗證（`reader.md`）。

打字中屏蔽所有 hotkey：`Space` 是空白、`?` 是問號（tdp K8）。

**送不出去時**（tdp F7、tdp K3）：框留著、打的字留著，原因寫在框的錯誤列 —— 新增目錄、匯入的目錄名、設定值都先檢查再關框。

**加書籤的 form**（Bookmarks 的 `a`，tdp form）：`URL`、`Title` 兩欄與 `Add` 按鈕，打開時填好 Web 畫面顯示中的那一頁；
`Enter` 開那一欄的輸入框，只有按鈕送出（tdp K3）。URL 空著（`A bookmark needs a URL`）或已經有這個書籤
（`Already bookmarked: …`）時送不出去，焦點回到那一欄；Title 空著就用 URL。`[2]` 的 `A` 不開 form，直接加（§A.1）。

### §2.2 Location（`o`）

`o`（2026-10-28：原本的 `L` 給了 Forward，跟 Vimium 一樣）開 Location 框：輸入列空的、目前分頁的 URL 灰字當提議：
`Tab` 接受、接進來編輯，`Backspace` 拒絕，直接打字從頭來。

`Enter`：有 `://` 的照原樣；`localhost`、`127.` 開頭的補 `http://`；網址的第一段沒有空白、有 `.`（不在結尾）的補 `https://`；
其他當搜尋，送到 `search_engine`（預設 DuckDuckGo 的 HTML 端點）。空的不做事。`o` 開在目前分頁（沒有分頁時開新分頁）；
`T` 的 New tab 框一樣解讀，沒有提議，開在新分頁。命令列給的網址也這樣解讀。

---

## §3 導覽詞彙

導覽字母 `h j k l u d g G` 不被任何動作佔用（tdp K12）。

| 鍵 | 動作 |
|---|---|
| `j\|k` | `[2]` 閱讀時捲一列；Visual mode 上下一列；Focus mode 下一個、上一個項目，清單與 menu 上下一列，這兩種頭尾繞回（tdp K12） |
| `u\|d` | 半頁 |
| `gg\|G` | 頭／尾 |
| `h\|l` | Visual mode 左右一個字元 |
| `w\|b\|e`、`0\|$` | Visual mode：下一個字、上一個字、字尾；列首、列尾 |
| `Esc` | 往上一層（§A.0.K） |
| `1\|2` | 直達面板 |

滑鼠：不做，terminu 家族只用鍵盤（2026-10-19 定案）。

---

## §4 Hotkey 分層與全表

bracket 印的就是要按的鍵，大小寫照按的（tdp M5）。

| 層 | 鍵 |
|---|---|
| core | `Space` `?` `!` `Tab` `1\|2` `Enter` `Esc` `q\|Ctrl-C` |
| 全域 | `W` `B` `H` `D` `S`、`q\|Ctrl-C`；`V`（splash，不揭露） |
| `[1]` item / panel | `Enter` `c` `o` `r` `y` / `T` `X` `U` |
| `[2]` 閱讀 | 移動 `j\|k` `u\|d` `gg\|G`（`PgUp\|PgDn`）；panel `R` `T` `H` `L` `b` `o` `/` `n` `N` `f` `v` `w` `A` `Z` `Y` `C`；menu-only：Source、Cookies、Requests、Yank markdown；`Esc` 收掉找到的字 |
| Focus mode | `j\|k` `gg\|G`、`Enter` `o` `y`、`Esc` |
| Visual mode | `h\|j\|k\|l` `w\|b\|e` `0\|$` `u\|d` `gg\|G`、`v` `y` `Enter`、`/` `n\|N`、`Esc` |
| Cookies viewer | `C` |
| Bookmarks | `Enter` `m` `r` `y` `x` / `a` `A` `I` `/` |
| History | `Enter` `y` `x` / `C` `/` |
| Downloads | `Enter` `o` `y` `x` / `C` `/` |
| Settings | `Enter` / `C` |

撞字：

- `H`：全域是 History，`[2]` 上是 Back —— panel 的熱鍵贏（tdp M4）；在 `[2]` 要去 History，從 global operation popup 或先到 `[1]`。
- `b` / `B`：`[2]` 的 Breadcrumb / 全域的 Bookmarks。
- `o`：`[1]` 是 Open in new tab，`[2]` 閱讀時是 Open（Location 框），Focus mode 裡是 Open in a new tab（focus 的那個）。
- `n|N`：閱讀時是找字的下一個、上一個；上一頁、下一頁因此用 `H|L`（2026-10-28，原本的 `P|N`）。
- `C`：`[2]` 關分頁，History、Downloads 是 Clear，Settings 是 Clear sign-in data，Cookies viewer 是清這個網站的 cookie。

---

## §5 浮層行為

浮層都是 lib 的（tdp F）：`Esc` 只關最上層（tdp F3、tdp K4）、`Space` 只開關 Space menu、在其他浮層上不作用（tdp K5）、
疊著時最上層以下都變暗（tdp F8）。

**離開的 confirm 疊在最上面**（2026-09-27，tdp K9）：有下載進行中時 `q|Ctrl-C` 問的那一題疊在整疊之上，不取代底下的；
`Esc` 回到原本的地方。

**source 留在底下**（2026-09-27，tdp F4、tdp T1）：從 Space menu、global operation popup 開出的框（confirm、輸入框、viewer、
麵包屑選單……）疊在 menu 上，menu 留在底下；`Esc` 回到 menu，完成動作才整疊清掉。

**要去別的地方的先清掉 menu**（tdp T1）：`T` 新分頁、`o` 網址、`/` 找字、書籤的 `m` 搬到別的目錄、麵包屑選了別的頁，
開的時候 menu 一起關掉；切畫面也是。

**先 confirm 的動作**（tdp F6）：有下載進行中時離開、Bookmarks 的 `x`（書籤與目錄都問）、History 的 `C`、Settings 的 `C`、
Cookies viewer 的 `C`、只有一個打字欄位的表單填完後的送出。

---

## §6 時間軸

| 情境 | 行為 |
|---|---|
| 導航 | 跟連結走、送出表單、`R`、`H\|L`、麵包屑、Location：從按下那一刻分頁在載入（`[1]` 的記號換成轉圈、`[2]` 的框上有 loading icon），畫面上那一頁留著，新的到了才換；導航中 `R`、`H\|L`、`b` 變暗，`Enter` 不作用，連按只算一次 |
| 到了 | 新的一頁從最上面開始、沒有被選的項目；網址有 `#錨點` 時那一列捲到最上面；Focus mode 與 `/` 找到的字收掉；狀態碼 400 以上照樣顯示內容，toast 說狀態 |
| 上一頁、下一頁 | `H\|L` 與麵包屑回到離開那一頁時的位置；`R` 留在原本的位置，不多一筆歷史 |
| 載入失敗 | 畫面上已經有這一頁的（`R`），頁面留著、toast 說沒有載成；沒有頁面可留的，`[2]` 寫出原因與 `Press [R] to retry`（tdp panel.15） |
| 不是網頁的 | HTML、純文字、JSON 以外的下載到下載資料夾：畫面上那一頁留著，Downloads 多一列，toast `Downloading …`（`reader.md`） |
| 新分頁 | `T`、Open in new tab（`[1]` 的 `o`、Focus mode 的 `o`）、`U`、Downloads 的 `o`：換到 Web 畫面、focus 到 `[2]`（tdp K3：使用者按了就預期看到那一頁）；書籤與歷史的 `Enter` 在背後開、留在清單上，toast 說開在哪；載入完成這種背景的事不搶 focus |
| `[1]` 的游標 | 移到哪個分頁 `[2]` 就顯示哪個（`[2]` 是 `[1]` 的明細，2026-10-18）；還沒載入的分頁換到才載 |
| 歷史記錄 | 載入完成後的最終網址與標題；同一個分頁連著同一個網址不重複記；`about:blank` 與載入失敗的不記 |
| 離開 | 寫回 session（每個分頁的網址、`[2]` 顯示的是哪一個），不論從哪裡離開（`q`、SIGINT／SIGTERM、終端機關掉）；`session.yaml` 讀不了、又沒搬開時不寫回 |
| 啟動 | `restore_session` 開著時還原上次的分頁，只載 `[2]` 顯示的那一個，其他切到才載；命令列給的網址各開一個分頁，顯示第一個 |

---

## 附錄 — hotkey 全表

### Core key
`Space` menu · `?` 這裡的鍵 · `!` App Log · `Tab`、`1|2` 換面板 · `Enter` 左鍵 · `Esc` 往上一層 · `q|Ctrl-C` 離開

### 全域
`W` Web · `B` Bookmarks · `H` History · `D` Downloads · `S` Settings · `q|Ctrl-C` quit

### `[1]` Tabs
`Enter` read · `c` close · `o` open in new tab · `r` reload · `y` yank URL · `T` new tab · `X` close others · `U` undo close

### `[2]` Page（閱讀）
`j|k` 捲一列 · `u|d` 半頁（`PgUp|PgDn`）· `gg|G` 頭尾 · `R` reload · `T` new tab · `H|L` back / forward · `b` breadcrumb · `o` open · `/` find · `n|N` next / previous match · `f` Focus mode · `v` Visual mode · `w` full page / content only · `A` add bookmark · `Z` zoom · `Y` yank page url · `C` close · `Esc` 收掉找到的字 · menu-only：Source、Cookies（裡面 `C` 清除）、Requests、Yank markdown

### Focus mode
`j|k` 下一個、上一個 · `gg|G` 第一個、最後一個 · `Enter` press · `o` open in a new tab · `y` copy its address · `Esc` leave

### Visual mode
`h|j|k|l` · `w|b|e` · `0|$` · `u|d` · `gg|G` · `v` select · `y` copy · `Enter` press · `/` find · `n|N` next match · `Esc` unselect / leave

### Bookmarks · History · Downloads · Settings
- Bookmarks：`Enter` open · `m` move · `r` rename · `y` yank · `x` delete · `a` add · `A` add folder · `I` import · `/` filter
- History：`Enter` open · `y` yank · `x` delete · `C` clear · `/` filter
- Downloads：`Enter` open · `o` open source · `y` yank path · `x` remove · `C` clear · `/` filter
- Settings：`Enter` change · `C` clear sign-in data

### 導覽
`j|k` · `u|d` · `gg|G` · `h|l`（Visual mode）· `1|2`
