package artexmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

func slot(s string) string { return ":::slot meta.cover locked\n" + s + "\n:::\n" }
func TestDialectRejects(t *testing.T) {
	tests := []struct{ md, code string }{
		{"\ufeff" + slot("a"), "MD_BOM"}, {slot("a\r"), "MD_CRLF"}, {slot("\ta"), "MD_TAB_INDENT"},
		{":::slot meta.cover locked\na\n", "MD_SLOT_UNCLOSED"},
		{slot(":::slot x edit\na\n:::"), "MD_SLOT_NESTED"},
		{slot("a") + slot("b"), "MD_SLOT_DUPLICATE"},
		{":::slot BAD locked\na\n:::\n", "MD_SLOT_ID_INVALID"},
		{"outside\n\n" + slot("a"), "MD_CONTENT_OUTSIDE_SLOT"},
		{":::slot meta.cover locked\na\n::: trailing\n", "MD_SLOT_TRAILING"},
		{"  :::slot meta.cover locked\na\n:::\n", "MD_SLOT_NOT_TOPLEVEL"},
		{slot("> :::slot child edit\n> a\n> :::"), "MD_SLOT_NOT_TOPLEVEL"},
		{slot("- :::slot child edit\n  a\n  :::"), "MD_SLOT_NOT_TOPLEVEL"},
		{slot("title\n---"), "MD_HEADING_SETEXT"}, {slot("# title"), "MD_HEADING_LEVEL"},
		{slot("- a\n  - b\n    - c"), "MD_LIST_DEPTH"},
		{slot("```text\n:::\n```"), "MD_CODE_BLOCK"},
		{slot("````text\n```\n:::\n````"), "MD_CODE_BLOCK"},
		{slot("    code"), "MD_CODE_BLOCK"}, {slot("<b>raw</b>"), "MD_RAW_HTML"},
		{slot("![x](a.png)"), "MD_IMAGE_DIRECT"}, {slot("[x](https://example.invalid)"), "MD_LINK"},
		{slot("[x]: https://example.invalid"), "MD_LINK"},
		{slot("> quoted"), "MD_NODE_UNSUPPORTED"}, {slot("---"), "MD_NODE_UNSUPPORTED"},
		{slot("```artex-nope\n{}\n```"), "MD_COMPONENT_UNKNOWN"},
		{slot("```artex-kpi\n{}\n```"), "MD_COMPONENT_INVALID"},
		{slot("```artex-kpi\n{\"schema_version\":\"artex-kpi-1\",\"items\":[],\"extra\":1}\n```"), "MD_COMPONENT_INVALID"},
		{slot("```artex-kpi\n{\"schema_version\":\"artex-kpi-1\",\"schema_version\":\"artex-kpi-1\",\"items\":[{\"value\":\"1\",\"label\":\"x\",\"label_id\":\"x\"}]}\n```"), "MD_COMPONENT_INVALID"},
	}
	for _, c := range tests {
		t.Run(c.code+"/"+c.md, func(t *testing.T) {
			_, e := Parse([]byte(c.md))
			if e == nil || e.Error() != c.code {
				t.Fatalf("got %v want %s", e, c.code)
			}
		})
	}
}
func TestDialectAccepts(t *testing.T) {
	for _, s := range []string{"plain http://example.invalid/api/\\<id\\> &amp;", "## heading\n\ntext *em* **strong** `code`", "3. a\n4. b\n\n- c\n  - d", "|a|b|\n|:--|--:|\n|x|y|", "~~strike~~ [ ] task\n\n[^n] footnote", "", "a\\\nb", "\\:::\n\\`\\`\\`text"} {
		if _, e := Parse([]byte(slot(s))); e != nil {
			t.Errorf("%q: %v", s, e)
		}
	}
	c := Component{Schema: "artex-kpi-1", Items: []KPIItem{{"85.7%", "Synthetic label", "test.label"}}}
	b, _ := json.Marshal(c)
	if _, e := Parse([]byte(slot("```artex-kpi\n" + string(b) + "\n```"))); e != nil {
		t.Fatal(e)
	}
	detail := Component{Schema: "artex-detail-1", FigureID: "exploration_flow", Columns: []string{"n", "full"}, Rows: [][]string{{"1", "complete text"}}}
	detail.SHA256, _ = DetailHash(detail)
	b, _ = json.Marshal(detail)
	if _, e := Parse([]byte(slot("```artex-detail\n" + string(b) + "\n```"))); e != nil {
		t.Fatal(e)
	}
	detail.Rows[0][1] = "changed"
	b, _ = json.Marshal(detail)
	if _, e := Parse([]byte(slot("```artex-detail\n" + string(b) + "\n```"))); e == nil {
		t.Fatal("tampered detail accepted")
	}
}
func TestLiteralEscapesAndNoLinkify(t *testing.T) {
	s := `http://example.invalid/api/\<id\> &amp; \*x\* \[x\]\(url\) \` + "`" + `\` + "`" + `\` + "`" + `\n`
	d, e := Parse([]byte(slot(s)))
	if e != nil {
		t.Fatal(e)
	}
	var text strings.Builder
	_ = ast.Walk(d.Root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if enter {
			switch v := n.(type) {
			case *ast.Link, *ast.AutoLink, *ast.RawHTML, *ast.FencedCodeBlock, *ast.CodeBlock:
				t.Fatalf("unexpected %T", v)
			case *ast.Text:
				text.WriteString(InlineText(v, d.Source))
			}
		}
		return ast.WalkContinue, nil
	})
	if !strings.Contains(text.String(), "<id> &amp; *x* [x](url)") {
		t.Fatal(text.String())
	}
}
