package ir_test

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/vulcanshen/webu/internal/ir"
)

// axNode is one AX node as Chromium reports it, for the tests that do not
// go through a browser. The backend id is the node id, so a Capture's
// per-node marks can name it.
func axNode(id int, role, name string, backend int, kids ...int) *accessibility.Node {
	val := func(s string) *accessibility.Value {
		raw, _ := json.Marshal(s)
		return &accessibility.Value{Type: accessibility.ValueTypeString, Value: raw}
	}
	n := &accessibility.Node{
		NodeID:           accessibility.NodeID(strconv.Itoa(id)),
		Role:             val(role),
		Name:             val(name),
		BackendDOMNodeID: cdp.BackendNodeID(backend),
	}
	for _, k := range kids {
		n.ChildIDs = append(n.ChildIDs, accessibility.NodeID(strconv.Itoa(k)))
	}
	return n
}

// docs/support.md is generated from the Roles table, so the document and the
// code cannot drift (function.md §3). The test fails when they have; -update
// rewrites the file.
const supportDoc = "../../docs/support.md"

func TestSupportDoc(t *testing.T) {
	got := ir.SupportDoc()
	if *update {
		if err := os.WriteFile(supportDoc, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(supportDoc)
	if err != nil {
		t.Fatalf("%s missing: run with -update", supportDoc)
	}
	if string(want) != got {
		t.Errorf("%s is out of date: run `go test ./internal/ir -run TestSupportDoc -update`", supportDoc)
	}
}

// A page writes for a screen reader alone by laying text out at a point:
// position:absolute, width:1px, height:1px, clipped. A reader meets it one
// line at a time and needs the repetition; webu puts the whole card on
// screen at once, where the same text three times is noise. The nodes are
// dropped — and nothing is lost, because Chromium has already folded that
// text into the accessible names it computes (Jira's backlog, 2026-09-22:
// every duplicate measured exactly 1×1).
func TestHiddenTextIsDropped(t *testing.T) {
	c := ir.Capture{
		Nodes: []*accessibility.Node{
			axNode(1, "RootWebArea", "Page", 1, 2, 3),
			axNode(2, "StaticText", "For a screen reader only:", 2),
			axNode(3, "StaticText", "The sentence you can see.", 3),
		},
		Hidden: map[cdp.BackendNodeID]bool{2: true},
	}
	got := ir.Dump(ir.Build(c))
	if strings.Contains(got, "screen reader only") {
		t.Errorf("a node laid out at a point should not be drawn:\n%s", got)
	}
	if !strings.Contains(got, "you can see") {
		t.Errorf("the visible run should survive:\n%s", got)
	}
	// A node with no entry is unmeasured, not hidden: absent data is not
	// evidence.
	c.Hidden = nil
	if !strings.Contains(ir.Dump(ir.Build(c)), "screen reader only") {
		t.Error("with no bounds at all nothing is hidden")
	}
}
