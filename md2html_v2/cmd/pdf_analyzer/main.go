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
	skipPages := flag.Int("skip", 0, "Number of pages to skip from the beginning (0 or -1 = auto-detect, positive number = manual)")
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

	// 목차 끝 페이지 자동 감지 또는 수동 설정
	var actualSkipPages int
	if *skipPages <= 0 {
		// 자동 감지 모드
		fmt.Fprintf(os.Stderr, "[INFO] Auto-detecting TOC end page...\n")
		detectedSkip := detectTocEndPage(sections, r)
		if detectedSkip > 0 {
			actualSkipPages = detectedSkip
		} else {
			// 감지 실패 시 기본값 사용 (표지 1p + 목차 2p = 3p)
			actualSkipPages = 3
			fmt.Fprintf(os.Stderr, "[INFO] Using default skip pages: %d\n", actualSkipPages)
		}
	} else {
		// 수동 지정 모드
		actualSkipPages = *skipPages
		fmt.Fprintf(os.Stderr, "[INFO] Using manual skip pages: %d\n", actualSkipPages)
	}

	// 각 페이지에서 텍스트 추출하고 섹션 제목 찾기
	// actualSkipPages만큼 건너뛰고 본문 페이지부터 검색
	startPage := actualSkipPages + 1
	fmt.Fprintf(os.Stderr, "[INFO] Searching from page %d (skipping %d pages)\n", startPage, actualSkipPages)

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

// detectTocEndPage는 목차 페이지와 본문 페이지를 동적으로 구분합니다.
// 목차 페이지: 여러 섹션 제목이 있지만 본문 텍스트가 거의 없음 (점선+페이지번호 패턴)
// 본문 페이지: 섹션 제목 아래에 실제 본문 텍스트가 풍부하게 존재
//
// 반환값: 목차가 끝나는 페이지 번호 (0이면 감지 실패, 기본값 사용 필요)
func detectTocEndPage(sections []SectionPage, r *pdf.Reader) int {
	if len(sections) == 0 {
		return 0
	}

	totalPages := r.NumPage()
	firstSectionTitle := sections[0].Title

	// 페이지 2부터 스캔 시작 (페이지 1은 항상 표지로 가정)
	for pageNum := 2; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		// 페이지 텍스트 추출
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		// 첫 번째 섹션 제목이 이 페이지에 있는지 확인
		if !containsTitle(text, firstSectionTitle) {
			continue
		}

		// 본문 페이지인지 판단하는 휴리스틱
		isContentPage := isBodyPage(text, sections)

		if isContentPage {
			// 본문 시작 페이지 발견 → 이전 페이지가 목차 끝
			tocEndPage := pageNum - 1
			fmt.Fprintf(os.Stderr, "[AUTO-DETECT] Content starts at page %d (first section: '%s')\n", pageNum, firstSectionTitle)
			fmt.Fprintf(os.Stderr, "[AUTO-DETECT] TOC ends at page %d (pages to skip: %d)\n", tocEndPage, tocEndPage)
			return tocEndPage
		}

		// 목차 페이지로 판단됨, 다음 페이지 확인
		fmt.Fprintf(os.Stderr, "[AUTO-DETECT] Page %d appears to be TOC (contains '%s' but no body text)\n", pageNum, firstSectionTitle)
	}

	// 감지 실패
	fmt.Fprintf(os.Stderr, "[WARN] Could not detect TOC end page (content start not found)\n")
	return 0
}

// isBodyPage는 페이지가 본문 페이지인지 판단합니다.
// 본문 페이지 조건:
// 1. 섹션 제목이 있고
// 2. 점선 패턴("......" 또는 "·····")이 거의 없으며
// 3. 일정 길이 이상의 텍스트가 있음 (본문 내용 존재)
func isBodyPage(text string, sections []SectionPage) bool {
	// 점선 패턴 카운트 (목차 특유의 패턴)
	// PDF 추출 시 점선이 \x00 등으로 나올 수 있으므로 공백 정리 후 패턴 확인
	dotLeaderPattern := regexp.MustCompile(`\.{2,}|·{2,}|…{1,}`)
	dotMatches := dotLeaderPattern.FindAllString(text, -1)
	dotCount := len(dotMatches)

	// 페이지 내 섹션 제목 개수 카운트
	sectionCount := 0
	for _, sec := range sections {
		if containsTitle(text, sec.Title) {
			sectionCount++
		}
	}

	// 텍스트 길이 (공백 제외)
	cleanText := strings.ReplaceAll(text, " ", "")
	cleanText = strings.ReplaceAll(cleanText, "\n", "")
	cleanText = strings.ReplaceAll(cleanText, "\t", "")
	textLength := len(cleanText)

	// 본문 페이지 판단 기준 강화:
	// 1. 섹션 제목이 너무 많으면 목차 페이지임 (보통 한 페이지에 5개 이상의 섹션이 시작되지 않음)
	if sectionCount > 5 {
		return false
	}

	// 2. 텍스트 길이가 너무 짧으면 본문이 아님 (목차는 제목만 나열됨)
	// 극도로 짧은 문서 대응을 위해 250 -> 100으로 추가 하향
	if textLength < 100 {
		return false
	}

	// 3. 섹션 1개당 평균 텍스트 길이 확인
	if sectionCount > 0 {
		avgTextPerSection := textLength / sectionCount
		// 본문은 섹션 1개당 최소 설명이 있어야 함
		// 짧은 섹션 대응을 위해 150 -> 80으로 하향
		if avgTextPerSection < 80 || dotCount > 3 {
			return false
		}
	} else {
		// 섹션이 없는데 텍스트만 많다면 (예: 서론 페이지) 본문으로 간주
		if textLength > 400 && dotCount == 0 {
			return true
		}
	}

	return true
}

// containsTitle는 텍스트에서 제목을 찾음 (숫자 접두사 및 특수문자 무시)
func containsTitle(text, title string) bool {
	// 정규화: 공백 정리
	text = strings.TrimSpace(text)
	title = strings.TrimSpace(title)

	// 이모지 및 특수문자 제거 로직 (추출 과정에서 유실될 수 있음)
	stripSpecial := func(s string) string {
		// 한글, 영문, 숫자만 남김
		reg := regexp.MustCompile(`[^a-zA-Z0-9가-힣\s\[\]\(\)\-_]`)
		return reg.ReplaceAllString(s, "")
	}

	cleanText := stripSpecial(text)
	cleanTitle := stripSpecial(title)

	// 직접 매칭
	if strings.Contains(cleanText, cleanTitle) {
		return true
	}

	// 숫자 접두사 제거 후 매칭 (예: "1. 설치 및 설정" -> "설치 및 설정")
	re := regexp.MustCompile(`^\d+\.\s*`)
	noNumberTitle := re.ReplaceAllString(cleanTitle, "")
	if noNumberTitle != cleanTitle && strings.Contains(cleanText, noNumberTitle) {
		return true
	}

	return false
}
