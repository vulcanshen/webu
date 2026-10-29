# webu — UI

> 本文件講**版面與 surface**：畫面、Web 畫面的兩個面板、`[2]` 裡頁面的擺法、popup、配色、存檔。頁面的元件怎麼畫、
> 按鍵的規格在 [`reader.md`](reader.md)；互動語意（core-key、Space menu、hotkey）在 `ux.md`；實作註記在 `dev-remarks.md`。
> 依 [terminu design principle](https://github.com/vulcanshen/terminu)（tdp）撰寫，每條版面決定標日期。

> **2026-10-18 起畫面建在 terminu-go 上。** 框、膠囊、畫面 chip 列、footer、每一種 popup、淡化、loading icon 都是 lib 的，
> 長相照 tdp components，本文件只寫 webu 用到哪些；webu 自己畫的只有 `[2]` 裡面的頁面。2026-10-28 起 webu 是閱讀器，
> 2026-10-29 起不再跑 Chromium（`reader.md`）。

---

## §1 版面

### 1.1 Grid

```
 [W]EB  [B]OOKMARKS  [H]ISTORY  [D]OWNLOADS  [S]ETTINGS                    1 download in flight   ← 畫面 chip 列（lib）；右端是 statusbar
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  ← 分隔線；下載中從左邊填 Green 當進度條
┌[1] TABS──────────────┐┏[2] PAGE━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
│ TITLE                │┃ 󰖟 https://en.wikipedia.org/wiki/Terminal_emulator                    ┃  ← 網址列：位置列，Lavender
│  Example Domain     │┃   1  Terminal emulator                                               ┃  ← 行號（Overlay0）＋頁面：h1 的字在 Peach 上
│  Terminal emulator  │┃   2                                                                  ┃
│                      │┃   3 A terminal emulator is a computer program that emulates a        ┃
│                      │┃   4 󰌷 video terminal within some other display architecture.         ┃  ← 連結：link icon＋字，Sapphire、底線
└─ Enter:read c:close ─┘┗━ H:back L:forward ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 1-24 of 310 ━┛  ← hint、捲動位置（lib）
Space:menu ?:help !:log Tab|1–2:panels q:quit                                                     ← footer（lib）
```

Web 畫面兩個面板並列，編號左到右：`[1]` 分頁清單、`[2]` 頁面。最上面一列是 lib 的畫面 chip 列（tdp screen.3），五個畫面
各一個膠囊，目前的那個點亮；右端是 statusbar（tdp screen.4），有下載進行中時寫 `N download(s) in flight`（Green、固定寬）。
chip 列下面一條分隔線，下載中從左邊填 Green 當進度條。最下面一列是 lib 的 footer（tdp screen.2）。網址在 `[2]` 裡面的第一列（§2）。

### 1.2 尺寸規則

| 項目 | 規則 |
|---|---|
| 最低尺寸 | 60×20（2026-10-19，tdp L1：家族的標準是 60×32，app 可以調低）：兩個面板並排，也放得下加書籤、登入這些表單。再小，lib 只畫名稱、`needs a terminal of at least 60×20` 與 `Press [Ctrl-C] twice to quit`（tdp screen.7） |
| 側欄寬 | `[1]` 固定 24 欄（`sideW`），`[2]` 拿其餘 |
| 窄寬 | 門檻 60 欄（`narrowW`，2026-10-28，tdp screen.5 不能高於 60），跟最低寬度一樣，所以 webu 畫得出來的寬度都是兩個面板並排 |
| zoom | `[2]` 的 `Z`：有 focus 的面板佔滿寬度，`Tab`、數字鍵照樣換面板、換到的也佔滿，在 `[2]` 再按 `Z` 還原（tdp screen.6 的預設） |
| chrome | chip 列 + 分隔線 + footer 共 3 列（lib）；`[2]` 的框上下 2 列、網址列 1 列，頁面拿其餘 |
| 寬度穩定 | 每一列都恰好是終端機寬：`TestTDP` 在各種尺寸、icon 一格／兩格下走過每個畫面與 popup，`TestEveryPopupFits`、`TestUnderTheLeastSize` 縮到最低尺寸以下（tdp L4、E5、E6）。網址從前面截，分頁標題截斷不折行 |
| 行號欄 | `[2]` 的頁面都有（2026-09-23）；寬度是位數加前後各一格，依頁面的列數；最多排兩次（先用上一次的寬度排，位數變了再排一次） |

---

## §2 畫面與兩個面板

### 畫面 chip 列 — 全域動作的常駐揭露

`[W]eb` `[B]ookmarks` `[H]istory` `[D]ownloads` `[S]ettings`，各一個全域鍵切換畫面；chip 列沒有 cursor、`Tab` 不停
（lib，tdp screen.3）。全域動作只有切畫面與離開，`[1]`、`[2]` 的熱鍵各自歸各自的面板（tdp P3、M4）。2026-10-29 起沒有
DevTools 畫面：留下來的 Source、Cookies、Requests 在 `[2]` 的 Space menu（§3）。

### list screen — Bookmarks / History / Downloads / Settings

一個面板佔滿 chip 列與 footer 之間，是 lib 的面板（2026-10-18）：表頭、下框 hint、Space menu 都由 lib 從 webu 的登記產生。

| 畫面 | 欄 | 內容 |
|---|---|---|
| Bookmarks | Title、URL | 目錄樹：可巢狀、可收合（目錄那一列按 `Enter`）、可搬移、可改名、可匯入瀏覽器匯出的書籤檔；`/` 篩選 |
| History | Title、URL、When | 造訪過的每一頁，新的在上，清除之前一直留著；`/` 篩選 |
| Downloads | File、State | 這次 session 的下載，新的在上；`/` 篩選 |
| Settings | 一列一個值（lib 的 `Values`，tdp panel.12） | `config.yaml` 的每個 key 一定有一列（`TestSettingsCoverConfig` 守著），設好就存；`C` 清除登入資料（`cookies.json`） |

長的欄（標題、URL、檔名、狀態）分短欄剩下的寬度；縮到最少還放不下時，History 整欄收掉 When（tdp panel.5）。書籤與歷史的
`Enter` 在背後開新分頁、留在清單上，toast 說開在哪裡（2026-10-18，tdp K3）。

### `[1]` — 分頁清單

lib 的清單，欄名 `Title`（user 2026-10-28：外框已經寫 TABS，原本的欄名 `Tab` 重複；試過顯示網址，user 要保留標題）。一列一個分頁：

- **網站的記號**在最前面（user 2026-10-28，`siteicon.go`）：Nerd Font 有那個網站的 icon 就用它，沒有就是地球；顏色是網站
  favicon 的主色，每個網站一個 session 問一次，拿到之前是 Blue。游標那一列的記號照 tdp color.1 換成 Base（lib 畫的）。載入中
  記號換成 live 記號（`glyphLive`，跟 kbu 的 Logs 同一個碼位）。
- **標題**：頁面的標題，沒有標題就是網址；截斷不折行。同一個網址的幾個分頁，照開啟的順序在前面標 `(1)`、`(2)`。
- **字色**：Green 是 `[2]` 正在顯示的分頁；Overlay0 是上次 session 留下、還沒載入的分頁。

### `[2]` — 頁面

**單一職責，永遠是頁面。** lib 的面板，label `Page`，沒有 panel tab；框、loading icon、失焦時的淡化是 lib 的，框裡面是 webu
自己畫的（`pagebody.go`）。

**`[2]` 是 `[1]` 的明細**（2026-10-18 暫定，到現在沒有重談；tdp panel.13）：`[1]` 的游標移到哪個分頁，`[2]` 就顯示哪個，
還沒載入的分頁換到才載；`[1]` 的 `Enter` 把 focus 送到 `[2]`（tdp K3）。`T`、連結的「開新分頁」換到 Web 畫面、focus 到 `[2]`；
載入完成這種背景的事不搶 focus。

#### 網址列

`[2]` 的第一列，tdp panel.10 的位置列（2026-10-29，terminu 確認）：glyph（nf-md-web）與網址都是 Lavender；太長從前面截、
保留尾端；只能看，不能按、cursor 不停、捲動位置不算它；`[2]` 失焦時跟著淡化。visual mode 裡用 `/` 找字時，這一列換成找的字與
目前是第幾筆（`/query  2/5`，Yellow）。

#### 頁面

網址列以下是頁面：左邊是行號欄（Overlay0，user 2026-10-28：行號說的是字在哪，不是字本身），右邊是內容樹畫成的 webu 元件
（`readerrender.go`）。規格以 `reader.md`「元件」為準，這裡是摘要：

- **標題**：字（Base、粗體）放在該層的顏色上、左右各一格空白，不畫圓端（user 2026-10-28：圓端是 tdp 的膠囊，P4）；顏色依層
  Peach、Yellow、Green、Teal、Flamingo，第六層起輪迴；標題裡的連結、程式碼畫在同一條上。
- **段落**：Text；粗體、斜體、刪除線、標記用終端機的粗體、斜體、刪除線、反白。
- **清單**：無序清單每一層都是 `-`（Blue），靠縮排分層；有序清單依層 `1.`、`a.`、`i.`，再回到數字輪迴。
- **程式碼**：區塊是 Mantle 底、照語言上色（chroma；網頁沒標語言就讓 chroma 猜），語言名寫在右上角（Overlay0），Tab 畫成
  `input_tab_width` 格；行內程式碼是 Pink 字、Surface0 底。
- **引用**：開頭一條 Lavender 的 `▎`（多層引用多條），Mantle 底。
- **表格**：資料表格畫成格子，表頭粗體、下面一條分隔線，不鋪底色；太寬切掉加 `…`。沒有 `<th>` 也沒有 `<caption>` 的表格當一般區塊。
- **連結與圖片**：連結是 link icon＋字，Sapphire、底線；圖片是 image icon＋alt（沒有 alt 寫網址的檔名），Overlay0；包在連結裡的
  圖片用連結的顏色。網址在 Focus 到它時寫在下框右邊。
- **表單**：`┌ Form: 名字` 開頭、`│` 沿著左邊、`└` 收尾（Overlay0）；文字欄位 `[值]`、密碼 `[•••]`、勾選框 `[ ]`／`[x]`、
  單選 `( )`／`(•)`、select `[值 ▾]` 是 Mauve，按鈕 `( 名字 )` 是 Sapphire。
- **分隔線**：一條橫跨面板的細線（Overlay0）。
- 畫面上沒有 markdown 符號，visual mode 複製的是畫面上的字；`Yank markdown` 把內容寫成 markdown。
- 頁面上的控制字元畫成 Red 的名字（`\t`、`\x1b`……，tdp components.5），整列照舊截到面板寬。

`w` 在正文與整頁（整個 `<body>`）之間切換（user 2026-10-28）。

#### 下框

lib 畫的（tdp panel.1）：左邊是 hint，閱讀時是 `H:back L:forward`（user 2026-10-28：不然不知道怎麼上下頁）；右邊是捲動位置
（`1-24 of 310`，整頁放得下時不寫）。Focus mode 裡右邊換成 focus 中的連結或圖片的網址（`reader.md`）。

#### 狀態與模式

- **loading**：lib 的 loading icon 接在 `[2]` 的膠囊後面；還沒有任何內容時，中間寫 `loading <host>…`（tdp panel.15）。換頁時，
  正在離開的那一頁留著並變暗，新的一頁到了才換。
- **錯誤**：置中兩句，事實是 Red，`Press [R] to retry` 是 Overlay0（tdp panel.15）。
- **Focus mode（`f`）與 Visual mode（`v`）**是 `[2]` 的 mode（tdp K11）：框變 Yellow、上框右邊是模式名（`Focus`、`Visual`，
  選取中 `Visual>select`），都是 lib 畫的。Focus 中的項目、`/` 找到的目前那一筆是 tdp 的 panel cursor 色塊（Subtext1 底、Base
  粗體）；其他找到的字加底線、保留原本的顏色（2026-10-19 user 定，tdp color）。

---

## §3 Popup

2026-10-18 起 popup 全是 terminu-go 的（tdp layout/popup 與 dialog、input 各元件）：外框、寬度、高度、層色、開關動畫、淡化、
loading icon、錯誤列、`Esc` 一次一層都照 tdp，webu 不自己畫。從 menu 執行之後，打開來看的、選一個值寫回的、新增的，menu 留在
底下；要去別的地方的（`T` 新分頁、`o` 網址、書籤的 `m`）開的時候清掉 menu（tdp T1）。webu 用到的：

| Popup（lib） | webu 用在 |
|---|---|
| Space menu、global operation、`?` key reference | lib 從 webu 的登記產生（`dev-remarks.md`「一份登記、lib 產生其餘」） |
| menu | 麵包屑（`[2]` 的 `b`，user 2026-10-29，照 kbu）：這個分頁走過的每一頁，編號加標題，選一頁就直接跳過去；歷史只有一頁時沒有這個動作 |
| viewer | **Source**（抓下來的原始內容）、**Cookies**（這個網站的；`C` 先問再清掉）、**Requests**（載入這一頁發出的請求）：`[2]` Space menu 最上面三項，沒有熱鍵（user 2026-10-29，取代 DevTools）。**Inspect**：Focus 中項目的 Space menu |
| text input | `o` 網址（目前的網址當灰字提議）、`T` 新分頁、`/` 找字（閱讀與 visual mode）、頁面的文字欄位、Settings 的文字設定、書籤的新目錄、改名、匯入到哪個目錄 |
| password input | 頁面的密碼欄位 |
| textarea | 頁面的多行欄位 |
| number input | Settings 的 `input_tab_width` |
| select | 頁面的 select、書籤搬到哪個目錄（`m`） |
| file picker | 匯入書籤（`I`）、Settings 的 `download_dir`（選目錄） |
| form | 加書籤（URL＋Title） |
| confirm | 只有一個可以打字的欄位的表單，填完時問要不要送出；刪書籤、刪目錄；清歷史、清登入資料、清一個網站的 cookie；有下載進行中時離開（tdp K9） |
| MoveAside | 讀不了的 `config.yaml`、`bookmarks.yaml`、`history.yaml`、`session.yaml`、`cookies.json`（tdp E3） |
| toast、error popup、App Log（`!`）、splash（`V`） | lib 的；成功做完的是 Success toast（tdp toast.2） |

---

## §4 色帶

**兩套配色（2026-09-22 定案）**：**外殼配色**（框、膠囊、chip 列、cursor、hint、footer、popup 的層色、模式色、淡化）2026-10-18
起都是 terminu-go 的，照 tdp color，webu 不另外定；**頁面配色**是 `[2]` 頁面自己的結構，封閉集合，只出現在 `[2]` 的頁面裡、
不參與層級（`theme.go`）。錨點都是 catppuccin-mocha。

外殼的顏色在 webu 說什麼（意思由 tdp color 定）：

| 色 | webu 用在 |
|---|---|
| Lavender | `[2]` 的網址列（使用者足跡：位置列，tdp panel.10） |
| Green | `[1]` 裡 `[2]` 正在顯示的分頁；chip 列右端的下載數與進度條（tdp 的「生效中」、進行中的工作） |
| Overlay0 | `[1]` 裡還沒載入的分頁 |
| Yellow | Focus mode、Visual mode 的框與模式名；visual mode 的選取範圍、網址列上的找字狀態 |
| Subtext1 底、Base 粗體 | cursor 色塊：Focus 中的項目、`/` 找到的目前那一筆 |
| Red | 頁面上畫成名字的控制字元（tdp components.5） |

頁面配色（`theme.go`；**顏色是概念不是元素**，2026-09-23 定案：可按的一個顏色、可填的一個顏色，靠 icon 分是哪一種）：

| 色 | 意思 |
|---|---|
| Text `#cdd6f4` | 段落 |
| Peach、Yellow、Green、Teal、Flamingo | 標題的底，依層輪迴（`levelInk`），字 Base |
| Sapphire `#74c7ec` | **可按的**：連結（加底線）、按鈕 |
| Mauve `#cba6f7` | **可填的**：文字欄位、勾選框、單選、select |
| Pink `#f5c2e7` | 行內程式碼的字，Surface0 `#313244` 底 |
| Mantle `#181825` | 程式碼區塊、引用的底 |
| Blue `#89b4fa` | 無序清單的 `-` |
| Lavender `#b4befe` | 引用的直槓 `▎` |
| Overlay0 `#6c7086` | 行號、圖片、分隔線、表單的框線、程式碼區塊的語言名 |
| 語法色 | 程式碼區塊裡（chroma）：key 與 string Pink、number Flamingo、常數與 keyword Sky、註解 Overlay0 斜體、標點 Overlay2 |

---

## §5 Chrome 三件套

| 件 | webu |
|---|---|
| Border title chip | `[1] TABS`、`[2] PAGE`（lib，tdp panel.1）；載入中 loading icon 接在膠囊後面（tdp panel.15）；在 mode 裡上框右邊是模式名 |
| Panel tab bar | 沒有：`[1]`、`[2]` 都沒有 tab |
| Border hint | 下框左邊 hint、右邊捲動位置，lib 從登記產生（tdp panel.1）：`[1]` 是 `Enter:read c:close`……，`[2]` 閱讀時是 `H:back L:forward`；`[2]` 的右邊在 Focus mode 是項目的網址（§2） |

focus 的框是 Blue 粗線 `┏━┓`、失焦是 Surface2 細線 `┌─┐`，同寬、切換零位移，失焦的面板內容淡化（lib，tdp panel.1、T2）。
footer 是 lib 的（tdp screen.2）：`Space:menu ?:help !:log Tab|1–2:panels q:quit`，只有一個面板的畫面沒有 `Tab|1–2:panels`，
mode 裡換成 `Space:menu ?:help`、模式的鍵、最後 `Esc:<離開的動詞>`；鍵 Blue、冒號與說明 Overlay0，寬度不夠從尾端整組捨棄。

`docs/icon.svg` 是 terminu family mark 的 webu 版；`V` 開 splash（lib，tdp S1；visual mode 因此是小寫 `v`）。
**Nerd Font 是設計、必裝**：網站記號、連結與圖片的 icon、框上的膠囊與 loading icon 都靠它，排版量它的寬；icon 與 powerline
符號佔一格還是兩格，lib 在啟動時探測，`WEBU__ICON_WIDTH`、`WEBU__POWERLINE_WIDTH`（或家族共用的 `TERMINU__` 兩個）可以自己指定。
終端機也要 truecolor：顏色與淡化都是 24-bit。

---

## §6 存檔

| 資料 | 形式 | 位置 |
|---|---|---|
| 設定 | `config.yaml`：`search_engine`（預設 DuckDuckGo html）、`download_dir`、`measure`（`full` 或 ≥ 20 的數字）、`restore_session`、`hide_secrets_in_preview`、`input_tab_width` | `~/.config/webu`（`$XDG_CONFIG_HOME/webu`、`$WEBU__CONFIG`） |
| session（狀態） | `session.yaml`：離開時開著的分頁 | 同上（`$WEBU__STATE`） |
| 登入（狀態） | `cookies.json`：每一個收到的 cookie，`0600`；Settings 的 `C` 全部清掉，Cookies popup 的 `C` 清一個網站的 | 同 session |
| 書籤 | `bookmarks.yaml`：flat yaml，`bookmarks:`（各帶 `folder`）+ `folders:`（讓空目錄留得住） | `~/.webu/datas`（`$WEBU__DATA`） |
| 歷史 | `history.yaml`：YAML sequence，一次 append 一筆；清除之前一直留著 | 同上 |
| 下載檔案 | `download_dir`，預設 `~/.webu/datas/downloads` | 同上 |
| 快取 | 現在沒有東西要放；0.5.0 以前的 Chromium 本體（`chromium-<rev>`）啟動時刪掉，App Log 寫刪了哪裡 | macOS `~/Library/Caches/webu`、Linux `~/.cache/webu`（`$WEBU__CACHE`） |

分法照 tdp E3「刪掉會怎樣」（2026-10-19；原本是使用者寫的在 config、webu 產生的在 data）：**設定**與**狀態**（刪了只是回到預設、
要重新登入）在 config、**資料**（刪了找不回來）在 data、**可重抓的**在 cache。舊版放在別處的，第一次啟動搬過去；搬不動就照舊用舊
位置。每次寫檔原子。讀不了的檔照 tdp E3 先問要不要搬開成 `.bak`（§3 的 MoveAside）。0.5.0 以前留在資料目錄裡的 `profile/` 與
`webu.log` 是資料，不刪，每次啟動在 App Log 寫一筆。
