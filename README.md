# Common Development Tools (tools)

다양한 프로젝트(`tkcli`, `tkadmin`, `codesign_service`)에서 공통으로 사용되는 개발, 빌드, 문서화 도구 모음입니다.

## 🛠️ 포함된 도구

### 1. html2pdf
- **기능**: HTML 문서를 PDF로 변환 (Chrome/Chromium 기반)
- **특징**: `check_version`과 달리 외부 브라우저 엔진(chromedp)을 사용하여 고품질 렌더링 지원.
- **실행**: `html2pdf.exe`

### 2. md2html
- **기능**: Markdown 명세서를 HTML 문서로 변환
- **특징**: 커스텀 템플릿 지원, `_sidebar.md` 기반 네비게이션 생성.
- **실행**: `md2html.exe`

### 3. check_version.bat
- **기능**: Git 리포지토리의 커밋 히스토리 및 태그 정보를 직관적으로 조회.
- **사용법**: `check_version.bat [-n count]`

### 4. mp4towebp.bat
- **기능**: MP4 동영상을 고효율 WebP 애니메이션으로 변환. (문서 첨부용)
- **특징**: 시스템에 FFmpeg가 없으면 자동으로 다운로드 및 설치.
- **사용법**: `mp4towebp.bat input.mp4 [output.webp] [fps] [width]`

### 5. upx.exe
- **기능**: 실행 파일(Binary) 압축 및 크기 최적화.
- **특징**: `codesign_service`, `tkcli` 등 Go 바이너리 크기를 줄이는 데 사용.
- **사용법**: `upx.exe -9 -o output_compressed.exe input.exe`

## 📦 설치 및 사용

이 프로젝트는 독립적으로 클론하여 사용하거나, 다른 프로젝트의 상위 `tools/` 디렉토리로 배치하여 사용합니다.

```bash
git clone ssh://taemire@code.myds.me:65022/volume1/git/tools.git
```
