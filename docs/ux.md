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
| **Contextual** | `Space` | footer 常駐 `space menu` | 當前 focus 的 contextual 動作 100% 在 Space menu 內。`[3]` 的 item operation **只走 Space menu、沒有 letter hotkey**（決定 2026-09-20：小寫 hotkey 造成困惑，先不做） |
| **Non-contextual** | `?` | footer 常駐 `? help` | 全域動作 100% 在 help popup 內；`[1]` 面板是三個全域 popup 的常駐 ambient 揭露（Layer 2） |

### §A.0.K core-key 語意

| Core-key | 一般模式 | 選取模式（§1） | 對應通用 |
|---|---|---|---|
| `Tab` | `[1]` → `[2]` → `[3]` 繞；`1`–`3` 直達 | 同 | §4.1 |
| `Enter` | **開游標所在 item 的 item operation 選單**，第一列是主要動作，再按 Enter 執行（修訂 2026-09-20，見下） | 離開模式、對字元游標所屬的節點開同一個選單 | §4.1 |
| `Esc` | 關最上層浮層；沒有浮層時**無作用**（上一頁是 `P`，Esc 不兼職） | 打字中：取消搜尋輸入；否則離開選取模式 | §4.3 |
| `Space` | **滑鼠右鍵 context menu**；再按關閉；在浮層上按 = 關掉它 | 熱鍵 cheatsheet，按列出的鍵即執行並關閉 | §A.1 |
| `?` | help；再按關閉；可疊在任何浮層上 | 同 | §A.2 |

**修訂（2026-09-20，實機試用後）**：Enter 不再直接 click。**Enter = 開該 item 的 item
operation 選單**，第一列是主要動作（link 的 Open、button 的 Click、空 textbox 的 Edit、有值
textbox 的 Submit、select 的 Choose），再按一次 Enter 執行；**Space = 完整選單**（item operation
+ panel operation）。理由：Enter 底下原本只有一個看不見的動作，使用者按下去不知道發生了什麼；
開選單讓「對這個東西能做什麼」被揭露，主要動作只多一次 Enter。原本「有值 textbox 是唯一例外」
的規則因此不再是例外，而是通則。

**與 filu 一致了**：filu 的 Enter 只進目錄不開檔；webu 的 Enter 也只開清單，不直接交給頁面。
VTP §A.0.K 只要求同一 app 內跨 surface 不變。

**不是 core-key 的**：`q` 離開、`Ctrl+C` 硬退、`P` / `N` 前後頁、`/` 搜尋、`[1]` 三個全域
字母。它們是 §A.2 軌的動作，全部列在 `?` help。選取模式沒有自己的全域鍵（修訂 2026-09-20：
`Alt+v` 拿掉，item 游標已經夠用；模式只為複製文字與搜尋，由 `/` 或 Space menu 的 Select text 進入）。

### §A.1 Contextual track — Space menu

每個 focus 按 `Space` 列出「這裡能做什麼」。兩個 region 固定叫 `item operation` /
`panel operation`（kbu / filu / sshu 同字串）；只有一個 region 就保持扁平；一列的 menu
不開、直接執行（sshu §11.16）。列的形狀 `[X]label` + 靠右說明，有 hotkey 才加 bracket。

**`[1]`**：每項只有「開啟該 popup」一個動作 → 一列 menu 不開，**Space = Enter**。

**`[2]` Tabs**

| item operation | panel operation |
|---|---|
| Enter 切換到此分頁、`[w] Close`（該分頁有下載進行中 → 先 confirm）、`[c] Clone`、`[r] Reload`、`[y] Yank url` | `[T] New tab`（開 goto popup）、`[X] Close others`、`[U] Undo close` |

**`[3]` 一般模式，item operation 依 role**（無 hotkey，menu-only）

| role | item operation |
|---|---|
| link | Open（= Enter）、Open in new tab、Yank url |
| button / checkbox / radio / switch | Click（= Enter） |
| textbox（空） | Edit（= Enter，開 input popup） |
| textbox（有值） | **Submit**（送 Enter 給頁面）、Edit、Clear、Yank；cursor 預設停 Submit（sshu §6.6.1 主要意圖優先） |
| textarea | Edit（多行 popup）、Edit in editor（§2 editor 鏈）、Clear、Yank；無 Submit（瀏覽器裡 textarea 的 Enter 是換行） |
| select | Choose（= Enter，option 清單 menu） |
| image / media 佔位框 | Click（= Enter）、Yank url |
| heading | Fold section：摺到下一個同級或更高級 heading 為止，換頁即忘 |
| 所有 item 共有 | Yank text、Inspect（message 類 popup：role、name、states、backendDOMNodeId、href 或 src） |
| 未支援 role | 第一列 disabled：「role: slider，尚未支援，只能 click」（function.md §3 fallback） |

頁面**沒有任何 item**（純文字頁）：Space menu 只剩 panel operation，扁平不分 region。

**`[3]` 一般模式，panel operation**：`[R] Reload`、`[P] Previous`、`[N] Next`、
`[/] Search`（進選取模式）、`Select text`（無 hotkey，進選取模式）、`[U] Go to URL`、`[A] Add to…`
（picker：Bookmarks / Shortcuts，多對象可選 → menu）、`[O] Outline`、`[D] DevTools`、
`[Z] Zoom`、`[V] View source`、`[Y] Yank page url`、`[W] Close`（關掉 `[3]` 正在顯示的分頁；
`[2]` 的小寫 `w` 關的是游標列，同字依 focus 面板不同義，修訂 2026-09-20）。

Outline 與 DevTools 的作用對象是目前頁面 → contextual → 在這裡，不在 `[1]`、不在 §A.2
（決定 2026-09-20）。使用者不用切面板，體驗留在頁面上。

**`[3]` 選取模式**：cheatsheet popup，列 `hjkl`、`w/e/b`、`0/$`、`u/d`、`gg/G`、`v/V`、
`y`、`/`、`n/N`、`Esc`。**按任一列出的鍵即執行並關閉 popup**（一步揭露，§A.0）。

**五個 popup 內部**

| popup | item operation | panel operation |
|---|---|---|
| Bookmarks | Enter 開啟、`[o] Open in new tab`、`[e] Edit`（form）、`[x] Delete`（confirm）、`[m] Move`（目錄 picker）、`[y] Yank` | `[A] Add` 目前頁、`[F] Folder` 新目錄、`[/]` 搜尋 |
| Shortcuts | Enter、`[o]`、`[e]`、`[x]`、`[y]` | `[A] Add` 目前頁 |
| History | Enter、`[o]`、`[x] Delete` 該筆 | `[C] Clear`（confirm；唯一清除入口，歷史無限保留） |
| DevTools › Storage | `[x] Delete`、`[y] Yank value` | `[C] Clear site data`（confirm）、`[/]` 過濾 |
| DevTools › Network | Enter detail | `[C] Clear`、`[/]` |
| DevTools › Console | — | `[C] Clear`、`[/]` |
| Outline | Enter 跳轉 | — |

delete 用 `x` 不用 `d`：`d` 是半頁（sshu `[x] Delete`）。

### §A.2 Non-contextual track — `?` help

| 全域動作 | 鍵 | 揭露 |
|---|---|---|
| Bookmarks popup | `B` | `[1]` 面板 + help |
| Shortcuts popup | `S` | `[1]` + help |
| History popup | `H` | `[1]` + help |
| 上一頁 / 下一頁 | `P` / `N` | help |
| 切面板 | `Tab`、`1`–`3` | footer + help |
| 選取模式 | 無全域鍵（修訂 2026-09-20）：`/` 或 `[3]` Space menu 的 Select text | help |
| 頁內搜尋 | `/`；同上 | help |
| 離開 | `q`（有下載進行中先 confirm；**浮層內不作用**，浮層只認 Esc）、`Ctrl+C` 硬退 | footer + help |
| 導覽詞彙 | §3 | help |

全域字母**在浮層開著、選取模式、打字中三種狀態下不作用**（sshu §4.8 同一判準）。

---

## §B. 元素專職化 in webu

| 元素 | 唯一語意 |
|---|---|
| 邊框 Blue / 雙線 | focus（結構） |
| `[2]` 綠字 | `[3]` 正在顯示的分頁 |
| 邊框 Yellow | 選取模式 |
| role glyph | 一個 role 一個 glyph，一張表定死，查字型不憑記憶；填值後 input 的 glyph 不變 |
| `[3]` 第一列 | 「你現在看的是什麼」：URL；搜尋中暫換成 `/query` |
| footer | 五個 core-key；選取模式時只剩該模式有效的鍵（sshu 誠實規則） |
| `y` 小寫 / `Y` 大寫 | 游標下的東西 / 整頁 |
| `r` 小寫 / `R` 大寫 | 這個分頁 / 這一頁 |

---

## §1 兩種模式

`[3]` 有兩種游標，各管一種事：

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

1. 輸入放在 **`[3]` 第一列**（URL 列暫換成 `/query`），頁面不被遮，版面不動
2. 打字即高亮所有 match，`/query` 右側顯示命中計數 `3/17`；**smart case**（全小寫不分大小寫，含大寫就區分，同 vim / rg）
3. `Enter` 確認 → 字元游標跳第一個 match；`n` / `N` 下一個 / 上一個
4. 游標停在 match 上 Enter = click 它所屬的節點（搜到連結直接開）
5. `Esc` 分層：打字中 → 取消輸入、清高亮、留在選取模式；沒在打字 → 離開選取模式

---

## §2 文字輸入

### 2.1 所有 textbox 一個形狀

Enter 一律開 **input popup**（sshu inputPopup），不 inline 編輯：寬度、IME、多行都在
popup 一套解決，`[3]` 不用調整大小。

| popup 內 | 語意 |
|---|---|
| `Enter` | 確認：值寫回頁面（`DOM.focus` + `Input.insertText`），popup 關閉，**不送 Enter 給頁面** |
| `Tab` | no-op |
| `Esc` | 取消，頁面不動 |

打字中屏蔽所有 hotkey（通用 §4.5）：`Space` 是空白、`?` 是問號。

### 2.2 空 vs 有值

| 狀態 | Enter | 顯示 |
|---|---|---|
| 空 | 直接開 input popup | `󰛿 label ______________` |
| 有值 | 開選單（= Space menu：Submit / Edit / Clear / Yank，cursor 在 Submit） | `󰛿 label ____value_____`，glyph 不變 |

**Submit** = 送一個 Enter keydown 給頁面（`Input.dispatchKeyEvent`）：沒有送出按鈕的搜尋框
靠這個送出，等同瀏覽器的 implicit submission。密碼欄顯示 `••••`。

`insertText` 後頁面長出的 autocomplete 建議會在重畫時出現在欄位下方的 item，`j` + Enter
即選，不特別處理。

### 2.3 textarea 與 editor 鏈

空的 Enter 開多行 popup；有值 Enter 開選單（Edit / Edit in editor / Clear / Yank）。

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
| `j` / `k` | 上下一列，**清單會繞**（`[1]` `[2]` 與所有 menu popup）；`[3]` item 游標**不繞**（頁面有頭尾） |
| `u` / `d` | 半頁，不繞 |
| `gg` / `G` | 頭 / 尾 |
| `h` / `l` | `[3]` 表格 row 內於互動 cell 之間移動（cursor 停在 row，Enter 是 click 整列，cell 裡的連結靠 h / l 到）；DevTools popup 切分頁；其他地方無 |
| 方向鍵 | 同義 |
| `1`–`3` | 直達面板 |

導覽字母不被任何動作佔用（`h j k l u d g G`）。

**Mouse（負面規範，通用 §5）**：v1 不做。日後若加，必須是鍵盤的 mapping、不引入新語意。

---

## §4 Hotkey 分層與全表

規則（sshu §4.4）：**小寫 = item operation**、**大寫 = panel operation 或全域**；bracket
印的就是要按的鍵，大小寫算數；導覽字母不佔。

| 層 | 鍵 |
|---|---|
| 全域 | `B` `S` `H` `P` `N`、`q`、`?`、`/`、`Tab`、`1`–`3` |
| `[2]` item | `w` `c` `r` `y` |
| `[2]` panel | `T` `X` `U` |
| `[3]` item | **無**（menu-only） |
| `[3]` panel | `R` `U` `A` `O` `D` `Z` `V` `Y` `W` |
| Bookmarks popup | `o` `e` `x` `m` `y` / `A` `F` `/` |
| Shortcuts popup | `o` `e` `x` `y` / `A` |
| History popup | `o` `x` / `C` |
| DevTools | `x` `y` / `C` `/`；`h/l` 切分頁 |
| 選取模式 | `hjkl` `w` `e` `b` `0` `$` `u` `d` `gg` `G` `v` `V` `y` `/` `n` `N` |

撞字檢查：全域 `B S H P N` 與各面板大寫 `R U A O D Z V Y W T X C F` 無重疊。`D` 與導覽 `d` 只差大小寫，
sshu 的 `[D]isconnect` 同例。`U` 在 `[2]`
是 undo close、在 `[3]` 是 go to URL，同字依 focus 面板不同義（決定 2026-09-20，kbu 的 `C` 同例）。Bookmarks 的新目錄用 `F`
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
| `[2]` `w` 關分頁，該分頁有下載進行中 | 會中斷下載 |
| `q` 離開，有下載進行中 | 同上；沒有下載直接走 |
| Bookmark / Shortcut delete | 破壞性 |
| History clear、Clear site data | 破壞性 |
| 憑證錯誤「要繼續嗎」 | 安全；第一版每次問，不記住 |
| `beforeunload` | 頁面自己要求 |

---

## §6 時間軸

| 情境 | 行為 |
|---|---|
| 頁面載入中 | `[3]` 邊框 hint 顯示 loading；`[2]` 該列掛 live glyph |
| 即時更新（SSE / WebSocket） | 重畫不退階（通用 §7.2）；cursor 靠 backendDOMNodeId 留位，失敗用指紋（function.md §4） |
| `target=_blank` | 新分頁加到 `[2]` 尾端並**自動切換** |
| 任何會換頁的動作（點 link、goto 確認、Bookmarks / Shortcuts / History 開啟、`[2]` Enter 切分頁） | context shift：Space menu 清掉；`[3]` 第一列 URL 更新；item 游標回到 main 第一個 item；**焦點一律回 `[3]`** |
| 歷史記錄時機 | `Page.loadEventFired` 後的最終 URL + 標題；SPA 的 `Page.navigatedWithinDocument` 也記；`about:blank` 與錯誤頁不記 |
| 離開 | 存 session（所有分頁 URL）→ 殺 Chromium（u-family 子行程慣例） |
| 啟動 | 還原分頁但**不預先載入**，切到才載；未載入的分頁在 `[2]` 以 dim 顯示，切到時 `[3]` 先是 loading |
| 空狀態 | `[2]` 無分頁：「no tabs」+ 提示 `T`；`[3]` 無頁面：「no page」+ 提示 `U`（sshu empty.go 形狀） |

---

## §7 goto popup

`U`（或 `[2]` 的 `T`）開 goto popup，filu goto picker 形式：

1. 輸入列：打字即從 Bookmarks / Shortcuts / History fuzzy 建議
2. Enter 時判斷輸入：
   - 像 URL（有 scheme、或 `host.tld` 形）→ 無 scheme 補 `https://`
   - 不像 URL → **當搜尋**，預設 DuckDuckGo，`config.yaml` 的 `search_engine` 可換
3. `U` 開在目前分頁，`T` 開新分頁

---

## §8 待決

| # | 項目 | 狀態 |
|---|---|---|
| 1 | ~~History 的全域鍵~~ | **已決**：`H` |
| 2 | ~~`U` 同字不同義~~ | **已決**：依 focus 面板不同義，不是全域鍵 |
| 3 | `[3]` item operation 之後要不要補 letter hotkey | 先不做，用了再說 |
| 4 | link 色帶 | ui.md §4，畫出來再挑 |

---

## 附錄 — hotkey 全表

### Core key（跨 surface 不變）
`Tab` 切面板 · `Enter` click · `Esc` 關浮層 / 離開選取模式 · `Space` menu · `?` help

### 全域
`B` Bookmarks · `S` Shortcuts · `H` History · `P` 上一頁 · `N` 下一頁 · `q` quit · `/` 搜尋（進選取模式）

### `[2]` Tabs
`w` close · `c` clone · `r` reload · `y` yank url · `T` new · `X` close others · `U` undo close

### `[3]` Page
`R` reload · `U` go to URL · `A` add to · `O` outline · `D` devtools · `Z` zoom · `V` view source · `Y` yank page url · `W` close this tab

### 導覽（跨 surface 同義）
`j/k` · `u/d` · `gg/G` · `h/l`（DevTools 分頁）· `1-3`
