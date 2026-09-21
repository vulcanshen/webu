package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// filePicker is the menu class with a filter row — filu's finder form,
// the picker sshu opens for an identity file — over one directory at a
// time: its folders first, then its files; type to narrow, the arrows to
// move, Enter to take. Enter on a folder steps into it, Backspace on an
// empty filter steps out to the parent. It exists so a file is PICKED
// rather than typed: a mistyped path fails far from where the mistake
// was made.
//
// It is not modal. Letters always filter and the arrows always move, so
// there is no "input state" versus "list state" to learn (§4.5): in a
// text-entry surface, letters type and arrows navigate.
type filePicker struct {
	anim    popupAnimator
	glyph   string
	title   string
	dir     string
	entries []pickerEntry
	matches []int // indices into entries, best first
	query   string
	cursor  int
	top     int
	note    string // why the list is empty — never a silent blank
	layer   int
	screenW int
	screenH int
}

type pickerEntry struct {
	path  string // absolute
	label string // the name, what the user reads
	dir   bool
	size  int64
}

func newFilePicker() filePicker { return filePicker{anim: newPopupAnimator("picker")} }

func (m filePicker) isActive() bool      { return m.anim.isActive() }
func (m filePicker) isInteractive() bool { return m.anim.isInteractive() }
func (m *filePicker) close() tea.Cmd     { return m.anim.close() }
func (m *filePicker) setSize(w, h int)   { m.screenW, m.screenH = w, h }

// open shows dir under a title of the caller's.
func (m *filePicker) open(glyph, title, dir string, layer int) tea.Cmd {
	m.glyph, m.title, m.layer = glyph, title, layer
	m.enter(dir)
	return m.anim.open()
}

// enter lists dir: folders first, then regular files, both by name and
// case aside; dot files left out. A symlink counts as what it points at.
func (m *filePicker) enter(dir string) {
	m.dir = dir
	m.query, m.cursor, m.top, m.note = "", 0, 0, ""
	m.entries = nil
	items, err := os.ReadDir(dir)
	if err != nil {
		m.note = "cannot read this folder"
	}
	for _, it := range items {
		name := it.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		e := pickerEntry{path: p, label: name, dir: info.IsDir()}
		if !e.dir {
			if !info.Mode().IsRegular() {
				continue
			}
			e.size = info.Size()
		}
		m.entries = append(m.entries, e)
	}
	sort.SliceStable(m.entries, func(a, b int) bool {
		if m.entries[a].dir != m.entries[b].dir {
			return m.entries[a].dir
		}
		return strings.ToLower(m.entries[a].label) < strings.ToLower(m.entries[b].label)
	})
	if m.note == "" && len(m.entries) == 0 {
		m.note = "nothing here"
	}
	m.refilter()
}

// refilter is the list for the query: everything, in directory order,
// when there is none; else the fuzzy matches, best first.
func (m *filePicker) refilter() {
	m.matches = m.matches[:0]
	m.cursor, m.top = 0, 0
	if m.query == "" {
		for i := range m.entries {
			m.matches = append(m.matches, i)
		}
		return
	}
	type scored struct{ idx, score int }
	var hits []scored
	for i, e := range m.entries {
		if s, ok := fuzzyScore(e.label, m.query); ok {
			hits = append(hits, scored{i, s})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score > hits[b].score
		}
		return m.entries[hits[a].idx].label < m.entries[hits[b].idx].label
	})
	for _, h := range hits {
		m.matches = append(m.matches, h.idx)
	}
}

// fuzzyScore matches q as a case-insensitive subsequence of s. Runs of
// adjacent characters and matches at a word boundary score high, so
// typing "bkm" puts bookmarks_9_21.html above something that merely
// happens to contain those letters.
func fuzzyScore(s, q string) (int, bool) {
	if q == "" {
		return 0, true
	}
	rs := []rune(strings.ToLower(s))
	rq := []rune(strings.ToLower(q))

	score, at, prev := 0, 0, -2
	for _, want := range rq {
		found := -1
		for i := at; i < len(rs); i++ {
			if rs[i] == want {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, false
		}
		if found == prev+1 {
			score += 8 // adjacent to the previous hit
		} else {
			score++
		}
		if found == 0 || strings.ContainsRune("/_-. ", rs[found-1]) {
			score += 4 // start of the name or of a word in it
		}
		prev, at = found, found+1
	}
	return score - len(rs)/8, true // shorter names win ties
}

// update takes a key. It reports the file picked, once one is.
func (m *filePicker) update(msg tea.KeyMsg) (picked string, done bool) {
	if !m.anim.isInteractive() {
		return "", false
	}
	switch msg.Type {
	case tea.KeyUp:
		m.cursor = moveCursor(m.cursor, len(m.matches), "k", m.visible())
	case tea.KeyDown:
		m.cursor = moveCursor(m.cursor, len(m.matches), "j", m.visible())
	case tea.KeyEnter:
		if m.cursor < len(m.matches) {
			e := m.entries[m.matches[m.cursor]]
			if !e.dir {
				return e.path, true
			}
			m.enter(e.path)
		}
	case tea.KeyBackspace:
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.refilter()
		} else if parent := filepath.Dir(m.dir); parent != m.dir {
			// Out to the parent, the cursor on the folder just left.
			from := m.dir
			m.enter(parent)
			for i, e := range m.entries {
				if e.path == from {
					m.cursor = i
				}
			}
		}
	case tea.KeySpace:
		m.query += " "
		m.refilter()
	case tea.KeyRunes:
		m.query += string(msg.Runes)
		m.refilter()
	}
	m.scroll()
	return "", false
}

func (m *filePicker) scroll() {
	vis := m.visible()
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+vis {
		m.top = m.cursor - vis + 1
	}
	m.top = max(0, m.top)
}

// visible is how many rows fit: the box costs its borders, and the query
// row and its divider come out of the content budget.
func (m filePicker) visible() int { return max(1, m.screenH-9) }

func (m filePicker) view() string {
	innerW := popupInnerW(m.screenW, 60)

	dim := lipgloss.NewStyle().Foreground(dimColor)
	txt := lipgloss.NewStyle().Foreground(textColor)
	hand := lipgloss.NewStyle().Foreground(handColor)
	cur := lipgloss.NewStyle().Foreground(lipgloss.Color(baseHex)).Background(handColor)

	// Query row, with the caret parked at the end.
	q := m.query + " "
	rows := []string{
		hand.Render(" "+glyphSearch+" ") + txt.Render(m.query) +
			cur.Render(" ") + strings.Repeat(" ", max(0, innerW-4-dispW(q))),
		dim.Render(strings.Repeat("─", innerW)),
	}
	switch {
	case m.note != "" && len(m.matches) == 0:
		rows = append(rows, dim.Render(padRight("  "+m.note, innerW)))
	case len(m.matches) == 0:
		rows = append(rows, dim.Render(padRight("  no match", innerW)))
	}

	// The size, right-aligned; the name takes the rest.
	const sizeW = 9
	nameW := innerW - sizeW - 3
	end := min(len(m.matches), m.top+m.visible())
	for i := m.top; i < end; i++ {
		e := m.entries[m.matches[i]]
		name, meta := e.label, humanBytes(float64(e.size))
		if e.dir {
			name, meta = glyphFolder+" "+e.label+"/", ""
		}
		name = padRight(truncate(name, nameW), nameW)
		meta = padLeft(meta, sizeW)
		if i == m.cursor {
			rows = append(rows, cur.Render(" "+name+" "+meta+" "))
			continue
		}
		rows = append(rows, " "+txt.Render(name)+" "+dim.Render(meta)+" ")
	}

	title := " " + m.glyph + " " + m.title + "  " + foldHome(m.dir) + " "
	hint := hintLegend([][2]string{{"↑↓", "select"}, {"Enter", "open / pick"}, {"Bksp", "up"}, {"Esc", "cancel"}})
	return drawPopupBoxPad(popupLayerColor(m.layer), title, hint,
		animRows(m.anim, capRows(rows, m.screenH)), innerW, false)
}

// foldHome writes a path the way the user does, ~ for the home directory.
func foldHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home || strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
