package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFigureChromeURL(t *testing.T) {
	for _, tc := range []struct {
		name, figureURL, reportURL, want string
	}{
		{"exec fallback", "", "", ""},
		{"shared Chrome", "", "http://127.0.0.1:9223", "http://127.0.0.1:9223"},
		{"figure Chrome", "http://127.0.0.1:9224", "", "http://127.0.0.1:9224"},
		{"figure override", "http://127.0.0.1:9224", "http://127.0.0.1:9223", "http://127.0.0.1:9224"},
		{"websocket URL", "ws://127.0.0.1:9224/devtools/browser/test", "http://127.0.0.1:9223", "ws://127.0.0.1:9224/devtools/browser/test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("REPORT_FIGURE_CHROME_URL", tc.figureURL)
			t.Setenv("REPORT_CHROME_URL", tc.reportURL)
			if got := figureChromeURL(); got != tc.want {
				t.Fatalf("Chrome URL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestR3PinnedFaceProfile(t *testing.T) {
	root := os.Getenv("REPORT_R3_FIXTURE")
	fonts := os.Getenv("REPORT_R3_FONTS")
	if root == "" || fonts == "" {
		t.Skip("explicit pinned local proof inputs required")
	}
	raw, e := os.ReadFile(filepath.Join(root, "bundle.json"))
	if e != nil {
		t.Fatal(e)
	}
	var b svgBundle
	if e = json.Unmarshal(raw, &b); e != nil {
		t.Fatal(e)
	}
	font, e := os.ReadFile(filepath.Join(fonts, "Pretendard-Regular.otf"))
	if e != nil {
		t.Fatal(e)
	}
	paths := []string{filepath.Join(fonts, "D2Coding.ttf"), filepath.Join(fonts, "Pretendard-Medium.otf"), filepath.Join(fonts, "Pretendard-Bold.otf")}
	faces, css, e := r3LoadFaces(b, font, paths)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(css, "font-synthesis:none") || !strings.Contains(css, "font-kerning:none") {
		t.Fatal("synthetic font rendering permitted")
	}
	for _, f := range b.Figures {
		if e = r3ValidateFaces(f, faces); e != nil {
			t.Fatal(e)
		}
	}
	if _, _, e = r3LoadFaces(b, font, paths[:2]); e == nil {
		t.Fatal("missing face accepted")
	}
	wrong := append([]string(nil), paths...)
	wrong[2] = paths[1]
	if _, _, e = r3LoadFaces(b, font, wrong); e == nil {
		t.Fatal("wrong bold file accepted")
	}
	f := b.Figures[0]
	f.SVG = strings.Replace(f.SVG, `font-weight="700"`, `font-weight="600"`, 1)
	if e = r3ValidateFaces(f, faces); e == nil {
		t.Fatal("synthetic weight accepted")
	}
	f = b.Figures[0]
	f.TextMetrics = nil
	if e = r3ValidateFaces(f, faces); e == nil {
		t.Fatal("unmeasured text accepted")
	}
	f = b.Figures[0]
	f.SHA256 = strings.Repeat("0", 64)
	if e = validateFigure(f); e == nil {
		t.Fatal("changed SVG accepted")
	}
}
