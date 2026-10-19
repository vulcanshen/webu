# webu：還沒符合 tdp 的地方

對照 tdp v0.8.1、terminu-go v0.2.0（2026-10-19），還沒符合、要等 terminu-go 才改得完的。都已回報 terminu，編號是回報的 WB。
全部修完就刪掉這個檔（tdp E7）。

| 違反 | 在哪裡 | 現況 | 該怎麼改 |
|---|---|---|---|
| tdp F1、components/dialog/confirm、components/input/README：問一個值用 lib 的 input | `internal/ui/pageask.go`：頁面的 prompt | 頁面的 prompt 是 webu 自己的 popup（`App.Popup`）。lib 的 TextInput 在值跟打開時一樣（例：預設答案是空的）時按 `Enter`，`Done` 與 `OnCancel` 都不呼叫，webu 聽不到這個回答，頁面會一直等（寫在 dev-remarks「偏離 tdp」） | WB17：lib 讓 app 聽得到「照原值回答」；之後換成 lib 的 TextInput（`TODO(lib WB17)`） |
| components/layout/panel（一般清單，欄放不下時）：先整欄收、再照比例縮，留下的欄讀得完 | `internal/ui/devtools.go` 的欄（Network、Console、Source） | 網址、一行文字的欄永遠放不下：能收的欄一定被收；設成不收，照比例縮時旁邊的短欄（Method、Status、Level、行號）縮成 `…`。暫時讓短欄收掉，`[2]` 照樣寫全 | WB19：規範或 lib 定「比例縮」不把短欄縮成 `…`（例：從最寬的欄先縮），之後短欄設回不收（`TODO(lib WB19)`） |
| tdp F1、components/dialog/form：一列一個值、按鈕送出的用 lib 的表單 | `internal/ui/pageask.go`：HTTP 登入 | 名字與密碼兩個值，照規範是表單；lib 的 Form 被 `Esc` 關掉時不通知 app，頁面收不到「取消」，所以是 webu 自己的 popup（`n` 名字、`p` 密碼、`Enter` 送出、`Esc` 取消） | WB18：lib 的 Form 加 `OnCancel`；之後換成 lib 的表單（`TODO(lib WB18)`） |
