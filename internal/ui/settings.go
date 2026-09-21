package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/store"
)

// setting is one row of the Settings screen: a key of config.yaml (ui.md
// §6), read and written through two funcs, so a row is nothing but a name
// and a text box. Every text setting edits the same way (2026-09-21): the
// box opens with the value in force on offer, as the Location box offers
// the page's URL — Tab takes it to edit, Backspace clears it; Enter on the
// untouched offer changes nothing, Enter on an emptied line means the
// default.
type setting struct {
	key, prompt string
	get         func(store.Config) string
	set         func(*store.Config, string)
	// def is what an empty value means, for the row and the offer.
	def func(AppModel) string
}

var settings = []setting{
	{key: "download_dir", prompt: "where downloads land",
		get: func(c store.Config) string { return c.DownloadDir },
		set: func(c *store.Config, v string) { c.DownloadDir = v },
		def: func(m AppModel) string { return m.downloadDir() }},
}

// settingEntries is the Settings screen's rows.
func (m AppModel) settingEntries() []listEntry {
	out := make([]listEntry, 0, len(settings))
	for i, s := range settings {
		v := s.get(m.cfg)
		if v == "" {
			v = "(default) " + s.def(m)
		}
		out = append(out, listEntry{title: s.key, meta: v, ref: i})
	}
	return out
}

// settingBox is Enter on a row: the text box, the value in force on offer.
func (m *AppModel) settingBox(ref int) tea.Cmd {
	if ref < 0 || ref >= len(settings) {
		return nil
	}
	s := settings[ref]
	m.settingRef = ref
	cur := s.get(m.cfg)
	if cur == "" {
		cur = s.def(*m)
	}
	return m.input.ask(inputPopup{title: "Settings", glyph: glyphSettings,
		prompt:      s.key + " — " + s.prompt + "; Backspace then Enter for the default",
		placeholder: cur, accept: "save", action: inputSetting}, m.layer())
}

// saveSetting is the box's answer. The browser is told at once when it is
// the download dir, so the next download lands in the new place.
func (m *AppModel) saveSetting(value string, untouched bool) tea.Cmd {
	if m.settingRef < 0 || m.settingRef >= len(settings) {
		return m.input.close()
	}
	if untouched {
		return m.input.close() // the offer was neither taken nor declined
	}
	s := settings[m.settingRef]
	s.set(&m.cfg, strings.TrimSpace(value))
	if err := store.SaveConfig(m.cfg); err != nil {
		return tea.Batch(m.input.close(), m.toast.show("config.yaml: "+err.Error(), toastError))
	}
	m.lists.setEntries(m.settingEntries())
	cmds := []tea.Cmd{m.input.close(), m.toast.show("saved config.yaml", toastInfo)}
	if s.key == "download_dir" {
		cmds = append(cmds, m.pointDownloads())
	}
	return tea.Batch(cmds...)
}
