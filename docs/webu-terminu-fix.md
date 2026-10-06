# webu — terminu fix

webu 發版前要修的 bug，逐條待修。修好一條就刪掉一條。

盤點日期：2026-10-06。依據 `main` 的 `7308ada`（已對齊 tdp v0.1.23，工作區乾淨）。**這一輪不是 tdp 改版**：是 input 盤點
（2026-09-29，terminu `.local/input-survey/webu.md`）翻出來、跟之後 tdp components 怎麼定無關的 bug；五個 app 修完就發版。
tdp 版本不變，連結不用改。這份清單由 terminu session 寫好留在工作樹，還沒 commit。


## 先看

- 每條先寫一個會紅的測試，再修；CHANGELOG `[Unreleased]` 的 Fixed 各記一條（合併或拆開由 webu 定）。
- 第 1 條是五個 app 共通的做法，五份清單的「做法」同一段文字：畫出來的樣子與行為要一樣，程式怎麼寫各 app 自己定。
- 最後「這一輪不修」列的要等 components 的 input 檔定案，不要先動。
- 修完刪掉這份清單。不 push、不打 tag、不發版：發版是下一步，版號由 user 在 terminu session 一個一個定。把這一輪寫進
  terminu `.local/family-fix/webu/README.md`。


## 1. 單行的值收進換行、Tab 與控制字元 —— 五個 app 共通

**現況**：`inputPopup`、`filePicker`、`finder`、`listPanel`、`devtoolsPopup`、`selectMode` 各自收字（`inputpopup.go:171-182`、
`filepicker.go:192-211`、`finder.go:338-380`、`listpanel.go:180-192`、`devtools.go:146-158`、`selectmode.go:93-112`），貼上的
換行原樣進值，框的那一列被折斷。

**做法**（五個 app 同一段，2026-10-06 user 定案；之後寫進 tdp components 的 input 檔）：

- 範圍：單行的值 —— 一行文字、路徑、密碼／PIN、搜尋與篩選列。多行編輯框不在這條（只有 webu 的 editor，另有一條）。
  只收數字的欄位照舊只收 `0`–`9`。
- 收進值：只看以文字進來的字元（貼上的那一段）。按下去的 `Tab`、`Enter`、`Ctrl-J` 這些鍵照舊做它們原本的事（K2、K3），
  不變成值裡的字元（locku 修的時候補的，2026-10-06）。
  - 換行（`\r\n` 算一個，單獨的 `\n`、`\r` 也各算一個）與 Tab 原樣留在值裡，不換成空白、不刪。`\r\n` 原樣存或存成
    `\n` 都可以（locku 原樣存），畫、數、刪都當一個。
  - 其他控制字元（其餘的 C0、DEL、C1）丟掉。
- 畫：換行畫成 `\n`、Tab 畫成 `\t`，Red `#f38ba8`，佔 2 格，跟手打的 `\`、`n`（一般值的顏色）分得開。量寬、截斷、
  捲動都把它當成一個 2 格寬、不能切開的單位。
- 遮罩的值：照樣遮罩，一個換行或 Tab 也是一顆遮罩符號，不露出 `\n`；使用者靠錯誤列知道。
- 刪：`Backspace` 一次刪掉整個（它本來就是一個字元）。
- 送出：值會被拿去用的 input（送出、存檔、執行、交給別的程式），值裡有換行或 Tab 時 `Enter` 不送出，錯誤列說出哪一欄
  不能有換行或 Tab（英文；句式、大小寫照該 app 現有的錯誤訊息，例：`Name can't have line breaks or tabs`）。其他照 K3：
  多欄表單 focus 跳到第一個不合格的欄位、label 變 Red；有「第一次送出後每鍵重驗」的照舊。
  - 原本送出不會失敗、所以沒預留錯誤列的 input，現在會失敗了，照 F7 打開時就預留錯誤列。
- 搜尋與篩選列（值只拿來找東西，不存、不執行；`Enter` 選的是清單裡的項目）：只照上面畫，不擋。

**為什麼**：單行的值裡換行沒有意義。原樣畫出來會把框畫壞；偷偷換成空白或刪掉，又改了使用者的值而看不出來（user：
「應該轉成 `\n` 或 `\t` 這種明確顯示」）。只在畫面上轉、值裡留原字元，是為了跟手打的 `\n` 分得開，也不會把沒有意義的
字元送出去。

**webu 要改的地方**：
- 收字：上面六處（editor 見第 2 條）。
- 畫：各自的值、query、篩選列畫 Red `\n`／`\t`；頁面的密碼欄與 Sign in 的密碼照樣遮罩。
- 擋：所有 `inputPopup`（Location、New tab、頁面的一行文字／密碼／搜尋框／日期時間顏色欄位、JS `prompt()`、Sign in、
  Console REPL、Settings 的文字設定、Add bookmark、Add folder、Rename、Import 的根目錄名稱）。現在只有 `canFail()`
  （`inputpopup.go:101-107`）列的六種預留錯誤列，其他的因為這條變成會失敗，照 F7 都要預留。
- 不擋：選檔 picker 的篩選、finder、Go to line、visual mode 的搜尋、DevTools 的篩選、清單畫面的篩選。


## 2. editor 貼上的換行沒有斷行

**現況**：editor（`editorPopup`，`editorpopup.go:108-139`）在寫的狀態貼上時，整段插在游標處，換行字元原樣插進同一行、
不斷行（盤點實測貼 `x\ny` 仍是 1 行）。

**怎麼改**：貼上的換行跟按 `Enter` 一樣斷行（`\r\n` 算一個）；Tab 跟按 `Tab` 一樣（現在插 4 個空白）；其他控制字元丟掉。
editor 是多行編輯框，第 1 條不適用。


## 3. `docs/ux.md` 的 finder 跟實際行為對不上

`docs/ux.md` §1.1「finder（`/`）與 go」寫清單上「再打字回到輸入」；盤點實測清單階段打字母沒有作用（仍在清單、query
不變）。哪個是本意由 webu 定：文件對就改程式，程式對就改文件。


## 這一輪不修

- 清單畫面與 DevTools 的篩選列，游標固定在最右邊。
- editor 水平捲動以字元計，CJK 對不齊。


## locku 先修完的經驗（2026-10-06）

- **值裡的 `\t`、`\n` 不能直接交給 lipgloss**：`Render` 會把 Tab 換成空白、在換行處斷成兩列。先換成要畫的 `\n`、`\t`，
  再上色。
- **同一個值有兩條路進來，要用同一個過濾。** locku 的設定畫面不過濾、鎖定畫面只收 `IsPrint`，兩邊各自合理，合起來就設得出
  一個解不開的 PIN。
- **測試要用跟舊行為不同的輸入。** 貼 `12\n34` 時字元數剛好等於單位數，測不出「`\r\n` 算一個」；換成 `12\r\n34` 才分得出來。

## webu 的裁定（2026-10-06，user 同意）

- **收字只看貼上的那一段**：Bubble Tea v1.3.10 的 bracketed paste 把整段原樣放進一個 `KeyRunes`（換行、`\r`、Tab、ESC 都在裡面）；
  按下去的 Tab、Enter、Ctrl-J 是別的 `msg.Type`，本來就不會變成字元。
- **`\r\n` 存成 `\n`**：一個 rune 就是一個單位，`Backspace`、遮罩、量寬都不用另外處理 `\r\n`。單獨的 `\r` 原樣留，畫成 `\n`。
- **預填的值走同一個過濾**：頁面 `prompt()` 的預設值、頁面欄位原本的值、匯入的書籤標題、`config.yaml` 的設定值，打開時就可能有換行、
  Tab 或其他控制字元。換行與 Tab 一樣畫 Red、一樣擋；其他控制字元丟掉，否則 ESC 會直接進終端機（locku 的經驗：同一個值有兩條路進來，
  用同一個過濾）。
- **擋的範圍是 12 種 `inputAction` 全部**，所以 `canFail()` 拿掉，每個 input 打開時就預留錯誤列。原本沒有的 6 種（Location、New tab、
  頁面的一行文字欄、`prompt()`、Sign in、Console REPL）各高兩列。
- **Console REPL 照擋**：多行的 JS 片段不能整段貼進去跑，只能一行一行打（user 知道這個影響，不給 REPL 例外）。
- **第 3 條改文件**：程式對。tdp F1 清單階段是 menu，`j/k/u/d/g/G` 都是清單的鍵；回到輸入是 `Tab`（`d26ba56`）。「再打字回到輸入」
  是 0.3.0 寫文件時（`5a8ca04`）寫的，程式從來沒這樣做過。同一句的 `gg` 照程式改成 `g`（finder 清單、message、editor 都是單按 `g`；
  頁面、Space menu、visual mode 是 `gg`，這個不一致這一輪不動）。


## 待確認

沒有。
