package artexmd

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"time"
)

const W = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
const R = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
const PR = "http://schemas.openxmlformats.org/package/2006/relationships"
const CT = "http://schemas.openxmlformats.org/package/2006/content-types"
const WordCT = "application/vnd.openxmlformats-officedocument.wordprocessingml."
const XHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
const Decl = ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"`

func xe(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }

// Styles are fixed and pinned by ARTEX independently of any response receipt.
var DOCXStyles = buildDOCXStyles()

func buildDOCXStyles() string {
	var b strings.Builder
	b.WriteString(XHead + `<w:styles xmlns:w="` + W + `"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="맑은 고딕" w:hAnsi="맑은 고딕" w:eastAsia="맑은 고딕"/><w:color w:val="2D3142"/><w:sz w:val="21"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:wordWrap w:val="0"/><w:spacing w:after="100" w:line="280" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>`)
	para := func(id, base, pr string) {
		b.WriteString(`<w:style w:type="paragraph" w:styleId="` + id + `"><w:name w:val="` + id + `"/><w:basedOn w:val="` + base + `"/><w:pPr>` + pr + `</w:pPr></w:style>`)
	}
	for _, level := range []int{2, 3} {
		n := strconv.Itoa(level)
		para("Heading"+n, "Normal", `<w:keepNext/><w:keepLines/><w:outlineLvl w:val="`+strconv.Itoa(level-1)+`"/>`)
		para("ArtexHeading"+n, "Heading"+n, `<w:keepNext/><w:keepLines/><w:outlineLvl w:val="`+strconv.Itoa(level-1)+`"/>`)
	}
	for _, id := range []string{"ArtexParagraph", "ArtexCell", "ArtexKPIValue", "ArtexKPILabel", "ArtexFigureEyebrow", "ArtexFigureTitle", "ArtexHeader"} {
		para(id, "Normal", "")
	}
	para("ArtexFigureNote", "Normal", `<w:keepNext/>`)
	for i := 0; i <= 2; i++ {
		for _, k := range []string{"U", "O"} {
			para(fmt.Sprintf("ArtexList%s%d", k, i), "Normal", "")
		}
	}
	for i := 1; i <= 50; i++ {
		para(fmt.Sprintf("ArtexFigurePage%d", i), "Normal", "")
	}
	for _, id := range []string{"ArtexData", "ArtexDetail", "ArtexKPI", "ArtexFigurePanel"} {
		b.WriteString(`<w:style w:type="table" w:styleId="` + id + `"><w:name w:val="` + id + `"/></w:style>`)
	}
	paths := []string{""}
	for _, x := range []string{"b", "i"} {
		paths = append(paths, x)
		for _, y := range []string{"b", "i"} {
			paths = append(paths, x+y)
		}
	}
	for _, path := range paths {
		for _, tail := range []string{"", "c", "H", "S"} {
			id := "ArtexI" + path + tail
			b.WriteString(`<w:style w:type="character" w:styleId="` + id + `"><w:name w:val="` + id + `"/></w:style>`)
		}
	}
	b.WriteString(`<w:style w:type="character" w:styleId="ArtexMarker"><w:name w:val="ArtexMarker"/></w:style></w:styles>`)
	return b.String()
}

type docxWriter struct {
	files      map[string][]byte
	rels       strings.Builder
	imageCount int
	assets     map[string]Asset
	notes      map[string]string
}

func rPr(path string, size int, bold bool, color, font string) string {
	if font == "" {
		font = "맑은 고딕"
	}
	weight := ""
	if bold || strings.Contains(path, "b") {
		weight += `<w:b/>`
	}
	if strings.Contains(path, "i") {
		weight += `<w:i/>`
	}
	return `<w:rPr><w:rStyle w:val="ArtexI` + path + `"/><w:rFonts w:ascii="` + font + `" w:hAnsi="` + font + `" w:eastAsia="맑은 고딕"/>` + weight + `<w:color w:val="` + color + `"/><w:sz w:val="` + strconv.Itoa(size) + `"/></w:rPr>`
}
func wordInline(xs []Inline, path string, size int, bold bool, color string) string {
	var b strings.Builder
	for _, x := range xs {
		switch x.Kind {
		case "strong":
			b.WriteString(wordInline(x.Children, path+"b", size, bold, color))
		case "emphasis":
			b.WriteString(wordInline(x.Children, path+"i", size, bold, color))
		case "text", "code":
			font := ""
			p := path
			if x.Kind == "code" {
				p += "c"
				font = "Consolas"
			}
			b.WriteString(`<w:r>` + rPr(p, size, bold, color, font) + `<w:t xml:space="preserve">` + xe(x.Text) + `</w:t></w:r>`)
		case "hard_break", "soft_break":
			suffix := "H"
			if x.Kind == "soft_break" {
				suffix = "S"
			}
			b.WriteString(`<w:r>` + rPr(path+suffix, size, bold, color, "") + `<w:br/></w:r>`)
		}
	}
	return b.String()
}
func wordP(xs []Inline, style string, size int, bold, keep bool, color string) string {
	k := ""
	if keep {
		k = `<w:keepNext/>`
	}
	return `<w:p><w:pPr><w:pStyle w:val="` + style + `"/>` + k + `<w:wordWrap w:val="0"/><w:spacing w:after="100" w:line="280" w:lineRule="auto"/></w:pPr>` + wordInline(xs, "", size, bold, color) + `</w:p>`
}
func plainInline(s string) []Inline {
	var out []Inline
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			out = append(out, Inline{Kind: "hard_break"})
		}
		if line != "" {
			out = append(out, Inline{Kind: "text", Text: line})
		}
	}
	return out
}
func tableOpen(style string, widths []int, fill string) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="` + style + `"/><w:tblW w:w="9071" w:type="dxa"/><w:tblBorders><w:top w:val="single" w:sz="4" w:color="CCD3DC"/><w:left w:val="single" w:sz="4" w:color="CCD3DC"/><w:bottom w:val="single" w:sz="4" w:color="CCD3DC"/><w:right w:val="single" w:sz="4" w:color="CCD3DC"/><w:insideH w:val="single" w:sz="4" w:color="CCD3DC"/><w:insideV w:val="single" w:sz="4" w:color="CCD3DC"/></w:tblBorders>`)
	if fill != "" {
		b.WriteString(`<w:shd w:fill="` + fill + `"/>`)
	}
	b.WriteString(`<w:tblLayout w:type="fixed"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="0" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`)
	for _, w := range widths {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, w)
	}
	b.WriteString(`</w:tblGrid>`)
	return b.String()
}
func widths(n int) []int {
	r := make([]int, n)
	for i := range r {
		r[i] = 9071 / n
	}
	if n == 3 {
		r = []int{5443, 1814, 1814}
	}
	return r
}
func wordTable(rows [][][]Inline, align []int, style string, overrides ...[]int) string {
	if len(rows) == 0 {
		return ""
	}
	ws := widths(len(rows[0]))
	if len(overrides) > 0 && len(overrides[0]) == len(ws) {
		ws = overrides[0]
	}
	if style == "ArtexDetail" && len(rows[0]) == 3 {
		ws = []int{680, 2041, 6350}
	}
	var b strings.Builder
	b.WriteString(tableOpen(style, ws, ""))
	for i, row := range rows {
		b.WriteString(`<w:tr><w:trPr>`)
		if !rowMaySplit(row) {
			b.WriteString(`<w:cantSplit/>`)
		}
		if i == 0 {
			b.WriteString(`<w:tblHeader/>`)
		}
		b.WriteString(`</w:trPr>`)
		for j, cell := range row {
			fill := "FFFFFF"
			if i == 0 {
				fill = "EDF1F5"
			}
			p := wordP(cell, "ArtexCell", 19, i == 0, false, "2D3142")
			if j < len(align) && align[j] != 4 {
				a := map[int]string{1: "left", 2: "right", 3: "center"}[align[j]]
				p = strings.Replace(p, `</w:pPr>`, `<w:jc w:val="`+a+`"/></w:pPr>`, 1)
			}
			fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:fill="%s"/><w:tcMar><w:top w:w="90" w:type="dxa"/><w:left w:w="90" w:type="dxa"/><w:bottom w:w="90" w:type="dxa"/><w:right w:w="90" w:type="dxa"/></w:tcMar></w:tcPr>%s</w:tc>`, ws[j], fill, p)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}
func (w *docxWriter) blocks(bs []Block, slot string, depth int) (string, error) {
	var out strings.Builder
	for idx, b := range bs {
		switch b.Kind {
		case "paragraph", "heading":
			style := "ArtexParagraph"
			size := 21
			bold, keep := false, false
			color := "2D3142"
			if b.Kind == "heading" {
				style = "ArtexHeading" + strconv.Itoa(b.Level)
				size = 28
				bold = true
				keep = true
				color = "2D3142"
				if slot == "meta.cover" && idx == 0 {
					size = 44
				}
				if strings.HasPrefix(slot, "figure_title.") {
					size = 24
				}
			}
			if strings.HasPrefix(slot, "figure_title.") && b.Kind == "paragraph" {
				size = 18
				color = "4F5D75"
				keep = true
			}
			if strings.HasPrefix(slot, "prose.") {
				size = 19
			}
			if strings.HasPrefix(slot, "prose.figure_note.") && b.Kind == "paragraph" {
				style = "ArtexFigureNote"
				keep = true
			}
			p := wordP(b.Inlines, style, size, bold, keep, color)
			if (strings.HasPrefix(slot, "figure_title.") || slot == "appendix.customer_facts") && idx == 0 {
				p = strings.Replace(p, `<w:wordWrap`, `<w:pageBreakBefore/><w:wordWrap`, 1)
			}
			if slot == "review.banner" {
				p = strings.Replace(p, `<w:wordWrap`, `<w:pBdr><w:left w:val="single" w:sz="16" w:color="B42318"/></w:pBdr><w:shd w:fill="FFF1F0"/><w:wordWrap`, 1)
			}
			if strings.HasPrefix(slot, "prose.") && !strings.HasPrefix(slot, "prose.figure_note.") {
				p = strings.Replace(p, `<w:wordWrap`, `<w:shd w:fill="F2F5F9"/><w:wordWrap`, 1)
			}
			out.WriteString(p)
		case "list":
			for i, item := range b.Items {
				marker := "•"
				ordered := "U"
				if b.Ordered {
					marker = strconv.Itoa(b.Start+i) + "."
					ordered = "O"
				}
				first := true
				for _, part := range item {
					if part.Kind == "list" {
						v, e := w.blocks([]Block{part}, slot, depth+1)
						if e != nil {
							return "", e
						}
						out.WriteString(v)
						continue
					}
					if part.Kind != "paragraph" {
						return "", Error("DOCX_RENDER_NODE")
					}
					style := fmt.Sprintf("ArtexList%s%d", ordered, depth)
					p := wordP(part.Inlines, style, 21, false, false, "2D3142")
					if first {
						p = strings.Replace(p, `</w:pPr>`, `<w:ind w:left="`+strconv.Itoa(360*(depth+1))+`" w:hanging="240"/></w:pPr><w:r><w:rPr><w:rStyle w:val="ArtexMarker"/><w:rFonts w:ascii="맑은 고딕" w:hAnsi="맑은 고딕" w:eastAsia="맑은 고딕"/><w:color w:val="2D3142"/><w:sz w:val="21"/></w:rPr><w:t>`+marker+`</w:t><w:tab/></w:r>`, 1)
						first = false
					}
					out.WriteString(p)
				}
			}
		case "table":
			if slot == "appendix.customer_facts" {
				out.WriteString(wordTable(b.Rows, b.Alignments, "ArtexData", []int{680, 8391}))
			} else {
				out.WriteString(wordTable(b.Rows, b.Alignments, "ArtexData"))
			}
		case "component":
			c := b.Component
			switch c.Schema {
			case "artex-kpi-1":
				ws := make([]int, len(c.Items))
				for i := range ws {
					ws[i] = 9071 / len(ws)
				}
				out.WriteString(tableOpen("ArtexKPI", ws, "F8F9FB") + `<w:tr><w:trPr><w:cantSplit/></w:trPr>`)
				for i, k := range c.Items {
					v := wordP(plainInline(k.Value), "ArtexKPIValue", 52, true, true, "A84617")
					fmt.Fprintf(&out, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:fill="F8F9FB"/><w:tcMar><w:top w:w="180" w:type="dxa"/><w:left w:w="180" w:type="dxa"/><w:bottom w:w="180" w:type="dxa"/><w:right w:w="180" w:type="dxa"/></w:tcMar></w:tcPr>%s%s</w:tc>`, ws[i], v, wordP(plainInline(k.Label), "ArtexKPILabel", 18, false, false, "4F5D75"))
				}
				out.WriteString(`</w:tr></w:tbl>`)
			case "artex-detail-1":
				var rows [][][]Inline
				all := append([][]string{c.Columns}, c.Rows...)
				for _, row := range all {
					var cells [][]Inline
					for _, v := range row {
						cells = append(cells, plainInline(v))
					}
					rows = append(rows, cells)
				}
				out.WriteString(wordTable(rows, nil, "ArtexDetail"))
			case "artex-figure-1":
				if c.Eyebrow != "" {
					out.WriteString(wordP(plainInline(c.Eyebrow), "ArtexFigureEyebrow", 18, false, true, "4F5D75"))
				}
				if c.Title != "" {
					out.WriteString(wordP(plainInline(c.Title), "ArtexFigureTitle", 24, true, true, "2D3142"))
				}
				for _, p := range c.Pages {
					a, ok := w.assets[p.Ref]
					if !ok {
						return "", Error("FIGURE_REF_MISSING")
					}
					w.imageCount++
					id := fmt.Sprintf("rImage%d", w.imageCount)
					name := fmt.Sprintf("figure-%04d.png", w.imageCount)
					w.files["word/media/"+name] = a.PNG
					w.rels.WriteString(`<Relationship Id="` + id + `" Type="` + R + `/image" Target="media/` + name + `"/>`)
					height := int64(a.Item.Height) * 5760000 / int64(a.Item.Width)
					image := fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="ArtexFigurePage%d"/><w:wordWrap w:val="0"/></w:pPr><w:r><w:drawing><wp:inline><wp:extent cx="5760000" cy="%d"/><wp:docPr id="%d" name="%s" descr="%s"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="5760000" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`, p.Page, height, w.imageCount, xe(p.Ref), xe(w.notes[c.FigureID]), w.imageCount, xe(p.Ref), id, height)
					out.WriteString(tableOpen("ArtexFigurePanel", []int{9071}, "F5F5F5") + `<w:tr><w:trPr><w:cantSplit/></w:trPr><w:tc><w:tcPr><w:shd w:fill="F5F5F5"/></w:tcPr>` + image + `</w:tc></w:tr></w:tbl>`)
				}
			default:
				return "", Error("DOCX_RENDER_NODE")
			}
		default:
			return "", Error("DOCX_RENDER_NODE")
		}
	}
	return out.String(), nil
}
func DOCX(p Projection, in RenderInputs, assets map[string]Asset) ([]byte, error) {
	if !ValidInputs(in) {
		return nil, Error("RENDER_INPUT_INVALID")
	}
	w := &docxWriter{files: map[string][]byte{}, assets: assets, notes: map[string]string{}}
	var body strings.Builder
	for _, s := range p.Slots {
		if strings.HasPrefix(s.ID, "detail.") {
			var text strings.Builder
			for _, b := range s.Blocks {
				if b.Kind == "paragraph" {
					for _, i := range b.Inlines {
						text.WriteString(i.Text)
					}
				}
			}
			w.notes[strings.TrimPrefix(s.ID, "detail.")] = text.String()
		}
	}
	for _, s := range p.Slots {
		value, e := w.blocks(s.Blocks, s.ID, 0)
		if e != nil {
			return nil, e
		}
		lock := `<w:showingPlcHdr/>`
		if s.Locked {
			lock = `<w:lock w:val="sdtContentLocked"/>`
		}
		body.WriteString(`<w:sdt><w:sdtPr>` + lock + `<w:tag w:val="artex:v1:` + xe(s.ID) + `"/>` + `</w:sdtPr><w:sdtContent>` + value + `</w:sdtContent></w:sdt>`)
	}
	body.WriteString(`<w:sectPr><w:headerReference w:type="default" r:id="rHeader"/><w:footerReference w:type="default" r:id="rFooter"/><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="964" w:right="1417" w:bottom="1134" w:left="1417" w:header="567" w:footer="567"/></w:sectPr>`)
	w.files["word/document.xml"] = []byte(XHead + `<w:document` + Decl + `><w:body>` + body.String() + `</w:body></w:document>`)
	w.files["word/styles.xml"] = []byte(DOCXStyles)
	w.files["word/settings.xml"] = []byte(XHead + `<w:settings` + Decl + `><w:doNotAutoCompressPictures/></w:settings>`)
	w.files["word/header1.xml"] = []byte(XHead + `<w:hdr` + Decl + `>` + wordP(plainInline(in.HeaderText), "ArtexHeader", 18, false, false, "4F5D75") + `</w:hdr>`)
	w.files["word/footer1.xml"] = []byte(XHead + `<w:ftr` + Decl + `><w:p><w:pPr><w:tabs><w:tab w:val="right" w:pos="9071"/></w:tabs></w:pPr>` + wordInline(plainInline(in.FooterText), "", 18, false, "4F5D75") + `<w:r><w:tab/></w:r><w:fldSimple w:instr="PAGE"><w:r><w:rPr><w:sz w:val="18"/></w:rPr><w:t>1</w:t></w:r></w:fldSimple></w:p></w:ftr>`)
	t, _ := time.Parse(time.RFC3339, in.GeneratedAt)
	at := t.UTC().Format(time.RFC3339)
	w.files["docProps/core.xml"] = []byte(XHead + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>` + xe(in.Title) + `</dc:title><dc:creator>` + xe(in.Author) + `</dc:creator><cp:lastModifiedBy>ARTEX</cp:lastModifiedBy><dcterms:created xsi:type="dcterms:W3CDTF">` + at + `</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">` + at + `</dcterms:modified></cp:coreProperties>`)
	w.files["docProps/app.xml"] = []byte(XHead + `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>ARTEX</Application><Manager></Manager><Company></Company></Properties>`)
	w.files["word/_rels/document.xml.rels"] = []byte(XHead + `<Relationships xmlns="` + PR + `"><Relationship Id="rStyles" Type="` + R + `/styles" Target="styles.xml"/><Relationship Id="rSettings" Type="` + R + `/settings" Target="settings.xml"/><Relationship Id="rHeader" Type="` + R + `/header" Target="header1.xml"/><Relationship Id="rFooter" Type="` + R + `/footer" Target="footer1.xml"/>` + w.rels.String() + `</Relationships>`)
	w.files["_rels/.rels"] = []byte(XHead + `<Relationships xmlns="` + PR + `"><Relationship Id="rDoc" Type="` + R + `/officeDocument" Target="word/document.xml"/><Relationship Id="rCore" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rApp" Type="` + R + `/extended-properties" Target="docProps/app.xml"/></Relationships>`)
	types := map[string]string{"word/document.xml": WordCT + "document.main+xml", "word/styles.xml": WordCT + "styles+xml", "word/settings.xml": WordCT + "settings+xml", "word/header1.xml": WordCT + "header+xml", "word/footer1.xml": WordCT + "footer+xml", "docProps/core.xml": "application/vnd.openxmlformats-package.core-properties+xml", "docProps/app.xml": "application/vnd.openxmlformats-officedocument.extended-properties+xml"}
	var ct strings.Builder
	ct.WriteString(XHead + `<Types xmlns="` + CT + `"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="png" ContentType="image/png"/>`)
	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ct.WriteString(`<Override PartName="/` + k + `" ContentType="` + types[k] + `"/>`)
	}
	ct.WriteString(`</Types>`)
	w.files["[Content_Types].xml"] = []byte(ct.String())
	return Zip(w.files)
}
func Zip(files map[string][]byte) ([]byte, error) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		raw := files[name]
		var compressed bytes.Buffer
		f, e := flate.NewWriter(&compressed, flate.DefaultCompression)
		if e != nil {
			return nil, e
		}
		if _, e = f.Write(raw); e != nil {
			return nil, e
		}
		if e = f.Close(); e != nil {
			return nil, e
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Flags: 0x800, CRC32: crc32.ChecksumIEEE(raw), UncompressedSize64: uint64(len(raw)), CompressedSize64: uint64(compressed.Len())}
		h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		wr, e := z.CreateRaw(h)
		if e != nil {
			return nil, e
		}
		if _, e = wr.Write(compressed.Bytes()); e != nil {
			return nil, e
		}
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
