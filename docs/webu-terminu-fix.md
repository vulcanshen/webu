# webu — terminu fix

webu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.0/principle)（tdp v0.1.0）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/ux.md`、`docs/ui.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-26。行號以當天的 `main` 為準。


## 先看：locku 修完的經驗（2026-09-26）

locku 是第一個照 tdp v0.1.0 修完的 app（v0.1.2、v0.1.3），修的時候發現這些，這裡的各條也適用：

- **每修一條補一個 model test**，測試名稱或註解寫明 tdp 條目（locku `internal/ui/app_test.go`）。
- **`?` menu 的 global 列也適用 F4**：選了會開下一個框（confirm、輸入、選項）的列，`?` menu 留在底下，
  取消回到它，完成才清掉整疊。浮層因此最多三層（menu、它開出的框、框上的 `?` help），D2 的層色要夠用。
- **K9 的離開流程**：離開的 confirm 開著時，`Ctrl-C` 直接離開；`q` 在離開 confirm 上不再疊一個。
- **S3**：splash 的判斷放在 `Ctrl-C` 之前，任何鍵都只關 splash。
- **F3**：判斷「popup 還在不在」用開啟中或已開（`owns()` 一類），不用含關閉中的 `isActive()`；toast 也一樣。
- **刻意保留的行為寫成偏離**：locku 的全螢幕預覽「任何鍵就回來」、欄位字典取代 `?` menu，都寫進
  `dev-remarks.md`「偏離 tdp」並附理由，而不是留在 fix 檔。
- **文件裡的 tdp 連結釘在 `v0.1.0` tag**（本檔與 README、dev-remarks 已改好）。
- **修完一批就更新 CHANGELOG、發一版**，README 與 `ux.md` 等描述行為的段落同一個 commit 改。

---

## 11. 程式碼註解仍引用 VTP 的 § 編號、u-family 與已退場的 implementation 文件 —— 文件對齊

- **現況**：`internal/ui` 等處的註解用 VTP 的 § 編號、「u-family」與 `webu-implementation.md`（不影響行為）。只換
  引用 VTP 的；寫明 `ux.md §…`、`ui.md §…`、`function.md §…` 的指的是 webu 自己的設計文件，不動（`ux.md` 保留原本的
  章節編號）。`main.go:48` 的 `§9`、`settings.go:13` 的 `§6`、`tab.go:306` 的 `§8`、`page/actions.go:263` 的 `§4`
  是上一行 `function.md` / `ui.md` 的延續，`app.go:983` 的 `§1`、`app.go:1378` 的 `§A.1` 是上一行 `ux.md` 的延續，
  都不動。
- **怎麼改**：照 [terminu `vtp/README.md` 的對照表](https://github.com/vulcanshen/terminu/blob/v0.1.0/vtp/README.md) 換成 tdp 編號：

| 檔案:行 | 現在 | 換成 |
|---|---|---|
| `cmd/webu/main.go:119` | `u-family: leave with no child behind` | `terminu family: leave with no child behind` |
| `internal/browser/launch.go:183` | `u-family: leave with no child left behind` | `terminu family: leave with no child left behind` |
| `internal/page/actions.go:54` | `webu-implementation.md §4 says why not` | `docs/dev-remarks.md「運作方式」says why not` |
| `app.go:90` | `§6.4` | `tdp F4` |
| `app.go:721` | `§6.2` | `tdp F3` |
| `app.go:730` | `§4.5` | `tdp K8` |
| `app.go:745` | `§4.3` | `tdp K4` |
| `app.go:833` | `§6.4` | `tdp F4` |
| `app.go:836` | `§6.2` | `tdp F3` |
| `app.go:927` | `§7.1` | `tdp T1` |
| `app.go:2793` | `§A.1 / §A.2` | `tdp M1` |
| `chrome.go:121` | `§1.1` | `tdp L1` |
| `chrome.go:136` | `§1.1` | `tdp L5`（見「待確認」） |
| `chrome.go:138` | `§1.3` | `tdp L3, L4` |
| `chrome.go:143` | `§7.2` | `tdp T2` |
| `chrome.go:178` | `§A.1 / §A.2` | `tdp M1` |
| `chrome.go:201`、`:265` | `§2.1` | `tdp D2` |
| `confirm.go:26` | `§6.1` | `tdp F1` |
| `confirm.go:64` | `§4.3` | `tdp K4` |
| `devconsole.go:11` | `§2.4` | `tdp D2` |
| `devnetdetail.go:16` | `kbu §6.0.4` | `tdp D3` |
| `devtools.go:255` | `kbu §8.2's starship chain` | `the family's powerline chain (tdp D1)` |
| `filepicker.go:22` | `§4.5` | `tdp K8` |
| `inputpopup.go:33` | `§6.1` | `tdp F1` |
| `inputpopup.go:38` | `§4.5` | `tdp K8` |
| `inputpopup.go:87` | `§4.3` | `tdp K4` |
| `inputpopup.go:134` | `§B` | `tdp P4` |
| `listpanel.go:173` | `§4.3` | `tdp K4` |
| `listpanel.go:301` | `§4.4` | `tdp M5` |
| `messagepopup.go:8` | `§6.1` | `tdp F1` |
| `pagepanel.go:119` | `VTP's z-axis` | `tdp D2's z-axis` |
| `pagepopup.go:169` | `VTP's z-axis` | `tdp D2's z-axis` |
| `popup.go:12` | `§2.2 / §6.3` | `tdp D2` |
| `popup.go:14` | `§B` | `tdp P4` |
| `popup.go:31` | `§6.2 … inside the 100-200ms band` | `tdp F2 … the family default (tdp D3)` |
| `popup.go:71` | `§6.2` | `tdp F2` |
| `popup.go:151` | `§4.5` | `tdp K8` |
| `popup.go:208`、`:214`、`:262` | `§4.4` | `tdp M5` |
| `popup.go:282` | `ux.md §A.1.2's in-place form`（`ux.md` 沒有 §A.1.2，那是 VTP 的） | `tdp M5's in-place form (tdp D4)` |
| `render_test.go:679` | `the VTP's lerp` | `tdp D2's lerp` |
| `spacemenu.go:12` | `§4.2` | `tdp M3` |
| `spacemenu.go:29` | `the §A.1 contextual entry point` | `the Space menu (tdp K5, M2)` |
| `spacemenu.go:215` | `§4.4` | `tdp M5` |
| `splash.go:19` | `u-family mark` | `terminu family mark` |
| `theme.go:16` | `the VTP in full` | `tdp D2 in full` |
| `theme.go:32` | `VTP §B` | `tdp P4` |
| `theme.go:37` | `§4.4` | `tdp M5` |
| `theme.go:47` | `§2.4` | `tdp D2` |
| `toast.go:26` | `§6.5` | `tdp F3` |
| `width.go:11` | `§1.2` | `tdp L2` |

（`app.go`、`chrome.go` 等未寫目錄的都在 `internal/ui/`。）

---

## 待確認

- **visual mode 裡 `Tab` 不作用（K1、K2）。** `selectMode.key()` 不認 `Tab`，visual mode 開著時 `Tab` 切不了 panel。
  visual mode 是 K4 說的「模式」（`Esc` 先離開它）；`Tab` 在模式裡該先離開模式再換 panel、還是模式開著時就
  不換？
- **`chrome.go:136` 的 `§1.1`**：「the lit capsule is what says which surface you are on」看起來是 focus 的標示（tdp L5），
  但 VTP §1.1 是窄寬可用（tdp L1）；可能是從 kbu 帶過來、指 kbu 自己的文件。要換成哪個，由改程式的人看上下文決定。
- **`chrome.go:88` 的 `§11.21`、`chrome.go:241` 的 `§11.22`**：不是 VTP，
  看起來是從 kbu / sshu 帶過來的引用，webu 自己的文件裡沒有對應的節。要不要拿掉？
