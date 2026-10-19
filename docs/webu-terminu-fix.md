# webu：還沒符合 tdp 的地方

對照 tdp v0.7.4 與它的 checklist（2026-10-18／19），還沒符合、或要等 terminu-go 與規範才改得完的。都已回報 terminu，編號是回報的
WB。全部修完就刪掉這個檔（tdp E7）。

| 違反 | 在哪裡 | 現況 | 該怎麼改 |
|---|---|---|---|
| tdp components/input/file-picker（checklist file-picker-35）：第二層樹枝從第一層名字那一欄開始，之後每深一層再縮 3 格 | lib 的 `drawTree`（terminu-go v0.1.13 `preview.go`）；webu 上傳、匯入書籤的目錄預覽與壓縮檔預覽 | 第二層樹枝從第 0 欄畫起，比圖少 2 格 | 等 WB8：lib 改好、發 tag 後升 `go.mod` |
| tdp E5（checklist E5-10、E5-13）：popup 比畫面大不能當掉，畫面測試涵蓋每一種 popup 比畫面大 | lib 的 finder（`finder.go` 的 `drawInfo`）；`internal/ui/app_test.go` 的 `TestEveryPopupFits` | file-picker 開著時把畫面縮到 20×6，lib panic；測試在高度小於 8 時跳過 file-picker | 等 WB12：lib 修好後拿掉測試裡的跳過（`TODO(lib WB12)`） |
| tdp T1（checklist T1-1、T1-2）：從 menu 開 confirm，取消或完成後回到原 menu | `internal/ui/pageactions.go` 的 `leavingLink`、`enterItem` | 一行的 list item 裡有一個連結加別的字：要先進去才知道它只有一行，於是「開連結」的 confirm 在 Do 裡才開；從 Space menu 執行時 menu 已經關了，`Esc` 回到 panel。其他會問的（連結、只裝一個連結的 cell 或 list item）都是 `Action.Ask`，回得到 menu | 等 WB15：lib 在 Do 開了 popup 時保留 menu，或讓 `Action.Ask` 按下去時才決定 |
| tdp M9（checklist M9-1）：同一個鍵在兩個 panel 做不同事，只有 focus 那邊的 hint 亮 | DevTools 的 [1]（`internal/ui/devtools.go`，Stream panel） | lib 不淡化 Stream panel 的 hint；暫時拿掉 [1] 的 `Enter:detail` hint 避開（Space menu 與 `?` 還列著） | 等 WB13：lib 讓 Stream 只管內容，hint 照樣淡化；之後把 hint 加回來（`TODO(lib WB13)`） |
| tdp components/layout/popup（checklist components-21）：標題裡的名稱照原樣 | `internal/ui/pageactions.go` 的 `enterCell` | lib 的 `App.Menu` 沒有 Subject、標題整串轉大寫；表格 cell 的 menu 標題暫時固定寫 `Cell`，欄名放在 Content 那一列的說明 | 等 WB9：lib 補 Subject 後改成 `CELL: <欄名>`（`TODO(lib WB9)`） |
| tdp F1、components/dialog/confirm：confirm 與 input 用 lib 的 | `internal/ui/pageask.go` | 頁面的 alert／confirm／prompt／beforeunload 與 HTTP 登入是 webu 自己的 popup（`App.Popup`），因為 lib 的 confirm、input 被 `Esc` 關掉時不通知 app，頁面收不到「取消」（寫在 dev-remarks「偏離 tdp」） | 等 WB3：lib 通知 app 後換回 lib 的 confirm 與 input（`TODO(lib WB3)`） |
| tdp E8：畫面與 core key 由 lib 管，app 呼叫它而不自己做一份 | `internal/ui/web.go` 的 `goWeb`、`focusPage` | lib 沒有從程式切畫面、移 focus 的呼叫，webu 送合成的按鍵（`W`、`2`） | 等 WB6：lib 補上後改用它（`TODO(lib WB6)`） |
| tdp E3（checklist E3-3、E3-4）：狀態放在哪、`<APP>__STATE` | `internal/store/store.go`（`session.yaml`） | tdp 沒定義什麼算狀態；上次的分頁（`session.yaml`）跟歷史、書籤一起放在資料目錄，沒有 `WEBU__STATE` | 等 WB10：tdp 定義狀態後照做；要搬的話先讀舊位置 |
| tdp E3：macOS 的 cache 在 `~/Library/Caches/<app>` | `internal/paths/paths.go` | 已經照 E3 放；但 macOS 上已經抓到舊的 `XDG_CACHE_HOME` 位置的 Chromium，暫時沿用舊的 | 等 WB5：決定舊位置的 Chromium 要自動搬、重新下載，還是一直沿用（`TODO(tdp WB5)`） |
| tdp project §目錄（checklist project-11）：Go 程式只在 `cmd/<app>/` 與 `internal/` | `tools/axdump/`（自己的 `go.mod`） | 開發用的小工具留在頂層 `tools/`；project 同一段又說小工具由 app 決定，前後矛盾 | 等 WB11：tdp 定下來；照「只在兩處」就搬到 `internal/tools/axdump`、併進 webu 的 `go.mod` |
| tdp project 範本檔（`.goreleaser.yaml`） | `.goreleaser.yaml` 的 `ignore` | 排除 linux/arm64：Chromium snapshot 沒有 Linux ARM 版；範本沒有排除平台的地方（寫在 dev-remarks「偏離 tdp」） | 等 WB1：範本補上排除平台的寫法（`TODO(tdp WB1)`） |
