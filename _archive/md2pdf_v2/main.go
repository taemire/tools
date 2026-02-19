package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/signintech/gopdf"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type TOCEntry struct {
	Level   int
	Title   string
	PageNum int
}

type PDFRenderer struct {
	pdf        *gopdf.GoPdf
	toc        []TOCEntry
	pageCount  int
	fontPath   string
	fontSize   float64
	lineHeight float64
	// Layout Constants
	marginL  float64
	marginT  float64 // Top margin for content
	marginB  float64 // Bottom margin for content
	contentW float64

	// State
	currentTitle string // For Header
	isDryRun     bool   // Flag for Pass 1
}

// Colors
const mm2pt = 2.83464567

func (r *PDFRenderer) setPrimaryColor() { r.pdf.SetTextColor(9, 9, 11) }        // #09090b
func (r *PDFRenderer) setMutedColor()   { r.pdf.SetTextColor(113, 113, 122) }   // #71717a
func (r *PDFRenderer) setAccentColor()  { r.pdf.SetTextColor(37, 99, 235) }     // #2563eb
func (r *PDFRenderer) setBorderColor()  { r.pdf.SetStrokeColor(228, 228, 231) } // #e4e4e7
func (r *PDFRenderer) setWhiteColor()   { r.pdf.SetTextColor(255, 255, 255) }

// headingPageMap stores the page number for each heading title from Pass 1
var headingPageMap = make(map[string]int)

func main() {
	input := flag.String("i", "", "Input markdown file")
	output := flag.String("o", "output.pdf", "Output PDF file")
	flag.Parse()

	if *input == "" {
		fmt.Println("Usage: md2pdf_v2 -i <input.md> [-o <output.pdf>]")
		return
	}

	data, err := ioutil.ReadFile(*input)
	if err != nil {
		log.Fatal(err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	reader := text.NewReader(data)
	doc := md.Parser().Parse(reader)

	marginH := 20.0 * mm2pt
	marginV := 25.0 * mm2pt

	r := &PDFRenderer{
		marginL:    marginH,
		marginT:    marginV,
		marginB:    marginV,
		contentW:   595.28 - (marginH * 2),
		fontSize:   11,
		lineHeight: 18,
	}

	// --- Pass 1: Dry Run (Calculate Page Numbers) ---
	fmt.Println("Starting Pass 1: Calculating page numbers...")

	// Initialize font path
	r.fontPath = `C:\Windows\Fonts\malgun.ttf`

	dryRunPdf := &gopdf.GoPdf{}
	dryRunPdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	r.pdf = dryRunPdf
	// Font setup for Dry Run
	dryRunPdf.AddTTFFont("malgun", r.fontPath)
	dryRunPdf.SetFont("malgun", "", 11)

	// Simulate Cover
	dryRunPdf.AddPage()

	// Simulate TOC pages
	// We estimate TOC pages initially, then it will be corrected in Pass 2 if needed
	// For Pass 1, we assume a fixed number of TOC pages (e.g., 2) to start content offset
	// However, to get EXACT content pages, we should just render content and see where headings land relative to start.
	// But simply, let's just render the body starting from a hypothetical page.
	// Let's assume TOC takes 2 pages for now.
	tocEstPages := 2
	r.pageCount = 1 + tocEstPages // Cover(1) + TOC(2)

	// Start Content for Dry Run
	dryRunPdf.AddPage()
	r.renderHeader("INTRODUCTION")
	r.renderFooter()
	dryRunPdf.SetY(r.marginT)

	// Render Body to capture page numbers
	r.isDryRun = true
	r.renderNode(doc, data)
	r.isDryRun = false

	fmt.Printf("Pass 1 Complete. Found %d headings.\n", len(headingPageMap))

	// --- Pass 2: Actual Render ---
	fmt.Println("Starting Pass 2: Final Rendering...")

	// Create final PDF
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddTTFFont("malgun", r.fontPath)
	pdf.SetFont("malgun", "", 11)
	r.pdf = pdf
	r.toc = nil // Reset TOC
	r.pageCount = 0

	pdf.AddPage() // Page 1: Cover
	r.renderCover()

	// Build TOC from captured map
	// We need to walk AST again or store the order?
	// Storing order is better. But let's just populate TOC in Pass 2 using the map.
	// Actually, we need TOC entries BEFORE rendering TOC.
	// So we should build r.toc list during Pass 1 as well.

	// Let's refine: In Pass 1, we populate r.toc with Titles and PageNums.
	// But we need to recreate r.toc in Pass 2?? No, just keep it.
	// The problem is r.toc is filled in renderNode via AST walk.
	// So we just clear r.toc before Pass 1?
	// r.toc is appended in renderNode?? No, strictly in current code it's not.
	// We need to modify renderNode to append to TOC.

	// Wait, previous code simulated TOC via ast.Walk separate from renderNode.
	// Now we will use renderNode for both.

	// Re-verify TOC logic:
	// We need to calculate how many pages TOC takes based on the items found in Pass 1.
	_ = r.calculateTOCPages() // Just for initial estimate, will be recalculated after Pass 1 re-run

	// Pass 1 will be re-run below with correct page starting point.
	// Actual shift is calculated after the re-run.

	// RETRYING LOGIC FOR SIMPLICITY:
	// Pass 1: Render ONLY CONTENT starting at Page 1. Capture Page Nums.
	// Pass 2: Render Cover (1 page) + TOC (N pages).
	//         Shift captured Page Nums by (1 + N).
	//         Render Content.

	// --- RE-DOING PASS 1 ---
	r.pdf = dryRunPdf
	r.toc = []TOCEntry{}
	r.pageCount = 1
	dryRunPdf.AddPage()
	dryRunPdf.SetY(r.marginT)

	r.isDryRun = true
	r.renderNode(doc, data)
	r.isDryRun = false

	// Calculate Shift
	realTOCPages := r.calculateTOCPages()
	totalShift := 1 + realTOCPages

	// Updates TOC entries with shift
	for i := range r.toc {
		r.toc[i].PageNum += totalShift
		// Decrement by 1 because we started at Page 1, but technically content starts after TOC?
		// No, if Pass 1 said "Introduction is on Page 1", and we have Cover(1)+TOC(1), then Intro is on Page 3.
		// So Page 1 -> Page 3. Shift is +2. Correct.
	}

	// --- Pass 2: Final Rendering (after shift applied) ---
	// Reset PDF
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddTTFFont("malgun", r.fontPath)
	pdf.SetFont("malgun", "", 11)
	r.pdf = pdf
	r.pageCount = 0

	pdf.AddPage() // Page 1: Cover
	r.renderCover()

	// TOC Pages
	// Note: calculateTOCPages only calculates count. We need to actually render.
	// Since we shifted the numbers in r.toc, renderTOC will show correct numbers.

	// Force page count sync
	r.pageCount = 1 // Cover done

	pdf.AddPage()
	r.pageCount++
	r.renderHeader("목차")
	r.renderFooter()
	r.renderTOC() // This handles multi-page TOC and increments r.pageCount

	// Content
	pdf.AddPage()
	r.pageCount++
	r.renderHeader("INTRODUCTION")
	r.renderFooter()
	pdf.SetY(r.marginT)

	r.renderNode(doc, data)

	pdf.WritePdf(*output)
	fmt.Printf("Successfully generated: %s\n", *output)
}

// simulateTOC removed in favor of 2-Pass renderNode

func (r *PDFRenderer) renderCover() {
	// Cover layout based on sample.html
	// Padding 25mm 20mm

	// 1. Tag
	r.pdf.SetY(r.marginT)
	r.pdf.SetX(r.marginL)
	r.pdf.SetFillColor(9, 9, 11) // Primary
	r.pdf.RectFromUpperLeftWithStyle(r.marginL, r.marginT, 120, 24, "F")

	r.setWhiteColor()
	r.pdf.SetFont("malgun", "", 10)
	r.pdf.SetX(r.marginL + 10)
	r.pdf.SetY(r.marginT + 6)
	r.pdf.Cell(nil, "CODESIGN SERVICE MANUAL")

	// 2. Big Title
	r.setPrimaryColor()
	r.pdf.SetFont("malgun", "", 42)
	r.pdf.SetY(r.marginT + 60)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "CodeSign Service")
	r.pdf.Br(50)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "User Manual")

	// 3. Subtitle
	r.setMutedColor()
	r.pdf.SetFont("malgun", "", 18)
	r.pdf.Br(30)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "Operational Guide & Reference")

	// 4. Info Block (Bottom)
	bottomY := 841.89 - r.marginB - 100
	r.pdf.SetY(bottomY)
	r.pdf.SetFont("malgun", "", 10)
	r.setPrimaryColor()

	// Draw info rows
	infos := []string{
		"Author: Technical Writing Team",
		"Date: 2025-12-21",
		"Version: v1.0.2",
	}

	for _, info := range infos {
		r.pdf.SetX(r.marginL)
		r.pdf.Cell(nil, info)
		r.pdf.Br(14)
	}

	// Copyright
	r.setBorderColor()
	r.pdf.Line(r.marginL, bottomY+50, 595.28-r.marginL, bottomY+50)

	r.setMutedColor()
	r.pdf.SetFont("malgun", "", 9)
	r.pdf.SetY(bottomY + 65)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "© 2025 Signin Technical. All rights reserved.")
}

func (r *PDFRenderer) renderHeader(rightText string) {
	r.currentTitle = rightText

	y := 10.0 * mm2pt
	r.pdf.SetY(y)

	// Left: Static Title
	r.setMutedColor()
	r.pdf.SetFont("malgun", "", 9)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "CODESIGN SERVICE 2025")

	// Right: Dynamic Section
	if rightText != "" {
		w, _ := r.pdf.MeasureTextWidth(rightText)
		r.pdf.SetX(595.28 - r.marginL - w)
		r.pdf.Cell(nil, rightText)
	}

	// Line
	r.setBorderColor()
	lineY := y + 12
	r.pdf.Line(r.marginL, lineY, 595.28-r.marginL, lineY)
}

func (r *PDFRenderer) renderFooter() {
	y := 841.89 - (10.0 * mm2pt)

	// Line
	r.setBorderColor()
	lineY := y - 12
	r.pdf.Line(r.marginL, lineY, 595.28-r.marginL, lineY)

	// Left: Copyright
	r.setMutedColor()
	r.pdf.SetFont("malgun", "", 9)
	r.pdf.SetY(y - 5)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "© 2025 Signin Technical Group")

	// Right: Page Numer
	pageStr := fmt.Sprintf("Page %02d", r.pageCount)
	w, _ := r.pdf.MeasureTextWidth(pageStr)
	r.pdf.SetX(595.28 - r.marginL - w)
	r.pdf.Cell(nil, pageStr)
}

func (r *PDFRenderer) calculateTOCPages() int {
	// Calculate height needed for TOC
	// Title (40) + Gap (40) + Items
	currentY := r.marginT + 40 + 40
	pages := 1

	for range r.toc {
		itemH := r.lineHeight * 1.5
		if currentY+itemH > 841.89-r.marginB {
			pages++
			currentY = r.marginT
		}
		currentY += itemH
	}
	return pages
}

func (r *PDFRenderer) renderTOC() {
	// Header is already drawn for the first page
	r.pdf.SetY(r.marginT)

	// Title
	r.setPrimaryColor()
	r.pdf.SetFont("malgun", "", 24)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "목차")
	r.pdf.Br(40)

	r.pdf.SetFont("malgun", "", 11)

	for _, entry := range r.toc {
		// Check Page Break
		if r.pdf.GetY()+r.lineHeight*1.5 > 841.89-r.marginB {
			r.pdf.AddPage()
			r.pageCount++ // Increment visible page number
			r.renderHeader("TABLE OF CONTENTS")
			r.renderFooter()
			r.pdf.SetY(r.marginT)
		}

		indent := float64(entry.Level-1) * 20
		r.pdf.SetX(r.marginL + indent)

		r.setPrimaryColor()
		title := entry.Title

		r.setAccentColor()
		pageNum := fmt.Sprintf("%02d", entry.PageNum)

		r.pdf.SetFont("malgun", "", 11)
		tw, _ := r.pdf.MeasureTextWidth(title)
		pw, _ := r.pdf.MeasureTextWidth(pageNum)

		// Title
		r.setPrimaryColor()
		r.pdf.Cell(nil, title)

		// Dots
		r.setBorderColor()
		dotStart := r.marginL + indent + tw + 5
		dotEnd := r.marginL + r.contentW - pw - 5
		if dotEnd > dotStart {
			dots := ""
			dotW, _ := r.pdf.MeasureTextWidth(".")
			count := int((dotEnd - dotStart) / dotW)
			if count > 0 {
				dots = strings.Repeat(".", count)
				r.pdf.SetX(dotStart)
				r.setMutedColor()
				r.pdf.Cell(nil, dots)
			}
		}

		// PageNum
		r.setAccentColor()
		r.pdf.SetX(r.marginL + r.contentW - pw)
		r.pdf.Cell(nil, pageNum)

		r.pdf.Br(r.lineHeight * 1.5)
	}
}

func (r *PDFRenderer) renderNode(n ast.Node, source []byte) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Heading:
			r.checkPageBreak(40, string(node.Text(source)))
			r.setPrimaryColor()
			r.pdf.SetFont("malgun", "", float64(24-node.Level*2))
			r.pdf.SetX(r.marginL)
			r.pdf.Cell(nil, string(node.Text(source)))
			r.pdf.Br(30)

			// Update current section title
			r.currentTitle = string(node.Text(source))

			// Record for TOC (Only if it's Pass 1)
			if r.isDryRun {
				r.toc = append(r.toc, TOCEntry{
					Level:   node.Level,
					Title:   string(node.Text(source)),
					PageNum: r.pageCount,
				})
			}
		case *ast.Paragraph:
			r.checkPageBreak(60, "")
			r.setPrimaryColor()
			r.pdf.SetFont("malgun", "", 11)
			r.pdf.SetX(r.marginL)
			text := string(node.Text(source))
			r.renderWrappedText(text)
			r.pdf.Br(r.lineHeight * 2)
		}
	}
}

func (r *PDFRenderer) renderWrappedText(text string) {
	words := strings.Fields(text)
	var line string
	for _, word := range words {
		testLine := line
		if line != "" {
			testLine += " "
		}
		testLine += word

		w, _ := r.pdf.MeasureTextWidth(testLine)
		if w > r.contentW {
			r.pdf.Cell(nil, line)
			r.pdf.Br(r.lineHeight)
			r.pdf.SetX(r.marginL)
			line = word
		} else {
			line = testLine
		}
	}
	r.pdf.Cell(nil, line)
}

func (r *PDFRenderer) checkPageBreak(h float64, newTitle string) {
	if r.pdf.GetY()+h > 841.89-r.marginB {
		r.pdf.AddPage()
		r.pageCount++

		// Use new title if provided (for Header)
		title := r.currentTitle
		if newTitle != "" {
			title = newTitle
		}

		r.renderHeader(title)
		r.renderFooter()
		r.pdf.SetY(r.marginT)
	}
}
