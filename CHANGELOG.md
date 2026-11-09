# 변경 이력 (Changelog)

이 문서는 `tools` 프로젝트의 주요 변경 사항을 기록합니다.

## [Unreleased]

### ✨ 새 도구
- **namefix**: 파일·폴더 이름의 자소 분리(NFD)와 URL 인코딩(`%XX`)을 정리하는 Windows 도구 추가 (2026-11-09)
  - 탐색기 오른쪽 클릭 메뉴 4종 등록: 파일, 폴더, 폴더와 하위 항목, 폴더 빈 공간 (현재 사용자, 관리자 권한 불필요)
  - 다중 선택 시 항목마다 실행된 프로세스의 요청을 한 프로세스로 모아 한 번에 처리하고 결과를 한 번만 표시
  - URL 디코딩은 올바른 UTF-8 시퀀스만 적용하고, 파일 이름 금지 문자로 풀리는 `%XX`(`%2F` 등)는 유지
  - 같은 이름의 다른 항목이 있으면(같은 파일의 하드 링크, 대소문자만 다른 항목 포함) 덮어쓰지 않고 충돌로 보고, 미리 보기도 예정 항목끼리의 충돌을 보고
  - 같은 요청을 다시 처리해도 결과가 같도록 멱등 처리: 전체 변환 고정점 반복, 요청 시점 원본 파일 ID로 현재 경로를 찾아 처리(SMB 공유는 정리된 이름을 따라가는 대체 탐색). 요청 당시 없던 경로나 경로에 새로 생긴 다른 항목은 처리하지 않음
  - 같은 항목을 다른 경로 표기(대소문자, 정션 별칭)로 여러 번 지정해도 한 번만 처리하고, 입력 표기가 아닌 저장된 이름을 정리
  - 처리 경로를 링크를 모두 따라간 실제 부모 경로 + 저장 이름으로 확정하고, 실제 상위 폴더 관계로 위상 정렬(정션 별칭, 연쇄 정션, 드라이브 루트 바로 아래 항목 포함)
  - 다중 실행 집계에서 각 프로세스의 종료 코드가 자기 요청의 결과(충돌·오류 여부)를 따름
  - 이름 변경 단계를 사용자 단위 실행 잠금으로 직렬화(동시 실행·재처리 시 오류 방지)
  - 링크(심볼릭 링크·정션) 폴더 안은 시작 폴더로 지정해도 처리하지 않음
  - 다중 실행 집계: 처리 완료 신호(완료 이벤트·수집자 수명 뮤텍스)로 처리 최소 한 번 보장, 요청 ID 중복 제거, 요청별 절대 경로·변환 옵션 유지, 연결·쓰기·읽기 제한 시간과 입출력 취소, 연결 단위 예외 격리
  - 예약 이름은 Microsoft 공식 목록(`COM1~9`, `COM¹~³`, `LPT1~9`, `LPT¹~³` 등)을 따르며, `COM0`·`LPT0`은 유효한 이름으로 처리
  - 변경 기록을 `%LOCALAPPDATA%\namefix\namefix.log`에 남기고 교체를 세션 간 뮤텍스로 직렬화
  - `.NET Framework 4.x` 기본 `csc.exe`로 빌드(SDK 불필요), 회귀 테스트 87건·탐색기 연동 테스트 10건

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
