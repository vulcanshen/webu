package paths

import (
	"path/filepath"
	"testing"
)

func TestDirsHonourOverrides(t *testing.T) {
	t.Setenv("WEBU__CONFIG", "/tmp/wc")
	t.Setenv("WEBU__CACHE", "/tmp/wk")
	t.Setenv("WEBU__DATA", "/tmp/wd")
	if d, _ := Config(); d != "/tmp/wc" {
		t.Errorf("Config %q", d)
	}
	if d, _ := Data(); d != "/tmp/wd" {
		t.Errorf("Data %q", d)
	}
	if d, _ := Downloads(); d != filepath.Join("/tmp/wd", "downloads") {
		t.Errorf("Downloads %q", d)
	}
	t.Setenv("WEBU__DATA", "")
	t.Setenv("HOME", "/tmp/h")
	if d, _ := Data(); d != filepath.Join("/tmp/h", ".webu", "datas") {
		t.Errorf("Data under HOME %q", d)
	}
	t.Setenv("WEBU__CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if d, _ := Config(); d != filepath.Join("/tmp/h", ".config", "webu") {
		t.Errorf("Config under HOME %q", d)
	}
	if d, _ := Cache(); d != "/tmp/wk" {
		t.Errorf("Cache %q", d)
	}
	t.Setenv("WEBU__CONFIG", "")
	t.Setenv("WEBU__CACHE", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/x")
	t.Setenv("XDG_CACHE_HOME", "/tmp/y")
	if d, _ := Config(); d != filepath.Join("/tmp/x", "webu") {
		t.Errorf("Config under XDG %q", d)
	}
	if d, _ := Cache(); d != filepath.Join("/tmp/y", "webu") {
		t.Errorf("Cache under XDG %q", d)
	}
}

// The family names its variables APP__NAME and keeps no old name (tdp D6
// v0.1.21): WEBU_CONFIG, WEBU_DATA and WEBU_CACHE are not read.
func TestOldNamesAreNotRead(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/x")
	t.Setenv("XDG_CACHE_HOME", "/tmp/y")
	for _, v := range []string{"WEBU__CONFIG", "WEBU__DATA", "WEBU__CACHE"} {
		t.Setenv(v, "")
	}
	t.Setenv("WEBU_CONFIG", "/tmp/old")
	t.Setenv("WEBU_DATA", "/tmp/old")
	t.Setenv("WEBU_CACHE", "/tmp/old")
	if d, _ := Config(); d != filepath.Join("/tmp/x", "webu") {
		t.Errorf("Config %q", d)
	}
	if d, _ := Data(); d != filepath.Join("/tmp/h", ".webu", "datas") {
		t.Errorf("Data %q", d)
	}
	if d, _ := Cache(); d != filepath.Join("/tmp/y", "webu") {
		t.Errorf("Cache %q", d)
	}
}
