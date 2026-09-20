package main

import (
	"context"
	"encoding/json"
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
	url := os.Args[1]
	limit := 60
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
		if !n.Ignored && !skip[val(n.Role)] {
			semantic++
			if printed < limit {
				fmt.Printf("%*s%s %q\n", depth*2, "", val(n.Role), val(n.Name))
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
