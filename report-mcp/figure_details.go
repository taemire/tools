package main

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

type VisualDetailRow struct {
	Number        int               `json:"number"`
	Key           string            `json:"key"`
	Role          string            `json:"role"`
	Metadata      string            `json:"metadata"`
	State         string            `json:"state,omitempty"`
	At            string            `json:"at,omitempty"`
	FullText      string            `json:"full_text"`
	Fields        map[string]string `json:"fields,omitempty"`
	SourceRefs    []string          `json:"source_refs"`
	MemberNumbers []int             `json:"member_numbers,omitempty"`
}
type VisualDetailTable struct {
	Version string            `json:"schema_version"`
	Kind    string            `json:"kind"`
	Rows    []VisualDetailRow `json:"rows"`
	SHA256  string            `json:"sha256"`
}

func validateFigureDetails(in svgBundle) error {
	tables := map[string]VisualDetailTable{}
	for _, t := range in.DetailTables {
		if _, ok := tables[t.Kind]; ok || t.Version != "visual-details-1" || len(t.Rows) == 0 || len(t.Rows) > 10000 {
			return fmt.Errorf("FIGURE_DETAIL_TABLE_INVALID")
		}
		raw, e := json.Marshal(t.Rows)
		if e != nil || figureHash(raw) != t.SHA256 {
			return fmt.Errorf("FIGURE_DETAIL_TABLE_HASH")
		}
		keys := map[string]bool{}
		for i, r := range t.Rows {
			if r.Number != i+1 || r.Key == "" || r.FullText == "" || keys[r.Key] || strings.Contains(r.Metadata+r.FullText, "[[L10N_KEY_MISSING:") {
				return fmt.Errorf("FIGURE_DETAIL_ROW_INVALID")
			}
			keys[r.Key] = true
			for _, n := range r.MemberNumbers {
				if n < 1 || n > len(t.Rows) {
					return fmt.Errorf("FIGURE_DETAIL_REFERENCE_MISSING")
				}
			}
		}
		tables[t.Kind] = t
	}
	for _, f := range in.Figures {
		t, ok := tables[f.Kind]
		if (len(in.DetailTables) > 0 || f.DetailTableSHA256 != "") && (!ok || f.DetailTableSHA256 != t.SHA256) {
			return fmt.Errorf("FIGURE_DETAIL_FIGURE_BINDING")
		}
	}
	if len(in.DetailTables) > 0 {
		for _, key := range []string{"visual.details.title", "visual.details.number", "visual.details.metadata", "visual.details.full_text"} {
			if in.PreviewLabels[key] == "" {
				return fmt.Errorf("L10N_KEY_MISSING:%s", key)
			}
		}
	}
	return nil
}
func figureDetailsHTML(in svgBundle, kind string) string {
	var table *VisualDetailTable
	for i := range in.DetailTables {
		if in.DetailTables[i].Kind == kind {
			table = &in.DetailTables[i]
			break
		}
	}
	if table == nil {
		return ""
	}
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<section class="detail-table" data-kind="` + esc(kind) + `"><h3>` + esc(in.PreviewLabels["visual.details.title"]) + `</h3><table><thead><tr>`)
	for _, id := range []string{"visual.details.number", "visual.details.metadata", "visual.details.full_text"} {
		b.WriteString(`<th>` + esc(in.PreviewLabels[id]) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range table.Rows {
		fmt.Fprintf(&b, `<tr><td>[%d]</td><td>%s</td><td>%s</td></tr>`, r.Number, figureURLBreakHTML(r.Metadata), figureURLBreakHTML(r.FullText))
	}
	b.WriteString(`</tbody></table></section>`)
	return b.String()
}

var figureURLText = regexp.MustCompile(`(?m)https?://[^\s]+|(?:^|[ \t])/[A-Za-z0-9_<>{}%][^\s]*`)

func figureURLBreakHTML(text string) string {
	var b strings.Builder
	pos := 0
	for _, span := range figureURLText.FindAllStringIndex(text, -1) {
		b.WriteString(html.EscapeString(text[pos:span[0]]))
		token := text[span[0]:span[1]]
		start := 0
		for i, r := range token {
			if r == '/' || r == '?' || r == '&' || r == '.' {
				piece := token[start : i+1]
				b.WriteString(`<span class="url-piece">` + html.EscapeString(piece) + `</span><wbr>`)
				start = i + 1
			}
		}
		if start < len(token) {
			b.WriteString(`<span class="url-piece">` + html.EscapeString(token[start:]) + `</span>`)
		}
		pos = span[1]
	}
	b.WriteString(html.EscapeString(text[pos:]))
	return b.String()
}
