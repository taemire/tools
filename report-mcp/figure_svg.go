package main

// Local prototype renderer. The MCP tool/ARTEX transaction wiring deliberately
// follows the PNG quality checkpoint; this CLI never reads operational sessions.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"golang.org/x/image/font/sfnt"
)

type svgFigure struct {
	DetailTableSHA256  string             `json:"detail_table_sha256,omitempty"`
	GeometryRulesPass  bool               `json:"geometry_rules_pass,omitempty"`
	GeometryPlanSHA256 string             `json:"geometry_plan_sha256,omitempty"`
	LayoutVariant      string             `json:"layout_variant,omitempty"`
	Caption            *figureCaption     `json:"caption,omitempty"`
	TextMetrics        []figureTextMetric `json:"text_metrics,omitempty"`
	Name               string             `json:"name"`
	Kind               string             `json:"kind"`
	Page               int                `json:"page"`
	Width              int                `json:"width"`
	Height             int                `json:"height"`
	MinPrintPt         float64            `json:"min_print_pt"`
	SHA256             string             `json:"svg_sha256"`
	SVG                string             `json:"svg"`
	SourceRefs         []string           `json:"source_refs"`
}
type svgBundle struct {
	DetailTables       []VisualDetailTable    `json:"detail_tables,omitempty"`
	LayoutRulesVersion string                 `json:"layout_rules_version,omitempty"`
	LayoutRulesSHA256  string                 `json:"layout_rules_sha256,omitempty"`
	Language           string                 `json:"language,omitempty"`
	PreviewLabels      map[string]string      `json:"preview_labels,omitempty"`
	ReviewWarnings     []figureDisplayWarning `json:"review_warnings,omitempty"`
	FontSHA256s        map[string]string      `json:"font_sha256s,omitempty"`
	TokensSHA256       string                 `json:"tokens_sha256,omitempty"`
	LayoutVersion      string                 `json:"layout_version,omitempty"`
	Version            string                 `json:"schema_version"`
	ModelSHA256        string                 `json:"model_sha256"`
	Figures            []svgFigure            `json:"figures"`
	Omissions          []struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	} `json:"omissions"`
	Ledger json.RawMessage `json:"fidelity_ledger"`
}
type figureResult struct {
	DetailTableSHA256  string  `json:"detail_table_sha256,omitempty"`
	GeometryRulesPass  bool    `json:"geometry_rules_pass,omitempty"`
	GeometryPlanSHA256 string  `json:"geometry_plan_sha256,omitempty"`
	LayoutVariant      string  `json:"layout_variant,omitempty"`
	MaxWidthDelta      float64 `json:"max_width_delta_px,omitempty"`
	Name               string  `json:"name"`
	Kind               string  `json:"kind"`
	Page               int     `json:"page"`
	SVGSHA256          string  `json:"svg_sha256"`
	PNGSHA256          string  `json:"png_sha256"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	MinPrintPt         float64 `json:"min_print_pt"`
	MinContrast        float64 `json:"min_contrast"`
}
type figuresReceipt struct {
	DetailTables       []VisualDetailTable `json:"detail_tables,omitempty"`
	LayoutRulesVersion string              `json:"layout_rules_version,omitempty"`
	LayoutRulesSHA256  string              `json:"layout_rules_sha256,omitempty"`
	FontSHA256s        map[string]string   `json:"font_sha256s,omitempty"`
	TokensSHA256       string              `json:"tokens_sha256,omitempty"`
	LayoutVersion      string              `json:"layout_version,omitempty"`
	MonoFontSHA256     string              `json:"mono_font_sha256,omitempty"`
	Version            string              `json:"schema_version"`
	ModelSHA256        string              `json:"model_sha256"`
	FontSHA256         string              `json:"font_sha256"`
	Renderer           string              `json:"renderer"`
	Scale              int                 `json:"scale"`
	Theme              string              `json:"theme"`
	Figures            []figureResult      `json:"figures"`
	Omissions          any                 `json:"omissions"`
	Status             string              `json:"status"`
	Ledger             json.RawMessage     `json:"fidelity_ledger,omitempty"`
}

func figureHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

var figureName = regexp.MustCompile(`^(summary_metrics|exploration_flow|asset_tree|timeline|attack_surface|attack_path|technique_map)-[0-9]{3}\.svg$`)
var figureHashRE = regexp.MustCompile(`^[a-f0-9]{64}$`)
var svgColor = regexp.MustCompile(`^(#[0-9A-Fa-f]{6}|none)$`)

// Only the exact static drawing vocabulary emitted by ARTEX is accepted. XML
// namespace aliases, CSS, URLs, images, scripts, active content and entities are
// rejected, independently of ARTEX's own safe string construction.
func validateFigure(f svgFigure) error {
	if !figureName.MatchString(f.Name) || f.Width != 960 || f.Height < 180 || (f.LayoutVariant == "" && f.Height < 300) || f.Height > 1080 || f.Page < 1 || f.Name != fmt.Sprintf("%s-%03d.svg", f.Kind, f.Page) || len(f.SVG) > 1<<20 || figureHash([]byte(f.SVG)) != f.SHA256 {
		return errors.New("FIGURE_INPUT_INVALID")
	}
	d := xml.NewDecoder(strings.NewReader(f.SVG))
	depth, count, roots, texts := 0, 0, 0, 0
	stack := []string{}
	allowed := map[string]map[string]bool{}
	for tag, attrs := range map[string]string{"svg": "width height viewBox font-family role", "g": "data-region", "rect": "x y width height rx fill stroke stroke-width stroke-dasharray", "text": "x y font-size font-weight font-family letter-spacing fill", "path": "d fill stroke stroke-width stroke-dasharray", "line": "x1 y1 x2 y2 stroke stroke-width", "polyline": "points fill stroke stroke-width stroke-dasharray", "polygon": "points fill stroke stroke-width", "circle": "cx cy r fill stroke stroke-width", "title": "", "desc": ""} {
		allowed[tag] = map[string]bool{}
		for _, a := range strings.Fields(attrs) {
			allowed[tag][a] = true
		}
	}
	for {
		tok, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return errors.New("FIGURE_XML_INVALID")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			count++
			if depth > 12 || count > 20000 {
				return errors.New("FIGURE_XML_LIMIT")
			}
			if t.Name.Space != "http://www.w3.org/2000/svg" {
				return errors.New("FIGURE_NAMESPACE_INVALID")
			}
			attrs, ok := allowed[t.Name.Local]
			if !ok {
				return errors.New("FIGURE_ELEMENT_REFUSED")
			}
			stack = append(stack, t.Name.Local)
			if depth == 1 {
				roots++
				if t.Name.Local != "svg" {
					return errors.New("FIGURE_ROOT_INVALID")
				}
			} else if t.Name.Local == "svg" {
				return errors.New("FIGURE_NESTED_SVG")
			}
			seen := map[string]bool{}
			hasSize := false
			for _, a := range t.Attr {
				if a.Name.Local == "xmlns" && a.Name.Space == "" && depth == 1 && a.Value == "http://www.w3.org/2000/svg" {
					continue
				}
				if a.Name.Space != "" || !attrs[a.Name.Local] || seen[a.Name.Local] {
					return errors.New("FIGURE_ATTRIBUTE_REFUSED")
				}
				seen[a.Name.Local] = true
				switch a.Name.Local {
				case "d":
					if !figurePathValid(a.Value) {
						return errors.New("FIGURE_PATH_INVALID")
					}
				case "fill", "stroke":
					if !svgColor.MatchString(a.Value) {
						return errors.New("FIGURE_COLOR_INVALID")
					}
				case "role":
					if a.Value != "img" {
						return errors.New("FIGURE_ROLE_INVALID")
					}
				case "font-family":
					if a.Value != "ARTEXReport" && !(t.Name.Local == "text" && a.Value == "ARTEXMono") {
						return errors.New("FIGURE_FONT_INVALID")
					}
				case "font-weight":
					if a.Value != "400" && a.Value != "500" && a.Value != "700" {
						return errors.New("FIGURE_FONT_WEIGHT_INVALID")
					}
				case "data-region":
					if a.Value != "card" {
						return errors.New("FIGURE_REGION_INVALID")
					}
				case "points", "viewBox", "stroke-dasharray":
					parts := strings.Fields(strings.ReplaceAll(a.Value, ",", " "))
					if len(parts) == 0 || len(parts) > 100 {
						return errors.New("FIGURE_POINTS_INVALID")
					}
					for _, x := range parts {
						if !figureNumber(x) {
							return errors.New("FIGURE_POINTS_INVALID")
						}
					}
					if a.Name.Local == "viewBox" && a.Value != fmt.Sprintf("0 0 %d %d", f.Width, f.Height) {
						return errors.New("FIGURE_VIEWBOX_INVALID")
					}
				default:
					if !figureNumber(a.Value) {
						return errors.New("FIGURE_NUMBER_INVALID")
					}
					v, _ := strconv.ParseFloat(a.Value, 64)
					if a.Name.Local == "font-size" {
						hasSize = true
						if v*160/25.4*72/960 < 8 {
							return errors.New("FIGURE_TEXT_TOO_SMALL")
						}
					}
					if depth == 1 && a.Name.Local == "width" && v != float64(f.Width) || depth == 1 && a.Name.Local == "height" && v != float64(f.Height) {
						return errors.New("FIGURE_SIZE_INVALID")
					}
				}
			}
			if t.Name.Local == "text" {
				texts++
				if !hasSize {
					return errors.New("FIGURE_TEXT_SIZE_REQUIRED")
				}
			}
		case xml.EndElement:
			depth--
			if len(stack) == 0 {
				return errors.New("FIGURE_XML_INVALID")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" && (len(stack) == 0 || !visualContains(stack[len(stack)-1], "text", "title", "desc")) {
				return errors.New("FIGURE_TEXT_LOCATION_INVALID")
			}
		case xml.Directive, xml.ProcInst, xml.Comment:
			return errors.New("FIGURE_DIRECTIVE_REFUSED")
		}
	}
	if depth != 0 || roots != 1 || texts == 0 {
		return errors.New("FIGURE_XML_INVALID")
	}
	return nil
}
func visualContains(s string, values ...string) bool {
	for _, x := range values {
		if s == x {
			return true
		}
	}
	return false
}

// Static M/L/Q only. Straight segments must be orthogonal; Q is limited
// to a corner whose control point follows the incoming/outgoing axes.
func figurePathValid(s string) bool {
	fields := strings.Fields(s)
	if len(fields) < 3 || len(fields) > 2000 {
		return false
	}
	x, y := 0.0, 0.0
	first := true
	for len(fields) > 0 {
		cmd := fields[0]
		n := 2
		if cmd == "Q" {
			n = 4
		} else if cmd != "M" && cmd != "L" {
			return false
		}
		if first && cmd != "M" || !first && cmd == "M" || len(fields) < n+1 {
			return false
		}
		v := make([]float64, n)
		for i := range v {
			if !figureNumber(fields[i+1]) {
				return false
			}
			v[i], _ = strconv.ParseFloat(fields[i+1], 64)
		}
		if cmd == "L" && v[0] != x && v[1] != y {
			return false
		}
		if cmd == "Q" && !((v[0] == x && v[3] == v[1]) || (v[1] == y && v[2] == v[0])) {
			return false
		}
		x, y = v[n-2], v[n-1]
		fields = fields[n+1:]
		first = false
	}
	return true
}
func figureNumber(s string) bool {
	v, e := strconv.ParseFloat(s, 64)
	return e == nil && !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 10000
}

func claimFigureOutput(out string) (func(), error) {
	if e := os.MkdirAll(filepath.Dir(out), 0700); e != nil {
		return nil, e
	}
	lock := out + ".lock"
	f, e := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, errors.New("FIGURE_OUTPUT_LOCKED")
	}
	info, e := f.Stat()
	_ = f.Close()
	if e != nil {
		return nil, e
	}
	release := func() {
		if current, e := os.Lstat(lock); e == nil && os.SameFile(info, current) {
			_ = os.Remove(lock)
		}
	}
	if _, e := os.Lstat(out); !os.IsNotExist(e) {
		release()
		return nil, errors.New("FIGURE_OUTPUT_MUST_BE_NEW")
	}
	return release, nil
}

const figureGeometryJS = `(() => {
 const svg=document.querySelector('svg'), failures=[];
 const texts=[...svg.querySelectorAll('text')], rects=[...svg.querySelectorAll('rect')];
 const rgb=s=>{const n=s.match(/[\d.]+/g);return n?n.slice(0,3).map(Number):[0,0,0]};
 const lum=c=>c.map(x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4}).reduce((a,x,i)=>a+x*[.2126,.7152,.0722][i],0);
 const bounds=e=>e.getBBox(), inside=(b,r)=>b.x>=r.x-1&&b.y>=r.y-1&&b.x+b.width<=r.x+r.width+1&&b.y+b.height<=r.y+r.height+1;
 let minPt=100,minContrast=100;
 const canvas={x:0,y:0,width:Number(svg.getAttribute('width')),height:Number(svg.getAttribute('height'))};
 for(const t of texts){
  const b=bounds(t),style=getComputedStyle(t),pt=parseFloat(style.fontSize)*160/25.4*72/960;minPt=Math.min(minPt,pt);
  if(!inside(b,canvas))failures.push('text_outside_canvas');
  if(!t.closest('g[data-region=card]'))for(const r of svg.querySelectorAll('g[data-region=card] > rect')){const z=bounds(r);if(Math.min(b.x+b.width,z.x+z.width)-Math.max(b.x,z.x)>1&&Math.min(b.y+b.height,z.y+z.height)-Math.max(b.y,z.y)>1)failures.push('text_overlaps_unrelated_card');}
  let bg='rgb(255,255,255)';for(const r of rects)if(inside(b,bounds(r)))bg=getComputedStyle(r).fill;
  const a=lum(rgb(style.fill)),z=lum(rgb(bg)),contrast=(Math.max(a,z)+.05)/(Math.min(a,z)+.05);minContrast=Math.min(minContrast,contrast);
 }
 for(const g of svg.querySelectorAll('g[data-region="card"]')){const r=g.querySelector('rect');for(const t of g.querySelectorAll('text'))if(!inside(bounds(t),bounds(r)))failures.push('text_outside_card');}
 for(let i=0;i<texts.length;i++)for(let j=i+1;j<texts.length;j++){const a=bounds(texts[i]),b=bounds(texts[j]);if(Math.min(a.x+a.width,b.x+b.width)-Math.max(a.x,b.x)>1&&Math.min(a.y+a.height,b.y+b.height)-Math.max(a.y,b.y)>1)failures.push('text_overlap');}
 if(minPt<8)failures.push('text_under_8pt');if(minContrast<4.5)failures.push('contrast_under_4.5');
 return {failures:[...new Set(failures)],min_pt:minPt,min_contrast:minContrast};
})()`

func figureChromeURL() string {
	return env("REPORT_FIGURE_CHROME_URL", os.Getenv("REPORT_CHROME_URL"))
}

func renderFigureBundle(parent context.Context, in svgBundle, out, chrome, fontPath string, monoPaths ...string) (figuresReceipt, error) {
	r := figuresReceipt{DetailTables: in.DetailTables, Version: "figure-receipt-1", LayoutVersion: in.LayoutVersion, LayoutRulesVersion: in.LayoutRulesVersion, LayoutRulesSHA256: in.LayoutRulesSHA256, FontSHA256s: in.FontSHA256s, TokensSHA256: in.TokensSHA256, ModelSHA256: in.ModelSHA256, Scale: 2, Theme: "light-still", Status: "HOLD", Omissions: in.Omissions, Ledger: in.Ledger}
	if (in.LayoutVersion != "" && in.LayoutVersion != "artex-task27-form-r2" && !isR3Layout(in.LayoutVersion)) || in.Version != "customer-visuals-1" || !figureHashRE.MatchString(in.ModelSHA256) || len(in.Figures) == 0 || len(in.Figures) > 100 {
		return r, errors.New("FIGURE_BUNDLE_INVALID")
	}
	if e := validateFigureDetails(in); e != nil {
		return r, e
	}
	seen := map[string]bool{}
	for _, f := range in.Figures {
		if in.LayoutVersion == "artex-layout-r3b-v1" && (in.LayoutRulesVersion == "" || !figureHashRE.MatchString(in.LayoutRulesSHA256) || f.LayoutVariant == "" || !f.GeometryRulesPass || !figureHashRE.MatchString(f.GeometryPlanSHA256)) {
			return r, errors.New("FIGURE_LAYOUT_PROFILE_REQUIRED")
		}
		if seen[f.Name] {
			return r, errors.New("FIGURE_DUPLICATE")
		}
		seen[f.Name] = true
		if e := validateFigure(f); e != nil {
			return r, e
		}
	}
	font, e := os.ReadFile(fontPath)
	if e != nil || len(font) > 20<<20 || len(font) < 100 {
		return r, errors.New("FIGURE_FONT_UNAVAILABLE")
	}
	r.FontSHA256 = figureHash(font)
	parsed, e := sfnt.Parse(font)
	if e != nil {
		return r, errors.New("FIGURE_FONT_INVALID")
	}
	var mono []byte
	var monoParsed *sfnt.Font
	needMono := isR3Layout(in.LayoutVersion)
	for _, f := range in.Figures {
		if strings.Contains(f.SVG, `font-family="ARTEXMono"`) {
			needMono = true
		}
	}
	if needMono {
		if len(monoPaths) < 1 || monoPaths[0] == "" {
			return r, errors.New("FIGURE_MONO_FONT_UNAVAILABLE")
		}
		mono, e = os.ReadFile(monoPaths[0])
		if e != nil || len(mono) < 100 || len(mono) > 20<<20 {
			return r, errors.New("FIGURE_MONO_FONT_UNAVAILABLE")
		}
		monoParsed, e = sfnt.Parse(mono)
		if e != nil {
			return r, errors.New("FIGURE_MONO_FONT_INVALID")
		}
		r.MonoFontSHA256 = figureHash(mono)
	}
	var r3Faces map[string]*sfnt.Font
	r3CSS := ""
	if isR3Layout(in.LayoutVersion) {
		r3Faces, r3CSS, e = r3LoadFaces(in, font, monoPaths)
		if e != nil {
			return r, e
		}
		for _, f := range in.Figures {
			if e = r3ValidateFaces(f, r3Faces); e != nil {
				return r, e
			}
		}
	}
	var glyphs sfnt.Buffer
	for _, f := range in.Figures {
		if isR3Layout(in.LayoutVersion) {
			continue
		}
		decoder := xml.NewDecoder(strings.NewReader(f.SVG))
		textFont := parsed
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return r, e
			}
			switch t := token.(type) {
			case xml.StartElement:
				if t.Name.Local == "text" {
					textFont = parsed
					for _, a := range t.Attr {
						if a.Name.Local == "font-family" && a.Value == "ARTEXMono" {
							textFont = monoParsed
						}
					}
				}
			case xml.CharData:
				for _, ch := range string(t) {
					if unicode.IsSpace(ch) {
						continue
					}
					index, e := textFont.GlyphIndex(&glyphs, ch)
					if e != nil || index == 0 {
						return r, errors.New("FIGURE_FONT_GLYPH_UNAVAILABLE")
					}
				}
			case xml.EndElement:
				if t.Name.Local == "text" {
					textFont = parsed
				}
			}
		}
	}
	monoCSS := ""
	if needMono {
		monoCSS = `@font-face{font-family:ARTEXMono;src:url(data:font/ttf;base64,` + base64.StdEncoding.EncodeToString(mono) + `)}`
	}
	release, e := claimFigureOutput(out)
	if e != nil {
		return r, e
	}
	defer release()
	stage, e := os.MkdirTemp(filepath.Dir(out), ".figures-stage-")
	if e != nil {
		return r, e
	}
	defer os.RemoveAll(stage)
	ctx, cancel := context.WithTimeout(parent, 180*time.Second)
	defer cancel()
	var alloc context.Context
	var stop context.CancelFunc
	if chromeURL := figureChromeURL(); chromeURL != "" {
		alloc, stop = chromedp.NewRemoteAllocator(ctx, chromeURL)
	} else {
		opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome), chromedp.Flag("disable-gpu", true), chromedp.Flag("force-color-profile", "srgb"), chromedp.Flag("disable-background-networking", true), chromedp.Flag("disable-extensions", true), chromedp.Flag("hide-scrollbars", true))
		alloc, stop = chromedp.NewExecAllocator(ctx, opts...)
	}
	defer stop()
	tab, closeTab := chromedp.NewContext(alloc)
	defer closeTab()
	if e = chromedp.Run(tab, chromedp.Navigate("about:blank"), network.Enable(), network.SetBlockedURLs([]string{"http://*", "https://*", "file://*", "ftp://*", "ws://*", "wss://*"})); e != nil {
		return r, fmt.Errorf("FIGURE_RENDER_UNAVAILABLE: %w", e)
	}
	if e = chromedp.Run(tab, chromedp.ActionFunc(func(c context.Context) error {
		_, product, revision, _, _, e := browser.GetVersion().Do(c)
		r.Renderer = "artex-svg-cpu-srgb-1/" + product + "/" + revision
		return e
	})); e != nil {
		return r, e
	}
	if isR3Layout(in.LayoutVersion) {
		r.Renderer = strings.Replace(r.Renderer, "artex-svg-cpu-srgb-1/", "artex-r3a-sfnt-1/", 1)
	}
	var preview strings.Builder
	preview.WriteString(`<!doctype html><html><meta charset="utf-8"><style>html{color-scheme:light;background:#fff}body{margin:32px auto;max-width:960px;font-family:system-ui;background:#fff;color:#2d3142;color-scheme:light}img{display:block;width:100%;margin:24px 0}aside{padding:16px;border:1px solid #999}code{font-family:monospace}.url-piece{white-space:nowrap;overflow-wrap:normal;word-break:keep-all}.detail-table table{width:100%;border-collapse:collapse;table-layout:fixed}.detail-table th,.detail-table td{padding:10px;border-bottom:1px solid #bfc0c0;text-align:left;vertical-align:top;white-space:pre-wrap;overflow-wrap:anywhere}.detail-table th:first-child,.detail-table td:first-child{width:70px}.detail-table th:nth-child(2),.detail-table td:nth-child(2){width:25%}.detail-table td{height:auto;max-height:none;overflow:visible;text-overflow:clip}</style>`)
	if isR3Layout(in.LayoutVersion) {
		header, e := r3PreviewHeader(in)
		if e != nil {
			return r, e
		}
		preview.WriteString(header)
	}
	for fIndex, f := range in.Figures {
		wrapperHTML := `<!doctype html><html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; font-src data:"><style>@font-face{font-family:ARTEXReport;src:url(data:font/ttf;base64,` + base64.StdEncoding.EncodeToString(font) + `)}` + monoCSS + `html,body{margin:0;width:960px;background:white}svg{display:block}</style>` + f.SVG + `</html>`
		if isR3Layout(in.LayoutVersion) {
			wrapperHTML = `<!doctype html><html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; font-src data:"><style>` + r3CSS + `html,body{margin:0;width:960px}svg{display:block}</style>` + f.SVG + `</html>`
		}
		var ready bool
		var geometry struct {
			Failures    []string `json:"failures"`
			MinPt       float64  `json:"min_pt"`
			MinContrast float64  `json:"min_contrast"`
		}
		var widths []float64
		var data []byte
		e = chromedp.Run(tab, emulation.SetDeviceMetricsOverride(960, int64(f.Height), 2, false), chromedp.ActionFunc(func(c context.Context) error {
			tree, e := page.GetFrameTree().Do(c)
			if e != nil {
				return e
			}
			return page.SetDocumentContent(tree.Frame.ID, wrapperHTML).Do(c)
		}), chromedp.Poll(`document.fonts.status==='loaded' && document.fonts.check('20px ARTEXReport') && (!document.querySelector('text[font-family=ARTEXMono]') || document.fonts.check('18px ARTEXMono'))`, &ready, chromedp.WithPollingTimeout(10*time.Second)), chromedp.Evaluate(figureGeometryJS, &geometry), chromedp.Evaluate(`Array.from(document.querySelectorAll("svg text"),e=>e.getComputedTextLength())`, &widths), chromedp.ActionFunc(func(c context.Context) error {
			var e error
			data, e = page.CaptureScreenshot().WithCaptureBeyondViewport(false).Do(c)
			return e
		}))
		if e != nil {
			return r, fmt.Errorf("FIGURE_RENDER_UNAVAILABLE: %w", e)
		}
		maxDelta := 0.0
		if isR3Layout(in.LayoutVersion) {
			if geometry.MinPt < 9 || len(widths) != len(f.TextMetrics) {
				return r, errors.New("FIGURE_R3_METRIC_CONTRACT")
			}
			for i, actual := range widths {
				delta := math.Abs(actual - f.TextMetrics[i].Width)
				maxDelta = math.Max(maxDelta, delta)
				if delta > 2 {
					return r, fmt.Errorf("FIGURE_WIDTH_DELTA:%s:%d:%.3f/%.3f", f.Name, i, actual, f.TextMetrics[i].Width)
				}
			}
		}
		if len(geometry.Failures) > 0 {
			return r, fmt.Errorf("FIGURE_VISUAL_CHECK: %s: %v", f.Name, geometry.Failures)
		}
		cfg, e := png.DecodeConfig(bytes.NewReader(data))
		if e != nil || cfg.Width != f.Width*2 || cfg.Height != f.Height*2 {
			return r, errors.New("FIGURE_PNG_SIZE_INVALID")
		}
		name := strings.TrimSuffix(f.Name, ".svg") + ".png"
		if e = os.WriteFile(filepath.Join(stage, name), data, 0600); e != nil {
			return r, e
		}
		if e = os.WriteFile(filepath.Join(stage, f.Name), []byte(f.SVG), 0600); e != nil {
			return r, e
		}
		r.Figures = append(r.Figures, figureResult{DetailTableSHA256: f.DetailTableSHA256, GeometryRulesPass: f.GeometryRulesPass, GeometryPlanSHA256: f.GeometryPlanSHA256, LayoutVariant: f.LayoutVariant, Name: name, Kind: f.Kind, Page: f.Page, SVGSHA256: f.SHA256, PNGSHA256: figureHash(data), Width: cfg.Width, Height: cfg.Height, MinPrintPt: geometry.MinPt, MinContrast: geometry.MinContrast, MaxWidthDelta: maxDelta})
		preview.WriteString(figureCaptionHTML(f.Caption))
		preview.WriteString(`<img alt="` + f.Kind + ` page ` + strconv.Itoa(f.Page) + `" src="data:image/png;base64,` + base64.StdEncoding.EncodeToString(data) + `">`)
		if fIndex == len(in.Figures)-1 || in.Figures[fIndex+1].Kind != f.Kind {
			preview.WriteString(figureDetailsHTML(in, f.Kind))
		}
		preview.WriteString(figureFootnoteHTML(f.Caption))
	}
	if len(in.DetailTables) > 0 {
		details, e := json.MarshalIndent(in.DetailTables, "", "  ")
		if e != nil {
			return r, e
		}
		if e = os.WriteFile(filepath.Join(stage, "details.json"), details, 0600); e != nil {
			return r, e
		}
	}
	r.Status = "FIGURES_MACHINE_CHECKS_PASS"
	receiptJSON, _ := json.MarshalIndent(r, "", "  ")
	if e = os.WriteFile(filepath.Join(stage, "receipt.json"), receiptJSON, 0600); e != nil {
		return r, e
	}
	if e = os.WriteFile(filepath.Join(stage, "preview.html"), []byte(preview.String()+`</html>`), 0600); e != nil {
		return r, e
	}
	if e = os.Rename(stage, out); e != nil {
		return r, e
	}
	return r, nil
}
func figuresCLI(args []string) error {
	f := flag.NewFlagSet("render-figures", flag.ContinueOnError)
	input := f.String("input", "", "customer-visuals-1 bundle JSON")
	out := f.String("out", "", "new output directory")
	chrome := f.String("chrome", env("REPORT_FIGURE_CHROME", "google-chrome"), "Chrome executable fallback when no Chrome URL is configured")
	font := f.String("font", env("REPORT_FIGURE_FONT", "/usr/share/fonts/truetype/nanum/NanumGothic.ttf"), "trusted local Korean font")
	medium := f.String("medium-font", "", "trusted real sans 500 face")
	bold := f.String("bold-font", "", "trusted real sans 700 face")
	mono := f.String("mono-font", env("REPORT_FIGURE_MONO_FONT", ""), "trusted local monospace font (required when used)")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *input == "" || *out == "" {
		return errors.New("--input and --out required")
	}
	raw, e := os.ReadFile(*input)
	if e != nil {
		return e
	}
	if len(raw) > 24<<20 {
		return errors.New("FIGURE_BUNDLE_LIMIT")
	}
	var in svgBundle
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&in); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("FIGURE_JSON_TRAILING")
	}
	r, e := renderFigureBundle(context.Background(), in, *out, *chrome, *font, *mono, *medium, *bold)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(r)
}
