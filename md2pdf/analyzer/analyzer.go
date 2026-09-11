// Package analyzer analyzes PDF documents to extract section page numbers.
// Extracted from pdf_analyzer tool.
package analyzer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

// SectionInput는 converter에서 출력한 JSON 형식
type SectionInput struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Level       int          `json:"level"`
	SubHeadings []SubHeading `json:"subheadings,omitempty"`
}

// SubHeading represents a sub-heading within a section
type SubHeading struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Level int    `json:"level"`
	Page  int    `json:"page,omitempty"`
}

// SectionPage는 섹션 ID와 페이지 번호 매핑
type SectionPage struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Page      int    `json:"page"`
	ParentID  string `json:"-"`
	ParentIdx int    `json:"-"`
}

// Result는 PDF 분석 결과
type Result struct {
	TotalPages int           `json:"total_pages"`
	Sections   []SectionPage `json:"sections"`
}

// AnalyzePDF analyzes a PDF to find which page each section starts on.
// sectionsJSONPath is the path to sections JSON from converter.
// skipPages: 0 or negative for auto-detect, positive for manual.
func AnalyzePDF(pdfPath, sectionsJSONPath string, skipPages int) (*Result, error) {
	// Open PDF
	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	totalPages := r.NumPage()
	fmt.Fprintf(os.Stderr, "[INFO] PDF has %d pages\n", totalPages)

	// Parse sections JSON
	var sections []SectionPage
	if sectionsJSONPath != "" {
		jsonData, err := os.ReadFile(sectionsJSONPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read sections file: %w", err)
		}

		var sectionInputs []SectionInput
		if err := json.Unmarshal(jsonData, &sectionInputs); err != nil {
			return nil, fmt.Errorf("failed to parse sections JSON: %w", err)
		}

		for _, input := range sectionInputs {
			// Add parent section
			sections = append(sections, SectionPage{
				ID:    input.ID,
				Title: input.Title,
				Page:  0,
			})
			parentIdx := len(sections) - 1

			// Add subheadings with parent mapping
			for _, sub := range input.SubHeadings {
				sections = append(sections, SectionPage{
					ID:        sub.ID,
					Title:     sub.Title,
					Page:      0,
					ParentID:  input.ID,
					ParentIdx: parentIdx,
				})
			}
		}
		fmt.Fprintf(os.Stderr, "[INFO] Loaded %d entries to map\n", len(sections))
	}

	// Resolve the actual PDF destinations, never text from the TOC or body.
	physicalPages, err := resolveDestinations(r, sections)
	if err != nil {
		return nil, err
	}
	actualSkipPages := skipPages
	if actualSkipPages <= 0 && len(sections) > 0 {
		actualSkipPages = physicalPages[sections[0].ID] - 1
	}
	if actualSkipPages < 0 {
		actualSkipPages = 0
	}
	fmt.Fprintf(os.Stderr, "[INFO] Destination mapping: skipping %d front-matter pages\n", actualSkipPages)
	for i := range sections {
		physical := physicalPages[sections[i].ID]
		sections[i].Page = physical - actualSkipPages
		if sections[i].Page <= 0 {
			return nil, fmt.Errorf("TOC validation failed: section %q (%s) is on physical page %d, at or before -skip %d", sections[i].Title, sections[i].ID, physical, actualSkipPages)
		}
		fmt.Fprintf(os.Stderr, "[FOUND] %s '%s' on page %d (physical: %d)\n", sections[i].ID, sections[i].Title, sections[i].Page, physical)
	}

	result := &Result{
		TotalPages: totalPages,
		Sections:   sections,
	}
	return result, nil
}

// SaveResult saves the analysis result to a JSON file.
func SaveResult(result *Result, outputPath string) error {
	jsonOutput, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := os.WriteFile(outputPath, jsonOutput, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[SUCCESS] Analysis saved to %s\n", outputPath)
	return nil
}

// resolveDestinations supports both catalog /Dests dictionaries (Chromium)
// and /Names /Dests name trees. Unsupported or ambiguous targets fail closed.
func resolveDestinations(r *pdf.Reader, sections []SectionPage) (map[string]int, error) {
	wanted := make(map[string]bool, len(sections))
	for _, section := range sections {
		if section.ID == "" || wanted[section.ID] {
			return nil, fmt.Errorf("TOC validation failed: empty or duplicate section ID %q", section.ID)
		}
		wanted[section.ID] = true
	}
	// This PDF reader resolves indirect references but does not expose their
	// object numbers. Its canonical dictionary representation retains child
	// references (Contents, Parent, etc.). Never guess if two pages are equal.
	pageNumbers := make(map[string]int, r.NumPage())
	for n := 1; n <= r.NumPage(); n++ {
		v := r.Page(n).V
		if v.IsNull() {
			return nil, fmt.Errorf("TOC validation failed: missing physical page %d", n)
		}
		key := v.String()
		if _, exists := pageNumbers[key]; exists {
			pageNumbers[key] = 0 // ambiguous; reject if used as a destination
		} else {
			pageNumbers[key] = n
		}
	}
	pages := make(map[string]int, len(sections))
	add := func(id string, destination pdf.Value) error {
		if !wanted[id] {
			// Chrome percent-encodes non-ASCII HTML fragment IDs.
			decoded, err := url.PathUnescape(id)
			if err != nil || !wanted[decoded] {
				return nil
			}
			id = decoded
		}
		if _, exists := pages[id]; exists {
			return fmt.Errorf("TOC validation failed: duplicate PDF destination %q", id)
		}
		if destination.Kind() == pdf.Dict {
			destination = destination.Key("D")
		}
		if destination.Kind() != pdf.Array || destination.Index(0).Key("Type").Name() != "Page" {
			return fmt.Errorf("TOC validation failed: unsupported PDF destination %q", id)
		}
		n := pageNumbers[destination.Index(0).String()]
		if n <= 0 {
			return fmt.Errorf("TOC validation failed: unknown or ambiguous page for destination %q", id)
		}
		pages[id] = n
		return nil
	}
	root := r.Trailer().Key("Root")
	legacy := root.Key("Dests")
	for _, id := range legacy.Keys() {
		if err := add(id, legacy.Key(id)); err != nil {
			return nil, err
		}
	}
	var walk func(pdf.Value, int) error
	nodes := 0
	walk = func(node pdf.Value, depth int) error {
		if node.IsNull() {
			return nil
		}
		nodes++
		if depth > 64 || nodes > 100000 {
			return fmt.Errorf("TOC validation failed: cyclic or oversized destination name tree")
		}
		names := node.Key("Names")
		if names.Len()%2 != 0 {
			return fmt.Errorf("TOC validation failed: malformed destination name tree")
		}
		for i := 0; i < names.Len(); i += 2 {
			if err := add(names.Index(i).Text(), names.Index(i+1)); err != nil {
				return err
			}
		}
		kids := node.Key("Kids")
		for i := 0; i < kids.Len(); i++ {
			if err := walk(kids.Index(i), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root.Key("Names").Key("Dests"), 0); err != nil {
		return nil, err
	}
	var missing []string
	for _, section := range sections {
		if pages[section.ID] == 0 {
			missing = append(missing, fmt.Sprintf("%s (%q)", section.ID, section.Title))
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("TOC validation failed: missing PDF destinations for %d section(s): %s", len(missing), strings.Join(missing, ", "))
	}
	return pages, nil
}

// ValidatePDF checks that Pass 2 retained the page mapping printed in its TOC.
func ValidatePDF(pdfPath, sectionsJSONPath string, skipPages int, expected *Result) error {
	actual, err := AnalyzePDF(pdfPath, sectionsJSONPath, skipPages)
	if err != nil {
		return err
	}
	if expected == nil || actual.TotalPages != expected.TotalPages || len(actual.Sections) != len(expected.Sections) {
		return fmt.Errorf("TOC validation failed: pagination changed between PDF passes")
	}
	for i, section := range actual.Sections {
		want := expected.Sections[i]
		if section.ID != want.ID || section.Page != want.Page {
			return fmt.Errorf("TOC validation failed: pagination changed for %s (%q): printed %d, actual %d", section.ID, section.Title, want.Page, section.Page)
		}
	}
	return nil
}
