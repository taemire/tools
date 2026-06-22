# md2pdf TOC 매핑 연구 — 동일 H2 제목 충돌

> 날짜: 2026-05-22 · 실제 다중 문서 PDF 컴파일 사용 사례 기반 · Go 소스 분석 + Go PDF 생태계 비교

## 0. 목적

이 문서는 `md2pdf`로 여러 Markdown 보고서를 하나의 PDF로 컴파일할 때 발견된 **목차 페이지 매핑 오류**를 분석한다. 내부 개선 로드맵으로 활용할 수 있도록 원인 분석, 비교를 포함한 **7가지 후보 해결책**, **우선순위 Top 3 권고안**, **Go 소스 수정 제안**을 정리한다.

## 1. 문제 진단 — 근본 원인

### 1.1 증상 (실측, 2026-05-22)

각각 "0. Document Purpose", "9. References" 같은 동일한 H2 섹션으로 시작하는 독립 보고서 7개를 하나의 PDF로 합쳤을 때:

| 섹션 | 기대 페이지 | **잘못 매핑된 페이지** |
|---|---|---|
| 보고서 A — `0. Document Purpose` | p.1 | ❌ **p.29** |
| 보고서 B — `0. Document Purpose` | p.7 | ❌ **p.29** |
| 보고서 C — `0. Document Purpose` | p.14 | ❌ **p.29** |
| 보고서 D — `0. Document Purpose` | p.17 | ❌ **p.29** |
| 보고서 E — `0. Document Purpose` | p.21 | ❌ **p.29** |
| 보고서 F — `0. Document Purpose` | p.25 | ❌ **p.29** |
| 보고서 G — `0. Document Purpose` | p.29 | ✅ p.29 (마지막 항목만 올바름) |
| 보고서 B — `8. EP-XX Update` | p.10 | ❌ **p.20** |
| 보고서 B — `9. References` | p.13 | ❌ **p.28** |

→ 여러 하위 문서에 **동일한 H2 제목 텍스트**가 등장하면 목차가 **모든 중복 항목을 마지막 등장 위치의 페이지**에 바인딩한다. 즉 Z-order 기준 마지막 쓰기 값이 승리한다.

### 1.2 근본 원인 — Go 소스 분석

**`analyzer/analyzer.go:113`** — 매핑 로직:

```go
// 본문 페이지에서 검색
startPage := actualSkipPages + 1
lastFoundPage := startPage
for i := range sections {
    found := false
    for pageNum := lastFoundPage; pageNum <= totalPages; pageNum++ {
        // ...
        if containsTitle(text, sections[i].Title) {
            docPageNum := pageNum - actualSkipPages
            sections[i].Page = docPageNum  // ← 반복마다 설정
            lastFoundPage = pageNum         // ← 단조 증가
            found = true
            break
        }
    }
}
```

**세 가지 문제**:

| # | 문제 | 영향 |
|---|---|---|
| 1 | **`containsTitle` 텍스트 기반 포함 검사** — 고유 앵커가 없음 | 같은 제목이 문자열 매칭에서 충돌 |
| 2 | **`lastFoundPage` 단조 증가** — 이전 페이지로 되돌아갈 수 없음 | 보고서 G 섹션이 처리된 뒤 이전 섹션을 다시 스캔할 수 없음 |
| 3 | **렌더러가 `map[title]page` 조회 사용** — sections 배열의 *고유 ID*를 사용하지 않음 | 마지막 반복 값이 같은 제목의 이전 항목을 모두 덮어써서 목차가 같은 최종 페이지로 렌더링됨 |

→ **근본 원인 = 렌더러가 sections 배열의 고유 ID를 무시하고 `title → page` 맵으로 조회하는 것**이다.

## 2. 일곱 가지 후보 해결책

### 해결책 1: 앵커 기반 매핑 (권장 ★★★)

각 Markdown heading에 *고유 HTML 앵커*를 부여한다. PDF 링크 annotation을 통해 anchor → page를 매핑한다.

```go
// renderer.go
heading_id := fmt.Sprintf("%s__%s", file_id, slug(heading_text))
// HTML: <h2 id="01-report-A__0-document-purpose">0. Document Purpose</h2>
```

**장점**:
- 고유 앵커 — 중복 heading 텍스트가 file_id prefix로 고유해짐
- PDF 링크 annotation을 자연스럽게 활용
- Markdown 표준 관행과 맞음 (GitHub heading anchors)

**단점**:
- analyzer가 텍스트 매칭 대신 PDF에서 앵커를 읽어야 함
- TOC 렌더링이 title → page 대신 anchor → page 조회를 사용해야 함

**예상 작업량**: Go 소스 약 50~100줄.

### 해결책 2: PDF Outline / Bookmark API (강력 권장 ★★★★)

Chromium / wkhtmltopdf의 *PDF outline(bookmark tree)*를 사용한다. `pdfcpu` 또는 `unipdf`로 outline을 직접 추출해 텍스트 매칭을 완전히 피한다.

```go
// converter.go
chromium_args = "--enable-pdf-outline --pdf-outline-headers=h1,h2,h3"

// analyzer.go (대체 구현)
import "github.com/pdfcpu/pdfcpu/pkg/api"
outline, _ := api.ExtractBookmarksFile(pdfPath, ...)
// outline 자체가 정확한 anchor → page 매핑이다
```

**장점**:
- 텍스트 매칭을 완전히 제거 — outline tree가 권위 있는 원천
- PDF 표준 — 모든 PDF 리더가 outline을 인식
- 외부 `pdfinfo` / `pdfgrep` 도구 의존성을 제거

**단점**:
- Chromium의 `--enable-pdf-outline` 지원 여부 검증 필요
- `pdfcpu` 또는 `unipdf` 의존성 추가

**예상 작업량**: 약 100~200줄.

### 해결책 3: 파일 인지 섹션 추적 (단순 ★★)

파일별 namespace를 부여한다. 같은 H2라도 `(file_name, heading_text)` 복합 키로 처리한다.

```go
// analyzer.go
type SectionKey struct {
    FileName string
    Title    string
}
sectionMap := make(map[SectionKey]int)  // (file, title) → page

// 또는 더 단순하게
type SectionInput struct {
    ID        string
    Title     string
    FileName  string  // ← 새 필드
    Page      int
}
```

**장점**:
- 최소 코드 변경 — 맵 키만 변경
- 사용자가 수동 prefix를 넣을 필요 없음
- 파일 출처가 자연스럽게 고유성을 제공

**단점**:
- TOC 렌더링에서 파일명을 표시해야 할 수 있음 (현재는 H1 표시가 유지됨)
- 앵커 기반 navigation은 별도 과제

**예상 작업량**: 약 30~50줄.

### 해결책 4: Heading Slug 자동 고유화 (단순 ★★★)

GitHub 방식 heading slug를 사용한다. 같은 텍스트가 두 번째 이상 등장하면 `slug-1`, `slug-2`처럼 suffix를 붙인다.

```go
// renderer.go
slugCounts := make(map[string]int)
for _, heading := range allHeadings {
    baseSlug := slug(heading.Text)
    if count, exists := slugCounts[baseSlug]; exists {
        slugCounts[baseSlug] = count + 1
        heading.Slug = fmt.Sprintf("%s-%d", baseSlug, count)
    } else {
        slugCounts[baseSlug] = 1
        heading.Slug = baseSlug
    }
}
```

**장점**:
- 표준 Markdown-to-HTML 패턴 (GitHub, GitLab)
- 사용자가 이해하기 쉬운 모델

**단점**:
- 제목 표시는 그대로이므로 slug만 고유함 — TOC에는 `0. Document Purpose`가 7번 동일하게 보임
- PDF 텍스트 매칭이 여전히 title 기반이면 충돌이 남음 — 해결책 1과 결합해야 함

**예상 작업량**: 약 30줄.

### 해결책 5: One-Pass + Native PDF Links (혁신적 ★★★★)

기존 *2-pass + 텍스트 매칭*을 *1-pass + HTML anchor → PDF link annotation*으로 교체한다. TOC는 내부 링크(`href="#A0-..."`)를 사용하며, 별도 페이지 번호 계산이 필요 없다.

```go
// converter.go
// HTML에 포함: <a href="#section-A-0">A.0. Document Purpose ............. [link]</a>
// PDF: link annotation이 클릭 가능한 링크로 렌더링됨
// TOC 페이지 번호는 생략하거나 링크 대상의 resolved page에서 파생 가능
```

**장점**:
- Pass 2 제거 — 빌드 시간 약 50% 단축
- 클릭 가능한 TOC (대상 페이지로 이동)
- 텍스트 매칭 제거 — 구조적으로 올바름

**단점**:
- TOC 항목 옆 페이지 번호(예: `.... 3`)가 자동 렌더링되지 않음 — 각 링크의 대상 페이지를 추출해 주입하는 후처리 단계 필요
- Chromium PDF link 옵션 검증 필요

**예상 작업량**: 약 150~200줄 + Chromium 옵션 검증.

### 해결책 6: pdfcpu Bookmark Tree (혁신적 ★★★)

PDF 생성 후 `pdfcpu`의 `AddBookmarks` API를 사용해 Markdown headings → PDF bookmark tree를 자동 매핑한다.

```go
import "github.com/pdfcpu/pdfcpu/pkg/api"

// PDF 생성 후
bookmarks := []*pdfcpu.Bookmark{...}  // sections + page mapping으로 생성
api.AddBookmarksFile(pdfPath, bookmarks, ...)
```

**장점**:
- PDF 리더의 outline 패널 활성화
- 텍스트 매칭 제거
- Markdown TOC와 PDF outline 공존 가능

**단점**:
- 두 TOC 원천(Markdown TOC vs PDF outline)을 동기화해야 함
- `pdfcpu` 의존성 추가

**예상 작업량**: 약 100줄.

### 해결책 7: Sidebar 인지 다중 파일 섹션 ID

`_sidebar.md`의 파일 순서를 사용해 *section ID*에 파일 index(0..N)를 자동 prefix로 붙인다.

```go
// renderer.go
for fileIdx, mdFile := range sidebarFiles {
    for headingIdx, heading := range parseHeadings(mdFile) {
        section.ID = fmt.Sprintf("file%d-heading%d", fileIdx, headingIdx)
        section.DisplayTitle = heading.Text  // 사용자에게 보이는 제목은 유지
    }
}
```

**장점**:
- md2pdf가 이미 인식하는 `_sidebar.md` 재사용
- 파일 index를 통해 section ID가 자연스럽게 고유해짐
- 제목 표시는 변경 없음

**단점**:
- `_sidebar.md`가 없으면 취약함 (디렉터리 정렬로 fallback)
- 단일 파일 사용에서는 효과가 작음

**예상 작업량**: 약 50줄.

## 3. 우선순위 Top 3

| 순위 | 해결책 | 점수 | 이유 |
|---|---|---|---|
| 🥇 1 | **#2 PDF Outline / Bookmark API** (★★★★) | 9/10 | 외부 라이브러리(`pdfcpu`)를 통해 텍스트 매칭을 피함. PDF 표준 사용 — 장기 안정성 높음 |
| 🥈 2 | **#5 One-Pass + Native PDF Links** (★★★★) | 9/10 | 빌드 시간 단축 + 클릭 가능한 TOC. 페이지 번호 표시는 보완 처리 필요 |
| 🥉 3 | **#1 앵커 기반 매핑** (★★★) | 8/10 | 최소 소스 변경 + Markdown 표준. 즉시 적용 가능 |

### 통합 로드맵

**Phase 1 (즉시)**: 해결책 **#3 + #4** — 파일 인지 sections + slug 고유화 — 30~80줄. 현재 사용자가 적용하는 수동 prefix workaround를 영구적으로 해소한다.

**Phase 2 (중기)**: 해결책 **#1 + #6** — Anchor + pdfcpu bookmark — 약 200줄 + PDF outline 사용.

**Phase 3 (장기)**: 해결책 **#2 + #5** — PDF Outline + Native Links — 전체 아키텍처 재설계.

## 4. 수동 Workaround vs. 영구 수정

현재 영향을 받는 사용자는 workaround로 H2에 수동 prefix를 적용한다(예: `sed 's/^## /## A./'`). 영구 수정(Phase 1)과 비교하면 다음과 같다.

| 관점 | 현재 방식 (수동 prefix) | **영구 수정 (Phase 1)** |
|---|---|---|
| 사용자 부담 | 각 H2에 `sed`로 prefix 추가 | 자동 |
| TOC 가독성 | "A.0. Document Purpose" (덜 깔끔함) | "0. Document Purpose" (깔끔함) |
| 다중 문서 통합 | 매번 prefix 재적용 | 자동 |
| 도구 재사용성 | 프로젝트별 workaround | 전역적으로 사용 가능 |
| 유지보수 비용 | 변환마다 검토 필요 | 0 |

→ **Phase 1 영구 수정**은 출력 품질과 운영 효율 양쪽 모두에서 더 낫다.

## 5. Go 소스 변경 제안 (Phase 1)

### 5.1 `analyzer/analyzer.go`

```go
type SectionInput struct {
    ID          string
    Title       string
    FileName    string  // ← 신규 (sidebar 또는 markdown 파일명)
    AnchorSlug  string  // ← 신규 (고유 slug)
    SubHeadings []SubHeading
}

type SubHeading struct {
    ID         string
    Title      string
    AnchorSlug string  // ← 신규 고유 slug (예: "file01__0-document-purpose")
    Page       int
}

// containsTitle을 containsAnchor 또는 (file_id, title) 복합 검색으로 대체
func containsSectionUnique(text, fileName, title string) bool {
    // PDF 텍스트에서 *file marker*와 *title*을 동시에 검색
    // 또는 *anchor link*를 검색 (해결책 1)
}
```

### 5.2 `renderer/renderer.go`

```go
type Heading struct {
    Title       string
    FileID      string  // ← 신규
    AnchorSlug  string  // ← 신규
    Page        int
}

// TOC 렌더링은 heading.AnchorSlug → Page 사용
// 기존 heading.Title → Page 맵 충돌을 해소
tocEntries := map[string]int{}
for _, h := range allHeadings {
    tocEntries[h.AnchorSlug] = h.Page  // 고유 키
}
// 조회에는 anchor slug를 사용하고, title은 표시 전용으로 둔다
```

### 5.3 `main.go`

```go
// 신규 CLI flags
flag.Bool("unique-anchor", true, "앵커 slug에 file_id prefix를 자동 추가")
flag.Bool("anchor-based-lookup", true, "제목 텍스트 매칭 대신 앵커 기반 페이지 조회 사용")
```

### 5.4 예상 PR 규모

| 파일 | 변경 줄 수 |
|---|---|
| `analyzer/analyzer.go` | 약 50 |
| `renderer/renderer.go` | 약 80 |
| `converter/converter.go` | 약 30 |
| `main.go` | 약 20 |
| 신규 테스트 (`*_test.go`) | 약 100 |
| **합계** | **약 280줄** |

## 6. 검증 — 실제 사용 사례

이 TOC 매핑 버그는 영업 자료용으로 **EDR 벤더 비교 보고서 7개**를 하나의 PDF로 컴파일하는 과정에서 발견되었다. 각 보고서는 독립적으로 "0. Document Purpose", "9. References" 같은 동일한 H2 섹션을 사용했다. 약 80개 H2 항목 중 약 25개가 제목 충돌 때문에 잘못 매핑되었다. 수동 workaround(A./B./C. prefix 추가)를 적용하자 TOC 정확도가 회복되었다.

이는 여러 개의 동일 구조 하위 보고서를 합성하는 문서의 대표 사례다. 벤더 비교, 다중 제품 분석, 통합 분기 보고서에서 흔히 나타나는 패턴이다.

## 7. 다음 단계 후보

- (a) Phase 1 PR — `analyzer.go` + `renderer.go` 파일 인지 + slug 고유화 (약 80줄)
- (b) `pdfcpu` 통합 PoC (Phase 2 해결책 #6)
- (c) Chromium `--enable-pdf-outline` 검증 PoC (Phase 2 해결책 #2)
- (d) 기존 사용자 문서 업데이트 — 수동 workaround를 전환기 방식으로 설명

## 8. 참고자료

### md2pdf 소스

- `main.go`
- `analyzer/analyzer.go` (특히 `:113` `containsTitle`)
- `renderer/renderer.go`
- `converter/converter.go`

### Go PDF 라이브러리

- pdfcpu — https://github.com/pdfcpu/pdfcpu
- unipdf — https://github.com/unidoc/unipdf
- go-pdf/fpdf — https://github.com/jung-kurt/gofpdf

### Markdown-to-PDF 도구 (비교)

- mdpdf (Node.js, Puppeteer) — HTML anchor + PDF link
- pandoc — `--pdf-engine=xelatex` cross-references
- md-to-pdf (npm) — Puppeteer + Chromium PDF link
- mkdocs-pdf-export-plugin — section-aware page numbering

### PDF 표준

- ISO 32000-1:2008 — PDF Outline / Bookmark Tree
- ISO 32000-2:2020 — PDF 2.0 link annotations
