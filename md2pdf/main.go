// md2pdf - Unified Markdown to PDF converter
// Combines md2html (converter), html2pdf (renderer), and pdf_analyzer (analyzer)
// into a single binary with 2-Pass TOC page number injection.
//
// Usage: md2pdf -i <input_dir> -o <output.pdf> [options]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"md2pdf/analyzer"
	"md2pdf/converter"
	"md2pdf/renderer"
)

var (
	BuildVersion = "1.0.0"
	BuildTime    = ""
)

func main() {
	// CLI flags (Same as md2pdf_v2.sh)
	inputDir := flag.String("i", "", "Input directory containing markdown files (required)")
	outputFile := flag.String("o", "", "Output PDF file path (required)")

	// Document metadata
	title := flag.String("title", "", "Main title (overrides config)")
	subtitle := flag.String("subtitle", "", "Subtitle (overrides config)")
	version := flag.String("version", "", "Document version")
	author := flag.String("author", "", "Author/Company name (overrides config)")
	header := flag.String("header", "", "Header text for printed pages (overrides config)")
	footer := flag.String("footer", "", "Footer text for printed pages (overrides config)")

	// Config file (GNU-style: -c / --config)
	var configFile string
	flag.StringVar(&configFile, "c", "", "Config file path (AUTHORS.yml)")
	flag.StringVar(&configFile, "config", "", "Config file path (AUTHORS.yml)")

	// Template
	templateName := flag.String("template", "report", "Template name (default: report)")

	// Output mode
	htmlOnly := flag.Bool("html-only", false, "Generate HTML only (no PDF conversion)")

	// PDF options
	skipPages := flag.Int("skip", 0, "Number of pages to skip for TOC analysis (0 = auto-detect)")
	// offset is accepted for compatibility but we use skip internally
	_ = flag.Int("offset", 0, "Page number offset (for compatibility)")

	// Version flag
	showVersion := flag.Bool("v", false, "Show version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "md2pdf - Unified Markdown to PDF converter\n\n")
		fmt.Fprintf(os.Stderr, "Usage: md2pdf -i <input_dir> -o <output.pdf|.html> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("md2pdf v%s (%s)\n", BuildVersion, BuildTime)
		return
	}

	if *inputDir == "" || *outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	// === HTML-only mode ===
	if *htmlOnly {
		if !strings.HasSuffix(strings.ToLower(*outputFile), ".html") {
			*outputFile = strings.TrimSuffix(*outputFile, filepath.Ext(*outputFile)) + ".html"
		}

		fmt.Println("==============================================================================")
		fmt.Println(" md2pdf - HTML Generation (html-only mode)")
		fmt.Println("==============================================================================")
		fmt.Println()

		opts := converter.Options{
			InputDir:    *inputDir,
			OutputFile:  *outputFile,
			ConfigFile:  configFile,
			Title:       *title,
			Subtitle:    *subtitle,
			Version:     *version,
			Author:      *author,
			Header:      *header,
			Footer:      *footer,
			Template:    *templateName,
			EmbedImages: true,
			PDFMode:     false,
		}

		_, err := converter.ConvertToHTML(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] HTML generation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("==============================================================================")
		fmt.Printf("[SUCCESS] HTML generated: %s\n", *outputFile)
		fmt.Println("==============================================================================")
		return
	}

	// === PDF mode (default) ===
	if !strings.HasSuffix(strings.ToLower(*outputFile), ".pdf") {
		*outputFile += ".pdf"
	}

	fmt.Println("==============================================================================")
	fmt.Println(" md2pdf - 2-Pass PDF Generation with Accurate TOC Page Numbers")
	fmt.Println("==============================================================================")
	fmt.Println()

	// Create temp directory for intermediate files
	tmpDir, err := os.MkdirTemp("", "md2pdf-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	htmlPass1 := filepath.Join(tmpDir, "pass1.html")
	pdfPass1 := filepath.Join(tmpDir, "pass1.pdf")
	sectionsJSON := filepath.Join(tmpDir, "sections.json")
	pagesJSON := filepath.Join(tmpDir, "pages.json")
	htmlPass2 := filepath.Join(tmpDir, "pass2.html")

	// ======================================================================
	// PASS 1: Generate HTML without page numbers, then render to PDF
	// ======================================================================
	fmt.Println("[PASS 1] Generating HTML (without page numbers)...")

	baseOpts := converter.Options{
		InputDir:     *inputDir,
		OutputFile:   htmlPass1,
		ConfigFile:   configFile,
		Title:        *title,
		Subtitle:     *subtitle,
		Version:      *version,
		Author:       *author,
		Header:       *header,
		Footer:       *footer,
		Template:     *templateName,
		EmbedImages:  true,
		PDFMode:      true,
		SectionsJSON: sectionsJSON,
	}

	_, err = converter.ConvertToHTML(baseOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Pass 1 HTML generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[PASS 1] Converting HTML to PDF...")
	err = renderer.RenderToPDF(htmlPass1, pdfPass1, renderer.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Pass 1 PDF generation failed: %v\n", err)
		os.Exit(1)
	}

	// ======================================================================
	// ANALYSIS: Extract page numbers from Pass 1 PDF
	// ======================================================================
	fmt.Println("[ANALYSIS] Analyzing PDF for page numbers...")

	result, err := analyzer.AnalyzePDF(pdfPass1, sectionsJSON, *skipPages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] PDF analysis failed: %v\n", err)
		os.Exit(1)
	}

	err = analyzer.SaveResult(result, pagesJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to save analysis result: %v\n", err)
		os.Exit(1)
	}

	// ======================================================================
	// PASS 2: Regenerate HTML with page numbers, then render final PDF
	// ======================================================================
	fmt.Println("[PASS 2] Regenerating HTML (with page numbers)...")

	pass2Opts := baseOpts
	pass2Opts.OutputFile = htmlPass2
	pass2Opts.SectionsJSON = "" // Don't regenerate sections
	pass2Opts.PagesJSON = pagesJSON

	_, err = converter.ConvertToHTML(pass2Opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Pass 2 HTML generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[PASS 2] Converting to final PDF...")
	err = renderer.RenderToPDF(htmlPass2, *outputFile, renderer.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Final PDF generation failed: %v\n", err)
		os.Exit(1)
	}

	// ======================================================================
	// SUCCESS
	// ======================================================================
	fmt.Println()
	fmt.Println("==============================================================================")
	fmt.Printf("[SUCCESS] PDF generated: %s\n", *outputFile)
	fmt.Println("==============================================================================")
}
