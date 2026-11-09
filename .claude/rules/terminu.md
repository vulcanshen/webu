# terminu harness

這個 app 照 terminu design（tdp）用 terminu-go 做，harness 的紀錄在 `harness-assets/`（格式見 terminu plugin 的
`skills/newapp/harness-assets.md`）。

- 改了 app 的行為（畫面、按鍵、訊息、讀寫的檔、外部指令），同一個 commit 更新 `harness-assets/work.yaml` 那個 task 的
  implementation 與 test。
- 使用者提了新需求或改了需求：先寫進 `harness-assets/idea.md`，spec.yaml 的 id 加一，新增或修改 task（task 的 spec 改成新的 id）。
- 行為沒變的 commit（重構、改註解、改文件），commit 訊息最後加一行 `Terminu-Work: unchanged`。
- app 的每個決定都要在 tdp 找得到根據，或在 tdp 明文寫「app 可以自行決定」的範圍裡；找不到、不清楚、矛盾時用
  `/terminu:reportgap` 回報，不自己決定。
- review 只判 work.yaml 寫了的機制；程式做了、work.yaml 沒寫的，harness 不保證。
- 改完用 MCP 工具 `check` 驗 harness-assets。
