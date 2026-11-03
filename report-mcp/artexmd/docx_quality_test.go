package artexmd

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

type qualityNode struct {
	name     string
	attrs    map[string]string
	children []*qualityNode
}

func qualityTree(t *testing.T, b []byte) *qualityNode {
	t.Helper()
	root := &qualityNode{}
	stack := []*qualityNode{root}
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		switch x := tok.(type) {
		case xml.StartElement:
			n := &qualityNode{name: x.Name.Local, attrs: map[string]string{}}
			for _, a := range x.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			p := stack[len(stack)-1]
			p.children = append(p.children, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	return root
}
func (n *qualityNode) walk(f func(*qualityNode)) {
	f(n)
	for _, c := range n.children {
		c.walk(f)
	}
}
func qualityFiles(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		out[f.Name], e = io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	return out
}
func TestDOCXDefinedStylesPropertySequenceAndPaging(t *testing.T) {
	d, e := Parse([]byte(":::slot meta.cover locked\n## Cover\n\n**bold** *em* `code`\\\nnext\n\n3. Item\n\n|a|b|\n|--|--|\n|one|two|\n\n```artex-kpi\n{\"schema_version\":\"artex-kpi-1\",\"items\":[{\"value\":\"1\",\"label\":\"Synthetic\",\"label_id\":\"test\"}]}\n```\n:::\n:::slot figure_title.timeline locked\nEyebrow\n\n## Figure\n:::\n:::slot prose.figure_note.timeline edit\n### Note\n\nInterpretation\n:::\n"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := DOCX(d.Projection(), testInputs(), nil)
	if e != nil {
		t.Fatal(e)
	}
	files := qualityFiles(t, b)
	defs := map[string]string{}
	keepNote, outline := false, 0
	qualityTree(t, files["word/styles.xml"]).walk(func(n *qualityNode) {
		if n.name == "style" {
			defs[n.attrs["styleId"]] = n.attrs["type"]
			if n.attrs["styleId"] == "ArtexFigureNote" {
				n.walk(func(c *qualityNode) { keepNote = keepNote || c.name == "keepNext" })
			}
		}
		if n.name == "outlineLvl" {
			outline++
		}
	})
	sequences := map[string]string{"pPr": "pStyle keepNext keepLines pageBreakBefore pBdr shd tabs wordWrap spacing ind jc outlineLvl", "rPr": "rStyle rFonts b i color sz", "tblPr": "tblStyle tblW tblBorders shd tblLayout tblCellMar"}
	for _, part := range []string{"word/document.xml", "word/styles.xml", "word/header1.xml", "word/footer1.xml"} {
		qualityTree(t, files[part]).walk(func(n *qualityNode) {
			want := map[string]string{"pStyle": "paragraph", "tblStyle": "table", "rStyle": "character"}[n.name]
			if want != "" && defs[n.attrs["val"]] != want {
				t.Fatalf("undefined %s %s", n.name, n.attrs["val"])
			}
			if seq, ok := sequences[n.name]; ok {
				rank := map[string]int{}
				for i, s := range strings.Fields(seq) {
					rank[s] = i
				}
				last := -1
				for _, c := range n.children {
					r, ok := rank[c.name]
					if !ok || r <= last {
						t.Fatalf("bad %s sequence %s after %d", n.name, c.name, last)
					}
					last = r
				}
			}
		})
	}
	if !keepNote || outline < 4 {
		t.Fatal("missing note chaining/heading outline", keepNote, outline)
	}
	doc := string(files["word/document.xml"])
	if !strings.Contains(doc, `<w:pageBreakBefore/>`) || strings.Contains(doc, "Cambria") || !strings.Contains(doc, `w:color w:val="A84617"`) {
		t.Fatal("paging/KPI policy")
	}
	if !strings.Contains(string(files["word/footer1.xml"]), `<w:tab w:val="right" w:pos="9071"/>`) {
		t.Fatal("footer alignment")
	}
	html, e := HTML(d.Projection(), testInputs(), nil, testFonts())
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(html), ".figure-title{break-before:page") || strings.Contains(string(html), "Cambria") || !strings.Contains(string(html), "color:#a84617") {
		t.Fatal("HTML paging/KPI")
	}
}
func TestLongRowsSplitAtSameUnicodeThreshold(t *testing.T) {
	if !rowMaySplit([][]Inline{{{Kind: "text", Text: strings.Repeat("a", 400)}, {Kind: "hard_break"}}}) {
		t.Fatal("line break must count identically to detail string")
	}
	for _, n := range []int{399, 400, 401} {
		row := [][]Inline{{{Kind: "text", Text: strings.Repeat("한", n)}}}
		split := n > 400
		if rowMaySplit(row) != split {
			t.Fatal(n)
		}
		doc := wordTable([][][]Inline{row}, nil, "ArtexDetail")
		if strings.Contains(doc, "cantSplit") == split {
			t.Fatal("Word threshold", n)
		}
		if strings.Contains(rowHTML(stringsRuneCount([]string{strings.Repeat("한", n)}) > 400), "split-row") != split {
			t.Fatal("HTML threshold", n)
		}
	}
}
