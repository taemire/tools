package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"report-mcp/artexmd"
)

func fallbackRenderFixture(t *testing.T, title, lang string) (*app, renderInput, string) {
	t.Helper()
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	session := filepath.Join(sessions, "conv-1")
	out := filepath.Join(root, "out")
	if err := os.MkdirAll(filepath.Join(session, "figures"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"schema_version":"artex-figure-bundle-1","items":[]}`)
	for name, b := range map[string][]byte{"report.md": []byte(":::slot meta.cover locked\n## " + title + "\n\n" + title + "\n:::\n"), "figures/manifest.json": manifest, "regular.ttf": goregular.TTF, "bold.ttf": gobold.TTF} {
		if err := os.WriteFile(filepath.Join(session, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("REPORT_V3_FONT_REGULAR", filepath.Join(session, "regular.ttf"))
	t.Setenv("REPORT_V3_FONT_BOLD", filepath.Join(session, "bold.ttf"))
	a, err := newApp(config{sessions, out, out, "/unused"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.close() })
	return a, renderInput{SessionID: "conv-1", MarkdownFile: "report.md", SourceFormat: "artex-md-3", Formats: []string{"html", "docx"}, RenderInputs: &artexmd.RenderInputs{GeneratedAt: "2026-10-30T00:00:00Z", Title: title, HeaderText: "Header", FooterText: "Footer", Language: lang, ManifestSHA256: artexmd.Hash(manifest)}}, out
}

func TestV3CJKFallbackEmbedding(t *testing.T) {
	for _, tc := range []struct{ lang, title, family string }{{"ja", "レポート検証", "ReportCJKJP"}, {"zh", "报告验证", "ReportCJKSC"}} {
		t.Run(tc.lang, func(t *testing.T) {
			a, in, out := fallbackRenderFixture(t, tc.title, tc.lang)
			receipt, err := a.render(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(out, in.SessionID, receipt.JobID, "report.html"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), tc.family) || !strings.Contains(string(b), "data:font/woff2;base64,") {
				t.Fatal("CJK text has no embedded font capable of rendering it")
			}
			if len(b) > 32<<20 {
				t.Fatal("fallback exceeded HTML delivery limit")
			}
			if receipt.V3 == nil || len(receipt.V3.FontSHA256) <= 2 {
				t.Fatal("fallback font hashes absent from receipt")
			}
		})
	}
}

func TestV3UnsupportedGlyphDoesNotPublishSuccess(t *testing.T) {
	t.Setenv("REPORT_CHROME_URL", "http://127.0.0.1:1")
	a, in, _ := fallbackRenderFixture(t, "Unsupported \U0010ffff", "en")
	in.Formats = []string{"pdf"}
	_, err := a.render(context.Background(), in)
	if err == nil || err.Error() != "RENDER_GLYPH_UNAVAILABLE" {
		t.Fatal("unsupported text was not refused", err)
	}
}

func TestV3EditableFormatsDoNotRequirePDFFontCoverage(t *testing.T) {
	a, in, _ := fallbackRenderFixture(t, "Editable \U0010ffff", "en")
	receipt, err := a.render(context.Background(), in)
	if err != nil || receipt.Checks["font_glyph_coverage"] != "UNVERIFIED" {
		t.Fatal("editable formats were gated by PDF font support", err, receipt.Checks)
	}
}
