package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sourceRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	r, e := os.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	return r, dir
}
func TestSessionSourceBoundary(t *testing.T) {
	r, dir := sourceRoot(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	os.WriteFile(outside, []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(dir, "escape.md"))
	for _, p := range []string{"../outside.md", outside, "escape.md", "nested/../escape.md", "file.txt"} {
		if _, e := stageMarkdown(r, p, t.TempDir()); e == nil {
			t.Errorf("accepted unsafe source %q", p)
		}
	}
}
func TestSnapshotAndHTMLIsolation(t *testing.T) {
	r, _ := sourceRoot(t)
	valid := []byte("# 정상 보고서\n\n```html\n<script>alert(1)</script>\n```\n")
	r.WriteFile("report.md", valid, 0600)
	dst := t.TempDir()
	got, e := stageMarkdown(r, "report.md", dst)
	if e != nil {
		t.Fatal(e)
	}
	if string(got) != string(valid) {
		t.Fatal("source was changed")
	}
	r.WriteFile("report.md", []byte("replacement"), 0600)
	stored, _ := os.ReadFile(filepath.Join(dst, "report.md"))
	if string(stored) != string(valid) {
		t.Fatal("snapshot changed with original")
	}
	r.WriteFile("html.md", []byte("# x\n<script>run()</script>"), 0600)
	escapedDir := t.TempDir()
	if _, e := stageMarkdown(r, "html.md", escapedDir); e != nil {
		t.Fatal(e)
	}
	escaped, _ := os.ReadFile(filepath.Join(escapedDir, "html.md"))
	if strings.Contains(string(escaped), "<script>") || !strings.Contains(string(escaped), "&lt;script&gt;") {
		t.Fatal("evidence HTML was not escaped")
	}
	for _, bad := range []string{"![a](https://example.com/a.png)", "![a](../../a.png)", "```mermaid\na-->b\n```", "[unsafe](javascript:alert)", "[private](file:///etc/passwd)"} {
		r.WriteFile("bad.md", []byte(bad), 0600)
		if _, e := stageMarkdown(r, "bad.md", t.TempDir()); e == nil {
			t.Errorf("unsafe markup accepted: %q", bad)
		}
	}
}
func TestLocalImageBoundaryAndMissingAssets(t *testing.T) {
	r, _ := sourceRoot(t)
	r.MkdirAll("images", 0700)
	r.WriteFile("images/x.png", []byte("image"), 0600)
	r.WriteFile("report.md", []byte("# Report\n![local](images/x.png)"), 0600)
	dst := t.TempDir()
	if _, e := stageMarkdown(r, "report.md", dst); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(dst, "images/x.png")); e != nil || string(b) != "image" {
		t.Fatal("asset not snapshotted")
	}
	r.WriteFile("missing.md", []byte("![missing](images/missing.png)"), 0600)
	if _, e := stageMarkdown(r, "missing.md", t.TempDir()); e == nil {
		t.Fatal("missing image silently accepted")
	}
}
func TestRendererBusyAndInvalidSessionDoNotRun(t *testing.T) {
	sessions, out := t.TempDir(), t.TempDir()
	a, e := newApp(config{sessions, out, "/app/data/report-pdf", "/does-not-exist"})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	if _, e = a.render(context.Background(), renderInput{SessionID: "../conv-1"}); e == nil {
		t.Fatal("invalid session accepted")
	}
	a.busy <- struct{}{}
	_, e = a.render(context.Background(), renderInput{SessionID: "conv-1"})
	if e == nil || !strings.Contains(e.Error(), "busy") {
		t.Fatalf("busy renderer: %v", e)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 0 {
		t.Fatal("busy renderer created files")
	}
}
func TestReceiptAtomicReadAndPathGuard(t *testing.T) {
	a, e := newApp(config{t.TempDir(), t.TempDir(), "/app/data/report-pdf", "unused"})
	if e != nil {
		t.Fatal(e)
	}
	defer a.close()
	r := receipt{SessionID: "conv-1", JobID: "job-" + strings.Repeat("a", 32), Status: "HOLD", CustomerRelease: "HOLD"}
	a.output.MkdirAll(filepath.Join(r.SessionID, r.JobID), 0700)
	if e = a.save(r); e != nil {
		t.Fatal(e)
	}
	got, e := a.load(resultInput{r.SessionID, r.JobID})
	if e != nil || got.Status != "HOLD" {
		t.Fatalf("receipt: %v", e)
	}
	if _, e = a.load(resultInput{"conv-1", "../../secret"}); e == nil {
		t.Fatal("job traversal accepted")
	}
}
func TestBoundsDetectHorizontalAndVerticalClipping(t *testing.T) {
	for _, test := range []struct {
		x   string
		bad int
	}{{`<html><page width="595" height="842"><word xMin="10" xMax="300" yMin="10" yMax="20">ok</word></page></html>`, 0}, {`<html><page width="595" height="842"><word xMin="580" xMax="620" yMin="10" yMax="20">cut</word><word xMin="10" xMax="20" yMin="830" yMax="860">cut</word></page></html>`, 2}} {
		p, b, e := bboxCheck([]byte(test.x))
		if e != nil || p != 1 || b != test.bad {
			t.Fatalf("bounds got %d/%d/%v", p, b, e)
		}
	}
	if _, _, e := bboxCheck([]byte("not xml")); e == nil {
		t.Fatal("malformed bounds accepted")
	}
}
