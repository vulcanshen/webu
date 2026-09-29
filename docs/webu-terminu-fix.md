# webu — terminu fix

webu 還沒符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.17/principle)（tdp v0.1.17）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

> **v0.1.17（2026-09-29，這份清單寫完後才出）**：只補了 M5 三點，已照它調整本清單 —— menu 與 key reference **說明欄**裡提到的鍵
> 算句子，加方括號；**README** 內文的鍵用 Markdown code 標（`` `Enter` ``），不加方括號，鍵名與寫法照 M5；**別的工具自己的按鍵**
> （tmux 的 `prefix l`、`C-a x`）照那個工具的寫法。其餘條目照 v0.1.14–v0.1.16。

盤點日期：2026-09-29（對照 tdp v0.1.16）。以 `main` 的 `c08db13` 為準（工作樹乾淨）。webu 在第八輪已對齊 v0.1.13，這一輪**只核對
v0.1.13 → v0.1.16 的改動**：

- v0.1.14：F1 / F8 / K11 的 toast、K10 / D5 的 PTY 出口鍵、術語「模式」（zoom 不是模式）、M5 按鍵的寫法、M6 的 key reference。
- v0.1.15：M5 的寫法全部定案 —— 鍵名（鍵帽上的名字、大駝峰、不自創縮寫）、`/` 與 `–`、句子裡的鍵加方括號、hint / footer 寫
  `鍵:說明`、key reference 兩欄；D1–D5 的例子跟著改，D2 定了鍵與說明的顏色。
- v0.1.16：D5 的 `Alt-Esc` 一律先 confirm；M6 補「說明別的 surface 的一段照亮顯示」。

每條照內容對程式碼核對過，位置寫檔案與函式，不寫行號。key reference 的亮暗、zoom、toast 是在 scratch 複本裡 render 量的
（100 × 40、`keysDriver`，沒有動 webu 的工作樹）。表裡的「現在」是畫面上印出來的樣子（去掉顏色）。

核對 M6 與 zoom 時碰到兩件不是這三版新增、但前幾輪沒抓到的事，一起列進來並在標題標明：Space menu 該變暗卻沒變暗的列（第 6 條，
M6 的 menu 那一半從 v0.1.0 就是規則）、zoom 裡 focus 跑到沒畫出來的 panel（第 9 條，L5）。


## 先看

- **清單先 commit，再動程式**；程式的 commit 跟清單分開，commit 只加自己改的路徑。
- **每修一處補 model test，逐處 mutation**：把修正單獨改回舊行為，確認對應的測試會紅。編譯不過不算抓到；預期值寫死，不用被測的
  函式算；量顏色要開顏色（`dim_test.go` 的 `withColour`）。
- **M5 會改到好幾個斷言畫面字串的測試**（第 1、2 條各自列了）。特別看**否定的斷言**：舊字串改完之後本來就不會出現，
  `strings.Contains(f, "space")` 這種檢查會永遠通過、什麼都守不到，要改成新寫法（`Space`）。
- **同一個 commit 同步 README 兩份與 `docs/dev-remarks.md`**（`ux.md` / `ui.md` 有寫到的那一段也一起）；CHANGELOG 記在 `[Unreleased]`。
- **修完拿 v0.1.17 的 rules 與 defaults 全文再逐條對一次**，不只看 CHANGELOG。`dev-remarks.md`「webu 照 tdp v0.1.13 逐條修完
  （2026-09-28）」那句，這次改連結時刻意沒動（還沒對齊）；修完改成 v0.1.17 與修完的日期（第八輪的經驗：寫死的版本號每輪都要對）。
- **不 push、不發版**：家族與 tdp 都穩定之前不發 release。
- 修完把這一輪寫進 terminu repo 的 `.local/family-fix/webu/README.md`：開頭的清單加第 9 點，另加一節「第九輪（v0.1.14–v0.1.16）」
  （修了什麼、commit、給下一個 app 的經驗）。
- 建議順序：第 1–4 條（M5）先做，改的是共用的畫法與字串；第 2 條與第 5 條都動 `helpPopup.view()`，可以一起做；第 5 條（`helpEntry`
  帶 `disabled`）是第 6、7 條的地基；第 8、9 條獨立。


## 1. hint 與 footer 沒寫成 `鍵:說明` —— M5、D1、D2（v0.1.15）

**現況**：

- 共用的畫法：
  - `internal/ui/popup.go` `hintLegend()`（每個 popup 的下框 hint、list screen 的下框、`[2]` 的下框）：一項寫「鍵 + 空白 + 說明」，
    項與項之間兩個空白，前後各一個空白。鍵 Blue（`focusColor` `#89b4fa`）、說明 Overlay0（`dimColor` `#6c7086`）—— 顏色已符合 D2。
  - `internal/ui/chrome.go` `keyLegend()`（footer）：一項寫「鍵 + 空白 + 說明」，項與項之間三個空白；寬度不夠從尾端整組捨棄（`plainW`
    照這個間距算）。鍵 Blue、說明 Overlay0 —— 顏色已符合。
- footer（`app.go` `footer()`、`selectmode.go` `selectLegendPairs()`）：

  | 位置 | 現在 | 改成 |
  |---|---|---|
  | web | `space menu   ? help   tab/1-2 panels   q quit` | `Space:menu ?:help Tab/1–2:panels q:quit` |
  | list screen | `space menu   ? help   esc web   q quit` | `Space:menu ?:help Esc:web q:quit` |
  | visual mode | `? help   y copy   v/V select   Esc leave   / search   n/N next/prev   Enter click   hjkl move   w/e/b word   0/$ line ends   u/d half page` | `?:help y:copy v/V:select Esc:leave /:search n/N:next/prev Enter:click h/j/k/l:move w/e/b:word 0/$:line ends u/d:half page` |
  | visual mode 打字中 | `Enter find   Esc cancel` | `Enter:find Esc:cancel` |

- 下框 hint（`hintLegend()` 的呼叫處）：

  | 位置 | 現在 | 改成 |
  |---|---|---|
  | `spacemenu.go` `view()`（Space menu、global operation、options、choices 共用） | `j/k move  Enter run  Esc close`；沒有列時 `Esc close` | `j/k:move Enter:run Esc:close`；`Esc:close` |
  | `confirm.go` `view()`（含 quit confirm） | `Enter <動詞>  Esc cancel` | `Enter:<動詞> Esc:cancel` |
  | `inputpopup.go` `legend()` | `Enter <動詞>  Tab next field  Tab accept`（單欄；group 是 `→ accept`）`  Bksp decline  Esc cancel` | `Enter:<動詞> Tab:next field Tab:accept`（group 是 `→:accept`）` Backspace:decline Esc:cancel` |
  | `editorpopup.go` `view()` 寫入狀態 | `Enter new line  Esc out to the box` | `Enter:new line Esc:out to the box` |
  | `editorpopup.go` `view()` 移動狀態 | `hjkl move  i write  Enter set  Esc cancel` | `h/j/k/l:move i:write Enter:set Esc:cancel` |
  | `filepicker.go` `view()` | `↑↓ select  Enter open / pick  Bksp up  Esc cancel` | `↑/↓:select Enter:open / pick Backspace:up Esc:cancel` |
  | `finder.go` `titleAndHint()` go | `0-9 filter  j/k move  Enter go  Esc close` | `0–9:filter j/k:move Enter:go Esc:close` |
  | `finder.go` `titleAndHint()` 清單 | `j/k/u/d move  Enter go  Tab query  Esc close` | `j/k/u/d:move Enter:go Tab:query Esc:close` |
  | `finder.go` `titleAndHint()` 打字 | `Tab list  Esc close` | `Tab:list Esc:close` |
  | `helppopup.go` `view()`（key reference 自己） | `Esc close`；放不下時 `j/k scroll  Esc close` | `Esc:close`；`j/k:scroll Esc:close` |
  | `messagepopup.go` `view()` | 同上 | 同上 |
  | `toast.go` `view()` | `Esc close` | `Esc:close` |
  | `devtools.go` `view()` 打字 | `Enter done  Esc clear` | `Enter:done Esc:clear` |
  | `devtools.go` `view()` Storage | `x delete  y yank value  C clear site data  / filter  h/l tab  Esc close` | `x:delete y:yank value C:clear site data /:filter h/l:tab Esc:close` |
  | `devtools.go` `view()` Network | `Enter detail  C clear  / filter  h/l tab  Esc close` | `Enter:detail C:clear /:filter h/l:tab Esc:close` |
  | `devtools.go` `view()` Source | `j/k scroll  u/d half page  / grep  h/l tab  Esc close` | `j/k:scroll u/d:half page /:grep h/l:tab Esc:close` |
  | `devtools.go` `view()` Console | `Enter detail  i insert: eval  C clear  / filter  h/l tab  Esc close` | `Enter:detail i:eval C:clear /:filter h/l:tab Esc:close`（說明本身的冒號拿掉，不然讀成 `i:insert: eval`） |
  | `devnetdetail.go` `view()` | `j/k scroll  u/d half page  Esc close` | `j/k:scroll u/d:half page Esc:close` |
  | `listpanel.go` `hintPairs()` 打字 | `Enter done  Esc clear` | `Enter:done Esc:clear` |
  | `listpanel.go` `hintPairs()` Settings | `Enter edit`（開關是 `Enter toggle`）`  Esc web` | `Enter:edit`（`Enter:toggle`）` Esc:web` |
  | `listpanel.go` `hintPairs()` Downloads | `Enter open file  o source in new tab  x remove  y yank path  C clear done  / filter  Esc web` | `Enter:open file o:source in new tab x:remove y:yank path C:clear done /:filter Esc:web` |
  | `listpanel.go` `hintPairs()` History | `Enter open in new tab  x delete  y yank url  C clear  / filter  Esc web` | `Enter:open in new tab x:delete y:yank url C:clear /:filter Esc:web` |
  | `listpanel.go` `hintPairs()` Bookmarks | `Enter open in new tab`（目錄是 `expand / collapse`）`  a add  m move  r rename  x delete  y yank url  A add folder  I import  / filter  Esc web` | `Enter:open in new tab`（`Enter:expand / collapse`）` a:add m:move r:rename x:delete y:yank url A:add folder I:import /:filter Esc:web` |
  | `pagepanel.go` `pagePopupFloats()`（頁面彈窗最上面那個） | `Enter act  Space menu  Esc does not close it` | `Enter:act Space:menu Esc:does not close it` |
  | `parts.go` `partHint()`（`[2]` 下框，手在 pagetab 上） | `12 items · Enter to stay` | `12 items Enter:stay`（狀態字照舊，鍵的部分照 hint） |

**規則**：M5（v0.1.15）—— 鍵名用鍵帽上的名字、大駝峰、不自創縮寫（`Backspace`，方向鍵 `↑ ↓ ← →`）；幾個鍵做同一件事用 `/`，範圍用
`–`；hint 與 footer 寫 `鍵:說明`，冒號前後不空格，項目之間一個空格，不再用 ` · `、兩個空格。D1 —— footer `Space:menu ?:help
Tab/1–N:panels q:quit`，寬度不夠從尾端整組捨棄。D2 —— hint、footer 的鍵 Blue `#89b4fa`，冒號與說明 Overlay0 `#6c7086`。D3 / D4 ——
confirm `Enter:<動詞> Esc:cancel`，menu `j/k:move Enter:run Esc:close`。

**怎麼改**：`hintLegend()` 與 `keyLegend()` 改成 `鍵 + ":" + 說明`，項目之間一個空白，冒號跟說明同一個 Overlay0；`keyLegend()` 的
`plainW` 跟著新間距算。字串照上表改。

**實作注意**：

- `[2]` 的狀態字也走 `hintLegend()`：`pagepanel.go` `panelFrame()` 與 `app.go` `pagePanel()` 把 `loading`、`a popup is up: answer it`、
  `2/26 · 節名 · 40%`、`1-20 of 80` 當成一項 `{hint, ""}` 丟進去，整串畫成鍵的 Blue。改成 `鍵:說明` 之後，說明是空的會多出一個冒號。
  這些不是鍵，建議不再經過 `hintLegend()`，另外畫（顏色由 app 決定；不是鍵就不必用 Blue）。
- `Bksp` → `Backspace` 多 5 格。input popup 在 80 欄時內寬 76，最寬的 legend（`Enter:add Tab:next field →:accept Backspace:decline
  Esc:cancel`）約 64 格，放得下；`TestInputGroupLegendFits` 會量。

**測試**（斷言畫面字串的要跟著改）：

- `app_test.go`：input 的 legend 斷言 `"Tab accept"`、`"Bksp decline"`。
- `keys_test.go` `TestInputGroupLegendFits`：`"Esc cancel"`、`"→ accept"`、`"Bksp decline"`。
- `keys_test.go` `TestVisualModeFooterShowsHelp`：`HasPrefix(…, "? help")` 與否定的 `Contains(f, "space")` —— 後者改完會永遠通過，
  要改成檢查 `Space`。
- 另外補一個掃描測試：所有 footer、每一種 hint 的輸出，去掉顏色後不出現 ` · `、兩個以上連續的空白、`Bksp`、小寫的 `space` / `esc` /
  `tab`、`+` 連接的 modifier；開顏色時冒號是 Overlay0。mutation：把 `hintLegend()` 的冒號拿掉、把一處改回 `Bksp`，各自要紅。


## 2. key reference 的鍵欄寫法與顏色 —— M5、D2（v0.1.15）

**現況**：`internal/ui/helppopup.go`。`helpPopup.view()` 兩欄：鍵用 `handColor`（Subtext1 `#bac2de`）、說明用 `textColor`（Text
`#cdd6f4`）；鍵不加括號、不加冒號（已符合）。幾個鍵放在一格時用 ` · ` 或空白隔開：

| 清單 | 現在 | 改成 |
|---|---|---|
| `coreKeys` | `Tab · 1 · 2`、`j · k`、`h · l`、`u · d`、`gg · G`、`q · Ctrl+C` | `Tab/1–2`、`j/k`、`h/l`、`u/d`、`gg/G`、`q/Ctrl-C` |
| `helpMenu`、`helpOptions` | `j · k`、`u · d`、`gg · G` | `j/k`、`u/d`、`gg/G` |
| `helpFinder` | `j · k · u · d` | `j/k/u/d` |
| `helpGo` | `0-9`、`j · k` | `0–9`、`j/k` |
| `helpEditor` | `h j k l`、`i · a · A · o` | `h/j/k/l`、`i/a/A/o` |
| `helpMessage` | `j · k` | `j/k` |
| `helpDevtools` | `h · l`、`j · k · u · d`、`x · y` | `h/l`、`j/k/u/d`、`x/y` |
| `floatHelp()` 的 Space menu | `Space · Esc` | `Space/Esc` |
| `selectmode.go` `selectKeys` | `h j k l`、`w e b`、`0 $`、`u d`、`gg G`、`v V` | `h/j/k/l`、`w/e/b`、`0/$`、`u/d`、`gg/G`、`v/V` |

`helpMenu` / `helpOptions` 鍵欄的 `a row's key` 不是鍵名，是說明「每一列自己的鍵」，照舊。說明欄裡提到的鍵見第 3 條。

**規則**：M5（v0.1.15）—— key reference 兩欄，鍵不加括號、不加冒號；鍵名與 `/`、`–` 照鍵名那一節；modifier 用 `-`、`Ctrl` 後面的字母
大寫。D2 —— key reference 的鍵 Blue `#89b4fa`、說明 Text `#cdd6f4`。

**怎麼改**：`helpPopup.view()` 的鍵改用 `focusColor`；上表的字串照改。**測試**：`keys_test.go` `TestQuestionMarkOnAPanelReads` 的
`helpHas(e, "q · Ctrl+C")`；同檔的 `helpHas(d.m.help.entries, "Space · Esc")` 與否定的 `helpHas(…, "W · B · H · D · S")`（否定那個改成
新寫法 `W/B/H/D/S`，不然永遠通過）。補一個掃描：每個 panel 的 `keyReference()` 輸出與每一份浮層的 help 清單，鍵欄不出現 ` · `、空白、
`+`、當範圍用的 `-`；開顏色 render 出來鍵是 Blue。第 5 條也動這個函式，可以一起做。


## 3. 句子裡的鍵沒加方括號 —— M5（v0.1.15）

**現況**：空狀態、toast、confirm / message 的內文、input 的提示、menu 的說明欄裡的鍵都是光寫。空狀態由 `internal/ui/empty.go`
`emptyHint()` 依整個字比對標出鍵、`renderHint()` 把鍵畫成 `handColor`、其餘 Overlay0。

| 位置 | 現在 | 改成 |
|---|---|---|
| 空狀態 `sidebar.go` `tabsBody()` | `Press T to open one` | `Press [T] to open one` |
| 空狀態 `pagepanel.go` `pageBody()` | `Press L to enter a location, or T for a new tab` | `Press [L] to enter a location, or [T] for a new tab` |
| 空狀態 `pagepanel.go` `pageBody()` 載入失敗 | `<錯誤> — press R to retry` | `<錯誤> — press [R] to retry` |
| 空狀態 `pagepanel.go` `pageBody()` 還沒載入 | `Press R to load it` | `Press [R] to load it` |
| 空狀態 `listpanel.go` `panel()` 過濾沒結果 | `Press Esc to clear the filter` | `Press [Esc] to clear the filter` |
| 空狀態 `listpanel.go` `emptyState()` History | `Pages you visit are listed here; W is the web` | `…; [W] is the web` |
| 空狀態 `listpanel.go` `emptyState()` Downloads | `Files the page saves are listed here; W is the web` | `…; [W] is the web` |
| 空狀態 `listpanel.go` `emptyState()` Bookmarks | `Press a to add one, A for a folder, or W for the web` | `Press [a] to add one, [A] for a folder, or [W] for the web` |
| splash `splash.go` `render()` | `Press Esc to close` | `Press [Esc] to close` |
| toast `app.go` `routeKey()`（visual mode 的 `Tab` / `1` / `2`） | `leave visual mode first (Esc), then switch panels` | `leave visual mode first with [Esc], then switch panels` |
| toast `app.go` `togglePagetab()` | `this popup wants an answer: Esc does not close it` | `this popup wants an answer: [Esc] does not close it` |
| toast `app.go` `listAction()` | `m moves a bookmark; put the cursor on one` | `[m] moves a bookmark; put the cursor on one` |
| confirm 內文 `app.go` `inputKey()`（搜尋框寫回後） | `Enter in the field: the page searches` | `[Enter] in the field: the page searches` |
| message 內文 `app.go` `enterOn()` | `Nothing is defined for Enter on this item yet.`、`Space lists what can be done with it.` | `…for [Enter] on this item yet.`、`[Space] lists what can be done with it.` |
| input 提示 `app.go` `openEvalPrompt()` | `JavaScript, run in the page (Esc ends)` | `JavaScript, run in the page ([Esc] ends)` |
| input 提示 `settings.go` `settingBox()` | `…; Backspace then Enter for the default` | `…; [Backspace] then [Enter] for the default` |
| menu 說明 `app.go` `pageMenuItems()` 的 `[/] Search` | `every part of the page; Enter goes there` | `every part of the page; [Enter] goes there` |
| menu 說明 `app.go` `pagetabItem()`（在 pagetab 上） | `h/l show a part, Enter stays on it` | `[h]/[l] show a part, [Enter] stays on it` |
| menu 說明 `app.go` `textboxItems()` 的 Submit | `press Enter in the field` | `press [Enter] in the field` |
| menu 說明 `helppopup.go` `globalMenuItems()` 的 Quit | `Ctrl+C too; asks while a download runs` | `[Ctrl-C] too; asks while a download runs` |
| key reference 說明 `selectmode.go` `selectKeys` 的 `/` | `search; Enter finds, n N step` | `search; [Enter] finds, [n]/[N] step` |

menu 的說明欄也會進 panel 的 key reference（`keyReference()` 把 label 與說明接成一句），改一處兩邊一起對。

不用改：toast「the frame could not be entered; its address is a Yank away」的 `Yank` 是 menu 列的名字，不是鍵；`enterOn()` 那個 message 的
標題 `Enter` 是框的標題，不是句子也不是 label，照舊。

**規則**：M5（v0.1.15）—— 句子（空狀態、toast、錯誤訊息）裡的鍵一律加方括號：`Press [A] or [Space]`。（判斷：confirm / message 的內文、
input 的提示、menu 與 key reference 的說明欄都是句子。）

**怎麼改**：字串照上表改。`emptyHint()` 是整個字比對，鍵要傳加了括號的樣子（`"[T]"`），或改成認得方括號；句子裡鍵的顏色條文沒有規定，
照舊（`handColor`）或統一成 Blue 由 app 決定。**測試**：現在沒有斷言這些句子的測試（`app_test.go`、`popup_test.go` 只在註解提到）；
補一個：每種空狀態 render 出來鍵有方括號，toast 的三句也一樣；mutation 改回一處光寫的鍵要紅。


## 4. README 兩份與設計文件的鍵名 —— M5（v0.1.14–v0.1.15）

**現況與改法**（README 兩份同一處；中文版的字句在括號裡）：

| 位置 | 現在 | 改成 |
|---|---|---|
| 「Why webu」一頁就是一份文件 | `` `n` / `p` `` | `` `n/p` `` |
| 「Quick start」第一點 | `` `j` / `k` `` | `` `j/k` `` |
| 「Quick start」第三點 | `` `P` / `N` `` | `` `P/N` `` |
| 「Key bindings / 按鍵」開頭 | `` `[A]dd folder` is shift+A ``（「是 shift+A」） | `` is `Shift-A` ``（「是 `Shift-A`」） |
| 「Everywhere / 到處都通」screens | `W / B / H / D / S` | `W/B/H/D/S` |
| 同上 panels | `1 / 2  ·  Tab` | `Tab/1–2`（跟 footer 同一個寫法） |
| 同上 cursor | `j k`、`u d`、`gg G`、`h l` | `j/k`、`u/d`、`gg/G`、`h/l` |
| 同上 global | `q / Ctrl+C` | `q/Ctrl-C` |
| 「Page / 頁面」第二段 | `` `P` / `N` ``、`` `n` / `p` `` | `` `P/N` ``、`` `n/p` `` |
| 「DevTools」 | `` `h` / `l` `` | `` `h/l` `` |
| 「Visual mode」 | `` `h j k l` ``、`` `w e b` ``、`` `0 $` ``、`` `gg G` ``、`` `v` / `V` ``、`` `n` / `N` `` | `` `h/j/k/l` ``、`` `w/e/b` ``、`` `0/$` ``、`` `gg/G` ``、`` `v/V` ``、`` `n/N` `` |

`Enter`、`Esc`、`Space`、`Tab`、`Backspace`、`→` 已經是鍵帽的名字；`Sections` / `One sheet` 是 menu 列的名字，不是鍵，照舊。README
其他段落用 ` · ` 串起「鍵 + 說明」的清單是 README 的排版，不是 hint，條文不管。

**設計文件**（規則沒要求，建議一起對齊，免得跟畫面對不上）：

- `docs/ui.md` §1.1 版面圖的 footer 那一行、§5 的兩種 footer（`space menu   ? help   tab/1-2 panels   q quit` 等）→ 第 1 條的新寫法。
- `docs/ux.md` §A.0 表的 `space menu`、`? help` → `Space:menu`、`?:help`；§2.1 的 `Tab accept`、`→ accept`、`Bksp decline` →
  `Tab:accept`、`→:accept`、`Backspace:decline`；§A.2 全域動作表「離開」那一列的兩處 `Ctrl+C` → `Ctrl-C`；§1、§2.3、§4 表裡的 `hjkl`
  → `h/j/k/l`。
- 不用改：CHANGELOG 已發布段落是歷史；`docs/support.md` 由 `internal/ir/roles.go` 的 `Display` 產生（「Enter opens」這類），不在畫面上，
  也不是 README；程式註解裡 Chrome 的 `Cmd+L`、`Cmd+Opt+I` 不在畫面上。

**規則**：M5（v0.1.15）—— 鍵名在畫面上所有地方與 README 都一樣：鍵帽上的名字；字母照實際大小寫（`A` 就是 `Shift-A`）；modifier 用
`-`；幾個鍵做同一件事用 `/`，範圍用 `–`。


## 5. `?` 的 key reference 沒把 Space menu 變暗的列畫暗 —— M6（v0.1.14）

**現況**：`internal/ui/helppopup.go` 的 `keyReference()` 從 panel 的 Space menu 列（`app.go` `panelMenu()`）產生 `helpEntry{key, desc}`，
`menuItem.disabled` 沒有帶過來；`helpPopup.view()` 每一列都照常畫。所以 Space menu 裡變暗的列，在 `?` 裡跟按得了的列一樣亮。scratch
實測（沒有分頁）：

- `[1]`：Space menu 的 `[X] close others`、`[U]ndo close` 畫成 dim 色（`108;112;134`）；`?` 裡 `X`、`U` 兩列的說明是 Text 色
  （`205;214;243`），跟 `T` 一樣。
- `[2]`：16 個有鍵的列裡 13 個在 Space menu 變暗（`R`、`P`、`N`、`/`、`v`、`A`、`Esc` Page parts、`go`、`n`、`p`、`I`、`Y`、`C`；只有
  `T`、`L`、`Z` 亮），`?` 裡 16 個全部一樣亮。

list screen（Bookmarks、History、Downloads、Settings）的 `listpanel.go` `menuItems()` 目前沒有 disabled 的列；這條改完，它們加了
disabled 之後自動跟上（該暗的見第 6 條）。

**規則**：M6（v0.1.14）—— `?` 的 key reference 照 menu 同一套：對象存在、現在不能按的鍵照樣列出、**變暗**；對象不存在就不列。
說明維持原句，不另寫原因。

**怎麼改**：`helpEntry` 加 `disabled`；`keyReference()` 從 `menuItem.disabled` 帶過來；`helpPopup.view()` 對 disabled 的列，鍵與說明
都用 Space menu 畫變暗列的同一個 dim 樣式（`spacemenu.go` `view()` 的 `it.disabled` 分支）。core key 不在 Space menu 裡，照舊列出。
測試：`[1]` 分頁少於兩個時 `X`、`U` 的 entry 是 disabled、`T` 不是；開顏色 render 出來，`X` 那一列是 dim 色、`T` 那一列不是（第 2 條
之後 `T` 的鍵是 Blue）；`[2]` 沒有分頁時只有 `T`、`L`、`Z` 亮。mutation：拿掉帶 `disabled` 那一行、拿掉 view 的 dim 分支，各自要紅。
文件：`ux.md` §A.2 與 `ui.md` §3 表的「`?` key reference」那一列補「menu 裡變暗的鍵，這裡也列出、變暗」；README 兩份的 `?` 沒寫到亮暗，
不用改。

**順帶**（不是規則，改 `keyReference()` 時可以一起看）：label 以鍵開頭、本身沒有括號的列（`X close others`，畫 menu 時才由
`bracketHotkey()` 括成 `[X] close others`），在 `?` 裡印成 `X  X close others — every tab but this one`：`rowKey()` 只拆 `[` 開頭的 label。


## 6. Space menu 該變暗卻沒變暗、按了才跳 toast 的列 —— M6（menu 那一半從 v0.1.0 就是規則；v0.1.14 起 `?` 跟著它）

**現況**：`disabled` 的條件比動作本身的前提寬 —— 列亮著，按下去才由動作回一個 toast。key reference 由 Space menu 產生（第 5 條），
menu 沒暗，`?` 也不會暗。

- `P` / `N`（`app.go` `pageMenuItems()` 的 `Previous` / `Next`）：只在沒有分頁時 disabled。沒有上一頁 / 下一頁時照樣亮；按下去
  `page.Back` / `page.Forward` 問過 CDP 才回 `page.ErrNoEntry`，`Update()` 收到 `navFailMsg` 跳 toast「nothing to go back to」。
- `v`、`/`（同一個函式的 `Visual mode`、`[/] Search`）：disabled 條件是 `t == nil`（`/` 另加頁面彈窗）；但 `enterSelect()`、
  `openFinder()` 還要 `t.root != nil`（頁面還沒抓到、或沒抓成），不然 toast「no page to select from」「no page to search」。
  （`[go] Go to line` 用 `lineCount()` 判斷，已經一致。）
- Downloads 的 `Open file`（`Enter`；`listpanel.go` `menuItems()`）不看下載狀態；`downloads.go` `openDownload()` 對進行中、已取消的列
  跳 toast「still downloading」「that download was cancelled」。
- History 的 `Clear`（`C`）在沒有任何紀錄時照樣問「Every visit ever recorded goes.」；Downloads 的 `Clear done`（`C`）在沒有已完成 /
  已取消的列時按了什麼都不做（`clearDownloads()`）。

**規則**：M6 —— 對象存在、但現在不能執行：列照樣出現、變暗，說明欄維持原句、不另寫原因；`?` 的 key reference 照同一套
（v0.1.14）。「沒有上一頁時的返回」正是這一種。

**怎麼改**：讓 `disabled` 等於動作的前提。

- `P` / `N`：`tab.go` `capture()` 每次都呼叫 `page.Entry()`，它已經拿了 `Page.getNavigationHistory`（只取目前 entry 的 id）；同一個回應
  就看得出前後有沒有 entry（跟 `page.step()` 同一個判斷：最前面的 `about:blank` 不算）。一起帶進 `pageMsg`、存在 tab 上，`Previous` /
  `Next` 用它 disabled。頁面自己改 history（`pushState`）不一定觸發 capture，`ErrNoEntry` 的 toast 留著當退路。
- `v`、`/`：disabled 加上 `t.root == nil`。
- Downloads 的 `Open file`：只有下載完成的列按得了，進行中、已取消的變暗（`listEntry` 要帶下載狀態，`menuItems()` 才看得到）。
- History 的 `Clear` 沒有紀錄時、Downloads 的 `Clear done` 沒有可清的列時變暗。
- （可選）導航中 `busy()` 會吞掉 `P` / `N` / `R`（`dispatch()`，`ux.md` §6）：一瞬間的狀態，頁面變暗、URL 列轉 icon 已經揭露；要一致的話
  三列的 disabled 加上 `m.busy()`。

panel 上直接按這些鍵時的回應（現在是 toast）不在這條的範圍：webu 對「拒絕的操作」一向用 toast（`ui.md` §3 表的 toast 列），這條只改
menu 與 `?` 的亮暗。測試：每個條件各一個 —— Space menu 的列 disabled、`?` 的 entry 也 disabled；mutation 逐條拿掉新加的條件。文件：
`ux.md` §A.1 的 `[2]` panel operation 與 screen 內部表，寫出這幾列什麼時候變暗。


## 7. 浮層的 key reference 是固定清單，現在按不了的鍵沒變暗 —— M6（v0.1.14；v0.1.16 的補充不適用）

**現況**：`internal/ui/helppopup.go` 的 `floatHelp()` 把固定清單原樣交給 `?`：

- DevTools（`helpDevtools`）：四個分頁的鍵**混在同一段**，沒有分頁的標題。`devtools.go` `update()` 依分頁回 action：`x/y` 只在 Storage、
  `i` 只在 Console、`Enter` 只在 Network 與 Console、`C` 在 Source 不作用；清單是空的時候 `Enter`、`x/y` 也不作用。`?` 裡全部照常亮，
  靠說明裡的「Storage:」「Console:」說範圍。
- message（`helpMessage`）：`j/k` 的說明是「scroll, when it is longer than the box」—— 把「什麼時候按得了」寫進說明，內容放得下時
  照常亮。（下框 hint 只在放不下時列 `j/k`；hint 只列按得了的鍵是允許的。）

**規則**：同第 5 條 —— 對象存在、現在不能按的鍵列出、變暗；說明維持原句，不另寫原因（v0.1.14）。v0.1.16 補的「另外加標題、說明**別的
surface** 的一段照亮顯示」不適用：DevTools 的四個分頁是同一個 popup、同一個 `?`，別的分頁 `h/l` 就到，不是看不到 key reference 的另一個
surface（條文的例子是 sshu 清單上 `?` 裡的 `ssh grid`：格子裡看不到 key reference）。所以不能靠替每個分頁加標題來免掉變暗。

**怎麼改**：`floatHelp()` 依目前狀態標 `disabled`（第 5 條加的欄位）。DevTools：目前分頁用不到的鍵變暗（其他分頁是「對象存在」，列出、
不拿掉；說明裡的「Storage:」「Console:」可以留著，指出在哪一頁按）。message：`j/k` 的說明改回「scroll」，內容放得下時變暗。其他浮層
（Space menu、global operation、options / choices、confirm、quit confirm、finder、go、editor）的 `?` 列的鍵，在那個浮層上一直按得了，
不用改。測試：DevTools 停在 Network 時 `x/y`、`i` 是 disabled，`Enter`、`C` 不是；短 message 的 `j/k` 是 disabled、長的不是；mutation
同第 5 條。


## 8. toast 的新寫法：行為已符合，缺測試與文件 —— F1、F8、K11（v0.1.14）

**現況**：行為符合，這條只補守它的測試與文件。

- `app.go` `routeKey()`：`Esc` 先問 `floatOwned()`（含 toast），`closeTop()` 第一個收 toast。其他鍵的路由 switch 沒有 toast 那一格；
  `popupOpen()`、`stackTop()`、`floatAboveMenu()` 也都不算 toast —— toast 在時其他鍵照樣到底下。scratch 實測：toast 在時按 `1`，focus
  換到 `[1]`；按 `B`，換到 Bookmarks；toast 都還在。再按 `Esc` 收掉 toast，畫面留在 Bookmarks。
- visual mode：`Tab` / `1` / `2` 跳 toast（`routeKey()` 的 `m.sel.on` 分支；字串見第 3 條）。toast 還在時第一個 `Esc` 由 `floatOwned()`
  先收 toast，模式還在；第二個才進 `selectKey()` 離開。
- 缺的：`keys_test.go` `TestVisualModeHoldsThePanel` 只量 toast 有出現；沒有測試守「toast 在時其他鍵穿過它」與「第一個 `Esc` 收 toast、
  第二個才離開模式」。`ux.md` §A.0.K 表 visual mode 的 `Esc` 欄只寫「打字中取消；否則離開模式」。

**規則**：F1 —— toast 除了 `Esc`，按鍵都穿過它；F8 —— toast 不觸發 dim（除了 `Esc` 它不收鍵，不是一層）；K11 —— 模式裡回應 `Tab`
的 toast 還在時，第一個 `Esc` 先收掉它（K4）。

**怎麼改**：補兩個測試 ——（1）toast 在時送一個不是 `Esc` 的鍵（例：`1`），鍵作用在底下、toast 還在；（2）visual mode 裡 `Tab` 跳出 toast，
`Esc` 之後 toast 沒了、`sel.on` 仍為真，再一個 `Esc` 才離開。mutation：`floatOwned()` 拿掉 toast，（2）要紅；在 `routeKey()` 的 switch 加一格
讓 toast 吞鍵，（1）要紅。文件：`ux.md` §A.0.K visual mode 的 `Esc` 欄補「toast 還在時先收 toast」；`ux.md` §5 或 `ui.md` §3 表的 toast 列
補「除了 `Esc` 不收鍵」。前例：第一輪對照 v0.1.5 時的 `TestTabIndentsInTheEditor` —— 行為本來就對，寫成規則後補測試守著。

**順帶**：`routeKey()` 的 `Esc` 分支 `case m.focus == panelPage:` 裡那個 `if m.toast.anim.owns()` 走不到（`floatOwned()` 已含 toast、
先回傳了）。要刪就刪；刪了沒有測試變紅是預期的。


## 9. zoom 裡 `Tab` / `1` 把 focus 移到沒畫出來的 `[1]` —— L5（不是這三版的改動；核對術語「模式」的 zoom 時發現）

**現況**：`app.go` `panelKey()` 的 `Tab`、`1`、`2` 只改 `m.focus`；`View()` 在 `m.zoom` 時一律只畫 `pagePanel()`（窄寬時是同一個
`case`）。scratch 實測：`2`、`Z`、`Tab` 之後 focus 在 `[1]`、`zoom` 仍為真，畫面只有 `[2]`，而且 `[2]` 用非 focus 的色調畫 —— 畫面上
沒有任何 focus 所在的 surface；這時 `j/k` 移的是看不到的 `[1]` 游標。

**規則**：L5 —— focus 所在的 surface 必須一眼可辨。術語「模式」（v0.1.14）：zoom 是版面的切換，沒有鍵換意思，`Esc` 不必退出它、由它
自己的鍵還原 —— `Tab` 照樣換 focus 是對的，問題在換過去的 panel 沒畫出來。

**怎麼改**：照 sshu（`internal/ui/sshtab.go` `setFocus()`：focus 離開放大的那一格就收掉 zoom）：focus 離開 `[2]`（`Tab`、`1`）時
`m.zoom = false` 並 `relayoutTabs()`。另一種也符合 L5 的做法是 zoom 跟著 focus 放大 `[1]`；清單建議照 sshu，家族一致。測試：`2`、`Z`、
`Tab` 之後 focus 在 `[1]`、`zoom` 為假、`View()` 畫出 `[1] Tabs`；mutation 拿掉那一行要紅。文件：`ui.md` §1.2 窄寬那一列的「`Z` zoom
讓頁面佔滿」補「focus 離開 `[2]` 就還原」；README 兩份只寫 `Z` zoom，不用改。


## 已經符合、不用修的（對照 v0.1.14–v0.1.16 的改動）

- **F1、F8 的 toast**：從下方彈出（`View()` 以 `overlay.Bottom` 疊上）、一行、`Esc` 或 `toastLife` 到就收；除了 `Esc`，按鍵都穿過它
  （第 8 條只補測試）；不觸發 dim、自己也不變暗（`dim_test.go` 守著）。
- **K11 的 `Tab`**：visual mode 裡 `Tab` 有回應（toast），第一個 `Esc` 先收 toast（第 8 條只補測試）。
- **K10、D5（v0.1.14、v0.1.16）**：webu 沒有 PTY —— 沒有跑在 app 裡的 shell、編輯器或遠端 session（`exec.Command` 只用在交給桌面開檔與
  剪貼簿），`Alt-Esc` 出口鍵與它的 confirm **不適用**。也沒有綁任何 `Alt` 組合鍵。
- **術語「模式」**：webu 唯一的模式是 visual mode（`Esc` 離開，K11 已對齊）。`Z` zoom、窄寬只畫一側、`Sections` / `One sheet`、pagetab
  都是版面或游標位置的切換，沒有鍵換意思：zoom 由 `Z` 自己還原，`Esc` 不碰它（實測 `Esc` 之後 zoom 仍在）；pagetab 上的 `h/l`
  就是 core 的「同列移動」。`dev-remarks.md`「偏離 tdp」沒有把其中任何一個寫成偏離，不用改。zoom 的 focus 問題見第 9 條。
- **M5 的 label**：括號標記只出現在 label —— menu 的列（`[Enter] Switch to`、`[/] Search`、`[go] Go to line`、`[Esc] Page parts`；單一字母
  由 `bracketHotkey()` 畫的時候括）、header 的 screen chip（`[W]eb` …）、panel 標題（`[1] Tabs`、`[2] Page`）。hint、footer、key reference
  都不加括號（它們的新寫法見第 1、2 條）。
- **M5 的 modifier 與大小寫**：畫面上唯一的 modifier 是 `Ctrl+C`（第 2、3 條改成 `Ctrl-C`）；字母鍵都照實際要按的大小寫印（`C`、`A`、
  `a`）。
- **D2 的顏色**：hint 與 footer 的鍵已是 Blue、說明已是 Overlay0（第 1 條只補冒號的顏色）；key reference 的說明已是 Text（鍵的顏色見
  第 2 條）。
- **M6 的 hint 與 footer**：只列現在按得了的也可以，由 app 決定。webu 的下框 hint 大多固定列出（Downloads 的 `Enter:open file`），
  message 與 key reference 自己的 hint 只在放不下時列 `j/k` —— 兩種都允許，不用改。
- **M6 的「對象不存在就不列」**：`[1]` 沒有分頁時 item operation 整區不出現，`c` `o` `r` `y` 也不在 `?`；Bookmarks 游標在目錄上時沒有
  `Move`（目錄搬不了），`m` 不在 `?`；空清單沒有 item 區（`itemRegion()`）。key reference 從 Space menu 產生，這些自然一致。
- **M6 在 Space menu、global operation popup 上的 `?`**：列的是通用的 menu 鍵（`j/k`、`Enter`、「a row's key」、`Esc`），不逐列列出
  熱鍵，沒有「某一列變暗」的問題；global operation popup 裡目前所在畫面那一列照 M6 變暗（`globalMenuItems()`）。
- **M6 的 visual mode `?`**（`selectmode.go` `selectKeys`）：`y` 沒有選取時複製這一列，一直按得了；`n/N` 寫在 `/` 那一行，是搜尋的
  後續步驟，不是單獨一列；`Enter` 是 core key。
- **M6「說明別的 surface 的一段照亮」（v0.1.16）**：webu 的 key reference 沒有這種段落 —— panel 的 `?` 只有這個 panel 的區塊與
  `core keys`，浮層的 `?` 只有那個浮層的鍵；DevTools 的分頁不算別的 surface（第 7 條）。


## 待確認

沒有。v0.1.14 版清單留下的疑問（footer 的小寫、`Bksp` 與 `Backspace`、`j · k` 與 `j/k`）v0.1.15 都定案了，照第 1、2、4 條改。

## 裁定（user 2026-09-29，動程式之前）

清單裡留給 app 決定或標「可選」「順帶」的六處：

1. **第 1 條 `[2]` 下框的狀態字**（`loading`、`a popup is up: answer it`、`2/26 · 節名 · 40%`、`1-20 of 80`）：不再經過
   `hintLegend()`，另外畫、用 Overlay0。它們不是鍵，不用 Blue；走 `鍵:說明` 會多出一個冒號。
2. **第 3 條句子裡加了方括號的鍵**：顏色照舊（`handColor`），這輪只補括號。
3. **第 5 條順帶的 `rowKey()`**：一起修 —— label 以單一字母開頭、沒有括號的列（`X close others`）在 `?` 裡拆成鍵 `X` 與
   說明，不再印成 `X  X close others`。
4. **第 6 條可選的「導航中 `P/N/R` 變暗」**：不做。一瞬間的狀態，頁面變暗與 URL 列的 icon 已經揭露；做了 Space menu 會在導航時閃。
5. **第 8 條順帶的死碼**（`routeKey()` 的 `Esc` 分支裡走不到的 `m.toast.anim.owns()`）：刪掉，跟第 8 條同一個 commit。
6. **第 9 條**：照 sshu，focus 離開 `[2]` 就收掉 zoom。
