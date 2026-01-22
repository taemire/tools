# 변경 이력 (Changelog)

이 문서는 `tools` 프로젝트의 주요 변경 사항을 기록합니다.

## [Unreleased]

### ✨ 기능 개선
- **md2html_v2**: MD 확장 구문 통합 지원 (2026-01-22)
  - **Callouts/Admonitions**: GitHub(`> [!NOTE]`), Docusaurus(`:::note`), Docsify(`!>`) 구문 통합 지원
    - NOTE, TIP, IMPORTANT, WARNING, CAUTION 5가지 타입 및 아이콘
    - `preprocessAlerts` 함수로 전처리 통합
  - **Highlight**: `==텍스트==` → `<mark>텍스트</mark>` 변환
  - **Emoji**: `:emoji:` 단축코드 → Unicode 이모지 변환 (40개+ 지원)
  - **Footnotes**: `[^1]` 각주 구문 지원 (Goldmark 확장)
  - **Definition Lists**: `용어 : 정의` 구문 지원 (Goldmark 확장)
- **md2html**: PDF 변환 시 내부 링크 자동 변환 (`--pdf-mode`)
  - 상대 경로 링크(`.md#anchor`)를 PDF 내부 앵커(`#section-id`)로 변환
  - HTML 단독 사용 시 기존 링크 유지 (Docsify/브라우저 호환)
- **md2pdf.bat**: `md2html` 호출 시 `--pdf-mode` 자동 적용
- **revlog.bat**: 태그 컬럼 시인성 개선
  - 태그 길이 자동 감지 및 컬럼 너비 동적 조절 (Dynamic Column Width)
  - 조회 시 태그 정보 로딩 속도 개선 (`git log --format=%D`)
  - 구분선(`---`) 정렬 자동 보정
- **pdf_analyzer**: 목차 페이지 자동 감지 기능 고도화 (Heuristic-based Detection)
  - 텍스트 추출 시 특수문자 및 이모지 무시 로직 추가 (`containsTitle` 개선)
  - 섹션 밀도(Section Density), 텍스트 길이, 점선 패턴(Dot Leader) 등을 종합 분석
  - 섹션 제목이 많지만 본문이 적은 페이지를 목차로, 제목이 적고 본문이 풍부한 페이지를 본문으로 동적 식별
  - 감지 실패 시 기본값 3 사용 (표지 1p + 목차 2p)
- **md2pdf_v2.bat**: PDF 메타데이터 제어 기능 대폭 확장
  - `-subtitle`, `-author`, `-header`, `-footer` 플래그 추가
  - `SKIP_PAGES` 기본값을 3에서 0으로 변경하여 자동 감지 모드 활성화
- **md2html**: Docsify 알림 문법(`!>`, `?>`) 파싱 및 스타일링 개선
  - `**제목**: 내용` 패턴 감지 시 제목/본문 분리 (콜론 제거)
  - `postProcessAlerts` 함수 리팩토링으로 안정적인 HTML 변환
- **layout_report.html**: 
  - 표지 디자인 수정: `.Header`를 배지 텍스트로 사용, `.Subtitle`을 타이틀 아래에 별도 표시
  - Font Awesome CDN 추가 및 Alert 스타일 통합
- **layout_modern.html**: 
  - 표지 디자인 수정: `.Header`를 배지 텍스트로 사용, `.Subtitle` 및 `.Version` 구분 표시
  - Alert 스타일 통합

### 🐛 버그 수정
- **pdf_analyzer**: 목차 페이지의 섹션 제목을 본문으로 오인하여 모든 페이지 번호가 1~2로 고정되던 이슈 수정
  - 섹션 밀도 분석(isBodyPage)을 통해 목차 내의 텍스트와 본문 내의 헤딩을 정확히 구분하도록 개선

### 🐛 버그 수정 (Fixes)
- **layout_report.html**: 페이지 번호 재설정 오류 수정 (Isolated Counters Strategy 적용)
  - 렌더러의 페이지 리셋 제한을 우회하기 위해 `page-toc`와 `page-main` 카운터를 분리
  - 목차(**i**)와 본문(**1**)이 물리적 페이지 흐름과 무관하게 독립적으로 1부터 시작하도록 개선
- **md2pdf_v2.bat**: CLI 도움말(`-h`, `--help`) 지원 추가

### 📝 문서화
- **PDF_PAGE_NUMBERING_TROUBLESHOOTING.md**: 페이지 번호 문제 해결 과정에 대한 상세 기술 회고록 추가
- **.agent/rules.md**: UI 목업 및 스타일링 규칙 추가
  - CSS 중앙 관리 원칙
  - 템플릿 순수성 원칙
  - Docsify 호환성 유지 원칙

## [v0.1.0] - 2025-12-20

### 🎉 초기 릴리스
- **저장소 통합**: `tkcli`, `codesign_service` 등에 흩어져 있던 도구들을 `tools` 저장소로 통합.
- **html2pdf**: CDP 로그 노이즈 억제(Quiet Mode) 적용.
- **revlog.bat**: Git 히스토리 및 태그 조회 스크립트 추가.
- **mp4towebp.bat**: FFmpeg 자동 다운로드 및 설치 기능이 포함된 WebP 변환 스크립트 추가.
- **md2html**: Markdown 변환기 및 템플릿 
