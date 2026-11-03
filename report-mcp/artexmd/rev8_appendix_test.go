package artexmd

import (
	"strings"
	"testing"
)

func TestRev8AppendixStartsNewPage(t *testing.T) {
	d, e := Parse([]byte(":::slot meta.cover locked\n## Cover\n:::\n:::slot appendix.customer_facts locked\n## Canonical details\n\n|N|Fact|\n|---|---|\n|1|HTTP 403|\n:::\n"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := DOCX(d.Projection(), testInputs(), nil)
	if e != nil {
		t.Fatal(e)
	}
	files := qualityFiles(t, b)
	doc := string(files["word/document.xml"])
	if !strings.Contains(doc, `<w:gridCol w:w="680"/><w:gridCol w:w="8391"/>`) {
		t.Fatal("appendix width regression")
	}
	at := strings.Index(doc, "Canonical details")
	if at < 0 {
		t.Fatal("missing appendix")
	}
	prefix := doc[:at]
	p := strings.LastIndex(prefix, "<w:p>")
	if p < 0 || !strings.Contains(prefix[p:], "<w:pageBreakBefore/>") {
		t.Fatal("appendix not new page")
	}
	html, e := HTML(d.Projection(), testInputs(), nil, []Font{{Weight: 400, Data: []byte{0, 1, 0, 0}}, {Weight: 700, Data: []byte{0, 1, 0, 0}}})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(html), `<colgroup><col style="width:8%"/><col style="width:92%"/></colgroup>`) {
		t.Fatal("HTML appendix width regression")
	}
	if !strings.Contains(string(html), `class=" figure-title" data-artex-slot="appendix.customer_facts"`) {
		t.Fatal("missing appendix page class")
	}
}
