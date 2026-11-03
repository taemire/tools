package main

import (
	"context"
	"errors"
	"strings"

	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"golang.org/x/sync/errgroup"
	"report-mcp/artexmd"
)

// Glyph coverage alone cannot prove the browser chose the embedded face for a
// combining sequence or emoji presentation. Check its actual text-node fonts.
func checkV3DOMFonts(ctx context.Context) error {
	if err := css.Enable().Do(ctx); err != nil {
		return err
	}
	root, err := dom.GetDocument().WithDepth(0).Do(ctx)
	if err != nil {
		return err
	}
	nodes, err := dom.QuerySelectorAll(root.NodeID, `[data-artex-inline],.url-piece,h3,.eyebrow,.kpi-value,.kpi-label,th,td,.header`).Do(ctx)
	if err != nil {
		return err
	}
	group, c := errgroup.WithContext(ctx)
	group.SetLimit(8)
	for _, node := range nodes {
		if c.Err() != nil {
			break
		}
		id := node
		group.Go(func() error {
			fonts, err := css.GetPlatformFontsForNode(id).Do(c)
			if err != nil {
				return err
			}
			for _, font := range fonts {
				if font.GlyphCount > 0 && !font.IsCustomFont {
					return errors.New("RENDER_GLYPH_UNAVAILABLE")
				}
			}
			return nil
		})
	}
	return group.Wait()
}

// Also cover print-only text (page-margin footers and page numbers) using the
// embedded PostScript identities of the exact fonts supplied to the renderer.
func checkV3FontUsage(ctx context.Context, pdf string, fonts []artexmd.Font, fallbacks []artexmd.FallbackFont) error {
	expected, err := artexmd.ExpectedPDFFonts(fonts, fallbacks)
	if err != nil {
		return err
	}
	output, err := run(ctx, "pdffonts", pdf)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return errors.New("PDF_FONT_METADATA_INVALID")
	}
	for _, line := range lines[2:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if len(name) > 7 && name[6] == '+' {
			prefix := true
			for _, r := range name[:6] {
				prefix = prefix && r >= 'A' && r <= 'Z'
			}
			if prefix {
				name = name[7:]
			}
		}
		if !expected[name] {
			return errors.New("RENDER_GLYPH_UNAVAILABLE")
		}
	}
	return nil
}
