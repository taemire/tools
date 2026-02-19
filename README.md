# Common Development Tools (tools)

다양한 프로젝트(`tkcli`, `tkadmin`, `codesign_service`)에서 공통으로 사용되는 개발, 빌드, 문서화 도구 모음입니다.

## 📚 문서 생성 도구 (Documentation)

### 1. [md2pdf](md2pdf/) ⭐ (통합 바이너리)
- **기능**: Markdown → HTML → PDF **올인원 변환기** (단일 Go 바이너리).
- **구조**: `converter/` (MD→HTML) + `renderer/` (HTML→PDF) + `analyzer/` (PDF 분석) 패키지 통합.
- **사용법**:
  ```bash
  # PDF 생성
  md2pdf -i docs/manual -o manual.pdf -title "사용자 매뉴얼" -version "1.0.0"
  # HTML만 생성
  md2pdf -i docs/manual -o manual.html -html-only
  ```
- **위치**: `md2pdf/` (Go 소스)

### 2. [md2pdf_v2](md2pdf_v2.bat) (호환 래퍼)
- **파일**: `md2pdf_v2.bat` (Windows), `md2pdf_v2.sh` (macOS/Linux)
- **기능**: `md2pdf` 바이너리의 래퍼 스크립트. 기존 호출 호환 유지.

---

## 🛠️ 개발 유틸리티 (Utilities)

### 3. [revlog](revlog.bat)
- **파일**: `revlog.bat` (Windows), `revlog.sh` (macOS/Linux)
- **기능**: Git 리포지토리의 커밋 히스토리 및 태그 정보를 직관적인 그래프로 조회.
- **사용법**: `revlog [-n count]`

### 4. [outlook_crawler](outlook_crawler)
- **기능**: Outlook 이메일 데이터를 수집 및 분석하는 도구.
- **위치**: `outlook_crawler/` (Node.js/Python 등)

### 5. [mp4towebp.bat](mp4towebp.bat)
- **기능**: MP4 동영상을 고효율 WebP 애니메이션으로 변환 (문서 첨부 최적화).
- **특징**: FFmpeg 자동 설치 지원.
- **사용법**: `mp4towebp.bat input.mp4 [output.webp]`

---

## 🗄️ 아카이브 (Archived)

사용되지 않거나 대체된 도구들은 `_archive/` 디렉토리로 이동되었습니다.

- **md2html (v1)**: `md2html_v2`로 대체됨.
- **md2html_v2**: `md2pdf` 통합 바이너리의 `converter/` 패키지로 흡수됨.
- **html2pdf**: `md2pdf` 통합 바이너리의 `renderer/` 패키지로 흡수됨.
- **md2pdf (v1)**: `md2pdf_v2` 스크립트 방식(html2pdf 기반)으로 대체됨.
- **md2pdf_v2 (Go Source)**: Go 네이티브 PDF 생성 시도 버전. 중단됨.

---

## 📦 설치 및 사용

이 프로젝트는 독립적으로 클론하여 사용하거나, 다른 프로젝트의 상위 `tools/` 디렉토리에 배치하여 사용합니다.

```bash
git clone ssh://taemire@code.myds.me:65022/volume1/git/tools.git
```
