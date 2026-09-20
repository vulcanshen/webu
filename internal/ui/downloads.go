package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/page"
)

// download is one file the browser is saving (function.md §8), as the
// Downloads popup lists it. The list is this session's: Chromium writes
// the file, webu only watches it land.
type download struct {
	guid, name, url, path string
	received, total       int64
	state                 downloadState
	at                    time.Time
}

type downloadState int

const (
	dlRunning downloadState = iota
	dlDone
	dlCancelled
)

// describe is the row's right-hand column: how far along, or where it went.
func (d download) describe() string {
	switch d.state {
	case dlDone:
		return d.path
	case dlCancelled:
		return "cancelled"
	}
	if d.total > 0 {
		return fmt.Sprintf("%d%%  %s of %s", d.received*100/d.total, humanBytes(float64(d.received)), humanBytes(float64(d.total)))
	}
	return humanBytes(float64(d.received)) + " so far"
}

// noteDownload folds one browser event into the list and says what is
// worth saying: a start and an end get a toast, progress only redraws.
func (m *AppModel) noteDownload(msg downloadMsg) tea.Cmd {
	var cmd tea.Cmd
	if msg.begin {
		m.dls = append(m.dls, download{guid: msg.guid, name: msg.name, url: msg.url, at: time.Now()})
		cmd = m.toast.show("downloading "+msg.name, toastInfo)
	} else {
		i := m.downloadIndex(msg.guid)
		if i < 0 {
			return nil // removed from the list already
		}
		d := &m.dls[i]
		d.received, d.total = msg.received, msg.total
		switch {
		case msg.done:
			d.state, d.path = dlDone, msg.path
			cmd = m.toast.show("saved "+msg.path, toastInfo)
		case msg.failed:
			d.state = dlCancelled
			cmd = m.toast.show("download cancelled", toastError)
		}
	}
	if m.lists.isActive() && m.lists.kind == listDownloads {
		m.lists.setEntries(m.listEntries(listDownloads))
	}
	return cmd
}

func (m AppModel) downloadIndex(guid string) int {
	for i := range m.dls {
		if m.dls[i].guid == guid {
			return i
		}
	}
	return -1
}

// downloading is how many are still in flight: the header says so, and q
// asks first while any is (ux.md §5).
func (m AppModel) downloading() int {
	n := 0
	for _, d := range m.dls {
		if d.state == dlRunning {
			n++
		}
	}
	return n
}

// downloadProgress blends the running downloads into one percent for the
// header's rule: bytes so far over bytes expected, counting only the ones
// whose size the server told us. moving is whether anything runs at all.
func (m AppModel) downloadProgress() (pct int, moving bool) {
	var got, want int64
	for _, d := range m.dls {
		if d.state != dlRunning {
			continue
		}
		moving = true
		if d.total > 0 {
			got, want = got+d.received, want+d.total
		}
	}
	if want > 0 {
		pct = int(got * 100 / want)
	}
	return pct, moving
}

// downloadAt maps a row of the popup — newest first — back to the list.
func (m AppModel) downloadAt(row int) (download, int) {
	i := len(m.dls) - 1 - row
	if i < 0 || i >= len(m.dls) {
		return download{}, -1
	}
	return m.dls[i], i
}

// openDownload is Enter on a row: the file, in whatever the desktop opens
// it with. A file still arriving is not a file yet.
func (m AppModel) openDownload(row int) tea.Cmd {
	d, i := m.downloadAt(row)
	switch {
	case i < 0:
		return nil
	case d.state == dlRunning:
		return m.toast.show("still downloading", toastInfo)
	case d.state == dlCancelled:
		return m.toast.show("that download was cancelled", toastInfo)
	}
	if err := openFile(d.path); err != nil {
		return m.toast.show(err.Error(), toastError)
	}
	return m.toast.show("opened "+d.name, toastInfo)
}

// yankDownload copies the saved path, or the source URL while the file is
// still on its way.
func (m AppModel) yankDownload(row int) tea.Cmd {
	d, i := m.downloadAt(row)
	if i < 0 {
		return nil
	}
	if d.path != "" {
		return copyToClipboard(d.path)
	}
	return copyToClipboard(d.url)
}

// removeDownload is x on a row: the row goes, and a download still running
// is stopped first. The file, if it landed, stays where it is.
func (m *AppModel) removeDownload(row int) tea.Cmd {
	d, i := m.downloadAt(row)
	if i < 0 {
		return nil
	}
	m.dls = append(m.dls[:i], m.dls[i+1:]...)
	m.lists.setEntries(m.listEntries(listDownloads))
	if d.state != dlRunning || m.browser == nil {
		return nil
	}
	ctx, guid := m.browser.Ctx, d.guid
	return func() tea.Msg {
		if err := page.CancelDownload(ctx, guid); err != nil && ctx.Err() == nil {
			return actionErrMsg{err: fmt.Errorf("cancel download: %w", err)}
		}
		return nil
	}
}

// clearDownloads is C: the finished and cancelled rows go, the running
// ones stay.
func (m *AppModel) clearDownloads() {
	kept := m.dls[:0]
	for _, d := range m.dls {
		if d.state == dlRunning {
			kept = append(kept, d)
		}
	}
	m.dls = kept
	m.lists.setEntries(m.listEntries(listDownloads))
}

// openFile hands a path to the desktop's opener.
func openFile(path string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	if err := exec.Command(opener, path).Start(); err != nil {
		return fmt.Errorf("%s: %w", opener, err)
	}
	return nil
}
