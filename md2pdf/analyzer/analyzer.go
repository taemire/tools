// Package analyzer analyzes PDF documents to extract section page numbers.
// Extracted from pdf_analyzer tool.
package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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
	ID    string `json:"id"`
	Title string `json:"title"`
	Page  int    `json:"page"`
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
			sections = append(sections, SectionPage{
				ID:    input.ID,
				Title: input.Title,
				Page:  0,
			})
			for _, sub := range input.SubHeadings {
				sections = append(sections, SectionPage{
					ID:    sub.ID,
					Title: sub.Title,
					Page:  0,
				})
			}
		}
		fmt.Fprintf(os.Stderr, "[INFO] Loaded %d sections (including subheadings)\n", len(sections))
	}

	// Detect or use provided skip pages
	var actualSkipPages int
	if skipPages <= 0 {
		fmt.Fprintf(os.Stderr, "[INFO] Auto-detecting TOC end page...\n")
		detectedSkip := detectTocEndPage(sections, r)
		if detectedSkip > 0 {
			actualSkipPages = detectedSkip
		} else {
			actualSkipPages = 3
			fmt.Fprintf(os.Stderr, "[INFO] Using default skip pages: %d\n", actualSkipPages)
		}
	} else {
		actualSkipPages = skipPages
		fmt.Fprintf(os.Stderr, "[INFO] Using manual skip pages: %d\n", actualSkipPages)
	}

	// Search from body pages
	startPage := actualSkipPages + 1
	fmt.Fprintf(os.Stderr, "[INFO] Searching from page %d (skipping %d pages)\n", startPage, actualSkipPages)

	for pageNum := startPage; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		for i := range sections {
			if sections[i].Page == 0 {
				if containsTitle(text, sections[i].Title) {
					docPageNum := pageNum - actualSkipPages
					sections[i].Page = docPageNum
					fmt.Fprintf(os.Stderr, "[FOUND] '%s' on page %d (physical: %d, skipped: %d)\n",
						sections[i].Title, docPageNum, pageNum, actualSkipPages)
				}
			}
		}
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

func detectTocEndPage(sections []SectionPage, r *pdf.Reader) int {
	if len(sections) == 0 {
		return 0
	}

	totalPages := r.NumPage()
	firstSectionTitle := sections[0].Title

	for pageNum := 2; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		if !containsTitle(text, firstSectionTitle) {
			continue
		}

		if isBodyPage(text, sections) {
			tocEndPage := pageNum - 1
			fmt.Fprintf(os.Stderr, "[AUTO-DETECT] Content starts at page %d (first section: '%s')\n", pageNum, firstSectionTitle)
			fmt.Fprintf(os.Stderr, "[AUTO-DETECT] TOC ends at page %d (pages to skip: %d)\n", tocEndPage, tocEndPage)
			return tocEndPage
		}

		fmt.Fprintf(os.Stderr, "[AUTO-DETECT] Page %d appears to be TOC (contains '%s' but no body text)\n", pageNum, firstSectionTitle)
	}

	fmt.Fprintf(os.Stderr, "[WARN] Could not detect TOC end page (content start not found)\n")
	return 0
}

func isBodyPage(text string, sections []SectionPage) bool {
	dotLeaderPattern := regexp.MustCompile(`\.{2,}|·{2,}|…{1,}`)
	dotMatches := dotLeaderPattern.FindAllString(text, -1)
	dotCount := len(dotMatches)

	sectionCount := 0
	for _, sec := range sections {
		if containsTitle(text, sec.Title) {
			sectionCount++
		}
	}

	cleanText := strings.ReplaceAll(text, " ", "")
	cleanText = strings.ReplaceAll(cleanText, "\n", "")
	cleanText = strings.ReplaceAll(cleanText, "\t", "")
	textLength := len(cleanText)

	if sectionCount > 5 {
		return false
	}
	if textLength < 100 {
		return false
	}
	if sectionCount > 0 {
		avgTextPerSection := textLength / sectionCount
		if avgTextPerSection < 80 || dotCount > 3 {
			return false
		}
	} else {
		if textLength > 400 && dotCount == 0 {
			return true
		}
	}
	return true
}

func containsTitle(text, title string) bool {
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)

	stripSpecial := func(s string) string {
		reg := regexp.MustCompile(`[^a-zA-Z0-9가-힣\s\[\]\(\)\-_]`)
		return reg.ReplaceAllString(s, "")
	}

	cleanText := stripSpecial(text)
	cleanTitle := stripSpecial(title)

	if strings.Contains(cleanText, cleanTitle) {
		return true
	}

	re := regexp.MustCompile(`^\d+\.\s*`)
	noNumberTitle := re.ReplaceAllString(cleanTitle, "")
	if noNumberTitle != cleanTitle && strings.Contains(cleanText, noNumberTitle) {
		return true
	}

	return false
}
