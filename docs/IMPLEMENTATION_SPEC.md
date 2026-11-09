# 제품 구현 명세서 (Implementation Specification)

**프로젝트**: Common Development Tools (tools)  
**버전**: 0.1.3  
**최종 갱신**: 2026-11-09

---

## 📋 목차

1. [개요](#1-개요)
2. [md2html_v2 - Markdown to HTML 변환기](#2-md2html_v2---markdown-to-html-변환기)
3. [html2pdf - HTML to PDF 변환기](#3-html2pdf---html-to-pdf-변환기)
4. [pdf_analyzer - PDF 섹션 분석기](#4-pdf_analyzer---pdf-섹션-분석기)
5. [md2pdf_v2.bat - 2-Pass PDF 생성기](#5-md2pdf_v2bat---2-pass-pdf-생성기)
6. [mp4towebp.bat - 동영상 변환 도구](#6-mp4towebpbat---동영상-변환-도구)
7. [revlog.bat - Git 버전 조회 도구](#7-revlogbat---git-버전-조회-도구)
8. [md2pdf_v2 - Direct Markdown to PDF 변환기](#8-md2pdf_v2---direct-markdown-to-pdf-변환기)
9. [지원 템플릿](#9-지원-템플릿)
14. [namefix - 파일명 자소 분리·URL 인코딩 정리 도구](#14-namefix---파일명-자소-분리url-인코딩-정리-도구)

---

## 1. 개요

### 1.1 프로젝트 목적
다양한 프로젝트(`tkcli`, `tkadmin`, `codesign_service`)에서 공통으로 사용되는 개발, 빌드, 문서화 도구를 통합 관리합니다.

### 1.2 주요 도구
- **md2html_v2**: Markdown → HTML 변환 (문서 생성)
- **html2pdf**: HTML → PDF 변환 (Chrome/Chromium 엔진 기반)
- **pdf_analyzer**: PDF 분석 및 섹션 페이지 번호 추출
- **md2pdf_v2.bat**: 2-Pass PDF 생성 파이프라인
- **mp4towebp.bat**: MP4 동영상을 WebP 애니메이션으로 변환
- **revlog.bat**: Git 히스토리 및 태그 조회
- **namefix**: 파일·폴더 이름의 자소 분리(NFD)·URL 인코딩 정리 (탐색기 오른쪽 클릭 메뉴 연동)

### 1.3 기술 스택
- **언어**: Go 1.27.1+, Windows Batch Script, C# 5 (.NET Framework 4.x 기본 컴파일러, namefix)
- **주요 라이브러리**:
  - `goldmark`: Markdown 파싱 (GFM, Table 확장 지원)
  - `chromedp`: Chrome DevTools Protocol 기반 PDF 렌더링
  - `github.com/ledongthuc/pdf`: PDF 파싱 및 텍스트 추출
  - `gopkg.in/yaml.v3`: YAML 설정 파일 파싱

---

## 2. md2html_v2 - Markdown to HTML 변환기

### 2.1 개요
Markdown 문서를 HTML로 변환하여 사용자 매뉴얼, API 문서, 리포트를 생성합니다.

### 2.2 주요 기능

#### 2.2.1 Markdown 변환
- **엔진**: `goldmark` (GitHub Flavored Markdown 지원)
- **확장 기능**:
  - GFM (GitHub Flavored Markdown)
  - Table 확장
  - Auto Heading ID 생성
  - Unsafe HTML 허용 (커스텀 HTML 태그 사용 가능)
  - Footnote (`[^1]` 각주 구문)
  - DefinitionList (`용어 : 정의` 구문)

#### 2.2.2 MD 확장 구문 지원 ✨ (v0.1.3)

**지원 알림 구문 (Callouts/Admonitions)**:

| 플랫폼 | 구문 예시 | 설명 |
|--------|-----------|------|
| GitHub | `> [!NOTE]` | NOTE, TIP, IMPORTANT, WARNING, CAUTION |
| Docusaurus | `:::note`, `:::tip[Title]` | note, tip, info, warning, danger |
| Docsify | `!>`, `?>` | Important, Tip |

**지원 하이라이트**:
- `==텍스트==` → `<mark>텍스트</mark>` (노란 배경)

**지원 이모지 단축코드** (40개+):
- `:rocket:` → 🚀, `:fire:` → 🔥, `:warning:` → ⚠️, `:+1:` → 👍 등
- 카테고리: 일반, 상태/알림, 감정, 개발, 화살표

#### 2.2.3 기존 Docsify 문법 지원
- **알림 블록**:
  - `!> 중요한 내용` → Important Alert (주황색, 느낌표 아이콘)
  - `?> 팁` → Tip Alert (파란색, 전구 아이콘)
- **제목/본문 분리**:
  - `**제목**: 내용` 패턴을 감지하여 제목과 본문으로 분리
  - 예: `**알림**: 다른 곳에서...` → 제목: "알림", 본문: "다른 곳에서..."

#### 2.2.3 이미지 임베딩
- **Base64 인코딩**: 로컬 이미지를 Data URI로 변환하여 HTML에 임베딩
- **MIME 타입 자동 감지**: 파일 확장자 기반 MIME 타입 설정
- **원격 이미지 유지**: HTTP(S) URL은 그대로 유지

#### 2.2.4 CSS 임베딩
- `<link rel="stylesheet">` 태그의 로컬 CSS 파일을 `<style>` 블록으로 변환
- 단일 HTML 파일로 배포 가능

#### 2.2.5 UI 컴포넌트 시스템
- **마커 문법**: `<!-- @ui:component-name -->` in Markdown
- **컴포넌트 로드**: `assets/ui/component-name.html` 파일을 찾아 삽입
- **용도**: 로그인 폼, 대시보드 목업 등 UI 스크린샷을 실제 렌더링 가능한 HTML로 삽입

#### 2.2.6 Mermaid 다이어그램 지원
- ` ```mermaid ` 코드 블록을 `<div class="mermaid">` 로 변환
- 템플릿에 Mermaid.js CDN 포함하여 자동 렌더링

#### 2.2.7 계층적 목차 (TOC) 생성
- **H1 (섹션 제목)**: 큰 목차 항목
- **H2 (서브헤딩)**: 들여쓰기된 하위 항목
- **H3 이상**: 목차에서 제외 (간결성 유지)
- **FAQ 필터링**: "Q." 로 시작하는 제목은 목차에서 제외

#### 2.2.8 2-Pass PDF 생성 지원
- **Pass 1**: 섹션 목록을 JSON으로 출력 (`-sections-json`)
- **Pass 2**: PDF 분석 결과(페이지 번호)를 입력받아 목차에 페이지 번호 삽입 (`-pages-json`)

### 2.3 사용법

#### 2.3.1 기본 사용
```bash
md2html_v2.exe -i docs/manual -o output.html -title "사용자 매뉴얼" -version "1.0.0"
```

#### 2.3.2 설정 파일 사용 (AUTHORS.yml)
```bash
md2html_v2.exe -i docs/manual -o output.html -c AUTHORS.yml
```

**AUTHORS.yml 구조**:
```yaml
project_name: "코드 서명 서비스"
organization: "회사명"
copyright: "© 2025 회사명. All rights reserved."
document:
  title: "API Reference"
  subtitle: "RESTful API 명세서"
  author: "개발팀"
  header: "코드 서명 서비스 - API Reference"
  footer: "회사명 © 2025"
```

#### 2.3.3 템플릿 지정
```bash
md2html_v2.exe -i docs -o output.html -template report
```

**사용 가능한 템플릿**:
- `default`: 기본 레이아웃
- `modern`: 모던 디자인 (어두운 배경, 그라데이션)
- `report`: 보고서 스타일 (프린트 최적화)

#### 2.3.4 2-Pass PDF 생성
```bash
# Pass 1: 섹션 목록 추출
md2html_v2.exe -i docs -o temp.html -sections-json sections.json

# PDF 생성 및 분석 (md2pdf_v2.bat에서 자동 수행)
html2pdf.exe -i temp.html -o temp.pdf
pdf_analyzer.exe -i temp.pdf -sections sections.json -o pages.json

# Pass 2: 페이지 번호가 포함된 최종 HTML 생성
md2html_v2.exe -i docs -o final.html -pages-json pages.json
html2pdf.exe -i final.html -o final.pdf
```

### 2.4 구현 상세

#### 2.4.1 파일 순서 결정
1. **우선순위**: `_sidebar.md` 파일이 있으면 해당 순서대로 처리
2. **폴백**: `_sidebar.md`가 없으면 디렉터리 스캔 (알파벳 순서)
3. **제외**: `README.md`는 웹 랜딩 페이지이므로 PDF에서 제외

#### 2.4.2 섹션 병합 로직
- **H1 (Level 1)**: 새 섹션 시작
- **H2 (Level 2)**: 이전 섹션에 병합 (앵커 ID 추가)
- **목적**: 관련된 내용을 하나의 페이지로 그룹화하여 PDF 가독성 향상

#### 2.4.3 Alert 변환 파이프라인
1. **전처리** (`preprocessAlerts`):
   - Docsify: `!> 내용` → `> [!IMPORTANT] 내용`
   - Docusaurus: `:::note` → `> [!NOTE]`, `:::danger` → `> [!CAUTION]`
   - 커스텀 제목: `:::tip[제목]` → `> [!TIP] **제목**`
2. **Goldmark 변환**: Blockquote를 `<blockquote><p>` HTML로 변환
3. **후처리** (`postProcessAlerts`):
   - `<blockquote><p>[!TYPE]...` → `<div class="alert alert-type">...`
   - 5가지 타입: NOTE, TIP, IMPORTANT, WARNING, CAUTION
   - 각 타입별 CSS 클래스 및 Font Awesome 아이콘 적용
   - 제목/본문 분리: `<strong>제목</strong>: 내용` → `<div class="alert-title">` + `<p class="alert-body">`

#### 2.4.4 Highlight 변환 (`preprocessHighlight`)
```go
func preprocessHighlight(content string) string {
    re := regexp.MustCompile(`==([^=]+)==`)
    return re.ReplaceAllString(content, "<mark>$1</mark>")
}
```

#### 2.4.5 Emoji 변환 (`preprocessEmoji`)
- 40개+ 이모지 매핑 테이블
- 정규식: `:([a-z0-9_+-]+):`
- 매핑되지 않은 단축코드는 그대로 유지

#### 2.4.4 자산 경로 정규화
- 상대 경로 (`../../assets/`) → 절대 경로 (`assets/`)
- 임베딩 실패 시에도 HTML에서 접근 가능하도록 보장

### 2.5 주요 데이터 구조

#### ManualSection
```go
type ManualSection struct {
    Title       string       `json:"title"`        // 섹션 제목
    ID          string       `json:"id"`           // 앵커 ID (파일명 기반)
    Content     string       `json:"-"`            // HTML 내용 (JSON 제외)
    Level       int          `json:"level"`        // 1=H1(섹션), 2=H2(병합)
    SubHeadings []SubHeading `json:"subheadings"`  // H2 서브헤딩 목록
    PageNumber  int          `json:"page,omitempty"` // PDF 페이지 번호 (2-Pass)
}
```

#### SubHeading
```go
type SubHeading struct {
    Title      string `json:"title"`          // 서브헤딩 제목
    ID         string `json:"id"`             // 앵커 ID
    Level      int    `json:"level"`          // 2=H2, 3=H3
    PageNumber int    `json:"page,omitempty"` // PDF 페이지 번호 (2-Pass)
}
```

---

## 3. html2pdf - HTML to PDF 변환기

### 3.1 개요
Chrome/Chromium 브라우저 엔진(CDP)을 사용하여 HTML을 고품질 PDF로 변환합니다.

### 3.2 주요 기능
- **Chrome DevTools Protocol**: 실제 브라우저 렌더링 엔진 사용
- **고품질 렌더링**: CSS, 웹폰트, SVG, Canvas 완벽 지원
- **인쇄 최적화**: `@media print` CSS 적용
- **헤더/푸터**: 페이지 번호, 제목, 날짜 자동 삽입
- **Quiet Mode**: CDP 디버그 로그 억제 (깔끔한 출력)

### 3.3 사용법
```bash
html2pdf.exe -i input.html -o output.pdf
```

### 3.4 구현 상세

#### 3.4.1 PDF 생성 설정
```go
chromedp.ActionFunc(func(ctx context.Context) error {
    buf, _, err := page.PrintToPDF().
        WithPrintBackground(true).          // 배경색/이미지 인쇄
        WithDisplayHeaderFooter(true).      // 헤더/푸터 표시
        WithHeaderTemplate(headerHTML).     // 헤더 HTML
        WithFooterTemplate(footerHTML).     // 푸터 HTML
        WithMarginTop(1.0).                 // 상단 여백 (cm)
        WithMarginBottom(1.0).              // 하단 여백 (cm)
        WithMarginLeft(0.5).                // 좌측 여백 (cm)
        WithMarginRight(0.5).               // 우측 여백 (cm)
        WithPaperWidth(8.27).               // A4 너비 (인치)
        WithPaperHeight(11.69).             // A4 높이 (인치)
        Do(ctx)
    return err
})
```

#### 3.4.2 Quiet Mode (CDP 로그 억제)
```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", true),
    chromedp.Flag("disable-gpu", true),
    chromedp.Flag("no-sandbox", true),
    chromedp.Flag("disable-dev-shm-usage", true),
    chromedp.Flag("log-level", "3"),        // 에러만 출력
)
```

---

## 4. pdf_analyzer - PDF 섹션 분석기

### 4.1 개요
PDF 파일을 분석하여 각 섹션이 어느 페이지에 위치하는지 추출합니다.

### 4.2 주요 기능

#### 4.2.1 목차 자동 감지 ✨ (v0.1.1 고도화)
- **동적 휴리스틱 감지** (`-skip 0` 또는 `-skip -1`):
  - 단순히 첫 섹션 제목을 찾는 것을 넘어, 페이지의 성격(목차 vs 본문)을 분석
  - **판단 기준**:
    - **섹션 밀도**: 한 페이지에 너무 많은 섹션 제목이 있으면 목차로 판단
    - **텍스트-섹션 비율**: 섹션 제목 하나당 텍스트 설명이 충분한지 확인
    - **점선 패턴**: 목차 특유의 도트 리더(`......`) 패턴 개수 확인
    - **최소 텍스트 길이**: 본문 페이지로서의 최소 실질 텍스트량 검증
  - 감지 실패 시 기본값 3 사용 (표지 1p + 목차 2p)
- **수동 모드** (`-skip N`, N > 0):
  - 지정된 페이지 수만큼 건너뛰고 검색 시작

#### 4.2.2 섹션 제목 매칭
- **직접 매칭**: 페이지 텍스트에 섹션 제목이 포함되어 있는지 확인
- **숫자 접두사 제거**: "1. 설치 및 설정" → "설치 및 설정" 으로 정규화하여 매칭
- **서브헤딩 지원**: H2, H3 등 모든 헤딩 레벨 추출 가능

#### 4.2.3 페이지 오프셋 처리
- **물리적 페이지**: PDF 파일의 실제 페이지 번호 (1부터 시작)
- **문서 페이지**: 사용자에게 표시되는 페이지 번호 (표지 제외)
- **계산**: `문서 페이지 = 물리적 페이지 - 오프셋`
- **기본 오프셋**: 1 (표지 페이지는 카운트하지 않음)

### 4.3 사용법

#### 4.3.1 자동 감지 모드 (권장)
```bash
pdf_analyzer.exe -i document.pdf -sections sections.json -skip 0 -offset 1 -o pages.json
```

**출력 예**:
```
[INFO] PDF has 37 pages
[INFO] Loaded 5 sections (including subheadings)
[INFO] Auto-detecting TOC end page...
[AUTO-DETECT] First section '소개' found on page 2
[AUTO-DETECT] TOC ends at page 1 (pages to skip: 1)
[INFO] Searching from page 2 (skipping 1 pages)
[FOUND] '소개' on page 2 (physical: 2)
[FOUND] '1. 설치 및 설정' on page 2 (physical: 2)
```

#### 4.3.2 수동 지정 모드
```bash
pdf_analyzer.exe -i document.pdf -sections sections.json -skip 3 -offset 1 -o pages.json
```

### 4.4 구현 상세

#### 4.4.1 목차 자동 감지 알고리즘 (Heuristic)
```go
func detectTocEndPage(sections []SectionPage, r *pdf.Reader) int {
    // ... Skip pages 2 to totalPages
    for pageNum := 2; pageNum <= totalPages; pageNum++ {
        text, _ := page.GetPlainText(nil)
        
        // 첫 번째 섹션 제목이 이 페이지에 있고, 
        // 본문 페이지 휴리스틱(isBodyPage)을 통과하면 본문 시작으로 판단
        if containsTitle(text, sections[0].Title) && isBodyPage(text, sections) {
            tocEndPage := pageNum - 1
            return tocEndPage
        }
    }
    return 0
}
```

#### 4.4.2 본문 페이지 판별 휴리스틱
```go
func isBodyPage(text string, sections []SectionPage) bool {
    // 1. 점선 패턴 카운트 (목차 특유 패턴)
    dotMatches := regexp.MustCompile(`\.{2,}|·{2,}|…{1,}`).FindAllString(text, -1)
    
    // 2. 섹션 밀도 확인 (한 페이지에 제목이 너무 많으면 목차)
    if sectionCount > 5 { return false }
    
    // 3. 텍스트-섹션 비율 (제목만 나열되면 목차)
    if sectionCount > 0 && (textLength / sectionCount) < 150 { return false }
    
    // 4. 점선이 많으면 목차
    if dotMatches > 5 { return false }
    
    return textLength > 700
}
```

#### 4.4.3 제목 매칭 함수
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

### 4.5 주요 데이터 구조

#### SectionPage
```go
type SectionPage struct {
    ID    string `json:"id"`    // 섹션 ID (md2html에서 생성)
    Title string `json:"title"` // 섹션 제목
    Page  int    `json:"page"`  // 문서 페이지 번호
}
```

#### AnalysisResult
```go
type AnalysisResult struct {
    TotalPages int           `json:"total_pages"` // 전체 페이지 수
    Sections   []SectionPage `json:"sections"`    // 섹션 목록 + 페이지 번호
}
```

---

## 5. md2pdf_v2.bat - 2-Pass PDF 생성기

### 5.1 개요
Markdown 문서를 정확한 목차 페이지 번호가 포함된 PDF로 변환하는 통합 파이프라인입니다.

### 5.2 2-Pass 전략

#### Pass 1: 초기 PDF 생성 및 분석
1. `md2html_v2.exe`: Markdown → HTML 변환 (섹션 목록 JSON 출력)
2. `html2pdf.exe`: HTML → PDF 변환
3. `pdf_analyzer.exe`: PDF 분석하여 각 섹션의 실제 페이지 번호 추출

#### Pass 2: 최종 PDF 생성
4. `md2html_v2.exe`: Markdown → HTML 변환 (페이지 번호 포함된 목차)
5. `html2pdf.exe`: HTML → PDF 변환 (최종 출력)

### 5.3 사용법

#### 5.3.1 기본 사용
```batch
md2pdf_v2.bat -i docs\manual -o USER_MANUAL
```

#### 5.3.2 설정 파일 사용
```batch
md2pdf_v2.bat -i docs\manual -o USER_MANUAL -c AUTHORS.yml
```

#### 5.3.3 수동 페이지 스킵 지정
```batch
md2pdf_v2.bat -i docs -o output -skip 4 -offset 2
```

### 5.4 파라미터

| 파라미터 | 필수 | 기본값 | 설명 |
|---------|-----|-------|------|
| `-i` | ✅ | - | 입력 Markdown 디렉터리 |
| `-o` | ✅ | - | 출력 파일 경로 (확장자 제외) |
| `-title` | ❌ | - | 문서 제목 |
| `-subtitle` | ❌ | - | 문서 부제 |
| `-version` | ❌ | - | 문서 버전 |
| `-author` | ❌ | - | 작성자 |
| `-header` | ❌ | - | 헤더 텍스트 |
| `-footer` | ❌ | - | 푸터 텍스트 |
| `-c` / `-config` | ❌ | - | AUTHORS.yml 설정 파일 경로 |
| `-template` | ❌ | `report` | 템플릿 이름 |
| `-skip` | ❌ | `0` | 목차 페이지 수 (0 = 자동 감지) |
| `-offset` | ❌ | `1` | 페이지 번호 오프셋 |

### 5.5 구현 상세

#### 5.5.1 파일 경로 처리
```batch
set HTML_OUT=!OUTPUT_PATH!.html
set PDF_OUT=!OUTPUT_PATH!.pdf
set HTML_PASS1=!OUTPUT_PATH!_pass1.html
set PDF_PASS1=!OUTPUT_PATH!_pass1.pdf
set SECTIONS_JSON=!OUTPUT_PATH!_sections.json
set PAGES_JSON=!OUTPUT_PATH!_pages.json
```

#### 5.5.2 중간 파일 정리
```batch
:cleanup
del "!HTML_PASS1!" 2>nul
del "!PDF_PASS1!" 2>nul
del "!SECTIONS_JSON!" 2>nul
del "!PAGES_JSON!" 2>nul
```

### 5.6 장점
- **정확한 페이지 번호**: 실제 PDF에서 추출하므로 100% 정확
- **자동화**: 수동 작업 없이 완전 자동화된 파이프라인
- **확장 가능**: 템플릿, 설정 파일로 다양한 문서 스타일 지원

---

## 6. mp4towebp.bat - 동영상 변환 도구

### 6.1 개요
MP4 동영상을 고효율 WebP 애니메이션으로 변환하여 문서 첨부용 파일 크기를 최소화합니다.

### 6.2 주요 기능
- **FFmpeg 자동 설치**: 시스템에 FFmpeg가 없으면 자동 다운로드 및 설치
- **고효율 압축**: WebP 형식으로 파일 크기 대폭 감소
- **품질 조정**: FPS, 너비, 품질 파라미터 커스터마이징 가능
- **파일명 공백 처리**: 공백이 포함된 파일명 지원

### 6.3 사용법

#### 6.3.1 기본 변환
```batch
mp4towebp.bat demo.mp4
```
→ `demo.webp` 생성 (기본: 10 FPS, 원본 너비, 품질 75)

#### 6.3.2 커스터마이징
```batch
mp4towebp.bat input.mp4 output.webp 15 800
```
→ `output.webp` 생성 (15 FPS, 너비 800px)

### 6.4 파라미터
| 파라미터 | 기본값 | 설명 |
|---------|-------|------|
| `%1` | (필수) | 입력 MP4 파일 경로 |
| `%2` | `%~n1.webp` | 출력 WebP 파일 경로 |
| `%3` | `10` | FPS (프레임 레이트) |
| `%4` | `-1` | 너비 (픽셀, -1=원본 유지) |

### 6.5 구현 상세

#### 6.5.1 FFmpeg 자동 다운로드
```batch
if not exist "%FFMPEG_EXE%" (
    echo [DOWNLOAD] FFmpeg가 없습니다. 자동 다운로드 중...
    powershell -Command "Invoke-WebRequest -Uri '%FFMPEG_URL%' -OutFile '%FFMPEG_ZIP%'"
    powershell -Command "Expand-Archive -Path '%FFMPEG_ZIP%' -DestinationPath '%FFMPEG_DIR%'"
)
```

#### 6.5.2 WebP 변환 명령어
```batch
"%FFMPEG_EXE%" -i "%INPUT%" -vf "fps=%FPS%,scale=%WIDTH%:-1" ^
  -c:v libwebp -quality 75 -loop 0 "%OUTPUT%"
```

---

## 7. revlog.bat - Git 버전 조회 도구

### 7.1 개요
Git 리포지토리의 커밋 히스토리 및 태그 정보를 직관적으로 조회합니다.

### 7.2 주요 기능
- **태그 포함 로그**: 커밋과 태그를 함께 표시
- **색상 출력**: 가독성 향상을 위한 컬러 코드 사용
- **커스터마이징**: 표시할 커밋 수 조정 가능

### 7.3 사용법

#### 7.3.1 기본 사용 (최근 10개 커밋)
```batch
revlog.bat
```

#### 7.3.2 커밋 수 지정
```batch
revlog.bat -n 20
```

### 7.4 출력 형식
```
* 6d6db9f (tag: v0.1.1) feat: pdf_analyzer 목차 페이지 자동 감지 기능 구현
* 4164d22 fix: mp4towebp.bat 인자 공백 처리 개선
* fa2d191 (tag: v0.1.0) style: UI Mockup 프리미엄 렌더링 디자인 고도화
```

---

## 8. md2pdf_v2 - Direct Markdown to PDF 변환기

### 8.1 개요
Chrome 엔진 없이 `gopdf` 라이브러리를 사용하여 마크다운을 직접 PDF로 변환하는 경량 도구입니다.

### 8.2 핵심 알고리즘: 2-Pass Rendering
브라우저와 달리 PDF 라이브러리는 렌더링 전에는 페이지 번호를 알 수 없으므로, 내부적으로 두 번의 렌더링 과정을 거칩니다.

1. **Pass 1 (Simulation)**:
    - 메모리 상의 PDF 컨텍스트에서 전체 마크다운을 가상 렌더링합니다.
    - 각 헤딩(H1, H2)이 배치되는 실제 페이지 번호를 기록합니다.
2. **목차 생성**:
    - Pass 1에서 수집된 정확한 페이지 번호를 사용하여 목차를 구성합니다.
3. **Pass 2 (Final Render)**:
    - 수집된 페이지 번호가 포함된 목차를 먼저 렌더링합니다.
    - 이어서 본문 콘텐츠를 최종 PDF 파일로 생성합니다.

### 8.3 주요 특징
- **의존성 제거**: 브라우저(Chrome) 설치가 필요 없는 Pure Go 구현
- **정확한 TOC**: 시뮬레이션 패스를 통해 100% 일치하는 페이지 번호 보장
- **커스터마이징**: `gopdf`를 이용한 세밀한 레이아웃 제어 가능

---

## 9. 지원 템플릿

### 8.1 default (layout.html)
- **용도**: 일반 문서, 간단한 매뉴얼
- **특징**: 
  - 깔끔한 레이아웃
  - 좌측 고정식 목차 (인쇄 시 숨김)
  - 흰색 배경, 검정 텍스트

### 8.2 modern (layout_modern.html)
- **용도**: 프레젠테이션, 마케팅 자료
- **특징**:
  - **어두운 배경**: `#0a0f1c` 다크 블루
  - **그라데이션**: 제목에 멀티 컬러 그라데이션
  - **글래스모피즘**: 반투명 카드 효과
  - **애니메이션**: 부드러운 페이드인 효과
  - **모던 타이포그래피**: Inter, Noto Sans KR

### 8.3 report (layout_report.html)
- **용도**: 공식 보고서, API 레퍼런스, 제품 명세서
- **특징**:
  - **인쇄 최적화**: 페이지 번호, 헤더/푸터 자동 삽입
  - **표지 페이지**: 프로젝트명, 버전, 날짜, 저작권 표시
  - **목차 페이지**: 계층적 목차 (페이지 번호 포함)
  - **전문적인 디자인**: 깔끔한 세리프 폰트
  - **코드 블록**: 문법 강조 및 배경색 구분
  - **Alert 블록**: Important, Tip 스타일링 (Font Awesome 아이콘)

---

## 9. 모범 사용 사례

### 9.1 사용자 매뉴얼 생성
```batch
# 프로젝트 루트에 AUTHORS.yml 생성
# docs/user_manual/ 에 Markdown 파일 작성
# docs/user_manual/_sidebar.md 에 순서 정의

md2pdf_v2.bat -i docs/user_manual -o dist/USER_MANUAL_v1.0.0 -c AUTHORS.yml
```

### 9.2 API 레퍼런스 생성
```batch
md2pdf_v2.bat -i docs/api -o dist/API_REFERENCE -template report -version "2.0.0"
```

### 9.3 화면 녹화 → WebP 변환
```batch
# 1. 화면 녹화 (OBS, ScreenToGif 등 사용)
# 2. MP4로 저장
# 3. WebP로 변환
mp4towebp.bat recordings/login_flow.mp4 docs/assets/login_flow.webp 12 600
```

### 9.4 빌드 스크립트 통합
```batch
@echo off
echo [BUILD] Building documentation...

# 사용자 매뉴얼
call tools\md2pdf_v2.bat -i docs\user_manual -o dist\docs\USER_MANUAL_v%VERSION% -c AUTHORS.yml

# API 레퍼런스
call tools\md2pdf_v2.bat -i docs\api -o dist\docs\API_REFERENCE_v%VERSION% -c AUTHORS.yml -template report

echo [SUCCESS] Documentation built successfully!
```

---

## 10. 제한사항 및 알려진 이슈

### 10.1 md2html_v2
- **대용량 이미지**: Base64 임베딩 시 HTML 파일 크기 증가
- **복잡한 HTML**: 중첩된 HTML 태그는 일부 정규식 처리가 실패할 수 있음

### 10.2 html2pdf
- **브라우저 의존성**: Chrome/Chromium이 시스템에 설치되어 있어야 함
- **폰트**: 시스템 폰트만 사용 가능 (웹폰트는 CDN 필요)

### 10.3 pdf_analyzer
- **PDF 암호화**: 암호화된 PDF는 분석 불가
- **OCR 불가**: 이미지 기반 PDF (스캔 문서)는 텍스트 추출 불가

---

## 11. 향후 개선 계획

### 11.1 다국어 지원
- i18n 시스템 도입 (영어, 한국어, 일본어)
- 템플릿별 언어 설정 파일 지원

### 11.2 테마 시스템
- CSS 변수 기반 테마 커스터마이징
- 사용자 정의 CSS 파일 로드 지원

### 11.3 PDF 테마 시스템 (제안 중)
- 커버 페이지 템플릿 선택 (Corporate, Modern, Minimal)
- 색상 팔레트 커스터마이징
- 로고 이미지 삽입 지원

자세한 내용은 [`docs/PDF_THEME_SYSTEM_PROPOSAL.md`](./PDF_THEME_SYSTEM_PROPOSAL.md) 참조.

---

## 12. 유지보수 가이드

### 12.1 빌드 방법
```batch
# md2html_v2 빌드
cd md2html_v2
go build -o md2html_v2.exe .

# html2pdf 빌드
cd html2pdf
go build -o html2pdf.exe .

# pdf_analyzer 빌드
cd md2html_v2\cmd\pdf_analyzer
go build -o pdf_analyzer.exe .
```

### 12.2 의존성 관리
```batch
# 의존성 다운로드
go mod download

# go.mod 정리
go mod tidy

# vendor 디렉터리 생성 (오프라인 빌드)
go mod vendor
```

### 12.3 테스트
```batch
# 단위 테스트
go test ./...

# 통합 테스트 (예제 문서 생성)
md2pdf_v2.bat -i examples\sample_manual -o test_output
```

---

## 13. 참조 문서

- [CHANGELOG.md](../CHANGELOG.md): 변경 이력
- [PROJECT_HISTORY.md](./PROJECT_HISTORY.md): 작업 이력 상세
- [ISSUES.md](./ISSUES.md): 알려진 이슈 및 버그 추적
- [PDF_THEME_SYSTEM_PROPOSAL.md](./PDF_THEME_SYSTEM_PROPOSAL.md): PDF 테마 시스템 제안서
- [namefix/README.md](../namefix/README.md): namefix 사용자 안내

---

## 14. namefix - 파일명 자소 분리·URL 인코딩 정리 도구

### 14.1 개요
파일·폴더 이름의 자소 분리(NFD)를 완성형(NFC)으로 합치고, URL 인코딩(`%XX`)을 UTF-8 기준으로 풉니다. 탐색기 오른쪽 클릭 메뉴와 명령줄에서 실행합니다. 사용법은 [`namefix/README.md`](../namefix/README.md)를 따릅니다.

### 14.2 구성
| 구성 요소 | 내용 |
|---|---|
| `namefix.exe` | 콘솔 하위 시스템. 명령줄·스크립트·테스트용 |
| `namefixw.exe` | Windows 하위 시스템(`/define:WINEXE`). 탐색기 메뉴용, 콘솔 창 없음, 항상 `--gui` 모드 |
| `build.bat` | `%WINDIR%\Microsoft.NET\Framework64\v4.0.30319\csc.exe`로 같은 소스에서 두 실행 파일 생성(`/codepage:65001 /warnaserror+`) |
| `install.bat` / `uninstall.bat` | `%LOCALAPPDATA%\Programs\namefix` 복사 및 HKCU 메뉴 등록·해제. 제거는 레지스트리 해제를 항상 `namefix.exe --unregister`로 수행합니다(설치본이 없으면 `bin\` 실행 파일, 그것도 없으면 빌드 후 사용). 해제 종료 코드와 설치 폴더 삭제가 모두 성공해야 종료 코드 0을 반환합니다. 레이블 없이 ASCII로만 작성합니다. |
| `tests/run-tests.ps1` | 회귀 테스트 87건(드라이브 루트 시험 2건은 `subst` 임시 드라이브를 만들 수 있을 때만 실행, SMB 공유 시험 2건은 로컬 관리 공유에 접근할 수 있을 때만 실행, 임시 폴더, 실제 메뉴 등록 변경 없음. 제거 테스트는 격리 레지스트리 경로와 임시 `LOCALAPPDATA` 사용, 대소문자 구분 폴더 1건은 지원 환경에서만 실행) |
| `src/ExecutionLock.cs` | 이름 변경 단계의 사용자 단위 실행 잠금(`Global\namefix-execute-<SID>`, 만들 수 없으면 `Local\` 이름으로 대체) |
| `tests/e2e-shell.ps1` | `Shell.Application`으로 등록 메뉴 동사를 호출하는 연동 테스트 10건 |

### 14.3 이름 변환 알고리즘 (`NameNormalizer`)
1. **URL 디코딩**: `%XX`의 선행 바이트로 UTF-8 시퀀스 길이(1~4바이트)를 정하고 연속 바이트를 검증합니다. 다음 경우에는 선행 `%XX` 원문을 유지하고 다음 위치부터 다시 해석합니다.
   - 잘못된 선행 바이트(0x80~0xC1, 0xF5~0xFF), 끊긴 시퀀스, 과잉 표현, 서로게이트 범위, U+10FFFF 초과
   - 디코딩 결과가 제어 문자(U+0000~001F, U+007F, U+0080~009F) 또는 파일 이름 금지 문자(`\ / : * ? " < > |`)
   - `+`는 공백으로 바꾸지 않습니다.
2. URL 디코딩은 바뀌지 않을 때까지(고정점) 반복합니다(이중·다중 인코딩 대응). 디코딩이 일어나면 문자열이 짧아지므로 반복은 반드시 끝납니다.
3. 디코딩 결과를 NFC 적용 후 Windows 이름 규칙으로 검사합니다. 빈 이름, 255자 초과, 끝 공백·마침표, 예약 장치 이름이면 URL 디코딩을 적용하지 않습니다. 예약 장치 이름은 `CON`, `PRN`, `AUX`, `NUL`, `COM1~9`, `COM¹~³`, `LPT1~9`, `LPT¹~³`입니다(대소문자 무시, 확장자 포함). 근거는 Microsoft "Naming Files, Paths, and Namespaces"(2024-08-28 판)이며, `COM0`·`LPT0`은 목록에 없으므로 유효한 이름입니다.
4. NFC 정규화를 적용합니다. 정규화할 수 없는 문자열(짝 없는 서로게이트)은 그대로 둡니다.
5. 1~4단계 전체를 결과가 바뀌지 않을 때까지(최대 16회) 반복합니다. NFC가 결합 문자를 합치면 다음 변환에서 디코딩 가능 여부나 유효성 판정이 달라질 수 있습니다(예: `CON%2E◌́%41.txt` → `CON%2É%41.txt` → `CON%2ÉA.txt`). 따라서 전체 변환을 고정점으로 만들어 `Normalize(Normalize(x)) == Normalize(x)`가 성립하게 합니다. 실제 이름은 2~3회 안에 고정점에 도달합니다.

### 14.4 이름 변경 (`Renamer`)
- **처리 범위**: 요청(`WorkItem`)마다 `IncludeSelf`(폴더 자신 포함 여부), `Depth`(0: 하위 없음, 1: 직속 하위, -1: 모든 하위), 변환 옵션(`UrlDecode`, `Nfc`)을 가집니다. 요청 시점 존재 여부(`Existed`), 요청 시점 원본 식별자(`Identity`, "볼륨 일련번호-파일 ID", 링크는 링크 자체, 없거나 얻지 못하면 빈 값), 출처(`Origin`, 요청 ID, 전송하지 않음)도 함께 가집니다. 파일은 항상 자기 자신만 처리합니다. 모든 결과(`ItemResult.Origins`)에는 그 결과를 낳은 요청의 출처가 붙습니다.

  | 옵션 | IncludeSelf | Depth |
  |---|---|---|
  | (없음) | true | 0 |
  | `-r` | true | -1 |
  | `-c` | false | 1 |
  | `-c -r` | false | -1 |

- **순회**: 명시적 스택으로 하위 항목을 수집합니다. 재분석 지점(심볼릭 링크·정션) 폴더 안으로는 들어가지 않습니다. 시작 폴더가 재분석 지점이고 `Depth`가 0이 아니면 하위 항목을 수집하지 않고 오류로 보고합니다(`IncludeSelf`이면 링크 자체 이름은 정리). 읽을 수 없는 폴더는 오류로 기록하고 계속 진행합니다.
- **중복 제거**: 대상은 "링크를 따라간 실제 부모 폴더 식별자 + 디스크에 저장된 항목 이름"으로 중복 제거합니다(`TargetSet`). 같은 항목을 다른 경로 표기(`D:\work`와 `D:\WORK`, 정션 별칭)로 여러 번 지정해도 한 번만 처리합니다. 저장 이름은 다음과 같이 정합니다.
  - 하위 항목 수집으로 얻은 대상: 열거 결과의 이름
  - 요청 경로 자신: 부모 폴더를 그 이름으로 검색한 결과(정확히 같은 이름, 없으면 대소문자만 다른 유일한 항목)

  같은 파일의 별개 하드 링크와 대소문자를 구분하는 폴더의 대소문자만 다른 항목은 저장 이름이 다르므로 따로 처리합니다. 식별자나 저장 이름을 알 수 없으면 경로 문자열(서수 비교)로 구분합니다. 같은 항목이 여러 요청에 있으면 첫 요청의 부모 경로 표기와 옵션을 따르고, 결과는 그 항목을 포함한 모든 요청에 돌려줍니다.
- **처리 경로 확정**: 대상 경로는 수집할 때 "링크를 모두 따라간 실제 부모 경로(`GetFinalPathNameByHandle`, 부모 폴더별 한 번) + 저장 이름"으로 확정합니다. 실제 경로를 얻지 못하는 파일 시스템에서는 입력한 부모 경로를 씁니다. 마지막 구성요소는 링크를 따라가지 않습니다(대상이 정션이면 링크 항목 자체의 이름을 바꿈).
  - 입력 경로의 대소문자가 달라도(`A%20X.TXT`로 지정한 `a%20x.txt`) 저장 이름을 변환하므로(`a x.txt`) 표기에 따라 결과가 달라지지 않습니다.
  - 부모 경로에 정션이 남지 않습니다. 따라서 다른 대상(경로 중간의 정션 자체, 정션이 가리키는 경로의 폴더, 연쇄 정션의 중간 경유지)의 이름이 바뀌어도 처리 경로가 끊기지 않습니다. 처리 경로에 영향을 주는 것은 실제 상위 폴더뿐입니다.
  - 결과와 변경 기록에는 확정한 실제 경로가 표시됩니다(예: `subst` 드라이브나 정션으로 지정해도 실제 위치).
- **순서(위상 정렬)**: 확정한 처리 경로(실제 경로)의 상위 구성요소마다 디렉터리 항목 키를 대상과 같은 규칙으로 계산합니다(경로별 캐시). 그 키를 가진 대상 D가 있으면 T를 D보다 먼저 처리합니다. D의 이름을 먼저 바꾸면 T의 경로가 무효가 되기 때문입니다. 처리 경로에 정션이 없으므로 실제 상위 폴더 관계만 보면 되며, 드라이브 루트 바로 아래 항목도 같은 규칙으로 처리됩니다. 관계로 정해지지 않는 순서는 경로 구성요소 수(루트 제외) 내림차순, 그다음 수집 순서를 따릅니다. 링크 순환처럼 관계가 닫히는 비정상 구조가 있으면 남은 대상을 기본 순서로 덧붙입니다. 하위 항목 3,031개 트리의 `-r` 처리에 약 1.5~1.7초가 걸렸습니다(로컬 NTFS).
- **충돌**: 새 이름의 항목이 있으면 부모 폴더를 새 이름으로 검색합니다(`Directory.EnumerateFileSystemEntries(parent, newName)`). 검색은 파일 시스템의 비교 규칙(대소문자·정규화 무시 여부, 짧은 이름)에 따라 새 이름에 대응하는 실제 항목을 돌려줍니다. 돌아온 항목의 저장 이름이 모두 원본 자신의 이름(서수 비교)일 때만 같은 항목을 다른 표기로 연 것(정규화를 구분하지 않는 macOS SMB 공유 등)으로 보고 이름 변경을 진행합니다. 그 밖의 항목이 하나라도 있으면(같은 파일의 하드 링크, 대소문자만 다른 항목 포함), 검색 결과가 없거나 검색에 실패하면 `Conflict`입니다.
- **미리 보기 충돌**: 미리 보기에서는 디스크가 바뀌지 않으므로, "예정"으로 판정한 새 이름을 폴더별로 예약하고 뒤 항목의 새 이름과 비교합니다. 폴더는 경로 문자열이 아니라 링크를 따라간 실제 폴더의 식별자(볼륨·파일 ID)로 구분합니다(`D:\Work`와 `D:\work`, 정션 `D:\alias`와 대상 `D:\real`은 같은 폴더). 식별자를 얻을 수 없으면 대문자로 바꾼 경로로 구분합니다. 이름 비교는 폴더의 대소문자 구분 설정(`GetFileInformationByHandleEx(FileCaseSensitiveInfo)`)에 따라 서수 또는 대소문자 무시 서수 비교를 씁니다. 실제 실행에서는 앞서 바꾼 이름이 디스크에 생기므로 디스크 검사로 같은 충돌이 드러납니다.
- **요청 항목 확인과 멱등 처리**: 요청할 때 없던 경로(`Existed=false`)는 처리 시점에 항목이 생겼어도 처리하지 않고 오류로 보고합니다. 그 밖에는 처리 시점에 요청 경로의 항목이 요청 시점 식별자와 같은지 확인합니다. 경로가 없거나 다른 항목으로 바뀌었으면 식별자로 원본의 현재 경로를 찾습니다. 찾는 방법은 다음 두 가지입니다.
  - **식별자로 직접 열기**: 요청 경로에서 가장 가까운 기존 상위 폴더를 볼륨 힌트로 엽니다(볼륨 일련번호가 같아야 함). `OpenFileById`(64비트 파일 ID)로 항목을 열어 식별자가 다시 일치하는지 확인한 뒤, `GetFinalPathNameByHandle`로 현재 경로를 얻습니다. 이름이나 상위 폴더가 어떻게 바뀌었든 찾습니다.
  - **정리된 이름 따라가기(대체 탐색)**: `OpenFileById`를 지원하지 않는 SMB 공유 등에서 씁니다. 요청 경로의 각 단계를 정리된 이름, 원래 이름 순서로 시도하며 내려갑니다. 마지막 항목의 식별자가 같고 하드 링크가 하나뿐인 경로만 인정하며, 확인하는 후보 경로는 최대 256개입니다. 하드 링크는 파일 ID가 같아 식별자만으로 디렉터리 항목을 구분할 수 없으므로, 링크가 여럿인 파일은 두 방법 모두 원본으로 인정하지 않습니다. namefix 자신이 원본이나 상위 폴더 이름을 정리한 경우를 복구합니다.

  원본이나 상위 폴더의 이름이 먼저 정리된 경우에도 찾습니다. 찾은 항목의 이름이 이미 정리된 이름이면 `Unchanged`(`AlreadyNormalized`)로 기록하고, 그 항목 자체는 다시 처리하지 않습니다. 정리되지 않았으면 현재 경로에서 처리합니다. 폴더 요청이면 하위 항목도 현재 경로에서 처리합니다. 다음 경우에는 처리하지 않고 오류로 보고합니다.
  - 요청 경로에 새로 생긴 다른 항목
  - 요청할 때 없던 경로
  - 파일 ID를 제공하지 않는 파일 시스템에서 경로가 없어진 항목
  - 하드 링크가 둘 이상이라 현재 경로를 하나로 정할 수 없는 파일
  - 찾지 못한 항목

  변환 자체도 멱등이므로 같은 요청을 다시 처리해도 결과가 같고 오류가 나지 않습니다.
- **실행 잠금**: 실제 이름 변경(대상 수집부터 변경까지)은 `ExecutionLock`(`Global\namefix-execute-<SID>`)으로 같은 사용자의 실행 사이에서 직렬화합니다. 수집자가 비정상 종료한 뒤의 재처리나 같은 트리를 동시에 실행하는 경우에도, 나중 실행이 앞선 실행의 결과를 보고 처리합니다. 미리 보기는 디스크를 바꾸지 않으므로 잠그지 않습니다. 앞선 실행이 비정상 종료해 잠금이 버려지면 소유권을 넘겨받습니다.
- **변경 후**: `SHChangeNotify(SHCNE_RENAMEITEM | SHCNE_RENAMEFOLDER, SHCNF_PATHW)`로 탐색기에 알립니다.

### 14.5 탐색기 메뉴 (`ShellIntegration`)
`HKCU\Software\Classes` 아래에 정적 명령줄 동사를 등록합니다. 모든 동사에 `MultiSelectModel=Player`를 지정하여 15개를 넘게 선택해도 메뉴가 보이게 합니다.

| 키 | 명령줄 |
|---|---|
| `*\shell\namefix` | `"…\namefixw.exe" --gui "%1"` |
| `Directory\shell\namefix` | `"…\namefixw.exe" --gui "%1"` |
| `Directory\shell\namefix.recursive` | `"…\namefixw.exe" --gui --recursive "%1"` |
| `Directory\Background\shell\namefix` | `"…\namefixw.exe" --gui --children "%V"` |

경로 인수는 명령줄 끝에 둡니다. 드라이브 루트가 `"D:\"`로 인용되면 `\"`가 따옴표 이스케이프로 해석되어 인수가 `D:"`가 되므로, `CommandLine.CleanPathArgument`가 끝 따옴표를 지우고 `D:`를 `D:\`로 복원합니다.

해제(`--unregister`)는 키 4개를 지운 뒤 다시 열어 남은 키가 없는지 확인합니다. 키가 없으면 성공이고, 삭제 권한 거부나 잔존 키는 오류(종료 코드 1)입니다. 환경 변수 `NAMEFIX_TEST_REGISTRY_ROOT`로 등록 위치를 바꿀 수 있습니다(제거 테스트용). 오용을 막기 위해 `Software\namefix-test`로 시작하는 HKCU 경로만 허용합니다. 값이 있는데 허용 범위가 아니면 실제 메뉴(`Software\Classes`)로 되돌아가지 않고 등록·해제를 실패(종료 코드 1)시킵니다.

### 14.6 다중 실행 집계 (`Collector`)
명령줄 동사는 선택 항목마다 프로세스를 실행합니다. `--gui` 모드(미리 보기 제외)에서는 다음 절차로 요청을 한 프로세스에 모읍니다.

1. 각 프로세스는 경로를 자기 작업 폴더 기준 절대 경로로 확정합니다(`Path.GetFullPath`). 확정할 수 없는 경로(잘못된 문자, 길이 초과 등)가 있으면 집계에 참여하지 않고 그 프로세스에서 오류로 보고합니다. 요청 ID(GUID, 소문자 16진수 32자)를 하나 만들어 자기 요청 항목의 출처로 표시합니다. 완료 이벤트 `Local\namefix-done-<요청 ID>`와 문제 이벤트 `Local\namefix-problem-<요청 ID>`(둘 다 수동 재설정)를 먼저 만듭니다.
2. `Local\namefix-collector-<SID>-<세션 ID>` 뮤텍스를 먼저 얻은 프로세스가 수집자가 됩니다. 수집자는 수명 뮤텍스 `Local\namefix-life-<자기 요청 ID>`를 소유한 채 같은 이름의 파이프(`PipeOptions.Asynchronous`)로 연결을 하나씩 받습니다.
3. 메시지는 4바이트 길이(리틀 엔디언)와 UTF-8 본문으로 구성되며, 최대 4MB입니다. 본문은 첫 줄 요청 ID와 `<IncludeSelf>|<Depth>|<UrlDecode>|<Nfc>|<Existed>|<Identity>|<Path>` 줄 목록입니다.
4. 수집자는 메시지를 끝까지 읽는 즉시 처음 보는 요청 ID의 요청을 목록에 넣고(항목 출처를 그 요청 ID로 표시) ACK를 보냅니다. ACK는 0x06 한 바이트와 수명 뮤텍스 ID 32자입니다. 이미 받은 ID면 목록에 넣지 않고 ACK만 보냅니다. 연결 하나에서 생긴 예외(연결 직후 끊김 등)는 그 연결만 버리고 수집을 계속합니다.
5. 마지막 요청 후 500ms(`NAMEFIX_COLLECT_MS`, 100~5000ms) 동안 연결이 없거나 15초가 지나면 파이프를 닫고 수집자 뮤텍스를 놓습니다. 수집자는 실행 잠금 아래에서 이름을 정리하고 기록을 남깁니다. 그 뒤 받은 요청마다, 출처가 그 요청인 결과에 충돌·오류가 있으면 문제 이벤트를 먼저 켜고 완료 이벤트를 켭니다. 그다음 수명 뮤텍스를 놓습니다. 처리 중 예외가 나면 완료 이벤트를 켜지 않고 수명 뮤텍스만 놓습니다.
6. 보내는 쪽은 ACK를 받으면 `WaitHandle.WaitAny(완료 이벤트, 수명 뮤텍스)`로 기다립니다. 완료 이벤트가 켜지면 문제 이벤트에 따라 종료 코드 1 또는 0으로 종료합니다(문제 이벤트는 완료 이벤트보다 먼저 켜지므로 확정된 값입니다). 수명 뮤텍스를 얻으면(정상 해제 또는 비정상 종료로 버려짐) 완료 이벤트를 한 번 더 확인하고, 꺼져 있으면 같은 요청 ID로 다시 시도합니다(스스로 수집자가 되어 직접 처리 포함). 수명 뮤텍스가 이미 없으면 완료 이벤트만 확인합니다. 연결·쓰기·ACK에 실패해도 같은 요청 ID로 다시 시도하며, 20초 안에 참여하지 못하면 자기 요청을 직접 처리합니다.
7. **처리 보장은 최소 한 번(at-least-once)입니다.** 요청은 수집자가 처리 완료를 알리거나 보낸 쪽이 직접 처리할 때까지 사라지지 않습니다. 수집자가 처리 후 완료를 알리기 전에 종료하면 같은 요청이 다시 처리될 수 있습니다. 통신만으로는 정확히 한 번을 보장할 수 없으므로 이름 정리를 멱등하게 만들고(14.3, 14.4) 실행 잠금으로 직렬화해, 다시 처리해도 결과가 같고 오류가 나지 않게 했습니다.
8. 파이프 연결 대기, 쓰기, 읽기에는 모두 제한 시간(연결 1초, 쓰기·읽기 3초)이 있습니다. 시간을 넘긴 비동기 작업(`BeginWaitForConnection`, `BeginWrite`, `BeginRead`)은 핸들이 열린 상태에서 `CancelIoEx`로 취소를 요청합니다. 이어서 `End*`를 정확히 한 번 호출해 작업 종료를 기다리고, 오버랩 구조체와 대기 핸들을 회수한 뒤 스트림을 닫습니다. 처리 완료 대기에는 제한 시간이 없으며, 수집자가 살아 있는 동안 기다리고 수집자가 끝나면 곧바로 깨어납니다. 보내는 쪽은 파이프 연결을 붙잡지 않으므로 선택 항목 수가 파이프 인스턴스 한도(255)를 넘어도 됩니다.
9. 테스트용 지점은 다음과 같습니다.
   - `NAMEFIX_TEST_DISABLE_OPEN_BY_ID=1`: 식별자로 직접 열기를 쓰지 않고 대체 탐색만 사용합니다(SMB 공유 동작 재현).
   - `NAMEFIX_TEST_PAUSE_MS`(1~60000): 요청 경로와 식별자를 확정한 뒤, 집계·처리 전에 그만큼 멈춥니다(요청과 처리 사이의 경로 변경 재현).
   - 비정상 종료 지점: `NAMEFIX_TEST_CRASH_POINT`(`before-process`, `after-process`)와 `NAMEFIX_TEST_CRASH_MARKER`가 있으면, 수집자는 표시 파일을 새로 만들 수 있을 때 한 번만 해당 지점에서 강제 종료합니다.

### 14.7 결과 보고
- **콘솔**: 변경·예정은 표준 출력, 충돌·오류는 표준 오류에 출력합니다. 종료 코드는 0(정상), 1(충돌·오류), 2(사용법 오류)입니다. 다중 실행 집계에서는 각 프로세스의 종료 코드가 자기 요청(출처)의 결과를 따릅니다. 수집자도 다른 요청의 결과는 대화상자로만 알리고, 종료 코드는 자기 요청으로 정합니다.
- **대화상자**: 충돌·오류가 있거나, 바꿀 것이 없거나, 미리 보기일 때만 표시합니다. 요청 항목이 모두 "이미 정리됨"인 경우(같은 요청의 중복 처리)는 성공으로 보고 표시하지 않습니다. 항목은 최대 12줄까지 보여 줍니다. 탐색기 창 뒤에 가려지지 않도록 항상 위에 있는 소유자 창을 둡니다.
- **변경 기록**: `%LOCALAPPDATA%\namefix\namefix.log`(UTF-8, 탭 구분). `RENAMED`·`CONFLICT`·`ERROR`·`SUMMARY`(끝에 `already=<이미 정리된 항목 수>`) 줄을 남기며 미리 보기는 기록하지 않습니다. 1MB를 넘으면 `.1`로 교체합니다. 크기 확인·교체·추가 기록 전체를 로그 경로별 뮤텍스(`Global\namefix-log-<경로 SHA-1>`)로 직렬화합니다. `Global\` 이름은 같은 사용자의 다른 로그온 세션과도 공유됩니다. 로그 경로가 사용자별이므로 다른 사용자와 이름이 겹치지 않습니다. 뮤텍스를 만들 수 없거나 5초 안에 얻지 못하면 교체 없이 추가 기록만 합니다. `.1`이 있으면 `File.Replace`로 원자적으로 바꾸므로, 교체에 실패해도 기존 기록이 남습니다.
- **처리 중 예외**: 처리 단계에서 예기치 않은 예외가 나면 콘솔 또는 대화상자로 알리고 종료 코드 1로 끝냅니다.

### 14.8 제한 사항
- Windows 11 새 메뉴의 첫 화면에는 표시되지 않습니다("추가 옵션 표시" 아래에 표시). 첫 화면 표시에는 서명된 sparse package와 `IExplorerCommand` 구현이 필요합니다.
- 전체 경로 260자 초과 항목은 오류로 보고됩니다(.NET Framework 긴 경로 옵트인 미적용).
- 호환용 자모(U+3131~)로 입력된 이름은 정규화 대상이 아닙니다.
- 탐색기는 선택 항목마다 프로세스를 실행하므로, 수백 개 이상을 한꺼번에 선택하면 느려질 수 있습니다. 이 경우 폴더 빈 공간 메뉴나 하위 포함 메뉴를 권장합니다.

---

**최종 갱신일**: 2026-11-09  
**작성자**: TSGroup / AI Agent (Antigravity)  
**버전**: 0.1.3
