// Command axdump prints the accessibility tree Chromium builds for a URL —
// the input webu's translation layer works from (docs/function.md §3).
//
//	go run . <url>            outline: role "name", generic/text nodes skipped
//	go run . -all <url>       outline with every node, ignored ones marked
//	go run . -json <url>      the raw Accessibility.getFullAXTree result
//
// It uses whatever Chrome chromedp finds on this machine, not webu's pinned
// one; for a fixture, use webu's own capture.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

func val(v *accessibility.Value) string {
	if v == nil || v.Value == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err != nil {
		return string(v.Value)
	}
	return s
}

func main() {
	all := flag.Bool("all", false, "print every node, including generic/text and ignored")
	asJSON := flag.Bool("json", false, "print the raw node list as JSON")
	limit := flag.Int("n", 60, "outline: max nodes to print")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: axdump [-all] [-json] [-n N] <url>")
		os.Exit(2)
	}
	url := flag.Arg(0)
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var nodes []*accessibility.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			nodes, err = accessibility.GetFullAXTree().Do(ctx)
			return err
		}),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", " ")
		enc.Encode(nodes)
		return
	}

	byID := map[accessibility.NodeID]*accessibility.Node{}
	for _, n := range nodes {
		byID[n.NodeID] = n
	}
	skip := map[string]bool{"generic": true, "none": true, "StaticText": true, "InlineTextBox": true}
	printed, semantic := 0, 0
	var walk func(id accessibility.NodeID, depth int)
	walk = func(id accessibility.NodeID, depth int) {
		n := byID[id]
		if n == nil {
			return
		}
		d := depth
		role := val(n.Role)
		show := *all || (!n.Ignored && !skip[role])
		if !n.Ignored && !skip[role] {
			semantic++
		}
		if show {
			if printed < *limit || *all {
				mark := ""
				if n.Ignored {
					mark = " (ignored)"
				}
				extra := ""
				if v := val(n.Value); v != "" {
					extra += fmt.Sprintf(" value=%q", v)
				}
				for _, p := range n.Properties {
					extra += fmt.Sprintf(" %s=%s", p.Name, string(p.Value.Value))
				}
				fmt.Printf("%*s%s %q%s%s  #%d\n", depth*2, "", role, val(n.Name), extra, mark, n.BackendDOMNodeID)
				printed++
			}
			d = depth + 1
		}
		for _, c := range n.ChildIDs {
			walk(c, d)
		}
	}
	walk(nodes[0].NodeID, 0)
	fmt.Printf("\n[raw AX nodes: %d] [semantic after skipping generic/text: %d] [printed: %d]\n", len(nodes), semantic, printed)
}
