package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/webu/internal/store"
)

// setting is one row of the Settings screen: a key of config.yaml (ui.md
// §6), read and written through funcs, so a row is nothing but a name, a
// description and the way its value is changed. A text setting edits in a
// box that opens with the value in force on offer, as the Location box
// offers the page's URL — Tab takes it to edit, Backspace clears it; Enter
// on the untouched offer changes nothing, Enter on an emptied line means
// the default. A switch flips on Enter (2026-09-21).
type setting struct {
	key string
	// desc says what the key does, on the row and in the box.
	desc string
	// get is the value as the row shows it; empty means the default.
	get func(store.Config) string
	// def is what an empty value means, for the row and the offer.
	def func(AppModel) string
	// set writes a text setting; toggle flips a switch. One of the two.
	set    func(*store.Config, string)
	toggle func(*store.Config)
}

var settings = []setting{
	{key: "download_dir", desc: "where downloads land",
		get: func(c store.Config) string { return c.DownloadDir },
		set: func(c *store.Config, v string) { c.DownloadDir = v },
		def: func(m AppModel) string { return m.downloadDir() }},
	{key: "restore_session", desc: "reopen the tabs that were open when webu last quit",
		get: func(c store.Config) string {
			if c.RestoreSession == nil {
				return ""
			}
			return onOff(*c.RestoreSession)
		},
		def: func(AppModel) string { return "on" },
		toggle: func(c *store.Config) {
			v := !c.Restore()
			c.RestoreSession = &v
		}},
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// settingEntries is the Settings screen's rows: the key, then the value
// in force — "(default)" when the file does not set it — and what the key
// does, so the screen reads as a settings page and not as a list of names.
func (m AppModel) settingEntries() []listEntry {
	out := make([]listEntry, 0, len(settings))
	for i, s := range settings {
		v := s.get(m.cfg)
		if v == "" {
			v = "(default) " + s.def(m)
		}
		out = append(out, listEntry{title: s.key, meta: v + "  " + s.desc, ref: i, toggle: s.toggle != nil})
	}
	return out
}

// changeSetting is Enter on a row: a switch flips and is saved at once, a
// text setting opens its box.
func (m *AppModel) changeSetting(ref int) tea.Cmd {
	if ref < 0 || ref >= len(settings) {
		return nil
	}
	s := settings[ref]
	if s.toggle != nil {
		s.toggle(&m.cfg)
		return m.saveConfig(s.key + " " + s.get(m.cfg))
	}
	return m.settingBox(ref)
}

// settingBox is the text box, the value in force on offer.
func (m *AppModel) settingBox(ref int) tea.Cmd {
	s := settings[ref]
	m.settingRef = ref
	cur := s.get(m.cfg)
	if cur == "" {
		cur = s.def(*m)
	}
	return m.input.ask(inputPopup{title: "Settings", glyph: glyphSettings,
		prompt:      s.key + " — " + s.desc + "; Backspace then Enter for the default",
		placeholder: cur, accept: "save", action: inputSetting}, m.layer())
}

// saveSetting is the box's answer.
func (m *AppModel) saveSetting(value string, untouched bool) tea.Cmd {
	if m.settingRef < 0 || m.settingRef >= len(settings) {
		return m.input.close()
	}
	if untouched {
		return m.input.close() // the offer was neither taken nor declined
	}
	s := settings[m.settingRef]
	s.set(&m.cfg, strings.TrimSpace(value))
	return tea.Batch(m.input.close(), m.saveConfig(s.key))
}

// saveConfig writes config.yaml, refreshes the rows, and tells the browser
// at once when it is the download dir that changed, so the next download
// lands in the new place.
func (m *AppModel) saveConfig(what string) tea.Cmd {
	if err := store.SaveConfig(m.cfg); err != nil {
		return m.toast.show("config.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.settingEntries())
	cmds := []tea.Cmd{m.toast.show(what+" — saved config.yaml", toastInfo)}
	if strings.HasPrefix(what, "download_dir") {
		cmds = append(cmds, m.pointDownloads())
	}
	return tea.Batch(cmds...)
}
