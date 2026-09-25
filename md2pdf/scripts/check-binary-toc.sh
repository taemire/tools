#!/usr/bin/env bash
set -euo pipefail

tool_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
binary="${1:-$tool_dir/md2pdf}"
if [[ ! -x "$binary" ]]; then
  echo "[FAIL] 실행 가능한 md2pdf 바이너리가 없습니다: $binary" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cat > "$tmpdir/input.md" <<'EOF'
# 목차 검증 문서

## 첫 번째 절

짧은 한글 본문입니다.

## 두 번째 절

두 번째 한글 본문입니다.
EOF

if ! "$binary" -i "$tmpdir/input.md" -o "$tmpdir/output.pdf" -title '목차 검증 문서' > "$tmpdir/run.log" 2>&1; then
  cat "$tmpdir/run.log" >&2
  echo "[FAIL] md2pdf 변환이 실패했습니다." >&2
  exit 1
fi

# 소스 테스트만 통과해도 배포 바이너리가 오래될 수 있으므로 실제 바이너리의
# 최종 PDF 검증 단계와 목차 번호 세 항목을 확인한다.
if ! grep -Fq '[VALIDATION] Checking final PDF destinations' "$tmpdir/run.log"; then
  echo "[FAIL] 바이너리에 최종 PDF 목차 검증 단계가 없습니다." >&2
  exit 1
fi
numbered="$(grep -Ec 'class="toc-page">[1-9][0-9]*</span>' "$tmpdir/output.html" || true)"
if [[ "$numbered" -ne 3 ]]; then
  echo "[FAIL] 목차 번호가 있는 항목은 $numbered/3개입니다." >&2
  exit 1
fi

if command -v qpdf >/dev/null 2>&1; then
  qpdf --check "$tmpdir/output.pdf" >/dev/null
fi
echo '[PASS] 실행 바이너리의 목차 3개와 최종 PDF 검증을 확인했습니다.'
