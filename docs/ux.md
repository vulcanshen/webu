# webu — UX

> 本文件講**互動語意**：core-key、兩種模式、文字輸入、Space menu 內容、hotkey 分層、
> `?` 內容、浮層行為、時間軸。版面與 surface 在 `ui.md`，功能邊界在 `function.md`。
>
> 依 VTP（`thoughts/tui-design`）撰寫，章節編號對齊 kbu / filu / sshu 的 implementation doc。

---

## §A. VTP in webu

### §A.0 揭露對照

| Track | 入口 | 入口自身怎麼被揭露 | 完整性 |
|---|---|---|---|
| **Contextual** | `Space` | footer 常駐 `space menu` | 當前 focus 的 contextual 動作 100% 在 Space menu 內。`[2]` 的 item operation **只走 Space menu、沒有 letter hotkey**（決定 2026-09-20：小寫 hotkey 造成困惑，先不做） |
| **Non-contextual** | `?` | footer 常駐 `? help` | 全域動作 100% 在 help popup 內；header 列是五個 screen（Web / Bookmarks / History / Downloads / Settings）的 chip 列，全域字母切換（Layer 2；修訂 2026-09-21：三個 list 從 popup 改成佔滿整個 body 的 screen，sshu 的 tab 作法，`[W]eb` `[S]ettings` 補上；2026-09-20 先從 `[1]` Places 面板併進 header） |

### §A.0.K core-key 語意

| Core-key | 一般模式 | 選取模式（§1） | 對應通用 |
|---|---|---|---|
| `Tab` | `[1]` ↔ `[2]`；`1`–`2` 直達（修訂 2026-09-20：只剩兩個面板，header 不是面板、Tab 不停） | 同 | §4.1 |
| `Enter` | VTP 2026-09-21 改寫為「**啟動該項目最直觀的操作、具體是什麼依 app context 而定**」（原「確認 / 進入」）。webu 的定義（定案 2026-09-21，見下）：**Enter = 滑鼠左鍵在 terminal 的對應，Space = 右鍵選單，兩者分開**。頁面 item：textbox / textarea / password → 直接開 input popup（password 遮罩）；select → 直接開 option 清單；button / checkbox / radio / switch / media / unsupported → 直接 click；link → **先 confirm popup**（連結文字 + URL），Enter 才開；navigation 列 → 它的 item operation 選單（裡面的連結，Enter 就開；2026-09-21）；自定義（左鍵沒有對應）：其他 landmark 標題列、heading、Bookmarks 目錄列 → 展開 / 收合；沒定義到的 → notice popup「尚未定義」。Bookmarks / History 列 → 開新分頁；Downloads 列 → 開檔；Settings 列 → 編輯框 | 離開模式、對字元游標所屬的節點做同一件事 | §A.0.K |
| `Esc` | 關最上層浮層；沒有浮層時**無作用**（上一頁是 `P`，Esc 不兼職） | 打字中：取消搜尋輸入；否則離開選取模式 | §4.3 |
| `Space` | **滑鼠右鍵 context menu**；再按關閉；在浮層上按 = 關掉它 | 熱鍵 cheatsheet，按列出的鍵即執行並關閉 | §A.1 |
| `?` | help；再按關閉；可疊在任何浮層上 | 同 | §A.2 |

**修訂史**：2026-09-20 實機試用後，Enter 曾改成「開該 item 的 item operation 選單、第一列主要動作」（Space = 完整選單），
理由是 Enter 底下的動作看不見、先揭露再做。2026-09-21 再試，**定案改回並收緊：Enter = 滑鼠左鍵在 terminal 的對應，
Space = 右鍵選單，兩者分開**，選單不再由 Enter 開——之前那樣是圖方便，現在 Enter 鎖定在「左鍵點下去會發生的事」的 terminal 版本，
左鍵沒有對應的才自定義：

- textbox / textarea / password：左鍵是聚焦打字 → 直接開 input popup（password 遮罩、不帶舊值）；Submit / Edit / Clear / Yank 在 Space 選單
- select：左鍵是拉下清單 → 直接開 option 清單
- button / checkbox / radio / switch：直接 click；media 佔位框、unsupported 也是（就是一個 click，頁面決定）
- link：左鍵會換頁、看不見要去哪 → **先 confirm popup**（連結文字 + URL），Enter 才開、Esc 不動；Open in new tab 在 Space 選單
- 入口型 landmark（banner / navigation / breadcrumb / search / complementary / contentinfo）：URL 底下那條 **tray**（膠囊列）上的膠囊 `󰍜 字 +N`（2026-09-22：一種一顆、順序固定 skip / header / nav / search / sidebar / footer / dialog / other，字是種類的固定字彙、dialog 用它的名字、icon 一律漢堡選單——每顆都是清單；「你在裡面的哪一格」放面板下框的 hint），內容不進畫面、頁面本體從內容開始；塞不下的收成最後一顆 `+N`、Enter 列出；`Esc`（沒 popup / mode 時）跳上 tray、再 `Esc` 回頁面；頁面最上列的 item 按 `k` 也上得去、`h`/`l` 走（會繞）、`j` 回來；Enter = 它的 item operation 選單（裡面每個 link / button / 輸入框 / 勾選 / select 一列、巢狀清單縮排、Enter 就做該物件的事）——「Enter = 該物件的期望操作」在入口型物件上的樣子；search 只有一個框時 Enter 直接開框；Space 照舊 item operation + panel operation（2026-09-21）
- 自定義：其他 landmark 標題列、heading、Bookmarks 目錄列 → 展開 / 收合
- 沒定義到的 → notice popup「Enter 尚未定義」，Space 仍列得出能做什麼

**與 filu 的關係**：filu 的 Enter 只進目錄不開檔，webu 的 Enter 是左鍵——VTP §A.0.K（2026-09-21）「啟動該項目最直觀的操作、依 app context 而定」
容許各 app 自己定義，兩者都合規。同一種列在所有 surface 的 Enter 都一樣，這條沒變。

**不是 core-key 的**：`q` 離開、`Ctrl+C` 硬退、`P` / `N` 前後頁、`/` 搜尋、header 的三個全域
字母 `B` `H` `D`、`v` visual mode。它們是 §A.2 軌的動作，全部列在 `?` help。Visual mode（本文件其他地方的
「選取模式」）的鍵是 `v`（修訂 2026-09-21：`V` 還給家族的 splash 彩蛋，visual mode 改回小寫 `v` 當特例——一個 mode 不是 panel operation。2026-09-20 的過程：`Alt+v` 先拿掉、補回裸 `v`、再改成大寫 `V` 與其他 panel
operation 一致 —— item 游標只停在 item 上，要進到段落文字裡得有一個直接的鍵；`/` 搜尋也進得去）。

### §A.1 Contextual track — Space menu

每個 focus 按 `Space` 列出「這裡能做什麼」。兩個 region 固定叫 `item operation` /
`panel operation`（kbu / filu / sshu 同字串）；只有一個 region 就保持扁平；一列的 menu
不開、直接執行（sshu §11.16）。列的形狀 `[X]label` + 靠右說明，有 hotkey 才加 bracket。

**header**：`[W]eb` `[B]ookmarks` `[H]istory` `[D]ownloads` `[S]ettings` 五個 chip，各自一個全域鍵切換 screen（修訂 2026-09-21：list 從 popup 改成 screen，佔滿 header 與 footer 之間，sshu 最上列 `[M]anage / [F]ile transfer / [S]SH` 的作法；`[W]eb` 是左右並列的 Tabs / Page）。header 本身不是面板、沒有 cursor、Tab 不停在上面；每個 list screen 有自己的 Space menu（item / panel 兩 region，鍵與直接按的同一組）。Esc 在 screen 上：先清 `/` 過濾，再回 `[W]eb`。

**`[1]` Tabs**

| item operation | panel operation |
|---|---|
| Enter 切換到此分頁、`[c] Close`（該分頁有下載進行中 → 先 confirm；修訂 2026-09-21：原 `[w]`，w 是 window 的字母、終端機沒有 window，改成 `[2]` 的 `C` 的小寫）、`[o] Open in new tab`（同一頁再開一個；原 `[c] Clone`，`c` 讓給 Close、`d` 是導覽鍵，`o` 與 Downloads 列的 `o` 同義：這一列的 URL 開新分頁）、`[r] Reload`、`[y] Yank url` | `[T] New tab`（開 goto popup）、`[X] Close others`、`[U] Undo close` |

**`[2]` 一般模式，item operation 依 role**（無 hotkey，menu-only）

| role | item operation |
|---|---|
| link | Open（= Enter，先 confirm；2026-09-21）、Open in new tab、Yank link url（寫明「link」：游標在連結上時，光寫 url 會被讀成整頁的，那是 panel operation 的 `[Y]`；修訂 2026-09-20） |
| button / checkbox / radio / switch | Click（= Enter） |
| textbox（空） | Edit（= Enter，直接開 input popup；password 欄位遮罩、prompt 寫 password） |
| textbox（有值） | Enter 直接 Edit（開 input popup、帶目前值；修訂 2026-09-21）。Space 選單：**Submit**（送 Enter 給頁面）、Edit、Clear、Yank；cursor 預設停 Submit（sshu §6.6.1 主要意圖優先） |
| textarea | Enter 直接 Edit（多行 popup）。Space 選單：Edit、Edit in editor（§2 editor 鏈）、Clear、Yank；無 Submit（瀏覽器裡 textarea 的 Enter 是換行） |
| select | Choose（= Enter，直接開 option 清單 menu） |
| image / media 佔位框 | Click（= Enter）、Yank media url |
| heading | Collapse / Expand（Enter 直接切換；2026-09-21 落地）：收合到下一個同級或更高級 heading、或所在 landmark 結束為止，收合列畫成 `▸ # 標題 · N items`；重抓後仍記得，換頁即忘 |
| 入口型 landmark：banner / navigation / breadcrumb / search / complementary / contentinfo | **URL 底下 tray（膠囊列）上的膠囊** `󰍜 字 +N`（2026-09-22：從頁面裡的一行搬到分隔線上、頁面本體從內容開始；**一種一顆**、順序固定 skip / header / nav / search / sidebar / footer / dialog（每個 dialog 一顆、用名字）/ other（有 main 時、main 之外又不在任何 landmark 裡的東西：促銷條、純 div 的 cookie 橫幅）；字是種類的固定字彙——真站上「你在哪一格」一半是 Homepage、(Top)、PRODUCTS 這種垃圾、站名又跟 URL 重複，所以「你在哪一格」改放面板下框的 hint（`navigation Repository · at Code · 7 inside`、`3 navigations · 21 inside`）；icon 一律漢堡選單，每顆都是清單；`+N` = 裡面的目標數；Enter 清單用 header 列分段（每個 nav 一段）、aria-current 的列 hint 標 here；一個目標都沒有的膠囊 Enter 看全文；`Esc` 上 tray / 回頁面；塞不下的收成最後一顆 `+N`；2026-09-21 user 定案：這一套 style 套到所有 page chrome，只用 icon 分種類；main / article / region / form 是內容，維持分隔線 + 可收合）。icon（2026-09-22 起全部改成漢堡選單 menu，種類靠字；原本：banner page_layout_header、navigation menu、breadcrumb map_marker_path、search search、complementary table_of_contents、contentinfo page_layout_footer）；dialog / alertdialog（cookie 同意、登入 modal，非 landmark 的雜訊也走這條——user 2026-09-21：用 popup menu 呈現）dock_window，字是 aria-label → 第一個 heading → 第一段文字，Enter 列出它的按鈕；**skip link**（「Skip to main content」那種：同頁 `#` 錨點且文字以 skip 開頭、或 DOM class 含 skip）也是一顆膠囊，icon skip_next、字就是它的文字；**skip 區塊**（Atlassian 的「Skip to: Top Bar / Sidebar / Main Content」：class / id / aria-label 含 skip 的容器、裡面全是同頁錨點）整個包成一顆膠囊 `󰒭 Skip to: +3`，Enter 列出目標（user 2026-09-21：跟收納 style 一樣、整個 skip to 是一個項目）。Enter / 選了目標 = 它的意思：**跳到錨點**——`jumpToAnchor` 用 DOM snapshot 的 id 與 parent 表，游標停到錨點元素（它本身是 item）或它裡面第一個 item；錨點找不到東西可停時，單一 skip link 退回「內文第一個 item」、其他就交給頁面 click。**同頁錨點連結一律這樣跳**（TOC 也是），不 confirm、不換頁。新頁面從內容開始（chrome 都不在頁面裡）；banner 等入口的選單裡不列單一 skip link。hint 的字（2026-09-22 前是膠囊的字；膠囊改成種類固定字彙後這套規則搬到面板下框的 hint）——**不寫 landmark 的名字，寫「你在裡面的哪一格」或它是什麼**：navigation 是目前的 tab（`aria-current` → URL 比對：連結 URL 等於頁面 URL、或最長路徑前綴 → 才退回 landmark 名字）、breadcrumb 是最後一節、banner 是站名（第一個不是 `#` skip link 的連結文字，logo）、search 是框的名字、complementary / contentinfo 是 landmark 名字 → 第一個 heading → 沒有就只剩 icon。`+N` = 裡面的 link / button / 輸入框 / 勾選 / select 數。內容不進畫面、不佔頁面；膠囊列本身是選單、不是一行字：固定順序、塞不下就 `+N`（Enter 列出，選一顆就把 hand 放上去、它暫佔最後一格、並開它的清單），絕不橫向捲動（橫向不定長的東西會誘導 `h/l`，terminal 最忌諱）；`k` 從頁面最上列上去、`h`/`l` 走（會繞）、`j` 回剛才的 item、其他移動鍵先回頁面。`aria-current` 與 breadcrumb 標記 AX tree 沒有，從 DOM snapshot 讀；沒包在 nav 裡的 `ol.breadcrumb` 在 ir 包成一個 navigation landmark。Enter = item operation 選單：裡面每個目標一列（巢狀清單縮排；link 標 URL、button / 輸入框 / 勾選 / select 各標種類）、Enter 就做該物件的事（link / button / 勾選 = click、不再 confirm，輸入框 = 開框，select = 開清單）；**search 只有一個框時 Enter 直接開框**；Space = 同一份 item operation + panel operation；Yank text、Inspect 照舊；不收合（沒東西可收）。取代原本的「導覽清單一行流式」與這些 landmark 的分隔線；2026-09-22 再從頁面裡的一行搬到 URL 底下的分隔線上 |
| table cell（有 header 的資料表） | **每一格都是 item**（表頭格也是；格裡的連結、按鈕不再是 item，藏在格後面）——格被壓縮、截斷是正常的，所以要能停上去看全文；這是唯一 `hjkl` 四鍵都在用的地方（`h/l` 同列換格、`j/k` 換列）。Enter：純文字格 → **內容 popup**（message popup，標題是該欄的表頭、全文折行、長了 `j/k` `u/d` `G` 捲動）；格裡只有一個連結 / 按鈕 / 欄位、沒有別的字 → 那一格就是那個東西，Enter 做它的 Enter（連結 confirm 後開）；文字加連結混合 → item operation 選單，第一列 Content、之後列出格裡的東西。Space 同一份。整張表有底色（surface0），表頭列更深一層（surface1）、文字 Text 粗體——**表頭靠底色分、不靠前景色**，Mauve 拿掉（2026-09-21） |
| 所有 item 共有 | Yank text、Inspect（message 類 popup：role、name、states、backendDOMNodeId、href 或 src） |
| 未支援 role | 第一列 disabled：「role: slider，尚未支援，只能 click」（function.md §3 fallback） |

頁面**沒有任何 item**（純文字頁）：Space menu 只剩 panel operation，扁平不分 region。

**`[2]` 一般模式，panel operation**：`[R] Reload`、`[T]ab`（新分頁，與 `[1]` 的 `[T]` 一模一樣；修訂 2026-09-20）、`[P] Previous`、`[N] Next`、
`[/] Search`（進 visual mode）、`[v]isual mode`（修訂 2026-09-20：原 `[V] View source` 搬進 DevTools › Source，`V` 讓給 visual mode；
2026-09-21：再改小寫 `v`，`V` 還給家族的 splash 彩蛋——mode 不是 panel operation，這是這一區唯一的小寫）、`[L]ocation`（修訂 2026-09-21：原 `UR[L]`，label 改 Location、hint 改「a URL or a search; this page's own is offered」；2026-09-20 從 `[U]` 改來，對應 Chrome 的 Cmd+L；全域鍵，任何面板都能按，見 §7）、`[A]dd bookmark`（修訂 2026-09-20：原 `[A] Add to…` picker 二選一，Shortcuts 拿掉後只剩 Bookmarks，直接加）、`[O] Outline`、`[I]nspect`（DevTools；修訂 2026-09-20：原 `[D]`，`D` 讓給 header 的 Downloads，`I` 對應 Chrome 的 Cmd+Opt+I）、
`[Z] Zoom`、`[Y] Yank page url`、`[C]lose`（關掉 `[2]` 正在顯示的分頁；`[1]` 的小寫 `c` 關的是游標列；
修訂 2026-09-21：原 `[W] Close`，`W` 讓給 header 的 Web）。

Outline 與 DevTools 的作用對象是目前頁面 → contextual → 在這裡，不在 header、不在 §A.2
（決定 2026-09-20）。使用者不用切面板，體驗留在頁面上。

**`[2]` 選取模式**：cheatsheet popup，列 `hjkl`、`w/e/b`、`0/$`、`u/d`、`gg/G`、`v/V`、
`y`、`/`、`n/N`、`Esc`。**按任一列出的鍵即執行並關閉 popup**（一步揭露，§A.0）。

**screen 與 popup 內部**

| screen / popup | item operation | panel operation |
|---|---|---|
| Bookmarks（screen） | 書籤列：Enter 開**新分頁**（修訂 2026-09-21：一律新分頁、不佔原分頁，`[o]` 因此拿掉）、`[a] Add`（兩個 input popup：URL、再 title，加在游標所在的目錄；`[W]eb` 正在顯示的頁面當 placeholder，Enter 直接收下——加目前頁就是 `a` Enter Enter）、`[m] Move`（options popup 當目錄 picker：`(no folder)` + 目錄樹（縮排表階層），數字鍵選）、`[x] Delete`（confirm）、`[y] Yank`、`[r] Rename`（改 title，框裡帶目前的值直接改；2026-09-21，原本規劃的 `[e] Edit` 就是這個；URL 要改就刪掉重加）。目錄列：Enter = Expand / Collapse（收起來一列 `▸ 󰉋 dev · 3 bookmarks`）、`[a] Add`（加在它裡面）、`[r] Rename`（只改這一層的名字：不能含 `/`、不能撞名、空的不收、框留著；底下的書籤與目錄路徑跟著換，摺疊狀態也跟著）、`[x] Delete`（空目錄直接刪、不問；有東西的先 confirm、列出裡面的書籤數與目錄數，確認就整棵樹刪掉——修訂 2026-09-21：import 進來的是一整棵樹，一列一列刪不是復原的辦法；上層目錄不受影響） | `[A] Add folder`（加在游標所在目錄下；路徑 `a/b/c` 一次開三層；空目錄也列出）、`[I] Import`（2026-09-21：file picker——filu finder 形式、同 sshu 的 identity picker，改成一次一個目錄：目錄在前、打字過濾、Enter 進目錄或選檔、空過濾時 Backspace 回上層、從 `~/Downloads` 開始——選瀏覽器匯出的 Netscape Bookmark HTML（Chrome / Firefox / Safari / Edge 同一格式；不是 XML）→ 選到就先解析、不是匯出檔立刻 toast → input popup **強制**輸入根目錄名（空的不收、已存在的不收、框留著）→ 整棵樹倒進 `名稱/…`，空目錄也保留、原有書籤不動）、`[/]` 搜尋。修訂 2026-09-21：原 `[A] Add this page` / `[F] Folder` / `[f] Subfolder` 拿掉 |
| Downloads（screen） | Enter 開檔（系統開啟器；進行中 → toast）、`[o]` 來源 URL 開新分頁、`[x] Remove`（進行中先取消；檔案不動）、`[y] Yank path`（未落地時 yank 來源 URL） | `[C] Clear` 已完成 / 取消的 |
| History（screen） | Enter 開**新分頁**（修訂 2026-09-21）、`[x] Delete` 該筆、`[y] Yank` | `[C] Clear`（confirm；唯一清除入口，歷史無限保留） |
| Settings（screen） | Enter → input popup 改值 → 寫回 `config.yaml`、立即生效；目前只有 `download_dir`（2026-09-21）。**所有文字設定的 popup 同 `[L]ocation`**：目前生效的值當 placeholder，Tab 接手編輯、Backspace 整個清掉；沒動過就 Enter = 不改，清空後 Enter = 回預設 | — |
| DevTools › Storage | `[x] Delete`、`[y] Yank value` | `[C] Clear site data`（confirm）、`[/]` 過濾 |
| DevTools › Network | Enter detail | `[C] Clear`、`[/]` |
| DevTools › Console | Enter：該筆的完整內容（detail） | `[i] Insert`：eval 輸入列（REPL：Enter 執行、輸入列留著、Esc 結束；`> 運算式` / `< 結果` 進清單）、`[C] Clear`、`[/]` |
| Outline | Enter 跳轉 | — |

delete 用 `x` 不用 `d`：`d` 是半頁（sshu `[x] Delete`）。

### §A.2 Non-contextual track — `?` help

| 全域動作 | 鍵 | 揭露 |
|---|---|---|
| Web screen | `W` | header chip + help（修訂 2026-09-21） |
| Bookmarks screen | `B` | header chip + help |
| History screen | `H` | header chip + help |
| Downloads screen | `D` | header chip + help（修訂 2026-09-20：Shortcuts 與 `S` 拿掉、Downloads 補上） |
| Settings screen | `S` | header chip + help（修訂 2026-09-21：目前只有 `download_dir`） |
| splash 彩蛋 | `V`，任何面板、任何 screen（家族同鍵；visual mode 為此改成小寫 `v`，2026-09-21） | 不揭露，是彩蛋 |
| 上一頁 / 下一頁 | `P` / `N` | help |
| 切面板 | `Tab`、`1`–`2` | footer + help |
| Visual mode（選取模式） | `v`（從 `[1]` 按 → 先把焦點移到 `[2]` 再進模式）；`/` 亦可 | help |
| 頁內搜尋 | `/`；同上 | help |
| 離開 | `q`（有下載進行中先 confirm；**浮層內不作用**，浮層只認 Esc）、`Ctrl+C` 硬退 | footer + help |
| 導覽詞彙 | §3 | help |

全域字母**在浮層開著、選取模式、打字中三種狀態下不作用**（sshu §4.8 同一判準）。

---

## §B. 元素專職化 in webu

| 元素 | 唯一語意 |
|---|---|
| 邊框 Blue / 雙線 | focus（結構） |
| `[1]` 綠字 | `[2]` 正在顯示的分頁 |
| 邊框 Yellow | 選取模式 |
| role glyph | 一個 role 一個 glyph，一張表定死，查字型不憑記憶；填值後 input 的 glyph 不變 |
| `[2]` 第一列 | 「你現在看的是什麼」：圖示 + URL（藍字，與 focus 同色是刻意的：都是「你在哪」）；圖示平時是 web、載入中換 live glyph；搜尋中暫換成 `/query`（修訂 2026-09-20） |
| footer | 五個 core-key；選取模式時只剩該模式有效的鍵（sshu 誠實規則） |
| `y` 小寫 / `Y` 大寫 | 游標下的東西 / 整頁 |
| `r` 小寫 / `R` 大寫 | 這個分頁 / 這一頁 |

---

## §1 兩種模式

`[2]` 有兩種游標，各管一種事：

| | 一般模式 | 選取模式（`/` 或 Space menu 的 Select text） |
|---|---|---|
| 游標單位 | item：互動節點 + heading；純文字是 item 之間的 flow | 字元 |
| 移動 | `j/k` item、`u/d` 半頁、`gg/G` | `hjkl`、`w/e/b`、`0/$`、`u/d`、`gg/G`（sshu copymode） |
| Enter | click item | click 字元所屬節點 |
| 選取 / 複製 | — | `v/V` 選、`y` 寫剪貼簿（OSC52 + pbcopy / wl-copy / xclip / xsel，kbu / filu 同套） |
| `/` | **自動進選取模式**並開搜尋 | 開搜尋 |
| 視覺 | Blue 邊框 | Yellow 邊框 |
| footer | core-key | 只剩選取模式有效鍵 |
| 全域字母 | 有效 | 無效（模式握著鍵盤） |
| 頁面即時更新 | 重畫、游標留位 | **凍結 IR 不重畫**（sshu copymode 凍結快照），live glyph 改 paused；Esc 離開時重畫 |
| 離開 | — | `Esc`；item 游標落到離字元游標最近的 item |

### 1.1 `/` 搜尋

1. 輸入放在 **`[2]` 第一列**（URL 列暫換成 `/query`），頁面不被遮，版面不動
2. 打字即高亮所有 match，`/query` 右側顯示命中計數 `3/17`；**smart case**（全小寫不分大小寫，含大寫就區分，同 vim / rg）
3. `Enter` 確認 → 字元游標跳第一個 match；`n` / `N` 下一個 / 上一個
4. 游標停在 match 上 Enter = 對它所屬的節點做 Enter 的事（§A.0.K：搜到連結 → confirm 後開）
5. `Esc` 分層：打字中 → 取消輸入、清高亮、留在選取模式；沒在打字 → 離開選取模式

---

## §2 文字輸入

### 2.1 所有 textbox 一個形狀

Enter 一律開 **input popup**（sshu inputPopup），不 inline 編輯：寬度、IME、多行都在
popup 一套解決，`[2]` 不用調整大小。

| popup 內 | 語意 |
|---|---|
| `Enter` | 確認：值寫回頁面（`DOM.focus` + `Input.insertText`），popup 關閉，**不送 Enter 給頁面** |
| `Tab` | 有 placeholder（goto popup 帶出的目前 URL）→ 接進輸入列編輯；否則 no-op |
| `Esc` | 取消，頁面不動 |

打字中屏蔽所有 hotkey（通用 §4.5）：`Space` 是空白、`?` 是問號。

### 2.2 空 vs 有值

| 狀態 | Enter | 顯示 |
|---|---|---|
| 空 | 直接開 input popup | `󰛿 label ______________` |
| 有值 | 同樣直接開 input popup，帶目前值（修訂 2026-09-21；原本開選單、cursor 在 Submit）。Submit / Edit / Clear / Yank 在 Space 選單，cursor 在 Submit | `󰛿 label ____value_____`，glyph 不變 |

**password**：popup 遮罩、prompt 寫 password、不帶舊值（Chromium 交出來的是 `•`，不是密碼）。欄位是不是 password 從
`page.Capture` 已經打的 DOMSnapshot 讀 `type=password`（`ir.Capture.Protected`），空的欄位也認得；AX tree 不說，原本只靠 value 的 `•` 猜、空的就漏了（修 2026-09-21）。

**search box**（role `searchbox`，即 `type=search`；或 search landmark 裡的任何框）：框的 Enter 把值寫回後**直接跳 confirm「Search」**（第一列是打的字），Enter = 送 Enter 給欄位（implicit submission，同 Submit）、Esc = 值留著不送（2026-09-21：搜尋框打字就是為了送出，不該再去 Space 選單找 Submit）。一般 input 與 search input 的分法就是 role：Chromium 把 `type=search` 給 `searchbox`、其他文字框給 `textbox`；沒標 type 但放在 `role=search` 表單裡的也當搜尋框。

**Submit** = 送一個 Enter keydown 給頁面（`Input.dispatchKeyEvent`）：沒有送出按鈕的搜尋框
靠這個送出，等同瀏覽器的 implicit submission。密碼欄顯示 `••••`。

`insertText` 後頁面長出的 autocomplete 建議會在重畫時出現在欄位下方的 item，`j` + Enter
即選，不特別處理。

### 2.3 textarea 與 editor 鏈

Enter 開多行 popup，空的、有值的都是（修訂 2026-09-21）；Space 選單：Edit / Edit in editor / Clear / Yank。

**Edit in editor** 的鏈：`$VISUAL` → `$EDITOR` → `vim` → `vi` → `nano` → **內建純文字
多行 popup**（無上色、無折行處理）。前五個在 pty popup 裡跑（sshu `[e]dit` 同路，多兩級）。

### 2.4 select

Enter 開 option 清單（menu），從 AX tree 的 option 節點列出，選完設 value 並 dispatch
`change`。

---

## §3 導覽詞彙

`nav.go` 一份，所有清單共用（sshu §4.2）：

| 鍵 | 動作 |
|---|---|
| `j` / `k` | 上下一列，**清單會繞**（`[1]` 與所有 menu popup）；`[2]` item 游標**不繞**（頁面有頭尾），且是**換列**：跳到下一列有 item 的列、落在欄位最接近的 item（修訂 2026-09-20：一列多個 item 之後，一列一列走才走得動）；頁面最上列再按 `k` 上到 URL 底下的 tray（膠囊列）、`j` 回到剛才的 item；`Esc` 也上得去、再 `Esc` 回來（2026-09-22） |
| `u` / `d` | 半頁，不繞 |
| `gg` / `G` | 頭 / 尾 |
| `h` / `l` | `[2]` 同一列的 item 之間左右移動（表格 row、導覽列、一段文字裡的多個連結都一樣；列尾不繞）；tray 上走膠囊、**會繞**（它是選單，2026-09-22）；DevTools popup 切分頁；其他地方無 |
| 方向鍵 | 同義 |
| `1`–`2` | 直達面板 |

導覽字母不被任何動作佔用（`h j k l u d g G`）。

**Mouse（負面規範，通用 §5）**：v1 不做。日後若加，必須是鍵盤的 mapping、不引入新語意。

---

## §4 Hotkey 分層與全表

規則（sshu §4.4）：**小寫 = item operation**、**大寫 = panel operation 或全域**；bracket
印的就是要按的鍵，大小寫算數；導覽字母不佔。

| 層 | 鍵 |
|---|---|
| 全域 | `W` `B` `H` `D` `S`、`q`、`?`、`V`（彩蛋）；只在 `[W]eb`：`P` `N` `L`、`/`、`v`、`Tab`、`1`–`2` |
| `[1]` item | `c` `o` `r` `y` |
| `[1]` panel | `T` `X` `U` |
| `[2]` item | **無**（menu-only） |
| `[2]` panel | `R` `T` `A` `O` `I` `Z` `Y` `C`（`v` 是全域的 visual mode，`L` 是全域的 location；`T` 與 `[1]` 同義；`C` 原是 `W`，修訂 2026-09-21） |
| Bookmarks screen | `a` `m` `r` `x` `y` / `A` `I` `/`（`a` 小寫 = 加書籤、`A` 大寫 = 加目錄，都在游標所在目錄下；`r` 改名（書籤 title 或目錄名）、`I` 匯入；2026-09-21） |
| Downloads screen | `o` `x` `y` / `C` `/` |
| History screen | `x` `y` / `C` `/` |
| Settings screen | Enter |
| DevTools | `x` `y` / `C` `/`；`h/l` 切分頁 |
| 選取模式 | `hjkl` `w` `e` `b` `0` `$` `u` `d` `gg` `G` `v` `V` `y` `/` `n` `N` |

撞字檢查：全域 `W B H D S P N L` 與各面板大寫 `R U A O I Z V Y C T X F` 無重疊；小寫 `v`（visual mode）與 `[1]` 的 `c o r y`、Bookmarks 的 `a m r x y`、Downloads 的 `o x y` 無重疊，visual mode 內的 `v`（開始選取）在模式裡、不同時可按；`V`（彩蛋）與各面板大寫無重疊（Bookmarks screen 的 `F` 是該 screen 的，與全域不撞）（修訂 2026-09-21：`W` `S` 進全域，page panel 的 close 改 `C`；screen 上的 `C` clear 與 page 的 `C`lose 在不同 screen，不會同時可按）。`D` 與導覽 `d` 只差大小寫，
sshu 的 `[D]isconnect` 同例；`L` 與導覽 `l`（沿列右移）同例；`I` 與 DevTools Console 的 `i`（insert）在不同 surface。
DevTools 原本的 `D` 讓給 header 的 Downloads（修訂 2026-09-20），改成 `[I]nspect`。`U` 只剩 `[1]` 的 undo close
（修訂 2026-09-20：原本 `[2]` 也用 `U` 開 goto、同字依面板不同義；goto 改成全域 `L` 後不再同字）。Bookmarks 的新目錄用 `F`
不用 `N`，避開全域 `N`。

---

## §5 浮層行為

沿用 u-family Popup Convention（ui.md §3）：一個 popup 一個檔一個 animator、動畫
8 × 16ms、`Esc` 只在 `closeTop` 一處解析、`Space` 在浮層上 = 關掉它、正在關閉的浮層
不握鍵盤、stack 預設保留 source。

**Context shift 清 source**（kbu §6.4.1）：Space menu 裡點了會換頁的動作（Open、Open in
new tab、Submit、goto），menu 這個 source 清掉，不回到 menu。

**先 confirm 的動作**（filu §4.6 / sshu §6.3）：

| 動作 | 為什麼 |
|---|---|
| `[1]` `c` 關分頁，該分頁有下載進行中 | 會中斷下載 |
| `q` 離開，有下載進行中 | 同上；沒有下載直接走 |
| Bookmark delete | 破壞性（Downloads 的 remove 不問：檔案還在，只是清列） |
| History clear、Clear site data | 破壞性 |
| 憑證錯誤「要繼續嗎」 | 安全；第一版每次問，不記住 |
| `beforeunload` | 頁面自己要求 |

---

## §6 時間軸

| 情境 | 行為 |
|---|---|
| 頁面載入中 | `[2]` 邊框 hint 顯示 loading；`[1]` 該列掛 live glyph。**修訂（2026-09-20，實機試用後）**：從按下鍵那一刻起 `[2]` 整頁變 dim、URL 列的圖示換成 live glyph，到新頁面落地為止；期間 `P` / `N` / `R` / item 的 click 一律吞掉（debounce）——終端機使用者按鍵很快，連按 `PPP` 只能算一次，不能一口氣退好幾頁。沒有上一頁時 toast 說「nothing to go back to」、頁面不動 |
| 即時更新（SSE / WebSocket） | 重畫不退階（通用 §7.2）；cursor 靠 backendDOMNodeId 留位，失敗用指紋（function.md §4） |
| `target=_blank` | 新分頁加到 `[1]` 尾端並**自動切換** |
| 任何會換頁的動作（點 link、goto 確認、Bookmarks / History 開啟——一律開新分頁（修訂 2026-09-21）、`[1]` Enter 切分頁） | context shift：Space menu 清掉；screen 回 `[W]eb`；`[2]` 第一列 URL 更新；item 游標回到 main 第一個 item；**焦點一律回 `[2]`** |
| 歷史記錄時機 | `Page.loadEventFired` 後的最終 URL + 標題；SPA 的 `Page.navigatedWithinDocument` 也記；`about:blank` 與錯誤頁不記 |
| 離開 | 存 session（所有分頁 URL）→ 殺 Chromium（u-family 子行程慣例） |
| 啟動 | 還原分頁但**不預先載入**，切到才載；未載入的分頁在 `[1]` 以 dim 顯示，切到時 `[2]` 先是 loading |
| 空狀態 | `[1]` 無分頁：「no tabs」+ 提示 `T`；`[2]` 無頁面：「no page」+ 提示 `L`（sshu empty.go 形狀） |

---

## §7 goto popup

`L`（`[L]ocation`，全域，對應 Chrome 的 Cmd+L；或 `[1]` / `[2]` 的 `T`）開 goto popup（title「Location」），filu goto picker 形式：

0. 開啟時輸入列是空的，以 dim 帶出目前分頁的 URL 當 placeholder（修訂 2026-09-20）：
   `Tab` 把它接進輸入列編輯、`Backspace` 整個清掉、直接打字則從頭來。
   placeholder 是提議不是值——Enter 送的是打出來的字，空的就什麼都不做。hint 列在有 placeholder 時多出 `Tab edit it` `Bksp clear`
1. 輸入列：打字即從 Bookmarks / History fuzzy 建議
2. Enter 時判斷輸入：
   - 像 URL（有 scheme、或 `host.tld` 形）→ 無 scheme 補 `https://`
   - 不像 URL → **當搜尋**，預設 Google（修訂 2026-09-21，原 DuckDuckGo），`config.yaml` 的 `search_engine` 可換（Settings screen 也能改）
3. `L` 開在目前分頁，`T` 開新分頁

---

## §8 待決

| # | 項目 | 狀態 |
|---|---|---|
| 1 | ~~History 的全域鍵~~ | **已決**：`H` |
| 2 | ~~`U` 同字不同義~~ | **已決**（2026-09-20 再修）：goto 改全域 `L`，`U` 只剩 `[1]` 的 undo close，不再同字 |
| 3 | `[2]` item operation 之後要不要補 letter hotkey | 先不做，用了再說 |
| 4 | link 色帶 | ui.md §4，畫出來再挑 |

---

## 附錄 — hotkey 全表

### Core key（跨 surface 不變）
`Tab` 切面板 · `Enter` click · `Esc` 關浮層 / 離開選取模式 / screen 回 Web · `Space` menu · `?` help

### 全域
`W` Web · `B` Bookmarks · `H` History · `D` Downloads · `S` Settings · `P` 上一頁 · `N` 下一頁 · `L` location · `q` quit · `/` 搜尋（進 visual mode）· `v` visual mode

### `[1]` Tabs
`c` close · `o` open in new tab · `r` reload · `y` yank url · `T` new · `X` close others · `U` undo close

### `[2]` Page
`R` reload · `T` new tab · `A` add bookmark · `O` outline · `I` inspect（DevTools：network / storage / console / source）· `Z` zoom · `Y` yank page url · `C` close this tab

### 導覽（跨 surface 同義）
`j/k` · `u/d` · `gg/G` · `h/l`（DevTools 分頁）· `1-2`
