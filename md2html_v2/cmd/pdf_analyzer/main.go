package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// SubHeading represents a sub-heading within a section
type SubHeading struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Level int    `json:"level"`
	Page  int    `json:"page,omitempty"`
}

// SectionInput는 md2html에서 출력한 JSON 형식
type SectionInput struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Level       int          `json:"level"`
	SubHeadings []SubHeading `json:"subheadings,omitempty"`
}

// SectionPage는 섹션 ID와 페이지 번호 매핑
type SectionPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Page  int    `json:"page"`
}

// AnalysisResult는 PDF 분석 결과
type AnalysisResult struct {
	TotalPages int           `json:"total_pages"`
	Sections   []SectionPage `json:"sections"`
}

func main() {
	pdfPath := flag.String("i", "", "Input PDF file path")
	sectionsJSON := flag.String("sections", "", "JSON array of section titles to find (or file path)")
	outputJSON := flag.String("o", "", "Output JSON file (optional, defaults to stdout)")
	skipPages := flag.Int("skip", 0, "Number of pages to skip from the beginning (e.g., cover + TOC)")
	pageOffset := flag.Int("offset", 0, "Page number offset (subtract from physical page number, 0=no adjustment, 1=cover not counted)")
	flag.Parse()

	if *pdfPath == "" {
		fmt.Println("Usage: pdf_analyzer -i <pdf_file> -sections '[{\"id\":\"intro\",\"title\":\"소개\"}]'")
		os.Exit(1)
	}

	// PDF 열기
	f, r, err := pdf.Open(*pdfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to open PDF: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	totalPages := r.NumPage()
	fmt.Fprintf(os.Stderr, "[INFO] PDF has %d pages\n", totalPages)

	// 섹션 목록 파싱 (JSON 문자열 또는 파일 경로)
	var sections []SectionPage
	if *sectionsJSON != "" {
		var jsonData []byte
		// 파일인지 확인
		if _, err := os.Stat(*sectionsJSON); err == nil {
			// 파일에서 읽기
			jsonData, err = os.ReadFile(*sectionsJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to read sections file: %v\n", err)
				os.Exit(1)
			}
		} else {
			// JSON 문자열로 처리
			jsonData = []byte(*sectionsJSON)
		}

		// SectionInput으로 파싱 (md2html 출력 형식)
		var sectionInputs []SectionInput
		if err := json.Unmarshal(jsonData, &sectionInputs); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to parse sections JSON: %v\n", err)
			os.Exit(1)
		}

		// SectionPage 리스트로 변환 (섹션 + 서브헤딩)
		for _, input := range sectionInputs {
			// 섹션 추가
			sections = append(sections, SectionPage{
				ID:    input.ID,
				Title: input.Title,
				Page:  0,
			})

			// 서브헤딩 추가
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

	// 각 페이지에서 텍스트 추출하고 섹션 제목 찾기
	// skipPages만큼 건너뛰고 본문 페이지부터 검색
	startPage := *skipPages + 1
	fmt.Fprintf(os.Stderr, "[INFO] Searching from page %d (skipping %d pages)\n", startPage, *skipPages)

	for pageNum := startPage; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		// 페이지 텍스트 추출
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		// 각 섹션 제목 검색
		for i := range sections {
			if sections[i].Page == 0 { // 아직 찾지 못한 경우
				// 제목이 페이지에 있는지 확인
				if containsTitle(text, sections[i].Title) {
					// 문서 페이지 번호 = 물리적 페이지 번호 - 오프셋 (표지가 페이지 카운터에 포함되지 않음)
					docPageNum := pageNum - *pageOffset
					sections[i].Page = docPageNum
					fmt.Fprintf(os.Stderr, "[FOUND] '%s' on page %d (physical: %d)\n", sections[i].Title, docPageNum, pageNum)
				}
			}
		}
	}

	// 결과 생성
	result := AnalysisResult{
		TotalPages: totalPages,
		Sections:   sections,
	}

	// JSON 출력
	jsonOutput, _ := json.MarshalIndent(result, "", "  ")

	if *outputJSON != "" {
		if err := os.WriteFile(*outputJSON, jsonOutput, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to write output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[SUCCESS] Analysis saved to %s\n", *outputJSON)
	} else {
		fmt.Println(string(jsonOutput))
	}
}

// containsTitle는 텍스트에서 제목을 찾음 (숫자 접두사 무시)
func containsTitle(text, title string) bool {
	// 정규화: 공백 정리
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)

	// 직접 매칭
	if strings.Contains(text, title) {
		return true
	}

	// 숫자 접두사 제거 후 매칭 (예: "1. 설치 및 설정" -> "설치 및 설정")
	re := regexp.MustCompile(`^\d+\.\s*`)
	cleanTitle := re.ReplaceAllString(title, "")
	if cleanTitle != title && strings.Contains(text, cleanTitle) {
		return true
	}

	return false
}
