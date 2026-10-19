package ui

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/store"
)

// The History screen (ui.md §2): every page loaded, newest first, kept
// until it is cleared.

func (m *AppModel) buildHistory() {
	m.histList = &terminu.List{
		// A title and a URL never fit whole: both stay and share the room,
		// the time going first (layout/panel).
		Columns:    []terminu.Column{{Title: "Title"}, {Title: "URL"}, {Title: "When", Drop: 1}},
		Filterable: true,
		EmptyFact:  "No history yet",
		EmptyHint:  "Press [W] to browse, or [Space] to see what you can do here",
	}
	m.histPanel = &terminu.Panel{Label: "History", Body: m.histList, Actions: []terminu.Action{
		{Key: "Enter", Label: "Open", Desc: "the page, in a new tab", Scope: terminu.ItemScope, Hint: "open",
			Do: m.do(func() tea.Cmd {
				if v, ok := m.histCurrent(); ok {
					return m.openTabBehind(v.URL)
				}
				return nil
			})},
		{Key: "y", Label: "Yank", Desc: "its URL", Scope: terminu.ItemScope,
			Do: func() tea.Cmd {
				if v, ok := m.histCurrent(); ok {
					return copyToClipboard(v.URL)
				}
				return nil
			}},
		{Key: "x", Label: "Delete", Desc: "this visit from the history", Scope: terminu.ItemScope, Hint: "delete",
			Do: m.do(m.deleteVisit)},
		{Key: "C", Label: "Clear", Desc: "every visit ever recorded", Scope: terminu.PanelScope,
			Disabled: func() bool { return len(m.history) == 0 },
			Ask: &terminu.Ask{Title: "Clear history", Details: []string{"Every visit ever recorded goes; this is the only way the log shrinks."},
				Question: "Clear the whole history?", Verb: "clear", Warn: []int{0}},
			Do: m.do(m.clearHistory)},
	}}
	m.histScreen = m.a.AddScreen(&terminu.Screen{Name: "History", Key: "H", Desc: "every page loaded",
		Panels: []*terminu.Panel{m.histPanel}})
}

// refreshHistory lists the visits, newest first.
func (m *AppModel) refreshHistory() {
	if m.histList == nil {
		return
	}
	rows := make([]terminu.Row, len(m.history))
	for i, v := range m.history {
		rows[i] = terminu.Row{ID: strconv.Itoa(i), Cells: []string{oneLine(nameOr(v.Title, v.URL)), oneLine(v.URL), ago(v.At)},
			Sort: []string{v.Title, v.URL, strconv.FormatInt(-v.At.Unix(), 10)}}
	}
	m.histList.SetRows(rows)
}

// histCurrent is the visit under the cursor.
func (m *AppModel) histCurrent() (store.Visit, bool) {
	r, ok := m.histList.Current()
	if !ok {
		return store.Visit{}, false
	}
	i, err := strconv.Atoi(r.ID)
	if err != nil || i < 0 || i >= len(m.history) {
		return store.Visit{}, false
	}
	return m.history[i], true
}

// deleteVisit is x: one visit out of the log.
func (m *AppModel) deleteVisit() tea.Cmd {
	v, ok := m.histCurrent()
	if !ok {
		return nil
	}
	for i, x := range m.history {
		if x == v {
			m.history = append(m.history[:i], m.history[i+1:]...)
			break
		}
	}
	err := store.DeleteVisit(v)
	m.refreshHistory()
	if err != nil {
		return m.saveFailed("history.yaml", err)
	}
	return nil
}

// clearHistory is C, once it is confirmed.
func (m *AppModel) clearHistory() tea.Cmd {
	m.history = nil
	m.refreshHistory()
	if err := store.ClearHistory(); err != nil {
		return m.saveFailed("history.yaml", err)
	}
	return nil
}

// ago says when, the way a person reads it: how long ago for today, the
// date otherwise.
func ago(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	}
	return t.Format("2006-01-02")
}

// refreshLists fills every list screen from what the app holds.
func (m *AppModel) refreshLists() {
	m.refreshBookmarks()
	m.refreshHistory()
	m.refreshDownloads()
	m.refreshSettings()
}
