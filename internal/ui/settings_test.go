package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vulcanshen/webu/internal/store"
)

// Every key of config.yaml is a row of the Settings screen (2026-09-21):
// a key that can only be set by editing the file is one most people never
// find. The struct's yaml tags are the list of keys.
func TestSettingsCoverConfig(t *testing.T) {
	rows := map[string]bool{}
	for _, s := range settings {
		rows[s.key] = true
	}
	ct := reflect.TypeOf(store.Config{})
	for i := 0; i < ct.NumField(); i++ {
		key := strings.Split(ct.Field(i).Tag.Get("yaml"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		if !rows[key] {
			t.Errorf("config.yaml key %q has no row on the Settings screen", key)
		}
	}
	for _, s := range settings {
		if (s.set == nil) == (s.toggle == nil) {
			t.Errorf("setting %q must be a text setting or a switch, not both or neither", s.key)
		}
	}
}
