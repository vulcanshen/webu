package ir_test

import (
	"os"
	"testing"

	"github.com/vulcanshen/webu/internal/ir"
)

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
