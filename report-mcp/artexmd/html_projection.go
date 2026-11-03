package artexmd

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

type htmlNode struct {
	Name     string
	Attr     map[string]string
	Children []*htmlNode
	Text     string
}

func (n *htmlNode) all(name string) []*htmlNode {
	var out []*htmlNode
	var walk func(*htmlNode)
	walk = func(x *htmlNode) {
		if x.Name == name {
			out = append(out, x)
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(n)
	return out
}
func (n *htmlNode) text() string {
	var b strings.Builder
	b.WriteString(n.Text)
	for _, c := range n.Children {
		b.WriteString(c.text())
	}
	return b.String()
}
func parseHTMLXML(b []byte) (*htmlNode, error) {
	d := xml.NewDecoder(bytes.NewReader(b))
	var root *htmlNode
	var stack []*htmlNode
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		switch v := t.(type) {
		case xml.StartElement:
			if len(stack) > 64 {
				return nil, Error("HTML_RESOURCE_LIMIT")
			}
			n := &htmlNode{Name: v.Name.Local, Attr: map[string]string{}}
			for _, a := range v.Attr {
				n.Attr[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, Error("HTML_INVALID")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, &htmlNode{Name: "#text", Text: string(v)})
			}
		}
	}
	return root, nil
}
func htmlReadInline(n *htmlNode) []Inline {
	var out []Inline
	for _, c := range n.Children {
		k := c.Attr["data-artex-inline"]
		i := Inline{Kind: k}
		switch k {
		case "text", "code":
			i.Text = c.text()
		case "strong", "emphasis":
			i.Children = htmlReadInline(c)
		}
		out = mergeInline(out, i)
	}
	return out
}
func htmlReadBlock(n *htmlNode) (Block, error) {
	b := Block{Kind: n.Attr["data-artex-block"]}
	switch b.Kind {
	case "paragraph", "heading":
		b.Inlines = htmlReadInline(n)
		if b.Kind == "heading" {
			b.Level, _ = strconv.Atoi(n.Attr["data-artex-level"])
		}
	case "list":
		b.Ordered = n.Attr["data-artex-ordered"] == "true"
		if b.Ordered {
			b.Start, _ = strconv.Atoi(n.Attr["start"])
		}
		for _, li := range n.Children {
			if li.Name != "li" {
				return b, Error("HTML_INVALID")
			}
			var item []Block
			for _, c := range li.Children {
				part, e := htmlReadBlock(c)
				if e != nil {
					return b, e
				}
				item = append(item, part)
			}
			b.Items = append(b.Items, item)
		}
	case "table":
		for r, row := range n.all("tr") {
			var cells [][]Inline
			for _, c := range row.Children {
				cells = append(cells, htmlReadInline(c))
				if r == 0 {
					v, _ := strconv.Atoi(c.Attr["data-artex-align"])
					b.Alignments = append(b.Alignments, v)
				}
			}
			b.Rows = append(b.Rows, cells)
		}
	case "component":
		c := &Component{Schema: n.Attr["data-artex-component"], FigureID: n.Attr["data-artex-figure"]}
		b.Component = c
		switch c.Schema {
		case "artex-kpi-1":
			for _, div := range n.all("div") {
				if _, ok := div.Attr["data-artex-item"]; !ok {
					continue
				}
				k := KPIItem{LabelID: div.Attr["data-artex-label-id"]}
				for _, span := range div.Children {
					if span.Attr["class"] == "kpi-value" {
						k.Value = span.text()
					}
					if span.Attr["class"] == "kpi-label" {
						k.Label = span.text()
					}
				}
				c.Items = append(c.Items, k)
			}
		case "artex-figure-1":
			for _, child := range n.Children {
				if child.Name == "p" {
					c.Eyebrow = child.text()
				}
				if child.Name == "h3" {
					c.Title = child.text()
				}
			}
			for _, img := range n.all("img") {
				page, _ := strconv.Atoi(img.Attr["data-artex-item"])
				c.Pages = append(c.Pages, FigurePage{page, img.Attr["data-artex-ref"]})
				src := img.Attr["src"]
				if !strings.HasPrefix(src, "data:image/png;base64,") {
					return b, Error("HTML_INVALID")
				}
				raw, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(src, "data:image/png;base64,"))
				if e != nil {
					return b, e
				}
				b.ImageSHA256s = append(b.ImageSHA256s, Hash(raw))
			}
		case "artex-detail-1":
			for r, row := range n.all("tr") {
				var values []string
				for _, cell := range row.Children {
					values = append(values, cell.text())
				}
				if r == 0 {
					c.Columns = values
				} else {
					c.Rows = append(c.Rows, values)
				}
			}
			c.SHA256, _ = DetailHash(*c)
		default:
			return b, Error("HTML_INVALID")
		}
	default:
		return b, Error("HTML_INVALID")
	}
	return b, nil
}

// HTMLProjection reads visible values and image bytes from emitted HTML. It does
// not accept a serialized expected projection embedded in the document.
func HTMLProjection(b []byte) (Projection, error) {
	p := Projection{Slots: []SlotProjection{}}
	root, e := parseHTMLXML(b)
	if e != nil || root == nil {
		return p, Error("HTML_INVALID")
	}
	mains := root.all("main")
	if len(mains) != 1 {
		return p, Error("HTML_INVALID")
	}
	for _, n := range mains[0].Children {
		if n.Name != "section" {
			return p, Error("HTML_INVALID")
		}
		s := SlotProjection{ID: n.Attr["data-artex-slot"], Locked: n.Attr["data-artex-mode"] == "locked", Blocks: []Block{}}
		for _, c := range n.Children {
			b, e := htmlReadBlock(c)
			if e != nil {
				return p, e
			}
			s.Blocks = append(s.Blocks, b)
		}
		p.Slots = append(p.Slots, s)
	}
	return p, nil
}
