// Package analyzer analyzes PDF documents to extract section page numbers.
// Extracted from pdf_analyzer tool.
package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

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

	lastFoundPage := startPage
	for i := range sections {
		found := false
		for pageNum := lastFoundPage; pageNum <= totalPages; pageNum++ {
			page := r.Page(pageNum)
			if page.V.IsNull() {
				continue
			}

			text, err := page.GetPlainText(nil)
			if err != nil {
				continue
			}

			if containsTitle(text, sections[i].Title) {
				docPageNum := pageNum - actualSkipPages
				sections[i].Page = docPageNum
				lastFoundPage = pageNum // Move pointer to this page
				found = true
				fmt.Fprintf(os.Stderr, "[FOUND] '%s' on page %d (physical: %d)\n",
					sections[i].Title, docPageNum, pageNum)
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "[WARN] Could not find section: '%s'\n", sections[i].Title)
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
	cleanText := strings.ReplaceAll(text, " ", "")
	cleanText = strings.ReplaceAll(cleanText, "\n", "")
	cleanText = strings.ReplaceAll(cleanText, "\t", "")
	textLength := len(cleanText)

	// TOC indicator: Even if "목차" is missing, many dots or ellipses are a strong TOC signal
	dotLeaderPattern := regexp.MustCompile(`\.{2,}|·{2,}|…{1,}|·{2,}`)
	dotMatches := dotLeaderPattern.FindAllString(text, -1)
	dotCount := len(dotMatches)

	// If a page has many dot leaders, it's definitely a TOC
	if dotCount > 5 {
		return false
	}

	if strings.Contains(text, "목차") || strings.Contains(text, "Table of Contents") {
		if dotCount > 2 || textLength < 500 {
			return false
		}
	}

	if sections == nil {
		// Simple check for body pages during main loop.
		// Body pages usually have paragraphs, while TOC pages are sparse or list-like.
		return textLength > 100 && dotCount <= 3
	}

	sectionCount := 0
	for _, sec := range sections {
		if containsTitle(text, sec.Title) {
			sectionCount++
		}
	}

	// If many titles from different sections appear, it's likely a TOC page
	if sectionCount > 8 {
		return false
	}

	if textLength < 150 {
		return false
	}

	if sectionCount > 0 {
		avgTextPerSection := textLength / sectionCount
		if avgTextPerSection < 100 {
			return false
		}
	}
	return true
}

func containsTitle(text, title string) bool {
	runeCount := utf8.RuneCountInString(title)
	if runeCount < 1 {
		return false
	}
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)

	stripSpecial := func(s string) string {
		// Keep only letters and numbers
		reg := regexp.MustCompile(`[^a-zA-Z0-9가-힣]`)
		return reg.ReplaceAllString(s, "")
	}

	// Remove all spaces and special characters for the most robust Korean matching in PDFs
	cleanText := stripSpecial(text)
	cleanTitle := stripSpecial(title)

	// Remove leading numbers (1., 1.1, 1.1.1, etc)
	reNum := regexp.MustCompile(`^[\d\.]+`)
	noNumText := reNum.ReplaceAllString(cleanText, "")
	noNumTitle := reNum.ReplaceAllString(cleanTitle, "")

	if cleanTitle == "" || noNumTitle == "" {
		return false
	}

	// Double check matching with both original (cleaned) and no-number versions
	if strings.Contains(cleanText, cleanTitle) ||
		strings.Contains(noNumText, noNumTitle) ||
		strings.Contains(cleanText, noNumTitle) {
		return true
	}

	return false
}
