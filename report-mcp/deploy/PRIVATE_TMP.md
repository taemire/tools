# 상주 Chrome과 converter의 파일 공유

레거시 converter의 중간 파일은 최종 PDF와 같은 job 폴더 아래의 `.md2pdf-<random>/`에 둔다. `PrivateTmp=yes`인 MCP의 `/tmp`를 상주 Chrome이 읽을 수 있다고 가정하지 않는다. 새 포트·HTTP 서버·DevTools 문서 전송 방식은 추가하지 않는다.

## 운영 전제

- MCP와 상주 Chrome은 모두 `artex-runtime`(운영 UID 2000)으로 실행한다.
- `REPORT_OUTPUT_ROOT=/opt/arx-lab/data/report-pdf`는 두 프로세스가 같은 내용으로 읽는 실제 경로여야 한다. `/tmp`·`/var/tmp`나 그 아래로 향하는 심볼릭 링크를 출력 루트로 사용하지 않는다.
- 출력 루트 및 `session/job` 폴더는 기존 소유자·그룹 `artex-runtime:artex-runtime`, 모드 `0750`을 유지한다. 경로의 상위 폴더에도 같은 UID의 탐색 권한이 있어야 한다.
- `.md2pdf-*`는 `MkdirTemp`가 만드는 owner-only `0700` 폴더다. MCP와 Chrome이 같은 UID이므로 둘 다 읽을 수 있다. 내부 파일은 converter의 기존 생성 모드에 unit의 `UMask=0027`을 적용한다(통상 `0640`). 다른 UID를 위해 `0777`로 넓히지 않는다.
- MCP `PrivateTmp=yes`, Chrome의 기존 `PrivateTmp` 설정, `ReadWritePaths`의 기존 출력 루트, UID 방화벽의 `127.0.0.1:9223` 경계는 유지한다. 새 환경 변수나 unit 변경은 필요 없다. 저장소 unit의 예시 루트(`/opt/artex-lab`)와 실제 배포 루트(`/opt/arx-lab`)는 기존 배포 설정으로 맞춘다.

## 파일과 정리

| 경로 | 접근·수명 |
|---|---|
| `job/source/` | 기존 Markdown·로컬 이미지 stage. 원래 경로 검사를 유지한다 |
| `job/.md2pdf-*/pass1.html`, `pass2.html` | Chrome이 읽는 중간 문서. 이미지는 data URI로 내장하며 customer 글꼴은 두 프로세스가 읽는 `/usr/share/fonts/...`를 사용한다 |
| 같은 중간 폴더의 PDF·sections/pages JSON | 두 번의 인쇄와 TOC 분석용. 다른 job과 별개인 무작위 폴더다 |
| `job/report.html`, `report.pdf` | 기존 검증 이후 공개하는 최종 산출물. 레거시 HTML의 글꼴 내장 처리도 유지한다 |

정상 성공과 오류 반환 모두 중간 폴더를 정리한다. CLI는 `run()`의 defer를 마친 뒤 `main`에서 종료 코드를 적용한다. MCP도 자식 프로세스의 종료를 기다린 뒤 해당 job의 `.md2pdf-*`만 정리하므로 180초 timeout의 converter SIGKILL도 처리한다. 정리 실패는 `CONVERTER_WORKSPACE_CLEANUP_FAILED`로 job을 실패 처리한다. MCP 자체의 SIGKILL·호스트 중단·파일 시스템 삭제 실패는 자동 정리를 보장하지 않는다. 잔여 폴더가 있으면 해당 job의 실행 종료를 확인한 후 그 `.md2pdf-*`만 정리한다. 진행 중인 job이나 전체 출력 루트를 일괄 삭제하지 않는다.

v3와 `arx-pentest-report-1`은 이미 `job/report.html`을 `file://`로 전달한다. 글꼴·그림은 각 렌더러의 기존 인라인 데이터 방식이다. `wm-position-check`도 CLI가 만든 `wm-position-*` 임시 폴더의 HTML을 `file://`로 전달한다. unit 밖의 터미널·sudo 명령은 MCP의 전용 `/tmp`에 들어가지 않으므로 상주 Chrome과 전역 `/tmp`를 공유할 수 있다. 이 CLI 통과를 unit 내부 converter의 임시 경로 수용 증거로 대체하지 않는다. 검사 CLI를 별도 PrivateTmp 환경 안에서 실행한다면 그 명령에 한해 공유 임시 경로를 지정하고 소유자·정리를 확인해야 한다.

## 배포·수용

1. 검증한 브랜치의 커밋된 상태에서 report-mcp와 **새 converter**를 한 쌍으로 빌드한다. converter는 `scripts/build-converter.sh`의 customer 양식·physical-page 패치를 포함해야 한다. 구 converter만 재사용하면 이 수정은 반영되지 않는다.
2. 기존 게이트·활동 확인·백업·롤백 절차를 적용한다. 서비스 설치 전에는 사용자·경로·권한을 읽기 전용으로 확인한다.

```sh
systemctl show report-mcp.service arx-chrome-9223.service \
  -p User -p Group -p PrivateTmp -p ReadOnlyPaths -p ReadWritePaths -p InaccessiblePaths
stat -c '%U:%G %a %n' /opt/arx-lab/data/report-pdf
readlink -f /opt/arx-lab/data/report-pdf
```

3. 상주 Chrome의 IPv4 endpoint가 응답하는 상태에서 레거시 Markdown(로컬 이미지 포함), v3, 새 형식을 각각 시험한다. 레거시는 두 PASS 모두 `Using running Chrome`과 job 아래 `.md2pdf-*` 경로를 사용해야 한다. PDF·TOC·폰트·다운로드·마스킹 검사를 함께 확인한다.
4. 완료 후 `.md2pdf-*`가 남지 않았는지, 다른 job과 자료가 섞이지 않았는지 확인한다. 재시작·운영 설치 수용은 로컬 시험과 구분한다.
5. 실패하면 추가 변경 없이 기존 바이너리·unit의 승인된 롤백 절차를 따른다. `PrivateTmp` 해제·방화벽 확대를 임시 우회로 적용하지 않는다.

macOS 회귀 시험은 `sandbox-exec`로 Chrome의 특정 임시 경로 읽기만 막는다. Go는 읽을 수 있고 Chrome은 `ERR_FILE_NOT_FOUND`를 내는 조건에서, 수정 전 converter는 레거시만 실패하고 수정 후 세 경로가 같은 상주 Chrome으로 성공해야 한다. 운영 Ubuntu namespace의 실제 read-back은 배포 단계에서 별도로 확인한다.
