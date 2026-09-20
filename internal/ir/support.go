package ir

import (
	"fmt"
	"sort"
	"strings"
)

// SupportDoc renders docs/support.md from the Roles table, so the document
// and the code cannot drift (function.md §3). TestSupportDoc keeps the file
// in step.
func SupportDoc() string {
	var supported, invisible []string
	for role, s := range Roles {
		if s.Transparent || s.Skip {
			invisible = append(invisible, role)
		} else {
			supported = append(supported, role)
		}
	}
	sort.Strings(supported)
	sort.Strings(invisible)

	var b strings.Builder
	b.WriteString("# webu — supported roles\n\n")
	b.WriteString("Generated from `internal/ir/roles.go` by `go test ./internal/ir -run TestSupportDoc -update`. Do not edit by hand.\n\n")
	b.WriteString("webu guarantees its translation per AX role, not per site (function.md §3). A role listed here is drawn and acted on as described; a role that is not falls back: its name is drawn as text, its children are kept, an unsupported glyph marks it, and Enter still clicks it.\n\n")
	b.WriteString("## Drawn\n\n| AX role | IR kind | Enter | Display |\n|---|---|---|---|\n")
	for _, role := range supported {
		s := Roles[role]
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", role, s.Kind, s.Action, s.Display)
	}
	b.WriteString("\n## Invisible\n\nThese roles carry nothing a terminal can use; the node vanishes and its children flow into the parent. A container the page laid out as a block still keeps its line break.\n\n| AX role | Why |\n|---|---|\n")
	for _, role := range invisible {
		fmt.Fprintf(&b, "| `%s` | %s |\n", role, Roles[role].Display)
	}
	fmt.Fprintf(&b, "\n## Everything else\n\n%s.\n", Fallback.Display)
	return b.String()
}
