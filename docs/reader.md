# webu 閱讀器：重新設計（2026-10-28 起）

webu 從「在終端機裡上網的瀏覽器」改成「在終端機裡讀網頁內容的閱讀器」。這份是重新設計的規格與進度；`function.md`、`ui.md`、`ux.md` 寫的是現在的 webu，跟這份衝突的地方以這份為準，等新版做完再整理掉。

## 定位

打開網址，看到這一頁直接給的內容，乾淨、好讀，用鍵盤跟著連結走。網站外殼（導覽列、側欄、頁尾、cookie 告示、浮層）不顯示；需要時可以切到整頁。

終端機擅長的是讀文字：文件、文章、README、新聞、搜尋結果。網頁 app（信箱、社群、後台）在終端機裡比不過真的瀏覽器，現在的 webu 大部分的複雜度花在那裡，這次不做。

## 決定

| 題目 | 決定 |
|---|---|
| 定位 | 閱讀器，不是瀏覽器（user，2026-10-28） |
| 取得頁面 | Go 的 `net/http` 抓原始 HTML，`x/net/html` 解析；Chromium 變選配，只在偵測到靠 JavaScript 才有內容的空殼頁時，用它跑完的 DOM（user 選 2） |
| 表單與登入 | 做到第 2 層：GET 表單（站內搜尋、篩選）把欄位組進網址；簡單的帳號密碼 POST 表單，帶上隱藏欄位送出。靠 JavaScript 的登入、第三方登入、驗證碼、兩步驟驗證不做 |
| cookie | 存在資料目錄，檔案只有使用者自己能讀寫（`0600`）；值不進 App Log、不上畫面；Settings 有「清除登入資料」 |
| 顯示 | 只顯示正文，有一個鍵切到整頁 |
| markdown | **階段性的**：先用 markdown 原文顯示，檢查挑出來的內容對不對；最終畫成 webu 定義的元件（例：h1 反白），表單也有自己的畫法 |

## 流程

```
HTML 來源（二選一）
 ├─ net/http 的原始 HTML（預設）
 └─ Chromium 跑完的 DOM（空殼頁才用；帶上元素有沒有畫出來、位置、role 的標註）
        │
        ▼
   解析 → 挑正文 → 內容樹（webu 的元件）─┬─→ markdown（現在：檢查用、[2] 暫時的顯示、Yank markdown）
                                     └─→ webu 的元件畫法（最終）
```

核心是**內容樹**，不是 markdown。markdown 寫不出來的（表單、欄位、按鈕）在樹裡有正式的節點；每個連結、欄位都記著它對應的 HTML 元素，之後才能按、能填。

內容樹沿用 `internal/ir` 的種類（Heading、Paragraph、Text、Link、List、ListItem、Table、Row、Cell、Code、Quote、Media、Separator、Textbox、Check、Combobox、Option、Button…），改成從 HTML 建，並加上 **Form**（記著 action、method、隱藏欄位，以及它管哪些欄位）。

## 挑正文

先用 Readability（`codeberg.org/readeck/go-readability/v2`，Firefox 閱讀模式的 Go 版）當基準，再加 webu 自己的規則。`tools/mddump` 對 8 個網站的實驗（dev-remarks 有細節）看到要補的：

- **表格**：沒有 `<th>` 也沒有 `<caption>` 的表格當一般區塊，不當資料表格（HN 的新聞列表連 Chromium 都判成資料表格）。
- **清單頁**：Readability 判斷不是文章時（HN、搜尋結果、新聞首頁），改走備援：整頁去掉 `<nav>`、`<header>`、`<footer>`、`<aside>`、navigation／complementary／contentinfo 等 role 與選單類的區塊。
- **浮層**：cookie 告示、對話框（dialog、alertdialog）、固定在畫面上的浮層拿掉。
- **程式碼**：`<pre>` 照原樣；被 JavaScript 改成編輯器的，退回原始 HTML 裡的版本。
- **藏起來的元素**：有 Chromium 的標註時，沒畫出來的拿掉；只有原始 HTML 時，看 `hidden`、`aria-hidden`、`style="display:none"`。

## 元件（user 2026-10-28 定）

終端機的字一樣大，元件靠 icon、背景色、前景色分。內容樹直接畫成這些元件；markdown 只是中間的轉換，畫面上沒有 markdown 符號（`#`、`**`、`[]()`、```` ``` ````），複製的也是畫面上的字。

| 元件 | 畫法 |
|---|---|
| 標題 h1～h6 | 底色只包住字、左右各一格空白，字 Base，不畫圓弧（user 2026-10-28：圓弧是 tdp 的標題膠囊，tdp P4 說它表示「哪一類畫面」，內容的標題不借用）；顏色依層：Peach、Yellow、Green、Teal、Flamingo，第六層起輪迴；不寫 `#`；標題裡的連結、程式碼畫成膠囊的一部分（user 2026-10-28：整列鋪滿不要） |
| 段落 | 一般文字色（Text） |
| 粗體、斜體、刪除線、標記 | 終端機的粗體、斜體、刪除線、反白 |
| 行內程式碼 | Surface0 底＋程式碼的顏色；在標題裡跟著標題的底色與字色，不切開那一條（user 2026-10-28） |
| 無序清單 | 每一層都是 `-`（Blue），靠縮排分層（user 2026-10-28：icon 太多餘；Font Awesome、Material 的都試過） |
| 有序清單 | 依層：阿拉伯數字 `1.`、小寫英文 `a.`（超過 26 個接 `aa.`）、小寫羅馬數字 `i.`，再回到數字輪迴 |
| 程式碼區塊 | Mantle 底（Base 會跟頁面底色撞），文字照語言上色（chroma；網頁沒標語言就讓 chroma 猜，猜不到不上色）；不畫 ```` ``` ````，語言名寫在區塊右上角（灰） |
| 引用 | 開頭一條 Lavender 的直槓（多層引用多條），Mantle 底，字 Text |
| 圖片 | image icon＋alt（沒有 alt 寫網址的檔名）；完整網址在 Focus 到它時寫在 hint 列；包在連結裡的圖片：link icon＋image icon＋alt |
| 連結 | link icon＋連結文字；網址在 Focus 到它時寫在 hint 列 |
| 表格 | 框線畫成格子；表頭粗體，下面一條分隔線，不另外鋪底色（user 2026-10-28：鋪了會蓋掉框線、超出表格）；太寬切掉加 `…` |
| 分隔線 | 一條橫跨面板的細線（Overlay0） |
| 表單與欄位 | 照現在的方框；欄位、按鈕、勾選框用 webu 既有的輸入框、按鈕樣子 |
| Focus 中的項目 | tdp 的 cursor 色塊 |

## 按鍵（user 2026-10-28 定）

- **閱讀（不在任何 mode）**：`j|k` 一次捲一行，`u|d` 半頁，`gg|G` 到頭尾，像 less；`/` 問要找什麼，所有命中的字加底線，`n|N` 跳下一個、上一個（目前那一個是 cursor 色塊，tdp 的搜尋規則），`Esc` 收掉；舊的 finder 拿掉。
- **Focus mode（`f`，名字 Focus）**：`j|k` 在可以按的項目（連結、圖片、欄位、按鈕）之間跳 focus，`gg|G` 第一個、最後一個，`Enter` 按下去，`Esc` 離開；進入時 focus 畫面上第一個項目。項目的動作（開新分頁、複製網址……）在它的 Space menu。
- **Visual mode（`v`）**：照舊，從畫面最上面那一列開始；複製的是畫面上的字。
- **上一頁、下一頁**：`H|L`（原本的 `P|N`，因為 `N` 要給搜尋），寫在 [2] 下框的 hint 上（`H:back L:forward`，user 2026-10-28：不然不知道怎麼上下頁）；章節的 `n|p` 在閱讀器拿掉。
- **[1] 分頁清單**：欄名 `Title`，每列是頁面標題（外框已經寫 TABS，原本的欄名 `Tab` 重複；試過顯示網址，user 要保留標題）。每列開頭是網站的記號（user 2026-10-28）：Nerd Font 有那個網站的 icon 就用它（GitHub、Wikipedia、YouTube、Go…，`siteicon.go` 的清單），沒有就用地球；顏色是網站 favicon 的主色（頁面寫的 `<link rel=icon>`，沒有就 `/favicon.ico`；看得到的像素平均，太黑就往白調一半；SVG 讀不了），每個網站一次 session 問一次；都沒有就是藍色的地球。終端機畫不了 favicon 本身（圖片協定不是每個終端機都有、tmux 預設會擋，也放不進一格一格的排版）。載入中照舊是轉圈的記號。
- **正文／整頁**：`w`（原本的 `f` 給了 Focus mode）。
- `f` 當 Focus、`w` 當整頁是我定的預設，user 可以再改。`H|L` 給了上一頁、下一頁，原本的 `L`（網址框）就移到 `o`，跟 Vimium 一樣（`o` 開網址、`H|L`、`f`、`/` 與 `n|N`）。
- 實作：`renderReader` 從內容樹直接畫（不經 markdown 文字）；一列可以有整列的底色與字色（`row.ground`、`row.ink`，`rowLine` 鋪滿）；閱讀時沒有項目被選（`cursor` -1），Focus mode 才有；搜尋的命中照 tdp color.1 畫（底線，目前那一個是 cursor 色塊）；Focus 中的連結、圖片網址寫在框的右下（panel 的 Scroll）。

## 第 1～3 階段定下的細節

- **閱讀器的分頁**：新分頁一律是閱讀器的（`tab.reader`），頁面經 `reader.Client` 來，Chromium 不參與；Chromium 的分頁只剩舊的測試在用（`chromiumPagesDefault`，測試的 `TestMain` 打開），第 4 階段定 Chromium 還畫哪些頁。分頁有自己的上一頁／下一頁（`hist`），回去時回到離開時的位置。
- **連結直接開，不先問**：markdown 原文就寫著網址，以前「畫面看不出會去哪」的理由不在了。頁內的 `#錨點` 跳到那一節。
- **可以按的**：標題（裡面有連結時換成連結）、連結、圖片、欄位、按鈕；`j|k` 在它們之間走。
- **表單**：欄位的值記在 `reader.Field`，同一個欄位在正文和整頁裡是同一份；送出時照 HTML 的規則組值（勾選框只在勾了時、按鈕只送按下的那個、不送檔案）。只有一個可以打字的欄位時，填完問要不要送出（頁面上在那一欄按 Enter 就是送出）。不在表單裡的按鈕、`type=button`：toast 說要網頁的 JavaScript。打進欄位的值不進 App Log。
- **cookie**：`cookies.json` 放狀態目錄（tdp E3：刪了只是登出），`0600`；讀不了時照 MOVE ASIDE 問，問的時候是空的 jar、不寫檔。Settings 的 `C` 清除。
- **不是網頁的**（HTML、純文字、JSON 以外，且狀態碼不是錯誤）：下載到下載資料夾，Downloads 有一列，`x` 取消。狀態碼 400 以上照樣顯示內容，toast 說狀態。
- **`f`**：正文與整頁切換（只在閱讀器的分頁出現）。
- **這一頁的資訊**（user 2026-10-29，取代 DevTools）：[2] 的 `Space` menu 最上面三項，沒有熱鍵，各開一個 lib 的 viewer（捲動、`/`、`v` 選取複製都是 viewer 的）。
  - **Source**：抓下來的原始內容（轉成 UTF-8），照它的 media type 上色。
  - **Cookies**：這個網站（可註冊網域，`en.wikipedia.org` 算 `wikipedia.org`；IP 與 localhost 就是自己）的 cookie，`名稱=值` 一行、domain／path／到期／Secure／HttpOnly 一行；`C` 先問再清掉這個網站的 cookie（`Jar.ClearSite`）。值只在這裡顯示，不進 App Log。
  - **Requests**：載入這一頁發出的請求，照 `curl -v` 的寫法：方法與網址、狀態／到回應 header 的時間／讀到的 body 大小、`>` 送出的 header、`<` 收到的 header；轉址每一跳都是一筆。記錄方式：載入時 context 帶 `reader.Requests`，client 的 transport（`recorder`）記下每個請求；favicon 這種不是這一頁的請求不帶，就不記。
  - 留不下來的：網頁的 Console 與 web storage（沒有 JavaScript）、樣式／script／圖片的請求（webu 不抓）。

## 不再做的

相對現在的 webu：網頁的 JavaScript 互動、網頁自己的對話框（alert、confirm、prompt）、iframe、靠 JavaScript 或第三方的登入、現在的 DevTools（要重新想，見下）。哪些真的拿掉，做到第 4 階段時寫進 CHANGELOG。

## 階段

| 階段 | 內容 | 狀態 |
|---|---|---|
| 0 | 這份設計 | 寫好（8c44e4e） |
| 1 | 取內容：`net/http`＋存檔的 cookie → 解析 → 挑正文 → 內容樹 → markdown；測試資料是照真實網站樣子寫的小頁面（真實網站的 HTML 有著作權，不放進 repo），8 個真實網站用 `tools/mddump` 與暫時的測試對照 | 完成（fea867a，`internal/reader`） |
| 2 | [2] 暫時顯示 markdown 原文，能在連結之間移動、按 `Enter` 開 | 完成 |
| 3 | 表單：GET 組網址送出；POST 帶隱藏欄位、存 cookie、能登入 | 完成 |
| 4 | 元件的畫法（render）與按鍵 | 完成（`readerrender.go`、`readerkeys.go`） |
| 5 | Chromium 完全拿掉（user 2026-10-29），空殼頁也不例外；DevTools 縮成「這一頁的資訊」；拿掉不再做的；CHANGELOG | 進行中：這一頁的資訊做好 |

## 還沒定的

- 空殼頁怎麼判斷：實驗工具用「`<body>` 文字少於 200 字又有 script」，x.com 沒登入時回的是靜態登入頁，這條判不出來，要找真正的空殼頁調整。
- 切到整頁的鍵、現在 [2] 那排 PAGE／HEADER／BODY／OTHERS／FOOTER 的 tab 怎麼處理。
- 按鈕（不在表單裡的 `<button>`）要怎麼顯示、按了做什麼：沒有 JavaScript 時多半什麼都不會發生。
