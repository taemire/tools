# 제품 구현 명세서 (Implementation Specification)

**프로젝트**: Common Development Tools (tools)  
**버전**: 0.1.2  
**최종 갱신**: 2025-12-26

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

### 1.3 기술 스택
- **언어**: Go 1.21+, Windows Batch Script
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

#### 2.2.2 Docsify 문법 지원
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
1. **전처리** (`preprocessDocsify`):
   - `!> 내용` → `> [!IMPORTANT] 내용` (Blockquote로 변환)
   - `?> 내용` → `> [!TIP] 내용`
2. **Goldmark 변환**: Blockquote를 `<blockquote><p>` HTML로 변환
3. **후처리** (`postProcessAlerts`):
   - `<blockquote><p>[!IMPORTANT]...` → `<div class="alert alert-important">...`
   - 제목/본문 분리: `<strong>제목</strong>: 내용` → `<div class="alert-title">` + `<p class="alert-body">`

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

---

**최종 갱신일**: 2025-12-26  
**작성자**: 장민석 TSGroup / AI Agent (Antigravity)  
**버전**: 0.1.2
