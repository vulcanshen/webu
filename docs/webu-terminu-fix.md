# webu：還沒符合 tdp 的地方

對照 tdp v0.8.0（2026-10-19），還沒符合、要等 terminu-go 才改得完的。都已回報 terminu，編號是回報的 WB 或定案的 S；
user 都已定案，lib 的部分在 terminu-go 的第一、二、三批，等它發版、webu 升一次 `go.mod` 後改掉。全部修完就刪掉這個檔（tdp E7）。

| 違反 | 在哪裡 | 現況 | 該怎麼改 |
|---|---|---|---|
| tdp components/input/file-picker（checklist file-picker-35）：第二層樹枝從第一層名字那一欄開始，之後每深一層再縮 3 格 | lib 的 `drawTree`（terminu-go v0.1.13 `preview.go`）；webu 上傳、匯入書籤的目錄預覽與壓縮檔預覽 | 第二層樹枝從第 0 欄畫起，比圖少 2 格 | WB8：lib 第一批 #6 已修，發版後升 `go.mod` 就好 |
| tdp E5（checklist E5-10、E5-13）：popup 比畫面大不能當掉 | lib 的 finder（`finder.go` 的 `drawInfo`）；`internal/ui/app_test.go` 的 `TestEveryPopupFits` | file-picker 開著時把畫面縮到 20×6，lib panic；測試在高度小於 8 時跳過 file-picker | WB12：lib 第一批 #1；升級後拿掉測試裡的跳過（`TODO(lib WB12)`） |
| tdp T1（checklist T1-1、T1-2）：從 menu 開的 confirm，取消一律回到 menu | `internal/ui/pageactions.go` 的 `leavingLink`、`enterItem` | 一行的 list item 裡有一個連結加別的字：要先進去才知道它只有一行，於是「開連結」的 confirm 在 Do 裡才開；從 Space menu 執行時 menu 已經關了，`Esc` 回到 panel。其他會問的都是 `Action.Ask`，回得到 menu | WB15：lib 第二批 #3（按下去才決定問不問）；升級後改用它（`TODO(lib WB15)`） |
| tdp T2、M9（checklist M9-1）：串流 panel 失焦時 hint 照樣變暗；同一個鍵在兩個 panel 做不同事，只有 focus 那邊的 hint 亮 | DevTools 的 [1]（`internal/ui/devtools.go`，Stream panel） | lib 不淡化 Stream panel 的 hint；暫時拿掉 [1] 的 `Enter:detail` hint 避開（Space menu 與 `?` 還列著） | WB13（S26）：lib 讓 hint 照樣淡化；升級後把 hint 加回來（`TODO(lib WB13)`） |
| tdp components/layout/popup（checklist components-21）：標題裡的名稱照原樣 | `internal/ui/pageactions.go` 的 `enterCell` | lib 的 `App.Menu` 沒有 Subject、標題整串轉大寫；表格 cell 的 menu 標題暫時固定寫 `Cell`，欄名放在 Content 那一列的說明 | WB9：lib 第二批 #4；升級後改成 `CELL: <欄名>`（`TODO(lib WB9)`） |
| tdp F1、components/dialog/confirm：confirm 與 input 用 lib 的 | `internal/ui/pageask.go` | 頁面的 alert／confirm／prompt／beforeunload 與 HTTP 登入是 webu 自己的 popup（`App.Popup`），因為 lib 的 confirm、input 被 `Esc` 關掉時不通知 app，頁面收不到「取消」（寫在 dev-remarks「偏離 tdp」） | WB3：lib 第二批 #5；升級後換回 lib 的 confirm 與 input（`TODO(lib WB3)`），偏離跟著拿掉 |
| tdp K3、E8：focus 在使用者預期時換，app 用 lib 的接口換畫面、移 focus | `internal/ui/web.go` 的 `goWeb`、`focusPage` | 書籤、歷史的 `Enter` 開新分頁、`T` 開新分頁、網頁開出新視窗時把使用者帶過去，是使用者預期的（S19）；但 lib 沒有接口，webu 送合成的按鍵（`W`、`2`） | WB6（S19）：lib 第三批；升級後改用它（`TODO(lib WB6)`） |
| components/dialog/toast（checklist toast-18、toast-19）：成功做完的用 Success toast | `internal/ui` 各處的 `m.toast(...)`：`Copied …`、`Saved …`、`Added …`、`Imported …`、`Renamed …`、`… saved to config.yaml`、`Opened …` | lib 還沒有 Success，暫時用 Info | S8：lib 第三批加 Success 後改用它 |
| components/input/number：有單位的數字，打字列下面一列單位列 | `internal/ui/settings.go` 的 `settingBox`（`input_tab_width`） | Settings 上已經寫 `4 spaces`；輸入框還沒有單位列，說明列寫著單位 | S47：lib 第三批的 NumberInput 單位欄位；升級後補上（`TODO(lib S47)`） |
| tdp M5：label 照它本來的大小寫寫，熱鍵字母出現在 label 裡就原地包（不分大小寫） | `bookmarks.go`、`devtools.go`、`downloads.go`、`history.go`、`web.go` 的 Action：`move`、`rename`、`yank`、`add`、`close`、`reload`、`open in new tab`、`yank url`、`open source`、`yank path`、`input JavaScript` | terminu-go v0.1.13 的 MarkLabel 分大小寫，label 寫成小寫才包得進去（畫出來是 `[m]ove`）；`?` 裡去掉標記的 label 因此是小寫 | S9：lib 第三批的 MarkLabel 不分大小寫；升級後 label 改回 `Move`、`Rename`…… |
| tdp v0.8.0 其他由 lib 畫的部分（例：L1 比最低尺寸小時只畫名稱與提示、M6 與 K6 的移動鍵、popup hint 留 `Esc`、好幾層一起收起、空狀態連表頭一起取代、表單打字說明 note 吃掉 `q`） | lib | webu 照 terminu-go v0.1.13 畫 | lib 第三批；升級後整個畫面再看一次 |
