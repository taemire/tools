# 프로젝트 히스토리 (Project History)

이 문서는 `tools` 프로젝트의 주요 작업, 의사결정, 설계 변경 사항을 시간순으로 기록합니다.

---

## 2026-01-22: MD 확장 구문 통합 지원

### 배경
- **요구사항**: GitHub-style Alerts(`> [!CAUTION]`)가 일반 blockquote로 렌더링되는 문제 해결 필요
- **목표**: 다양한 MD 확장 구문(GitHub, Docusaurus, Obsidian 등)을 통합 지원

### 작업 내용

#### Phase 1: Callouts/Admonitions
- **전처리 로직 확장**: `preprocessAlerts` 함수로 Docsify(`!>`, `?>`) 및 Docusaurus(`:::type`) 구문을 GFM Alert 형식으로 통합
  - `:::note`, `:::tip`, `:::info`, `:::warning`, `:::danger` → `> [!TYPE]` 변환
  - 커스텀 제목 지원: `:::note[제목]` → `> [!NOTE] **제목**`
- **후처리 로직 확장**: `postProcessAlerts` 함수에서 5가지 Alert 타입 처리
  - NOTE, TIP, IMPORTANT, WARNING, CAUTION
  - 각 타입별 CSS 클래스 및 Font Awesome 아이콘 매핑

#### Phase 2: Highlight & Emoji
- **Highlight**: `preprocessHighlight` 함수 추가
  - `==텍스트==` → `<mark>텍스트</mark>` 변환
- **Emoji**: `preprocessEmoji` 함수 추가
  - `:emoji:` 단축코드 → Unicode 이모지 변환
  - 40개+ 이모지 매핑 테이블 (일반, 상태, 감정, 개발, 화살표 카테고리)

#### Phase 3: Footnotes & Definition Lists
- **Goldmark 확장 활성화**:
  - `extension.Footnote`: `[^1]` 각주 구문 지원
  - `extension.DefinitionList`: `용어 : 정의` 구문 지원
- **CSS 스타일 추가**: `.footnotes`, `dl`, `dt`, `dd` 스타일링

### 관련 파일
- `md2html_v2/main.go`: `preprocessAlerts`, `preprocessHighlight`, `preprocessEmoji`, `postProcessAlerts` 함수
- `md2html_v2/templates/layout_modern.html`: CSS 스타일 추가
- `md2html_v2/templates/layout_report.html`: CSS 스타일 추가
- `docs/MD_EXTENDED_SYNTAX.md`: 지원 구문 종합 문서

---

## 2026-01-19: revlog.bat 태그 출력 가독성 개선

### 배경
- **이슈**: 태그명(`0.4.19.125`)이 길어질 경우, `revlog.bat` 출력 시 컬럼 폭(10자) 제한으로 인해 끝부분이 `...`이 아닌 `.`으로 불분명하게 잘리거나 짤림.
- **요구**: 태그 전체 이름을 온전히 보거나, 적절히 가독성 있게 조절 필요.

### 작업 내용
- **동적 너비 계산**: `git log` 결과를 파싱하여 조회된 커밋들 중 **가장 긴 태그의 길이**를 계산.
- **컬럼 자동 조절**: 계산된 최대 길이에 맞춰 `Tag` 컬럼 너비와 구분선(`---`) 길이를 동적으로 생성.
- **PowerShell 최적화**: `git log` 호출 시 `%D`(Ref names) 옵션을 사용하여 별도 `git tag` 호출 없이 태그 정보를 한 번에 추출.

## 2026-01-13: PDF 페이지 번호 체계 개선 (Isolated Counter Strategy)

### 배경
- **이슈**: PDF 생성 시 목차(TOC)가 2페이지(ii)로 표시되고, 본문이 3페이지(3)로 표시되는 문제 발생 (연속된 물리 페이지 번호가 노출됨).
- **원인**: Headless Browser 기반 렌더러가 `div`와 같은 DOM 요소 상의 `counter-reset`을 무시하고 물리적 페이지 흐름을 유지함.

### 해결 방안
- **카운터 격리 전략 (Isolated Counter Strategy)** 도입:
  1.  단일 `page` 카운터를 폐기하고, **`page-toc`**와 **`page-main`**으로 분리.
  2.  `@page frontmatter` 스코프에서는 `page-toc`만 증가.
  3.  `@page main` 스코프에서는 `page-main`만 증가.
  4.  각 섹션 진입 시 해당 카운터가 자연스럽게 1부터 시작되도록 유도.

### 결과
- **목차**: 로마자 **i**부터 시작 (물리 2페이지라도 논리 1페이지).
- **본문**: 아라비아 숫자 **1**부터 시작 (물리 3페이지라도 논리 1페이지).
- **산출물**: `d:/wdata/dev/tools/docs/PDF_PAGE_NUMBERING_TROUBLESHOOTING.md` (기술 회고록) 작성.

---

## 2025-12-25: pdf_analyzer 목차 자동 감지 기능 구현

### 배경
- **이슈**: `md2pdf_v2.bat`의 `SKIP_PAGES`가 3으로 하드코딩되어 있어, 목차가 정확히 2페이지가 아닌 경우 본문 섹션 페이지 번호 계산에 오류 발생
- **영향**:
  - 목차가 1페이지인 경우: 본문 1페이지(전체 3p)의 섹션을 인식하지 못하고 건너뜀
  - 목차가 3페이지 이상인 경우: 목차 내용을 본문 섹션으로 오인할 위험

### 해결 방안 (v0.1.1)
단순히 첫 번째 섹션을 찾는 방식에서 벗어나, **Heuristic 분석**을 통해 목차 페이지와 본문 페이지를 명확히 구분:

1. **isBodyPage() 분석**:
   - **섹션 밀도(Section Density)**: 한 페이지에 너무 많은 섹션 제목이 있으면 목차로 판단 (정상 본문은 5개 미만)
   - **텍스트-섹션 비율**: 섹션 제목 1개당 본문 텍스트가 일정 길이(150자) 이상이어야 본문으로 판단
   - **점선 패턴(Dot Leader)**: `......` 등의 패턴이 많으면 목차로 판단
   - **최소 텍스트 길이**: 일정 길이(700자) 이상의 실질 데이터가 본문 페이지에 있어야 함

2. **동적 스킵 계산**:
   - 페이지 2부터 스캔하여 `isBodyPage`가 `true`가 될 때까지를 목차로 인식
   - 자동 감지 실패 시 기본값 3(표지 1+목차 2)으로 폴백

### 테스트 결과 (v0.1.1)
- **TKCLI 사용자 매뉴얼**: 
  - 물리 페이지 2(목차): 섹션 29개 발견 → `isBodyPage=false` (목차로 건너뜀)
  - 물리 페이지 4(본문): 섹션 1개 + 풍부한 본문 → `isBodyPage=true` (본문 시작점으로 인식)
  - 모든 섹션의 페이지 번호가 실제 위치에 맞게 정확히 매핑됨 ✅

### 관련 파일
- `md2html_v2/cmd/pdf_analyzer/main.go`: 동적 감지 휴리스틱 구현
- `md2html_v2/md2pdf_v2.bat`: 기본값 변경 및 Help 메시지 업데이트
- `docs/ISSUES.md`: 이슈 상태 업데이트 및 해결 내역 기록
- `docs/PROJECT_HISTORY.md`: 작업 이력 갱신
- `CHANGELOG.md`: 버전 정보 갱신

---

## 2025-12-20: tools 저장소 초기 설정

### 배경
여러 프로젝트(`tkcli`, `codesign_service` 등)에서 사용되는 공통 도구들이 각 저장소에 흩어져 있어 관리가 어려웠음.

### 작업 내용
1. **저장소 통합**: 공통 도구들을 `tools` 저장소로 통합
2. **도구 정리**:
   - `html2pdf`: CDP 로그 노이즈 억제 적용
   - `md2html`: Markdown → HTML 변환기 및 템플릿
   - `revlog.bat`: Git 히스토리 및 태그 조회 스크립트
   - `mp4towebp.bat`: FFmpeg 자동 다운로드 및 WebP 변환 스크립트
3. **문서화**: CHANGELOG, ISSUES, 규칙 문서 작성

### 의사결정
- 각 프로젝트의 `build.bat`에서 `tools` 저장소의 도구를 참조하도록 변경
- 도구 버전 관리는 Git 태그로 수행
