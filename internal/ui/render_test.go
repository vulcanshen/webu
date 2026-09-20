package ui

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vulcanshen/webu/internal/ir"
)

var updateRender = flag.Bool("update", false, "rewrite testdata/*.render from the current renderer")

// TestRenderFixtures draws every IR fixture at a fixed width and checks the
// plain text against a golden. The goldens are the page as a user would see
// it, minus colour — the fastest way to review a layout change is to read
// the diff.
func TestRenderFixtures(t *testing.T) {
	jsons, _ := filepath.Glob("../ir/testdata/*.json")
	if len(jsons) == 0 {
		t.Fatal("no IR fixtures")
	}
	sort.Strings(jsons)
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range jsons {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var c ir.Capture
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			got := dumpLayout(render(ir.Build(c), 60))
			golden := filepath.Join("testdata", name+".render")
			if *updateRender {
				os.WriteFile(golden, []byte(got), 0o644)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("no golden: run with -update")
			}
			if string(want) != got {
				t.Errorf("render differs\n--- want\n%s--- got\n%s", want, got)
			}
			for i, r := range render(ir.Build(c), 60).rows {
				if w := dispW(r.plain()); w > 60 {
					t.Errorf("row %d is %d cells wide: %q", i, w, r.plain())
				}
			}
		})
	}
}

func dumpLayout(l layout) string {
	var b strings.Builder
	for _, r := range l.rows {
		b.WriteString(r.plain())
		b.WriteString("\n")
	}
	b.WriteString("-- items --\n")
	for i, it := range l.items {
		fmt.Fprintf(&b, "%d %s %q rows %d-%d\n", i, it.node.Kind, oneLine(it.node.Text()), it.first, it.last)
	}
	return b.String()
}

func TestWrapKeepsItemSpans(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{{
		Kind: ir.Paragraph, Children: []*ir.Node{
			{Kind: ir.Text, Name: "before "},
			{Kind: ir.Link, Name: "a link whose text is long enough to wrap onto the next row", URL: "u"},
			{Kind: ir.Text, Name: " after"},
		},
	}}}
	l := render(root, 30)
	if len(l.items) != 1 {
		t.Fatalf("items %d", len(l.items))
	}
	it := l.items[0]
	if it.first != 0 || it.last < 1 {
		t.Errorf("link should span from row 0 onto a later row, got %d-%d", it.first, it.last)
	}
	if got := l.itemAt(it.last); got != 0 {
		t.Errorf("itemAt(last) = %d", got)
	}
	for i, r := range l.rows {
		if w := dispW(r.plain()); w > 30 {
			t.Errorf("row %d overflows: %d", i, w)
		}
	}
}

func TestLongWordIsSplit(t *testing.T) {
	root := &ir.Node{Kind: ir.Document, Children: []*ir.Node{{
		Kind: ir.Paragraph, Children: []*ir.Node{{Kind: ir.Text, Name: strings.Repeat("x", 25)}},
	}}}
	l := render(root, 10)
	if len(l.rows) != 3 {
		t.Fatalf("rows %d: %q", len(l.rows), dumpLayout(l))
	}
	for _, r := range l.rows {
		if dispW(r.plain()) > 10 {
			t.Errorf("overflow: %q", r.plain())
		}
	}
}
