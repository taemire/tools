package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func validFigureForTest() svgFigure {
	s := `<svg xmlns="http://www.w3.org/2000/svg" width="960" height="400" viewBox="0 0 960 400" font-family="ARTEXReport" role="img"><title>합성</title><rect width="960" height="400" fill="#FFFFFF"/><text x="32" y="80" font-size="20" fill="#23364D">합성 도식</text></svg>`
	return svgFigure{Name: "timeline-001.svg", Kind: "timeline", Page: 1, Width: 960, Height: 400, SVG: s, SHA256: figureHash([]byte(s))}
}
func TestFigureSVGAllowlist(t *testing.T) {
	f := validFigureForTest()
	if e := validateFigure(f); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*svgFigure){
		"script": func(f *svgFigure) { f.SVG = strings.Replace(f.SVG, "</svg>", "<script>alert(1)</script></svg>", 1) },
		"external-image": func(f *svgFigure) {
			f.SVG = strings.Replace(f.SVG, "</svg>", `<image href="https://example.invalid/x"/></svg>`, 1)
		},
		"foreignObject": func(f *svgFigure) { f.SVG = strings.Replace(f.SVG, "</svg>", "<foreignObject/></svg>", 1) },
		"css-url": func(f *svgFigure) {
			f.SVG = strings.Replace(f.SVG, `fill="#FFFFFF"`, `fill="url(https://example.invalid/x)"`, 1)
		},
		"event":   func(f *svgFigure) { f.SVG = strings.Replace(f.SVG, `role="img"`, `role="img" onload="alert(1)"`, 1) },
		"doctype": func(f *svgFigure) { f.SVG = `<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]>` + f.SVG },
		"namespace": func(f *svgFigure) {
			f.SVG = strings.Replace(f.SVG, `role="img"`, `xmlns:x="http://example.invalid" role="img"`, 1)
		},
		"tiny-font": func(f *svgFigure) { f.SVG = strings.Replace(f.SVG, `font-size="20"`, `font-size="8"`, 1) },
		"nan":       func(f *svgFigure) { f.SVG = strings.Replace(f.SVG, `x="32"`, `x="NaN"`, 1) },
		"escape":    func(f *svgFigure) { f.Name = "../timeline-001.svg" },
		"viewbox":   func(f *svgFigure) { f.SVG = strings.Replace(f.SVG, `0 0 960 400`, `0 0 960 1`, 1) },
	} {
		t.Run(name, func(t *testing.T) {
			x := f
			change(&x)
			x.SHA256 = figureHash([]byte(x.SVG))
			if e := validateFigure(x); e == nil {
				t.Fatal("unsafe SVG accepted")
			}
		})
	}
	f.SHA256 = strings.Repeat("0", 64)
	if e := validateFigure(f); e == nil {
		t.Fatal("wrong hash accepted")
	}
}
func TestFigureOutputLockAndFailure(t *testing.T) {
	out := filepath.Join(t.TempDir(), "new")
	release, e := claimFigureOutput(out)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = claimFigureOutput(out); e == nil {
		t.Fatal("concurrent writer accepted")
	}
	release()
	if e = os.Mkdir(out, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = claimFigureOutput(out); e == nil {
		t.Fatal("existing output overwritten")
	}
	f := validFigureForTest()
	bundle := svgBundle{Version: "customer-visuals-1", ModelSHA256: strings.Repeat("a", 64), Figures: []svgFigure{f}}
	_, e = renderFigureBundle(context.Background(), bundle, filepath.Join(t.TempDir(), "failed"), "missing-chrome", "missing-font")
	if e == nil || !strings.Contains(e.Error(), "FONT_UNAVAILABLE") {
		t.Fatal(e)
	}
}

func TestFigureOutputConcurrentClaim(t *testing.T) {
	out := filepath.Join(t.TempDir(), "one-output")
	start, finish := make(chan struct{}), make(chan struct{})
	var ready sync.WaitGroup
	var workers sync.WaitGroup
	var winners atomic.Int32
	for i := 0; i < 12; i++ {
		ready.Add(1)
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			release, e := claimFigureOutput(out)
			if e == nil {
				winners.Add(1)
			}
			ready.Done()
			if e == nil {
				<-finish
				release()
			}
		}()
	}
	close(start)
	ready.Wait()
	if winners.Load() != 1 {
		t.Errorf("writers admitted: %d", winners.Load())
	}
	close(finish)
	workers.Wait()
	release, e := claimFigureOutput(out)
	if e != nil {
		t.Fatal("lock not released", e)
	}
	release()
}
func TestFigureChromeIntegration(t *testing.T) {
	chrome, font := os.Getenv("REPORT_FIGURE_TEST_CHROME"), os.Getenv("REPORT_FIGURE_TEST_FONT")
	if chrome == "" || font == "" {
		t.Skip("explicit local renderer integration only")
	}
	in := svgBundle{Version: "customer-visuals-1", ModelSHA256: strings.Repeat("a", 64), Figures: []svgFigure{validFigureForTest()}}
	dir := t.TempDir()
	a, e := renderFigureBundle(context.Background(), in, filepath.Join(dir, "a"), chrome, font)
	if e != nil {
		t.Fatal(e)
	}
	b, e := renderFigureBundle(context.Background(), in, filepath.Join(dir, "b"), chrome, font)
	if e != nil {
		t.Fatal(e)
	}
	if a.Figures[0].PNGSHA256 != b.Figures[0].PNGSHA256 {
		t.Fatal("fixed local environment did not reproduce PNG")
	}
	preview, e := os.ReadFile(filepath.Join(dir, "a", "preview.html"))
	if e != nil {
		t.Fatal(e)
	}
	const embedded = `src="data:image/png;base64,`
	parts := strings.SplitN(string(preview), embedded, 2)
	if len(parts) != 2 || strings.Contains(string(preview), `src="timeline-001.png"`) {
		t.Fatal("preview still depends on sibling image files")
	}
	encoded := strings.SplitN(parts[1], `"`, 2)[0]
	pixels, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || figureHash(pixels) != a.Figures[0].PNGSHA256 {
		t.Fatal("embedded preview image differs from rendered PNG")
	}
	if a.Figures[0].MinPrintPt < 8 || a.Figures[0].MinContrast < 4.5 {
		t.Fatal("unreadable PNG")
	}
	bad := in
	bad.Figures = append([]svgFigure(nil), in.Figures...)
	bad.Figures[0].SVG = strings.Replace(bad.Figures[0].SVG, `fill="#23364D"`, `fill="#FFFFFF"`, 1)
	bad.Figures[0].SHA256 = figureHash([]byte(bad.Figures[0].SVG))
	out := filepath.Join(dir, "contrast-fail")
	if _, e = renderFigureBundle(context.Background(), bad, out, chrome, font); e == nil {
		t.Fatal("white on white accepted")
	}
	if _, e = os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("failed rendering published output")
	}
	stages, _ := filepath.Glob(filepath.Join(dir, ".figures-stage-*"))
	if len(stages) != 0 {
		t.Fatal("failed render left staged files")
	}
	_, e = json.Marshal(a)
	if e != nil {
		t.Fatal(e)
	}
}

func TestFigureTypographyAllowlist(t *testing.T) {
	for _, family := range []string{"ARTEXReport", "ARTEXMono"} {
		f := validFigureForTest()
		f.SVG = strings.Replace(f.SVG, `font-size="20"`, `font-size="20" font-weight="700" font-family="`+family+`"`, 1)
		f.SHA256 = figureHash([]byte(f.SVG))
		if e := validateFigure(f); e != nil {
			t.Fatal(e)
		}
	}
	for _, attr := range []string{`font-family="system-ui"`, `font-weight="900"`} {
		f := validFigureForTest()
		f.SVG = strings.Replace(f.SVG, `font-size="20"`, `font-size="20" `+attr, 1)
		f.SHA256 = figureHash([]byte(f.SVG))
		if e := validateFigure(f); e == nil {
			t.Fatal("unbound typography accepted", attr)
		}
	}
}
func TestFigureMonoFontRequired(t *testing.T) {
	font := os.Getenv("REPORT_FIGURE_TEST_FONT")
	if font == "" {
		t.Skip("trusted font integration fixture required")
	}
	f := validFigureForTest()
	f.SVG = strings.Replace(f.SVG, `font-size="20"`, `font-size="20" font-family="ARTEXMono"`, 1)
	f.SHA256 = figureHash([]byte(f.SVG))
	b := svgBundle{Version: "customer-visuals-1", LayoutVersion: "artex-task27-form-r2", ModelSHA256: strings.Repeat("a", 64), Figures: []svgFigure{f}}
	if _, e := renderFigureBundle(context.Background(), b, filepath.Join(t.TempDir(), "failed"), "missing-chrome", font); e == nil || e.Error() != "FIGURE_MONO_FONT_UNAVAILABLE" {
		t.Fatal("mono fallback accepted", e)
	}
}

func TestFigureRoundedPathVocabulary(t *testing.T) {
	for _, path := range []string{"M 32 80 L 90 80 Q 98 80 98 88 L 98 110 Q 98 118 106 118 L 130 118", "M 32 80 L 130 80"} {
		f := validFigureForTest()
		f.SVG = strings.Replace(f.SVG, "</svg>", `<path d="`+path+`" fill="none" stroke="#65758B" stroke-width="2"/></svg>`, 1)
		f.SHA256 = figureHash([]byte(f.SVG))
		if e := validateFigure(f); e != nil {
			t.Fatal(e)
		}
	}
	for _, path := range []string{"M 1 2 L 3 4", "M 1 2 C 1 2 3 4 5 6", "M 1 2 Q 3 4 5 6", "M NaN 2", "M 1 2 url(http://bad)", "M 1 2 M 3 4", "M 1 2 L -3 2", "M 1"} {
		if figurePathValid(path) {
			t.Fatal("unsafe or diagonal path accepted", path)
		}
	}
}
