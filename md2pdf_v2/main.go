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
}

// Colors
func (r *PDFRenderer) setPrimaryColor() { r.pdf.SetTextColor(9, 9, 11) }        // #09090b
func (r *PDFRenderer) setMutedColor()   { r.pdf.SetTextColor(113, 113, 122) }   // #71717a
func (r *PDFRenderer) setAccentColor()  { r.pdf.SetTextColor(37, 99, 235) }     // #2563eb
func (r *PDFRenderer) setBorderColor()  { r.pdf.SetStrokeColor(228, 228, 231) } // #e4e4e7
func (r *PDFRenderer) setWhiteColor()   { r.pdf.SetTextColor(255, 255, 255) }

const mm2pt = 2.83464567

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

	// --- Pass 1: Dry Run (Approximate TOC Page Numbers) ---
	r.simulateTOC(doc, data)

	// --- Pass 2: Actual Render ---
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	r.pdf = pdf

	r.fontPath = `C:\Windows\Fonts\malgun.ttf`
	pdf.AddTTFFont("malgun", r.fontPath)
	pdf.SetFont("malgun", "", 11)

	pdf.AddPage() // Page 1: Cover
	r.renderCover()

	// Check if TOC needs multiple pages
	r.pageCount = 2
	tocPages := r.calculateTOCPages()
	if tocPages > 1 {
		// Shift all content page numbers
		shift := tocPages - 1
		for i := range r.toc {
			r.toc[i].PageNum += shift
		}
	}

	pdf.AddPage() // Page 2: Table of Contents
	r.renderHeader("TABLE OF CONTENTS")
	r.renderFooter()

	r.renderTOC()

	// Set correct page count for content start
	r.pageCount = 2 + tocPages
	pdf.AddPage()                  // Start Content
	r.renderHeader("INTRODUCTION") // Default start
	r.renderFooter()

	pdf.SetY(r.marginT)
	r.renderNode(doc, data)

	pdf.WritePdf(*output)
	fmt.Printf("Successfully generated: %s\n", *output)
}

func (r *PDFRenderer) simulateTOC(n ast.Node, source []byte) {
	r.pageCount = 3
	currentY := r.marginT

	ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if h, ok := node.(*ast.Heading); ok {
			title := string(h.Text(source))
			r.toc = append(r.toc, TOCEntry{
				Level:   h.Level,
				Title:   title,
				PageNum: r.pageCount,
			})
			height := 40.0
			if currentY+height > 841.89-r.marginB {
				r.pageCount++
				currentY = r.marginT
			}
			currentY += height
		} else if _, ok := node.(*ast.Paragraph); ok {
			height := 60.0 // Simplified fixed height per paragraph for simulation
			if currentY+height > 841.89-r.marginB {
				r.pageCount++
				currentY = r.marginT
			}
			currentY += height
		}
		return ast.WalkContinue, nil
	})
}

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
