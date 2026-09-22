package ui

import (
	"errors"
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
// the default. A switch flips on Enter.
//
// Every key config.yaml knows has a row here — the rule since 2026-09-21,
// and TestSettingsCoverConfig holds it: a key that can only be set by
// editing the file is a key most people never find.
type setting struct {
	key string
	// desc says what the key does, on the row and in the box.
	desc string
	// get is the value as the row shows it; empty means the default.
	get func(store.Config) string
	// def is what an empty value means, for the row and the offer.
	def func(AppModel) string
	// set writes a text setting, refusing what it cannot take; toggle
	// flips a switch. One of the two.
	set    func(*store.Config, string) error
	toggle func(*store.Config)
}

// settings is in config.yaml's order.
var settings = []setting{
	{key: "search_engine", desc: "where a search goes when what you typed is not a URL",
		get: func(c store.Config) string { return c.SearchEngine },
		set: func(c *store.Config, v string) error {
			if v != "" && !strings.Contains(v, "://") {
				return errors.New("a URL the words are appended to, like " + store.DefaultSearch)
			}
			c.SearchEngine = v
			return nil
		},
		def: func(AppModel) string { return store.DefaultSearch }},
	{key: "download_dir", desc: "where downloads land",
		get: func(c store.Config) string { return c.DownloadDir },
		set: func(c *store.Config, v string) error { c.DownloadDir = v; return nil },
		def: func(m AppModel) string { return m.downloadDir() }},
	{key: "measure", desc: "how wide a paragraph flows before it wraps: full, or a number of cells",
		get: func(c store.Config) string {
			if c.Measure <= store.MeasureFull {
				return ""
			}
			return c.Measure.String()
		},
		set: func(c *store.Config, v string) error {
			n, err := store.ParseMeasure(v)
			if err != nil {
				return err
			}
			c.Measure = n
			return nil
		},
		def: func(AppModel) string { return store.MeasureFull.String() }},
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

// saveSetting is the box's answer. A value the setting refuses keeps the
// box open with the reason, so what was typed is not lost.
func (m *AppModel) saveSetting(value string, untouched bool) tea.Cmd {
	if m.settingRef < 0 || m.settingRef >= len(settings) {
		return m.input.close()
	}
	if untouched {
		return m.input.close() // the offer was neither taken nor declined
	}
	s := settings[m.settingRef]
	if err := s.set(&m.cfg, strings.TrimSpace(value)); err != nil {
		return m.toast.show(s.key+": "+err.Error(), toastError)
	}
	return tea.Batch(m.input.close(), m.saveConfig(s.key))
}

// saveConfig writes config.yaml, refreshes the rows, and applies what
// takes effect at once: the browser is pointed at a new download dir, the
// pages are laid out again for a new measure.
func (m *AppModel) saveConfig(what string) tea.Cmd {
	if err := store.SaveConfig(m.cfg); err != nil {
		return m.toast.show("config.yaml: "+err.Error(), toastError)
	}
	m.lists.setEntries(m.settingEntries())
	cmds := []tea.Cmd{m.toast.show(what+" — saved config.yaml", toastInfo)}
	switch {
	case strings.HasPrefix(what, "download_dir"):
		cmds = append(cmds, m.pointDownloads())
	case strings.HasPrefix(what, "measure"):
		m.applyMeasure()
	}
	return tea.Batch(cmds...)
}

// applyMeasure gives every tab the measure now in force and lays it out
// again, cursor kept in view.
func (m *AppModel) applyMeasure() {
	for _, t := range m.tabs {
		t.measure = m.cfg.TextWidth()
		if t.root != nil {
			t.relayout(m.pageW())
			t.scrollToCursor(m.pageVisible())
		}
	}
}
