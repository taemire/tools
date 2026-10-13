#!/usr/bin/env bash
set -euo pipefail
report_component=$(cd "$(dirname "$0")/.." && pwd)
report_source=$(cd "$report_component/../md2pdf" && pwd)
report_build=${1:?new absolute build directory required}
report_output=${2:?absolute output binary path required}
[[ "$report_build" = /* && "$report_output" = /* && ! -e "$report_build" ]] || { echo 'Use absolute paths and a new build directory' >&2; exit 2; }
mkdir -p "$report_build" "$(dirname "$report_output")"
tar -C "$report_source" --exclude=.git --exclude='*.pdf' --exclude='md2pdf' -cf - . | tar -C "$report_build" -xf -
patch --batch --forward -d "$report_build" -p1 < "$report_component/scripts/md2pdf-runtime.patch"
cp "$report_component/templates/layout_customer.html" "$report_build/converter/templates/layout_customer.html"
cd "$report_build"
go test ./analyzer ./converter
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$report_output" .
