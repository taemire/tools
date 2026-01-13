# PDF 페이지 번호 문제 해결 로그 (Troubleshooting Log)
**날짜**: 2026-01-13
**주제**: PDF 생성 시 "목차(TOC)가 ii로, 본문이 3으로 시작하는 문제" 해결

## 1. 문제 상황 (The Problem)
전문적인 리포트 형식을 갖추기 위해 다음과 같은 페이지 번호 체계가 필요했습니다:
1.  **표지 (Cover)**: 페이지 번호 없음.
2.  **목차 (TOC)**: 로마자 표기, **i**부터 시작.
3.  **본문 (Main Content)**: 아라비아 숫자 표기, **1**부터 시작.

**초기 상태**:
- PDF 렌더러(Chrome/wkhtmltopdf 등 `html2pdf` 하부 엔진)가 문서를 하나의 연속된 흐름으로 인식함.
- 표지(1p) -> 목차(2p) -> 본문(3p) 순으로 물리적 페이지가 매겨짐.
- **결과**: 목차는 **ii** (2페이지), 본문은 **3** (3페이지)으로 표시됨. 리셋 로직이 작동하지 않음.

## 2. 실패한 시도들 (Failed Attempts)

### 시도 1: 표준 CSS 카운터 리셋 (Standard CSS Counter Reset)
`.frontmatter`와 `.mainmatter` 래퍼에 `counter-reset: page 1;`을 적용.
```css
.frontmatter { counter-reset: page 1; }
.mainmatter { counter-reset: page 1; }
```
**결과**: 렌더러가 이를 무시함. 물리적 페이지 번호(1, 2, 3...)가 그대로 푸터에 출력됨.

### 시도 2: `pdf_analyzer` 오프셋 조정 (Offset Adjustment)
Go 툴을 수정하여 목차 링크 생성 시 표지/목차 페이지 수를 차감하도록 함.
**결과**: 목차 텍스트의 *하이퍼링크*는 정상적으로 "1"을 가리켰으나, **실제 페이지 푸터**에는 여전히 "3"이 찍혀있어 시각적 불일치 발생.

### 시도 3: 단일 커스텀 카운터 (`visible-page`)
`visible-page`라는 커스텀 카운터를 정의하고 `@page`에서 증가, 래퍼에서 리셋 시도.
```css
@page { counter-increment: visible-page; }
.mainmatter { counter-reset: visible-page 1; }
```
**결과**: 절반의 실패. 목차는 'i'로 시작했으나, 본문이 리셋되지 않고 '2'로 이어짐 (목차 다음 페이지로 인식). `.mainmatter`의 리셋 규칙이 페이지 흐름 상 너무 늦게 적용되거나 무시됨.

## 3. 해결책: 카운터 격리 전략 (The Solution: Isolated Counters Strategy)

글로벌 카운터를 "리셋"하려는 접근을 버리고, **각 영역별로 서로 다른 카운터를 사용하는 방식**으로 전환하여 해결했습니다.

### 구현 내용
1.  **두 개의 카운터 정의**: `page-toc` (목차용)와 `page-main` (본문용).
2.  **스코프별 증가 로직 (Scope-Specific Increment)**:
    *   `@page frontmatter`에서는 오직 `page-toc`만 증가시킴.
    *   `@page main`에서는 오직 `page-main`만 증가시킴.
3.  **결과**:
    *   목차 진입 시: `page-toc`가 0에서 시작 -> 1 (**i**)로 증가.
    *   본문 진입 시: `page-main`이 0에서 시작 -> 1 (**1**)로 증가.
    *   표지: `counter-increment: none` 적용하여 카운트 제외.

```css
/* Isolated Counters Strategy (카운터 격리 전략) */
@page frontmatter {
    counter-increment: page-toc;
    @bottom-right { content: counter(page-toc, lower-roman); }
}

@page main {
    counter-increment: page-main;
    @bottom-right { content: counter(page-main); }
}
```

## 4. 교훈 (Lesson Learned)
Paged Media CSS (특히 Headless Browser 렌더러 환경)에서는 `div`와 같은 **DOM 요소에 카운터 리셋을 걸어도 페이지 컨텍스트(헤더/푸터)에는 영향을 주지 못하는 경우**가 많습니다.
가장 확실한 방법은 `@page` 컨텍스트 내부에서 **증가(increment) 로직 자체를 제어**하고, `page: name` 속성을 통해 컨텍스트를 완전히 분리하는 것입니다.
