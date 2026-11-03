package artexmd

import (
	"encoding/base64"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type RenderInputs struct {
	GeneratedAt    string `json:"generated_at"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	HeaderText     string `json:"header_text"`
	FooterText     string `json:"footer_text"`
	Language       string `json:"language"`
	ManifestSHA256 string `json:"figure_manifest_sha256"`
}
type Font struct {
	Weight int
	Data   []byte
}

const HTMLTemplate = "artex-html-customer-3.1"
const HTMLStyle = `.url-piece{white-space:nowrap;overflow-wrap:normal;word-break:keep-all}@page{size:A4;margin:17mm 25mm 20mm}*{box-sizing:border-box}body{font:10.5pt/1.45 ReportKorean,sans-serif;color:#2d3142;background:white;margin:0;overflow-wrap:anywhere}h1,h2,h3{line-height:1.25;break-after:avoid;page-break-after:avoid;color:#2d3142}h1{font-size:24pt;margin:8mm 0 5mm}h2{font-size:16pt;margin:7mm 0 3mm}h3{font-size:12pt;margin:5mm 0 2mm}p{margin:0 0 3mm;orphans:2;widows:2;white-space:pre-wrap}li p{margin:0}ul,ol{padding-left:7mm}table{width:100%;table-layout:fixed;border-collapse:collapse;margin:3mm 0 5mm;font-size:9pt}thead{display:table-header-group}th,td{border:1px solid #ccd3dc;padding:2mm;vertical-align:top;text-align:left;white-space:pre-wrap;overflow-wrap:anywhere}th{background:#edf1f5}tr{break-inside:avoid;page-break-inside:avoid}.kpis{display:flex;gap:3mm;margin:5mm 0}.kpi{flex:1;min-width:0;border:1px solid #d7dde4;padding:4mm;background:#f8f9fb}.kpi-value{font:700 27pt/1.15 ReportKorean,sans-serif;display:block;color:#a84617}.kpi-label{font-size:9pt;display:block;margin-top:2mm}.figure-panel{width:16cm;max-width:100%;padding:0;background:#f5f5f5;break-inside:avoid}.figure-panel img{display:block;width:100%;height:auto}.eyebrow{font-size:9pt;letter-spacing:1pt;color:#4f5d75}.edit{border-left:2pt solid #c85427;padding-left:3mm}.banner{padding:3mm;background:#fff1f0;color:#9d261c;border-left:3pt solid #b42318}code{font:9.5pt ReportKorean,monospace}.header{font-size:8pt;color:#4f5d75;margin-bottom:3mm}.detail{break-before:avoid}.detail td:first-child{width:8%}.review-note{margin-bottom:4mm}[data-artex-slot^="figure_title."] h2{margin-top:2mm}.figure-title{break-before:page;page-break-before:always;break-after:avoid}.figure-title+section{break-after:avoid;page-break-after:avoid}.split-row{break-inside:auto;page-break-inside:auto}`

func esc(s string) string { return html.EscapeString(s) }
func ValidInputs(in RenderInputs) bool {
	_, e := time.Parse(time.RFC3339, in.GeneratedAt)
	return e == nil && len(in.Title) <= 600 && len(in.Author) <= 300 && len(in.HeaderText) <= 1000 && len(in.FooterText) <= 1000 && (in.Language == "ko" || in.Language == "en" || in.Language == "zh" || in.Language == "es" || in.Language == "ja")
}
func HTML(p Projection, in RenderInputs, assets map[string]Asset, fonts []Font, fallbacks ...FallbackFont) ([]byte, error) {
	if !ValidInputs(in) {
		return nil, Error("RENDER_INPUT_INVALID")
	}
	var out strings.Builder
	out.WriteString(`<!doctype html><html lang="` + esc(in.Language) + `"><head><meta charset="utf-8"/><title>` + esc(in.Title) + `</title>`)
	licenses := map[string]bool{}
	for _, font := range fallbacks {
		if !licenses[font.Family] {
			out.WriteString(`<meta name="font-license-` + esc(font.Family) + `" content="` + esc(font.License) + `"/>`)
			licenses[font.Family] = true
		}
	}
	out.WriteString(`<style>` + HTMLStyle)
	weights := map[int]bool{}
	for _, f := range fonts {
		if (f.Weight != 400 && f.Weight != 700) || len(f.Data) < 4 || len(f.Data) > 20<<20 || string(f.Data[:4]) == "OTTO" {
			return nil, Error("RENDER_TTF_REQUIRED")
		}
		weights[f.Weight] = true
		fmt.Fprintf(&out, `@font-face{font-family:ReportKorean;font-weight:%d;src:url(data:font/ttf;base64,%s)}`, f.Weight, base64.StdEncoding.EncodeToString(f.Data))
	}
	if !weights[400] || !weights[700] {
		return nil, Error("RENDER_TTF_REQUIRED")
	}
	fontStack := "ReportKorean"
	licensed := map[string]bool{}
	for _, font := range fallbacks {
		if font.Family != "ReportCJKJP" && font.Family != "ReportCJKSC" || font.Weight != 400 && font.Weight != 700 || len(font.Data) < 4 || len(font.Data) > 20<<20 || string(font.Data[:4]) != "wOF2" {
			return nil, Error("RENDER_FALLBACK_FONT_INVALID")
		}
		if !licensed[font.Family] {
			fontStack += "," + font.Family
			licensed[font.Family] = true
		}
		fmt.Fprintf(&out, `@font-face{font-family:%s;font-weight:%d;src:url(data:font/woff2;base64,%s) format("woff2")}`, font.Family, font.Weight, base64.StdEncoding.EncodeToString(font.Data))
	}
	if len(fallbacks) > 0 {
		out.WriteString(`body,.kpi-value,code{font-family:` + fontStack + `,sans-serif}`)
	}
	var footerEsc strings.Builder
	for _, r := range in.FooterText {
		fmt.Fprintf(&footerEsc, `\%x `, r)
	}
	footer := footerEsc.String()
	out.WriteString(`@page{@bottom-left{content:"` + footer + `";font:8pt ` + fontStack + `;color:#4f5d75}@bottom-right{content:counter(page);font:8pt ` + fontStack + `}}</style></head><body>`)
	out.WriteString(`<header class="header">` + esc(in.HeaderText) + `</header><main>`)
	for _, s := range p.Slots {
		class := ""
		if !s.Locked {
			class = "edit"
		}
		if s.ID == "review.banner" {
			class = "banner"
		}
		if strings.HasPrefix(s.ID, "figure_title.") || s.ID == "appendix.customer_facts" {
			class += " figure-title"
		}
		mode := "edit"
		if s.Locked {
			mode = "locked"
		}
		out.WriteString(`<section class="` + class + `" data-artex-slot="` + esc(s.ID) + `" data-artex-mode="` + mode + `">`)
		for i, b := range s.Blocks {
			if e := htmlBlock(&out, b, assets, s.ID+"-"+strconv.Itoa(i), s.ID == "meta.cover" && i == 0); e != nil {
				return nil, e
			}
		}
		out.WriteString(`</section>`)
	}
	out.WriteString(`</main></body></html>`)
	return []byte(out.String()), nil
}
func htmlInline(out *strings.Builder, ins []Inline) {
	for _, i := range ins {
		switch i.Kind {
		case "text":
			out.WriteString(`<span data-artex-inline="text">` + urlHTML(i.Text) + `</span>`)
		case "hard_break", "soft_break":
			out.WriteString(`<br data-artex-inline="` + i.Kind + `"/>`)
		case "code":
			out.WriteString(`<code data-artex-inline="code">` + esc(i.Text) + `</code>`)
		case "strong", "emphasis":
			tag := "em"
			if i.Kind == "strong" {
				tag = "strong"
			}
			out.WriteString(`<` + tag + ` data-artex-inline="` + i.Kind + `">`)
			htmlInline(out, i.Children)
			out.WriteString(`</` + tag + `>`)
		}
	}
}
func htmlBlock(out *strings.Builder, b Block, assets map[string]Asset, id string, cover bool) error {
	attr := ` data-artex-block="` + b.Kind + `"`
	switch b.Kind {
	case "paragraph", "heading":
		tag := "p"
		if b.Kind == "heading" {
			tag = "h" + strconv.Itoa(b.Level)
			if cover {
				tag = "h1"
			}
			attr += ` data-artex-level="` + strconv.Itoa(b.Level) + `" id="` + esc(id) + `"`
		}
		out.WriteString(`<` + tag + attr + `>`)
		htmlInline(out, b.Inlines)
		out.WriteString(`</` + tag + `>`)
	case "list":
		tag := "ul"
		if b.Ordered {
			tag = "ol"
		}
		fmt.Fprintf(out, `<%s%s data-artex-ordered="%t" start="%d">`, tag, attr, b.Ordered, b.Start)
		for _, item := range b.Items {
			out.WriteString(`<li>`)
			for _, part := range item {
				if e := htmlBlock(out, part, assets, id, false); e != nil {
					return e
				}
			}
			out.WriteString(`</li>`)
		}
		out.WriteString(`</` + tag + `>`)
	case "table":
		out.WriteString(`<table` + attr + `>`)
		if strings.HasPrefix(id, "appendix.customer_facts-") {
			out.WriteString(`<colgroup><col style="width:8%"/><col style="width:92%"/></colgroup>`)
		}
		for r, row := range b.Rows {
			if r == 0 {
				out.WriteString(`<thead>`)
			} else if r == 1 {
				out.WriteString(`<tbody>`)
			}
			out.WriteString(rowHTML(rowMaySplit(row)))
			for c, cell := range row {
				tag := "td"
				if r == 0 {
					tag = "th"
				}
				align := 0
				if c < len(b.Alignments) {
					align = b.Alignments[c]
				}
				fmt.Fprintf(out, `<%s data-artex-align="%d">`, tag, align)
				htmlInline(out, cell)
				out.WriteString(`</` + tag + `>`)
			}
			out.WriteString(`</tr>`)
			if r == 0 {
				out.WriteString(`</thead>`)
			}
		}
		if len(b.Rows) > 1 {
			out.WriteString(`</tbody>`)
		}
		out.WriteString(`</table>`)
	case "component":
		c := b.Component
		if c == nil {
			return Error("MD_COMPONENT_INVALID")
		}
		out.WriteString(`<div` + attr + ` data-artex-component="` + esc(c.Schema) + `" data-artex-figure="` + esc(c.FigureID) + `">`)
		switch c.Schema {
		case "artex-kpi-1":
			out.WriteString(`<div class="kpis">`)
			for i, k := range c.Items {
				fmt.Fprintf(out, `<div class="kpi" data-artex-item="%d" data-artex-label-id="%s"><span class="kpi-value">%s</span><span class="kpi-label">%s</span></div>`, i, esc(k.LabelID), esc(k.Value), esc(k.Label))
			}
			out.WriteString(`</div>`)
		case "artex-figure-1":
			if c.Eyebrow != "" {
				out.WriteString(`<p class="eyebrow">` + esc(c.Eyebrow) + `</p>`)
			}
			if c.Title != "" {
				out.WriteString(`<h3>` + esc(c.Title) + `</h3>`)
			}
			for _, p := range c.Pages {
				a, ok := assets[p.Ref]
				if !ok {
					return Error("FIGURE_REF_MISSING")
				}
				fmt.Fprintf(out, `<div class="figure-panel"><img data-artex-item="%d" data-artex-ref="%s" src="data:image/png;base64,%s" alt="%s"/></div>`, p.Page, esc(p.Ref), base64.StdEncoding.EncodeToString(a.PNG), esc(c.Title))
			}
		case "artex-detail-1":
			out.WriteString(`<table class="detail">`)
			if len(c.Columns) == 3 {
				out.WriteString(`<colgroup><col style="width:7.5%"/><col style="width:22.5%"/><col style="width:70%"/></colgroup>`)
			}
			out.WriteString(`<thead><tr>`)
			for _, col := range c.Columns {
				out.WriteString(`<th>` + esc(col) + `</th>`)
			}
			out.WriteString(`</tr></thead><tbody>`)
			for _, row := range c.Rows {
				out.WriteString(rowHTML(stringsRuneCount(row) > 400))
				for _, cell := range row {
					out.WriteString(`<td>` + urlHTML(cell) + `</td>`)
				}
				out.WriteString(`</tr>`)
			}
			out.WriteString(`</tbody></table>`)
		default:
			return Error("MD_COMPONENT_UNKNOWN")
		}
		out.WriteString(`</div>`)
	default:
		return Error("MD_NODE_UNSUPPORTED")
	}
	return nil
}

// The same Unicode-code-point threshold governs HTML and DOCX row splitting.
func stringsRuneCount(row []string) int {
	n := 0
	for _, s := range row {
		n += utf8.RuneCountInString(s)
	}
	return n
}
func rowMaySplit(row [][]Inline) bool {
	var plain func([]Inline) int
	plain = func(xs []Inline) int {
		n := 0
		for _, x := range xs {
			n += utf8.RuneCountInString(x.Text) + plain(x.Children)
			if x.Kind == "hard_break" || x.Kind == "soft_break" {
				n++
			}
		}
		return n
	}
	n := 0
	for _, x := range row {
		n += plain(x)
	}
	return n > 400
}
func rowHTML(split bool) string {
	if split {
		return `<tr class="split-row">`
	}
	return `<tr>`
}
