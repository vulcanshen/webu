# webu — build / run / test / package（家族共用的 target 在 family.mk）
#
# 跑 `make`（或 `make help`）列出所有指令。

BINARY := webu
include family.mk

##@ webu 自己的

.PHONY: fixtures
fixtures: ## 用釘死版本的 Chromium 重新產生 internal/ir 的 role fixture 與 docs/support.md
	go test ./internal/ir -run "TestFixtures|TestSupportDoc" -capture -update

.PHONY: axdump
axdump: ## 印出某頁的 AX tree：make axdump URL=https://…（用本機 Chrome）
	cd tools/axdump && go run . $(URL)

.PHONY: mddump
mddump: ## 實驗：net/http 抓一頁、挑出內容、印成 markdown：make mddump URL=https://…（不用 Chromium）
	cd tools/mddump && GOWORK=off go run . $(URL)
