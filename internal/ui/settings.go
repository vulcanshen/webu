package ui

import (
	"errors"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"github.com/vulcanshen/webu/internal/store"
)

// The Settings screen (ui.md §6): one value per row (layout/panel), a key
// of config.yaml each. A value takes effect, and is saved, the moment it
// is set: a text setting in its input popup — the value in force on
// offer, Backspace then Enter for the default — a switch in place.
//
// Every key config.yaml knows has a row here — the rule since 2026-09-21,
// and TestSettingsCoverConfig holds it: a key that can only be set by
// editing the file is a key most people never find.

// setting is one row.
type setting struct {
	key string
	// desc says what the key does, in its input popup or beside its switch.
	desc string
	// get is the value as the row shows it; empty means the default.
	get func(store.Config) string
	// def is what an empty value means.
	def func(*AppModel) string
	// set writes a text setting, refusing what it cannot take; a number
	// setting takes the range of number. on and flip make a switch.
	set    func(*store.Config, string) error
	number [2]float64
	on     func(store.Config) bool
	flip   func(*store.Config)
}

// settings is in config.yaml's order.
var settings = []setting{
	{key: "search_engine", desc: "Where a search goes when what you typed is not a URL",
		get: func(c store.Config) string { return c.SearchEngine },
		set: func(c *store.Config, v string) error {
			if v != "" && !strings.Contains(v, "://") {
				return errors.New("The search engine is a URL the words are appended to, like " + store.DefaultSearch)
			}
			c.SearchEngine = v
			return nil
		},
		def: func(*AppModel) string { return store.DefaultSearch }},
	{key: "download_dir", desc: "Where downloads land",
		get: func(c store.Config) string { return c.DownloadDir },
		set: func(c *store.Config, v string) error { c.DownloadDir = v; return nil },
		def: func(m *AppModel) string { return m.downloadDir() }},
	{key: "measure", desc: "How wide a paragraph flows before it wraps: full, or a number of cells",
		get: func(c store.Config) string {
			if c.Measure <= store.MeasureFull {
				return ""
			}
			return c.Measure.String()
		},
		set: func(c *store.Config, v string) error {
			n, err := store.ParseMeasure(v)
			if err != nil {
				return errors.New("The measure " + strings.TrimPrefix(err.Error(), "measure "))
			}
			c.Measure = n
			return nil
		},
		def: func(*AppModel) string { return store.MeasureFull.String() }},
	{key: "restore_session", desc: "reopen the tabs that were open when webu last quit",
		on: func(c store.Config) bool { return c.Restore() },
		flip: func(c *store.Config) {
			v := !c.Restore()
			c.RestoreSession = &v
		}},
	{key: "hide_secrets_in_preview", desc: "hide private keys and owner-only files in the file preview",
		on:   func(c store.Config) bool { return c.HideSecretsInPreview },
		flip: func(c *store.Config) { c.HideSecretsInPreview = !c.HideSecretsInPreview }},
	{key: "input_tab_width", desc: "How many spaces Tab puts in a text of several lines",
		get: func(c store.Config) string {
			if c.InputTabWidth <= 0 {
				return ""
			}
			return strconv.Itoa(c.InputTabWidth)
		},
		number: [2]float64{1, 16},
		set: func(c *store.Config, v string) error {
			if v == "" {
				c.InputTabWidth = 0
				return nil
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			c.InputTabWidth = n
			return nil
		},
		def: func(*AppModel) string { return "4" }},
}

// buildSettings registers the Settings screen.
func (m *AppModel) buildSettings() {
	m.setValues = &terminu.Values{Header: [2]string{"Setting", "Value"}}
	for i := range settings {
		m.setValues.Fields = append(m.setValues.Fields, m.settingField(i))
	}
	m.setPanel = &terminu.Panel{Label: "Settings", Body: m.setValues}
	m.setScreen = m.a.AddScreen(&terminu.Screen{Name: "Settings", Key: "S", Desc: "config.yaml, a key a row",
		Panels: []*terminu.Panel{m.setPanel}})
}

// settingField is one setting's row.
func (m *AppModel) settingField(i int) *terminu.Field {
	s := settings[i]
	f := &terminu.Field{Label: s.key}
	if s.flip != nil {
		f.Options, f.Multi, f.Checked = []string{s.desc}, true, []bool{false}
		f.OnChange = m.do(func() tea.Cmd {
			if s.on(m.cfg) != f.Checked[0] {
				s.flip(&m.cfg)
			}
			return m.saveConfig(s.key)
		})
		return f
	}
	f.Value = func() string { return s.get(m.cfg) }
	f.Empty = "default: " + s.def(m)
	f.Edit = m.do(func() tea.Cmd { return m.settingBox(i) })
	return f
}

// refreshSettings puts the switches where config.yaml has them, and the
// defaults that depend on it.
func (m *AppModel) refreshSettings() {
	if m.setValues == nil {
		return
	}
	for i, s := range settings {
		f := m.setValues.Fields[i]
		if s.flip != nil {
			f.Checked = []bool{s.on(m.cfg)}
		} else {
			f.Empty = "default: " + s.def(m)
		}
	}
}

// settingBox is a text setting's input popup, the value in force on offer.
func (m *AppModel) settingBox(i int) tea.Cmd {
	s := settings[i]
	cur := s.get(m.cfg)
	if cur == "" {
		cur = s.def(m)
	}
	done := m.cmdDone(func(v string) tea.Cmd {
		if err := s.set(&m.cfg, strings.TrimSpace(v)); err != nil {
			return m.a.Error("Settings", err.Error())
		}
		return m.saveConfig(s.key)
	})
	check := func(v string) error {
		c := m.cfg
		return s.set(&c, strings.TrimSpace(v))
	}
	prompt := s.desc + "; [Backspace] then [Enter] for the default"
	if s.number[1] > 0 {
		return m.a.Number(terminu.NumberInput{Title: "Settings", Subject: s.key, Glyph: glyphSettings, Name: s.key,
			Prompt: prompt, Verb: "save", Old: cur, Min: s.number[0], Max: s.number[1], Check: check, Done: done})
	}
	return m.a.Text(terminu.TextInput{Title: "Settings", Subject: s.key, Glyph: glyphSettings, Name: s.key,
		Prompt: prompt, Verb: "save", Old: cur, Check: check, Done: done})
}

// saveConfig writes config.yaml and applies what takes effect at once: the
// browser is pointed at a new download dir, the pages are laid out again
// for a new measure, the lib is told the family's keys.
func (m *AppModel) saveConfig(key string) tea.Cmd {
	m.applyConfig()
	m.refreshSettings()
	if err := store.SaveConfig(m.cfg); err != nil {
		return m.toastErr("config.yaml: " + err.Error())
	}
	cmds := []tea.Cmd{m.toast(key + " saved to config.yaml")}
	switch key {
	case "download_dir":
		cmds = append(cmds, m.pointDownloads())
	case "measure":
		m.applyMeasure()
	}
	return tea.Batch(cmds...)
}

// applyConfig hands the lib the family's keys (tdp E3).
func (m *AppModel) applyConfig() {
	m.a.HideSecretsInPreview = m.cfg.HideSecretsInPreview
	m.a.InputTabWidth = m.cfg.TabWidth()
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
