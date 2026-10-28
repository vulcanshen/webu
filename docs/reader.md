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

## 元件（render 階段再定）

每種元件長什麼樣子，到 render 階段跟 user 一起定，照 tdp 的配色與規則。要定的清單：標題（h1～h6）、段落、連結、清單、表格、程式碼、引用、圖片、分隔線、表單與它的欄位（文字、密碼、多行、下拉、勾選、單選）、按鈕。

在那之前，markdown 裡表單暫時這樣寫：

```
┌ 表單：登入
│ 帳號：[            ]
│ 密碼：[ ••••••     ]
│ ( 送出 )
```

## 第 1～3 階段定下的細節

- **閱讀器的分頁**：新分頁一律是閱讀器的（`tab.reader`），頁面經 `reader.Client` 來，Chromium 不參與；Chromium 的分頁只剩舊的測試在用（`chromiumPagesDefault`，測試的 `TestMain` 打開），第 4 階段定 Chromium 還畫哪些頁。分頁有自己的上一頁／下一頁（`hist`），回去時回到離開時的位置。
- **連結直接開，不先問**：markdown 原文就寫著網址，以前「畫面看不出會去哪」的理由不在了。頁內的 `#錨點` 跳到那一節。
- **可以按的**：標題（裡面有連結時換成連結）、連結、圖片、欄位、按鈕；`j|k` 在它們之間走。
- **表單**：欄位的值記在 `reader.Field`，同一個欄位在正文和整頁裡是同一份；送出時照 HTML 的規則組值（勾選框只在勾了時、按鈕只送按下的那個、不送檔案）。只有一個可以打字的欄位時，填完問要不要送出（頁面上在那一欄按 Enter 就是送出）。不在表單裡的按鈕、`type=button`：toast 說要網頁的 JavaScript。打進欄位的值不進 App Log。
- **cookie**：`cookies.json` 放狀態目錄（tdp E3：刪了只是登出），`0600`；讀不了時照 MOVE ASIDE 問，問的時候是空的 jar、不寫檔。Settings 的 `C` 清除。
- **不是網頁的**（HTML、純文字、JSON 以外，且狀態碼不是錯誤）：下載到下載資料夾，Downloads 有一列，`x` 取消。狀態碼 400 以上照樣顯示內容，toast 說狀態。
- **`f`**：正文與整頁切換（只在閱讀器的分頁出現）。
- **DevTools**：閱讀器的分頁沒有 Chromium，Network／Console／Storage／Source 都是空的，等第 4 階段重新設計。

## 不再做的

相對現在的 webu：網頁的 JavaScript 互動、網頁自己的對話框（alert、confirm、prompt）、iframe、靠 JavaScript 或第三方的登入、現在的 DevTools（要重新想，見下）。哪些真的拿掉，做到第 4 階段時寫進 CHANGELOG。

## 階段

| 階段 | 內容 | 狀態 |
|---|---|---|
| 0 | 這份設計 | 寫好（8c44e4e） |
| 1 | 取內容：`net/http`＋存檔的 cookie → 解析 → 挑正文 → 內容樹 → markdown；測試資料是照真實網站樣子寫的小頁面（真實網站的 HTML 有著作權，不放進 repo），8 個真實網站用 `tools/mddump` 與暫時的測試對照 | 完成（fea867a，`internal/reader`） |
| 2 | [2] 暫時顯示 markdown 原文，能在連結之間移動、按 `Enter` 開 | 完成 |
| 3 | 表單：GET 組網址送出；POST 帶隱藏欄位、存 cookie、能登入 | 完成 |
| 4 | 元件的畫法（render）；Chromium 只留給空殼頁；DevTools 重新設計；拿掉不再做的；CHANGELOG | |

## 還沒定的

- DevTools 要變成什麼（user：「現在的 uiux 甚是詭異」，之後再談）；也許縮成「這個回應的資訊與原始 HTML」。
- 空殼頁怎麼判斷：實驗工具用「`<body>` 文字少於 200 字又有 script」，x.com 沒登入時回的是靜態登入頁，這條判不出來，要找真正的空殼頁調整。
- 切到整頁的鍵、現在 [2] 那排 PAGE／HEADER／BODY／OTHERS／FOOTER 的 tab 怎麼處理。
- 按鈕（不在表單裡的 `<button>`）要怎麼顯示、按了做什麼：沒有 JavaScript 時多半什麼都不會發生。
