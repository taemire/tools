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
	marginL    float64
	marginT    float64
	contentW   float64
}

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

	margin := 15.0 * 2.83464567
	r := &PDFRenderer{
		marginL:    margin,
		marginT:    margin,
		contentW:   595.28 - (margin * 2),
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

	pdf.AddPage() // Page 2: Table of Contents
	r.renderTOC()

	r.pageCount = 3
	pdf.AddPage() // Page 3: Start Content
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
			if currentY+height > 841.89-r.marginT {
				r.pageCount++
				currentY = r.marginT
			}
			currentY += height
		} else if _, ok := node.(*ast.Paragraph); ok {
			height := 60.0 // Simplified fixed height per paragraph for simulation
			if currentY+height > 841.89-r.marginT {
				r.pageCount++
				currentY = r.marginT
			}
			currentY += height
		}
		return ast.WalkContinue, nil
	})
}

func (r *PDFRenderer) renderCover() {
	r.pdf.SetY(150)
	r.pdf.SetFont("malgun", "", 32)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "User Manual")
}

func (r *PDFRenderer) renderTOC() {
	r.pdf.SetY(r.marginT)
	r.pdf.SetFont("malgun", "", 18)
	r.pdf.SetX(r.marginL)
	r.pdf.Cell(nil, "목차")
	r.pdf.Br(40)

	r.pdf.SetFont("malgun", "", 11)
	for _, entry := range r.toc {
		indent := float64(entry.Level-1) * 20
		r.pdf.SetX(r.marginL + indent)

		title := entry.Title
		pageNum := fmt.Sprintf("%d", entry.PageNum)

		tw, _ := r.pdf.MeasureTextWidth(title)
		pw, _ := r.pdf.MeasureTextWidth(pageNum)

		r.pdf.Cell(nil, title)

		// Dot Leader
		dotStart := r.marginL + indent + tw + 5
		dotEnd := r.marginL + r.contentW - pw - 5
		if dotEnd > dotStart {
			dots := strings.Repeat(".", int((dotEnd-dotStart)/2))
			r.pdf.SetX(dotStart)
			r.pdf.Cell(nil, dots)
		}

		r.pdf.SetX(r.marginL + r.contentW - pw)
		r.pdf.Cell(nil, pageNum)
		r.pdf.Br(r.lineHeight)
	}
}

func (r *PDFRenderer) renderNode(n ast.Node, source []byte) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Heading:
			r.checkPageBreak(40)
			r.pdf.SetFont("malgun", "", float64(24-node.Level*2))
			r.pdf.SetX(r.marginL)
			r.pdf.Cell(nil, string(node.Text(source)))
			r.pdf.Br(30)
		case *ast.Paragraph:
			r.checkPageBreak(60)
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

func (r *PDFRenderer) checkPageBreak(h float64) {
	if r.pdf.GetY()+h > 841.89-r.marginT {
		r.pdf.AddPage()
		r.pageCount++
		r.pdf.SetY(r.marginT)
	}
}
