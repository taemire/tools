# md2pdf

2-Pass markdown → PDF generator with automatic TOC and accurate page numbers.

## Features

- Markdown to PDF conversion with embedded Korean fonts
- Automatic TOC generation (2-pass anchor-to-PDF-page resolution, with final validation)
- `_sidebar.md` aware file ordering
- Skips `README.md` automatically (when used as input directory)
- HTML output option (`-html-only`)

## Usage

```bash
md2pdf -i <input_dir> -o <output.pdf> [-title "Title"] [-subtitle "Subtitle"] [-author "Author"]
```

### Required flags

| Flag | Description |
|:---|:---|
| `-i <path>` | Input directory or a single Markdown file (relative assets are resolved from that file) |
| `-o <file>` | Output PDF (or HTML) file path |

### Optional flags

| Flag | Default | Description |
|:---|:---|:---|
| `-title` | — | Document title |
| `-subtitle` | — | Subtitle |
| `-author` | — | Author / company name |
| `-header` | — | Print header |
| `-footer` | — | Print footer |
| `-template` | `report` | Template name |
| `-html-only` | false | Generate HTML only (skip PDF conversion) |
| `-c` / `-config` | — | AUTHORS.yml config path |
| `-skip` | 0 (auto) | Front-matter pages to subtract; auto uses the first section's PDF destination |
| `-offset` | 0 | Accepted for compatibility; currently has no effect |

## Build

```bash
go build -o md2pdf .
./scripts/check-binary-toc.sh
```

`go test ./...`는 현재 소스를 검사하지만 저장된 `md2pdf` 실행 파일이 최신인지는 검사하지 않습니다. 소스를 바꾸거나 바이너리를 배포할 때는 다시 빌드한 뒤 위 스모크 테스트를 실행하세요. 이 테스트는 실제 바이너리로 PDF를 생성하고, 세 개 목차 항목에 양수 페이지 번호가 있으며 최종 PDF 목적지 검증 단계가 실행되는지 확인합니다.

## Notes

- `README.md` inside the input directory is automatically skipped.
- `_sidebar.md` (if present) determines file order; otherwise files are sorted by filename.
- `appendix/` subdirectories are also included.
- Korean fonts are embedded — Korean documents render correctly.
- 인쇄용 `report`·`modern`·`default` 템플릿은 제목(H1~H4)과 바로 다음 블록이 서로 다른 페이지로 분리되지 않도록 페이지 나눔 규칙을 적용합니다. Chrome 조판 결과는 실제 PDF를 렌더링해 확인하세요.

## TOC validation and CLI compatibility

Each file and H2 heading gets a unique ASCII TOC anchor (`md2pdf-section-N[-heading-M]`). The analyzer reads the PDF catalog's named destinations, including destination name trees, to resolve these anchors to physical pages. It does not search extracted title text. Korean, English, wrapped headings, and repeated titles therefore use the same mapping path. Displayed numbers start at the first body section (physical page minus front-matter pages).

PDF conversion exits **1** with `[ERROR]` and `TOC validation failed` if a destination is missing, an ID/page is invalid or ambiguous, or the final PDF's pagination differs from the numbers printed in the TOC. Missing mappings are never silently rendered as blank page numbers. Pass 2 runs in temporary files; mapping validation failure leaves existing output PDF/HTML files untouched. An output I/O error during publication is still reported as an error and can leave a partially written output.

Existing flags, output filenames, success exit code **0**, and HTML-only operation remain available. No permissive fallback flag was added: scripts should check the exit code before using the output, rather than treating file existence or an intermediate `[SUCCESS]` log as completion. `-skip N` still subtracts N physical pages; values that produce a zero/negative TOC page now fail. `-html-only` does not perform PDF page validation.

TOC number space is reserved in both passes, including for long titles. The `report`, `default`, and `modern` templates display page numbers. Legacy Markdown fragment anchors are retained; hand-written links to repeated legacy IDs can still be ambiguous, while generated TOC links use the unique IDs. The `md2pdf-section-` prefix is reserved. Destination-less PDFs and ambiguous page dictionaries are rejected rather than guessed. Pagination changes between passes require correcting the document/rendering inputs and rerunning; the tool does not retry indefinitely.

See [the original research and implementation update](docs/toc-mapping-research.md) and [the fix validation report](docs/toc-mapping-fix-validation.md).

## Browser connection

md2pdf starts its own headless Chrome on a random DevTools port by default. Two environment variables change that:

| Variable | Effect |
|---|---|
| `MD2PDF_CHROME_URL` | Reuse a running Chrome, for example `http://127.0.0.1:9223`. Only a numeric loopback address is accepted. md2pdf reads `/json/version` and pins the advertised WebSocket URL to this host and port, so a Chrome that advertises `ws://[::1]:…` or `ws://localhost:…` is still reached over the configured address. If the endpoint does not answer and `MD2PDF_DEVTOOLS_PORT` is set, md2pdf starts its own Chrome instead. |
| `MD2PDF_DEVTOOLS_PORT` | Start the own Chrome on this fixed port bound to `127.0.0.1` (for hosts whose firewall allows only that port). md2pdf first checks that `127.0.0.1:<port>` is free and fails with `DEVTOOLS_PORT_BUSY` otherwise. When the port is taken, Chrome 156 falls back to `[::1]:<port>` even with `--remote-debugging-address=127.0.0.1`; a connection failure after such an advertisement is reported as `DEVTOOLS_ADDRESS_FAMILY_MISMATCH`. |

A reused Chrome is reached over an unauthenticated loopback DevTools port. Each render runs in a new browser context created with `disposeOnDetach`, so cookies, storage and cache are not shared with other users of that Chrome and are discarded afterwards. Use `MD2PDF_CHROME_URL` only on a single-tenant host where that port belongs to a Chrome dedicated to the report service (the ARTEX host restricts it per UID with iptables); do not use it on shared multi-user hosts.

`renderer/devtools_test.go` covers the address pinning, the busy-port refusal and, with a local Chrome, reuse of a resident Chrome on the same port (`MD2PDF_TEST_CHROME` selects the executable).

## Tests

```bash
go test ./...
go vet ./...
go build -o /tmp/md2pdf .
gofmt -l main.go main_test.go analyzer/*.go converter/*.go renderer/*.go
```

The full test suite includes actual Chrome/Chromium PDF rendering and CLI failure checks; Chrome/Chromium must be installed and launchable. These tests are not skipped. It covers named destinations, missing/duplicate IDs, all three templates, Korean/English/mixed and multiline headings, repeated titles, TOC/body confusion, pagination drift, and preservation of existing outputs on mapping failure. Each CLI test has a 90-second timeout.

## License

Internal tool — please refer to repository policy.
