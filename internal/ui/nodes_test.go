package ui

import "github.com/vulcanshen/webu/internal/ir"

// The tests' small pages, built as the reader builds its content tree.

func doc(kids ...*ir.Node) *ir.Node { return &ir.Node{Kind: ir.Document, Children: kids} }

func hd(level int, s string) *ir.Node {
	return &ir.Node{Kind: ir.Heading, Level: level, Children: []*ir.Node{text(s)}}
}

func link(name, url string) *ir.Node { return &ir.Node{Kind: ir.Link, Name: name, URL: url} }

func para(children ...*ir.Node) *ir.Node { return &ir.Node{Kind: ir.Paragraph, Children: children} }
func text(s string) *ir.Node             { return &ir.Node{Kind: ir.Text, Name: s} }
