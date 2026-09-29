# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.21/principle)（tdp v0.1.21）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。

盤點日期：2026-09-29。依據 `main` 的 `ec44e07`（已對齊 v0.1.20，工作區乾淨）。**這一輪只對 v0.1.20 → v0.1.21 的改動**
（`git -C ~/Documents/sideproj/terminu diff v0.1.20 v0.1.21 -- principle/`）：

- D5：選取文字的模式照 vim 移動 —— `h/j/k/l`、`w/b/e`、`0/$`、`gg/G`、`u/d`（user 要求寫回 tdp，filu 轉達）。
- D6：環境變數命名 `<APP>__<NAME>`（app 名後兩個底線、變數名全大寫單底線分隔），共用名 `<APP>__CONFIG` / `__STATE` / `__DATA`
  / `__CACHE`（都指向**目錄**）/ `__ICON_WIDTH`；給別的程式讀的變數例外；**改名不留舊名**（user 2026-09-29 裁定）。
- D6：疊 popup 時 popup 比畫面寬或高（調整終端機大小的那一格）：起點取 0、超出的部分切掉，不可以 panic（kbu、locku 照搬時抓到，
  filu 的參考實作有這個 bug）。

tdp 連結（README 兩份、docs、`.claude/rules`）已由 terminu session 從 v0.1.20 改成 v0.1.21，只改網址，跟這份清單一起留在工作樹，
還沒 commit。


## 先看

- 清單與改釘的連結先一起 commit，再動程式；commit 只加自己改的路徑。
- 每修一處補 model test、做 mutation；同一個 commit 同步 README 兩份與 dev-remarks；CHANGELOG 記 `[Unreleased]`。
- **環境變數改名是破壞性改動**：CHANGELOG `[Unreleased]` 要寫一條「改名，舊名不再讀」並列出新舊對照；CHANGELOG 裡已發版的舊段落不改。
  `.checkpoints/` 是本機筆記，不用改。改完 `grep -rn '<舊名>'` 除了 CHANGELOG 舊段落應該是零。
- **修完拿 v0.1.21 全文再逐條對一次**，修完刪掉這份清單。不 push、不發版；把這一輪寫進 terminu `.local/family-fix/webu/README.md`。


## 1. `composite()` 在 popup 比畫面大時 panic —— D6（v0.1.21，等 filu 先修）

**現況**（`internal/ui/width.go`）：`composite()`（照搬 filu 的 `compositeDisp()`）與 `clampSpan()` 在 popup 比畫面寬或高時起點算成負的，
`strings.Repeat` 或 `bgLines[y+i]` panic；調整終端機大小的那一格就會遇到。

**規則**：D6（v0.1.21）—— 疊 popup 時 popup 可能比畫面寬或高（調整終端機大小的那一格還是舊尺寸）：起點取 0、超出畫面的部分切掉，
**不可以 panic**；測試的邊界要含這種情況。

**怎麼改**：等 filu 清單第 1 條修完（參考實作），照搬它的 `compositeDisp()` 與測試（kbu 的 `TestD6_CompositeDisp` 同一組邊界：比畫面寬、
比畫面高、兩者都大；不 panic、每列剛好畫面寬、列數等於畫面高）。webu 保留自己的函式名（`composite`）即可。


## 已修完（2026-09-29）

第 2 條（環境變數改名 `WEBU__*`，舊名不再讀）已修完、已從本清單刪掉：`8b98d03`。只剩第 1 條，等 filu。


## 已經符合、不用修的（對照 v0.1.21 的改動）

- **D5 選取模式的移動**：visual mode 有 `h/j/k/l`、`w/b/e`、`0/$`、`gg/G`、`u/d`（`selectmode.go`）。


## 待確認

沒有。
