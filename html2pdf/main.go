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
	flag.Parse()

	if *showVersion {
		fmt.Printf("html2pdf v%s (%s)\n", BuildVersion, BuildTime)
		return
	}

	if *input == "" {
		fmt.Println("Usage: html2pdf -i input.html [-o output.pdf] [--landscape]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -i string      Input HTML file path (required)")
		fmt.Println("  -o string      Output PDF file path (default: input.pdf)")
		fmt.Println("  --landscape    Use landscape orientation")
		fmt.Println("  -v             Show version")
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
