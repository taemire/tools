# md2pdf

2-Pass markdown → PDF generator with automatic TOC and accurate page numbers.

## Features

- Markdown to PDF conversion with embedded Korean fonts
- Automatic TOC generation (2-pass page-number resolution)
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
| `-i <dir>` | Input directory containing markdown files |
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
| `-skip` | 0 (auto) | Pages to skip during TOC analysis |
| `-offset` | 0 | Page-number offset |

## Build

```bash
go build -o md2pdf .
```

## Notes

- `README.md` inside the input directory is automatically skipped.
- `_sidebar.md` (if present) determines file order; otherwise files are sorted by filename.
- `appendix/` subdirectories are also included.
- Korean fonts are embedded — Korean documents render correctly.

## Known Limitations & Roadmap

**Same H2 heading text collision**: When multiple sub-documents in the same input directory share identical H2 headings (e.g. "0. Document Purpose", "9. References"), the TOC currently maps all duplicates to the last occurrence's page. A manual workaround is to add a unique prefix to each file's H2 (e.g. `## A.0. Document Purpose`).

**Research & roadmap**: See [`docs/toc-mapping-research.md`](docs/toc-mapping-research.md) for the root-cause analysis, 7 candidate solutions, and the proposed Phase 1 / 2 / 3 improvement plan (~280-line PR estimate).

## License

Internal tool — please refer to repository policy.
