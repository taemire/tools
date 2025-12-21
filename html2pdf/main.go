package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var (
	BuildVersion = "1.0.0"
	BuildTime    = ""
)

func main() {
	// Flags
	input := flag.String("i", "", "Input HTML file path (required)")
	output := flag.String("o", "", "Output PDF file path (default: same as input with .pdf)")
	landscape := flag.Bool("landscape", false, "Use landscape orientation")
	showVersion := flag.Bool("v", false, "Show version")

	// 헤더/푸터 관련 옵션
	displayHeaderFooter := flag.Bool("hf", false, "Display header and footer")
	headerTemplate := flag.String("header", "", "Custom header HTML template")
	footerTemplate := flag.String("footer", "", "Custom footer HTML template")

	flag.Parse()

	if *showVersion {
		fmt.Printf("html2pdf v%s (%s)\n", BuildVersion, BuildTime)
		return
	}

	if *input == "" {
		fmt.Println("Usage: html2pdf -i input.html [-o output.pdf] [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -i string      Input HTML file path (required)")
		fmt.Println("  -o string      Output PDF file path (default: input.pdf)")
		fmt.Println("  --landscape    Use landscape orientation")
		fmt.Println("  -hf            Display header and footer with page numbers")
		fmt.Println("  -header string Custom header HTML template")
		fmt.Println("  -footer string Custom footer HTML template")
		fmt.Println("  -v             Show version")
		fmt.Println()
		fmt.Println("Template placeholders (use CSS classes in span):")
		fmt.Println("  <span class=\"date\"></span>       - Current date")
		fmt.Println("  <span class=\"title\"></span>      - Document title")
		fmt.Println("  <span class=\"url\"></span>        - Document URL")
		fmt.Println("  <span class=\"pageNumber\"></span> - Current page number")
		fmt.Println("  <span class=\"totalPages\"></span> - Total pages")
		os.Exit(1)
	}

	// Default output path
	if *output == "" {
		ext := filepath.Ext(*input)
		*output = strings.TrimSuffix(*input, ext) + ".pdf"
	}

	// Convert to absolute path
	absInput, err := filepath.Abs(*input)
	if err != nil {
		log.Fatalf("Failed to resolve input path: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(absInput); os.IsNotExist(err) {
		log.Fatalf("Input file not found: %s", absInput)
	}

	// Convert to file URL
	fileURL := "file:///" + strings.ReplaceAll(absInput, "\\", "/")

	fmt.Printf("[INFO] Converting: %s\n", *input)
	fmt.Printf("[INFO] Output: %s\n", *output)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create Chrome context (Quiet mode to ignore CDP unmarshal noise)
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithLogf(func(string, ...interface{}) {}),
		chromedp.WithErrorf(func(string, ...interface{}) {}),
	)
	defer cancel()

	// PDF options
	printParams := page.PrintToPDF().
		WithPrintBackground(true).
		WithPreferCSSPageSize(true)

	if *landscape {
		printParams = printParams.WithLandscape(true)
	}

	// 헤더/푸터 설정
	if *displayHeaderFooter {
		printParams = printParams.WithDisplayHeaderFooter(true)

		// 기본 헤더 템플릿 (비어있으면 공백)
		defaultHeader := `<div style="font-size: 9px; color: #666; width: 100%; text-align: center; padding: 5px 0;"></div>`
		if *headerTemplate != "" {
			defaultHeader = *headerTemplate
		}

		// 기본 푸터 템플릿 (페이지 번호 중앙 정렬)
		defaultFooter := `<div style="font-size: 9px; color: #666; width: 100%; text-align: center; padding: 5px 0;">
			<span class="pageNumber"></span> / <span class="totalPages"></span>
		</div>`
		if *footerTemplate != "" {
			defaultFooter = *footerTemplate
		}

		printParams = printParams.
			WithHeaderTemplate(defaultHeader).
			WithFooterTemplate(defaultFooter).
			WithMarginTop(0.6).   // 헤더 공간 확보 (인치)
			WithMarginBottom(0.6) // 푸터 공간 확보 (인치)

		fmt.Println("[INFO] Header/Footer enabled")
	}

	var pdfBuf []byte

	// Run Chrome actions
	err = chromedp.Run(ctx,
		chromedp.Navigate(fileURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(1*time.Second), // Wait for fonts/images to load
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = printParams.Do(ctx)
			return err
		}),
	)
	if err != nil {
		log.Fatalf("Failed to generate PDF: %v", err)
	}

	// Write PDF
	if err := os.WriteFile(*output, pdfBuf, 0644); err != nil {
		log.Fatalf("Failed to write PDF: %v", err)
	}

	fmt.Printf("[SUCCESS] PDF generated: %s\n", *output)
}
