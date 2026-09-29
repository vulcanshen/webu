# webu — UX

> 本文件講**互動語意**：core-key、Space menu 內容、hotkey 分層、兩種游標、每種輸入怎麼填、
> 浮層行為、時間軸。版面與 surface 在 `ui.md`，功能邊界在 `function.md`。
> 依 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.20/principle)（tdp）撰寫，章節編號對齊 kbu / filu / sshu（§A、§B 沿用 VTP 時期的分章，各章標出對應的 tdp 條目）；每條決定標日期。

---

## §A. tdp in webu

### §A.0 揭露對照（tdp M1、P2）

| Track | 入口 | 入口自身怎麼被揭露 | 完整性 |
|---|---|---|---|
| **Contextual** | `Space` | footer 常駐 `Space:menu` | 當前 focus 的 contextual 動作 100% 在 Space menu 內；`[2]` 的 item operation menu-only、沒有 letter hotkey |
| **Non-contextual** | `?` | footer 常駐 `?:help` | `?` 是最前端 surface 的 key reference，唯讀；全域動作 100% 在 global operation popup內、可直接執行，由每個 Space menu 最後一列打開；header 五個 screen 的 chip 是 Layer 2 ambient 揭露 |

**規則（2026-09-22）：一個面板操作沒進 Space menu 就等於不存在。** core key（Enter / Esc / `/` / `go`）
沒有字母可以括，所以鍵寫進 label 自己：`[Esc] Page parts`、`[Enter] Open section`、`[/] Search`、
`[go] Go to line`、`[n] Next section`。

### §A.0.K core-key 語意（tdp K 章）

| Core-key | 一般模式 | visual mode |
|---|---|---|
| `Tab` | `[1]` ↔ `[2]`；`1/2` 直達 | 不作用，toast 說先 `Esc` 離開模式（2026-09-27，tdp K2、K4） |
| `Enter` | **滑鼠左鍵在 terminal 的對應**（定案 2026-09-21；0.3.0 起先 hover 再按）；左鍵沒有對應的才自定義——**進去**：目錄開一節、清單裡的一件事走進去、frame 走進去；heading / landmark 開合；pagetab 上 = 確認回頁面。細表在 §A.0.K.1 | 對字元所屬的節點做同一件事，並離開模式 |
| `Esc` | **往上一層，一次一步**（2026-09-23）：關最上層浮層 → 走出一件事 / frame → 回目錄 → 上 pagetab → 再 Esc 回頁面。頁面自己的彈窗**不關**（toast：它要一個回答）。上一頁是 `P`，Esc 不兼職 | 打字中取消；回應 `Tab` 的 toast 還在時先收 toast（2026-09-29，tdp K11、K4）；否則離開模式 |
| `Space` | **滑鼠右鍵 context menu**；再按關閉；其他浮層上不作用（2026-09-27，tdp K5、F6） | **不作用**：模式沒有 Space menu、沒有可執行的按鍵清單（2026-09-28，tdp K11 v0.1.10） |
| `?` | **最前端 surface 的 key reference**，唯讀、可捲動：面板上是這個面板的鍵（由它的 Space menu 產生）+ core key；浮層上是這個浮層的鍵；再按關閉（2026-09-27，tdp K6、M4） | **模式的 key reference**，唯讀：模式的鍵直接按，列在這裡與 footer（tdp K11） |

#### §A.0.K.1 Enter 對每種東西

| 東西 | Enter |
|---|---|
| link | confirm（連結文字 + URL）→ Enter 開、Esc 不動；同頁 `#` 錨點直接跳到目標、目標捲成第一列 |
| button / check / radio / switch / menuitem / treeitem / tab / option / `<summary>` | 按下（hover + click） |
| disabled 的東西 | toast「it is disabled」，不按 |
| textbox（一行）| input popup，邊框寫它收什麼（§2） |
| textarea / contenteditable | editor popup（§2.3） |
| password | input popup 遮罩、不帶舊值 |
| searchbox（`type=search` 或 search landmark 裡的框） | 寫回後 confirm「Search」→ Enter 送 Enter 給欄位 |
| date / datetime-local / time / month / week / color | input popup，邊框寫形狀；不合形狀寫在框的錯誤列、框留著 |
| slider | 數字清單，Enter 移到游標那個數字 |
| `<select>` | option 清單 |
| `<input type=file>` | 檔案 picker |
| iframe | 走進去（同站、跨站、巢狀） |
| 清單裡的一件事（多於一行） | 走進去；只有一行的直接對裡面的東西做事 |
| heading / landmark 標題列 | 收合 / 展開 |
| 資料表的格 | 純文字 → 內容 popup；一個目標 → 它的 Enter；混合 → 選單 |
| code block | 全文 popup |
| 目錄的一列 | 開那一節 |
| pagetab 上 | 確認、回頁面 |
| image / video / canvas / unsupported | 按下，頁面決定 |
| progressbar / meter / tooltip / timer / 純文字 | 不是停靠點 |

**修訂史**：2026-09-20 實機試用後 Enter 曾改成「開該 item 的選單」；09-21 定案改回左鍵；
0.3.0 加上「進去」這一類（目錄 / 一件事 / frame）與 hover。

### §A.1 Contextual track — Space menu（tdp K5、M2）

region 固定叫 `item operation` / `panel operation`，一律有標題，即使只剩其中一區（2026-09-27）。每個 Space menu 最後
是分隔線與固定一列 `Global operation`，**不加區塊標題**（2026-09-28，tdp M2 v0.1.7：`global operation` 標題底下只有一列
`Global operation`，是同一句話講兩次），`Enter` 打開 global operation popup（§A.2）。menu 本身：`j`/`k` 走（環繞）、`u`/`d` 半窗、`gg`/`G` 首尾（2026-09-23）、Enter 執行、
letter hotkey 在 menu 裡也有效。

不能執行的列照樣出現、變暗，說明維持原本那句、不另寫原因，`Enter` 與熱鍵都不作用（tdp M6）；空清單
沒有 item，item operation 區連標題一起不出現（tdp M2）（2026-09-27）。

**`[1]` Tabs**

| item operation | panel operation |
|---|---|
| `[Enter] Switch to`、`[c] Close`（有下載進行中先 confirm）、`[o] Open in new tab`、`[r] Reload`、`[y] Yank url` | `[T] Tab`（新分頁，Location 框）、`[X] close others`、`[U]ndo close` |

**`[2]` Page — item operation 依 role**（menu-only）

| 東西 | item operation |
|---|---|
| link | Open（= Enter）、Open in new tab、Yank link url |
| button / check / radio / switch … | Click |
| textbox（空） | Edit |
| textbox（有值） | **Submit**（送 Enter 給欄位）、Edit、Clear、Yank；cursor 停 Submit |
| select | Choose |
| media / frame | Click、Yank media url |
| heading / landmark | Collapse / Expand |
| 資料表的格 | Content、然後格裡的東西各一列 |
| code block | `[Enter] Read` |
| 未支援 role | Click（說明寫明這個 role 還不支援；2026-09-27 起不另放一列 disabled） |
| 所有 item | Yank text、Inspect（role / name / value / url / state / node id） |

**`[2]` Page — panel operation**：`[R]eload`、`[T]ab`、`[P]revious`、`[N]ext`、`[/] Search`
（finder，§1.1）、`[v]isual mode`、`[L]ocation`、`[A]dd bookmark`、`[go] Go to line`、
`[Esc] Page parts` / `[Esc] Back to the page`、`Sections` / `One sheet`（目錄 ↔ 一整張；沒標題的頁
disabled）、`[n] Next section` / `[p] Previous section`（讀一節時）、`[I]nspect`（DevTools）、
`[Z]oom`、`[Y]ank page url`、`Yank markdown`（menu-only）、`[C]lose`。Outline popup 已拿掉
（2026-09-22）：目錄就是那個畫面。
變暗的條件就是動作的前提（2026-09-29，tdp M6）：`[P]revious` / `[N]ext` 在分頁的 history 前 / 後沒有頁面時（最前面的
`about:blank` 不算；capture 時跟 history entry 一起拿），`[/] Search`、`[v]isual mode` 在頁面還沒抓到時。導航中的一瞬間不變暗
（`dispatch()` 吞鍵，頁面變暗與 URL 列的 icon 已經揭露）；頁面自己 `pushState` 不一定觸發 capture，按了才發現沒有上一頁時照舊 toast。

**screen 內部**（2026-09-21 起未變）

| screen | item operation | panel operation |
|---|---|---|
| Bookmarks | 書籤列：Enter 開新分頁、`[a] Add`、`[m] Move`、`[x] Delete`（confirm）、`[y] Yank`、`[r] Rename`；目錄列：Enter 開合、`[a]` 加在裡面、`[r]` 改名（底下跟著搬）、`[x]`（空的直接刪、有東西的 confirm 後整棵刪） | `[A] Add folder`（`a/b/c` 一次三層）、`[I] Import`（picker 選瀏覽器匯出的 HTML → 強制輸入根目錄名）、`[/] Filter` |
| History | Enter 開新分頁、`[x] Delete`、`[y] Yank` | `[C] Clear`（confirm；沒有紀錄時變暗）、`[/]` |
| Downloads | Enter 開檔（只有下載完成的列；進行中、已取消的變暗）、`[o]` 來源開新分頁、`[x] Remove`（進行中先取消）、`[y] Yank path` | `[C] Clear` 已完成的（沒有已完成 / 已取消的列時變暗）、`[/]` |
| Settings | Enter → 文字框（目前值當提議；清空 = 預設）或翻開關 | — |
| DevTools › Storage / Network / Console / Source | `[x]` / `[y]`、Enter detail、`[i]` REPL | `[C] Clear`、`[/]`；`h`/`l` 切分頁 |

delete 用 `x` 不用 `d`：`d` 是半頁。

### §A.2 Non-contextual track — `?` key reference 與 global operation（tdp K6、M4）

**`?` 只讀**（2026-09-27，tdp v0.1.2）：打開最前端 surface 的 key reference —— 唯讀、可捲動、沒有游標、不能執行。
面板上是這個面板的鍵：從它此刻的 Space menu 產生（有鍵可按的列：字母、寫在 label 裡的 `Enter` / `Esc` / `/` / `go`；
menu-only 的列不列），照 menu 的區塊標題分組，最後接 core key；面板自己寫了 `Enter` 做什麼時，core key 不再重複
通用的 `Enter`。Space menu 裡變暗的列，這裡也列出、變暗，說明照原句（2026-09-29，tdp M6 v0.1.14）。浮層上是那個浮層自己的鍵，現在按不了的同樣變暗：DevTools 別的分頁的鍵（它們在 `h/l` 過去的分頁上，列出、不拿掉）、清單空著時的 `Enter` 與 `x/y`、放得下的 message 的 `j/k`（2026-09-29）。`Esc` 或再按 `?` 關掉。框的寬度跟所有 popup 一樣（tdp F7、D4），說明太長就截尾。

**全域動作在 global operation popup**（2026-09-27，tdp M4、F4、T1）：每個 Space menu 最後一列 `Global operation` 打開它，
**疊在 Space menu 上**；`Esc` 回到 Space menu，執行一列就關掉整疊（開出新框的列除外，那個框疊上去）。目前所在畫面的
那一列照 M6 變暗、不作用，不拿掉。列出下表的動作，一列一個，`j`/`k`、`Enter`、熱鍵照 Space menu 執行。

| 全域動作 | 鍵 |
|---|---|
| screen | `W` `B` `H` `D` `S` |
| 離開 | `q/Ctrl-C`：輸入框以外到處都通，浮層上、visual mode 裡也是；有下載進行中先 confirm，confirm 上再按 `Ctrl-C` 立刻離開（2026-09-27，tdp K9） |

**只有作用在 app 上的才是全域動作**（2026-09-27，tdp M3、P3）：`P/N`、`L`、`/`、`v` 作用在頁面上，是 `[2]` 的
panel operation，不在 global operation popup 裡；它們的熱鍵在 web 的兩個面板上都通（`[1]` 上也作用在顯示中的那一頁）。
切面板的 `Tab`、`1/2` 是 core key。splash 彩蛋 `V` 不揭露（tdp S2）。

全域字母在浮層開著、visual mode、打字中三種狀態下不作用；`q` 例外，只有打字中不作用。

---

## §B. 元素專職化（tdp P4）

| 元素 | 唯一語意 |
|---|---|
| 邊框 Blue / 雙線 | focus |
| `[1]` 綠字 | `[2]` 正在顯示的分頁 |
| 邊框 Yellow、上框右側寫 `Visual mode` | visual mode（tdp K11 v0.1.18：模式標示自己） |
| glyph | 一個 role 一個 glyph；一個 part 一個 glyph；碼位查字型不憑記憶 |
| URL 列的圖示 | 靜止是地球、抓取中是圓餅填滿（settling 期間一直轉） |
| pagetab 整條 rosewater 反色 | 手在 pagetab 上 |
| 面板 header 列變色 | 在哪一節（該深度的 ink）/ 在哪一件事裡 |
| 下框 hint | 在哪：`2/26 · 節名 · 40%` |
| `y` / `Y` | 游標下的東西 / 整頁 |
| `r` / `R` | 這個分頁 / 這一頁 |
| `c` / `C` | `[1]` 游標列 / `[2]` 正在顯示的分頁 |

---

## §1 兩種游標

| | item 游標（一般） | 字元游標（visual mode，`v`） |
|---|---|---|
| 單位 | item；文字是 flow | 字元 |
| 移動 | `j/k` 換列、`h/l` 同列、`u/d` 半頁、`gg/G` | `h/j/k/l`、`w/e/b`、`0/$`、`u/d`、`gg/G` |
| Enter | §A.0.K.1 | 字元所屬節點的 Enter |
| 選取 / 複製 | — | `v/V` 選、`y` 寫剪貼簿（OSC52 + pbcopy / wl-copy / xclip / xsel） |
| `/` | **finder**（§1.1） | 逐字搜尋（smart case、`n`/`N`） |
| 視覺 | Blue 雙線邊框 | Yellow 雙線邊框，上框右側寫 `Visual mode` |
| 即時更新 | 重畫、游標留位 | 凍結；Esc 離開時套用 |

### §1.1 finder（`/`）與 go

**`/` 是找東西，不是逐字搜**（2026-09-23 重新定義，filu 的 `/` 形式）：popup 三區——輸入 / 命中清單 /
預覽（寬 ≥ 96 欄並排，否則上下）。索引是**整頁四個 part** 裡「持有文字的最小區塊」各一次
（段落、清單裡的一件事、表格的格、連結、按鈕、欄位、標題…），散文字面比對優先、名字模糊比對
（≤ 64 字）；清單列前面是 part 的 glyph。打字時 Enter 進清單（第一個命中已在游標下，所以連按兩次 Enter 就是去第一個）、清單上 `j`/`k`/`u`/`d`/`gg`/`G` 走、再打字回到輸入；清單上的 **Enter = 去那裡**：
切到那個 part、開那一節、沿路走進清單裡的一件事、游標落定——**不按**。`Tab` 在輸入與清單之間切換（打字時
`Enter` 也進清單）；`Esc` 不論在哪一區都關掉 finder —— 階段不是一層（2026-09-28，tdp F1 v0.1.9）。只有拿鍵的那一邊是亮的（2026-09-29，tdp F1、D3 v0.1.18）：打字時篩選列亮、游標列是 Subtext1 的淡反白；`Tab` 到清單後篩選列整列 Overlay0（不是 F8 的淡化、沒有游標），游標列換成 popup 層色底、深色粗體字。`go` 只有一個階段，不在此列。finder 開著時頁面還在 settle，命中只記 id 與路徑、落地時從當下的樹重算。彈窗裡沒有 `/`。

**`go`**（`g` 然後 `o`）只管**行號**：popup 列出目前畫面的每一行（目錄上就是節的序號），打數字當
前綴篩、Enter 跳。彈窗裡沒有 `go`。

---

## §2 文字輸入

### §2.1 每種輸入怎麼填（2026-09-23 全部定義）

| 輸入 | 行為 |
|---|---|
| text / email / tel / url / number / search / spinbutton、沒有 option 的 ARIA combobox | **一行 input popup**：邊框寫型別（`text`、`email`、`phone number`、`number`、`search`；填錯的加 ` · invalid`），框裡那行是欄位名、預填目前值；Enter 寫回（`insertText`，框架收得到 input 事件）、Esc 不動 |
| password | 同上，遮罩、不帶舊值、邊框 `password` |
| searchbox | 寫回後 confirm「Search」：Enter = 送 Enter 給欄位（implicit submission）、Esc 值留著不送 |
| textarea / contenteditable | **editor popup**（§2.3） |
| date / datetime-local / time / month / week / color | 一行 input popup，邊框寫**形狀**：`date · YYYY-MM-DD`、`date and time · YYYY-MM-DDTHH:MM`、`time · HH:MM`、`month · YYYY-MM`、`week · YYYY-Www`、`color · #rrggbb`；不合形狀在框的錯誤列寫「wants YYYY-MM-DD」、框留著（瀏覽器會無聲丟掉不合的值，寧可擋在前面）；合的整串設值 |
| slider（range / ARIA） | options menu 列出 bar 上每個數字（範圍很大的以十、百為步）、10 列一窗、游標在目前值置中；`j`/`k`/`u`/`d`/`gg`/`G`、Enter 移過去（2026-09-23 user 定：打字輸入數字不好用） |
| checkbox / radio / switch | Enter 切換 |
| `<select>` | option 清單，Enter 選、寫回並發 change |
| `<input type=file>` | picker（同書籤匯入）：`~/Downloads` 起、一次一個目錄、Enter 選檔 |

打字中屏蔽所有 hotkey：`Space` 是空白、`?` 是問號。

**input group**（2026-09-27，tdp K3）：幾個欄位一起才有意義的，放進同一個 input popup —— 加書籤是 URL + 標題，HTTP 驗證是
帳號 + 密碼（遮罩）。`Tab` / `Shift-Tab` 在欄位之間移動，`Enter` 一律送出整組；送出不成立時框留著、焦點回到那一欄、
原因寫在框的錯誤列（加書籤的 URL 不可空）。**錯誤列**（2026-09-28，tdp F7、K3）：送出可能失敗的框 —— 加書籤、日期時間顏色、設定、
新增目錄、匯入名稱、改名 —— 打開時就有一列空白的錯誤列，拒絕時原因寫在那裡，框的高度不變、打的字留著；不會失敗的框（Location、
欄位、登入、JS prompt、console）不留。原因不再丟到 toast。新增目錄也先檢查重名、再關框。group 裡 `Tab` 只換欄，空欄位上的提議用 `→` 接受；單欄框沒有別欄可換，`Tab` 接受提議（`→` 也可以）
（2026-09-27，tdp K2 v0.1.6）。`Backspace` 在空欄位上拒絕提議。下框一個動作只露一個鍵：單欄 `Tab:accept`、group `→:accept`，
拒絕是 `Backspace:decline`。
加書籤的標題提議跟著 URL 欄：URL 是目前這頁就提議頁面標題，否則提議 URL；欄位空著送出就用提議。`Esc` 取消整組。

### §2.2 Location（`L`）

開啟時輸入列空的、目前分頁的 URL dim 當提議：`Tab` 接受、接進來編輯（`→` 也可以；tdp K2 v0.1.6）、`Backspace` 拒絕、
直接打字從頭來。
Enter：像 URL（有 scheme 或 `host.tld`）→ 補 `https://`；不像 → 當搜尋（`search_engine`）。`L` 開在
目前分頁，`T` 開新分頁。

### §2.3 editor popup（textarea）

大框、多行。**寫**的狀態：打字、Enter 換行、`Tab` 縮排（插四個空白；多行文字的寫入狀態裡 `Tab` 是字元，tdp K8、K2）、Backspace 跨行合併；`Esc` 出到**移**的狀態：`h/j/k/l`、`u`/`d`、
`g`/`G`/`0`/`$` 走，`i`/`a`/`A`/`o` 回到寫，`Enter` 設值寫回，再 `Esc` 取消。`Space` 永不關框。
`$EDITOR` 鏈未做。

---

## §3 導覽詞彙

`nav.go` 一份，所有清單共用；導覽字母 `h j k l u d g G` 不被任何動作佔用。

| 鍵 | 動作 |
|---|---|
| `j/k` | 上下一列。`[2]` 換列（跳到下一列有 item 的列、落在最近的欄位）、不繞；目錄與 menu 繞 |
| `u/d` | 半頁 |
| `gg/G` | 頭 / 尾（頁面、目錄、menu、editor 都通） |
| `h/l` | `[2]` 同列的 item 之間；pagetab 上走 part（走到哪內容切到哪、會繞）；DevTools 切分頁 |
| `n/p` | 讀一節時：同深度的下一節 / 上一節 |
| `Esc` | 往上一層（§A.0.K） |
| `go` | 行號 |
| `1/2` | 直達面板 |

Mouse：不做；日後若加必須是鍵盤的 mapping。

---

## §4 Hotkey 分層與全表

規則：**小寫 = item operation 或本畫面的移動**、**大寫 = panel operation 或全域**；bracket 印的就是要按的鍵。

| 層 | 鍵 |
|---|---|
| 全域 | `W` `B` `H` `D` `S`、`q`、`?`、`V`（彩蛋）；只在 `[W]eb`：`P` `N` `L`、`/`、`v`、`Tab`、`1/2` |
| `[1]` item / panel | `c` `o` `r` `y` / `T` `X` `U` |
| `[2]` item | 無（menu-only） |
| `[2]` panel | `R` `T` `A` `I` `Z` `Y` `C`；`n` `p`（讀一節時）；`go`；`Esc` |
| Bookmarks | `a` `m` `r` `x` `y` / `A` `I` `/` |
| Downloads | `o` `x` `y` / `C` `/` |
| History | `x` `y` / `C` `/` |
| Settings | Enter |
| DevTools | `x` `y` / `C` `/`；`h` `l`；Console `i` |
| visual mode | `h/j/k/l` `w` `e` `b` `0` `$` `u` `d` `gg` `G` `v` `V` `y` `/` `n` `N` |
| editor（移） | `h/j/k/l` `u` `d` `g` `G` `0` `$` `i` `a` `A` `o` |

撞字：`n/p` 小寫只在讀一節時有意義，與 `N/P`（前後頁）只差大小寫、同 sshu 的 `[D]isconnect` 與 `d`；
`O` Outline 已拿掉；其餘 2026-09-21 的檢查不變。

---

## §5 浮層行為

沿用 terminu family 的 popup 慣例（tdp D3）：一個 popup 一個檔一個 animator、`Esc` 只在 `closeTop` 一處解析、
`Space` 只開關 Space menu、在其他浮層上不作用、正在關閉的浮層不握鍵盤。

**頁面自己的彈窗**是例外（`ui.md` §2.4）：它不是 webu 的浮層，是頁面的；`Esc` 不關、`Space` 是它的
選單、要回答它才會走（按裡面的按鈕、或頁面自己收掉）。

**離開的 confirm 疊在最上面**（2026-09-27，tdp D3、K4）：有下載進行中時 `q/Ctrl-C` 問的那一題是自己的浮層，疊在整疊
之上，不取代底下正在回答的 confirm；`Esc` 回到那一題。它的 `?` 也是自己的 help，順序 quit help > quit confirm > help > 其他。

**source 留在底下**（2026-09-27，tdp F4、T1、D3）：從 Space menu、global operation popup或 options 開出的框（confirm、
input、finder、DevTools……）疊在 menu 上，menu 留在底下；`Esc` 回到 menu，完成動作才整疊清掉。

**Context shift 清 source**：Space menu 裡點了會換頁的動作，menu 這個 source 清掉。

**先 confirm 的動作**：`[1]` `c` 關有下載進行中的分頁、`q` 離開時有下載、Bookmark delete（目錄有東西時）、
History clear、Clear site data、憑證錯誤、`beforeunload`、link Open、搜尋框 Search。

---

## §6 時間軸

| 情境 | 行為 |
|---|---|
| 按下 / 導航 | 從按鍵那一刻起 URL 列的圖示轉；導航時 `P/N` / `R` / click 吞掉（連按 `PPP` 只算一次）；沒有上一頁 toast |
| settling | 每次 capture 算指紋；指紋不同、又在 8 秒寬限內 → 還在轉、游標不重設；兩次一樣才落地（`function.md` §6） |
| 彈窗窗 | 按下 / 載入後 8 秒內新出現的子樹才可能是彈窗 |
| 一次一個動作 | 每個分頁一把鎖：讀 box 與按下是同一個動作；對話框 / auth 的回答不排隊 |
| 換頁 | context shift：menu 清掉、screen 回 `[W]eb`、URL 更新、焦點回 `[2]`；文件型的頁面先進目錄，否則游標到 main 第一個 item |
| 上一頁 | 回到離開那個 entry 時的 part / 目錄或節 / 捲動 / 游標 |
| 即時更新 | 重畫不退階；游標留位；visual mode 裡凍結 |
| 歷史記錄 | load 後的最終 URL + 標題；同 URL 的 settle 不重複記；`about:blank` 與錯誤頁不記 |
| 離開 | 存 session → 殺 Chromium |
| 啟動 | 還原分頁但不預載，切到才載 |

---

## 附錄 — hotkey 全表

### Core key
`Tab` 切面板 · `Enter` 左鍵 / 進去 · `Esc` 往上一層 · `Space` menu · `?` help

### 全域
`W` Web · `B` Bookmarks · `H` History · `D` Downloads · `S` Settings · `P` 上一頁 · `N` 下一頁 · `L` location · `q` quit · `/` finder · `v` visual mode

### `[1]` Tabs
`c` close · `o` open in new tab · `r` reload · `y` yank url · `T` new · `X` close others · `U` undo close

### `[2]` Page
`R` reload · `T` new tab · `A` add bookmark · `I` inspect · `Z` zoom · `Y` yank page url · `C` close this tab · `n`/`p` 下一節 / 上一節 · `go` 行號 · `Esc` 上一層（一件事 → 目錄 → pagetab）

### 導覽
`j/k` · `u/d` · `gg/G` · `h/l`（同列、pagetab、DevTools 分頁）· `1-2`
