// Package renderer converts HTML documents to PDF using Chrome/Chromium.
// Extracted from html2pdf project.
package renderer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Options for PDF rendering
type Options struct {
	Landscape bool
	Scale     float64
	Timeout   int // seconds
}

// RenderToPDF converts an HTML file to a PDF file using Chrome/Chromium.
func RenderToPDF(inputHTML, outputPDF string, opts Options) error {
	// Validate input
	if inputHTML == "" {
		return fmt.Errorf("input HTML file path is required")
	}

	// Default output
	if outputPDF == "" {
		outputPDF = strings.TrimSuffix(inputHTML, filepath.Ext(inputHTML)) + ".pdf"
	}

	// Resolve absolute path for file:// URL
	absInput, err := filepath.Abs(inputHTML)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Default scale
	scale := opts.Scale
	if scale <= 0 {
		scale = 1.0
	}

	// Default timeout
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 300 // 5 minutes
	}

	fileURL := "file://" + absInput
	fmt.Printf("[INFO] Converting: %s\n", absInput)
	fmt.Printf("[INFO] Output: %s\n", outputPDF)

	// Create Chrome context
	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("disable-background-networking", true),
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Navigate and wait
	if err := chromedp.Run(ctx,
		chromedp.Navigate(fileURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return fmt.Errorf("failed to load page: %w", err)
	}

	// Wait for Mermaid rendering
	mermaidDone := false
	_ = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.mermaid') !== null`, &mermaidDone),
	)
	if mermaidDone {
		fmt.Println("[INFO] Waiting for Mermaid diagrams...")
		_ = chromedp.Run(ctx, chromedp.Sleep(5*time.Second))
	}

	// Print to PDF
	var buf []byte
	printParams := page.PrintToPDF().
		WithPrintBackground(true).
		WithPreferCSSPageSize(true).
		WithScale(scale).
		WithLandscape(opts.Landscape).
		WithMarginTop(0).
		WithMarginBottom(0).
		WithMarginLeft(0).
		WithMarginRight(0)

	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = printParams.Do(ctx)
			return err
		}),
	); err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Ensure output directory exists
	if dir := filepath.Dir(outputPDF); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Write PDF
	if err := os.WriteFile(outputPDF, buf, 0644); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}

	fmt.Printf("[SUCCESS] Generated PDF: %s (%.1f MB)\n", outputPDF, float64(len(buf))/(1024*1024))
	return nil
}
