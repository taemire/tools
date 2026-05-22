# md2pdf TOC Mapping Research — Same H2 Heading Collision

> Date: 2026-05-22 · Based on real-world multi-document PDF compilation use case · Go source analysis + Go PDF ecosystem comparison

## 0. Purpose

This document analyzes a **TOC page mapping error** discovered when compiling multiple markdown reports into a single PDF using `md2pdf`. It provides root-cause analysis, **7 candidate solutions** with comparison, **Top 3 priority recommendations**, and **Go source modification proposals** as an internal improvement roadmap.

## 1. Problem Diagnosis — Root Cause

### 1.1 Symptom (real-world measurement, 2026-05-22)

When combining 7 independent reports (each starting with the same H2 sections such as "0. Document Purpose", "9. References") into one PDF:

| Section | Expected page | **Wrongly mapped to** |
|---|---|---|
| Report A — `0. Document Purpose` | p.1 | ❌ **p.29** |
| Report B — `0. Document Purpose` | p.7 | ❌ **p.29** |
| Report C — `0. Document Purpose` | p.14 | ❌ **p.29** |
| Report D — `0. Document Purpose` | p.17 | ❌ **p.29** |
| Report E — `0. Document Purpose` | p.21 | ❌ **p.29** |
| Report F — `0. Document Purpose` | p.25 | ❌ **p.29** |
| Report G — `0. Document Purpose` | p.29 | ✅ p.29 (only the last one is correct) |
| Report B — `8. EP-XX Update` | p.10 | ❌ **p.20** |
| Report B — `9. References` | p.13 | ❌ **p.28** |

→ When **the same H2 title text** appears in multiple sub-documents, the TOC binds **all duplicates to the last occurrence's page** (Z-order last write).

### 1.2 Root Cause — Go Source Analysis

**`analyzer/analyzer.go:113`** — mapping logic:

```go
// Search from body pages
startPage := actualSkipPages + 1
lastFoundPage := startPage
for i := range sections {
    found := false
    for pageNum := lastFoundPage; pageNum <= totalPages; pageNum++ {
        // ...
        if containsTitle(text, sections[i].Title) {
            docPageNum := pageNum - actualSkipPages
            sections[i].Page = docPageNum  // ← per-iter set
            lastFoundPage = pageNum         // ← monotonically increasing
            found = true
            break
        }
    }
}
```

**Three issues**:

| # | Issue | Impact |
|---|---|---|
| 1 | **`containsTitle` text-based contains** — no unique anchor | Same title collides on string matching |
| 2 | **`lastFoundPage` monotonic increase** — cannot rewind | After Report G section is processed, prior sections cannot be re-scanned |
| 3 | **renderer uses `map[title]page` lookup** — sections array's *unique ID* is not used | Last iteration's value overwrites all prior entries with same title → TOC renders all duplicates with the same final page |

→ **Root cause = renderer's `title → page` map lookup**, ignoring the sections array's unique IDs.

## 2. Seven Candidate Solutions

### Solution 1: Anchor-Based Mapping (recommended ★★★)

Assign each markdown heading a *unique HTML anchor*. Map anchor → page via PDF link annotations.

```go
// renderer.go
heading_id := fmt.Sprintf("%s__%s", file_id, slug(heading_text))
// HTML: <h2 id="01-report-A__0-document-purpose">0. Document Purpose</h2>
```

**Pros**:
- Unique anchor — duplicate heading text becomes unique via file_id prefix
- Natural use of PDF link annotations
- Markdown-standard (GitHub heading anchors)

**Cons**:
- Analyzer must read anchors from PDF instead of text-matching
- TOC rendering needs anchor → page lookup (instead of title → page)

**Estimated effort**: ~50–100 lines of Go source.

### Solution 2: PDF Outline / Bookmark API (highly recommended ★★★★)

Use Chromium / wkhtmltopdf's *PDF outline (bookmark tree)*. Extract outline directly via `pdfcpu` or `unipdf` — avoiding text matching entirely.

```go
// converter.go
chromium_args = "--enable-pdf-outline --pdf-outline-headers=h1,h2,h3"

// analyzer.go (replacement)
import "github.com/pdfcpu/pdfcpu/pkg/api"
outline, _ := api.ExtractBookmarksFile(pdfPath, ...)
// outline itself is an accurate anchor → page mapping
```

**Pros**:
- Completely avoids text matching — outline tree is authoritative
- PDF standard — every PDF reader recognizes outlines
- Removes dependency on external `pdfinfo` / `pdfgrep` tools

**Cons**:
- Chromium's `--enable-pdf-outline` support must be verified
- Adds dependency on `pdfcpu` or `unipdf`

**Estimated effort**: ~100–200 lines.

### Solution 3: File-Aware Section Tracking (simple ★★)

Per-file namespace — same H2 becomes `(file_name, heading_text)` composite key.

```go
// analyzer.go
type SectionKey struct {
    FileName string
    Title    string
}
sectionMap := make(map[SectionKey]int)  // (file, title) → page

// or, simpler
type SectionInput struct {
    ID        string
    Title     string
    FileName  string  // ← new field
    Page      int
}
```

**Pros**:
- Minimal code change — only map key changes
- User doesn't need manual prefix
- File source naturally provides uniqueness

**Cons**:
- TOC rendering may need to display file name (currently H1 display is preserved)
- Anchor-based navigation is separate

**Estimated effort**: ~30–50 lines.

### Solution 4: Automatic Heading Slug Uniquification (simple ★★★)

GitHub-style heading slugs — second and later occurrences of the same text become `slug-1`, `slug-2`, etc.

```go
// renderer.go
slugCounts := make(map[string]int)
for _, heading := range allHeadings {
    baseSlug := slug(heading.Text)
    if count, exists := slugCounts[baseSlug]; exists {
        slugCounts[baseSlug] = count + 1
        heading.Slug = fmt.Sprintf("%s-%d", baseSlug, count)
    } else {
        slugCounts[baseSlug] = 1
        heading.Slug = baseSlug
    }
}
```

**Pros**:
- Standard markdown-to-HTML pattern (GitHub, GitLab)
- Familiar mental model

**Cons**:
- Title display is unchanged (only slug is unique) — TOC shows `0. Document Purpose` 7 times identically
- PDF text matching remains title-based and still collides — must combine with Solution 1

**Estimated effort**: ~30 lines.

### Solution 5: One-Pass + Native PDF Links (innovative ★★★★)

Replace the existing *2-pass + text matching* with *1-pass + HTML anchor → PDF link annotation*. TOC uses internal links (`href="#A0-..."`) — no separate page-number calculation needed.

```go
// converter.go
// HTML embeds: <a href="#section-A-0">A.0. Document Purpose ............. [link]</a>
// PDF: link annotation is rendered as a clickable link
// TOC page numbers can be omitted or derived from the link target's resolved page
```

**Pros**:
- Removes Pass 2 — ~50% build-time reduction
- Clickable TOC (jump to target page)
- No text matching — fundamentally correct

**Cons**:
- Page numbers next to TOC entries (e.g. `.... 3`) aren't auto-rendered — would need a post-process step to extract each link's target page and inject it
- Requires Chromium PDF link option validation

**Estimated effort**: ~150–200 lines + Chromium option validation.

### Solution 6: pdfcpu Bookmark Tree (innovative ★★★)

After PDF generation, use `pdfcpu`'s `AddBookmarks` API — automatically map markdown headings → PDF bookmark tree.

```go
import "github.com/pdfcpu/pdfcpu/pkg/api"

// After PDF generation
bookmarks := []*pdfcpu.Bookmark{...}  // built from sections + page mapping
api.AddBookmarksFile(pdfPath, bookmarks, ...)
```

**Pros**:
- PDF reader's outline panel is enabled
- Removes text matching
- Markdown TOC and PDF outline coexist

**Cons**:
- Two TOC sources to keep in sync (markdown TOC vs PDF outline)
- Adds `pdfcpu` dependency

**Estimated effort**: ~100 lines.

### Solution 7: Sidebar-Aware Multi-File Section ID

Use `_sidebar.md`'s file ordering to auto-prefix *section ID* with file index (0..N).

```go
// renderer.go
for fileIdx, mdFile := range sidebarFiles {
    for headingIdx, heading := range parseHeadings(mdFile) {
        section.ID = fmt.Sprintf("file%d-heading%d", fileIdx, headingIdx)
        section.DisplayTitle = heading.Text  // user-facing stays the same
    }
}
```

**Pros**:
- Reuses `_sidebar.md` (already recognized by md2pdf)
- Section ID becomes naturally unique via file index
- Title display unchanged

**Cons**:
- Fragile when `_sidebar.md` is absent (falls back to directory sort)
- Less effective for single-file usage

**Estimated effort**: ~50 lines.

## 3. Top 3 Priorities

| Rank | Solution | Score | Reason |
|---|---|---|---|
| 🥇 1 | **#2 PDF Outline / Bookmark API** (★★★★) | 9/10 | Avoids text matching via external library (`pdfcpu`). Uses PDF standard — long-term stability |
| 🥈 2 | **#5 One-Pass + Native PDF Links** (★★★★) | 9/10 | Reduces build time + clickable TOC. Page-number display needs supplemental handling |
| 🥉 3 | **#1 Anchor-Based Mapping** (★★★) | 8/10 | Minimal source change + Markdown-standard. Can be applied immediately |

### Combined Roadmap

**Phase 1 (immediate)**: Solutions **#3 + #4** — file-aware sections + slug uniquification — 30–80 lines. Permanently resolves the manual-prefix workaround that current users apply.

**Phase 2 (medium term)**: Solutions **#1 + #6** — Anchor + pdfcpu bookmark — ~200 lines + PDF outline usage.

**Phase 3 (long term)**: Solutions **#2 + #5** — PDF Outline + Native Links — full architecture redesign.

## 4. Manual Workaround vs. Permanent Fix

Currently affected users apply a manual H2 prefix (e.g. `sed 's/^## /## A./'`) as a workaround. Permanent fix (Phase 1) comparison:

| Aspect | Current (manual prefix) | **Permanent fix (Phase 1)** |
|---|---|---|
| User burden | `sed` to add prefix to each H2 | Automatic |
| TOC readability | "A.0. Document Purpose" (less clean) | "0. Document Purpose" (clean) |
| Multi-document integration | Re-apply prefix every time | Automatic |
| Tool reusability | Project-specific workaround | Globally usable |
| Maintenance cost | Review each conversion | 0 |

→ **Phase 1 permanent fix** is better for both output quality and operational efficiency.

## 5. Proposed Go Source Changes (Phase 1)

### 5.1 `analyzer/analyzer.go`

```go
type SectionInput struct {
    ID          string
    Title       string
    FileName    string  // ← new (sidebar or markdown filename)
    AnchorSlug  string  // ← new (unique slug)
    SubHeadings []SubHeading
}

type SubHeading struct {
    ID         string
    Title      string
    AnchorSlug string  // ← new unique slug (e.g. "file01__0-document-purpose")
    Page       int
}

// Replace containsTitle with containsAnchor or (file_id, title) composite search
func containsSectionUnique(text, fileName, title string) bool {
    // Search PDF text for *file marker* + *title* simultaneously
    // Or search for *anchor link* (Solution 1)
}
```

### 5.2 `renderer/renderer.go`

```go
type Heading struct {
    Title       string
    FileID      string  // ← new
    AnchorSlug  string  // ← new
    Page        int
}

// TOC rendering uses heading.AnchorSlug → Page
// Current heading.Title → Page map collision is resolved
tocEntries := map[string]int{}
for _, h := range allHeadings {
    tocEntries[h.AnchorSlug] = h.Page  // unique key
}
// Use anchor slug for lookup; title is display only
```

### 5.3 `main.go`

```go
// New CLI flags
flag.Bool("unique-anchor", true, "Auto-add file_id prefix to anchor slug")
flag.Bool("anchor-based-lookup", true, "Use anchor-based page lookup instead of title text matching")
```

### 5.4 Estimated PR Size

| File | Lines changed |
|---|---|
| `analyzer/analyzer.go` | ~50 |
| `renderer/renderer.go` | ~80 |
| `converter/converter.go` | ~30 |
| `main.go` | ~20 |
| New tests (`*_test.go`) | ~100 |
| **Total** | **~280 lines** |

## 6. Verification — Real-World Use Case

The TOC mapping bug was discovered while compiling **7 EDR vendor comparison reports** into a single PDF for sales material. Each report independently used identical H2 sections like "0. Document Purpose", "9. References". Out of ~80 H2 entries, ~25 were mis-mapped due to title collisions. The manual workaround (adding A./B./C. prefixes) was applied, restoring TOC accuracy.

This is presented as a representative use case for documents that compose multiple equally-structured sub-reports — a common pattern in vendor comparisons, multi-product analyses, and consolidated quarterly reports.

## 7. Next Steps Candidate

- (a) Phase 1 PR — `analyzer.go` + `renderer.go` file-aware + slug uniquification (~80 lines)
- (b) `pdfcpu` integration PoC (Phase 2 Solution #6)
- (c) Chromium `--enable-pdf-outline` validation PoC (Phase 2 Solution #2)
- (d) Existing user documentation update — describe the manual workaround as transitional

## 8. References

### md2pdf Source

- `main.go`
- `analyzer/analyzer.go` (particularly `:113` `containsTitle`)
- `renderer/renderer.go`
- `converter/converter.go`

### Go PDF Libraries

- pdfcpu — https://github.com/pdfcpu/pdfcpu
- unipdf — https://github.com/unidoc/unipdf
- go-pdf/fpdf — https://github.com/jung-kurt/gofpdf

### Markdown-to-PDF Tools (comparison)

- mdpdf (Node.js, Puppeteer) — HTML anchor + PDF link
- pandoc — `--pdf-engine=xelatex` cross-references
- md-to-pdf (npm) — Puppeteer + Chromium PDF link
- mkdocs-pdf-export-plugin — section-aware page numbering

### PDF Standards

- ISO 32000-1:2008 — PDF Outline / Bookmark Tree
- ISO 32000-2:2020 — PDF 2.0 link annotations
