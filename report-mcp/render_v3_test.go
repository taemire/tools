package main

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"report-mcp/artexmd"
)

func TestV3SingleParseAndDeterminism(t *testing.T) {
	base := t.TempDir()
	session := filepath.Join(base, "sessions", "task-1")
	if e := os.MkdirAll(filepath.Join(session, "figures"), 0700); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(base, "out")
	os.Mkdir(out, 0700)
	md := []byte(":::slot meta.cover locked\n## Synthetic\n\nhttp://example.invalid/\\<id\\> &amp; \\`\\`\\`\\\nnext line\n:::\n")
	for _, f := range []string{"first.md", "different-name.md"} {
		os.WriteFile(filepath.Join(session, f), md, 0600)
	}
	manifest := []byte(`{"schema_version":"artex-figure-bundle-1","items":[]}`)
	os.WriteFile(filepath.Join(session, "figures/manifest.json"), manifest, 0600)
	for key, data := range map[string][]byte{"REPORT_V3_FONT_REGULAR": goregular.TTF, "REPORT_V3_FONT_BOLD": gobold.TTF} {
		p := filepath.Join(base, key+".ttf")
		os.WriteFile(p, data, 0600)
		t.Setenv(key, p)
	}
	a, e := newApp(config{filepath.Join(base, "sessions"), out, out, "/missing-legacy-converter"})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	calls := 0
	a.parseV3 = func(b []byte) (*artexmd.Document, error) { calls++; return artexmd.Parse(b) }
	in := renderInput{SessionID: "task-1", MarkdownFile: "first.md", SourceFormat: "artex-md-3", Formats: []string{"html", "docx"}, RenderInputs: &artexmd.RenderInputs{GeneratedAt: "2026-10-17T00:00:00Z", Title: "Synthetic", HeaderText: "Header", FooterText: "Footer", Language: "en", ManifestSHA256: artexmd.Hash(manifest)}}
	a.nowV3 = func() time.Time { return time.Date(2026, 10, 17, 14, 59, 59, 0, time.UTC) }
	one, e := a.render(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatalf("parse calls %d", calls)
	}
	in.MarkdownFile = "different-name.md"
	t.Setenv("TZ", "America/Los_Angeles")
	a.nowV3 = func() time.Time { return time.Date(2026, 10, 17, 15, 0, 1, 0, time.UTC) }
	two, e := a.render(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatalf("expected exactly one parse per job, got %d", calls)
	}
	if one.CreatedAt == two.CreatedAt {
		t.Fatal("clock injection ineffective")
	}
	for _, name := range []string{"report.html", "report.docx"} {
		x, _ := os.ReadFile(filepath.Join(out, "task-1", one.JobID, name))
		y, _ := os.ReadFile(filepath.Join(out, "task-1", two.JobID, name))
		if len(x) == 0 || !bytes.Equal(x, y) {
			t.Fatal("artifact varies with wall clock/TZ/path/name", name)
		}
	}
	doc, _ := os.ReadFile(filepath.Join(out, "task-1", one.JobID, "report.docx"))
	z, e := zip.NewReader(bytes.NewReader(doc), int64(len(doc)))
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range z.File {
		if f.Flags&8 != 0 {
			t.Fatalf("data descriptor: %s", f.Name)
		}
	}
	bad := in
	os.WriteFile(filepath.Join(session, "bad.md"), []byte("bad v3 with no slots"), 0600)
	bad.MarkdownFile = "bad.md"
	if _, e = a.render(context.Background(), bad); e == nil {
		t.Fatal("bad v3 silently fell back")
	}
}
