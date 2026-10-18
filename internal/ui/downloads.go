package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/page"
)

// download is one file the browser is saving (function.md §8), as the
// Downloads screen lists it. The list is this session's: Chromium writes
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

// describe is the row's state column: how far along, or where it went.
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

// buildDownloads registers the Downloads screen (ui.md §2): this session's
// downloads, newest first.
func (m *AppModel) buildDownloads() {
	m.dlList = &terminu.List{
		Columns:    []terminu.Column{{Title: "File", Flex: true}, {Title: "State", Flex: true, Path: true}},
		Filterable: true,
		EmptyFact:  "No downloads this session",
		EmptyHint:  "Press [W] to browse, or [Space] to see what you can do here",
	}
	done := func() bool { d, ok := m.dlCurrent(); return !ok || d.state != dlDone }
	m.dlPanel = &terminu.Panel{Label: "Downloads", Body: m.dlList, Actions: []terminu.Action{
		{Key: "Enter", Label: "Open", Desc: "the file, in the desktop's app for it", Scope: terminu.ItemScope, Hint: "open",
			Disabled: done, Do: m.do(m.openDownload)},
		{Key: "o", Label: "open source", Desc: "the page it came from, in a new tab", Scope: terminu.ItemScope,
			Do: m.do(func() tea.Cmd {
				if d, ok := m.dlCurrent(); ok && d.url != "" {
					return m.openTab(d.url, true)
				}
				return nil
			})},
		{Key: "y", Label: "yank path", Desc: "where it was saved, or its URL while it is on its way", Scope: terminu.ItemScope,
			Do: func() tea.Cmd {
				d, ok := m.dlCurrent()
				switch {
				case !ok:
					return nil
				case d.path != "":
					return copyToClipboard(d.path)
				}
				return copyToClipboard(d.url)
			}},
		{Key: "x", Label: "Remove", Desc: "the row; a download still running is stopped", Scope: terminu.ItemScope, Hint: "remove",
			Do: m.do(m.removeDownload)},
		{Key: "C", Label: "Clear", Desc: "the finished and cancelled rows", Scope: terminu.PanelScope,
			Disabled: func() bool {
				for _, d := range m.dls {
					if d.state != dlRunning {
						return false
					}
				}
				return true
			},
			Do: m.do(func() tea.Cmd { m.clearDownloads(); return nil })},
	}}
	m.dlScreen = m.a.AddScreen(&terminu.Screen{Name: "Downloads", Key: "D", Desc: "the files saved this session",
		Panels: []*terminu.Panel{m.dlPanel}})
}

// refreshDownloads lists the downloads, newest first, as Chrome does.
func (m *AppModel) refreshDownloads() {
	if m.dlList == nil {
		return
	}
	rows := make([]terminu.Row, 0, len(m.dls))
	for i := len(m.dls) - 1; i >= 0; i-- {
		d := m.dls[i]
		state := d.describe()
		switch d.state {
		case dlRunning:
			state = terminu.Fg(terminu.Green).Render(state) // in progress (components/color)
		case dlCancelled:
			state = terminu.Fg(terminu.Overlay0).Render(state)
		}
		rows = append(rows, terminu.Row{ID: d.guid, Cells: []string{oneLine(d.name), state}})
	}
	m.dlList.SetRows(rows)
}

// dlCurrent is the download under the cursor.
func (m *AppModel) dlCurrent() (download, bool) {
	r, ok := m.dlList.Current()
	if !ok {
		return download{}, false
	}
	if i := m.downloadIndex(r.ID); i >= 0 {
		return m.dls[i], true
	}
	return download{}, false
}

// noteDownload folds one browser event into the list and says what is
// worth saying: a start and an end get a toast, progress only redraws.
func (m *AppModel) noteDownload(msg downloadMsg) tea.Cmd {
	var cmd tea.Cmd
	if msg.begin {
		m.dls = append(m.dls, download{guid: msg.guid, name: msg.name, url: msg.url, at: time.Now()})
		cmd = m.toast("Downloading " + msg.name)
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
			cmd = m.toast("Saved " + msg.path)
		case msg.failed:
			d.state = dlCancelled
			cmd = m.toastErr("Download cancelled: " + d.name)
		}
	}
	m.refreshDownloads()
	return cmd
}

func (m *AppModel) downloadIndex(guid string) int {
	for i := range m.dls {
		if m.dls[i].guid == guid {
			return i
		}
	}
	return -1
}

// downloading is how many are still in flight: the chip row says so, and
// q asks first while any is (ux.md §5).
func (m *AppModel) downloading() int {
	n := 0
	for _, d := range m.dls {
		if d.state == dlRunning {
			n++
		}
	}
	return n
}

// downloadProgress blends the running downloads into one percent for the
// line under the chips: bytes so far over bytes expected, counting only the
// ones whose size the server told us. moving is whether anything runs at
// all.
func (m *AppModel) downloadProgress() (pct int, moving bool) {
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

// openDownload is Enter on a row: the file, in whatever the desktop opens
// it with. A file still arriving is not a file yet.
func (m *AppModel) openDownload() tea.Cmd {
	d, ok := m.dlCurrent()
	if !ok || d.state != dlDone {
		return nil
	}
	if err := openFile(d.path); err != nil {
		return m.a.Error("Open failed", err.Error())
	}
	return m.toast("Opened " + d.name)
}

// removeDownload is x on a row: the row goes, and a download still running
// is stopped first. The file, if it landed, stays where it is.
func (m *AppModel) removeDownload() tea.Cmd {
	d, ok := m.dlCurrent()
	if !ok {
		return nil
	}
	i := m.downloadIndex(d.guid)
	m.dls = append(m.dls[:i], m.dls[i+1:]...)
	m.refreshDownloads()
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
	m.refreshDownloads()
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

// humanBytes is a size the way a person reads one.
func humanBytes(n float64) string {
	switch {
	case n < 1024:
		return strconv.FormatFloat(n, 'f', 0, 64) + " B"
	case n < 1024*1024:
		return strconv.FormatFloat(n/1024, 'f', 1, 64) + " KB"
	case n < 1024*1024*1024:
		return strconv.FormatFloat(n/1024/1024, 'f', 1, 64) + " MB"
	}
	return strconv.FormatFloat(n/1024/1024/1024, 'f', 1, 64) + " GB"
}
