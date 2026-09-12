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
```

## Notes

- `README.md` inside the input directory is automatically skipped.
- `_sidebar.md` (if present) determines file order; otherwise files are sorted by filename.
- `appendix/` subdirectories are also included.
- Korean fonts are embedded — Korean documents render correctly.

## TOC validation and CLI compatibility

Each file and H2 heading gets a unique ASCII TOC anchor (`md2pdf-section-N[-heading-M]`). The analyzer reads the PDF catalog's named destinations, including destination name trees, to resolve these anchors to physical pages. It does not search extracted title text. Korean, English, wrapped headings, and repeated titles therefore use the same mapping path. Displayed numbers start at the first body section (physical page minus front-matter pages).

PDF conversion exits **1** with `[ERROR]` and `TOC validation failed` if a destination is missing, an ID/page is invalid or ambiguous, or the final PDF's pagination differs from the numbers printed in the TOC. Missing mappings are never silently rendered as blank page numbers. Pass 2 runs in temporary files; mapping validation failure leaves existing output PDF/HTML files untouched. An output I/O error during publication is still reported as an error and can leave a partially written output.

Existing flags, output filenames, success exit code **0**, and HTML-only operation remain available. No permissive fallback flag was added: scripts should check the exit code before using the output, rather than treating file existence or an intermediate `[SUCCESS]` log as completion. `-skip N` still subtracts N physical pages; values that produce a zero/negative TOC page now fail. `-html-only` does not perform PDF page validation.

TOC number space is reserved in both passes, including for long titles. The `report`, `default`, and `modern` templates display page numbers. Legacy Markdown fragment anchors are retained; hand-written links to repeated legacy IDs can still be ambiguous, while generated TOC links use the unique IDs. The `md2pdf-section-` prefix is reserved. Destination-less PDFs and ambiguous page dictionaries are rejected rather than guessed. Pagination changes between passes require correcting the document/rendering inputs and rerunning; the tool does not retry indefinitely.

See [the original research and implementation update](docs/toc-mapping-research.md) and [the fix validation report](docs/toc-mapping-fix-validation.md).

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
