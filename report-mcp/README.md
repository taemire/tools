# ARTEX 보고서 MCP

ARTEX Ubuntu 호스트에서 기존 작업 자료를 고객 검토용 PDF·HTML로 변환하는 서비스입니다. 스캔이나 모델 호출을 실행하지 않습니다. 기존 `md2pdf`의 목차 분석과 PDF 검사를 재사용합니다.

## 구성

- Go MCP SDK v1.8.0, Streamable HTTP, `127.0.0.1:8891/mcp`
- `render_report`, `validate_report`, `get_report_result`
- 입력: `/opt/artex-lab/data/sessions/{conv-N|task-N}` 아래 Markdown 및 로컬 이미지
- 출력: `/opt/artex-lab/data/report-pdf/{session}/job-<128bit>` 아래 PDF, HTML, 페이지 PNG, receipt.json
- ARTEX 표시 경로: `/app/data/report-pdf`
- UID/GID 2000의 `artex-runtime` 계정과 `report-mcp.service`
- 로컬 Chrome 제어 포트: 9223. 렌더링은 한 번에 한 건만 처리합니다.

기존 작업 상세의 **보고서** 탭이 작업별 자료를 읽어 개요, 범위, 자산, 대표 관계 그래프, 진행 내역, 관찰, 발견사항을 구성합니다. 빈 섹션, 승인 제어, 내부 작업 번호, 실행 지시, 인증 정보는 제외합니다. 발견사항의 실제 검증 상태를 보존하며, 작업 완료를 취약점 검증 완료로 바꾸지 않습니다.

그래프는 저장된 노드·관계를 유형별 대표 항목으로 집약합니다. 전체 탐색 그래프나 정량적인 커버리지 증명은 아닙니다. 같은 유형 노드의 관계는 대표 노드로 연결합니다.

## 빌드

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
