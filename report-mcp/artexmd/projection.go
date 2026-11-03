package artexmd

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	ea "github.com/yuin/goldmark/extension/ast"
)

// Projection is a report-specific meaning contract, not a serialized parser AST.
// Each output is checked independently against ARTEX's input-derived projection.
type Projection struct {
	Slots []SlotProjection `json:"slots"`
}
type SlotProjection struct {
	ID     string  `json:"id"`
	Locked bool    `json:"locked"`
	Blocks []Block `json:"blocks"`
}
type Inline struct {
	Kind     string   `json:"kind"`
	Text     string   `json:"text,omitempty"`
	Children []Inline `json:"children,omitempty"`
}
type Block struct {
	Kind         string       `json:"kind"`
	Level        int          `json:"level,omitempty"`
	Start        int          `json:"start,omitempty"`
	Ordered      bool         `json:"ordered,omitempty"`
	Inlines      []Inline     `json:"inlines,omitempty"`
	Items        [][]Block    `json:"items,omitempty"`
	Rows         [][][]Inline `json:"rows,omitempty"`
	Alignments   []int        `json:"alignments,omitempty"`
	Component    *Component   `json:"component,omitempty"`
	ImageSHA256s []string     `json:"image_sha256s,omitempty"`
}

func mergeInline(out []Inline, v Inline) []Inline {
	if v.Kind == "text" && v.Text == "" {
		return out
	}
	if len(out) > 0 && v.Kind == "text" && out[len(out)-1].Kind == "text" {
		out[len(out)-1].Text += v.Text
		return out
	}
	return append(out, v)
}
func (d *Document) inlines(parent ast.Node) []Inline {
	var out []Inline
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch v := n.(type) {
		case *ast.Text:
			out = mergeInline(out, Inline{Kind: "text", Text: InlineText(v, d.Source)})
			if v.HardLineBreak() {
				out = append(out, Inline{Kind: "hard_break"})
			} else if v.SoftLineBreak() {
				out = append(out, Inline{Kind: "soft_break"})
			}
		case *ast.String:
			out = mergeInline(out, Inline{Kind: "text", Text: string(v.Value)})
		case *ast.Emphasis:
			k := "emphasis"
			if v.Level == 2 {
				k = "strong"
			}
			out = append(out, Inline{Kind: k, Children: d.inlines(n)})
		case *ast.CodeSpan:
			var b strings.Builder
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if tx, ok := c.(*ast.Text); ok {
					b.Write(tx.Segment.Value(d.Source))
				}
			}
			out = append(out, Inline{Kind: "code", Text: strings.ReplaceAll(b.String(), "\n", " ")})
		}
	}
	return out
}
func (d *Document) blocks(parent ast.Node) []Block {
	var out []Block
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		var b Block
		switch v := n.(type) {
		case *ast.Heading:
			b = Block{Kind: "heading", Level: v.Level, Inlines: d.inlines(n)}
		case *ast.Paragraph, *ast.TextBlock:
			b = Block{Kind: "paragraph", Inlines: d.inlines(n)}
		case *ast.List:
			b = Block{Kind: "list", Ordered: v.IsOrdered()}
			if b.Ordered {
				b.Start = v.Start
			}
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				b.Items = append(b.Items, d.blocks(item))
			}
		case *ea.Table:
			b.Kind = "table"
			for _, a := range v.Alignments {
				b.Alignments = append(b.Alignments, int(a))
			}
			for row := n.FirstChild(); row != nil; row = row.NextSibling() {
				var cells [][]Inline
				for c := row.FirstChild(); c != nil; c = c.NextSibling() {
					cells = append(cells, d.inlines(c))
				}
				b.Rows = append(b.Rows, cells)
			}
		case *ast.FencedCodeBlock:
			c := *d.Components[v]
			c.Items = append([]KPIItem(nil), c.Items...)
			for i := range c.Items {
				c.Items[i].LabelID = ""
			}
			b = Block{Kind: "component", Component: &c}
		}
		out = append(out, b)
	}
	return out
}
func (d *Document) Projection() Projection {
	p := Projection{Slots: []SlotProjection{}}
	for _, s := range d.Slots {
		blocks := d.blocks(s)
		if blocks == nil {
			blocks = []Block{}
		}
		p.Slots = append(p.Slots, SlotProjection{s.ID, s.Locked, blocks})
	}
	return p
}

func (d *Document) ProjectionWithAssets(assets map[string]Asset) Projection {
	p := d.Projection()
	for i := range p.Slots {
		for j := range p.Slots[i].Blocks {
			b := &p.Slots[i].Blocks[j]
			if b.Component != nil && b.Component.Schema == "artex-figure-1" {
				for _, page := range b.Component.Pages {
					b.ImageSHA256s = append(b.ImageSHA256s, Hash(assets[page.Ref].PNG))
				}
			}
		}
	}
	return p
}
