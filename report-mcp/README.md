# ARTEX 보고서 MCP

형식 1.1의 기본 출력은 부록 C의 면책 항목 두 개만 포함하며 보존·파기 문장을 넣지 않습니다. `appendix.retention_policy`는 이전 요청과의 스키마 호환을 위해 검증만 하고 출력하지 않습니다. 정책이 없고 `retention_days`가 있는 기존 1.0·1.1 문서는 종전 일수 문장을 유지합니다. 두 필드가 없을 때 `[N]` 자리표시자를 만들지 않습니다. 1.0 스키마의 일수 필수 조건은 유지됩니다.

정책 종류별 허용 필드와 사용자 정의 문구의 길이·공백·제어 문자 검증은 플랫폼과 같습니다. 사용자 정의 문구의 등록 민감값은 거부합니다. 영수증 `arx_pentest_report.retention_policy`에는 `{"applied": true|false}`만 남으며 정책 종류·일수·문구는 남기지 않습니다.

## 0.3.0 고객 보고서 새 형식 `arx-pentest-report-1`

플랫폼(ARX)이 만든 구조화 JSON(형식 `arx-pentest-report` 1.0·1.1)을 HTML 템플릿과 인쇄 CSS로 PDF로 만듭니다. 플랫폼은 데이터·계산·마스킹·게이트·워터마크 설정 해석을, 이 서비스는 템플릿·인쇄·PDF 검사를 맡습니다(플랫폼 ADR 0008). 렌더러는 값을 계산·재작성하지 않고 받은 그대로 배치하며, 재현 절차와 PoC 문자열도 바꾸지 않습니다. 기본 양식은 전달된 샘플 보고서의 양식(제목 구성, 문서 정보·검토 및 승인 표, 장·절, 표의 열·머리글, 발견 사항 상세의 절)과 같고, 이후 추가된 요소는 `render_inputs.form`으로만 켭니다. 구현과 자산 출처는 [pentestreport/README.md](pentestreport/README.md)에 있습니다.

### 요청

`render_report`에 다음 필드를 보냅니다. `markdown_file`·`title`·`graph`·`formats`는 이 형식에서 받지 않습니다.

| 필드 | 값 |
|---|---|
| `session_id` | `conv-N` 또는 `task-N`. 출력 위치(`<REPORT_OUTPUT_ROOT>/<session>/job-…`)만 정합니다 |
| `source_format` | `"arx-pentest-report-1"` |
| `source` | 플랫폼의 `Evaluation.Resolved`(JSON 객체, 8 MiB 이하). `format.version`의 스키마로 검증하고 알 수 없는 판은 거부합니다 |
| `render_inputs.watermark` | 플랫폼이 해석한 적용 워터마크(`AppliedWatermark`). 필수 |
| `render_inputs.form` | 선택 요소 7개(`revision_history` 등). 생략하면 샘플 양식 |
| `render_inputs.names` | 표지·바닥글 제목과 고정 문구의 제품명(`Evaluation.Names`). 필수 |
| `render_inputs.sensitive_values` | 마스킹 전 등록 민감값. 출력에서 발견되면 실패합니다. 디스크·로그·영수증에 남기지 않습니다 |
| `render_inputs.metadata` | PDF Author·Subject |
| `render_inputs.expect` | 시험한 템플릿 판·해시 고정(선택) |
| `render_inputs.formats` | `pdf`·`html`·`docx`(생략 시 `pdf`·`html`). DOCX는 요청할 때만 만듭니다 |

형식 1.1의 표시 필드 4개(`team_text`, `list_title`, `detail_title`, `kisa_note`)는 값이 있으면 샘플 표기대로 그 자리에 넣고, 없으면 기존 패턴으로 출력합니다(규칙은 [pentestreport/README.md](pentestreport/README.md)).

`source_format`을 생략하면 기존 Markdown(md2pdf) 경로, `"artex-md-3"`이면 기존 v3 경로입니다. 그 밖의 값(예: `"markdown"`)은 `SOURCE_FORMAT_UNSUPPORTED`와 함께 지원 값과 Markdown 사용법을 돌려줍니다. `source`나 `render_inputs`만 보내고 `source_format`을 빠뜨리면 `SOURCE_FORMAT_REQUIRED`입니다.

### 검사와 영수증

- 거부: `SCHEMA_INVALID`, `SOURCE_FORMAT_VERSION_UNSUPPORTED`, `SOURCE_NOT_RESOLVED`(계산값 누락), `SOURCE_INCONSISTENT`(같은 값의 두 사본이 다름), `WATERMARK_*`, `TEMPLATE_*_MISMATCH`.
- 출력 검사: `RENDERED_ACTIVE_CONTENT`, `SENSITIVE_VALUE_IN_OUTPUT`(HTML 보이는 텍스트·주석·속성·원문, PDF 텍스트·문서 정보, 워터마크 문구; 값 대신 번호만 보고), `PDF_FONT_UNEXPECTED`(DejaVu Sans·Noto Sans CJK SC·Noto Sans Mono CJK SC·Noto Sans JP/SC 외), `PDF_FONT_TYPE3`, `WATERMARK_TEXT_IN_PDF`, 쪽 수, 기존 `qpdf --check`·텍스트 경계·쪽 미리보기.
- 실패한 job은 `report.html`·`report.pdf`를 지우고 `FAILED` 영수증만 남깁니다.
- 영수증 `arx_pentest_report`에 형식·언어, 렌더러·템플릿 판, Chrome 판, 입력 해시(`source_sha256`, 민감값을 뺀 `render_inputs_sha256`), 템플릿·CSS·토큰·사전·스키마 해시, 글꼴 파일 해시, 적용 워터마크 설정과 윤곽 해시, 양식, 이름 해시, 민감값 개수, 출력 해시, PDF 쪽 수·글꼴을 남깁니다. 입력 해시는 키를 정렬하고 HTML 이스케이프를 하지 않은 JSON(UTF-8, 끝 개행 없음) 기준입니다.
- 영수증·오류 응답·명령 출력에는 요청의 자유 문자열을 원문으로 넣지 않습니다. 이름·워터마크 문구·PDF 문서 정보는 SHA-256과 길이, 출처 표시·`field_sources`는 허용 값만, `rejected_overrides`는 개수만 남깁니다. 오류 메시지는 코드·고정 경로·숫자만 담고, 요청 문자열이나 등록 민감값이 섞이면 상세를 뺍니다. 저장 직전에 영수증 전체를 등록 민감값으로 다시 검사합니다(`RECEIPT_SENSITIVE_VALUE`).
- 기계 검사 통과는 `GENERATED_MACHINE_CHECKS_PASS`이고 `customer_release`는 `HOLD_REQUIRES_CONTENT_AND_VISUAL_REVIEW`입니다. 발행 게이트 판정은 플랫폼 영수증에 있습니다.

상주 Chrome에서 인쇄할 때는 렌더마다 `disposeOnDetach` 브라우저 컨텍스트를 새 창으로 열고 끝나면 지웁니다. 쿠키·저장소·캐시를 다른 렌더와 공유하지 않습니다. DevTools 포트는 인증이 없으므로 `REPORT_CHROME_URL`은 보고서 서비스 전용 Chrome이 있는 단일 사용자 호스트의 루프백 주소로만 둡니다.

### DOCX 출력

사용자 원본 Word 양식의 편집 가능한 DOCX를 PDF와 같은 요청으로 만듭니다. 요청 스키마(`source`)는 그대로이고 `render_inputs.formats`에 `"docx"`를 넣습니다(예: `["pdf","html","docx"]`). PDF와 DOCX는 같은 `Build` 결과로 만들므로 문구·표 값·발견 순서·재현 절차·PoC 원문이 같습니다. 양식·글꼴·검사는 [렌더러 안내](pentestreport/README.md#docx-출력)를 따릅니다.

| 항목 | 값 |
|---|---|
| 지원 확인 | `get_report_result`(`capabilities_only=true`)의 `capabilities`에 `arx-pentest-report-1:docx`. 이 값이 없는 렌더러는 `docx`를 `RENDER_FORMAT_INVALID`로 거부합니다 |
| 출력 파일 | job 폴더의 `report.docx`. 영수증 최상위 `docx_path`(표시 경로 `/app/data/report-pdf/<session>/<job>/report.docx`) |
| 영수증 | `arx_pentest_report.outputs.docx_sha256`, `arx_pentest_report.docx`(부품 목록, 지정 글꼴, `fonts_embedded: false`, 워터마크 방식, 문서 속성의 해시·길이, `docx_template_sha256`), `template.docx_template_sha256`과 `template.docx/*` 파일 해시 |
| 검사 | `docx_structure`, `docx_fonts_not_embedded`, `docx_text_matches_html`이 `PASS`, `docx_sensitive_values`가 `PASS_<n>_VALUES` |
| 실패 | `DOCX_*`, `SENSITIVE_VALUE_IN_OUTPUT`(위치 `docx_visible`·`docx_hidden`·`docx_properties`·`docx_raw`). 실패하면 HTML·PDF·DOCX를 모두 지웁니다 |
| `sha256` | 기존과 같습니다(PDF가 있으면 PDF, 없으면 HTML의 SHA-256) |

DOCX 템플릿 부품이 템플릿 묶음에 들어가므로 `TemplateSHA256()`이 바뀝니다. `expect.template_sha256`을 쓰는 호출자는 새 값으로 갱신합니다. `validate_report`는 기존처럼 PDF만 다시 검사합니다.

### 워터마크 위치 회귀 시험

워터마크는 `@page` 배경이며, Chromium이 배경을 쪽 영역 기준으로 놓는 동작에 맞춰 여백만큼 보정합니다. Chrome을 갱신하면 운영 호스트에서 다음을 실행합니다. 표지·본문 3쪽의 워터마크 중심이 쪽 중심에서 2 mm 안에 있고, `body_only`이면 표지에 없어야 통과합니다.

```sh
sudo -u artex-runtime /opt/artex-lab/report-mcp/bin/report-mcp wm-position-check --chrome-url http://127.0.0.1:9223
sudo -u artex-runtime /opt/artex-lab/report-mcp/bin/report-mcp wm-position-check --chrome-url http://127.0.0.1:9223 --pages body_only
```

Chrome 156.0.8078.80(macOS)에서 세 쪽 모두 중심 오차 (0.3 mm, −1.0 mm)였고, 보정을 끄면 실패로 잡힙니다.

### md2pdf 브라우저 연결(운영 장애 2026-11-02 수정)

상주 Chrome이 `127.0.0.1:9223`을 쓰는 상태에서 md2pdf가 같은 포트로 새 Chrome을 띄우면, Chrome은 IPv4 바인딩에 실패하고 `[::1]:9223`으로 DevTools를 엽니다(`--remote-debugging-address=127.0.0.1`을 주어도 같음, Chrome 156 재현). 방화벽은 UID 2000에 `127.0.0.1:9223`만 허용하므로 연결이 거부됐습니다. 이제 report-mcp는 md2pdf에 `MD2PDF_CHROME_URL=$REPORT_CHROME_URL`과 `MD2PDF_DEVTOOLS_PORT=$REPORT_MD2PDF_DEVTOOLS_PORT`(기본 9223)를 넘깁니다. md2pdf는 응답하는 상주 Chrome을 재사용하고 WebSocket 주소를 설정한 IPv4 주소로 고정합니다. 상주 Chrome이 없으면 포트가 비어 있는지 확인한 뒤에만 자체 Chrome을 띄우고, 점유돼 있으면 `DEVTOOLS_PORT_BUSY`, IPv6 광고를 보면 `DEVTOOLS_ADDRESS_FAMILY_MISMATCH`로 원인을 밝혀 실패합니다. 런타임 패치(`scripts/md2pdf-runtime.patch`)에서 고정 포트 줄은 뺐습니다.

### `PrivateTmp`와 상주 Chrome의 공유 경로

레거시 converter는 최종 출력 job 아래의 `.md2pdf-*`(0700)에 중간 HTML·PDF·TOC 자료를 만들고 성공·오류 반환 후 정리합니다. MCP도 자식 종료 후 해당 job의 중간 폴더를 정리해 converter timeout에 대비합니다. MCP의 전용 `/tmp`에 만든 HTML은 다른 namespace의 상주 Chrome이 읽을 수 없어 `ERR_FILE_NOT_FOUND`가 됩니다. v3·새 형식은 기존대로 job의 HTML을 읽습니다. MCP와 Chrome은 같은 `artex-runtime` UID를 사용하고 출력 루트가 두 프로세스에 같은 경로로 보여야 합니다. `PrivateTmp=yes`와 `127.0.0.1:9223` 방화벽은 유지하며 새 포트·서버·환경 변수는 필요 없습니다. 새 converter를 포함한 검증 빌드를 배포하고 [경로·권한·정리·수용 절차](deploy/PRIVATE_TMP.md)를 따릅니다.

### Markdown 경로 영수증의 HTML 해시

`source_format`을 생략한 Markdown(md2pdf) 경로도 영수증의 `v3.html_sha256`에 최종 `report.html`의 SHA-256(소문자 16진수 64자)을 남깁니다. 해시 대상은 글꼴을 data URI로 넣은 뒤 디스크에 쓴 바이트입니다. artex-md-3와 같은 필드이며, ARX 대화 보고서 카드는 이 값과 파일이 일치할 때만 HTML 다운로드를 엽니다.

- 이 경로의 `v3`는 `html_sha256`만 값이 있습니다. 나머지 필드는 빈 값(`source_format: ""`, 판·해시 필드 `""`, `font_sha256`·`figures`는 `null`)입니다. 도구 출력 스키마가 `v3`의 필드를 필수로 두므로 빈 값으로 채우며, artex-md-3 출처를 뜻하지 않습니다. artex-md-3 결과는 `v3.source_format`이 `"artex-md-3"`인지로 판별합니다.
- 최상위 `sha256`은 기존처럼 PDF의 해시입니다. `validate_report`는 영수증을 다시 쓸 때 `v3`를 그대로 보존하며 HTML을 다시 검사하지는 않습니다.
- HTML이 만들어지지 않으면 job은 기존처럼 실패하고 HTML 해시를 남기지 않습니다. 이 변경 전에 만든 영수증에는 `v3`가 없으므로 그 결과의 HTML 다운로드는 계속 비활성입니다.
- 도구 입출력 스키마와 템플릿 해시(`TemplateSHA256`, artex-md-3 `TemplateSHA256`)는 바뀌지 않습니다.

### 운영 배포(별도 승인 단계)

1. 빌드: 이 저장소의 커밋된 상태에서 빌드합니다. `build-converter.sh`는 작업 트리의 md2pdf를 복사하므로 미커밋 변경이 없는지 먼저 `git status`로 확인합니다.
   ```sh
   cd report-mcp
   go test -race ./... && go vet ./...
   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /abs/out/report-mcp .
   ./scripts/build-converter.sh /abs/new-build-dir /abs/out/md2pdf
   sha256sum /abs/out/report-mcp /abs/out/md2pdf
   ```
2. 백업: 운영 `/opt/artex-lab/report-mcp/bin/{report-mcp,md2pdf}`와 `/etc/systemd/system/report-mcp.service`를 날짜 붙은 이름으로 복사합니다. 실행 중인 렌더(`report-pdf` job의 `RENDERING` 영수증)와 진행 중인 대화가 없는지 확인합니다.
3. 상주 Chrome 확인: `curl -s http://127.0.0.1:9223/json/version`의 `Browser`와 `webSocketDebuggerUrl` 호스트가 `127.0.0.1`인지, `ss -ltnp | grep 9223`에 `[::1]:9223` 리스너가 없는지 봅니다. 상주 Chrome을 띄우는 unit은 이 저장소 `deploy/`에 없으므로 운영 구성을 따로 확인합니다.
4. 설치: 두 바이너리를 같은 경로에 0755로 바꾸고, `deploy/report-mcp.service`의 새 `Environment` 줄 3개(`REPORT_CHROME_URL`, `REPORT_MD2PDF_DEVTOOLS_PORT`, `REPORT_PR_FONT_DIR`)를 반영합니다. 일본어·중국어 워터마크를 쓸 때만 허용 목록 SHA-256과 같은 `NotoSansJP-700.ttf`·`NotoSansSC-700.ttf`를 `/opt/artex-lab/report-mcp/fonts/`(0644)에 둡니다.
5. 재시작과 확인:
   ```sh
   sudo systemctl daemon-reload && sudo systemctl restart report-mcp.service
   curl -s http://127.0.0.1:8891/health                # "version":"0.3.0"
   sudo -u artex-runtime /opt/artex-lab/report-mcp/bin/report-mcp wm-position-check
   ```
   MCP `get_report_result`에 `capabilities_only=true`를 보내 `arx-pentest-report-1`이 있는지 보고, 기존 Markdown 경로로 시험 문서 한 건을 생성해 `render.log`에 `Using running Chrome at http://127.0.0.1:9223`과 `GENERATED_MACHINE_CHECKS_PASS`를 확인합니다.
6. 롤백: 백업한 두 바이너리와 unit 파일을 되돌리고 `daemon-reload`·`restart` 후 `/health`의 버전이 0.2.x인지 확인합니다. 새 형식 job 폴더는 그대로 두어도 기존 경로에 영향이 없습니다. 플랫폼이 새 형식을 호출하기 전이면 롤백은 이 서비스만으로 끝납니다.

위험: 출력 HTML은 글꼴을 data URI로 넣어 약 12 MB입니다. 서비스 메모리 상한(1536M)과 운영 Chrome에서의 메모리·속도는 로컬에서 시험하지 않았으므로 배포 단계에서 확인합니다. 운영 Chrome 판이 156이 아니면 위치 회귀 시험을 먼저 통과시켜야 합니다.


## 0.2.1 보고서 글꼴·출력 품질 수정

플랫폼이 직렬화하는 SVG 글꼴 이름은 기존 wire 계약인 `ARTEXReport`·`ARTEXMono`입니다. 표시 브랜드·설정 접두어와 구분하며 MCP의 SVG 허용 목록을 확대하지 않습니다.

V3 HTML은 원본 TTF에 없는 문자에 한해 번들 Noto Sans JP/SC 400·700 WOFF2를 포함합니다. 원본·파생 폰트의 SHA-256, Unicode 범위와 PostScript 이름을 함께 고정합니다. 오프라인 데이터이므로 서비스 실행 시 폰트를 내려받거나 Python을 사용하지 않습니다. [출처·OFL 라이선스와 재현 절차](artexmd/fallbackfonts/README.md)를 따릅니다.

PDF는 글리프 지원, 실제 브라우저의 웹폰트 선택과 출력 PDF의 폰트 이름을 확인합니다. 지원하지 않는 문자는 `RENDER_GLYPH_UNAVAILABLE`로 거부하며 깨진 PDF를 성공으로 공개하지 않습니다. HTML·DOCX 초안은 PDF 전용 글꼴 검사에 묶지 않습니다. 해당 초안의 지원 범위를 확정할 수 없으면 원문을 보존하고 receipt의 `font_glyph_coverage`를 `UNVERIFIED`로 기록합니다. 이는 내용·시각 검토나 고객 제출 승인이 아닙니다.

기존 source format·DOCX 스타일·발급본과 저장 파일은 변경하지 않습니다. 0.2.1 소스·검증 결과와 운영 서비스의 실제 설치 버전은 별도로 확인해야 합니다.

ARTEX Ubuntu 호스트에서 기존 작업 자료를 고객 검토용 문서로 변환하는 서비스입니다. 스캔이나 모델 호출을 실행하지 않습니다. 형식 생략은 기존 md2pdf PDF·HTML 경로이며, 로컬 구현 v0.2.0은 artex-md-3의 단일 AST에서 HTML·DOCX를 만들고 Chrome PDF를 인쇄합니다. 운영 재배포는 하지 않았습니다.

## 구성

- Go MCP SDK v1.8.0, Streamable HTTP, `127.0.0.1:8891/mcp`
- `render_report`, `validate_report`, `get_report_result`, 로컬 추가 `render_report_figures`
- 입력: `/opt/artex-lab/data/sessions/{conv-N|task-N}` 아래 Markdown 및 로컬 이미지
- 출력: `/opt/artex-lab/data/report-pdf/{session}/job-<128bit>` 아래 PDF, HTML, 페이지 PNG, receipt.json
- ARTEX 표시 경로: `/app/data/report-pdf`
- UID/GID 2000의 `artex-runtime` 계정과 `report-mcp.service`
- 로컬 Chrome 제어 포트: 9223. 렌더링은 한 번에 한 건만 처리합니다.

기존 작업 상세의 **보고서** 탭이 작업별 자료를 읽어 개요, 범위, 자산, 대표 관계 그래프, 진행 내역, 관찰, 발견사항을 구성합니다. 빈 섹션, 승인 제어, 내부 작업 번호, 실행 지시, 인증 정보는 제외합니다. 발견사항의 실제 검증 상태를 보존하며, 작업 완료를 취약점 검증 완료로 바꾸지 않습니다.

그래프는 저장된 노드·관계를 유형별 대표 항목으로 집약합니다. 전체 탐색 그래프나 정량적인 커버리지 증명은 아닙니다. 같은 유형 노드의 관계는 대표 노드로 연결합니다.

## 빌드

### 보고서 도식 B1 시제품(로컬·미통합)

`render-figures` CLI는 ARTEX가 만든 `customer-visuals-1` SVG bundle을 받아 고정 로컬 한글 폰트와 Chrome의 렌더별 새 탭에서 2배 PNG로 변환합니다. SVG의 script/CSS/외부 자원·URL·foreignObject를 거부하고 글꼴 glyph, 16cm 폭 기준 최소 8pt, 대비 4.5:1, 텍스트 겹침·카드/그림 밖 넘침을 검사합니다. 실패하면 임시 출력만 제거하며 기존 출력은 덮어쓰지 않습니다. 출력 배타 잠금과 단계 디렉터리를 사용합니다.

```sh
go build -o /absolute/local/report-mcp .
/absolute/local/report-mcp render-figures --input /absolute/bundle.json \
  --out /absolute/new-output-directory --chrome /absolute/chrome \
  --font /absolute/trusted-korean-font.ttf \
  --mono-font /absolute/trusted-monospace-font.ttf
```

receipt는 model/SVG/PNG/font 해시·Chrome/렌더 프로필 버전·2배 해상도·생략/증류 원장을 담습니다. PNG를 data URI로 내장한 정적 `preview.html`도 생성합니다(상대 그림 경로 없음). R2는 `layout_version`과 고정폭 글꼴 해시를 함께 남깁니다. URL/경로에 ARTEXMono를 사용한 bundle은 `--mono-font` 없이는 거부하며, 시스템 글꼴로 자동 대체하지 않습니다. 같은 출력 디렉터리 재요청은 덮어쓰기 대신 거부합니다. stale `.lock`는 자동 삭제하지 말고 실행 주체가 종료됐는지 확인합니다. 프로세스 강제 종료·분산/NFS 잠금 복구는 미검증입니다.

초기 B1은 PNG 품질 검토용 CLI 시제품이었습니다. 이후 R-4에서 같은 검사기를 additive render_report_figures 도구로 연결했습니다. 아래 v3 절을 따릅니다. 운영 서비스·systemd·방화벽·기존 legacy 그래프 경로는 변경하지 않았습니다. 도식을 보안 점검·침투 실행이나 고객 제출 승인으로 해석하지 않습니다.

## artex-md-3 — 로컬 구현·미배포

- source_format="artex-md-3", formats=[html,pdf,docx], render_inputs를 명시합니다. 생략 시 기존 legacy 분기이며 실패한 v3는 legacy로 재시도하지 않습니다. artexmd는 Table + 슬롯 BlockParser만 사용하고 일반 코드·원시 HTML·직접 이미지·링크를 거부합니다.
- 슬롯은 `:::slot <id> locked|edit`와 `:::`이며 document 직속·0열만 허용합니다. 구성요소는 fenced JSON artex-kpi/artex-figure/artex-detail이고 각 schema_version이 필요합니다. report-mcp가 job당 Parse 1회와 같은 AST를 공유합니다.
- get_report_result에 session_id="", job_id="", capabilities_only=true를 보내면 artex-md-3/docx/render_report_figures 지원을 확인합니다. 기존 job 조회 입력·출력은 유지합니다.
- render_inputs: generated_at, title, author, header_text, footer_text, language, figure_manifest_sha256. MD/PNG/입력과 출처 버전이 같으면 HTML/DOCX 바이트가 같으며 job·시계·TZ·경로·입력 이름을 넣지 않습니다. PDF는 구조/폰트/Type3/경계/북마크 목적지/전 페이지 PNG를 검사합니다.
- PNG는 세션의 figures/manifest.json(artex-figure-bundle-1)로만 참조하고 파일당8MiB·합계64MiB·50개·4천만픽셀을 제한합니다. source ref의 누락/중복/미참조를 거부하며 한 번 읽은 바이트를 고정합니다. DOCX는 job/report.docx 정규 파일입니다.
- render_report_figures(session_id,bundle_file)는 artex-layout-r3b-v1만 받습니다. 기존 단일 busy 잠금과 B1/R3의 보안·폰트 해시·9pt·대비·폭 차 검사를 재사용하며 outer timeout은120초입니다. 결과 PNG·전체 audit receipt는 job/figures/에 보존하고 RPC에는 중복 산문/원장만 생략합니다.
- 신뢰한 서비스 설정: REPORT_FIGURE_FONT_REGULAR/MONO/MEDIUM/BOLD. 도식 Chrome 연결은 REPORT_FIGURE_CHROME_URL → REPORT_CHROME_URL 순으로 선택하며, 둘 다 비어 있으면 REPORT_FIGURE_CHROME(기본 google-chrome, CLI는 --chrome)을 실행합니다. URL 연결 실패 시 자체 실행으로 재시도하지 않습니다. 공유 Chrome에는 렌더마다 새 탭을 만들고 종료하며 브라우저는 유지합니다. ARTEX는 같은 파일의 sfnt 메트릭용 ARTEX_REPORT_VISUAL_FONT_DIR이 필요합니다. 경로 미설정/실패 시 발급은 닫히며 설치나 그림 없는 자동 대체는 하지 않습니다.
- PDF 본문 TTF는 REPORT_V3_FONT_REGULAR/BOLD(기본 /usr/share/fonts/truetype/nanum/NanumGothic[Bold].ttf), Chrome은 REPORT_CHROME_URL(기본 http://127.0.0.1:9223)입니다. 운영 설정·글꼴 설치는 별도 승인 대상입니다.
- DOCX는 CreateRaw·Deflate·고정 ZIP 시각/순서를 쓰고 GD 허용 파트만 만듭니다. 머리글/바닥글과 PAGE, KPI 표·16cm PNG·전문 표의 반복 머리글·행 유지가 포함됩니다. 실제 Word 왕복과 최종 지면 판정은 별도입니다.
- provenance는 실제 렌더 소스 SHA, 고정 DOCX styles SHA, HTML CSS SHA, parser/renderer/token/font 식별값을 기록합니다. token JSON은 ARTEX 정본이며 복제하지 않고 승인된 해시/값 프로필을 고정합니다. token 변경 시 두 쪽 검토·pin 갱신이 필요합니다.
- 독립 빌드: go test -race ./..., go vet ./..., go build . (개인 스킬·외부 replace 의존 없음). render-v3 CLI는 합성 fixture용 세션/출력 루트와 JSON 요청을 명시해 서버 없이 동일 경로를 시험할 수 있습니다.

Go 1.27.1을 사용합니다. 저장소의 다른 `md2pdf` 변경을 건드리지 않고 별도 빌드 디렉터리에 고객 템플릿과 루프백 포트·물리 쪽번호 패치를 적용합니다.

```sh
go test -race ./...
go vet ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/report-mcp .
./scripts/build-converter.sh /absolute/new-build-directory /absolute/output/md2pdf
```

`scripts/md2pdf-runtime.patch`는 현재 md2pdf 소스에 대한 최소 패치입니다. 기준 소스가 달라 적용되지 않으면 중단하고 검토합니다. 원본 `md2pdf/go.mod`와 기존 바이너리는 변경하지 않습니다. `templates/layout_customer.html`이 고객 지면의 정본입니다.

## 호스트 설치

필요 실행 파일은 Google Chrome, qpdf, pdfinfo, pdffonts, pdftotext, pdftoppm입니다. NanumGothic와 Bold 폰트를 `/usr/share/fonts/truetype/nanum/`에서 읽습니다. 설치 파일의 출처와 버전을 확인한 뒤 설치합니다.

`/opt/artex-lab/report-mcp/bin`에 두 바이너리를 배치하고, cache와 출력 폴더만 UID 2000에 쓰기 권한을 부여합니다. 입력 sessions는 서비스에서 읽기 전용입니다. `deploy/`의 systemd 및 방화벽 파일을 지정된 호스트 경로에 배치합니다. 계정 UID가 이미 사용 중인지 먼저 확인합니다.

`deploy/report-mcp.service`는 `REPORT_FIGURE_FONT_REGULAR/MONO/MEDIUM/BOLD`를 앱 글꼴 폴더(`ARTEX_REPORT_VISUAL_FONT_DIR`의 호스트 경로 `/opt/artex-lab/data/fonts`)의 Pretendard-Regular·D2Coding·Pretendard-Medium·Pretendard-Bold 파일로 지정합니다. 네 키 중 하나라도 없으면 `render_report_figures`가 `FIGURE_RENDER_UNAVAILABLE`로 즉시 실패합니다.

`report-mcp-firewall.sh`는 기존 UID 2000용 `ARTEX_LAB_OUTPUT`에 **127.0.0.1 TCP 8891/9223**만 추가합니다. SG·외부 인바운드·다른 UID 정책은 변경하지 않습니다. 기본 방화벽 서비스를 수동 재시작하면 추가 규칙이 사라질 수 있으므로 보고서 방화벽 서비스를 다시 실행해야 합니다. enabled 상태와 실제 재부팅 검증은 별개입니다.

MCP 설정에 HTTP 서버 `report-pdf`, URL `http://127.0.0.1:8891/mcp`를 등록하고 reporter에게 도구를 연결합니다. 서비스 자체에는 외부 인증을 붙이지 않으므로 루프백 노출을 유지합니다. UI API와 파일 다운로드에는 기존 ARTEX 로그인이 적용됩니다.

## API 연결

ARTEX 서버의 확장 파일은 `report/customer.go`, `server/report_pdf.go`이며 기존 보고서 탭과 `web/src/lib/api.ts`를 연결합니다.

| API | 역할 |
|---|---|
| GET `/api/tasks/{id}/report/customer` | 기존 작업 자료의 고객 미리보기 |
| GET `/api/tasks/{id}/report/pdf` | 마지막 생성 결과 |
| POST `/api/tasks/{id}/report/pdf` | 보고서 생성 |
| GET `/api/tasks/{id}/report/files/{job}/{name}` | 해당 작업의 인증된 파일 다운로드 |

파일 API는 허용한 파일명·작업·job 경로만 읽습니다. PDF 미리보기와 다운로드는 인증된 파일을 blob으로 열며 토큰을 URL에 넣지 않습니다. UI 배포는 기존 이미지 기반으로 바이너리만 교체하고, 활동 중인 작업이 없을 때 compose·이미지·DB 백업을 먼저 남깁니다.

LLM 채팅창과 작업 세션의 공용 Transcript에서도 성공한 report-pdf 도구 결과에 다운로드 카드를 표시합니다. 직접 도구 호출과 `ExecuteExtraTool`의 지연 호출 모두 실제 도구명을 확인합니다. `/api/report-pdf/{session}/{job}/{name}`는 로그인·세션·생성 receipt를 확인하고 해당 job 내부의 허용 파일만 내려줍니다. 일반 문장의 임의 링크는 다운로드 카드로 바꾸지 않습니다. 2026-10-13 실제 보고서 작성 대화 58에서 생성·카드 복구·PDF/HTML 다운로드·미리보기를 검증했습니다.

## 검증과 제출 판정

생성 시 목차 목적지, PDF 구조, 모든 폰트 임베딩, 페이지 수, 텍스트 경계, SHA-256, 모든 페이지 PNG를 확인합니다. HTML에는 폰트와 그림을 포함합니다. 경로 이탈, 외부 이미지, 실행 가능한 링크를 거부하고 원문 HTML을 문자로 표시합니다.

기계 검사 성공은 `GENERATED_MACHINE_CHECKS_PASS`입니다. 내용의 사실성이나 고객 제출 승인이 아닙니다. `customer_release`는 항상 `HOLD_REQUIRES_CONTENT_AND_VISUAL_REVIEW`로 남깁니다. 해당 고객·범위, 미검증 발견사항, 영향·조치·재검증 내용은 제출 전에 별도로 검토해야 합니다.

2026-10-13 실제 ARTEX UID 2000 네이티브 MCP 클라이언트와 로그인 UI에서 작업 23/26의 생성·미리보기·PDF/HTML 다운로드를 검증했습니다. 서비스 재부팅 복원과 DB 전체 복원 리허설은 미검증입니다. 전체 ARTEX CI 대신 영향 경로의 Go 테스트와 UI 타입 검사·정적 빌드를 수행했습니다.

## 롤백

UI는 해당 배포 백업의 compose를 복구하고 이전 이미지로 ARTEX만 다시 올립니다. DB dump는 자동 복원하지 않습니다. MCP는 설정을 비활성화한 뒤 서비스를 중지합니다. 기본 방화벽 전체를 재시작하지 않고 아래 두 규칙만 제거합니다.

```sh
sudo systemctl disable --now report-mcp.service report-mcp-firewall.service
sudo iptables -w -D ARTEX_LAB_OUTPUT -d 127.0.0.1/32 -p tcp --dport 8891 -j ACCEPT
sudo iptables -w -D ARTEX_LAB_OUTPUT -d 127.0.0.1/32 -p tcp --dport 9223 -j ACCEPT
```

기존 HTTP/HTTPS·HexStrike 허용 규칙과 출력 보고서는 보존합니다.

### R3 보고서 도식 — 로컬 검증 단계

`artex-layout-r3b-v1` bundle은 16cm 인쇄 폭에서 최소 **9pt**를 검사합니다. `--font`(Pretendard Regular) 외에 `--mono-font`(D2Coding), `--medium-font`, `--bold-font`의 실제 파일 4종과 bundle의 SHA-256이 일치해야 합니다. 가짜 굵게와 커닝은 끄며, Go가 기록한 모든 텍스트 폭을 Chrome 실측과 비교해 차이가 2px를 넘으면 실패합니다. 그림별 `layout_variant`, `geometry_rules_pass`, `geometry_plan_sha256`와 전체 `layout_rules_version`, `layout_rules_sha256`, `tokens_sha256`를 receipt에 보존합니다. Go의 연결선 계획 검사 결과는 전달된 계약 증거이며 MCP가 독립적으로 그 계획의 모든 의미를 재구성한 결과는 아닙니다.

제목·설명·주석은 그림 밖의 HTML 텍스트로 표시합니다. 미리보기 PNG는 data URI로 내장하므로 단일 HTML을 옮겨도 상대 이미지 경로가 깨지지 않습니다. 제품 문구는 ARTEX가 전달한 l10n ID 해석 결과를 사용하고 번역 초안·언어 대체·중복 목표 경고를 표시합니다. 이 구현은 로컬 CLI 시험 단계이며 운영 서비스에 재배포하지 않았습니다.

### 번호 그림과 전문 상세 표

`detail_tables`(`visual-details-1`)가 있는 bundle은 그림별 `detail_table_sha256`와 일치하는 표를 요구합니다. 표 전문 변경·번호 오류·표 누락을 거부하고, HTML에는 각 도식의 마지막 PNG 아래 전문 표를 한 번만 냅니다. `details.json`과 receipt에도 같은 전문·해시를 남깁니다. 미리보기는 밝은 배경과 글자색을 명시하며, 표의 줄 수·높이를 제한하거나 말줄임하지 않습니다. OOXML 표·잠금 슬롯 연결은 ARTEX의 후속 DOCX 단계입니다. 이 기능은 로컬 시험만 수행했습니다.

### R-5 지면 보완(로컬 검증)

v3 DOCX는 모든 참조 paragraph/character/table 스타일을 정의하고 Heading 기반·outline과 해설 스타일 keepNext를 사용한다. 도식 제목은 새 쪽에서 시작하며, KPI 숫자는 설정된 Pretendard TTF(HTML)·맑은 고딕 굵게(DOCX)와 A84617이다. 기본 TTF 경로는 `/usr/share/fonts/truetype/pretendard/Pretendard-{Regular,Bold}.ttf`이며 파일이 없으면 실패한다. 이번 작업은 글꼴을 설치하거나 운영 MCP를 바꾸지 않았다. 문서 표의 Unicode 문자 수가 400자를 넘으면 HTML·DOCX 양쪽에서 행 분할을 허용한다. 수신 프로파일의 스타일 해시도 함께 갱신해야 하며, 기존 발급 기록을 다시 쓰지 않는다. 실제 Word 및 최종 지면 품질은 별도 검토 대상이다.

### policy3 상세 부록(로컬·미배포)

ARTEX의 `appendix.customer_facts` 슬롯은 독립 쪽에서 시작합니다. 해당 슬롯의 2열 표만 번호 680/내용 8391 dxa(HTML 8/92%)로 렌더하며, 기존 슬롯·일반 표의 배치는 유지합니다. 새 슬롯을 포함한 의미 투영은 ARTEX가 발급 전에 독립 계산합니다. 후보 제출은 원본 상세를 바꾸지 않으며, 렌더 성공을 고객 내용 승인으로 해석하지 않습니다.

새 형식의 지면(줄 간격, 표 행 높이, 쪽 번호, 쪽 나눔, 한글 줄바꿈)은 Word로 만든 사용자 원본 13쪽 기준입니다. 글꼴은 맑은 고딕·Consolas 대신 DejaVu Sans와 정적 TrueType Noto CJK SC를 그대로 사용하므로 글자 폭과 줄바꿈 위치는 원본과 다를 수 있고, Type 3는 거부합니다. 승인 기록이 없어도 작성·검토·승인 행과 빈 서명 칸을 유지합니다. 글꼴 출처·라이선스·원본 내용 재현 시험은 [렌더러 안내](pentestreport/README.md)를 따릅니다.
