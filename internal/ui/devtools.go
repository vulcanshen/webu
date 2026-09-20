package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/webu/internal/page"
)

// The DevTools popup (ui.md §3.2): the family's first popup with a tab
// bar. This file is the shell — the frame, the chip chain in its title,
// h/l between the three tabs, the filter line, the key routing — and each
// tab's content is its own file and sub-model (devstorage.go,
// devnetwork.go, devconsole.go). The Network tab's detail is a sub-popup
// with its own animator (devnetdetail.go), composited here.
//
// Nearly full screen and no padRow (ui.md §3.2): the Network columns want
// the width, and every row is content.

type devTab int

const (
	devStorage devTab = iota
	devNetwork
	devConsole
)

var devTabLabels = []string{"Storage", "Network", "Console"}

// devTickMsg refreshes the popup while it is open: the log fills on
// another goroutine and nothing else would redraw it.
type devTickMsg struct{ gen int }

const devTickEvery = 500 * time.Millisecond

type devtoolsPopup struct {
	anim    popupAnimator
	tab     devTab
	tabID   int // the tab whose log is shown
	storage devStorageTab
	network devNetworkTab
	console devConsoleTab
	detail  devDetailPopup

	// filter is typed with / and applies to the current tab's rows.
	filter  [3]string
	typing  bool
	tickGen int
	layer   int
	screenW int
	screenH int
}

func newDevtoolsPopup() devtoolsPopup {
	return devtoolsPopup{anim: newPopupAnimator("devtools"), detail: newDevDetailPopup()}
}

func (m devtoolsPopup) isActive() bool      { return m.anim.isActive() }
func (m devtoolsPopup) isInteractive() bool { return m.anim.isInteractive() }
func (m *devtoolsPopup) setSize(w, h int) {
	m.screenW, m.screenH = w, h
	m.detail.setSize(w, h)
}

func (m *devtoolsPopup) open(tabID, layer int) tea.Cmd {
	m.tabID, m.layer = tabID, layer
	m.typing = false
	m.detail = newDevDetailPopup()
	m.detail.setSize(m.screenW, m.screenH)
	m.tickGen++
	return tea.Batch(m.anim.open(), m.tick())
}

func (m *devtoolsPopup) close() tea.Cmd {
	m.tickGen++ // a tick from before does nothing
	return tea.Batch(m.detail.close(), m.anim.close())
}

func (m devtoolsPopup) tick() tea.Cmd {
	gen := m.tickGen
	return tea.Tick(devTickEvery, func(time.Time) tea.Msg { return devTickMsg{gen: gen} })
}

// innerW / rows are the box's inside: the screen less a margin, and the
// screen less the two borders and a margin.
func (m devtoolsPopup) innerW() int { return popupInnerW(m.screenW, m.screenW-4) }
func (m devtoolsPopup) rows() int   { return max(3, m.screenH-4) }

// listRows is how many entry rows the current tab has, after the filter
// line has taken its row.
func (m devtoolsPopup) listRows() int {
	if m.typing || m.filter[m.tab] != "" {
		return m.rows() - 1
	}
	return m.rows()
}

// refresh re-reads the tab's log into the Network and Console tabs. The
// Storage tab is fetched, not read (storageMsg).
func (m *devtoolsPopup) refresh(l *page.DevLog) {
	if l == nil {
		return
	}
	m.network.entries = l.Net()
	m.console.entries = l.Console()
}

// devAction is what a key did that the app must carry out.
type devAction int

const (
	devNone devAction = iota
	devFetchStorage
	devDeleteCookie
	devDeleteItem
	devYank // text in the string
	devClearSite
	devClearNet
	devClearConsole
	devDetail
)

// update handles one key. The tab sub-models get the navigation keys and
// answer the questions the shell asks; the shell decides.
func (m *devtoolsPopup) update(msg tea.KeyMsg) (devAction, string) {
	if !m.anim.isInteractive() {
		return devNone, ""
	}
	if m.detail.anim.owns() {
		m.detail.update(msg)
		return devNone, ""
	}
	k := msg.String()
	if m.typing {
		switch msg.Type {
		case tea.KeyEnter:
			m.typing = false
		case tea.KeyBackspace:
			if r := []rune(m.filter[m.tab]); len(r) > 0 {
				m.filter[m.tab] = string(r[:len(r)-1])
			}
		case tea.KeySpace:
			m.filter[m.tab] += " "
		case tea.KeyRunes:
			m.filter[m.tab] += string(msg.Runes)
		}
		return devNone, ""
	}
	switch k {
	case "h", "left":
		m.tab = (m.tab + 2) % 3
		if m.tab == devStorage {
			return devFetchStorage, ""
		}
		return devNone, ""
	case "l", "right":
		m.tab = (m.tab + 1) % 3
		if m.tab == devStorage {
			return devFetchStorage, ""
		}
		return devNone, ""
	case "/":
		m.typing = true
		return devNone, ""
	}
	if navKeys[k] && k != "h" && k != "l" && k != "left" && k != "right" {
		switch m.tab {
		case devStorage:
			m.storage.move(k, m.listRows(), m.filter[m.tab])
		case devNetwork:
			m.network.move(k, m.listRows(), m.filter[m.tab])
		case devConsole:
			m.console.move(k, m.listRows(), m.filter[m.tab])
		}
		return devNone, ""
	}
	switch m.tab {
	case devStorage:
		switch k {
		case "x":
			if r, ok := m.storage.current(m.filter[m.tab]); ok {
				if r.cookie != nil {
					return devDeleteCookie, ""
				}
				return devDeleteItem, ""
			}
		case "y":
			if r, ok := m.storage.current(m.filter[m.tab]); ok {
				return devYank, r.value
			}
		case "C":
			return devClearSite, ""
		}
	case devNetwork:
		switch k {
		case "enter":
			if _, ok := m.network.current(m.filter[m.tab]); ok {
				return devDetail, ""
			}
		case "C":
			return devClearNet, ""
		}
	case devConsole:
		if k == "C" {
			return devClearConsole, ""
		}
	}
	return devNone, ""
}

// escTyping is Esc while a filter is being typed: the filter goes, the
// popup stays.
func (m *devtoolsPopup) escTyping() bool {
	if !m.typing && m.filter[m.tab] == "" {
		return false
	}
	m.typing, m.filter[m.tab] = false, ""
	return true
}

func (m devtoolsPopup) view() string {
	innerW := m.innerW()
	bc := popupLayerColor(m.layer)
	bs := lipgloss.NewStyle().Foreground(bc)
	ts := lipgloss.NewStyle().Foreground(bc).Bold(true)
	dim := lipgloss.NewStyle().Foreground(dimColor)
	edit := lipgloss.NewStyle().Foreground(editColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(editColor)

	// The title row: glyph and name, then the chip chain of tabs (kbu
	// §8.2's starship chain, moved into a popup title).
	title := " " + glyphDevTools + " DevTools "
	chain := tabChain(devTabLabels, int(m.tab))
	titleW := dispW(title) + 1 + tabChainW(devTabLabels)
	var b strings.Builder
	b.WriteString(bs.Render("╭─") + ts.Render(title) + " " + chain +
		bs.Render(strings.Repeat("─", max(0, innerW-1-titleW))+"╮") + "\n")

	var rows []string
	if m.typing || m.filter[m.tab] != "" {
		line := " / " + m.filter[m.tab]
		if m.typing {
			rows = append(rows, edit.Render(padRight(line, innerW-1))+cur.Render(" "))
		} else {
			rows = append(rows, dim.Render(padRight(line, innerW)))
		}
	}
	n := m.listRows()
	switch m.tab {
	case devStorage:
		rows = append(rows, m.storage.view(innerW, n, m.filter[m.tab])...)
	case devNetwork:
		rows = append(rows, m.network.view(innerW, n, m.filter[m.tab])...)
	case devConsole:
		rows = append(rows, m.console.view(innerW, n, m.filter[m.tab])...)
	}
	for len(rows) < m.rows() {
		rows = append(rows, strings.Repeat(" ", innerW))
	}
	rows = animRows(m.anim, rows[:m.rows()])

	left, right := bs.Render("│"), bs.Render("│")
	for _, line := range rows {
		line = clipANSI(line, innerW)
		b.WriteString(left + line + strings.Repeat(" ", max(0, innerW-dispW(line))) + right + "\n")
	}

	var pairs [][2]string
	switch {
	case m.typing:
		pairs = [][2]string{{"Enter", "done"}, {"Esc", "clear"}}
	case m.tab == devStorage:
		pairs = [][2]string{{"x", "delete"}, {"y", "yank value"}, {"C", "clear site data"}, {"/", "filter"}, {"h/l", "tab"}, {"Esc", "close"}}
	case m.tab == devNetwork:
		pairs = [][2]string{{"Enter", "detail"}, {"C", "clear"}, {"/", "filter"}, {"h/l", "tab"}, {"Esc", "close"}}
	default:
		pairs = [][2]string{{"C", "clear"}, {"/", "filter"}, {"h/l", "tab"}, {"Esc", "close"}}
	}
	hint := clipANSI(hintLegend(pairs), innerW-1)
	b.WriteString(bs.Render("╰─") + hint + bs.Render(strings.Repeat("─", max(0, innerW-1-dispW(hint)))+"╯"))
	out := b.String()
	if m.detail.isActive() {
		out = m.detail.over(out)
	}
	return out
}

// matches is the one filter rule for every tab: a case-insensitive
// substring over the row's text.
func devMatches(filter, text string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(filter))
}
