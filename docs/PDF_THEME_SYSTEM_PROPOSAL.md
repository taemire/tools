# 📋 PDF 테마 시스템 제안서

## gopdf 기반 JSON 테마 엔진 설계

**작성일**: 2025년 12월 20일  
**버전**: 1.0  
**관련 프로젝트**: tkcli, tkadmin, codesign_service

---

## 1. 개요

### 1.1 배경
현재 HTML/CSS 기반 PDF 생성 방식은 Chrome의 CSS Paged Media 미지원으로 인해 헤더/푸터 구현에 한계가 있습니다. gopdf PoC를 통해 프로그래매틱 방식으로 완전한 헤더/푸터 구현이 가능함을 확인했습니다.

### 1.2 목표
- HTML/CSS 레이아웃 정의를 **JSON 테마 파일**로 추출
- 다양한 문서 양식(사용자 매뉴얼, API 문서, 제안서 등)을 **테마 전환**으로 지원
- **gopdf 렌더링 엔진**을 통해 완전한 헤더/푸터/페이지 번호 구현

---

## 2. 현재 HTML/CSS에서 JSON 테마 추출

### 2.1 분석 대상

**소스 파일**: `tools/md2html/templates/layout_report.html`  
**샘플 출력**: `file:///D:/wdata/dev/tkcli/dist/docs/USER_MANUAL.html`

### 2.2 CSS → JSON 매핑 테이블

현재 `layout_report.html`의 CSS 스타일을 JSON 테마 속성으로 추출한 매핑입니다:

#### 📌 색상 팔레트 (Colors)

| CSS 위치 | CSS 값 | JSON 경로 | JSON 값 |
|----------|--------|-----------|---------|
| `.cover-page border-top` | `#0056b3` | `colors.primary` | `"#0056b3"` |
| `.logo color` | `#0056b3` | `colors.primary` | `"#0056b3"` |
| `body color` | `#333` | `colors.text.default` | `"#333333"` |
| `.category color` | `#666` | `colors.text.muted` | `"#666666"` |
| `.footer color` | `#777` | `colors.text.light` | `"#777777"` |
| `.toc background` | `#f8fafc` | `colors.background.alt` | `"#f8fafc"` |
| `pre background` | `#1e293b` | `colors.background.code` | `"#1e293b"` |
| `border-color` | `#e2e8f0` | `colors.border` | `"#e2e8f0"` |

#### 📌 표지 (Cover)

| CSS 클래스 | CSS 속성 | JSON 경로 | JSON 값 |
|-----------|----------|-----------|---------|
| `.cover-page` | `border-top: 15px solid` | `cover.topBorder.height` | `15` |
| `.cover-page` | `padding: 40px` | `cover.padding` | `40` |
| `.cover-page` | `background: linear-gradient(...)` | `cover.background.gradient` | `true` |
| `.logo` | `font-size: 24px` | `cover.logo.fontSize` | `24` |
| `.logo` | `text-align: right` | `cover.logo.position.x` | `"right"` |
| `.title` | `font-size: 48px` | `cover.title.fontSize` | `48` |
| `.title` | `font-weight: 800` | `cover.title.fontWeight` | `800` |
| `.subtitle` | `font-size: 20px` | `cover.subtitle.fontSize` | `20` |
| `.subtitle` | `border-left: 4px solid` | `cover.subtitle.leftBorder.width` | `4` |
| `.info-table td` | `padding: 8px 15px` | `cover.infoTable.cellPadding` | `{"v": 8, "h": 15}` |

#### 📌 목차 (TOC)

| CSS 클래스 | CSS 속성 | JSON 경로 | JSON 값 |
|-----------|----------|-----------|---------|
| `.toc` | `background: #f8fafc` | `toc.background.color` | `"background.alt"` |
| `.toc` | `padding: 30px 40px` | `toc.background.padding` | `30` |
| `.toc` | `border-radius: 12px` | `toc.background.borderRadius` | `12` |
| `.toc h2` | `font-size: 24px` | `toc.title.fontSize` | `24` |
| `.toc h2` | `color: #0056b3` | `toc.title.color` | `"primary"` |
| `.toc li` | `margin: 12px 0` | `toc.item.lineHeight` | `30` |
| `.toc a` | `color: #0056b3` | `toc.item.color` | `"primary"` |

#### 📌 본문 타이포그래피 (Content Typography)

| CSS 클래스 | CSS 속성 | JSON 경로 | JSON 값 |
|-----------|----------|-----------|---------|
| `h1` | `font-size: 28px` | `content.heading.h1.fontSize` | `28` |
| `h1` | `color: #0056b3` | `content.heading.h1.color` | `"primary"` |
| `h1` | `border-bottom: 2px solid` | `content.heading.h1.underline` | `true` |
| `h1` | `margin-top: 50px` | `content.heading.h1.marginTop` | `50` |
| `h2` | `font-size: 22px` | `content.heading.h2.fontSize` | `22` |
| `h2` | `border-left: 4px solid` | `content.heading.h2.leftBorder` | `true` |
| `h3` | `font-size: 18px` | `content.heading.h3.fontSize` | `18` |
| `p` | `line-height: 1.8` | `content.paragraph.lineHeight` | `1.8` |

#### 📌 코드 블록 (Code)

| CSS 클래스 | CSS 속성 | JSON 경로 | JSON 값 |
|-----------|----------|-----------|---------|
| `code` | `font-family` | `fonts.code.family` | `"JetBrains Mono"` |
| `code` | `background: #f1f5f9` | `content.code.inline.backgroundColor` | `"#f1f5f9"` |
| `code` | `color: #be185d` | `content.code.inline.color` | `"#be185d"` |
| `code` | `font-size: 14px` | `content.code.inline.fontSize` | `14` |
| `pre` | `background: #1e293b` | `content.code.block.backgroundColor` | `"#1e293b"` |
| `pre` | `color: #e2e8f0` | `content.code.block.color` | `"#e2e8f0"` |
| `pre` | `padding: 20px` | `content.code.block.padding` | `20` |
| `pre` | `border-radius: 8px` | `content.code.block.borderRadius` | `8` |

#### 📌 표 (Table)

| CSS 클래스 | CSS 속성 | JSON 경로 | JSON 값 |
|-----------|----------|-----------|---------|
| `th` | `background: #f8fafc` | `content.table.headerBackground` | `"background.alt"` |
| `th, td` | `padding: 12px 16px` | `content.table.cellPadding` | `{"v": 12, "h": 16}` |
| `th, td` | `border: 1px solid #e2e8f0` | `content.table.borderColor` | `"border"` |
| `tr:nth-child(even)` | `background: #f8fafc` | `content.table.altRowBackground` | `"background.alt"` |

#### 📌 인용구/알림 (Blockquote)

| CSS 클래스 | CSS 속성 | JSON 경로 | JSON 값 |
|-----------|----------|-----------|---------|
| `blockquote` | `background: #eff6ff` | `content.blockquote.backgroundColor` | `"#eff6ff"` |
| `blockquote` | `border-left: 4px solid` | `content.blockquote.leftBorder.width` | `4` |
| `blockquote` | `padding: 16px 20px` | `content.blockquote.padding` | `{"v": 16, "h": 20}` |

### 2.3 추출 프로세스

```
┌──────────────────────────────────────────────────────────────────────────┐
│  1. HTML 템플릿 분석                                                      │
│     layout_report.html의 <style> 섹션에서 CSS 규칙 추출                   │
└───────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  2. CSS 속성 분류                                                         │
│     - colors: 색상 값 (#hex, rgb)                                         │
│     - fonts: font-family, font-size, font-weight                         │
│     - spacing: padding, margin, gap                                       │
│     - borders: border-width, border-color, border-radius                 │
│     - layout: width, height, position                                    │
└───────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  3. JSON 스키마에 매핑                                                     │
│     CSS 속성 → JSON 경로로 변환 (위 매핑 테이블 참조)                       │
└───────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  4. 테마 파일 생성                                                         │
│     corporate-blue.json 등 테마 파일로 저장                               │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.4 자동화 도구 (향후 구현)

CSS에서 JSON 테마를 자동으로 추출하는 CLI 도구:

```bash
# CSS → JSON 변환
css2theme -i layout_report.html -o corporate-blue.json

# JSON → CSS 역변환 (HTML 미리보기용)
theme2css -i corporate-blue.json -o preview.css
```

---

## 3. 아키텍처

```
┌─────────────────────────────────────────────────────────────────┐
│                        입력 레이어                               │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   Markdown      │   AUTHORS.yml   │   theme.json                │
│   (콘텐츠)       │   (메타데이터)   │   (레이아웃/스타일)          │
└────────┬────────┴────────┬────────┴────────┬────────────────────┘
         │                 │                 │
         ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    PDF 렌더링 엔진 (gopdf)                       │
│  ┌─────────────┬─────────────┬─────────────┬─────────────────┐  │
│  │  표지 렌더러  │  목차 렌더러  │  본문 렌더러  │  헤더/푸터 렌더러 │  │
│  └─────────────┴─────────────┴─────────────┴─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                        출력 (PDF)                               │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. JSON 테마 스키마 설계

### 3.1 테마 파일 구조 (`theme.json`)

```json
{
  "name": "corporate-blue",
  "version": "1.0.0",
  "description": "기업용 블루 테마",
  
  "page": {
    "size": "A4",
    "orientation": "portrait",
    "margins": {
      "top": 20,
      "right": 15,
      "bottom": 20,
      "left": 15
    }
  },
  
  "fonts": {
    "primary": {
      "family": "Pretendard",
      "path": "./fonts/Pretendard-Regular.ttf"
    },
    "heading": {
      "family": "Pretendard-Bold",
      "path": "./fonts/Pretendard-Bold.ttf"
    },
    "code": {
      "family": "JetBrainsMono",
      "path": "./fonts/JetBrainsMono-Regular.ttf"
    }
  },
  
  "colors": {
    "primary": "#0056b3",
    "secondary": "#64748b",
    "accent": "#3b82f6",
    "text": {
      "default": "#1a1a1a",
      "muted": "#666666",
      "light": "#999999"
    },
    "background": {
      "default": "#ffffff",
      "alt": "#f8fafc",
      "code": "#1e293b"
    },
    "border": "#e2e8f0"
  },
  
  "cover": {
    "enabled": true,
    "topBorder": {
      "enabled": true,
      "height": 15,
      "color": "primary"
    },
    "logo": {
      "position": { "x": "right", "y": 50 },
      "fontSize": 24,
      "color": "primary"
    },
    "title": {
      "position": { "x": 40, "y": 280 },
      "fontSize": 48,
      "color": "text.default"
    },
    "subtitle": {
      "position": { "x": 40, "y": 350 },
      "fontSize": 20,
      "color": "text.muted",
      "leftBorder": {
        "enabled": true,
        "width": 4,
        "color": "primary"
      }
    },
    "infoTable": {
      "position": { "x": 40, "y": 650 },
      "labelWidth": 100,
      "valueWidth": 415,
      "rowHeight": 35,
      "labelBackground": "background.alt",
      "fontSize": 12,
      "fields": ["발행일", "버전", "작성자", "참여자"]
    },
    "copyright": {
      "position": { "x": 40, "y": 780 },
      "fontSize": 12,
      "color": "text.light",
      "template": "© {{year}} {{copyright}}. All Rights Reserved."
    }
  },
  
  "toc": {
    "enabled": true,
    "title": {
      "text": "📋 목차",
      "fontSize": 24,
      "color": "primary"
    },
    "background": {
      "enabled": true,
      "color": "background.alt",
      "padding": 20
    },
    "item": {
      "fontSize": 14,
      "color": "primary",
      "lineHeight": 30,
      "showPageNumber": true,
      "pageNumberAlign": "right",
      "dotLeader": {
        "enabled": true,
        "char": ".",
        "spacing": 3,
        "color": "text.light"
      },
      "clickable": true
    },
    "levels": {
      "h1": { "indent": 0, "fontSize": 14, "bold": true },
      "h2": { "indent": 15, "fontSize": 13, "bold": false },
      "h3": { "indent": 30, "fontSize": 12, "bold": false }
    },
    "header": { "enabled": false },
    "footer": { "enabled": false }
  },
  
  "content": {
    "header": {
      "enabled": true,
      "height": 35,
      "text": "{{header}}",
      "fontSize": 10,
      "color": "secondary",
      "align": "left",
      "border": {
        "bottom": true,
        "color": "border"
      }
    },
    "footer": {
      "enabled": true,
      "height": 25,
      "left": {
        "text": "{{footer}}",
        "fontSize": 10,
        "color": "secondary"
      },
      "right": {
        "text": "Page {{page}}",
        "fontSize": 10,
        "color": "secondary"
      },
      "border": {
        "top": true,
        "color": "border"
      }
    },
    "heading": {
      "h1": { "fontSize": 28, "color": "primary", "marginTop": 20, "marginBottom": 15, "underline": true },
      "h2": { "fontSize": 22, "color": "primary", "marginTop": 15, "marginBottom": 10, "leftBorder": true },
      "h3": { "fontSize": 18, "color": "text.default", "marginTop": 12, "marginBottom": 8 },
      "h4": { "fontSize": 14, "color": "text.default", "marginTop": 10, "marginBottom": 6 }
    },
    "paragraph": {
      "fontSize": 12,
      "lineHeight": 1.6,
      "color": "text.default"
    },
    "code": {
      "inline": {
        "fontSize": 11,
        "fontFamily": "code",
        "backgroundColor": "#f1f5f9",
        "padding": { "horizontal": 4, "vertical": 2 },
        "borderRadius": 4,
        "color": "#e11d48"
      },
      "block": {
        "fontSize": 11,
        "fontFamily": "code",
        "backgroundColor": "background.code",
        "color": "#e2e8f0",
        "padding": 15,
        "borderRadius": 8,
        "lineNumbers": true
      }
    },
    "table": {
      "headerBackground": "primary",
      "headerColor": "#ffffff",
      "borderColor": "border",
      "cellPadding": { "horizontal": 12, "vertical": 8 },
      "altRowBackground": "background.alt"
    },
    "list": {
      "bullet": { "fontSize": 12, "indent": 20, "spacing": 6 },
      "numbered": { "fontSize": 12, "indent": 20, "spacing": 6 }
    },
    "links": {
      "internal": {
        "color": "primary",
        "underline": false,
        "clickable": true
      },
      "external": {
        "color": "accent",
        "underline": true,
        "clickable": true
      }
    }
  }
}
```

---

## 5. 마크다운 내부 링크 구현

### 4.1 지원 범위

마크다운 내부 링크 기능은 **md2html**과 **md2pdf** 모두에 적용됩니다.

| 링크 유형 | 마크다운 문법 | HTML 출력 | PDF 출력 |
|----------|-------------|----------|----------|
| 내부 앵커 | `[소개](#소개)` | `<a href="#소개">` | PDF 내부 링크 |
| 섹션 참조 | `[2장 참조](#2-설치-및-설정)` | HTML 앵커 링크 | PDF 페이지 점프 |
| 외부 URL | `[공식 문서](https://...)` | `<a href="https://..." target="_blank">` | PDF 외부 링크 |
| 이미지 링크 | `![alt](image.png)` | `<img>` 태그 | 이미지 임베딩 |

### 4.2 구현 상세

#### 4.2.1 앵커 ID 생성 규칙

```
입력: "## 1. 설치 및 설정"
출력 ID: "1-설치-및-설정"

규칙:
1. 헤딩 텍스트에서 특수문자 제거 (##, *, _ 등)
2. 공백 → 하이픈(-) 변환
3. 연속 하이픈 제거
4. 소문자 변환 (선택적)
5. 중복 ID는 -1, -2 등 접미사 추가
```

#### 4.2.2 md2html 내부 링크 처리

```go
// 헤딩 파싱 시 앵커 ID 생성
func generateAnchorID(heading string) string {
    // 마크다운 기호 제거
    id := regexp.MustCompile(`[#*_\[\]()]`).ReplaceAllString(heading, "")
    // 공백 → 하이픈
    id = strings.ReplaceAll(strings.TrimSpace(id), " ", "-")
    // 연속 하이픈 제거
    id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
    return id
}

// HTML 출력
<h2 id="1-설치-및-설정">1. 설치 및 설정</h2>

// 링크 변환
[설치 가이드](#1-설치-및-설정)  →  <a href="#1-설치-및-설정">설치 가이드</a>
```

#### 4.2.3 gopdf 내부 링크 처리

```go
// gopdf는 PDF 내부 링크 지원
func (r *Renderer) addInternalLink(text string, targetID string, x, y float64) {
    // 텍스트 렌더링
    r.pdf.SetX(x)
    r.pdf.SetY(y)
    r.pdf.SetTextColor(0, 86, 179) // primary color
    r.pdf.Cell(nil, text)
    
    // 내부 링크 영역 설정
    textWidth, _ := r.pdf.MeasureTextWidth(text)
    r.pdf.AddInternalLink(targetID, x, y, textWidth, 14)
}

// 앵커 등록
func (r *Renderer) registerAnchor(id string, page int, y float64) {
    r.anchors[id] = AnchorInfo{Page: page, Y: y}
}
```

### 4.3 목차(TOC) 링크 연동

목차의 각 항목은 해당 섹션으로 **클릭 이동** 가능:

```
┌─────────────────────────────────────────────────────────────┐
│  📋 목차                                                     │
├─────────────────────────────────────────────────────────────┤
│  소개 .................................................. 3   │  ← 클릭 시 3페이지로 이동
│  1. 설치 및 설정 ....................................... 5   │  ← 클릭 시 5페이지로 이동
│     1.1 시스템 요구사항 ................................ 5   │
│     1.2 다운로드 및 설치 ............................... 6   │
│  2. 기본 사용법 ........................................ 8   │
│  3. 주요 명령어 ....................................... 12   │
└─────────────────────────────────────────────────────────────┘
```

**도트 리더(Dot Leader)** 구현:

```go
func (r *Renderer) drawTOCItem(title string, pageNum int, y float64) {
    leftX := 70.0
    rightX := 530.0
    
    // 제목 출력
    r.pdf.SetX(leftX)
    r.pdf.SetY(y)
    r.pdf.Cell(nil, title)
    titleWidth, _ := r.pdf.MeasureTextWidth(title)
    
    // 도트 리더 그리기
    dotStartX := leftX + titleWidth + 5
    pageNumStr := fmt.Sprintf("%d", pageNum)
    pageNumWidth, _ := r.pdf.MeasureTextWidth(pageNumStr)
    dotEndX := rightX - pageNumWidth - 5
    
    r.pdf.SetTextColor(153, 153, 153) // text.light
    for x := dotStartX; x < dotEndX; x += 6 {
        r.pdf.SetX(x)
        r.pdf.SetY(y)
        r.pdf.Cell(nil, ".")
    }
    
    // 페이지 번호 (우측 정렬)
    r.pdf.SetTextColor(0, 86, 179)
    r.pdf.SetX(rightX - pageNumWidth)
    r.pdf.SetY(y)
    r.pdf.Cell(nil, pageNumStr)
    
    // 클릭 영역 등록 (전체 행)
    r.pdf.AddInternalLink(title, leftX, y, rightX-leftX, 14)
}
```

## 6. 사전 정의 테마 예시

### 4.1 기본 제공 테마

| 테마명 | 설명 | 용도 |
|-------|------|------|
| `corporate-blue` | 기업용 블루 테마 | 공식 문서, 사용자 매뉴얼 |
| `corporate-dark` | 다크 모드 테마 | 개발자 문서, API 레퍼런스 |
| `minimal-clean` | 미니멀 화이트 | 제안서, 보고서 |
| `technical-mono` | 기술 문서용 모노톤 | 기술 사양서, 설계 문서 |
| `vibrant-modern` | 모던 컬러풀 | 마케팅 자료, 소개서 |

### 4.2 테마 전환 CLI 사용법

```bash
# 기본 테마 사용
md2pdf -i docs/manual -o USER_MANUAL.pdf --theme corporate-blue

# 커스텀 테마 파일 지정
md2pdf -i docs/manual -o USER_MANUAL.pdf --theme ./themes/custom.json

# 테마 목록 확인
md2pdf --list-themes

# 테마 검증
md2pdf --validate-theme ./themes/custom.json
```

---

## 7. 구현 계획

### 5.1 단계별 구현

| 단계 | 작업 | 예상 기간 |
|:---:|------|:-------:|
| **1** | 테마 JSON 스키마 정의 및 파서 구현 | 2일 |
| **2** | gopdf 기반 렌더러 구현 (표지, 목차, 본문) | 3일 |
| **3** | 마크다운 파서 → gopdf 렌더링 연동 | 3일 |
| **4** | 헤더/푸터 렌더러 구현 | 1일 |
| **5** | **목차 도트 리더 + 페이지 번호 + 클릭 링크** | 1일 |
| **6** | **마크다운 내부 링크 → PDF/HTML 변환** | 1일 |
| **7** | 기본 테마 5종 제작 | 2일 |
| **8** | CLI 통합 및 테스트 | 2일 |
| **총계** | | **15일** |

### 5.2 파일 구조

```
tools/
├── md2pdf/
│   ├── main.go              # CLI 진입점
│   ├── parser/
│   │   ├── markdown.go      # 마크다운 파서
│   │   ├── links.go         # 내부/외부 링크 처리
│   │   └── theme.go         # 테마 JSON 파서
│   ├── renderer/
│   │   ├── engine.go        # gopdf 렌더링 엔진
│   │   ├── cover.go         # 표지 렌더러
│   │   ├── toc.go           # 목차 렌더러 (도트 리더 + 링크)
│   │   ├── content.go       # 본문 렌더러
│   │   ├── links.go         # PDF 내부/외부 링크 렌더러
│   │   └── header_footer.go # 헤더/푸터 렌더러
│   └── themes/
│       ├── corporate-blue.json
│       ├── corporate-dark.json
│       ├── minimal-clean.json
│       ├── technical-mono.json
│       └── vibrant-modern.json
└── go.mod
```

---

## 8. 기대 효과

### 6.1 장점

| 항목 | 현재 (HTML/CSS) | 제안 (JSON 테마) |
|------|:--------------:|:---------------:|
| 헤더/푸터 | ❌ 미지원 | ✅ 완벽 지원 |
| 테마 전환 | HTML 템플릿 수정 필요 | JSON 파일 교체만 |
| 페이지 번호 | 제한적 | ✅ 완벽 제어 |
| **목차 페이지 번호** | ❌ 수동 입력 | ✅ 자동 생성 + 도트 리더 |
| **내부 링크** | ⚠️ HTML만 | ✅ HTML + PDF 모두 |
| 외부 의존성 | Chrome 필요 | 순수 Go (없음) |
| 빌드 속도 | 느림 (Chrome 실행) | 빠름 (네이티브) |
| 커스터마이징 | CSS 지식 필요 | JSON 편집만으로 가능 |

### 6.2 확장성

- **다국어 지원**: 폰트 경로를 테마에서 정의하여 CJK 폰트 쉽게 전환
- **브랜드 가이드라인**: 회사별 테마 파일로 브랜딩 일관성 유지
- **테마 마켓플레이스**: 커뮤니티 테마 공유 가능

---

## 9. 결론

**gopdf + JSON 테마 시스템**을 통해:

1. ✅ Chrome 의존성 제거 → 순수 Go 바이너리
2. ✅ 완전한 헤더/푸터/페이지 번호 지원
3. ✅ 테마 전환으로 다양한 문서 디자인 지원
4. ✅ 빠른 PDF 생성 속도
5. ✅ 비개발자도 JSON 편집으로 디자인 커스터마이징 가능
6. ✅ **목차 자동 페이지 번호 + 도트 리더 + 클릭 링크**
7. ✅ **마크다운 내부 링크 → PDF/HTML 완벽 지원**

**권장**: 이 제안을 채택하여 차세대 PDF 생성 시스템 구축

---

## 부록: 참고 자료

- gopdf 공식 문서: https://github.com/signintech/gopdf
- JSON Schema 표준: https://json-schema.org/
- gopdf PoC 결과: `d:\wdata\dev\tools\gopdf_poc\gopdf_poc.pdf`
