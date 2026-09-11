package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A real, minimal PDF with deliberately no searchable heading text. The only
// authoritative mapping is the destination's page dictionary reference.
func fixture(t *testing.T, catalog, extra string, inputs []SectionInput) (string, string) {
	t.Helper()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R " + catalog + " >>",
		"<< /Type /Pages /Count 4 /Kids [3 0 R 4 0 R 5 0 R 6 0 R] >>",
	}
	for i := 0; i < 4; i++ {
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /StructParents %d >>", i))
	}
	if extra != "" {
		objects = append(objects, extra)
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	dir := t.TempDir()
	pdfPath, jsonPath := filepath.Join(dir, "input.pdf"), filepath.Join(dir, "sections.json")
	if err := os.WriteFile(pdfPath, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	return pdfPath, jsonPath
}

func TestAnalyzeDestinations(t *testing.T) {
	inputs := []SectionInput{{ID: "body", Title: "한국어 매뉴얼", SubHeadings: []SubHeading{
		{ID: "ko", Title: "한국어 제목"}, {ID: "en", Title: "English heading"},
		{ID: "mixed", Title: "AIRS 설정 및 API"},
		{ID: "long", Title: strings.Repeat("긴 제목 long heading ", 20)},
		{ID: "dup1", Title: "References"}, {ID: "dup2", Title: "References"},
	}}}
	dests := "/body [5 0 R /XYZ 0 800 null] /ko [5 0 R /XYZ 0 700 null] /en [5 0 R /Fit] /mixed [5 0 R /Fit] /long [6 0 R /Fit] /dup1 [5 0 R /Fit] /dup2 [6 0 R /Fit]"
	p, s := fixture(t, "/Dests << "+dests+" >>", "", inputs)
	r, err := AnalyzePDF(p, s, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{1, 1, 1, 1, 2, 1, 2} {
		if r.Sections[i].Page != want {
			t.Errorf("%s: got %d, want %d", r.Sections[i].ID, r.Sections[i].Page, want)
		}
	}
	r, err = AnalyzePDF(p, s, 1)
	if err != nil || r.Sections[0].Page != 2 {
		t.Fatalf("manual skip: result=%+v err=%v", r, err)
	}
}

func TestAnalyzeNameTree(t *testing.T) {
	inputs := []SectionInput{{ID: "body", Title: "Repeated title"}}
	p, s := fixture(t, "/Names << /Dests << /Kids [7 0 R] >> >>", "<< /Names [(body) << /D [5 0 R /Fit] >>] >>", inputs)
	r, err := AnalyzePDF(p, s, 0)
	if err != nil || r.Sections[0].Page != 1 {
		t.Fatalf("name tree: result=%+v err=%v", r, err)
	}
}

func TestAnalyzeRejectsIncompleteMappings(t *testing.T) {
	for _, tc := range []struct {
		name, catalog, extra string
		inputs               []SectionInput
		skip                 int
	}{
		{"missing", "/Dests << /body [5 0 R /Fit] >>", "", []SectionInput{{ID: "body"}, {ID: "absent", Title: "Missing 한국어"}}, 0},
		{"no destinations", "", "", []SectionInput{{ID: "body"}}, 0},
		{"duplicate ID", "/Dests << /body [5 0 R /Fit] >>", "", []SectionInput{{ID: "body"}, {ID: "body"}}, 0},
		{"empty ID", "", "", []SectionInput{{Title: "Title"}}, 0},
		{"nonpositive page", "/Dests << /body [5 0 R /Fit] >>", "", []SectionInput{{ID: "body"}}, 3},
		{"invalid target", "/Dests << /body [2 0 R /Fit] >>", "", []SectionInput{{ID: "body"}}, 0},
		{"cyclic tree", "/Names << /Dests 7 0 R >>", "<< /Kids [7 0 R] >>", []SectionInput{{ID: "body"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, s := fixture(t, tc.catalog, tc.extra, tc.inputs)
			if result, err := AnalyzePDF(p, s, tc.skip); err == nil {
				t.Fatalf("must fail, got %+v", result)
			} else if !strings.Contains(err.Error(), "TOC validation failed") {
				t.Fatalf("expected a mapping validation error, got %v", err)
			}
		})
	}
}

func TestValidatePDFRejectsPaginationDrift(t *testing.T) {
	inputs := []SectionInput{{ID: "body", SubHeadings: []SubHeading{{ID: "heading"}}}}
	p, s := fixture(t, "/Dests << /body [5 0 R /Fit] /heading [6 0 R /Fit] >>", "", inputs)
	expected, err := AnalyzePDF(p, s, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePDF(p, s, 0, expected); err != nil {
		t.Fatal(err)
	}
	expected.Sections[1].Page = 1
	if err := ValidatePDF(p, s, 0, expected); err == nil || !strings.Contains(err.Error(), "pagination changed") {
		t.Fatalf("page drift must fail: %v", err)
	}
}

func TestAnalyzePercentEncodedDestination(t *testing.T) {
	p, s := fixture(t, "/Names << /Dests << /Names [(test%20%ED%95%9C%EA%B8%80) [5 0 R /Fit]] >> >>", "", []SectionInput{{ID: "test 한글"}})
	if _, err := AnalyzePDF(p, s, 0); err != nil {
		t.Fatal(err)
	}
}
