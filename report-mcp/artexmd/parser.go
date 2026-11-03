// Package artexmd implements the deliberately small artex-md-3 dialect.
// A job owns one Document/AST; renderers never reparse its source.
package artexmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	ea "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const Version = "artex-md-3.1"

type Error string

func (e Error) Error() string { return string(e) }

type Document struct {
	Source     []byte
	Root       ast.Node
	Slots      []*Slot
	Components map[*ast.FencedCodeBlock]*Component
}

var slotKind = ast.NewNodeKind("ArtexSlot")

type Slot struct {
	ast.BaseBlock
	ID     string
	Locked bool
	closed bool
}

func (*Slot) Kind() ast.NodeKind     { return slotKind }
func (s *Slot) Dump(b []byte, l int) { ast.DumpHelper(s, b, l, map[string]string{"id": s.ID}, nil) }

var idRE = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,95}$`)
var errKey = parser.NewContextKey()

func fail(pc parser.Context, code string) {
	if pc.Get(errKey) == nil {
		pc.Set(errKey, Error(code))
	}
}

type slotParser struct{}

func (*slotParser) Trigger() []byte             { return []byte{':'} }
func (*slotParser) CanInterruptParagraph() bool { return true }
func (*slotParser) CanAcceptIndentedLine() bool { return true }
func (*slotParser) Open(parent ast.Node, r text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := r.PeekLine()
	raw := strings.TrimSuffix(string(line), "\n")
	s := strings.TrimLeft(raw, " \t")
	if !strings.HasPrefix(s, ":::") {
		return nil, parser.NoChildren
	}
	if _, ok := parent.(*Slot); ok {
		fail(pc, "MD_SLOT_NESTED")
	} else if parent.Kind() != ast.KindDocument || r.LineOffset() != 0 || raw != s {
		fail(pc, "MD_SLOT_NOT_TOPLEVEL")
	}
	p := strings.Split(s, " ")
	if len(p) != 3 || p[0] != ":::slot" || !idRE.MatchString(p[1]) || (p[2] != "locked" && p[2] != "edit") {
		fail(pc, "MD_SLOT_ID_INVALID")
		return nil, parser.NoChildren
	}
	r.Advance(len(line) - boolInt(bytes.HasSuffix(line, []byte{'\n'})))
	return &Slot{ID: p[1], Locked: p[2] == "locked"}, parser.HasChildren
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func (*slotParser) Continue(n ast.Node, r text.Reader, pc parser.Context) parser.State {
	// A delimiter inside an open raw code block belongs to that code block.
	opened := pc.OpenedBlocks()
	if len(opened) > 0 {
		if _, ok := opened[len(opened)-1].Node.(*ast.FencedCodeBlock); ok {
			return parser.Continue | parser.HasChildren
		}
	}
	line, _ := r.PeekLine()
	s := strings.TrimSuffix(string(line), "\n")
	if strings.HasPrefix(strings.TrimLeft(s, " \t"), ":::") && !strings.HasPrefix(s, ":::slot ") {
		if s != ":::" {
			if strings.TrimLeft(s, " \t") == ":::" {
				fail(pc, "MD_SLOT_NOT_TOPLEVEL")
			} else {
				fail(pc, "MD_SLOT_TRAILING")
			}
		}
		n.(*Slot).closed = true
		r.Advance(len(line) - boolInt(bytes.HasSuffix(line, []byte{'\n'})))
		return parser.Close
	}
	return parser.Continue | parser.HasChildren
}
func (*slotParser) Close(n ast.Node, _ text.Reader, pc parser.Context) {
	if !n.(*Slot).closed {
		fail(pc, "MD_SLOT_UNCLOSED")
	}
}

func Parse(src []byte) (*Document, error) {
	if len(src) > 2<<20 {
		return nil, Error("MD_RESOURCE_LIMIT")
	}
	if !utf8.Valid(src) {
		return nil, Error("MD_UTF8")
	}
	if bytes.HasPrefix(src, []byte{0xef, 0xbb, 0xbf}) {
		return nil, Error("MD_BOM")
	}
	if bytes.ContainsRune(src, '\r') {
		return nil, Error("MD_CRLF")
	}
	for _, line := range bytes.Split(src, []byte{'\n'}) {
		if len(line) > 0 && line[0] == '\t' {
			return nil, Error("MD_TAB_INDENT")
		}
	}
	src = append([]byte(nil), src...)
	pc := parser.NewContext()
	g := goldmark.New(goldmark.WithExtensions(extension.Table), goldmark.WithParserOptions(parser.WithBlockParsers(util.Prioritized(&slotParser{}, 50))))
	root := g.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	if e := pc.Get(errKey); e != nil {
		return nil, e.(Error)
	}
	if len(pc.References()) > 0 {
		return nil, Error("MD_LINK")
	}
	d := &Document{Source: src, Root: root, Components: map[*ast.FencedCodeBlock]*Component{}}
	seen := map[string]bool{}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		s, ok := n.(*Slot)
		if !ok {
			return nil, Error("MD_CONTENT_OUTSIDE_SLOT")
		}
		if seen[s.ID] {
			return nil, Error("MD_SLOT_DUPLICATE")
		}
		seen[s.ID] = true
		d.Slots = append(d.Slots, s)
	}
	if len(d.Slots) == 0 {
		return nil, Error("MD_CONTENT_OUTSIDE_SLOT")
	}
	err := ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		var code string
		switch x := n.(type) {
		case *ast.Document, *Slot, *ast.Paragraph, *ast.TextBlock, *ast.Text, *ast.String, *ast.CodeSpan, *ast.ListItem, *ea.Table, *ea.TableHeader, *ea.TableRow, *ea.TableCell:
		case *ast.Heading:
			if x.Parent().Kind() != slotKind {
				code = "MD_NODE_UNSUPPORTED"
				break
			}
			seg := x.Lines().At(0)
			start := bytes.LastIndexByte(src[:seg.Start], '\n') + 1
			if !bytes.HasPrefix(bytes.TrimLeft(src[start:], " "), []byte{'#'}) {
				code = "MD_HEADING_SETEXT"
			} else if x.Level != 2 && x.Level != 3 {
				code = "MD_HEADING_LEVEL"
			}
		case *ast.Emphasis:
			depth := 1
			for p := n.Parent(); p != nil; p = p.Parent() {
				if _, ok := p.(*ast.Emphasis); ok {
					depth++
				}
			}
			if depth > 2 {
				code = "MD_NODE_UNSUPPORTED"
			}
		case *ast.List:
			depth := 1
			for p := n.Parent(); p != nil; p = p.Parent() {
				if _, ok := p.(*ast.List); ok {
					depth++
				}
			}
			if depth > 2 {
				code = "MD_LIST_DEPTH"
			}
		case *ast.FencedCodeBlock:
			if x.Parent().Kind() != slotKind {
				code = "MD_NODE_UNSUPPORTED"
				break
			}
			c, e := parseComponent(x, src)
			if e != nil {
				return ast.WalkStop, e
			}
			d.Components[x] = c
		case *ast.CodeBlock:
			code = "MD_CODE_BLOCK"
		case *ast.RawHTML, *ast.HTMLBlock:
			code = "MD_RAW_HTML"
		case *ast.Image:
			code = "MD_IMAGE_DIRECT"
		case *ast.Link, *ast.AutoLink:
			code = "MD_LINK"
		default:
			code = "MD_NODE_UNSUPPORTED"
		}
		if code != "" {
			return ast.WalkStop, Error(code)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

type KPIItem struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelID string `json:"label_id"`
}
type FigurePage struct {
	Page int    `json:"page"`
	Ref  string `json:"ref"`
}
type Component struct {
	Schema   string       `json:"schema_version"`
	Items    []KPIItem    `json:"items,omitempty"`
	FigureID string       `json:"figure_id,omitempty"`
	Eyebrow  string       `json:"eyebrow,omitempty"`
	Title    string       `json:"title,omitempty"`
	Pages    []FigurePage `json:"pages,omitempty"`
	Columns  []string     `json:"columns,omitempty"`
	Rows     [][]string   `json:"rows,omitempty"`
	SHA256   string       `json:"sha256,omitempty"`
}

func parseComponent(n *ast.FencedCodeBlock, src []byte) (*Component, error) {
	info := ""
	if n.Info != nil {
		info = string(n.Info.Segment.Value(src))
	}
	schemas := map[string]string{"artex-kpi": "artex-kpi-1", "artex-figure": "artex-figure-1", "artex-detail": "artex-detail-1"}
	want, ok := schemas[info]
	if !ok {
		if strings.HasPrefix(info, "artex-") {
			return nil, Error("MD_COMPONENT_UNKNOWN")
		}
		return nil, Error("MD_CODE_BLOCK")
	}
	var body bytes.Buffer
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		body.Write(seg.Value(src))
	}
	if !uniqueJSONKeys(body.Bytes()) {
		return nil, Error("MD_COMPONENT_INVALID")
	}
	var c Component
	dec := json.NewDecoder(&body)
	dec.DisallowUnknownFields()
	if dec.Decode(&c) != nil || dec.Decode(new(any)) != io.EOF || c.Schema != want {
		return nil, Error("MD_COMPONENT_INVALID")
	}
	valid := true
	switch info {
	case "artex-kpi":
		valid = len(c.Items) >= 1 && len(c.Items) <= 4 && c.FigureID == "" && len(c.Pages) == 0 && len(c.Columns) == 0 && c.SHA256 == "" && c.Title == "" && c.Eyebrow == "" && len(c.Rows) == 0
		for _, i := range c.Items {
			valid = valid && i.Value != "" && i.Label != "" && i.LabelID != ""
		}
	case "artex-figure":
		valid = c.FigureID != "" && len(c.Pages) > 0 && len(c.Pages) <= 50 && len(c.Items) == 0 && len(c.Columns) == 0 && len(c.Rows) == 0 && c.SHA256 == ""
		for i, p := range c.Pages {
			valid = valid && p.Page == i+1 && p.Ref != ""
		}
	case "artex-detail":
		valid = c.FigureID != "" && len(c.Columns) > 0 && len(c.Columns) <= 12 && len(c.Rows) <= 10000 && len(c.Items) == 0 && len(c.Pages) == 0 && c.Title == "" && c.Eyebrow == ""
		for _, r := range c.Rows {
			valid = valid && len(r) == len(c.Columns)
		}
		hash, e := DetailHash(c)
		valid = valid && e == nil && hash == c.SHA256
	}
	if !valid {
		return nil, Error("MD_COMPONENT_INVALID")
	}
	return &c, nil
}

// InlineText intentionally unescapes punctuation, never HTML entities.
func InlineText(n *ast.Text, src []byte) string {
	s := n.Segment.Value(src)
	if n.IsRaw() {
		return string(s)
	}
	return string(util.UnescapePunctuations(s))
}
func (d *Document) Debug() string { return fmt.Sprintf("%s slots=%d", Version, len(d.Slots)) }

func uniqueJSONKeys(b []byte) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 32 {
			return false
		}
		t, e := d.Token()
		if e != nil {
			return false
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				s, ok := k.(string)
				if e != nil || !ok || keys[s] {
					return false
				}
				keys[s] = true
				if !value(depth + 1) {
					return false
				}
			}
		case '[':
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
		default:
			return false
		}
		_, e = d.Token()
		return e == nil
	}
	if !value(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
