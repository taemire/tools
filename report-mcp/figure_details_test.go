package main

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestFigureNumberedDetailsContract(t *testing.T) {
	root := os.Getenv("REPORT_R3_FIXTURE")
	if root == "" {
		t.Skip("explicit numbered local fixture required")
	}
	raw, e := os.ReadFile(filepath.Join(root, "bundle.json"))
	if e != nil {
		t.Fatal(e)
	}
	var in svgBundle
	if e = json.Unmarshal(raw, &in); e != nil {
		t.Fatal(e)
	}
	if len(in.DetailTables) != 6 {
		t.Fatal("numbered six-table fixture required")
	}
	if e = validateFigureDetails(in); e != nil {
		t.Fatal(e)
	}
	for _, table := range in.DetailTables {
		body := figureDetailsHTML(in, table.Kind)
		for _, row := range table.Rows {
			if !strings.Contains(html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(body, "")), row.FullText) {
				t.Fatal("full row clipped", table.Kind, row.Number)
			}
		}
	}
	bad := in
	bad.DetailTables = append([]VisualDetailTable(nil), in.DetailTables...)
	bad.DetailTables[0].Rows = append([]VisualDetailRow(nil), in.DetailTables[0].Rows...)
	bad.DetailTables[0].Rows[0].FullText += "tampered"
	if validateFigureDetails(bad) == nil {
		t.Fatal("modified table accepted")
	}
	bad = in
	bad.DetailTables = nil
	if validateFigureDetails(bad) == nil {
		t.Fatal("missing bound tables accepted")
	}
	bad = in
	bad.DetailTables = append([]VisualDetailTable(nil), in.DetailTables...)
	bad.DetailTables[0].Rows = append([]VisualDetailRow(nil), in.DetailTables[0].Rows...)
	bad.DetailTables[0].Rows[0].FullText = "<script>alert('example')</script> & full text"
	data, _ := json.Marshal(bad.DetailTables[0].Rows)
	bad.DetailTables[0].SHA256 = figureHash(data)
	rendered := figureDetailsHTML(bad, bad.DetailTables[0].Kind)
	if strings.Contains(rendered, "<script>") || !strings.Contains(rendered, "&lt;script&gt;") {
		t.Fatal("detail HTML injection")
	}
}

func TestFigureURLBreaksPreserveTokens(t *testing.T) {
	text := "http://127.0.0.1:8081/api/users/<id>\n/api/users/<id>"
	body := figureURLBreakHTML(text)
	plain := html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(body, ""))
	if plain != text || !strings.Contains(body, `<span class="url-piece">&lt;id&gt;</span>`) || strings.Contains(body, `&lt;i<wbr>`) {
		t.Fatal("URL content or atomic placeholder changed")
	}
}
