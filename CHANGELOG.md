# 변경 이력 (Changelog)

이 문서는 `tools` 프로젝트의 주요 변경 사항을 기록합니다.

## [Unreleased]

### ✨ 기능 개선
- **md2html**: Docsify 알림 문법(`!>`, `?>`) 파싱 및 스타일링 개선
  - `**제목**: 내용` 패턴 감지 시 제목/본문 분리 (콜론 제거)
  - `postProcessAlerts` 함수 리팩토링으로 안정적인 HTML 변환
- **layout_report.html**: Font Awesome CDN 추가 및 Alert 스타일 통합
- **layout_modern.html**: Alert 스타일 통합

### 📝 문서화
- **.agent/rules.md**: UI 목업 및 스타일링 규칙 추가
  - CSS 중앙 관리 원칙
  - 템플릿 순수성 원칙
  - Docsify 호환성 유지 원칙

## [v0.1.0] - 2025-12-20

### 🎉 초기 릴리스
- **저장소 통합**: `tkcli`, `codesign_service` 등에 흩어져 있던 도구들을 `tools` 저장소로 통합.
- **html2pdf**: CDP 로그 노이즈 억제(Quiet Mode) 적용.
- **check_version.bat**: Git 히스토리 및 태그 조회 스크립트 추가.
- **mp4towebp.bat**: FFmpeg 자동 다운로드 및 설치 기능이 포함된 WebP 변환 스크립트 추가.
- **md2html**: Markdown 변환기 및 템플릿 