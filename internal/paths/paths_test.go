package paths

import (
	"path/filepath"
	"testing"
)

func TestDirsHonourOverrides(t *testing.T) {
	t.Setenv("WEBU_CONFIG", "/tmp/wc")
	t.Setenv("WEBU_CACHE", "/tmp/wk")
	t.Setenv("WEBU_DATA", "/tmp/wd")
	if d, _ := Config(); d != "/tmp/wc" {
		t.Errorf("Config %q", d)
	}
	if d, _ := Data(); d != "/tmp/wd" {
		t.Errorf("Data %q", d)
	}
	if d, _ := Downloads(); d != filepath.Join("/tmp/wd", "downloads") {
		t.Errorf("Downloads %q", d)
	}
	t.Setenv("WEBU_DATA", "")
	t.Setenv("HOME", "/tmp/h")
	if d, _ := Data(); d != filepath.Join("/tmp/h", ".webu", "datas") {
		t.Errorf("Data under HOME %q", d)
	}
	if d, _ := Cache(); d != "/tmp/wk" {
		t.Errorf("Cache %q", d)
	}
	t.Setenv("WEBU_CONFIG", "")
	t.Setenv("WEBU_CACHE", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/x")
	t.Setenv("XDG_CACHE_HOME", "/tmp/y")
	if d, _ := Config(); d != filepath.Join("/tmp/x", "webu") {
		t.Errorf("Config under XDG %q", d)
	}
	if d, _ := Cache(); d != filepath.Join("/tmp/y", "webu") {
		t.Errorf("Cache under XDG %q", d)
	}
}
