// Package renderer provides PDF rendering functionality using gopdf
package renderer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"md2pdf/parser"

	"github.com/signintech/gopdf"
)

// PDFRenderer handles PDF generation
type PDFRenderer struct {
	pdf         *gopdf.GoPdf
	theme       *parser.Theme
	config      *DocumentConfig
	currentPage int
	contentPage int // page number for content (excluding cover/toc)
	pageHeight  float64
	pageWidth   float64
	contentY    float64
	anchors     map[string]AnchorInfo
	tocEntries  []TOCRenderEntry
}

// DocumentConfig holds document metadata
type DocumentConfig struct {
	Title        string
	Subtitle     string
	Version      string
	Author       string
	Date         string
	Copyright    string
	Organization string
	Header       string
	Footer       string
}

// AnchorInfo stores anchor position information
type AnchorInfo struct {
	Page int
	Y    float64
}

// TOCRenderEntry stores TOC entry with rendered page info
type TOCRenderEntry struct {
	Level    int
	Title    string
	AnchorID string
	Page     int
}

// NewPDFRenderer creates a new PDF renderer
func NewPDFRenderer(theme *parser.Theme, config *DocumentConfig) *PDFRenderer {
	pdf := &gopdf.GoPdf{}

	// Set page size based on theme
	var pageSize *gopdf.Rect
	switch theme.Page.Size {
	case "A4":
		pageSize = gopdf.PageSizeA4
	case "Letter":
		pageSize = gopdf.PageSizeLetter
	default:
		pageSize = gopdf.PageSizeA4
	}

	pdf.Start(gopdf.Config{PageSize: *pageSize})

	return &PDFRenderer{
		pdf:        pdf,
		theme:      theme,
		config:     config,
		pageWidth:  pageSize.W,
		pageHeight: pageSize.H,
		anchors:    make(map[string]AnchorInfo),
		tocEntries: make([]TOCRenderEntry, 0),
	}
}

// LoadFonts loads fonts specified in the theme
func (r *PDFRenderer) LoadFonts(basePath string) error {
	// Try multiple paths for fonts
	fontPaths := []string{
		basePath + "/" + r.theme.Fonts.Primary.Regular,
		r.theme.Fonts.Primary.Regular,
		"fonts/malgun.ttf",
		"C:/Windows/Fonts/malgun.ttf",
	}

	var primaryLoaded bool
	for _, path := range fontPaths {
		if err := r.pdf.AddTTFFont("primary", path); err == nil {
			primaryLoaded = true
			break
		}
	}

	if !primaryLoaded {
		return fmt.Errorf("failed to load primary font from any path")
	}

	// Load bold font
	boldPaths := []string{
		basePath + "/" + r.theme.Fonts.Primary.Bold,
		r.theme.Fonts.Primary.Bold,
		"fonts/malgunbd.ttf",
		"C:/Windows/Fonts/malgunbd.ttf",
	}

	var boldLoaded bool
	for _, path := range boldPaths {
		if err := r.pdf.AddTTFFont("primary-bold", path); err == nil {
			boldLoaded = true
			break
		}
	}

	// If bold font not loaded, use primary as fallback
	if !boldLoaded {
		for _, path := range fontPaths {
			if err := r.pdf.AddTTFFont("primary-bold", path); err == nil {
				break
			}
		}
	}

	// Load code font (use primary as fallback)
	codePaths := []string{
		basePath + "/" + r.theme.Fonts.Code.Regular,
		r.theme.Fonts.Code.Regular,
		"fonts/malgun.ttf",
		"C:/Windows/Fonts/malgun.ttf",
	}

	for _, path := range codePaths {
		if err := r.pdf.AddTTFFont("code", path); err == nil {
			break
		}
	}

	return nil
}

// RenderDocument renders the complete document
func (r *PDFRenderer) RenderDocument(doc *parser.MarkdownDocument) error {
	// First pass: calculate page numbers for TOC
	// For now, we'll do a simplified version where we render everything
	// and track page numbers

	// Render cover page
	if r.theme.Cover.Enabled {
		r.renderCoverPage()
	}

	// Store section page info for TOC
	for _, entry := range doc.TOC {
		r.tocEntries = append(r.tocEntries, TOCRenderEntry{
			Level:    entry.Level,
			Title:    entry.Title,
			AnchorID: entry.AnchorID,
			Page:     0, // will be updated during content rendering
		})
	}

	// Render table of contents
	if r.theme.TOC.Enabled {
		r.renderTOCPage()
	}

	// Render content sections
	r.contentPage = 0
	for i, section := range doc.Sections {
		r.renderSection(section, i)
	}

	return nil
}

// renderCoverPage renders the cover page
func (r *PDFRenderer) renderCoverPage() {
	r.pdf.AddPage()
	r.currentPage++

	cover := r.theme.Cover

	// Top border
	if cover.TopBorder.Enabled {
		r.setFillColor(cover.TopBorder.Color)
		r.pdf.RectFromUpperLeftWithStyle(0, 0, r.pageWidth, cover.TopBorder.Height, "F")
	}

	// Logo
	r.setTextColor(cover.Logo.Color)
	r.pdf.SetFont("primary-bold", "", cover.Logo.FontSize)
	logoX := r.pageWidth - r.theme.Page.Margins.Right - 100
	r.pdf.SetXY(logoX, cover.Logo.Position.Y)
	r.pdf.Cell(nil, r.config.Organization)

	// Title
	r.setTextColor(cover.Title.Color)
	r.pdf.SetFont("primary-bold", "", cover.Title.FontSize)
	titleX := r.getPositionX(cover.Title.Position.X)
	r.pdf.SetXY(titleX, cover.Title.Position.Y)
	r.pdf.Cell(nil, r.config.Title)

	// Subtitle
	if cover.Subtitle.LeftBorder.Enabled {
		r.setFillColor(cover.Subtitle.LeftBorder.Color)
		r.pdf.RectFromUpperLeftWithStyle(
			r.getPositionX(cover.Subtitle.Position.X),
			cover.Subtitle.Position.Y,
			cover.Subtitle.LeftBorder.Width,
			20,
			"F",
		)
	}
	r.setTextColor(cover.Subtitle.Color)
	r.pdf.SetFont("primary", "", cover.Subtitle.FontSize)
	subtitleX := r.getPositionX(cover.Subtitle.Position.X) + 15
	r.pdf.SetXY(subtitleX, cover.Subtitle.Position.Y)
	r.pdf.Cell(nil, fmt.Sprintf("v%s 문서", r.config.Version))

	// Info table
	r.renderInfoTable()

	// Copyright
	r.setTextColor(cover.Copyright.Color)
	r.pdf.SetFont("primary", "", cover.Copyright.FontSize)
	r.pdf.SetXY(r.getPositionX(cover.Copyright.Position.X), cover.Copyright.Position.Y)
	copyrightText := strings.ReplaceAll(cover.Copyright.Template, "{{year}}", time.Now().Format("2006"))
	copyrightText = strings.ReplaceAll(copyrightText, "{{copyright}}", r.config.Copyright)
	r.pdf.Cell(nil, copyrightText)
}

// renderInfoTable renders the info table on cover
func (r *PDFRenderer) renderInfoTable() {
	table := r.theme.Cover.InfoTable
	x := r.getPositionX(table.Position.X)
	y := table.Position.Y

	data := map[string]string{
		"발행일": r.config.Date,
		"버전":  r.config.Version,
		"작성자": r.config.Author,
	}

	r.pdf.SetFont("primary", "", table.FontSize)

	for _, field := range table.Fields {
		// Label cell
		r.setFillColor(table.LabelBackground)
		r.pdf.RectFromUpperLeftWithStyle(x, y, table.LabelWidth, table.RowHeight, "F")
		r.setTextColor("text.default")
		r.pdf.SetXY(x+10, y+10)
		r.pdf.Cell(nil, field)

		// Value cell
		r.setStrokeColor(r.theme.Colors.Border)
		r.pdf.RectFromUpperLeftWithStyle(x+table.LabelWidth, y, table.ValueWidth, table.RowHeight, "D")
		r.pdf.SetXY(x+table.LabelWidth+10, y+10)
		if val, ok := data[field]; ok {
			r.pdf.Cell(nil, val)
		}

		y += table.RowHeight
	}
}

// renderTOCPage renders the table of contents page
func (r *PDFRenderer) renderTOCPage() {
	r.pdf.AddPage()
	r.currentPage++

	toc := r.theme.TOC

	// Title
	r.setTextColor(toc.Title.Color)
	r.pdf.SetFont("primary-bold", "", toc.Title.FontSize)
	r.pdf.SetXY(r.theme.Page.Margins.Left+20, 60)
	r.pdf.Cell(nil, toc.Title.Text)

	// Background box
	if toc.Background.Enabled {
		r.setFillColor(toc.Background.Color)
		r.pdf.RectFromUpperLeftWithStyle(
			r.theme.Page.Margins.Left+10,
			100,
			r.pageWidth-r.theme.Page.Margins.Left-r.theme.Page.Margins.Right-20,
			float64(len(r.tocEntries))*toc.Item.LineHeight+toc.Background.Padding*2,
			"F",
		)
	}

	// TOC entries
	y := 110 + toc.Background.Padding
	r.pdf.SetFont("primary", "", toc.Item.FontSize)

	for i, entry := range r.tocEntries {
		levelConfig := toc.Levels["h1"]
		if entry.Level == 2 {
			levelConfig = toc.Levels["h2"]
		} else if entry.Level >= 3 {
			levelConfig = toc.Levels["h3"]
		}

		x := r.theme.Page.Margins.Left + 30 + levelConfig.Indent

		// Set font based on level
		if levelConfig.Bold {
			r.pdf.SetFont("primary-bold", "", levelConfig.FontSize)
		} else {
			r.pdf.SetFont("primary", "", levelConfig.FontSize)
		}

		r.setTextColor(toc.Item.Color)
		r.pdf.SetXY(x, y)
		r.pdf.Cell(nil, entry.Title)

		// Dot leader
		if toc.Item.DotLeader.Enabled {
			titleWidth, _ := r.pdf.MeasureTextWidth(entry.Title)
			dotStartX := x + titleWidth + 5
			pageNumStr := strconv.Itoa(entry.Page + 3) // offset for cover and toc
			pageNumWidth, _ := r.pdf.MeasureTextWidth(pageNumStr)
			rightX := r.pageWidth - r.theme.Page.Margins.Right - 30
			dotEndX := rightX - pageNumWidth - 5

			r.setTextColor(toc.Item.DotLeader.Color)
			r.pdf.SetFont("primary", "", levelConfig.FontSize)
			for dotX := dotStartX; dotX < dotEndX; dotX += 6 {
				r.pdf.SetXY(dotX, y)
				r.pdf.Cell(nil, toc.Item.DotLeader.Char)
			}

			// Page number
			r.setTextColor(toc.Item.Color)
			r.pdf.SetXY(rightX-pageNumWidth, y)
			r.pdf.Cell(nil, pageNumStr)
		}

		// Update entry page for later reference
		r.tocEntries[i].Page = entry.Page

		y += toc.Item.LineHeight
	}
}

// renderSection renders a document section
func (r *PDFRenderer) renderSection(section parser.Section, sectionIndex int) {
	// Start new page for major sections (h1)
	if section.Level == 1 || r.currentPage == 0 {
		r.addContentPage()
		r.tocEntries[sectionIndex].Page = r.contentPage
	}

	// Register anchor
	r.anchors[section.AnchorID] = AnchorInfo{
		Page: r.currentPage,
		Y:    r.contentY,
	}

	// Render heading
	r.renderHeading(section.Level, section.Title)

	// Render content blocks
	for _, block := range section.Content {
		r.renderBlock(block)
	}
}

// addContentPage adds a new content page with header/footer
func (r *PDFRenderer) addContentPage() {
	r.pdf.AddPage()
	r.currentPage++
	r.contentPage++

	content := r.theme.Content

	// Header
	if content.Header.Enabled {
		r.setTextColor(content.Header.Color)
		r.pdf.SetFont("primary", "", content.Header.FontSize)
		headerY := r.theme.Page.Margins.Top
		r.pdf.SetXY(r.theme.Page.Margins.Left, headerY)

		headerText := strings.ReplaceAll(content.Header.Text, "{{header}}", r.config.Header)
		r.pdf.Cell(nil, headerText)

		// Header border
		if content.Header.Border.Bottom {
			r.setStrokeColor(content.Header.Border.Color)
			r.pdf.Line(
				r.theme.Page.Margins.Left,
				headerY+content.Header.Height-5,
				r.pageWidth-r.theme.Page.Margins.Right,
				headerY+content.Header.Height-5,
			)
		}
	}

	// Footer
	if content.Footer.Enabled {
		footerY := r.pageHeight - r.theme.Page.Margins.Bottom - content.Footer.Height

		// Footer border
		if content.Footer.Border.Top {
			r.setStrokeColor(content.Footer.Border.Color)
			r.pdf.Line(
				r.theme.Page.Margins.Left,
				footerY,
				r.pageWidth-r.theme.Page.Margins.Right,
				footerY,
			)
		}

		// Left text
		r.setTextColor(content.Footer.Left.Color)
		r.pdf.SetFont("primary", "", content.Footer.Left.FontSize)
		r.pdf.SetXY(r.theme.Page.Margins.Left, footerY+5)
		footerLeft := strings.ReplaceAll(content.Footer.Left.Text, "{{footer}}", r.config.Footer)
		r.pdf.Cell(nil, footerLeft)

		// Right text (page number)
		r.pdf.SetFont("primary", "", content.Footer.Right.FontSize)
		pageText := strings.ReplaceAll(content.Footer.Right.Text, "{{page}}", strconv.Itoa(r.contentPage))
		pageWidth, _ := r.pdf.MeasureTextWidth(pageText)
		r.pdf.SetXY(r.pageWidth-r.theme.Page.Margins.Right-pageWidth, footerY+5)
		r.pdf.Cell(nil, pageText)
	}

	// Set content start position
	r.contentY = r.theme.Page.Margins.Top + r.theme.Content.Header.Height + 10
}

// renderHeading renders a heading
func (r *PDFRenderer) renderHeading(level int, text string) {
	var style parser.HeadingStyleConfig
	switch level {
	case 1:
		style = r.theme.Content.Heading.H1
	case 2:
		style = r.theme.Content.Heading.H2
	case 3:
		style = r.theme.Content.Heading.H3
	default:
		style = r.theme.Content.Heading.H4
	}

	r.contentY += style.MarginTop

	// Left border for h2
	if style.LeftBorder {
		r.setFillColor(style.LeftBorderColor)
		r.pdf.RectFromUpperLeftWithStyle(
			r.theme.Page.Margins.Left,
			r.contentY,
			style.LeftBorderWidth,
			style.FontSize+5,
			"F",
		)
	}

	// Text
	r.setTextColor(style.Color)
	r.pdf.SetFont("primary-bold", "", style.FontSize)

	x := r.theme.Page.Margins.Left
	if style.LeftBorder {
		x += style.LeftBorderWidth + 10
	}

	r.pdf.SetXY(x, r.contentY)
	r.pdf.Cell(nil, text)

	r.contentY += style.FontSize

	// Underline for h1
	if style.Underline {
		r.setStrokeColor(style.UnderlineColor)
		r.pdf.Line(
			r.theme.Page.Margins.Left,
			r.contentY+5,
			r.pageWidth-r.theme.Page.Margins.Right,
			r.contentY+5,
		)
		r.contentY += 10
	}

	r.contentY += style.MarginBottom
}

// renderBlock renders a content block
func (r *PDFRenderer) renderBlock(block parser.Block) {
	switch block.Type {
	case parser.BlockParagraph:
		r.renderParagraph(block.Content)
	case parser.BlockCodeBlock:
		r.renderCodeBlock(block.Content, block.Lang)
	case parser.BlockList:
		r.renderList(block.Items, false)
	case parser.BlockNumberedList:
		r.renderList(block.Items, true)
	case parser.BlockBlockquote:
		r.renderBlockquote(block.Content)
	}
}

// renderParagraph renders a paragraph
func (r *PDFRenderer) renderParagraph(text string) {
	para := r.theme.Content.Paragraph

	r.setTextColor(para.Color)
	r.pdf.SetFont("primary", "", para.FontSize)

	// Simple text rendering (TODO: wrap text properly)
	r.pdf.SetXY(r.theme.Page.Margins.Left, r.contentY)
	r.pdf.Cell(nil, text)

	r.contentY += para.FontSize * para.LineHeight

	// Check page break
	r.checkPageBreak()
}

// renderCodeBlock renders a code block
func (r *PDFRenderer) renderCodeBlock(code string, lang string) {
	codeStyle := r.theme.Content.Code.Block

	lines := strings.Split(code, "\n")
	blockHeight := float64(len(lines))*(codeStyle.FontSize+2) + codeStyle.Padding*2

	// Check if we need a page break
	if r.contentY+blockHeight > r.pageHeight-r.theme.Page.Margins.Bottom-30 {
		r.addContentPage()
	}

	// Background
	r.setFillColor(codeStyle.BackgroundColor)
	r.pdf.RectFromUpperLeftWithStyle(
		r.theme.Page.Margins.Left,
		r.contentY,
		r.pageWidth-r.theme.Page.Margins.Left-r.theme.Page.Margins.Right,
		blockHeight,
		"F",
	)

	// Code text
	r.pdf.SetFont("code", "", codeStyle.FontSize)
	r.setTextColor(codeStyle.Color)

	y := r.contentY + codeStyle.Padding
	for i, line := range lines {
		x := r.theme.Page.Margins.Left + codeStyle.Padding

		// Line numbers
		if codeStyle.LineNumbers {
			r.pdf.SetXY(x, y)
			r.pdf.Cell(nil, fmt.Sprintf("%3d ", i+1))
			x += 30
		}

		r.pdf.SetXY(x, y)
		r.pdf.Cell(nil, line)
		y += codeStyle.FontSize + 2
	}

	r.contentY += blockHeight + 10
}

// renderList renders a list
func (r *PDFRenderer) renderList(items []string, numbered bool) {
	listStyle := r.theme.Content.List.Bullet
	if numbered {
		listStyle = r.theme.Content.List.Numbered
	}

	r.pdf.SetFont("primary", "", listStyle.FontSize)
	r.setTextColor(r.theme.Content.Paragraph.Color)

	for i, item := range items {
		x := r.theme.Page.Margins.Left + listStyle.Indent
		r.pdf.SetXY(x, r.contentY)

		if numbered {
			r.pdf.Cell(nil, fmt.Sprintf("%d. %s", i+1, item))
		} else {
			r.pdf.Cell(nil, "• "+item)
		}

		r.contentY += listStyle.FontSize + listStyle.Spacing
		r.checkPageBreak()
	}

	r.contentY += 5
}

// renderBlockquote renders a blockquote
func (r *PDFRenderer) renderBlockquote(text string) {
	bq := r.theme.Content.Blockquote

	height := 40.0 // simplified

	// Background
	r.setFillColor(bq.BackgroundColor)
	r.pdf.RectFromUpperLeftWithStyle(
		r.theme.Page.Margins.Left,
		r.contentY,
		r.pageWidth-r.theme.Page.Margins.Left-r.theme.Page.Margins.Right,
		height,
		"F",
	)

	// Left border
	r.setFillColor(bq.LeftBorder.Color)
	r.pdf.RectFromUpperLeftWithStyle(
		r.theme.Page.Margins.Left,
		r.contentY,
		bq.LeftBorder.Width,
		height,
		"F",
	)

	// Text
	r.setTextColor(r.theme.Content.Paragraph.Color)
	r.pdf.SetFont("primary", "", r.theme.Content.Paragraph.FontSize)
	r.pdf.SetXY(r.theme.Page.Margins.Left+bq.Padding.Horizontal, r.contentY+bq.Padding.Vertical)
	r.pdf.Cell(nil, text)

	r.contentY += height + 10
}

// checkPageBreak checks if a page break is needed
func (r *PDFRenderer) checkPageBreak() {
	if r.contentY > r.pageHeight-r.theme.Page.Margins.Bottom-50 {
		r.addContentPage()
	}
}

// Save saves the PDF to a file
func (r *PDFRenderer) Save(path string) error {
	return r.pdf.WritePdf(path)
}

// Helper functions

func (r *PDFRenderer) setTextColor(colorRef string) {
	rgb := r.resolveColor(colorRef)
	r.pdf.SetTextColor(rgb[0], rgb[1], rgb[2])
}

func (r *PDFRenderer) setFillColor(colorRef string) {
	rgb := r.resolveColor(colorRef)
	r.pdf.SetFillColor(rgb[0], rgb[1], rgb[2])
}

func (r *PDFRenderer) setStrokeColor(colorRef string) {
	rgb := r.resolveColor(colorRef)
	r.pdf.SetStrokeColor(rgb[0], rgb[1], rgb[2])
}

func (r *PDFRenderer) resolveColor(colorRef string) [3]uint8 {
	// Check if it's a theme reference
	switch colorRef {
	case "primary":
		return hexToRGB(r.theme.Colors.Primary)
	case "secondary":
		return hexToRGB(r.theme.Colors.Secondary)
	case "accent":
		return hexToRGB(r.theme.Colors.Accent)
	case "text.default":
		return hexToRGB(r.theme.Colors.Text.Default)
	case "text.muted":
		return hexToRGB(r.theme.Colors.Text.Muted)
	case "text.light":
		return hexToRGB(r.theme.Colors.Text.Light)
	case "background.default":
		return hexToRGB(r.theme.Colors.Background.Default)
	case "background.alt":
		return hexToRGB(r.theme.Colors.Background.Alt)
	case "background.code":
		return hexToRGB(r.theme.Colors.Background.Code)
	case "border":
		return hexToRGB(r.theme.Colors.Border)
	default:
		// Assume it's a hex color
		return hexToRGB(colorRef)
	}
}

func hexToRGB(hex string) [3]uint8 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return [3]uint8{0, 0, 0}
	}

	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)

	return [3]uint8{uint8(r), uint8(g), uint8(b)}
}

func (r *PDFRenderer) getPositionX(pos interface{}) float64 {
	switch v := pos.(type) {
	case float64:
		return v
	case string:
		if v == "right" {
			return r.pageWidth - r.theme.Page.Margins.Right - 100
		}
		return r.theme.Page.Margins.Left
	default:
		return r.theme.Page.Margins.Left
	}
}
