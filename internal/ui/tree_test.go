package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/webu/internal/browser"
	"github.com/vulcanshen/webu/internal/ir"
)

// A tree draws the way the Bookmarks folder tree does: indented by
// level, a triangle for a branch, the name once. Enter on a branch
// opens it and its items appear under it, deeper; Enter on a leaf picks
// it (user, 2026-09-23).
func TestATreeIsATree(t *testing.T) {
	t.Setenv("WEBU_CONFIG", t.TempDir())
	t.Setenv("WEBU_DATA", t.TempDir())
	exe, ok := browser.Installed()
	if !ok {
		t.Skip("pinned Chromium not installed; run webu once")
	}
	b, err := browser.Launch(exe, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	abs, _ := filepath.Abs("testdata/tree.html")
	d := newDriver(t, New(b, "file://"+abs))
	defer d.m.Close()
	d.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.until("the tree page", d.loaded("Tree"))
	p := d.page()
	v := dumpLayout(p.lay)
	for _, want := range []string{"▸ Projects", "▸ Reports", "  Letters"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "treeitem") || strings.Contains(v, "Projects Projects") {
		t.Errorf("an item is its name, once, not its role:\n%s", v)
	}
	d.cursorOn(ir.Group, "Projects")
	d.key("enter")
	d.until("opened", func() bool { return strings.Contains(dumpLayout(p.lay), "▾ Projects") })
	v = dumpLayout(p.lay)
	if !strings.Contains(v, "\n  ▸") && !strings.Contains(v, "\n    project-1.docx") {
		// The items under it, one level deeper.
		if !strings.Contains(v, "  project-1.docx") {
			t.Errorf("the branch's items are under it, indented:\n%s", v)
		}
	}
	// The leaf by its own name: the branch's text holds every leaf's.
	found := false
	for i, it := range p.lay.items {
		if it.node.Role == "treeitem" && strings.TrimSpace(it.node.Name) == "project-2.docx" {
			p.cursor, found = i, true
		}
	}
	if !found {
		t.Fatalf("the leaf is an item to stop on:\n%s", dumpLayout(p.lay))
	}
	d.key("enter")
	d.until("picked", func() bool { return strings.Contains(dumpLayout(p.lay), "Selected: project-2.docx") })
}
