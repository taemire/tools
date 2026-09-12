# TOC 목적지 매핑 수정 검증 — 2026-09-08

판정: **PASS**. 원인·구현·테스트·AIRS 재현 결과를 기록한다.

## 원인과 수정

기존 analyzer는 GetPlainText 기반 제목 검색 및 본문 시작 추정을 사용했다. AIRS 재현에서 22개 중 16개가 누락됐지만 페이지 0을 성공으로 반환했다. converter도 누락된 페이지 맵을 오류로 처리하지 않았고, 조건부 템플릿이 번호를 숨겼다. 문서별로 다시 생성되는 제목 ID와 동일 파일명 section ID는 중복될 수 있었다.

현재는 파일/H2별 고유 ASCII 앵커를 PDF `/Dests` 또는 `/Names/Dests`로 연결한다. 누락·중복·잘못된 대상·0 이하 페이지는 오류다. 두 패스 모두 목차 번호 칸을 확보하고, 최종 PDF를 재분석해 번호가 달라지면 기존 출력에 쓰기 전에 종료 코드 1로 중단한다. CLI 옵션과 HTML-only 경로는 유지한다. 추가 의존성과 renderer 변경은 없다.

## 변경 파일

- `analyzer/analyzer.go`: 목적지 분석과 최종 검증.
- `converter/converter.go`: 고유 앵커, 여러 줄 H2 추출, 엄격한 페이지 적용.
- `converter/templates/layout{,_report,_modern}.html`: 링크·번호 표시 및 공간 확보.
- `main.go`: 임시 생성 → 검증 → 게시.
- `analyzer/analyzer_test.go`, `converter/converter_test.go`, `main_test.go`: 실제 PDF fixture 및 Chrome/CLI 회귀 테스트.
- `README.md`, `docs/toc-mapping-research.md`, 이 문서: CLI 계약·테스트·기존 연구 정정.

## 검증

```text
go test ./...
ok  md2pdf            46.750s
ok  md2pdf/analyzer    1.101s
ok  md2pdf/converter   1.722s
?   md2pdf/renderer   [no test files]
```

`go vet ./...`, `go build -o /private/tmp/md2pdf-toc-fix/md2pdf-after .`, `git diff --check`는 exit 0. gofmt 차이는 0이다. 사용자가 지정한 목적지·고유 앵커·누락 매핑 개별 테스트도 모두 통과했다. 테스트 삭제·기대값 하향·skip은 하지 않았다. Chrome 테스트는 3개 템플릿에서 긴 한영 제목, 본문의 앞선 제목 언급, 중복 제목을 검증하며, 숨긴 제목으로 CLI 실패와 기존 출력 보존도 검증한다.

AIRS 원본 Markdown 및 assets로 `_toc-fixed.html/pdf`를 실제 생성했다. 50페이지, 목차 22개, 1차와 최종 분석 각각 FOUND 22, WARN/ERROR/빈 번호 0. pdfinfo 목적지 및 pdftotext 목차 번호가 22/22 일치했다. qpdf/pdfinfo/pdftotext/pdftoppm은 모두 exit 0이며 50페이지 전체 PNG를 생성했다.

최종 보고서와 실행 명령·22개 항목 표·해시·로그·전체 diff:
`/Users/msjang/Documents/work_docs/output/PANW/AIRS_SCM_Manual_2026-09-07/qa/TOC_TOOL_FIX_REPORT_2026-09-08.md`

기존 원본 HTML/PDF는 이 세션에서 덮어쓰지 않았다. 이후 다른 작업으로 원본 이름이 수정본과 같아진 것을 확인했으며, 이전 원본은 `qa/archive/pre-toc-tool-fix-2026-09-08/`에 시작 SHA-256과 같은 상태로 보존돼 있다. 사용자 `tools/CLAUDE.md`도 시작/종료 해시가 동일하다. SCM 접속·변경, 저장소 commit/push, 기존 바이너리 교체는 수행하지 않았다.

## HTML 후속 변경

출력 HTML은 생성 이후 외부 폰트·아이콘·Mermaid 로딩 태그가 제거된 변경을 감지했다. 이를 덮어쓰지 않았으며, 목차 22개 링크·번호는 그대로임을 확인했다. PDF 해시는 바뀌지 않아 PDF 검증은 유효하다. 현재 HTML을 Chrome으로 임시 PDF에 렌더링해 50페이지·22개 목적지와 번호 일치를 재확인했다(exit 0). 최종 QA 보고서에는 생성 직후·현재 해시와 별도 검증 로그를 구분했다.

## 제한

PDF 목적지 없는 파일·모호한 페이지는 실패 처리한다. 중복된 옛 ID를 직접 가리키는 수동 링크는 별도 문제지만 생성 목차에는 영향이 없다. 두 패스 사이 페이지 구성이 변하면 자동 추정이나 무한 재시도 없이 실패한다. 최종 출력 게시 단계의 I/O 오류에 대한 두 파일 단위 원자성은 보장하지 않는다. 전체 테스트에는 실행 가능한 Chrome/Chromium이 필요하다.
