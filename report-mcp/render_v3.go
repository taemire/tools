package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"report-mcp/artexmd"
)

type v3Receipt struct {
	SourceFormat     string               `json:"source_format"`
	HTMLSHA256       string               `json:"html_sha256,omitempty"`
	DOCXPath         string               `json:"docx_path,omitempty"`
	DOCXSHA256       string               `json:"docx_sha256,omitempty"`
	ProjectionSHA256 string               `json:"projection_sha256"`
	ReportMCPVersion string               `json:"report_mcp_version"`
	ArtexmdVersion   string               `json:"artexmd_version"`
	GoldmarkVersion  string               `json:"goldmark_version"`
	TemplateSHA256   string               `json:"template_sha256"`
	StyleSHA256      string               `json:"style_sha256"`
	HTMLStyleSHA256  string               `json:"html_style_sha256"`
	TokensSHA256     string               `json:"tokens_sha256"`
	FontSHA256       []string             `json:"font_sha256"`
	Figures          []artexmd.FigureItem `json:"figures"`
}

func loadV3Fonts() ([]artexmd.Font, error) {
	var fonts []artexmd.Font
	for _, f := range []struct {
		w         int
		key, path string
	}{{400, "REPORT_V3_FONT_REGULAR", "/usr/share/fonts/truetype/pretendard/Pretendard-Regular.ttf"}, {700, "REPORT_V3_FONT_BOLD", "/usr/share/fonts/truetype/pretendard/Pretendard-Bold.ttf"}} {
		b, e := os.ReadFile(env(f.key, f.path))
		if e != nil || len(b) > 20<<20 {
			return nil, errors.New("RENDER_TTF_REQUIRED")
		}
		fonts = append(fonts, artexmd.Font{Weight: f.w, Data: b})
	}
	return fonts, nil
}
func (a *app) renderV3(ctx context.Context, in renderInput, chrome string) (r receipt, err error) {
	if !sessionPattern.MatchString(in.SessionID) || in.RenderInputs == nil || !artexmd.ValidInputs(*in.RenderInputs) || validRel(in.MarkdownFile) != nil {
		return r, errors.New("RENDER_INPUT_INVALID")
	}
	formats := map[string]bool{}
	for _, f := range in.Formats {
		if f != "html" && f != "pdf" && f != "docx" {
			return r, errors.New("RENDER_FORMAT_INVALID")
		}
		if formats[f] {
			return r, errors.New("RENDER_FORMAT_INVALID")
		}
		formats[f] = true
	}
	if len(formats) == 0 {
		return r, errors.New("RENDER_FORMAT_INVALID")
	}
	select {
	case a.busy <- struct{}{}:
		defer func() { <-a.busy }()
	default:
		return r, errors.New("renderer is busy")
	}
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	s, e := a.sessions.OpenRoot(in.SessionID)
	if e != nil {
		return r, e
	}
	defer s.Close()
	source, e := boundedRead(s, in.MarkdownFile, 2<<20)
	if e != nil {
		return r, e
	}
	parse := a.parseV3
	if parse == nil {
		parse = artexmd.Parse
	}
	d, e := parse(source)
	if e != nil {
		return r, e
	}
	assets, manifest, e := artexmd.LoadAssets(s, in.RenderInputs.ManifestSHA256, d)
	if e != nil {
		return r, e
	}
	projection := d.ProjectionWithAssets(assets)
	projectJSON, e := artexmd.CanonicalJSON(projection)
	if e != nil {
		return r, e
	}
	fonts, e := loadV3Fonts()
	if e != nil {
		return r, e
	}
	fallbacks, e := artexmd.ResolveFallbackFonts(projection, *in.RenderInputs, fonts)
	fontCoverage := e == nil
	if e != nil && (formats["pdf"] || e.Error() != "RENDER_GLYPH_UNAVAILABLE") {
		return r, e
	}
	html, e := artexmd.HTML(projection, *in.RenderInputs, assets, fonts, fallbacks...)
	if e != nil {
		return r, e
	}
	// Independent read-back from visible HTML values, not an embedded projection.
	actual, e := artexmd.HTMLProjection(html)
	if e != nil {
		return r, e
	}
	got, _ := artexmd.CanonicalJSON(actual)
	if string(got) != string(projectJSON) {
		return r, errors.New("HTML_PROJECTION_MISMATCH")
	}
	var id [16]byte
	if _, e = rand.Read(id[:]); e != nil {
		return r, e
	}
	now := a.nowV3
	if now == nil {
		now = time.Now
	}
	r = receipt{SessionID: in.SessionID, JobID: "job-" + hex.EncodeToString(id[:]), Status: "RENDERING", CreatedAt: now().UTC().Format(time.RFC3339), SourceSHA256: artexmd.Hash(source), CustomerRelease: "HOLD_REQUIRES_CONTENT_AND_VISUAL_REVIEW", PreviewPaths: []string{}}
	rel := filepath.Join(r.SessionID, r.JobID)
	if e = a.output.MkdirAll(rel, 0750); e != nil {
		return r, e
	}
	dir := filepath.Join(a.cfg.output, rel)
	defer func() {
		if err != nil {
			r.Status = "FAILED"
			if r.Checks == nil {
				r.Checks = map[string]string{}
			}
			r.Checks["validation_error"] = err.Error()
			_ = a.save(r)
		}
	}()
	r.V3 = &v3Receipt{SourceFormat: "artex-md-3", HTMLSHA256: artexmd.Hash(html), ProjectionSHA256: artexmd.Hash(projectJSON), ReportMCPVersion: version, ArtexmdVersion: artexmd.Version, GoldmarkVersion: "v1.6.0", TokensSHA256: artexmd.VisualTokensSHA256, TemplateSHA256: artexmd.TemplateSHA256(), StyleSHA256: artexmd.Hash([]byte(artexmd.DOCXStyles)), HTMLStyleSHA256: artexmd.Hash([]byte(artexmd.HTMLStyle)), Figures: manifest.Items}
	for _, f := range fonts {
		r.V3.FontSHA256 = append(r.V3.FontSHA256, artexmd.Hash(f.Data))
	}
	for _, f := range fallbacks {
		r.V3.FontSHA256 = append(r.V3.FontSHA256, artexmd.Hash(f.Data))
	}
	for name, data := range map[string][]byte{"report.html": html, "source.md": source, "projection.json": projectJSON} {
		if e = os.WriteFile(filepath.Join(dir, name), data, 0640); e != nil {
			return r, e
		}
	}
	inputJSON, _ := json.Marshal(in.RenderInputs)
	if e = os.WriteFile(filepath.Join(dir, "render-inputs.json"), inputJSON, 0640); e != nil {
		return r, e
	}
	r.HTMLPath = filepath.ToSlash(filepath.Join(a.cfg.public, rel, "report.html"))
	if formats["docx"] {
		docx, e := artexmd.DOCX(projection, *in.RenderInputs, assets)
		if e != nil {
			return r, e
		}
		if e = os.WriteFile(filepath.Join(dir, "report.docx"), docx, 0640); e != nil {
			return r, e
		}
		r.V3.DOCXPath = filepath.ToSlash(filepath.Join(a.cfg.public, rel, "report.docx"))
		r.V3.DOCXSHA256 = artexmd.Hash(docx)
	}
	if formats["pdf"] {
		if e = printV3(ctx, filepath.Join(dir, "report.html"), filepath.Join(dir, "report.pdf"), chrome); e != nil {
			return r, e
		}
		if e = checkV3FontUsage(ctx, filepath.Join(dir, "report.pdf"), fonts, fallbacks); e != nil {
			return r, e
		}
		r.PDFPath = filepath.ToSlash(filepath.Join(a.cfg.public, rel, "report.pdf"))
		if e = a.inspect(ctx, &r); e != nil {
			return r, e
		}
		if e = checkV3PDF(ctx, filepath.Join(dir, "report.pdf"), projection); e != nil {
			return r, e
		}
		r.Checks["toc"] = "PASS_BOOKMARK_DESTINATIONS"
		r.Checks["no_type3_fonts"] = "PASS"
		r.Checks["embedded_font_selection"] = "PASS"
	} else {
		r.Status = "GENERATED_MACHINE_CHECKS_PASS"
		r.Checks = map[string]string{"content_review": "UNVERIFIED", "visual_review": "UNVERIFIED"}
	}
	r.Checks["html_projection"] = "PASS"
	r.Checks["font_glyph_coverage"] = "PASS"
	if !fontCoverage {
		r.Checks["font_glyph_coverage"] = "UNVERIFIED"
	}
	r.Checks["ast_parse_count"] = "1"
	if e = a.save(r); e != nil {
		return r, e
	}
	return r, nil
}
func printV3(ctx context.Context, html, pdf, chrome string) error {
	var alloc context.Context
	var cancel context.CancelFunc
	if chrome != "" {
		opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome), chromedp.Flag("disable-background-networking", true), chromedp.Flag("disable-extensions", true), chromedp.Flag("force-color-profile", "srgb"))
		alloc, cancel = chromedp.NewExecAllocator(ctx, opts...)
	} else {
		alloc, cancel = chromedp.NewRemoteAllocator(ctx, env("REPORT_CHROME_URL", "http://127.0.0.1:9223"))
	}
	defer cancel()
	tab, close := chromedp.NewContext(alloc)
	defer close()
	var b []byte
	e := chromedp.Run(tab, chromedp.Navigate("file://"+html), chromedp.Poll(`document.fonts.status === 'loaded' && Array.from(document.images).every(i=>i.complete&&i.naturalWidth>0)`, nil, chromedp.WithPollingTimeout(60*time.Second)), chromedp.ActionFunc(checkV3DOMFonts), chromedp.ActionFunc(func(c context.Context) error {
		var e error
		b, _, e = page.PrintToPDF().WithPrintBackground(true).WithPreferCSSPageSize(true).WithGenerateDocumentOutline(true).WithGenerateTaggedPDF(true).Do(c)
		return e
	}))
	if e != nil {
		return e
	}
	return os.WriteFile(pdf, b, 0640)
}

type pdfOutline struct {
	Title string       `json:"title"`
	Page  int          `json:"destpageposfrom1"`
	Kids  []pdfOutline `json:"kids"`
}

func checkV3PDF(ctx context.Context, pdf string, p artexmd.Projection) error {
	fonts, e := run(ctx, "pdffonts", pdf)
	if e != nil {
		return e
	}
	if strings.Contains(fonts, "Type 3") {
		return errors.New("PDF_TYPE3_FONT")
	}
	data, e := run(ctx, "qpdf", "--json", "--json-key=outlines", "--json-key=pages", pdf)
	if e != nil {
		return e
	}
	var tree struct {
		Pages    []json.RawMessage `json:"pages"`
		Outlines []pdfOutline      `json:"outlines"`
	}
	if json.Unmarshal([]byte(data), &tree) != nil {
		return errors.New("PDF_DESTINATIONS_INVALID")
	}
	var wanted []string
	var plain func([]artexmd.Inline) string
	plain = func(xs []artexmd.Inline) string {
		var b strings.Builder
		for _, x := range xs {
			if len(x.Children) > 0 {
				b.WriteString(plain(x.Children))
			} else if x.Kind == "hard_break" || x.Kind == "soft_break" {
				b.WriteByte(' ')
			} else {
				b.WriteString(x.Text)
			}
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	for _, s := range p.Slots {
		for _, b := range s.Blocks {
			if b.Kind == "heading" {
				wanted = append(wanted, plain(b.Inlines))
			}
			if b.Component != nil && b.Component.Title != "" {
				wanted = append(wanted, strings.Join(strings.Fields(b.Component.Title), " "))
			}
		}
	}
	var actual []string
	var walk func([]pdfOutline) error
	walk = func(xs []pdfOutline) error {
		for _, x := range xs {
			if x.Page < 1 || x.Page > len(tree.Pages) {
				return errors.New("PDF_DESTINATIONS_INVALID")
			}
			actual = append(actual, strings.Join(strings.Fields(x.Title), " "))
			if e := walk(x.Kids); e != nil {
				return e
			}
		}
		return nil
	}
	if e = walk(tree.Outlines); e != nil {
		return e
	}
	if len(wanted) != len(actual) {
		return fmt.Errorf("PDF_DESTINATIONS_MISSING: want %d got %d", len(wanted), len(actual))
	}
	for i := range wanted {
		if wanted[i] != actual[i] {
			return errors.New("PDF_DESTINATIONS_MISMATCH")
		}
	}
	return nil
}
func renderV3CLI(args []string) error {
	f := flag.NewFlagSet("render-v3", flag.ContinueOnError)
	sessions := f.String("sessions", "", "session root")
	output := f.String("out", "", "output root")
	input := f.String("input", "", "JSON render request")
	chrome := f.String("chrome", "", "isolated local Chrome executable")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *sessions == "" || *output == "" || *input == "" {
		return errors.New("sessions, out, input required")
	}
	if e := os.MkdirAll(*output, 0700); e != nil {
		return e
	}
	raw, e := os.ReadFile(*input)
	if e != nil {
		return e
	}
	var in renderInput
	if e = json.Unmarshal(raw, &in); e != nil {
		return e
	}
	if in.SourceFormat != "artex-md-3" {
		return errors.New("SOURCE_FORMAT_UNSUPPORTED")
	}
	a, e := newApp(config{*sessions, *output, *output, "/not-used-by-v3"})
	if e != nil {
		return e
	}
	defer a.close()
	r, e := a.renderV3(context.Background(), in, *chrome)
	if e != nil {
		return e
	}
	b, _ := json.Marshal(r)
	fmt.Println(string(b))
	return nil
}
